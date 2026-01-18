package storage

import "time"

type Endpoint struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ClientType  string    `json:"clientType"` // 客户端类型: claude, gemini, codex
	APIUrl      string    `json:"apiUrl"`
	APIKey      string    `json:"apiKey"`
	Status      string    `json:"status"`     // 端点状态: available, unavailable, disabled
	Enabled     bool      `json:"enabled"`    // 向后兼容字段
	Transformer string    `json:"transformer"`
	Model       string    `json:"model"`
	Remark      string    `json:"remark"`
	Tags        string    `json:"tags"` // 逗号分隔的标签
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	// 智能路由相关字段
	ModelPatterns      string  `json:"modelPatterns"`      // 模型匹配模式，逗号分隔
	CostPerInputToken  float64 `json:"costPerInputToken"`  // 每百万输入 Token 成本
	CostPerOutputToken float64 `json:"costPerOutputToken"` // 每百万输出 Token 成本
	QuotaLimit         int64   `json:"quotaLimit"`         // Token 配额限制
	QuotaResetCycle    string  `json:"quotaResetCycle"`    // 配额重置周期
	Priority           int     `json:"priority"`           // 优先级
}

type DailyStat struct {
	ID                  int64
	EndpointName        string
	ClientType          string // 客户端类型: claude, gemini, codex
	Date                string
	Requests            int
	Errors              int
	InputTokens         int
	CacheCreationTokens int // 新增：缓存创建 token
	CacheReadTokens     int // 新增：缓存读取 token
	OutputTokens        int
	DeviceID            string
	CreatedAt           time.Time
}

type EndpointStats struct {
	Requests            int
	Errors              int
	InputTokens         int64
	CacheCreationTokens int64 // 新增：缓存创建 token
	CacheReadTokens     int64 // 新增：缓存读取 token
	OutputTokens        int64
}

// RequestStat 请求级别统计（新增）
type RequestStat struct {
	ID                  int64     `json:"id"`
	EndpointName        string    `json:"endpointName"`
	ClientType          string    `json:"clientType"` // 客户端类型: claude, gemini, codex
	ClientIP            string    `json:"clientIp"`   // 客户端 IP 地址
	RequestID           string    `json:"requestId"`
	Timestamp           time.Time `json:"timestamp"`
	Date                string    `json:"date"`
	InputTokens         int       `json:"inputTokens"`
	CacheCreationTokens int       `json:"cacheCreationTokens"`
	CacheReadTokens     int       `json:"cacheReadTokens"`
	OutputTokens        int       `json:"outputTokens"`
	Model               string    `json:"model"`
	IsStreaming         bool      `json:"isStreaming"`
	Success             bool      `json:"success"`
	DeviceID            string    `json:"deviceId"`
	DurationMs          int64     `json:"durationMs"` // 请求时长（毫秒）
	ErrorMessage        string    `json:"errorMessage"` // 错误消息（失败时记录）
}

// ClientStats 连接客户端统计信息
type ClientStats struct {
	ClientIP            string    `json:"clientIp"`
	LastSeen            time.Time `json:"lastSeen"`
	RequestCount        int       `json:"requestCount"`
	InputTokens         int       `json:"inputTokens"`
	CacheCreationTokens int       `json:"cacheCreationTokens"`
	CacheReadTokens     int       `json:"cacheReadTokens"`
	OutputTokens        int       `json:"outputTokens"`
	EndpointsUsed       []string  `json:"endpointsUsed"`
}

// HealthHistoryRecord 端点健康历史记录
type HealthHistoryRecord struct {
	ID           int64     `json:"id"`
	EndpointName string    `json:"endpointName"`
	ClientType   string    `json:"clientType"`
	Status       string    `json:"status"` // healthy, warning, error, unknown
	LatencyMs    float64   `json:"latencyMs"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	DeviceID     string    `json:"deviceId"`
}

// EndpointQuota 端点配额跟踪记录
type EndpointQuota struct {
	ID           int64     `json:"id"`
	EndpointName string    `json:"endpointName"`
	ClientType   string    `json:"clientType"`
	PeriodStart  time.Time `json:"periodStart"`  // 当前周期开始时间
	PeriodEnd    time.Time `json:"periodEnd"`    // 当前周期结束时间
	TokensUsed   int64     `json:"tokensUsed"`   // 已使用 Token
	QuotaLimit   int64     `json:"quotaLimit"`   // 配额限制
	LastUpdated  time.Time `json:"lastUpdated"`
}

type Storage interface {
	// Endpoints
	GetEndpoints() ([]Endpoint, error)
	GetEndpointsByClient(clientType string) ([]Endpoint, error) // 按客户端类型获取端点
	SaveEndpoint(ep *Endpoint) error
	UpdateEndpoint(ep *Endpoint) error
	DeleteEndpoint(name string, clientType string) error // 按名称和客户端类型删除

	// Stats
	RecordDailyStat(stat *DailyStat) error
	GetDailyStats(endpointName, clientType, startDate, endDate string) ([]DailyStat, error) // 添加 clientType 参数
	GetAllStats() (map[string][]DailyStat, error)
	GetTotalStats() (int, map[string]*EndpointStats, error)
	GetTotalStatsByClient(clientType string) (int, map[string]*EndpointStats, error) // 按客户端类型获取统计
	GetEndpointTotalStats(endpointName string, clientType string) (*EndpointStats, error)

	// Request Stats（新增）
	RecordRequestStat(stat *RequestStat) error
	GetRequestStats(endpointName string, clientType string, startDate, endDate string, limit, offset int) ([]RequestStat, error)
	GetRequestStatsCount(endpointName string, clientType string, startDate, endDate string) (int, error)
	CleanupOldRequestStats(daysToKeep int) error
	GetConnectedClients(hoursAgo int) ([]ClientStats, error)

	// Health History（健康历史）
	RecordHealthHistory(record *HealthHistoryRecord) error
	GetHealthHistory(endpointName, clientType string, startTime, endTime time.Time, limit int) ([]HealthHistoryRecord, error)
	CleanupOldHealthHistory(daysToKeep int) error
	GetAllEndpointTags() ([]string, error)

	// Quota（配额跟踪）
	GetEndpointQuota(endpointName, clientType string) (*EndpointQuota, error)
	UpdateEndpointQuota(quota *EndpointQuota) error
	ResetExpiredQuotas() error

	// Config
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error

	// Close
	Close() error
}
