package interaction

import "time"

// Record 表示一次完整的请求/响应交互记录
type Record struct {
	RequestID string       `json:"requestId"`
	Timestamp time.Time    `json:"timestamp"`
	Endpoint  EndpointInfo `json:"endpoint"`
	Client    ClientInfo   `json:"client"`
	Request   RequestData  `json:"request"`
	Response  ResponseData `json:"response"`
	Stats     StatsData    `json:"stats"`
}

// EndpointInfo 端点信息
type EndpointInfo struct {
	Name        string `json:"name"`
	Transformer string `json:"transformer"`
}

// ClientInfo 客户端信息
type ClientInfo struct {
	Type   string `json:"type"`   // claude, gemini, codex
	Format string `json:"format"` // claude, openai, gemini
	IP     string `json:"ip"`
}

// RequestData 请求数据
type RequestData struct {
	Path        string      `json:"path"`        // /v1/messages, /v1/chat/completions, etc.
	Model       string      `json:"model"`       // 模型名称
	Raw         interface{} `json:"raw"`         // 原始请求 JSON
	Transformed interface{} `json:"transformed"` // 转换后请求 JSON
}

// ResponseData 响应数据
type ResponseData struct {
	Status      int         `json:"status"`      // HTTP 状态码
	Raw         interface{} `json:"raw"`         // 原始响应或 SSE 事件数组
	Transformed interface{} `json:"transformed"` // 转换后响应
}

// StatsData 统计数据
type StatsData struct {
	DurationMs          int64  `json:"durationMs"`
	IsStreaming         bool   `json:"isStreaming"`
	InputTokens         int    `json:"inputTokens"`
	CacheCreationTokens int    `json:"cacheCreationTokens"`
	CacheReadTokens     int    `json:"cacheReadTokens"`
	OutputTokens        int    `json:"outputTokens"`
	Success             bool   `json:"success"`
	ErrorMessage        string `json:"errorMessage,omitempty"`
}

// IndexEntry 索引条目，用于列表展示（不包含完整请求/响应内容）
type IndexEntry struct {
	RequestID    string    `json:"requestId"`
	Timestamp    time.Time `json:"timestamp"`
	EndpointName string    `json:"endpointName"`
	ClientType   string    `json:"clientType"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"inputTokens"`
	OutputTokens int       `json:"outputTokens"`
	DurationMs   int64     `json:"durationMs"`
	Success      bool      `json:"success"`
}

// ToIndexEntry 将 Record 转换为 IndexEntry
func (r *Record) ToIndexEntry() IndexEntry {
	return IndexEntry{
		RequestID:    r.RequestID,
		Timestamp:    r.Timestamp,
		EndpointName: r.Endpoint.Name,
		ClientType:   r.Client.Type,
		Model:        r.Request.Model,
		InputTokens:  r.Stats.InputTokens,
		OutputTokens: r.Stats.OutputTokens,
		DurationMs:   r.Stats.DurationMs,
		Success:      r.Stats.Success,
	}
}
