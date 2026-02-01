# Token 统计显示 0/0 问题修复

## 问题描述

在"最近请求"面板中，Token 使用量始终显示为 `0/0`（输入Token / 输出Token），即使请求成功完成。

## 根本原因

在流式响应转换过程中，三个主要的 API 格式转换器（OpenAI、OpenAI2、Gemini）在将响应转换回 Claude 格式时，**硬编码了 Token 使用量为 0**，而没有从原始 API 响应中提取真实的 Token 数据。

### 受影响的文件

1. **`internal/transformer/convert/claude_openai.go`**
   - 第 472 行：`message_start` 事件中硬编码 `"input_tokens": 0, "output_tokens": 0`
   - 第 541 行：`message_delta` 事件中硬编码 `"output_tokens": 0`

2. **`internal/transformer/convert/claude_openai2.go`**
   - 第 441 行：`response.created` 事件中硬编码 `"input_tokens": 0, "output_tokens": 0`
   - 第 501 行：`response.completed` 事件中硬编码 `"output_tokens": 0`

3. **`internal/transformer/convert/claude_gemini.go`**
   - 第 381 行：`message_start` 事件中硬编码 `"input_tokens": 0, "output_tokens": 0`
   - 第 440 行：`message_delta` 事件中硬编码 `"output_tokens": 0`

## 修复方案

### OpenAI 转换器 (`claude_openai.go`)

**修改前：**
```go
"usage": map[string]interface{}{"input_tokens": 0, "output_tokens": 0}
```

**修改后：**
```go
// 从 OpenAI chunk 中提取 input_tokens（如果有）
inputTokens := ctx.InputTokens // 使用预估的 input tokens 作为后备
if chunk.Usage != nil && chunk.Usage.PromptTokens > 0 {
    inputTokens = chunk.Usage.PromptTokens
}
"usage": map[string]interface{}{"input_tokens": inputTokens, "output_tokens": 0}
```

```go
// 从 OpenAI chunk 中提取 output_tokens（如果有）
outputTokens := 0
if chunk.Usage != nil && chunk.Usage.CompletionTokens > 0 {
    outputTokens = chunk.Usage.CompletionTokens
}
"usage": map[string]interface{}{"output_tokens": outputTokens}
```

### OpenAI2 转换器 (`claude_openai2.go`)

**修改前：**
```go
"usage": map[string]interface{}{"input_tokens": 0, "output_tokens": 0}
```

**修改后：**
```go
// 从 OpenAI2 response 中提取 input_tokens（如果有）
inputTokens := ctx.InputTokens
if evt.Response != nil && evt.Response.Usage.InputTokens > 0 {
    inputTokens = evt.Response.Usage.InputTokens
}
"usage": map[string]interface{}{"input_tokens": inputTokens, "output_tokens": 0}
```

```go
// 从 OpenAI2 response 中提取 output_tokens（如果有）
outputTokens := 0
if evt.Response != nil && evt.Response.Usage.OutputTokens > 0 {
    outputTokens = evt.Response.Usage.OutputTokens
}
"usage": map[string]interface{}{"output_tokens": outputTokens}
```

### Gemini 转换器 (`claude_gemini.go`)

**修改前：**
```go
"usage": map[string]interface{}{"input_tokens": 0, "output_tokens": 0}
```

**修改后：**
```go
// 从 Gemini response 中提取 input_tokens（如果有）
inputTokens := ctx.InputTokens
if resp.UsageMetadata != nil && resp.UsageMetadata.PromptTokenCount > 0 {
    inputTokens = resp.UsageMetadata.PromptTokenCount
}
"usage": map[string]interface{}{"input_tokens": inputTokens, "output_tokens": 0}
```

```go
// 从 Gemini response 中提取 output_tokens（如果有）
outputTokens := 0
if resp.UsageMetadata != nil && resp.UsageMetadata.CandidatesTokenCount > 0 {
    outputTokens = resp.UsageMetadata.CandidatesTokenCount
}
"usage": map[string]interface{}{"output_tokens": outputTokens}
```

## 技术细节

### Token 数据流

1. **原始 API 响应** → 包含 Token 使用信息
   - OpenAI: `chunk.Usage.PromptTokens` / `chunk.Usage.CompletionTokens`
   - OpenAI2: `evt.Response.Usage.InputTokens` / `evt.Response.Usage.OutputTokens`
   - Gemini: `resp.UsageMetadata.PromptTokenCount` / `resp.UsageMetadata.CandidatesTokenCount`

2. **转换器** → 将 Token 信息转换为 Claude 格式
   - `message_start` 事件：包含 `input_tokens`
   - `message_delta` 事件：包含 `output_tokens`

3. **代理提取** → `extractTokensFromEvent()` 从转换后的事件中提取
   - 查找 `type: "message_start"` 和 `type: "message_delta"` 事件
   - 提取 `usage.input_tokens` 和 `usage.output_tokens`

4. **数据库存储** → `RecordRequestStat()` 保存到 `request_stats` 表

5. **前端显示** → 从数据库读取并显示在"最近请求"面板

### 后备机制

修复方案包含后备机制：
- 如果 API 响应中没有 Token 信息，使用 `ctx.InputTokens`（预估值）
- 预估值在 `streaming.go:76-78` 中计算：
  ```go
  if bodyBytes != nil {
      streamCtx.InputTokens = p.estimateInputTokens(bodyBytes)
  }
  ```

## 测试验证

1. 编译通过：`go build` 成功
2. 需要运行时测试：
   - 使用 OpenAI 格式端点发送请求
   - 检查"最近请求"面板是否显示正确的 Token 数量
   - 验证数据库 `request_stats` 表中的数据

## 影响范围

- **修复的转换器**：`cc_openai`, `cc_openai2`, `cc_gemini`
- **不受影响**：`cc_claude`（直接使用 Claude API，Token 信息已正确传递）
- **不受影响**：Codex 转换器（`cx_chat_*`, `cx_resp_*`）

## 注意事项

1. **流式响应的 Token 时机**：
   - OpenAI: Token 信息通常在最后一个 chunk 中（带 `finish_reason`）
   - OpenAI2: Token 信息在 `response.completed` 事件中
   - Gemini: Token 信息在带 `finishReason` 的 chunk 中

2. **`[DONE]` 事件**：
   - 这个事件通常不包含 Token 信息
   - 真实的 Token 应该在之前的 chunk 中已经发送
   - 保持为 0 是正确的行为

## 后续建议

1. 添加日志记录，跟踪 Token 提取过程
2. 添加单元测试，验证 Token 提取逻辑
3. 考虑在前端显示 Token 估算值（当 API 不返回时）
