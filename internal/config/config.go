package config

import (
	"fmt"
	"strconv"
	"sync"
)

// Endpoint represents a single API endpoint configuration
type Endpoint struct {
	Name        string `json:"name"`
	ClientType  string `json:"clientType,omitempty"`  // Client type: claude, gemini, codex (default: claude)
	APIUrl      string `json:"apiUrl"`
	APIKey      string `json:"apiKey"`
	Enabled     bool   `json:"enabled"`
	Transformer string `json:"transformer,omitempty"` // Transformer type: claude, openai, gemini, deepseek
	Model       string `json:"model,omitempty"`       // Target model name for non-Claude APIs
	Remark      string `json:"remark,omitempty"`      // Optional remark for the endpoint
	Tags        string `json:"tags,omitempty"`        // Comma-separated tags for grouping/filtering

	// 智能路由相关字段
	ModelPatterns      string  `json:"modelPatterns,omitempty"`      // 模型匹配模式，逗号分隔，支持通配符如 claude-*,gpt-4*
	CostPerInputToken  float64 `json:"costPerInputToken,omitempty"`  // 每百万输入 Token 成本（美元）
	CostPerOutputToken float64 `json:"costPerOutputToken,omitempty"` // 每百万输出 Token 成本（美元）
	QuotaLimit         int64   `json:"quotaLimit,omitempty"`         // Token 配额限制，0 表示无限制
	QuotaResetCycle    string  `json:"quotaResetCycle,omitempty"`    // 配额重置周期：daily/weekly/monthly/never
	Priority           int     `json:"priority,omitempty"`           // 优先级，数字越小优先级越高，默认100
}

// WebDAVConfig represents WebDAV synchronization configuration
type WebDAVConfig struct {
	URL        string `json:"url"`        // WebDAV server URL
	Username   string `json:"username"`   // Username
	Password   string `json:"password"`   // Password
	ConfigPath string `json:"configPath"` // Config backup path (default /ccNexus/config)
	StatsPath  string `json:"statsPath"`  // Stats backup path (default /ccNexus/stats)
}

// LocalBackupConfig represents local backup configuration
type LocalBackupConfig struct {
	Dir string `json:"dir"` // Local directory to store backups
}

// S3BackupConfig represents S3-compatible backup configuration
type S3BackupConfig struct {
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region,omitempty"`
	Bucket         string `json:"bucket"`
	Prefix         string `json:"prefix,omitempty"`
	AccessKey      string `json:"accessKey"`
	SecretKey      string `json:"secretKey"`
	SessionToken   string `json:"sessionToken,omitempty"`
	UseSSL         bool   `json:"useSSL"`
	ForcePathStyle bool   `json:"forcePathStyle"`
}

// BackupConfig represents backup/sync configuration across providers
type BackupConfig struct {
	Provider string             `json:"provider"` // webdav | local | s3
	Local    *LocalBackupConfig `json:"local,omitempty"`
	S3       *S3BackupConfig    `json:"s3,omitempty"`
}

// ProxyConfig represents HTTP proxy configuration
type ProxyConfig struct {
	URL string `json:"url"` // Proxy URL, e.g., http://127.0.0.1:7890 or socks5://127.0.0.1:1080
}

// AlertConfig 端点故障告警配置
type AlertConfig struct {
	Enabled              bool `json:"enabled"`              // 是否启用告警
	ConsecutiveFailures  int  `json:"consecutiveFailures"`  // 连续失败次数触发告警，默认3次
	NotifyOnRecovery     bool `json:"notifyOnRecovery"`     // 恢复时是否通知
	SystemNotification   bool `json:"systemNotification"`   // 是否发送系统通知
	AlertCooldownMinutes int  `json:"alertCooldownMinutes"` // 告警冷却时间（分钟），避免频繁告警，默认5分钟
	// 性能异常告警配置
	PerformanceAlertEnabled   bool `json:"performanceAlertEnabled"`   // 是否启用性能异常告警
	LatencyThresholdMs        int  `json:"latencyThresholdMs"`        // 延迟阈值（毫秒），超过此值触发告警，默认5000ms
	LatencyIncreasePercent    int  `json:"latencyIncreasePercent"`    // 延迟增加百分比，相比平均值增加此比例触发告警，默认200%
}

// CacheConfig 请求缓存配置
type CacheConfig struct {
	Enabled    bool `json:"enabled"`    // 是否启用缓存
	TTLSeconds int  `json:"ttlSeconds"` // 缓存过期时间（秒），默认300秒（5分钟）
	MaxEntries int  `json:"maxEntries"` // 最大缓存条目数，默认1000
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled          bool `json:"enabled"`          // 是否启用速率限制
	GlobalLimit      int  `json:"globalLimit"`      // 全局每分钟最大请求数，默认60
	PerEndpointLimit int  `json:"perEndpointLimit"` // 每端点每分钟最大请求数，默认30
}

// Config represents the application configuration
type Config struct {
	Port                       int              `json:"port"`
	Endpoints                  []Endpoint       `json:"endpoints"`
	LogLevel                   int              `json:"logLevel"`                      // 0=DEBUG, 1=INFO, 2=WARN, 3=ERROR
	Language                   string           `json:"language"`                      // UI language: en, zh-CN
	Theme                      string           `json:"theme"`                         // UI theme: light, dark
	ThemeAuto                  bool             `json:"themeAuto"`                     // Auto switch theme based on time
	AutoThemeMode              string           `json:"autoThemeMode,omitempty"`       // Auto theme mode: time, system (default: time)
	AutoLightTheme             string           `json:"autoLightTheme,omitempty"`      // Theme to use in daytime when auto mode is on
	AutoDarkTheme              string           `json:"autoDarkTheme,omitempty"`       // Theme to use in nighttime when auto mode is on
	WindowWidth                int              `json:"windowWidth"`                   // Window width in pixels
	WindowHeight               int              `json:"windowHeight"`                  // Window height in pixels
	CloseWindowBehavior        string           `json:"closeWindowBehavior,omitempty"` // "quit", "minimize", "ask"
	HealthCheckInterval        int              `json:"healthCheckInterval"`           // Health check interval in seconds, 0 to disable
	HealthHistoryRetentionDays int              `json:"healthHistoryRetentionDays"`    // Health history retention days, default 7
	RequestTimeout             int              `json:"requestTimeout"`                // Request timeout in seconds, 0 for default (300s)
	Alert                      *AlertConfig     `json:"alert,omitempty"`               // 端点故障告警配置
	Cache                      *CacheConfig     `json:"cache,omitempty"`               // 请求缓存配置
	RateLimit                  *RateLimitConfig `json:"rateLimit,omitempty"`           // 速率限制配置
	Routing                    *RoutingConfig   `json:"routing,omitempty"`             // 智能路由配置
	WebDAV                     *WebDAVConfig    `json:"webdav,omitempty"`              // WebDAV synchronization config
	Backup                     *BackupConfig    `json:"backup,omitempty"`              // Backup/sync configuration
	Proxy                      *ProxyConfig     `json:"proxy,omitempty"`               // HTTP proxy config
	mu                         sync.RWMutex
}

// CopyFrom copies all configuration values from another Config (excluding mutex)
// This is safe to use when you need to update config values without copying the mutex
func (c *Config) CopyFrom(other *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	c.Port = other.Port
	c.Endpoints = make([]Endpoint, len(other.Endpoints))
	copy(c.Endpoints, other.Endpoints)
	c.LogLevel = other.LogLevel
	c.Language = other.Language
	c.Theme = other.Theme
	c.ThemeAuto = other.ThemeAuto
	c.AutoThemeMode = other.AutoThemeMode
	c.AutoLightTheme = other.AutoLightTheme
	c.AutoDarkTheme = other.AutoDarkTheme
	c.WindowWidth = other.WindowWidth
	c.WindowHeight = other.WindowHeight
	c.CloseWindowBehavior = other.CloseWindowBehavior
	c.HealthCheckInterval = other.HealthCheckInterval
	c.HealthHistoryRetentionDays = other.HealthHistoryRetentionDays
	c.RequestTimeout = other.RequestTimeout

	if other.WebDAV != nil {
		c.WebDAV = &WebDAVConfig{
			URL:      other.WebDAV.URL,
			Username: other.WebDAV.Username,
			Password: other.WebDAV.Password,
		}
	} else {
		c.WebDAV = nil
	}

	if other.Backup != nil {
		c.Backup = &BackupConfig{}
		if other.Backup.S3 != nil {
			c.Backup.S3 = &S3BackupConfig{
				Endpoint:       other.Backup.S3.Endpoint,
				Region:         other.Backup.S3.Region,
				Bucket:         other.Backup.S3.Bucket,
				Prefix:         other.Backup.S3.Prefix,
				AccessKey:      other.Backup.S3.AccessKey,
				SecretKey:      other.Backup.S3.SecretKey,
				SessionToken:   other.Backup.S3.SessionToken,
				UseSSL:         other.Backup.S3.UseSSL,
				ForcePathStyle: other.Backup.S3.ForcePathStyle,
			}
		}
		if other.Backup.Local != nil {
			c.Backup.Local = &LocalBackupConfig{
				Dir: other.Backup.Local.Dir,
			}
		}
		c.Backup.Provider = other.Backup.Provider
	} else {
		c.Backup = nil
	}

	if other.Proxy != nil {
		c.Proxy = &ProxyConfig{URL: other.Proxy.URL}
	} else {
		c.Proxy = nil
	}

	if other.Alert != nil {
		c.Alert = &AlertConfig{
			Enabled:                   other.Alert.Enabled,
			ConsecutiveFailures:       other.Alert.ConsecutiveFailures,
			NotifyOnRecovery:          other.Alert.NotifyOnRecovery,
			SystemNotification:        other.Alert.SystemNotification,
			AlertCooldownMinutes:      other.Alert.AlertCooldownMinutes,
			PerformanceAlertEnabled:   other.Alert.PerformanceAlertEnabled,
			LatencyThresholdMs:        other.Alert.LatencyThresholdMs,
			LatencyIncreasePercent:    other.Alert.LatencyIncreasePercent,
		}
	} else {
		c.Alert = nil
	}

	if other.Cache != nil {
		c.Cache = &CacheConfig{
			Enabled:    other.Cache.Enabled,
			TTLSeconds: other.Cache.TTLSeconds,
			MaxEntries: other.Cache.MaxEntries,
		}
	} else {
		c.Cache = nil
	}

	if other.RateLimit != nil {
		c.RateLimit = &RateLimitConfig{
			Enabled:          other.RateLimit.Enabled,
			GlobalLimit:      other.RateLimit.GlobalLimit,
			PerEndpointLimit: other.RateLimit.PerEndpointLimit,
		}
	} else {
		c.RateLimit = nil
	}

	if other.Routing != nil {
		c.Routing = &RoutingConfig{
			EnableModelRouting:   other.Routing.EnableModelRouting,
			EnableLoadBalance:    other.Routing.EnableLoadBalance,
			EnableCostPriority:   other.Routing.EnableCostPriority,
			EnableQuotaRouting:   other.Routing.EnableQuotaRouting,
			LoadBalanceAlgorithm: other.Routing.LoadBalanceAlgorithm,
		}
	} else {
		c.Routing = nil
	}
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Port:         3003,
		LogLevel:     1,       // Default to INFO level
		Language:     "zh-CN", // Default to Chinese
		WindowWidth:  1024,    // Default window width
		WindowHeight: 768,     // Default window height
		Endpoints: []Endpoint{
			{
				Name:        "Claude Official",
				ClientType:  "claude",
				APIUrl:      "api.anthropic.com",
				APIKey:      "your-api-key-here",
				Enabled:     true,
				Transformer: "claude",
			},
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	if len(c.Endpoints) == 0 {
		return fmt.Errorf("no endpoints configured")
	}

	for i, ep := range c.Endpoints {
		if ep.APIUrl == "" {
			return fmt.Errorf("endpoint %d: apiUrl is required", i+1)
		}
		if ep.APIKey == "" {
			return fmt.Errorf("endpoint %d: apiKey is required", i+1)
		}

		// Default to claude transformer if not specified
		if ep.Transformer == "" {
			c.Endpoints[i].Transformer = "claude"
		}

		// Non-Claude transformers require model field
		if ep.Transformer != "claude" && ep.Model == "" {
			return fmt.Errorf("endpoint %d (%s): model is required for transformer '%s'", i+1, ep.Name, ep.Transformer)
		}
	}

	return nil
}

// GetEndpoints returns a copy of endpoints (thread-safe)
func (c *Config) GetEndpoints() []Endpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	endpoints := make([]Endpoint, len(c.Endpoints))
	copy(endpoints, c.Endpoints)
	return endpoints
}

// GetEndpointsByClient returns endpoints filtered by client type (thread-safe)
func (c *Config) GetEndpointsByClient(clientType string) []Endpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Default to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	var filtered []Endpoint
	for _, ep := range c.Endpoints {
		epClientType := ep.ClientType
		if epClientType == "" {
			epClientType = "claude"
		}
		if epClientType == clientType {
			filtered = append(filtered, ep)
		}
	}
	return filtered
}

// GetEnabledEndpointsByClient returns enabled endpoints filtered by client type (thread-safe)
func (c *Config) GetEnabledEndpointsByClient(clientType string) []Endpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Default to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	var filtered []Endpoint
	for _, ep := range c.Endpoints {
		epClientType := ep.ClientType
		if epClientType == "" {
			epClientType = "claude"
		}
		if epClientType == clientType && ep.Enabled {
			filtered = append(filtered, ep)
		}
	}
	return filtered
}

// GetPort returns the configured port (thread-safe)
func (c *Config) GetPort() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Port
}

// GetLogLevel returns the configured log level (thread-safe)
func (c *Config) GetLogLevel() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LogLevel
}

// UpdateEndpoints updates the endpoints (thread-safe)
func (c *Config) UpdateEndpoints(endpoints []Endpoint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Endpoints = endpoints
}

// SetEndpointEnabled 设置指定客户端类型下指定索引的端点的启用状态
func (c *Config) SetEndpointEnabled(clientType string, index int, enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if clientType == "" {
		clientType = "claude"
	}

	// 找到该客户端类型的端点在全局列表中的实际索引
	currentIndex := 0
	for i := range c.Endpoints {
		epClientType := c.Endpoints[i].ClientType
		if epClientType == "" {
			epClientType = "claude"
		}
		if epClientType == clientType {
			if currentIndex == index {
				c.Endpoints[i].Enabled = enabled
				return
			}
			currentIndex++
		}
	}
}

// MoveEndpoint 将指定客户端类型下的端点从 fromIndex 移动到 toIndex
func (c *Config) MoveEndpoint(clientType string, fromIndex, toIndex int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if clientType == "" {
		clientType = "claude"
	}

	if fromIndex == toIndex {
		return
	}

	// 找到该客户端类型的端点在全局列表中的实际索引
	var globalIndices []int
	for i := range c.Endpoints {
		epClientType := c.Endpoints[i].ClientType
		if epClientType == "" {
			epClientType = "claude"
		}
		if epClientType == clientType {
			globalIndices = append(globalIndices, i)
		}
	}

	if fromIndex < 0 || fromIndex >= len(globalIndices) || toIndex < 0 || toIndex >= len(globalIndices) {
		return
	}

	// 获取全局索引
	globalFromIndex := globalIndices[fromIndex]
	globalToIndex := globalIndices[toIndex]

	// 执行移动
	endpoint := c.Endpoints[globalFromIndex]
	// 先删除
	c.Endpoints = append(c.Endpoints[:globalFromIndex], c.Endpoints[globalFromIndex+1:]...)
	// 调整目标索引（如果源索引在目标索引之前）
	if globalFromIndex < globalToIndex {
		globalToIndex--
	}
	// 再插入
	c.Endpoints = append(c.Endpoints[:globalToIndex], append([]Endpoint{endpoint}, c.Endpoints[globalToIndex:]...)...)
}

// UpdatePort updates the port (thread-safe)
func (c *Config) UpdatePort(port int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Port = port
}

// UpdateLogLevel updates the log level (thread-safe)
func (c *Config) UpdateLogLevel(level int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LogLevel = level
}

// GetLanguage returns the configured language (thread-safe)
func (c *Config) GetLanguage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Language
}

// UpdateLanguage updates the language (thread-safe)
func (c *Config) UpdateLanguage(language string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Language = language
}

// GetWindowSize returns the configured window size (thread-safe)
func (c *Config) GetWindowSize() (width, height int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WindowWidth, c.WindowHeight
}

// UpdateWindowSize updates the window size (thread-safe)
func (c *Config) UpdateWindowSize(width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WindowWidth = width
	c.WindowHeight = height
}

// GetCloseWindowBehavior returns the close window behavior (thread-safe)
// Returns: "quit", "minimize", "ask"
func (c *Config) GetCloseWindowBehavior() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CloseWindowBehavior
}

// UpdateCloseWindowBehavior updates the close window behavior (thread-safe)
// Accepts: "quit", "minimize", "ask"
func (c *Config) UpdateCloseWindowBehavior(behavior string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CloseWindowBehavior = behavior
}

// GetTheme returns the configured theme (thread-safe)
// Returns: "light", "dark"
func (c *Config) GetTheme() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Theme
}

// UpdateTheme updates the theme (thread-safe)
// Accepts: "light", "dark"
func (c *Config) UpdateTheme(theme string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Theme = theme
}

// GetThemeAuto returns whether auto theme switching is enabled (thread-safe)
func (c *Config) GetThemeAuto() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ThemeAuto
}

// UpdateThemeAuto updates the auto theme setting (thread-safe)
func (c *Config) UpdateThemeAuto(auto bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ThemeAuto = auto
}

// GetAutoLightTheme returns the theme to use in daytime when auto mode is on (thread-safe)
func (c *Config) GetAutoLightTheme() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AutoLightTheme
}

// UpdateAutoLightTheme updates the auto light theme (thread-safe)
func (c *Config) UpdateAutoLightTheme(theme string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AutoLightTheme = theme
}

// GetAutoDarkTheme returns the theme to use in nighttime when auto mode is on (thread-safe)
func (c *Config) GetAutoDarkTheme() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AutoDarkTheme
}

// UpdateAutoDarkTheme updates the auto dark theme (thread-safe)
func (c *Config) UpdateAutoDarkTheme(theme string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AutoDarkTheme = theme
}

// GetAutoThemeMode returns the auto theme mode (thread-safe)
// Returns: "time" (default), "system"
func (c *Config) GetAutoThemeMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.AutoThemeMode == "" {
		return "time"
	}
	return c.AutoThemeMode
}

// UpdateAutoThemeMode updates the auto theme mode (thread-safe)
// Accepts: "time", "system"
func (c *Config) UpdateAutoThemeMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AutoThemeMode = mode
}

// GetWebDAV returns the WebDAV configuration (thread-safe)
func (c *Config) GetWebDAV() *WebDAVConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WebDAV
}

// UpdateWebDAV updates the WebDAV configuration (thread-safe)
func (c *Config) UpdateWebDAV(webdav *WebDAVConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WebDAV = webdav
}

// GetBackup returns the backup configuration (thread-safe)
func (c *Config) GetBackup() *BackupConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Backup
}

// UpdateBackup updates the backup configuration (thread-safe)
func (c *Config) UpdateBackup(backup *BackupConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Backup = backup
}

// GetProxy returns the Proxy configuration (thread-safe)
func (c *Config) GetProxy() *ProxyConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Proxy
}

// UpdateProxy updates the Proxy configuration (thread-safe)
func (c *Config) UpdateProxy(proxy *ProxyConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Proxy = proxy
}

// GetHealthCheckInterval returns the health check interval in seconds (thread-safe)
// Returns 0 if health check is disabled
func (c *Config) GetHealthCheckInterval() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.HealthCheckInterval
}

// UpdateHealthCheckInterval updates the health check interval (thread-safe)
// Set to 0 to disable health check
func (c *Config) UpdateHealthCheckInterval(interval int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.HealthCheckInterval = interval
}

// GetRequestTimeout returns the request timeout in seconds (thread-safe)
// Returns 0 if using default (300 seconds)
func (c *Config) GetRequestTimeout() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RequestTimeout
}

// UpdateRequestTimeout updates the request timeout (thread-safe)
// Set to 0 to use default (300 seconds)
func (c *Config) UpdateRequestTimeout(timeout int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RequestTimeout = timeout
}

// GetHealthHistoryRetentionDays returns the health history retention days (thread-safe)
// Returns default 7 if not set
func (c *Config) GetHealthHistoryRetentionDays() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.HealthHistoryRetentionDays == 0 {
		return 7
	}
	return c.HealthHistoryRetentionDays
}

// UpdateHealthHistoryRetentionDays updates the health history retention days (thread-safe)
func (c *Config) UpdateHealthHistoryRetentionDays(days int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.HealthHistoryRetentionDays = days
}

// GetAlert returns the alert configuration (thread-safe)
// Returns default config if not set
func (c *Config) GetAlert() *AlertConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Alert == nil {
		return &AlertConfig{
			Enabled:                   false,
			ConsecutiveFailures:       3,
			NotifyOnRecovery:          true,
			SystemNotification:        true,
			AlertCooldownMinutes:      5,
			PerformanceAlertEnabled:   false,
			LatencyThresholdMs:        5000,
			LatencyIncreasePercent:    200,
		}
	}
	return c.Alert
}

// UpdateAlert updates the alert configuration (thread-safe)
func (c *Config) UpdateAlert(alert *AlertConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Alert = alert
}

// GetCache returns the cache configuration (thread-safe)
// Returns default config if not set
func (c *Config) GetCache() *CacheConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Cache == nil {
		return &CacheConfig{
			Enabled:    false,
			TTLSeconds: 300,
			MaxEntries: 1000,
		}
	}
	return c.Cache
}

// UpdateCache updates the cache configuration (thread-safe)
func (c *Config) UpdateCache(cache *CacheConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Cache = cache
}

// GetRateLimit returns the rate limit configuration (thread-safe)
// Returns default config if not set
func (c *Config) GetRateLimit() *RateLimitConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.RateLimit == nil {
		return &RateLimitConfig{
			Enabled:          false,
			GlobalLimit:      60,
			PerEndpointLimit: 30,
		}
	}
	return c.RateLimit
}

// UpdateRateLimit updates the rate limit configuration (thread-safe)
func (c *Config) UpdateRateLimit(rateLimit *RateLimitConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RateLimit = rateLimit
}

// StorageAdapter defines the interface needed for loading/saving config
type StorageAdapter interface {
	GetEndpoints() ([]StorageEndpoint, error)
	GetEndpointsByClient(clientType string) ([]StorageEndpoint, error)
	SaveEndpoint(ep *StorageEndpoint) error
	UpdateEndpoint(ep *StorageEndpoint) error
	DeleteEndpoint(name string, clientType string) error
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error
}

// StorageEndpoint represents an endpoint in storage
type StorageEndpoint struct {
	Name        string
	ClientType  string
	APIUrl      string
	APIKey      string
	Enabled     bool
	Transformer string
	Model       string
	Remark      string
	Tags        string
	SortOrder   int

	// 智能路由相关字段
	ModelPatterns      string
	CostPerInputToken  float64
	CostPerOutputToken float64
	QuotaLimit         int64
	QuotaResetCycle    string
	Priority           int
}

// LoadFromStorage loads configuration from SQLite storage
func LoadFromStorage(storage StorageAdapter) (*Config, error) {
	config := &Config{
		Endpoints: []Endpoint{},
	}

	// Load endpoints
	endpoints, err := storage.GetEndpoints()
	if err != nil {
		return nil, fmt.Errorf("failed to load endpoints: %w", err)
	}

	for _, ep := range endpoints {
		clientType := ep.ClientType
		if clientType == "" {
			clientType = "claude"
		}
		endpoint := Endpoint{
			Name:               ep.Name,
			ClientType:         clientType,
			APIUrl:             ep.APIUrl,
			APIKey:             ep.APIKey,
			Enabled:            ep.Enabled,
			Transformer:        ep.Transformer,
			Model:              ep.Model,
			Remark:             ep.Remark,
			Tags:               ep.Tags,
			ModelPatterns:      ep.ModelPatterns,
			CostPerInputToken:  ep.CostPerInputToken,
			CostPerOutputToken: ep.CostPerOutputToken,
			QuotaLimit:         ep.QuotaLimit,
			QuotaResetCycle:    ep.QuotaResetCycle,
			Priority:           ep.Priority,
		}
		if endpoint.Transformer == "" {
			endpoint.Transformer = "claude"
		}
		// 默认优先级为 100
		if endpoint.Priority == 0 {
			endpoint.Priority = 100
		}
		config.Endpoints = append(config.Endpoints, endpoint)
	}

	// Load app config
	if portStr, err := storage.GetConfig("port"); err == nil && portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}
	if config.Port == 0 {
		config.Port = 3003
	}

	if logLevelStr, err := storage.GetConfig("logLevel"); err == nil && logLevelStr != "" {
		if logLevel, err := strconv.Atoi(logLevelStr); err == nil {
			config.LogLevel = logLevel
		}
	}

	if lang, err := storage.GetConfig("language"); err == nil {
		config.Language = lang
	}

	if widthStr, err := storage.GetConfig("windowWidth"); err == nil && widthStr != "" {
		if width, err := strconv.Atoi(widthStr); err == nil {
			config.WindowWidth = width
		}
	}
	if config.WindowWidth == 0 {
		config.WindowWidth = 1024
	}

	if heightStr, err := storage.GetConfig("windowHeight"); err == nil && heightStr != "" {
		if height, err := strconv.Atoi(heightStr); err == nil {
			config.WindowHeight = height
		}
	}
	if config.WindowHeight == 0 {
		config.WindowHeight = 768
	}

	// Load close window behavior
	if behaviorStr, err := storage.GetConfig("closeWindowBehavior"); err == nil && behaviorStr != "" {
		config.CloseWindowBehavior = behaviorStr
	}
	// Default to "ask" if not set
	if config.CloseWindowBehavior == "" {
		config.CloseWindowBehavior = "ask"
	}

	// Load theme
	if theme, err := storage.GetConfig("theme"); err == nil && theme != "" {
		config.Theme = theme
	}
	// Default to "light" if not set
	if config.Theme == "" {
		config.Theme = "light"
	}

	// Load themeAuto
	if themeAuto, err := storage.GetConfig("themeAuto"); err == nil && themeAuto != "" {
		config.ThemeAuto = themeAuto == "true"
	}

	// Load autoLightTheme
	if autoLightTheme, err := storage.GetConfig("autoLightTheme"); err == nil && autoLightTheme != "" {
		config.AutoLightTheme = autoLightTheme
	}
	// Default to "light" if not set
	if config.AutoLightTheme == "" {
		config.AutoLightTheme = "light"
	}

	// Load autoDarkTheme
	if autoDarkTheme, err := storage.GetConfig("autoDarkTheme"); err == nil && autoDarkTheme != "" {
		config.AutoDarkTheme = autoDarkTheme
	}
	// Default to "dark" if not set
	if config.AutoDarkTheme == "" {
		config.AutoDarkTheme = "dark"
	}

	// Load autoThemeMode
	if autoThemeMode, err := storage.GetConfig("autoThemeMode"); err == nil && autoThemeMode != "" {
		config.AutoThemeMode = autoThemeMode
	}
	// Default to "time" if not set
	if config.AutoThemeMode == "" {
		config.AutoThemeMode = "time"
	}

	// Load WebDAV config if exists
	if url, err := storage.GetConfig("webdav_url"); err == nil && url != "" {
		username, _ := storage.GetConfig("webdav_username")
		password, _ := storage.GetConfig("webdav_password")
		configPath, _ := storage.GetConfig("webdav_configPath")
		statsPath, _ := storage.GetConfig("webdav_statsPath")

		config.WebDAV = &WebDAVConfig{
			URL:        url,
			Username:   username,
			Password:   password,
			ConfigPath: configPath,
			StatsPath:  statsPath,
		}
	}

	// Load Backup config
	provider, _ := storage.GetConfig("backup_provider")
	if provider != "" {
		config.Backup = &BackupConfig{Provider: provider}
	}
	if provider == "local" {
		backupDir, _ := storage.GetConfig("backup_local_dir")
		config.Backup.Local = &LocalBackupConfig{Dir: backupDir}
	}
	if provider == "s3" {
		s3Endpoint, _ := storage.GetConfig("backup_s3_endpoint")
		s3Region, _ := storage.GetConfig("backup_s3_region")
		s3Bucket, _ := storage.GetConfig("backup_s3_bucket")
		s3Prefix, _ := storage.GetConfig("backup_s3_prefix")
		s3AccessKey, _ := storage.GetConfig("backup_s3_accessKey")
		s3SecretKey, _ := storage.GetConfig("backup_s3_secretKey")
		s3SessionToken, _ := storage.GetConfig("backup_s3_sessionToken")
		s3UseSSLStr, _ := storage.GetConfig("backup_s3_useSSL")
		s3ForcePathStyleStr, _ := storage.GetConfig("backup_s3_forcePathStyle")

		config.Backup.S3 = &S3BackupConfig{
			Endpoint:       s3Endpoint,
			Region:         s3Region,
			Bucket:         s3Bucket,
			Prefix:         s3Prefix,
			AccessKey:      s3AccessKey,
			SecretKey:      s3SecretKey,
			SessionToken:   s3SessionToken,
			UseSSL:         s3UseSSLStr == "true",
			ForcePathStyle: s3ForcePathStyleStr == "true",
		}
	}

	// Load Proxy config
	if proxyURL, err := storage.GetConfig("proxy_url"); err == nil && proxyURL != "" {
		config.Proxy = &ProxyConfig{URL: proxyURL}
	}

	// Load health check interval
	if intervalStr, err := storage.GetConfig("healthCheckInterval"); err == nil && intervalStr != "" {
		if interval, err := strconv.Atoi(intervalStr); err == nil {
			config.HealthCheckInterval = interval
		}
	}

	// Load health history retention days
	if retentionStr, err := storage.GetConfig("healthHistoryRetentionDays"); err == nil && retentionStr != "" {
		if retention, err := strconv.Atoi(retentionStr); err == nil {
			config.HealthHistoryRetentionDays = retention
		}
	}
	// Default to 7 days if not set
	if config.HealthHistoryRetentionDays == 0 {
		config.HealthHistoryRetentionDays = 7
	}

	// Load request timeout
	if timeoutStr, err := storage.GetConfig("requestTimeout"); err == nil && timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.RequestTimeout = timeout
		}
	}

	// Load alert config
	if alertEnabled, err := storage.GetConfig("alert_enabled"); err == nil && alertEnabled != "" {
		config.Alert = &AlertConfig{
			Enabled:              alertEnabled == "true",
			ConsecutiveFailures:  3,
			NotifyOnRecovery:     true,
			SystemNotification:   true,
			AlertCooldownMinutes: 5,
		}
		if consecutiveStr, err := storage.GetConfig("alert_consecutiveFailures"); err == nil && consecutiveStr != "" {
			if consecutive, err := strconv.Atoi(consecutiveStr); err == nil {
				config.Alert.ConsecutiveFailures = consecutive
			}
		}
		if notifyRecovery, err := storage.GetConfig("alert_notifyOnRecovery"); err == nil && notifyRecovery != "" {
			config.Alert.NotifyOnRecovery = notifyRecovery == "true"
		}
		if sysNotify, err := storage.GetConfig("alert_systemNotification"); err == nil && sysNotify != "" {
			config.Alert.SystemNotification = sysNotify == "true"
		}
		if cooldownStr, err := storage.GetConfig("alert_cooldownMinutes"); err == nil && cooldownStr != "" {
			if cooldown, err := strconv.Atoi(cooldownStr); err == nil {
				config.Alert.AlertCooldownMinutes = cooldown
			}
		}
	}

	// Load routing config
	if enableModelRouting, err := storage.GetConfig("routing_enableModelRouting"); err == nil && enableModelRouting != "" {
		config.Routing = &RoutingConfig{
			EnableModelRouting: enableModelRouting == "true",
		}
		if enableLoadBalance, err := storage.GetConfig("routing_enableLoadBalance"); err == nil && enableLoadBalance != "" {
			config.Routing.EnableLoadBalance = enableLoadBalance == "true"
		}
		if enableCostPriority, err := storage.GetConfig("routing_enableCostPriority"); err == nil && enableCostPriority != "" {
			config.Routing.EnableCostPriority = enableCostPriority == "true"
		}
		if enableQuotaRouting, err := storage.GetConfig("routing_enableQuotaRouting"); err == nil && enableQuotaRouting != "" {
			config.Routing.EnableQuotaRouting = enableQuotaRouting == "true"
		}
		if loadBalanceAlgorithm, err := storage.GetConfig("routing_loadBalanceAlgorithm"); err == nil && loadBalanceAlgorithm != "" {
			config.Routing.LoadBalanceAlgorithm = loadBalanceAlgorithm
		}
	}

	return config, nil
}

// SaveToStorage saves configuration to SQLite storage
func (c *Config) SaveToStorage(storage StorageAdapter) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get existing endpoints from storage
	existingEndpoints, err := storage.GetEndpoints()
	if err != nil {
		return fmt.Errorf("failed to get existing endpoints: %w", err)
	}

	// Use clientType:name as key to track existing endpoints
	existingKeys := make(map[string]string) // key -> clientType
	for _, ep := range existingEndpoints {
		clientType := ep.ClientType
		if clientType == "" {
			clientType = "claude"
		}
		key := clientType + ":" + ep.Name
		existingKeys[key] = clientType
	}

	// Save/update endpoints
	for i, ep := range c.Endpoints {
		clientType := ep.ClientType
		if clientType == "" {
			clientType = "claude"
		}
		endpoint := &StorageEndpoint{
			Name:               ep.Name,
			ClientType:         clientType,
			APIUrl:             ep.APIUrl,
			APIKey:             ep.APIKey,
			Enabled:            ep.Enabled,
			Transformer:        ep.Transformer,
			Model:              ep.Model,
			Remark:             ep.Remark,
			Tags:               ep.Tags,
			SortOrder:          i, // Use array index as sort order
			ModelPatterns:      ep.ModelPatterns,
			CostPerInputToken:  ep.CostPerInputToken,
			CostPerOutputToken: ep.CostPerOutputToken,
			QuotaLimit:         ep.QuotaLimit,
			QuotaResetCycle:    ep.QuotaResetCycle,
			Priority:           ep.Priority,
		}

		key := clientType + ":" + ep.Name
		if _, exists := existingKeys[key]; exists {
			if err := storage.UpdateEndpoint(endpoint); err != nil {
				return fmt.Errorf("failed to update endpoint %s: %w", ep.Name, err)
			}
		} else {
			if err := storage.SaveEndpoint(endpoint); err != nil {
				return fmt.Errorf("failed to save endpoint %s: %w", ep.Name, err)
			}
		}
		delete(existingKeys, key)
	}

	// Delete endpoints that no longer exist
	for key, clientType := range existingKeys {
		// Extract name from key (clientType:name)
		name := key[len(clientType)+1:]
		if err := storage.DeleteEndpoint(name, clientType); err != nil {
			return fmt.Errorf("failed to delete endpoint %s: %w", name, err)
		}
	}

	// Save app config
	storage.SetConfig("port", strconv.Itoa(c.Port))
	storage.SetConfig("logLevel", strconv.Itoa(c.LogLevel))
	storage.SetConfig("language", c.Language)
	storage.SetConfig("theme", c.Theme)
	storage.SetConfig("themeAuto", strconv.FormatBool(c.ThemeAuto))
	storage.SetConfig("autoThemeMode", c.AutoThemeMode)
	storage.SetConfig("autoLightTheme", c.AutoLightTheme)
	storage.SetConfig("autoDarkTheme", c.AutoDarkTheme)
	storage.SetConfig("windowWidth", strconv.Itoa(c.WindowWidth))
	storage.SetConfig("windowHeight", strconv.Itoa(c.WindowHeight))
	storage.SetConfig("closeWindowBehavior", c.CloseWindowBehavior)

	// Save WebDAV config
	if c.WebDAV != nil {
		storage.SetConfig("webdav_url", c.WebDAV.URL)
		storage.SetConfig("webdav_username", c.WebDAV.Username)
		storage.SetConfig("webdav_password", c.WebDAV.Password)
		storage.SetConfig("webdav_configPath", c.WebDAV.ConfigPath)
		storage.SetConfig("webdav_statsPath", c.WebDAV.StatsPath)
	}

	// Save Backup config
	if c.Backup != nil {
		storage.SetConfig("backup_provider", c.Backup.Provider)
		if c.Backup.Local != nil {
			storage.SetConfig("backup_local_dir", c.Backup.Local.Dir)
		}
		if c.Backup.S3 != nil {
			storage.SetConfig("backup_s3_endpoint", c.Backup.S3.Endpoint)
			storage.SetConfig("backup_s3_region", c.Backup.S3.Region)
			storage.SetConfig("backup_s3_bucket", c.Backup.S3.Bucket)
			storage.SetConfig("backup_s3_prefix", c.Backup.S3.Prefix)
			storage.SetConfig("backup_s3_accessKey", c.Backup.S3.AccessKey)
			storage.SetConfig("backup_s3_secretKey", c.Backup.S3.SecretKey)
			storage.SetConfig("backup_s3_sessionToken", c.Backup.S3.SessionToken)
			storage.SetConfig("backup_s3_useSSL", strconv.FormatBool(c.Backup.S3.UseSSL))
			storage.SetConfig("backup_s3_forcePathStyle", strconv.FormatBool(c.Backup.S3.ForcePathStyle))
		}
	}

	// Save Proxy config
	if c.Proxy != nil {
		storage.SetConfig("proxy_url", c.Proxy.URL)
	} else {
		storage.SetConfig("proxy_url", "")
	}

	// Save health check interval
	storage.SetConfig("healthCheckInterval", strconv.Itoa(c.HealthCheckInterval))

	// Save health history retention days
	storage.SetConfig("healthHistoryRetentionDays", strconv.Itoa(c.HealthHistoryRetentionDays))

	// Save request timeout
	storage.SetConfig("requestTimeout", strconv.Itoa(c.RequestTimeout))

	// Save alert config
	if c.Alert != nil {
		storage.SetConfig("alert_enabled", strconv.FormatBool(c.Alert.Enabled))
		storage.SetConfig("alert_consecutiveFailures", strconv.Itoa(c.Alert.ConsecutiveFailures))
		storage.SetConfig("alert_notifyOnRecovery", strconv.FormatBool(c.Alert.NotifyOnRecovery))
		storage.SetConfig("alert_systemNotification", strconv.FormatBool(c.Alert.SystemNotification))
		storage.SetConfig("alert_cooldownMinutes", strconv.Itoa(c.Alert.AlertCooldownMinutes))
	}

	// Save routing config
	if c.Routing != nil {
		storage.SetConfig("routing_enableModelRouting", strconv.FormatBool(c.Routing.EnableModelRouting))
		storage.SetConfig("routing_enableLoadBalance", strconv.FormatBool(c.Routing.EnableLoadBalance))
		storage.SetConfig("routing_enableCostPriority", strconv.FormatBool(c.Routing.EnableCostPriority))
		storage.SetConfig("routing_enableQuotaRouting", strconv.FormatBool(c.Routing.EnableQuotaRouting))
		storage.SetConfig("routing_loadBalanceAlgorithm", c.Routing.LoadBalanceAlgorithm)
	}

	return nil
}
