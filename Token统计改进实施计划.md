# ccNexus Token 统计系统改进实现计划

## 1. 概述

### 1.1 改进目标
- ✅ **双层统计**：保留每日聚合统计 + 新增请求级别详细统计
- ✅ **三分类 Cache Token**：区分标准输入/缓存创建/缓存读取
- ✅ **向后兼容**：历史数据保持不变
- ✅ **准确记录**：每次请求记录所属端点和详细 token 消耗

### 1.2 实施原则
1. 数据库 Schema 通过 migration 方式平滑升级
2. 保留旧接口的同时添加新接口
3. 新字段对历史记录为 NULL/0，不影响现有查询
4. 请求级别统计支持自动清理（默认保留 90 天）

---

## 2. 数据库 Schema 修改

### 2.1 修改 `daily_stats` 表

**Migration SQL：**
```sql
-- 添加 cache token 字段
ALTER TABLE daily_stats ADD COLUMN cache_creation_tokens INTEGER DEFAULT 0;
ALTER TABLE daily_stats ADD COLUMN cache_read_tokens INTEGER DEFAULT 0;

-- 创建复合索引优化查询
CREATE INDEX IF NOT EXISTS idx_daily_stats_composite
ON daily_stats(endpoint_name, date, device_id);
```

**完整 Schema：**
```sql
CREATE TABLE daily_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint_name TEXT NOT NULL,
    date TEXT NOT NULL,
    requests INTEGER DEFAULT 0,
    errors INTEGER DEFAULT 0,
    input_tokens INTEGER DEFAULT 0,              -- 标准输入 token
    cache_creation_tokens INTEGER DEFAULT 0,     -- 缓存创建 token（新增）
    cache_read_tokens INTEGER DEFAULT 0,         -- 缓存读取 token（新增）
    output_tokens INTEGER DEFAULT 0,
    device_id TEXT DEFAULT 'default',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(endpoint_name, date, device_id)
);
```

### 2.2 新增 `request_stats` 表

**创建 SQL：**
```sql
CREATE TABLE IF NOT EXISTS request_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint_name TEXT NOT NULL,
    request_id TEXT,                             -- 可选：用于关联日志
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    date TEXT NOT NULL,                          -- 冗余字段便于按日期查询
    input_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    model TEXT,                                  -- 使用的模型
    is_streaming BOOLEAN DEFAULT FALSE,
    success BOOLEAN DEFAULT TRUE,
    device_id TEXT DEFAULT 'default'
);

-- 性能优化索引
CREATE INDEX IF NOT EXISTS idx_request_stats_endpoint ON request_stats(endpoint_name);
CREATE INDEX IF NOT EXISTS idx_request_stats_date ON request_stats(date);
CREATE INDEX IF NOT EXISTS idx_request_stats_timestamp ON request_stats(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_request_stats_composite ON request_stats(endpoint_name, date, device_id);
```

**字段说明：**
- `request_id`: 未来可用于关联请求日志（当前可为空）
- `date`: 从 `timestamp` 提取的日期字符串（YYYY-MM-DD），便于快速按日期查询
- `model`: 记录实际使用的模型名称
- `is_streaming`: 标记是否为流式请求
- `success`: 标记请求是否成功（当前仅记录成功请求）

---

## 3. 数据结构定义

### 3.1 核心类型 (`internal/transformer/types.go`)

**新增结构体：**
```go
// TokenUsageDetail 详细的 token 使用量分类
type TokenUsageDetail struct {
    InputTokens              int  // 标准输入 token
    CacheCreationInputTokens int  // 缓存创建 token
    CacheReadInputTokens     int  // 缓存读取 token
    OutputTokens             int  // 输出 token
}

// TotalInputTokens 返回所有输入 token 的总和
func (t *TokenUsageDetail) TotalInputTokens() int {
    return t.InputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
}

// ExtractTokenUsageDetail 从响应中提取详细的 token 使用量
func ExtractTokenUsageDetail(usage map[string]interface{}) TokenUsageDetail {
    detail := TokenUsageDetail{}

    if val, ok := usage["input_tokens"].(float64); ok {
        detail.InputTokens = int(val)
    }
    if val, ok := usage["cache_creation_input_tokens"].(float64); ok {
        detail.CacheCreationInputTokens = int(val)
    }
    if val, ok := usage["cache_read_input_tokens"].(float64); ok {
        detail.CacheReadInputTokens = int(val)
    }
    if val, ok := usage["output_tokens"].(float64); ok {
        detail.OutputTokens = int(val)
    }

    return detail
}

// ExtractInputTokens 保留向后兼容（返回总和）
func ExtractInputTokens(usage map[string]interface{}) int {
    return ExtractTokenUsageDetail(usage).TotalInputTokens()
}
```

### 3.2 Proxy 层类型 (`internal/proxy/stats.go`)

**更新现有结构体：**
```go
// StatRecord 每日聚合统计记录
type StatRecord struct {
    EndpointName          string
    Date                  string
    Requests              int
    Errors                int
    InputTokens           int
    CacheCreationTokens   int  // 新增
    CacheReadTokens       int  // 新增
    OutputTokens          int
    DeviceID              string
}

// RequestStatRecord 请求级别统计记录（新增）
type RequestStatRecord struct {
    EndpointName          string
    RequestID             string
    Timestamp             time.Time
    Date                  string
    InputTokens           int
    CacheCreationTokens   int
    CacheReadTokens       int
    OutputTokens          int
    Model                 string
    IsStreaming           bool
    Success               bool
    DeviceID              string
}

// DailyStats 每日统计数据（用于查询返回）
type DailyStats struct {
    Requests              int
    Errors                int
    InputTokens           int
    CacheCreationTokens   int  // 新增
    CacheReadTokens       int  // 新增
    OutputTokens          int
}
```

### 3.3 Storage 层类型 (`internal/storage/interface.go`)

**更新现有结构体：**
```go
// DailyStat 数据库记录
type DailyStat struct {
    ID                    int64
    EndpointName          string
    Date                  string
    Requests              int
    Errors                int
    InputTokens           int
    CacheCreationTokens   int  // 新增
    CacheReadTokens       int  // 新增
    OutputTokens          int
    DeviceID              string
    CreatedAt             time.Time
}

// RequestStat 请求级别数据库记录（新增）
type RequestStat struct {
    ID                    int64
    EndpointName          string
    RequestID             string
    Timestamp             time.Time
    Date                  string
    InputTokens           int
    CacheCreationTokens   int
    CacheReadTokens       int
    OutputTokens          int
    Model                 string
    IsStreaming           bool
    Success               bool
    DeviceID              string
}

// EndpointStats 端点统计汇总
type EndpointStats struct {
    Requests              int
    Errors                int
    InputTokens           int64
    CacheCreationTokens   int64  // 新增
    CacheReadTokens       int64  // 新增
    OutputTokens          int64
}
```

---

## 4. Token 提取逻辑修改

### 4.1 非流式响应 (`internal/proxy/response.go`)

**修改 `extractTokenUsage` 函数：**
```go
// extractTokenUsage 提取详细的 token 使用量
func extractTokenUsage(responseBody []byte) transformer.TokenUsageDetail {
    var resp map[string]interface{}
    if err := json.Unmarshal(responseBody, &resp); err != nil {
        return transformer.TokenUsageDetail{}
    }

    if usageMap, ok := resp["usage"].(map[string]interface{}); ok {
        return transformer.ExtractTokenUsageDetail(usageMap)
    }

    return transformer.TokenUsageDetail{}
}
```

**修改 `handleNonStreamingResponse` 函数签名：**
```go
// handleNonStreamingResponse 处理非流式响应
func (p *Proxy) handleNonStreamingResponse(w http.ResponseWriter, resp *http.Response,
    endpoint config.Endpoint, trans transformer.Transformer) (transformer.TokenUsageDetail, error) {

    // ... 现有的响应处理逻辑 ...

    // 提取 token 使用量
    usage := extractTokenUsage(transformedResp)

    // ... 返回响应给客户端 ...

    return usage, nil
}
```

### 4.2 流式响应 (`internal/proxy/streaming.go`)

**修改 `extractTokensFromEvent` 函数：**
```go
// extractTokensFromEvent 从 SSE 事件中提取详细的 token 使用量
func (p *Proxy) extractTokensFromEvent(eventData []byte, usage *transformer.TokenUsageDetail) {
    scanner := bufio.NewScanner(bytes.NewReader(eventData))
    for scanner.Scan() {
        line := scanner.Text()
        if !strings.HasPrefix(line, "data:") {
            continue
        }

        jsonData := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
        var event map[string]interface{}
        if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
            continue
        }

        eventType, _ := event["type"].(string)

        // message_start 事件包含输入 token 信息
        if eventType == "message_start" {
            if message, ok := event["message"].(map[string]interface{}); ok {
                if usageMap, ok := message["usage"].(map[string]interface{}); ok {
                    detail := transformer.ExtractTokenUsageDetail(usageMap)
                    usage.InputTokens = detail.InputTokens
                    usage.CacheCreationInputTokens = detail.CacheCreationInputTokens
                    usage.CacheReadInputTokens = detail.CacheReadInputTokens
                }
            }
        }

        // message_delta 事件包含输出 token 信息
        if eventType == "message_delta" {
            if usageMap, ok := event["usage"].(map[string]interface{}); ok {
                if output, ok := usageMap["output_tokens"].(float64); ok {
                    usage.OutputTokens = int(output)
                }
            }
        }
    }
}
```

**修改 `handleStreamingResponse` 函数签名：**
```go
// handleStreamingResponse 处理流式响应
func (p *Proxy) handleStreamingResponse(w http.ResponseWriter, resp *http.Response,
    endpoint config.Endpoint, trans transformer.Transformer, transformerName string,
    thinkingEnabled bool, modelName string, bodyBytes []byte) (transformer.TokenUsageDetail, string) {

    var usage transformer.TokenUsageDetail
    var outputText strings.Builder

    // ... 流式处理循环 ...

    // 在循环中提取 token
    p.extractTokensFromEvent(transformedEvent, &usage)

    // ... 其他处理逻辑 ...

    return usage, outputText.String()
}
```

---

## 5. 统计记录层修改

### 5.1 Stats 接口更新 (`internal/proxy/stats.go`)

**修改 StatsStorage 接口：**
```go
// StatsStorage 统计存储接口
type StatsStorage interface {
    RecordDailyStat(stat interface{}) error
    RecordRequestStat(stat interface{}) error  // 新增
    GetTotalStats() (int, map[string]interface{}, error)
    GetDailyStats(endpointName, startDate, endDate string) ([]interface{}, error)
    GetRequestStats(endpointName string, startDate, endDate string, limit, offset int) ([]interface{}, error)  // 新增
    GetRequestStatsCount(endpointName string, startDate, endDate string) (int, error)  // 新增
}
```

**更新 Stats 方法：**
```go
// RecordTokens 记录详细的 token 使用量到每日聚合统计
func (s *Stats) RecordTokens(endpointName string, usage transformer.TokenUsageDetail) {
    date := time.Now().Format("2006-01-02")

    stat := &StatRecord{
        EndpointName:        endpointName,
        Date:                date,
        Requests:            0,
        Errors:              0,
        InputTokens:         usage.InputTokens,
        CacheCreationTokens: usage.CacheCreationInputTokens,
        CacheReadTokens:     usage.CacheReadInputTokens,
        OutputTokens:        usage.OutputTokens,
        DeviceID:            s.deviceID,
    }

    if err := s.storage.RecordDailyStat(stat); err != nil {
        logger.Error("Failed to record tokens: %v", err)
    }
}

// RecordRequestStat 记录请求级别的详细统计（新增）
func (s *Stats) RecordRequestStat(record *RequestStatRecord) {
    record.DeviceID = s.deviceID
    record.Date = record.Timestamp.Format("2006-01-02")

    if err := s.storage.RecordRequestStat(record); err != nil {
        logger.Error("Failed to record request stat: %v", err)
    }
}
```

**更新 DailyStats 结构体：**
```go
// DailyStats 每日统计数据
type DailyStats struct {
    Requests              int
    Errors                int
    InputTokens           int
    CacheCreationTokens   int  // 新增
    CacheReadTokens       int  // 新增
    OutputTokens          int
}
```

### 5.2 Stats 查询方法更新

**修改 `GetDailyStats` 和 `GetPeriodStats`：**
```go
// GetDailyStats 获取指定日期的统计（需要累加 cache tokens）
func (s *Stats) GetDailyStats(date string) map[string]*DailyStats {
    // ... 现有查询逻辑 ...
    // 更新返回的 DailyStats 结构体包含 cache token 字段
}

// GetPeriodStats 获取时间段的统计（需要累加 cache tokens）
func (s *Stats) GetPeriodStats(startDate, endDate string) map[string]*DailyStats {
    // ... 现有查询逻辑 ...
    // 更新返回的 DailyStats 结构体包含 cache token 字段
}
```

---

## 6. 存储层实现

### 6.1 SQLite Migration (`internal/storage/sqlite.go`)

**添加 migration 函数：**
```go
// migrateCacheTokens 添加 cache token 字段到 daily_stats 表
func (s *SQLiteStorage) migrateCacheTokens() error {
    // 检查列是否已存在
    var count int
    err := s.db.QueryRow(`
        SELECT COUNT(*) FROM pragma_table_info('daily_stats')
        WHERE name='cache_creation_tokens'
    `).Scan(&count)
    if err != nil {
        return err
    }

    if count == 0 {
        _, err := s.db.Exec(`
            ALTER TABLE daily_stats ADD COLUMN cache_creation_tokens INTEGER DEFAULT 0;
        `)
        if err != nil {
            return err
        }
        logger.Info("Added cache_creation_tokens column to daily_stats")
    }

    // 检查第二个列
    err = s.db.QueryRow(`
        SELECT COUNT(*) FROM pragma_table_info('daily_stats')
        WHERE name='cache_read_tokens'
    `).Scan(&count)
    if err != nil {
        return err
    }

    if count == 0 {
        _, err := s.db.Exec(`
            ALTER TABLE daily_stats ADD COLUMN cache_read_tokens INTEGER DEFAULT 0;
        `)
        if err != nil {
            return err
        }
        logger.Info("Added cache_read_tokens column to daily_stats")
    }

    return nil
}

// migrateRequestStats 创建 request_stats 表
func (s *SQLiteStorage) migrateRequestStats() error {
    _, err := s.db.Exec(`
        CREATE TABLE IF NOT EXISTS request_stats (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            endpoint_name TEXT NOT NULL,
            request_id TEXT,
            timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
            date TEXT NOT NULL,
            input_tokens INTEGER DEFAULT 0,
            cache_creation_tokens INTEGER DEFAULT 0,
            cache_read_tokens INTEGER DEFAULT 0,
            output_tokens INTEGER DEFAULT 0,
            model TEXT,
            is_streaming BOOLEAN DEFAULT 0,
            success BOOLEAN DEFAULT 1,
            device_id TEXT DEFAULT 'default'
        );

        CREATE INDEX IF NOT EXISTS idx_request_stats_endpoint
        ON request_stats(endpoint_name);

        CREATE INDEX IF NOT EXISTS idx_request_stats_date
        ON request_stats(date);

        CREATE INDEX IF NOT EXISTS idx_request_stats_timestamp
        ON request_stats(timestamp DESC);

        CREATE INDEX IF NOT EXISTS idx_request_stats_composite
        ON request_stats(endpoint_name, date, device_id);
    `)

    if err != nil {
        return err
    }

    logger.Info("Created request_stats table with indexes")
    return nil
}
```

**在 `initSchema` 中调用 migration：**
```go
func (s *SQLiteStorage) initSchema() error {
    // ... 现有的表创建逻辑 ...

    // 执行 migrations
    if err := s.migrateSortOrder(); err != nil {
        return err
    }

    if err := s.migrateCacheTokens(); err != nil {
        return err
    }

    if err := s.migrateRequestStats(); err != nil {
        return err
    }

    return nil
}
```

### 6.2 更新 RecordDailyStat

**修改 SQL 语句包含 cache token 字段：**
```go
func (s *SQLiteStorage) RecordDailyStat(stat *DailyStat) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    _, err := s.db.Exec(`
        INSERT INTO daily_stats (
            endpoint_name, date, requests, errors,
            input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens,
            device_id
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(endpoint_name, date, device_id) DO UPDATE SET
            requests = requests + excluded.requests,
            errors = errors + excluded.errors,
            input_tokens = input_tokens + excluded.input_tokens,
            cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
            cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
            output_tokens = output_tokens + excluded.output_tokens
    `,
        stat.EndpointName, stat.Date, stat.Requests, stat.Errors,
        stat.InputTokens, stat.CacheCreationTokens, stat.CacheReadTokens, stat.OutputTokens,
        stat.DeviceID,
    )

    return err
}
```

### 6.3 新增 RecordRequestStat

```go
func (s *SQLiteStorage) RecordRequestStat(stat *RequestStat) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    _, err := s.db.Exec(`
        INSERT INTO request_stats (
            endpoint_name, request_id, timestamp, date,
            input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens,
            model, is_streaming, success, device_id
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `,
        stat.EndpointName, stat.RequestID, stat.Timestamp, stat.Date,
        stat.InputTokens, stat.CacheCreationTokens, stat.CacheReadTokens, stat.OutputTokens,
        stat.Model, stat.IsStreaming, stat.Success, stat.DeviceID,
    )

    return err
}
```

### 6.4 更新查询方法

**修改 `GetDailyStats` 包含 cache tokens：**
```go
func (s *SQLiteStorage) GetDailyStats(endpointName, startDate, endDate string) ([]DailyStat, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    query := `
        SELECT id, endpoint_name, date,
            SUM(requests), SUM(errors),
            SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)),
            SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens),
            device_id, created_at
        FROM daily_stats
        WHERE endpoint_name=? AND date>=? AND date<=?
        GROUP BY date
        ORDER BY date DESC
    `

    rows, err := s.db.Query(query, endpointName, startDate, endDate)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var stats []DailyStat
    for rows.Next() {
        var stat DailyStat
        if err := rows.Scan(
            &stat.ID, &stat.EndpointName, &stat.Date,
            &stat.Requests, &stat.Errors,
            &stat.InputTokens, &stat.CacheCreationTokens, &stat.CacheReadTokens, &stat.OutputTokens,
            &stat.DeviceID, &stat.CreatedAt,
        ); err != nil {
            return nil, err
        }
        stats = append(stats, stat)
    }

    return stats, rows.Err()
}
```

**修改 `GetTotalStats` 包含 cache tokens：**
```go
func (s *SQLiteStorage) GetTotalStats() (int, map[string]*EndpointStats, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    query := `
        SELECT endpoint_name,
            SUM(requests), SUM(errors),
            SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)),
            SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens)
        FROM daily_stats
        GROUP BY endpoint_name
    `

    rows, err := s.db.Query(query)
    if err != nil {
        return 0, nil, err
    }
    defer rows.Close()

    result := make(map[string]*EndpointStats)
    totalRequests := 0

    for rows.Next() {
        var endpointName string
        var requests, errors int
        var inputTokens, cacheCreationTokens, cacheReadTokens, outputTokens int64

        if err := rows.Scan(
            &endpointName, &requests, &errors,
            &inputTokens, &cacheCreationTokens, &cacheReadTokens, &outputTokens,
        ); err != nil {
            return 0, nil, err
        }

        result[endpointName] = &EndpointStats{
            Requests:            requests,
            Errors:              errors,
            InputTokens:         inputTokens,
            CacheCreationTokens: cacheCreationTokens,
            CacheReadTokens:     cacheReadTokens,
            OutputTokens:        outputTokens,
        }
        totalRequests += requests
    }

    return totalRequests, result, rows.Err()
}
```

**新增 `GetRequestStats` 和 `GetRequestStatsCount`：**
```go
func (s *SQLiteStorage) GetRequestStats(endpointName string, startDate, endDate string, limit, offset int) ([]RequestStat, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    query := `
        SELECT id, endpoint_name, request_id, timestamp, date,
            input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens,
            model, is_streaming, success, device_id
        FROM request_stats
        WHERE endpoint_name=? AND date>=? AND date<=?
        ORDER BY timestamp DESC
        LIMIT ? OFFSET ?
    `

    rows, err := s.db.Query(query, endpointName, startDate, endDate, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var stats []RequestStat
    for rows.Next() {
        var stat RequestStat
        if err := rows.Scan(
            &stat.ID, &stat.EndpointName, &stat.RequestID, &stat.Timestamp, &stat.Date,
            &stat.InputTokens, &stat.CacheCreationTokens, &stat.CacheReadTokens, &stat.OutputTokens,
            &stat.Model, &stat.IsStreaming, &stat.Success, &stat.DeviceID,
        ); err != nil {
            return nil, err
        }
        stats = append(stats, stat)
    }

    return stats, rows.Err()
}

func (s *SQLiteStorage) GetRequestStatsCount(endpointName string, startDate, endDate string) (int, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    var count int
    err := s.db.QueryRow(`
        SELECT COUNT(*) FROM request_stats
        WHERE endpoint_name=? AND date>=? AND date<=?
    `, endpointName, startDate, endDate).Scan(&count)

    return count, err
}
```

**新增自动清理方法：**
```go
func (s *SQLiteStorage) CleanupOldRequestStats(daysToKeep int) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    cutoffDate := time.Now().AddDate(0, 0, -daysToKeep).Format("2006-01-02")

    result, err := s.db.Exec(`
        DELETE FROM request_stats WHERE date < ?
    `, cutoffDate)

    if err != nil {
        return err
    }

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected > 0 {
        logger.Info("Cleaned up %d old request stats (before %s)", rowsAffected, cutoffDate)
    }

    return nil
}
```

### 6.5 更新 Stats Adapter (`internal/storage/stats_adapter.go`)

**修改 `RecordDailyStat`：**
```go
func (a *StatsStorageAdapter) RecordDailyStat(stat interface{}) error {
    v := reflect.ValueOf(stat)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }

    dailyStat := &DailyStat{
        EndpointName:        v.FieldByName("EndpointName").String(),
        Date:                v.FieldByName("Date").String(),
        Requests:            int(v.FieldByName("Requests").Int()),
        Errors:              int(v.FieldByName("Errors").Int()),
        InputTokens:         int(v.FieldByName("InputTokens").Int()),
        CacheCreationTokens: int(v.FieldByName("CacheCreationTokens").Int()),
        CacheReadTokens:     int(v.FieldByName("CacheReadTokens").Int()),
        OutputTokens:        int(v.FieldByName("OutputTokens").Int()),
        DeviceID:            v.FieldByName("DeviceID").String(),
    }

    return a.storage.RecordDailyStat(dailyStat)
}
```

**新增 `RecordRequestStat`：**
```go
func (a *StatsStorageAdapter) RecordRequestStat(stat interface{}) error {
    v := reflect.ValueOf(stat)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }

    requestStat := &RequestStat{
        EndpointName:        v.FieldByName("EndpointName").String(),
        RequestID:           v.FieldByName("RequestID").String(),
        Timestamp:           v.FieldByName("Timestamp").Interface().(time.Time),
        Date:                v.FieldByName("Date").String(),
        InputTokens:         int(v.FieldByName("InputTokens").Int()),
        CacheCreationTokens: int(v.FieldByName("CacheCreationTokens").Int()),
        CacheReadTokens:     int(v.FieldByName("CacheReadTokens").Int()),
        OutputTokens:        int(v.FieldByName("OutputTokens").Int()),
        Model:               v.FieldByName("Model").String(),
        IsStreaming:         v.FieldByName("IsStreaming").Bool(),
        Success:             v.FieldByName("Success").Bool(),
        DeviceID:            v.FieldByName("DeviceID").String(),
    }

    return a.storage.RecordRequestStat(requestStat)
}
```

---

## 7. Proxy 层集成

### 7.1 更新主请求处理器 (`internal/proxy/proxy.go`)

**修改流式响应处理（约 line 417-431）：**
```go
if resp.StatusCode == http.StatusOK && isStreaming {
    usage, outputText := p.handleStreamingResponse(w, resp, endpoint, trans,
        transformerName, thinkingEnabled, streamReq.Model, bodyBytes)

    // Fallback: 当 token 为 0 时估算
    if usage.TotalInputTokens() == 0 || usage.OutputTokens == 0 {
        totalInput := usage.TotalInputTokens()
        if totalInput == 0 {
            totalInput = p.estimateInputTokens(bodyBytes, endpoint.Name)
            usage.InputTokens = totalInput
        }
        if usage.OutputTokens == 0 {
            usage.OutputTokens = p.estimateOutputTokens(outputText, endpoint.Name)
        }
    }

    // 记录每日聚合统计
    p.stats.RecordTokens(endpoint.Name, usage)

    // 记录请求级别统计
    p.stats.RecordRequestStat(&RequestStatRecord{
        EndpointName:        endpoint.Name,
        Timestamp:           time.Now(),
        InputTokens:         usage.InputTokens,
        CacheCreationTokens: usage.CacheCreationInputTokens,
        CacheReadTokens:     usage.CacheReadInputTokens,
        OutputTokens:        usage.OutputTokens,
        Model:               streamReq.Model,
        IsStreaming:         true,
        Success:             true,
    })

    p.markRequestInactive(endpoint.Name)
    if p.onEndpointSuccess != nil {
        p.onEndpointSuccess(endpoint.Name)
    }
    logger.Debug("[%s] Request completed successfully (streaming)", endpoint.Name)
    return
}
```

**修改非流式响应处理（约 line 434-443）：**
```go
if resp.StatusCode == http.StatusOK {
    usage, err := p.handleNonStreamingResponse(w, resp, endpoint, trans)
    if err == nil {
        // Fallback: 当 token 为 0 时估算
        if usage.TotalInputTokens() == 0 || usage.OutputTokens == 0 {
            totalInput := usage.TotalInputTokens()
            if totalInput == 0 {
                totalInput = p.estimateInputTokens(bodyBytes, endpoint.Name)
                usage.InputTokens = totalInput
            }
            // 注意：非流式响应没有 outputText，无法估算 output tokens
        }

        // 记录每日聚合统计
        p.stats.RecordTokens(endpoint.Name, usage)

        // 记录请求级别统计
        // 从请求体中提取模型名称
        var reqBody map[string]interface{}
        json.Unmarshal(bodyBytes, &reqBody)
        modelName, _ := reqBody["model"].(string)

        p.stats.RecordRequestStat(&RequestStatRecord{
            EndpointName:        endpoint.Name,
            Timestamp:           time.Now(),
            InputTokens:         usage.InputTokens,
            CacheCreationTokens: usage.CacheCreationInputTokens,
            CacheReadTokens:     usage.CacheReadInputTokens,
            OutputTokens:        usage.OutputTokens,
            Model:               modelName,
            IsStreaming:         false,
            Success:             true,
        })

        p.markRequestInactive(endpoint.Name)
        if p.onEndpointSuccess != nil {
            p.onEndpointSuccess(endpoint.Name)
        }
        logger.Debug("[%s] Request completed successfully", endpoint.Name)
        return
    }
}
```

### 7.2 更新 Token 估算方法 (`internal/proxy/utils.go`)

**拆分估算方法以便单独调用：**
```go
func (p *Proxy) estimateInputTokens(bodyBytes []byte, endpointName string) int {
    var req tokencount.CountTokensRequest
    if json.Unmarshal(bodyBytes, &req) == nil {
        tokens := tokencount.EstimateInputTokens(&req)
        logger.Debug("[%s] Estimated input tokens: %d", endpointName, tokens)
        return tokens
    }
    return 0
}

func (p *Proxy) estimateOutputTokens(outputText string, endpointName string) int {
    if outputText != "" {
        tokens := tokencount.EstimateOutputTokens(outputText)
        logger.Debug("[%s] Estimated output tokens: %d", endpointName, tokens)
        return tokens
    }
    return 0
}

// 保留原有方法作为兼容
func (p *Proxy) estimateTokens(bodyBytes []byte, outputText string,
    inputTokens, outputTokens int, endpointName string) (int, int) {

    if inputTokens == 0 {
        inputTokens = p.estimateInputTokens(bodyBytes, endpointName)
    }
    if outputTokens == 0 {
        outputTokens = p.estimateOutputTokens(outputText, endpointName)
    }
    return inputTokens, outputTokens
}
```

---

## 8. Service 层修改

### 8.1 更新 StatsService (`internal/service/stats.go`)

**修改统计汇总方法包含 cache tokens：**
```go
func (s *StatsService) getPeriodStats(period, startDate, endDate string) string {
    var stats map[string]*proxy.DailyStats
    if startDate == endDate {
        stats = s.proxy.GetStats().GetDailyStats(startDate)
    } else {
        stats = s.proxy.GetStats().GetPeriodStats(startDate, endDate)
    }

    var totalRequests, totalErrors int
    var totalInputTokens, totalCacheCreationTokens, totalCacheReadTokens, totalOutputTokens int

    for _, st := range stats {
        totalRequests += st.Requests
        totalErrors += st.Errors
        totalInputTokens += st.InputTokens
        totalCacheCreationTokens += st.CacheCreationTokens
        totalCacheReadTokens += st.CacheReadTokens
        totalOutputTokens += st.OutputTokens
    }

    activeEndpoints, totalEndpoints := s.countEndpoints()

    result := map[string]interface{}{
        "period":                   period,
        "totalRequests":            totalRequests,
        "totalErrors":              totalErrors,
        "totalSuccess":             totalRequests - totalErrors,
        "totalInputTokens":         totalInputTokens,
        "totalCacheCreationTokens": totalCacheCreationTokens,
        "totalCacheReadTokens":     totalCacheReadTokens,
        "totalOutputTokens":        totalOutputTokens,
        "activeEndpoints":          activeEndpoints,
        "totalEndpoints":           totalEndpoints,
        "endpoints":                stats,
    }

    if startDate == endDate {
        result["date"] = startDate
    } else {
        result["startDate"] = startDate
        result["endDate"] = endDate
    }

    data, _ := json.Marshal(result)
    return string(data)
}
```

**新增请求级别统计查询方法：**
```go
// GetRequestStats 获取请求级别的统计数据（分页）
func (s *StatsService) GetRequestStats(endpointName string, startDate, endDate string, page, pageSize int) string {
    offset := (page - 1) * pageSize

    requestStats, err := s.proxy.GetStats().GetStorage().GetRequestStats(
        endpointName, startDate, endDate, pageSize, offset,
    )
    if err != nil {
        return `{"error": "Failed to get request stats"}`
    }

    total, err := s.proxy.GetStats().GetStorage().GetRequestStatsCount(
        endpointName, startDate, endDate,
    )
    if err != nil {
        total = 0
    }

    result := map[string]interface{}{
        "requests":  requestStats,
        "total":     total,
        "page":      page,
        "pageSize":  pageSize,
        "totalPages": (total + pageSize - 1) / pageSize,
    }

    data, _ := json.Marshal(result)
    return string(data)
}

// CleanupRequestStats 清理旧的请求级别统计
func (s *StatsService) CleanupRequestStats(daysToKeep int) error {
    return s.proxy.GetStats().GetStorage().(*storage.SQLiteStorage).CleanupOldRequestStats(daysToKeep)
}
```

### 8.2 更新 ArchiveService (`internal/service/archive.go`)

**修改归档数据查询包含 cache tokens：**
```go
// GetArchiveData 中需要更新数据结构以包含 cache token 字段
// 确保从数据库读取时包含 cache_creation_tokens 和 cache_read_tokens
```

---

## 9. 定时任务：自动清理旧请求统计

### 9.1 添加清理任务 (`internal/proxy/proxy.go` 或 `cmd/desktop/main.go`)

```go
// StartRequestStatsCleanup 启动定时清理任务
func (p *Proxy) StartRequestStatsCleanup(daysToKeep int) {
    ticker := time.NewTicker(24 * time.Hour)  // 每天执行一次

    go func() {
        for range ticker.C {
            if err := p.stats.GetStorage().CleanupOldRequestStats(daysToKeep); err != nil {
                logger.Error("Failed to cleanup old request stats: %v", err)
            }
        }
    }()
}
```

**在应用启动时调用：**
```go
// 启动定时清理，保留 90 天的请求统计
proxy.StartRequestStatsCleanup(90)
```

---

## 10. 实施步骤

### 步骤 1: 数据库层修改（基础）
1. ✅ 修改 `internal/storage/sqlite.go`
   - 添加 `migrateCacheTokens()` 函数
   - 添加 `migrateRequestStats()` 函数
   - 在 `initSchema()` 中调用 migrations
   - 更新 `RecordDailyStat()` SQL
   - 新增 `RecordRequestStat()` 方法
   - 更新 `GetDailyStats()` 和 `GetTotalStats()` 查询
   - 新增 `GetRequestStats()` 和 `GetRequestStatsCount()` 方法
   - 新增 `CleanupOldRequestStats()` 方法

2. ✅ 修改 `internal/storage/interface.go`
   - 更新 `DailyStat` 结构体
   - 新增 `RequestStat` 结构体
   - 更新 `EndpointStats` 结构体

### 步骤 2: Transformer 层修改（核心）
1. ✅ 修改 `internal/transformer/types.go`
   - 新增 `TokenUsageDetail` 结构体
   - 新增 `ExtractTokenUsageDetail()` 函数
   - 保留 `ExtractInputTokens()` 向后兼容

### 步骤 3: Proxy 层修改（提取和记录）
1. ✅ 修改 `internal/proxy/response.go`
   - 更新 `extractTokenUsage()` 返回 `TokenUsageDetail`
   - 更新 `handleNonStreamingResponse()` 签名

2. ✅ 修改 `internal/proxy/streaming.go`
   - 更新 `extractTokensFromEvent()` 处理 `TokenUsageDetail`
   - 更新 `handleStreamingResponse()` 签名

3. ✅ 修改 `internal/proxy/stats.go`
   - 更新 `StatRecord` 结构体
   - 新增 `RequestStatRecord` 结构体
   - 更新 `RecordTokens()` 方法
   - 新增 `RecordRequestStat()` 方法
   - 更新 `DailyStats` 结构体
   - 更新 `StatsStorage` 接口

4. ✅ 修改 `internal/proxy/proxy.go`
   - 更新流式响应处理逻辑（line ~417-431）
   - 更新非流式响应处理逻辑（line ~434-443）

5. ✅ 修改 `internal/proxy/utils.go`
   - 拆分 `estimateInputTokens()` 和 `estimateOutputTokens()`

### 步骤 4: Storage Adapter 修改
1. ✅ 修改 `internal/storage/stats_adapter.go`
   - 更新 `RecordDailyStat()` 处理 cache tokens
   - 新增 `RecordRequestStat()` 方法

### 步骤 5: Service 层修改
1. ✅ 修改 `internal/service/stats.go`
   - 更新 `getPeriodStats()` 包含 cache tokens
   - 新增 `GetRequestStats()` 方法
   - 新增 `CleanupRequestStats()` 方法

2. ✅ 修改 `internal/service/archive.go`
   - 确保归档数据包含 cache tokens

### 步骤 6: 添加定时清理任务
1. ✅ 在应用启动时启动请求统计清理任务

### 步骤 7: UI 修改（可选，后续）
1. 桌面版 UI (`cmd/desktop/frontend`)
   - 更新统计展示组件显示 cache tokens
   - 新增请求历史查看页面

2. 服务器版 UI（如果有）
   - 类似桌面版的更新

---

## 11. 测试计划

### 11.1 单元测试
- [ ] 测试 `ExtractTokenUsageDetail()` 正确提取所有 token 字段
- [ ] 测试 `RecordDailyStat()` 正确累加 cache tokens
- [ ] 测试 `RecordRequestStat()` 正确保存请求统计
- [ ] 测试 migration 在现有数据库上正常执行

### 11.2 集成测试
- [ ] 发送包含 cache tokens 的 Claude API 响应，验证统计准确性
- [ ] 发送流式和非流式请求，验证两种模式都正确记录
- [ ] 验证每日聚合统计和请求级别统计数据一致性
- [ ] 测试请求统计的自动清理功能

### 11.3 手动测试
- [ ] 启动应用，验证数据库 migration 成功
- [ ] 发送多个请求，检查 `daily_stats` 表的 cache token 字段
- [ ] 查询 `request_stats` 表，验证每次请求都有记录
- [ ] 在 UI 中查看统计数据，确认显示正确

---

## 12. 向后兼容性保证

### 12.1 数据库兼容性
- ✅ 新列使用 `DEFAULT 0`，历史记录自动填充 0
- ✅ 查询时使用 `COALESCE(cache_creation_tokens, 0)` 处理 NULL
- ✅ 现有查询不受影响（仍然能正常读取 input_tokens 和 output_tokens）

### 12.2 代码兼容性
- ✅ 保留 `ExtractInputTokens()` 函数，返回总和
- ✅ 旧的统计查询接口继续工作
- ✅ 新增的字段在旧版本中被忽略

### 12.3 数据迁移
- ✅ 无需手动数据迁移
- ✅ 应用启动时自动执行 migration
- ✅ Migration 具有幂等性，可重复执行

---

## 13. 性能考虑

### 13.1 请求统计表大小预估
假设平均每天 1000 个请求：
- 每条记录约 200 字节
- 30 天：1000 × 30 × 200B ≈ 6MB
- 90 天：1000 × 90 × 200B ≈ 18MB

建议保留 **90 天**的请求统计，定期自动清理。

### 13.2 查询优化
- ✅ 在 `endpoint_name`、`date`、`timestamp` 上创建索引
- ✅ 使用复合索引 `(endpoint_name, date, device_id)` 优化常见查询
- ✅ 请求统计查询使用分页，避免一次加载大量数据

### 13.3 写入性能
- ✅ 每日聚合统计使用 `ON CONFLICT` 进行 upsert，避免重复记录
- ✅ 请求统计直接 INSERT，无需查询
- ✅ 使用事务批量写入（如果需要）

---

## 14. 未来扩展

### 14.1 可能的增强功能
- 支持按用户/会话分组统计
- 支持自定义时间范围查询
- 导出请求统计为 CSV/JSON
- 统计数据可视化图表

### 14.2 潜在问题
- 请求统计表可能快速增长，需要监控磁盘空间
- 高并发情况下写入性能可能成为瓶颈
- 跨设备同步时请求统计数据量较大

---

## 15. 总结

本改进方案实现了：
1. ✅ **完整的 cache token 统计**：区分标准输入、缓存创建、缓存读取
2. ✅ **双层统计架构**：每日聚合（长期保存）+ 请求级别（短期保留）
3. ✅ **向后兼容**：不影响现有数据和功能
4. ✅ **准确记录**：每次请求都记录所属端点和详细 token 消耗
5. ✅ **性能优化**：合理的索引和自动清理机制

通过本方案，用户可以：
- 清楚看到每天消耗了多少 cache token
- 分析哪些请求使用了缓存
- 追踪每个端点的详细使用情况
- 查看历史请求的 token 消耗记录
