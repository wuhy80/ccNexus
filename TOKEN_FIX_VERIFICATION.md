# Token 统计修复必要性验证

## 问题确认

用户报告："最近请求"右边下面一行始终显示 `0/0`，即使请求成功。

## 数据流追踪

### 1. 请求阶段 ✅

**文件**: `internal/transformer/convert/claude_openai.go:142-144`

```go
// Enable usage tracking for streaming
if req.Stream {
    openaiReq.StreamOptions = &transformer.StreamOptions{IncludeUsage: true}
}
```

**结论**: 代码已正确设置 `stream_options.include_usage = true`，OpenAI 会返回 usage 信息。

**参考**: [OpenAI 官方公告](https://community.openai.com/t/usage-stats-now-available-when-using-streaming-with-the-chat-completions-api-or-completions-api/738156) - "Usage stats now available when using streaming"

---

### 2. OpenAI 响应阶段 ✅

**OpenAI 流式响应结构** (`internal/transformer/types.go:75-96`):

```go
type OpenAIStreamChunk struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []struct {
        Index int `json:"index"`
        Delta struct {
            Role    string `json:"role,omitempty"`
            Content string `json:"content,omitempty"`
            // ...
        } `json:"delta"`
        FinishReason *string `json:"finish_reason"`
    } `json:"choices"`
    Usage *struct {
        PromptTokens     int `json:"prompt_tokens"`      // ← 输入 Token
        CompletionTokens int `json:"completion_tokens"`  // ← 输出 Token
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage,omitempty"`  // ← 在最后一个 chunk 中返回
}
```

**结论**: OpenAI 在带 `finish_reason` 的最后一个 chunk 中返回 `usage` 对象。

---

### 3. 转换器阶段 ❌ **问题所在**

**文件**: `internal/transformer/convert/claude_openai.go:428-547`

#### 问题代码 1: message_start 事件（第 467-474 行）

```go
result = append(result, buildClaudeEvent("message_start", map[string]interface{}{
    "message": map[string]interface{}{
        "id": chunk.ID, "type": "message", "role": "assistant",
        "content": []interface{}{}, "model": ctx.ModelName,
        "stop_reason": nil, "stop_sequence": nil,
        "usage": map[string]interface{}{
            "input_tokens": 0,   // ← 硬编码为 0！
            "output_tokens": 0   // ← 硬编码为 0！
        },
    },
})...)
```

**问题**: 忽略了 `chunk.Usage.PromptTokens`，硬编码为 0。

#### 问题代码 2: message_delta 事件（第 539-542 行）

```go
result = append(result, buildClaudeEvent("message_delta", map[string]interface{}{
    "delta": map[string]interface{}{"stop_reason": stopReason, "stop_sequence": nil},
    "usage": map[string]interface{}{
        "output_tokens": 0  // ← 硬编码为 0！
    },
})...)
```

**问题**: 忽略了 `chunk.Usage.CompletionTokens`，硬编码为 0。

---

### 4. Token 提取阶段 ❌ **受影响**

**文件**: `internal/proxy/streaming.go:254-287`

```go
func (p *Proxy) extractTokensFromEvent(eventData []byte, usage *transformer.TokenUsageDetail) {
    // ...
    eventType, _ := event["type"].(string)
    if eventType == "message_start" {
        if message, ok := event["message"].(map[string]interface{}); ok {
            if usageMap, ok := message["usage"].(map[string]interface{}); ok {
                detail := transformer.ExtractTokenUsageDetail(usageMap)
                usage.InputTokens = detail.InputTokens  // ← 提取到的是 0
                // ...
            }
        }
    } else if eventType == "message_delta" {
        if usageMap, ok := event["usage"].(map[string]interface{}); ok {
            if output, ok := usageMap["output_tokens"].(float64); ok {
                usage.OutputTokens = int(output)  // ← 提取到的是 0
            }
        }
    }
}
```

**结论**: 这个函数工作正常，但它提取的是**转换后的 Claude 格式事件**。如果转换器输出 0，这里就提取到 0。

---

### 5. 数据库存储阶段 ❌ **受影响**

**文件**: `internal/proxy/proxy.go:1074-1087`

```go
p.stats.RecordRequestStat(&RequestStatRecord{
    EndpointName:        endpoint.Name,
    ClientType:          string(clientType),
    ClientIP:            clientIP,
    Timestamp:           time.Now(),
    InputTokens:         usage.InputTokens,   // ← 存储的是 0
    CacheCreationTokens: usage.CacheCreationInputTokens,
    CacheReadTokens:     usage.CacheReadInputTokens,
    OutputTokens:        usage.OutputTokens,  // ← 存储的是 0
    Model:               streamReq.Model,
    IsStreaming:         true,
    Success:             true,
    DurationMs:          durationMs,
})
```

**结论**: 数据库中存储的就是 0。

---

### 6. 前端显示阶段 ❌ **受影响**

**文件**: `cmd/desktop/frontend/src/modules/monitor.js:343-344`

```javascript
inputTokens: req.inputTokens + req.cacheCreationTokens + req.cacheReadTokens,
outputTokens: req.outputTokens,
```

**结论**: 前端显示的是数据库中的值，即 0/0。

---

## 修复验证

### 修复前的数据流

```
OpenAI API 返回:
  chunk.Usage.PromptTokens = 1234
  chunk.Usage.CompletionTokens = 567

↓ 转换器 (claude_openai.go)
  硬编码: input_tokens = 0, output_tokens = 0  ← 问题！

↓ extractTokensFromEvent
  提取: InputTokens = 0, OutputTokens = 0

↓ 数据库
  存储: input_tokens = 0, output_tokens = 0

↓ 前端
  显示: 0 / 0
```

### 修复后的数据流

```
OpenAI API 返回:
  chunk.Usage.PromptTokens = 1234
  chunk.Usage.CompletionTokens = 567

↓ 转换器 (claude_openai.go) - 已修复
  从 chunk.Usage 提取: input_tokens = 1234, output_tokens = 567  ← 修复！

↓ extractTokensFromEvent
  提取: InputTokens = 1234, OutputTokens = 567

↓ 数据库
  存储: input_tokens = 1234, output_tokens = 567

↓ 前端
  显示: 1234 / 567  ← 正确！
```

---

## 其他转换器验证

### OpenAI2 转换器

**文件**: `internal/transformer/convert/claude_openai2.go`

**问题**: 同样硬编码为 0（第 441 和 501 行）

**OpenAI2 响应结构** (`types.go:431-441`):
```go
type OpenAI2Response struct {
    // ...
    Usage struct {
        InputTokens  int `json:"input_tokens"`   // ← 有数据
        OutputTokens int `json:"output_tokens"`  // ← 有数据
        TotalTokens  int `json:"total_tokens"`
    } `json:"usage"`
}
```

**结论**: 需要修复。

---

### Gemini 转换器

**文件**: `internal/transformer/convert/claude_gemini.go`

**问题**: 同样硬编码为 0（第 381 和 440 行）

**Gemini 响应结构** (`types.go:353-357`):
```go
UsageMetadata *struct {
    PromptTokenCount     int `json:"promptTokenCount"`      // ← 有数据
    CandidatesTokenCount int `json:"candidatesTokenCount"`  // ← 有数据
    TotalTokenCount      int `json:"totalTokenCount"`
} `json:"usageMetadata,omitempty"`
```

**结论**: 需要修复。

---

## 最终结论

### ✅ 修改是必要的

**原因**:

1. **OpenAI/OpenAI2/Gemini API 都返回 usage 信息**
   - OpenAI: 通过 `stream_options.include_usage = true` 启用
   - OpenAI2: 在 `response.completed` 事件中
   - Gemini: 在 `usageMetadata` 字段中

2. **转换器硬编码为 0，丢弃了真实数据**
   - 这是 Bug，不是设计意图
   - 导致整个统计链路失效

3. **用户看到的 0/0 是真实问题**
   - 不是前端问题
   - 不是数据库问题
   - 是转换器问题

4. **修复简单且安全**
   - 只需从原始响应中提取真实值
   - 保留后备机制（使用预估值）
   - 不影响其他功能

### 📊 影响范围

- **受影响的端点类型**: 使用 `cc_openai`, `cc_openai2`, `cc_gemini` 转换器的端点
- **不受影响**: `cc_claude` 转换器（直接使用 Claude API）
- **不受影响**: Codex 转换器（不同的实现）

### 🎯 修复效果

修复后，用户将看到：
- ✅ 正确的输入 Token 数量
- ✅ 正确的输出 Token 数量
- ✅ 准确的成本估算
- ✅ 有意义的使用统计

---

## 参考资料

1. [OpenAI - Usage stats now available when using streaming](https://community.openai.com/t/usage-stats-now-available-when-using-streaming-with-the-chat-completions-api-or-completions-api/738156)
2. [Stack Overflow - How to get token usage in streaming mode](https://stackoverflow.com/questions/75824798/how-to-get-token-usage-for-each-openai-chatcompletion-api-call-in-streaming-mode)
3. [Medium - Calculate OpenAI usage for Chat Completion API stream](https://medium.com/@votanlean/calculate-openai-usage-for-chat-completion-api-stream-in-nodejs-03eb9172d407)
