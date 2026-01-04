package storage

import "time"

type Endpoint struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ClientType  string    `json:"clientType"` // 客户端类型: claude, gemini, codex
	APIUrl      string    `json:"apiUrl"`
	APIKey      string    `json:"apiKey"`
	Enabled     bool      `json:"enabled"`
	Transformer string    `json:"transformer"`
	Model       string    `json:"model"`
	Remark      string    `json:"remark"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
	ID                  int64
	EndpointName        string
	ClientType          string // 客户端类型: claude, gemini, codex
	RequestID           string
	Timestamp           time.Time
	Date                string
	InputTokens         int
	CacheCreationTokens int
	CacheReadTokens     int
	OutputTokens        int
	Model               string
	IsStreaming         bool
	Success             bool
	DeviceID            string
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

	// Config
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error

	// Close
	Close() error
}
