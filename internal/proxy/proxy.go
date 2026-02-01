package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lich0821/ccNexus/internal/cache"
	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/interaction"
	"github.com/lich0821/ccNexus/internal/logger"
	"github.com/lich0821/ccNexus/internal/ratelimit"
	"github.com/lich0821/ccNexus/internal/storage"
	"github.com/lich0821/ccNexus/internal/transformer"
)

// requestCounter is used to generate unique request IDs for monitoring
var requestCounter uint64

// generateMonitorRequestID generates a unique request ID for monitoring
func generateMonitorRequestID() string {
	id := atomic.AddUint64(&requestCounter, 1)
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), id)
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
}

// Usage represents token usage information from API response
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// APIResponse represents the structure of API responses to extract usage
type APIResponse struct {
	Usage Usage `json:"usage"`
}

// Proxy represents the proxy server
type Proxy struct {
	config           *config.Config
	stats            *Stats
	cache            *cache.Cache                 // 请求缓存
	rateLimiter      *ratelimit.RateLimiter       // 速率限制器
	currentIndex     int                          // Legacy: for backward compatibility
	currentIndexByClient map[ClientType]int       // Per-client endpoint index
	mu               sync.RWMutex
	server           *http.Server
	activeRequests   map[string]bool              // tracks active requests by endpoint name
	activeRequestsMu sync.RWMutex                 // protects activeRequests map
	endpointCtx      map[string]context.Context   // context per endpoint for cancellation
	endpointCancel   map[string]context.CancelFunc // cancel functions per endpoint
	ctxMu            sync.RWMutex                 // protects context maps
	onEndpointSuccess func(endpointName string, clientType string)   // callback when endpoint request succeeds
	onEndpointRotated func(endpointName string, clientType string)   // callback when endpoint rotates
	interactionStorage *interaction.Storage       // interaction recording storage
	monitor          *Monitor                     // real-time request monitoring

	// 智能路由相关
	router           *Router                      // 智能路由选择器
	quotaTracker     *QuotaTracker                // 配额跟踪器
	sessionAffinity  *SessionAffinityManager      // 会话亲和性管理器
}

// New creates a new Proxy instance
func New(cfg *config.Config, statsStorage StatsStorage, deviceID string) *Proxy {
	stats := NewStats(statsStorage, deviceID)

	// 初始化缓存
	var reqCache *cache.Cache
	if cfg.Cache != nil {
		reqCache = cache.New(cfg.Cache.Enabled, cfg.Cache.TTLSeconds, cfg.Cache.MaxEntries)
	} else {
		reqCache = cache.New(false, 300, 1000) // 默认禁用
	}

	// 初始化速率限制器
	var rateLimiter *ratelimit.RateLimiter
	if cfg.RateLimit != nil {
		rateLimiter = ratelimit.New(cfg.RateLimit.Enabled, cfg.RateLimit.GlobalLimit, cfg.RateLimit.PerEndpointLimit)
	} else {
		rateLimiter = ratelimit.New(false, 60, 30) // 默认禁用
	}

	return &Proxy{
		config:              cfg,
		stats:               stats,
		cache:               reqCache,
		rateLimiter:         rateLimiter,
		currentIndex:        0,
		currentIndexByClient: make(map[ClientType]int),
		activeRequests:      make(map[string]bool),
		endpointCtx:         make(map[string]context.Context),
		endpointCancel:      make(map[string]context.CancelFunc),
		monitor:             NewMonitor(),
	}
}

// SetOnEndpointSuccess sets the callback for successful endpoint requests
func (p *Proxy) SetOnEndpointSuccess(callback func(endpointName string, clientType string)) {
	p.onEndpointSuccess = callback
}

// SetOnEndpointRotated sets the callback for endpoint rotation
func (p *Proxy) SetOnEndpointRotated(callback func(endpointName string, clientType string)) {
	p.onEndpointRotated = callback
}

// SetInteractionStorage sets the interaction storage for recording requests/responses
func (p *Proxy) SetInteractionStorage(storage *interaction.Storage) {
	p.interactionStorage = storage
}

// SetupRouter 初始化智能路由器和配额跟踪器
// store: 用于配额持久化的存储接口
func (p *Proxy) SetupRouter(store storage.Storage) {
	p.quotaTracker = NewQuotaTracker(p.config, store)
	p.router = NewRouter(p.config, p.monitor)

	// 初始化会话亲和性管理器
	if p.config.SessionAffinity != nil && p.config.SessionAffinity.Enabled {
		p.sessionAffinity = NewSessionAffinityManager(p.config)
		p.sessionAffinity.Start()
		logger.Info("Session affinity enabled")
	}
}

// GetRouter 获取路由器实例
func (p *Proxy) GetRouter() *Router {
	return p.router
}

// GetQuotaTracker 获取配额跟踪器实例
func (p *Proxy) GetQuotaTracker() *QuotaTracker {
	return p.quotaTracker
}

// GetSessionAffinity 获取会话亲和性管理器实例
func (p *Proxy) GetSessionAffinity() *SessionAffinityManager {
	return p.sessionAffinity
}

// UpdateRouterConfig 更新路由器配置（配置变更时调用）
func (p *Proxy) UpdateRouterConfig(cfg *config.Config) {
	if p.router != nil {
		p.router.UpdateConfig(cfg)
	}
	if p.quotaTracker != nil {
		p.quotaTracker.UpdateConfig(cfg)
	}
}

// GetMonitor returns the monitor instance for real-time request tracking
func (p *Proxy) GetMonitor() *Monitor {
	return p.monitor
}

// GetCache returns the cache instance
func (p *Proxy) GetCache() *cache.Cache {
	return p.cache
}

// GetCacheStats returns cache statistics
func (p *Proxy) GetCacheStats() cache.CacheStats {
	return p.cache.GetStats()
}

// ClearCache clears all cached entries
func (p *Proxy) ClearCache() {
	p.cache.Clear()
}

// UpdateCacheConfig updates cache configuration
func (p *Proxy) UpdateCacheConfig(enabled bool, ttlSeconds, maxEntries int) {
	p.cache.UpdateConfig(enabled, ttlSeconds, maxEntries)
}

// GetRateLimiter returns the rate limiter instance
func (p *Proxy) GetRateLimiter() *ratelimit.RateLimiter {
	return p.rateLimiter
}

// GetRateLimitStats returns rate limit statistics
func (p *Proxy) GetRateLimitStats() ratelimit.RateLimitStats {
	return p.rateLimiter.GetStats()
}

// UpdateRateLimitConfig updates rate limit configuration
func (p *Proxy) UpdateRateLimitConfig(enabled bool, globalLimit, perEndpointLimit int) {
	p.rateLimiter.UpdateConfig(enabled, globalLimit, perEndpointLimit)
}

// ResetRateLimitStats resets rate limit statistics
func (p *Proxy) ResetRateLimitStats() {
	p.rateLimiter.Reset()
}

// Start starts the proxy server
func (p *Proxy) Start() error {
	return p.StartWithMux(nil)
}

// StartWithMux starts the proxy server with an optional custom mux
// If the port is already in use, it will automatically try the next port (up to 10 attempts)
func (p *Proxy) StartWithMux(customMux *http.ServeMux) error {
	port := p.config.GetPort()

	var mux *http.ServeMux
	if customMux != nil {
		mux = customMux
	} else {
		mux = http.NewServeMux()
	}

	// Register proxy routes
	mux.HandleFunc("/", p.handleProxy)
	mux.HandleFunc("/v1/messages/count_tokens", p.handleCountTokens)
	mux.HandleFunc("/health", p.handleHealth)
	mux.HandleFunc("/stats", p.handleStats)

	// Try to find an available port (up to 10 attempts)
	maxAttempts := 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		currentPort := port + attempt
		addr := fmt.Sprintf(":%d", currentPort)

		// Try to listen on the port first to check availability
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Warn("Port %d is in use, trying port %d...", currentPort, currentPort+1)
			continue
		}

		p.server = &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		logger.Info("ccNexus starting on port %d", currentPort)
		logger.Info("Configured %d endpoints", len(p.config.GetEndpoints()))

		// Use the listener we already created
		return p.server.Serve(listener)
	}

	return fmt.Errorf("failed to find available port after %d attempts (tried ports %d-%d)", maxAttempts, port, port+maxAttempts-1)
}

// Stop stops the proxy server
func (p *Proxy) Stop() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

// getEnabledEndpoints returns available endpoints (only endpoints with status=available)
// 注意：此方法已改为只返回可用状态的端点，不再返回所有启用的端点
func (p *Proxy) getEnabledEndpoints() []config.Endpoint {
	allEndpoints := p.config.GetEndpoints()
	available := make([]config.Endpoint, 0)
	for _, ep := range allEndpoints {
		// 允许使用 available 和 untested 状态的端点
		if ep.Status == config.EndpointStatusAvailable || ep.Status == config.EndpointStatusUntested {
			available = append(available, ep)
		}
	}
	return available
}

// getEnabledEndpointsForClient returns available endpoints for a specific client type
// 注意：此方法返回可用状态和未检测状态的端点
func (p *Proxy) getEnabledEndpointsForClient(clientType ClientType) []config.Endpoint {
	return p.config.GetAvailableEndpointsByClient(string(clientType))
}

// getCurrentEndpoint returns the current endpoint (thread-safe) - legacy for backward compatibility
func (p *Proxy) getCurrentEndpoint() config.Endpoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	endpoints := p.getEnabledEndpoints()
	if len(endpoints) == 0 {
		// Return empty endpoint if no enabled endpoints
		return config.Endpoint{}
	}
	// Make sure currentIndex is within bounds
	index := p.currentIndex % len(endpoints)
	return endpoints[index]
}

// getCurrentEndpointForClient returns the current endpoint for a specific client type (thread-safe)
func (p *Proxy) getCurrentEndpointForClient(clientType ClientType) config.Endpoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	endpoints := p.getEnabledEndpointsForClient(clientType)
	if len(endpoints) == 0 {
		return config.Endpoint{}
	}

	index := p.currentIndexByClient[clientType] % len(endpoints)
	return endpoints[index]
}

// selectEndpointForRequest 使用智能路由选择端点（支持会话亲和性）
// 当路由器可用且启用路由策略时使用智能路由，否则回退到优先级选择
func (p *Proxy) selectEndpointForRequest(clientType ClientType, requestModel string, sessionID string) config.Endpoint {
	// 1. 检查会话亲和性
	if p.sessionAffinity != nil && sessionID != "" {
		if endpointName, exists := p.sessionAffinity.GetEndpointForSession(sessionID, string(clientType)); exists {
			// 验证端点仍然可用（必须是 available 状态）
			endpoint := p.config.GetEndpointByName(endpointName, string(clientType))
			if endpoint != nil && endpoint.Status == config.EndpointStatusAvailable {
				logger.Debug("[SESSION:%s] Using bound endpoint: %s", sessionID, endpointName)
				return *endpoint
			} else {
				// 端点不可用，解除绑定
				p.sessionAffinity.UnbindSession(sessionID)
				logger.Debug("[SESSION:%s] Endpoint %s unavailable, unbinding", sessionID, endpointName)
			}
		}
	}

	// 2. 新会话：检查是否启用智能路由策略
	var selectedEndpoint config.Endpoint
	if p.router != nil {
		routingCfg := p.config.GetRoutingConfig()
		// 如果启用了任一高级路由策略，使用智能路由
		if routingCfg.EnableModelRouting || routingCfg.EnableLoadBalance ||
			routingCfg.EnableCostPriority || routingCfg.EnableQuotaRouting {
			endpoint, err := p.router.SelectEndpoint(clientType, requestModel, p.quotaTracker)
			if err == nil {
				logger.Debug("[ROUTER:%s] Selected endpoint: %s (model: %s)", clientType, endpoint.Name, requestModel)
				selectedEndpoint = endpoint
				// 绑定会话到选中的端点
				if p.sessionAffinity != nil && sessionID != "" {
					p.sessionAffinity.BindSession(sessionID, endpoint.Name, string(clientType))
					logger.Debug("[SESSION:%s] Bound to new endpoint: %s", sessionID, endpoint.Name)
				}
				return selectedEndpoint
			}
			logger.Warn("[ROUTER:%s] Selection failed: %v, falling back to priority", clientType, err)
		}
	}

	// 3. 回退到优先级选择（默认行为）
	// 即使没有启用高级路由策略，也应该按优先级选择端点
	if p.router != nil {
		endpoint, err := p.router.selectByPriority(p.config.GetAvailableEndpointsByClient(string(clientType)))
		if err == nil {
			logger.Debug("[PRIORITY:%s] Selected endpoint: %s", clientType, endpoint.Name)
			selectedEndpoint = endpoint
			if p.sessionAffinity != nil && sessionID != "" {
				p.sessionAffinity.BindSession(sessionID, endpoint.Name, string(clientType))
				logger.Debug("[SESSION:%s] Bound to priority endpoint: %s", sessionID, endpoint.Name)
			}
			return selectedEndpoint
		}
		logger.Warn("[PRIORITY:%s] Selection failed: %v, falling back to round-robin", clientType, err)
	}

	// 4. 最后回退到传统轮询逻辑（仅当优先级选择也失败时）
	selectedEndpoint = p.getCurrentEndpointForClient(clientType)
	if p.sessionAffinity != nil && sessionID != "" {
		p.sessionAffinity.BindSession(sessionID, selectedEndpoint.Name, string(clientType))
		logger.Debug("[SESSION:%s] Bound to round-robin endpoint: %s", sessionID, selectedEndpoint.Name)
	}
	return selectedEndpoint
}

// recordQuotaUsage 记录配额使用量（请求成功后调用）
func (p *Proxy) recordQuotaUsage(endpointName, clientType string, usage transformer.TokenUsageDetail) {
	if p.quotaTracker == nil {
		return
	}

	// 计算总 Token 数（输入 + 输出）
	totalTokens := int64(usage.TotalInputTokens() + usage.OutputTokens)
	if totalTokens > 0 {
		p.quotaTracker.RecordUsage(endpointName, clientType, totalTokens)
	}
}

// markRequestActive marks an endpoint as having active requests
func (p *Proxy) markRequestActive(endpointName string) {
	p.activeRequestsMu.Lock()
	defer p.activeRequestsMu.Unlock()
	p.activeRequests[endpointName] = true
}

// markRequestInactive marks an endpoint as having no active requests
func (p *Proxy) markRequestInactive(endpointName string) {
	p.activeRequestsMu.Lock()
	defer p.activeRequestsMu.Unlock()
	delete(p.activeRequests, endpointName)
}

// hasActiveRequests checks if an endpoint has active requests
func (p *Proxy) hasActiveRequests(endpointName string) bool {
	p.activeRequestsMu.RLock()
	defer p.activeRequestsMu.RUnlock()
	return p.activeRequests[endpointName]
}

// isCurrentEndpoint checks if the given endpoint is still the current one - legacy for backward compatibility
func (p *Proxy) isCurrentEndpoint(endpointName string) bool {
	current := p.getCurrentEndpoint()
	return current.Name == endpointName
}

// isCurrentEndpointForClient checks if the given endpoint is still the current one for a client type
func (p *Proxy) isCurrentEndpointForClient(endpointName string, clientType ClientType) bool {
	current := p.getCurrentEndpointForClient(clientType)
	return current.Name == endpointName
}

// getEndpointContext returns a context for the given endpoint, creating one if needed
func (p *Proxy) getEndpointContext(endpointName string) context.Context {
	p.ctxMu.Lock()
	defer p.ctxMu.Unlock()

	if ctx, ok := p.endpointCtx[endpointName]; ok {
		return ctx
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.endpointCtx[endpointName] = ctx
	p.endpointCancel[endpointName] = cancel
	return ctx
}

// cancelEndpointRequests cancels all requests for the given endpoint
func (p *Proxy) cancelEndpointRequests(endpointName string) {
	p.ctxMu.Lock()
	defer p.ctxMu.Unlock()

	if cancel, ok := p.endpointCancel[endpointName]; ok {
		cancel()
		delete(p.endpointCtx, endpointName)
		delete(p.endpointCancel, endpointName)
	}
}

// rotateEndpoint switches to the next endpoint (thread-safe)
// waitForActive: if true, waits briefly for active requests to complete before switching
func (p *Proxy) rotateEndpoint() config.Endpoint {
	p.mu.Lock()
	defer p.mu.Unlock()

	endpoints := p.getEnabledEndpoints()
	if len(endpoints) == 0 {
		return config.Endpoint{}
	}

	oldIndex := p.currentIndex % len(endpoints)
	oldEndpoint := endpoints[oldIndex]

	// Check if there are active requests on the current endpoint
	// Wait a short time for them to complete (max 500ms)
	if p.hasActiveRequests(oldEndpoint.Name) {
		logger.Debug("[SWITCH] Waiting for active requests on %s to complete...", oldEndpoint.Name)
		p.mu.Unlock() // Release lock while waiting

		for i := 0; i < 10; i++ { // Check 10 times, 50ms each = 500ms max
			time.Sleep(50 * time.Millisecond)
			if !p.hasActiveRequests(oldEndpoint.Name) {
				break
			}
		}

		p.mu.Lock() // Re-acquire lock

		// Re-fetch endpoints after re-acquiring lock (may have changed)
		endpoints = p.getEnabledEndpoints()
		if len(endpoints) == 0 {
			return config.Endpoint{}
		}
	}

	// Use oldIndex to calculate next, avoiding skip if currentIndex was modified during wait
	p.currentIndex = (oldIndex + 1) % len(endpoints)

	newEndpoint := endpoints[p.currentIndex]
	logger.Debug("[SWITCH] %s → %s (#%d)", oldEndpoint.Name, newEndpoint.Name, p.currentIndex+1)

	return newEndpoint
}

// rotateEndpointForClient switches to the next endpoint for a specific client type (thread-safe)
func (p *Proxy) rotateEndpointForClient(clientType ClientType) config.Endpoint {
	p.mu.Lock()
	defer p.mu.Unlock()

	endpoints := p.getEnabledEndpointsForClient(clientType)
	if len(endpoints) == 0 {
		return config.Endpoint{}
	}

	oldIndex := p.currentIndexByClient[clientType] % len(endpoints)
	oldEndpoint := endpoints[oldIndex]

	// Check if there are active requests on the current endpoint
	// Wait a short time for them to complete (max 500ms)
	if p.hasActiveRequests(oldEndpoint.Name) {
		logger.Debug("[SWITCH:%s] Waiting for active requests on %s to complete...", clientType, oldEndpoint.Name)
		p.mu.Unlock() // Release lock while waiting

		for i := 0; i < 10; i++ { // Check 10 times, 50ms each = 500ms max
			time.Sleep(50 * time.Millisecond)
			if !p.hasActiveRequests(oldEndpoint.Name) {
				break
			}
		}

		p.mu.Lock() // Re-acquire lock

		// Re-fetch endpoints after re-acquiring lock (may have changed)
		endpoints = p.getEnabledEndpointsForClient(clientType)
		if len(endpoints) == 0 {
			return config.Endpoint{}
		}
	}

	// Use oldIndex to calculate next, avoiding skip if currentIndex was modified during wait
	p.currentIndexByClient[clientType] = (oldIndex + 1) % len(endpoints)

	newEndpoint := endpoints[p.currentIndexByClient[clientType]]
	logger.Debug("[SWITCH:%s] %s → %s (#%d)", clientType, oldEndpoint.Name, newEndpoint.Name, p.currentIndexByClient[clientType]+1)

	// Trigger rotation callback
	if p.onEndpointRotated != nil {
		go p.onEndpointRotated(newEndpoint.Name, string(clientType))
	}

	return newEndpoint
}

// GetCurrentEndpointName returns the current endpoint name (thread-safe)
func (p *Proxy) GetCurrentEndpointName() string {
	endpoint := p.getCurrentEndpoint()
	return endpoint.Name
}

// GetCurrentEndpointNameForClient returns the current endpoint name for a specific client type (thread-safe)
func (p *Proxy) GetCurrentEndpointNameForClient(clientType string) string {
	endpoint := p.getCurrentEndpointForClient(ClientType(clientType))
	return endpoint.Name
}

// SetCurrentEndpoint manually switches to a specific endpoint by name
// Returns error if endpoint not found or not enabled
// Thread-safe and cancels ongoing requests on the old endpoint
func (p *Proxy) SetCurrentEndpoint(targetName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	endpoints := p.getEnabledEndpoints()
	if len(endpoints) == 0 {
		return fmt.Errorf("no enabled endpoints")
	}

	// Find the endpoint by name
	for i, ep := range endpoints {
		if ep.Name == targetName {
			oldEndpoint := endpoints[p.currentIndex%len(endpoints)]
			if oldEndpoint.Name != targetName {
				// Cancel all requests on the old endpoint
				p.cancelEndpointRequests(oldEndpoint.Name)
			}
			p.currentIndex = i
			logger.Info("[MANUAL SWITCH] %s → %s", oldEndpoint.Name, ep.Name)
			return nil
		}
	}

	return fmt.Errorf("endpoint '%s' not found or not enabled", targetName)
}

// SetCurrentEndpointForClient manually switches to a specific endpoint by name for a client type
// Returns error if endpoint not found or not enabled
// Thread-safe and cancels ongoing requests on the old endpoint
func (p *Proxy) SetCurrentEndpointForClient(clientType string, targetName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ct := ClientType(clientType)
	endpoints := p.getEnabledEndpointsForClient(ct)
	if len(endpoints) == 0 {
		return fmt.Errorf("no enabled endpoints for client type: %s", clientType)
	}

	// Find the endpoint by name
	for i, ep := range endpoints {
		if ep.Name == targetName {
			oldIndex := p.currentIndexByClient[ct] % len(endpoints)
			oldEndpoint := endpoints[oldIndex]
			if oldEndpoint.Name != targetName {
				// Cancel all requests on the old endpoint
				p.cancelEndpointRequests(oldEndpoint.Name)
			}
			p.currentIndexByClient[ct] = i
			logger.Info("[MANUAL SWITCH:%s] %s → %s", clientType, oldEndpoint.Name, ep.Name)
			return nil
		}
	}

	return fmt.Errorf("endpoint '%s' not found or not enabled for client type: %s", targetName, clientType)
}

// ClientFormat represents the API format used by the client
type ClientFormat string

const (
	ClientFormatClaude          ClientFormat = "claude"           // Claude Code: /v1/messages
	ClientFormatOpenAIChat      ClientFormat = "openai_chat"      // Codex (chat): /v1/chat/completions
	ClientFormatOpenAIResponses ClientFormat = "openai_responses" // Codex (responses): /v1/responses
)

// ClientType represents the client category for endpoint grouping
type ClientType string

const (
	ClientTypeClaude ClientType = "claude" // Claude Code client
	ClientTypeGemini ClientType = "gemini" // Gemini client
	ClientTypeCodex  ClientType = "codex"  // Codex CLI client
)

// detectClientFormat identifies the client format based on request path (legacy support)
func detectClientFormat(path string) ClientFormat {
	switch {
	case strings.HasPrefix(path, "/v1/chat/completions") || strings.HasPrefix(path, "/chat/completions"):
		return ClientFormatOpenAIChat
	case strings.HasPrefix(path, "/v1/responses") || strings.HasPrefix(path, "/responses"):
		return ClientFormatOpenAIResponses
	default:
		return ClientFormatClaude
	}
}

// extractClientAndFormat parses the request path to determine client type and format
// New paths: /claude/..., /gemini/..., /codex/...
// Legacy paths (backward compatible): /v1/messages, /v1/chat/completions, /v1/responses
func extractClientAndFormat(path string) (ClientType, ClientFormat, string) {
	// New routing: /{client}/...
	if strings.HasPrefix(path, "/claude/") {
		subPath := strings.TrimPrefix(path, "/claude")
		return ClientTypeClaude, ClientFormatClaude, subPath
	}
	if strings.HasPrefix(path, "/gemini/") {
		subPath := strings.TrimPrefix(path, "/gemini")
		return ClientTypeGemini, ClientFormatClaude, subPath // Gemini uses Claude format from client
	}
	if strings.HasPrefix(path, "/codex/") {
		subPath := strings.TrimPrefix(path, "/codex")
		if strings.HasPrefix(subPath, "/v1/chat/completions") || strings.HasPrefix(subPath, "/chat/completions") {
			return ClientTypeCodex, ClientFormatOpenAIChat, subPath
		}
		if strings.HasPrefix(subPath, "/v1/responses") || strings.HasPrefix(subPath, "/responses") {
			return ClientTypeCodex, ClientFormatOpenAIResponses, subPath
		}
		// Default to chat format for codex
		return ClientTypeCodex, ClientFormatOpenAIChat, subPath
	}

	// Legacy routing (backward compatible) - defaults to claude client type
	format := detectClientFormat(path)
	return ClientTypeClaude, format, path
}

// getClientIP extracts the real client IP from the request
// It handles reverse proxy scenarios by checking X-Forwarded-For header
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for reverse proxy scenarios)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs: client, proxy1, proxy2...
		// The first IP is the original client
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// Check X-Real-IP header (common in nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	// RemoteAddr is in the form "IP:port", need to extract IP
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// handleProxy handles the main proxy logic
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request) {
	// Capture request start time for duration tracking
	requestStartTime := time.Now()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Extract client type and format from path
	clientType, clientFormat, _ := extractClientAndFormat(r.URL.Path)

	// Extract client IP address
	clientIP := getClientIP(r)

	// 提取会话ID（用于会话亲和性）
	var sessionID string
	if p.sessionAffinity != nil {
		sessionID = p.sessionAffinity.ExtractSessionID(r)
		logger.Debug("[REQUEST] Session ID: %s", sessionID)
	}

	logger.DebugLog("=== Proxy Request ===")
	logger.DebugLog("Method: %s, Path: %s, ClientType: %s, ClientFormat: %s", r.Method, r.URL.Path, clientType, clientFormat)
	logger.DebugLog("Request Body: %s", string(bodyBytes))

	// Create interaction record for logging
	var interactionRecord *interaction.Record
	if p.interactionStorage != nil && p.interactionStorage.IsEnabled() {
		var rawReq interface{}
		json.Unmarshal(bodyBytes, &rawReq)

		interactionRecord = &interaction.Record{
			RequestID: interaction.GenerateRequestID(),
			Timestamp: requestStartTime,
			Client: interaction.ClientInfo{
				Type:   string(clientType),
				Format: string(clientFormat),
				IP:     clientIP,
			},
			Request: interaction.RequestData{
				Path: r.URL.Path,
				Raw:  rawReq,
			},
		}
	}

	var streamReq struct {
		Model    string      `json:"model"`
		Thinking interface{} `json:"thinking"`
		Stream   bool        `json:"stream"`
	}
	json.Unmarshal(bodyBytes, &streamReq)

	// Set model in interaction record
	if interactionRecord != nil {
		interactionRecord.Request.Model = streamReq.Model
	}

	// 缓存检查（仅对非流式请求启用缓存）
	// 流式请求不缓存，因为需要实时返回数据
	if !streamReq.Stream && p.cache.IsEnabled() {
		cacheKey := cache.GenerateKey(bodyBytes)
		if entry, found := p.cache.Get(cacheKey); found {
			logger.Debug("[CACHE] Serving cached response for key: %s", cacheKey[:16])
			// 返回缓存的响应
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-CCNexus-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			w.Write(entry.Response)
			return
		}
	}

	// 速率限制检查（在端点选择之前）
	// 测试请求不受速率限制
	specifiedEndpoint := r.Header.Get("X-CCNexus-Endpoint")
	if specifiedEndpoint == "" && p.rateLimiter.IsEnabled() {
		// 先获取当前端点名称用于检查
		currentEndpoint := p.getCurrentEndpointForClient(clientType)
		if currentEndpoint.Name != "" {
			allowed, waitTime := p.rateLimiter.Allow(currentEndpoint.Name)
			if !allowed {
				logger.Warn("[RATELIMIT] Request rejected, wait: %v", waitTime)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", waitTime.Seconds()))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"type":    "rate_limit_error",
						"message": fmt.Sprintf("Rate limit exceeded. Please retry after %.0f seconds.", waitTime.Seconds()),
					},
				})
				return
			}
		}
	}

	// Check if a specific endpoint is requested via header (used for testing)
	// This must be checked BEFORE the enabled endpoints check, because test requests
	// can target disabled endpoints
	var fixedEndpoint *config.Endpoint
	if specifiedEndpoint != "" {
		// For test requests, search in ALL endpoints (including disabled ones)
		allEndpoints := p.config.GetEndpointsByClient(string(clientType))
		for _, ep := range allEndpoints {
			if ep.Name == specifiedEndpoint {
				epCopy := ep
				fixedEndpoint = &epCopy
				break
			}
		}
		if fixedEndpoint == nil {
			http.Error(w, fmt.Sprintf("Specified endpoint '%s' not found for client type: %s", specifiedEndpoint, clientType), http.StatusBadRequest)
			return
		}
	}

	endpoints := p.getEnabledEndpointsForClient(clientType)
	// Only check for enabled endpoints if this is NOT a test request
	if fixedEndpoint == nil && len(endpoints) == 0 {
		logger.Error("No enabled endpoints available for client type: %s", clientType)
		http.Error(w, fmt.Sprintf("No enabled endpoints configured for client type: %s", clientType), http.StatusServiceUnavailable)
		return
	}

	maxRetries := len(endpoints) * 2
	endpointAttempts := 0
	lastEndpointName := ""

	if fixedEndpoint != nil {
		// For test requests, use reduced retry count
		maxRetries = 3
		logger.Debug("[TEST:%s] Using fixed endpoint: %s (max retries: %d)", clientType, specifiedEndpoint, maxRetries)
		// Mark as test request in interaction record
		if interactionRecord != nil {
			interactionRecord.Stats.RequestType = "test"
		}
	}

	var lastError string // Track the last error message for better error reporting

	for retry := 0; retry < maxRetries; retry++ {
		var endpoint config.Endpoint
		if fixedEndpoint != nil {
			endpoint = *fixedEndpoint
		} else {
			// 使用智能路由选择端点（如果启用），传递会话ID
			endpoint = p.selectEndpointForRequest(clientType, streamReq.Model, sessionID)
		}
		if endpoint.Name == "" {
			http.Error(w, fmt.Sprintf("No enabled endpoints available for client type: %s", clientType), http.StatusServiceUnavailable)
			return
		}

		// Reset attempts counter if endpoint changed (e.g., manual switch)
		if lastEndpointName != "" && lastEndpointName != endpoint.Name {
			endpointAttempts = 0
		}
		lastEndpointName = endpoint.Name

		endpointAttempts++
		p.markRequestActive(endpoint.Name)
		p.stats.RecordRequest(endpoint.Name, string(clientType))

		// Start monitoring this request attempt
		monitorReqID := generateMonitorRequestID()
		messagePreview := ExtractMessagePreview(bodyBytes, 300)
		p.monitor.StartRequest(monitorReqID, endpoint.Name, string(clientType), streamReq.Model, messagePreview)

		// Log request attempt with test indication if applicable
		if fixedEndpoint != nil {
			logger.Debug("[TEST:%s][%s] Testing endpoint (attempt %d/%d)", clientType, endpoint.Name, endpointAttempts, maxRetries)
		}

		trans, err := prepareTransformerForClient(clientFormat, endpoint)
		if err != nil {
			lastError = fmt.Sprintf("[%s] %v", endpoint.Name, err)
			logger.Error("[%s:%s] %v", clientType, endpoint.Name, err)
			p.stats.RecordError(endpoint.Name, string(clientType))
			p.monitor.CompleteRequest(monitorReqID, false, err.Error())
			p.markRequestInactive(endpoint.Name)
			if p.handleEndpointRotation(fixedEndpoint, clientType, endpoint, endpointAttempts) {
				endpointAttempts = 0
			}
			continue
		}

		transformerName := trans.Name()

		transformedBody, err := trans.TransformRequest(bodyBytes)
		if err != nil {
			lastError = fmt.Sprintf("[%s] Failed to transform request: %v", endpoint.Name, err)
			logger.Error("[%s:%s] Failed to transform request: %v", clientType, endpoint.Name, err)
			p.stats.RecordError(endpoint.Name, string(clientType))
			p.monitor.CompleteRequest(monitorReqID, false, err.Error())
			p.markRequestInactive(endpoint.Name)
			if p.handleEndpointRotation(fixedEndpoint, clientType, endpoint, endpointAttempts) {
				endpointAttempts = 0
			}
			continue
		}

		logger.DebugLog("[%s] Transformer: %s", endpoint.Name, transformerName)
		logger.DebugLog("[%s] Transformed Request: %s", endpoint.Name, string(transformedBody))

		// Record transformed request in interaction record
		if interactionRecord != nil {
			var transformedReq interface{}
			json.Unmarshal(transformedBody, &transformedReq)
			interactionRecord.Request.Transformed = transformedReq
			interactionRecord.Endpoint = interaction.EndpointInfo{
				Name:        endpoint.Name,
				Transformer: transformerName,
			}
		}

		cleanedBody, err := cleanIncompleteToolCalls(transformedBody)
		if err != nil {
			logger.Warn("[%s] Failed to clean tool calls: %v", endpoint.Name, err)
			cleanedBody = transformedBody
		}
		transformedBody = cleanedBody

		var thinkingEnabled bool
		if strings.Contains(transformerName, "openai") {
			var openaiReq map[string]interface{}
			if err := json.Unmarshal(transformedBody, &openaiReq); err == nil {
				if enable, ok := openaiReq["enable_thinking"].(bool); ok {
					thinkingEnabled = enable
				}
			}
		}

		proxyReq, err := buildProxyRequest(r, endpoint, transformedBody, transformerName)
		if err != nil {
			lastError = fmt.Sprintf("[%s] Failed to create request: %v", endpoint.Name, err)
			logger.Error("[%s:%s] Failed to create request: %v (URL: %s)", clientType, endpoint.Name, err, endpoint.APIUrl)
			p.stats.RecordError(endpoint.Name, string(clientType))
			p.monitor.CompleteRequest(monitorReqID, false, err.Error())
			p.markRequestInactive(endpoint.Name)
			if p.handleEndpointRotation(fixedEndpoint, clientType, endpoint, endpointAttempts) {
				endpointAttempts = 0
			}
			continue
		}

		// Update monitor phase to sending
		p.monitor.UpdatePhase(monitorReqID, PhaseSending)

		ctx := p.getEndpointContext(endpoint.Name)
		resp, err := sendRequest(ctx, proxyReq, p.config)
		if err != nil {
			lastError = fmt.Sprintf("[%s] Request failed: %v", endpoint.Name, err)
			logger.Error("[%s:%s] Request failed: %v (URL: %s, Model: %s)", clientType, endpoint.Name, err, endpoint.APIUrl, streamReq.Model)
			p.stats.RecordError(endpoint.Name, string(clientType))
			p.monitor.CompleteRequest(monitorReqID, false, err.Error())
			p.markRequestInactive(endpoint.Name)
			if p.handleEndpointRotation(fixedEndpoint, clientType, endpoint, endpointAttempts) {
				endpointAttempts = 0
			}
			continue
		}

		contentType := resp.Header.Get("Content-Type")
		isStreaming := contentType == "text/event-stream" || (streamReq.Stream && strings.Contains(contentType, "text/event-stream"))

		if resp.StatusCode == http.StatusOK && isStreaming {
			// Update monitor phase to streaming
			p.monitor.UpdatePhase(monitorReqID, PhaseStreaming)

			usage, outputText, rawEvents, transformedEvents, streamErr := p.handleStreamingResponse(w, resp, endpoint, trans, transformerName, thinkingEnabled, streamReq.Model, bodyBytes, clientType)

			// Handle retryable streaming errors (before response headers sent)
			if errors.Is(streamErr, ErrStreamRetryable) {
				logger.Warn("[%s:%s] Streaming failed before response sent, will retry: %v", clientType, endpoint.Name, streamErr)
				p.stats.RecordError(endpoint.Name, string(clientType))
				p.monitor.CompleteRequest(monitorReqID, false, streamErr.Error())
				p.markRequestInactive(endpoint.Name)
				// endpointAttempts already incremented at loop start (line 487)
				if p.handleEndpointRotation(fixedEndpoint, clientType, endpoint, endpointAttempts) {
					endpointAttempts = 0
				}
				continue // Retry with same or different endpoint
			}

			// Fallback: estimate tokens when usage is 0
			if usage.TotalInputTokens() == 0 || usage.OutputTokens == 0 {
				if usage.TotalInputTokens() == 0 {
					usage.InputTokens = p.estimateInputTokens(bodyBytes)
				}
				if usage.OutputTokens == 0 {
					usage.OutputTokens = p.estimateOutputTokens(outputText)
				}
			}

			// Record daily aggregated stats
			p.stats.RecordTokens(endpoint.Name, string(clientType), usage)

			// Handle non-retryable streaming errors (after response headers sent)
			if streamErr != nil {
				logger.Warn("[%s] 流式传输异常结束: %v", endpoint.Name, streamErr)
				p.stats.RecordError(endpoint.Name, string(clientType))
				durationMs := time.Since(requestStartTime).Milliseconds()

				// Limit error message to 500 characters
				errorMsg := streamErr.Error()
				if len(errorMsg) > 500 {
					errorMsg = errorMsg[:500]
				}

				p.stats.RecordRequestStat(&RequestStatRecord{
					EndpointName:        endpoint.Name,
					ClientType:          string(clientType),
					ClientIP:            clientIP,
					Timestamp:           time.Now(),
					InputTokens:         usage.InputTokens,
					CacheCreationTokens: usage.CacheCreationInputTokens,
					CacheReadTokens:     usage.CacheReadInputTokens,
					OutputTokens:        usage.OutputTokens,
					Model:               streamReq.Model,
					IsStreaming:         true,
					Success:             false,
					DurationMs:          durationMs,
					ErrorMessage:        errorMsg,
				})

				// Save interaction record (with error)
				if interactionRecord != nil {
					interactionRecord.Response = interaction.ResponseData{
						Status:      resp.StatusCode,
						Raw:         rawEvents,
						Transformed: transformedEvents,
					}
					requestType := interactionRecord.Stats.RequestType // preserve RequestType
					interactionRecord.Stats = interaction.StatsData{
						DurationMs:          time.Since(requestStartTime).Milliseconds(),
						IsStreaming:         true,
						InputTokens:         usage.InputTokens,
						CacheCreationTokens: usage.CacheCreationInputTokens,
						CacheReadTokens:     usage.CacheReadInputTokens,
						OutputTokens:        usage.OutputTokens,
						Success:             false,
						ErrorMessage:        streamErr.Error(),
						RequestType:         requestType,
					}
					go p.interactionStorage.Save(interactionRecord)
				}

				p.monitor.CompleteRequest(monitorReqID, false, streamErr.Error())
				p.markRequestInactive(endpoint.Name)
				// Cannot retry: HTTP headers already sent to client
				return
			}

			// Record request-level stats
			durationMs := time.Since(requestStartTime).Milliseconds()
			p.stats.RecordRequestStat(&RequestStatRecord{
				EndpointName:        endpoint.Name,
				ClientType:          string(clientType),
				ClientIP:            clientIP,
				Timestamp:           time.Now(),
				InputTokens:         usage.InputTokens,
				CacheCreationTokens: usage.CacheCreationInputTokens,
				CacheReadTokens:     usage.CacheReadInputTokens,
				OutputTokens:        usage.OutputTokens,
				Model:               streamReq.Model,
				IsStreaming:         true,
				Success:             true,
				DurationMs:          durationMs,
			})

			// Save interaction record (success)
			if interactionRecord != nil {
				interactionRecord.Response = interaction.ResponseData{
					Status:      resp.StatusCode,
					Raw:         rawEvents,
					Transformed: transformedEvents,
				}
				requestType := interactionRecord.Stats.RequestType // preserve RequestType
				interactionRecord.Stats = interaction.StatsData{
					DurationMs:          time.Since(requestStartTime).Milliseconds(),
					IsStreaming:         true,
					InputTokens:         usage.InputTokens,
					CacheCreationTokens: usage.CacheCreationInputTokens,
					CacheReadTokens:     usage.CacheReadInputTokens,
					OutputTokens:        usage.OutputTokens,
					Success:             true,
					RequestType:         requestType,
				}
				go p.interactionStorage.Save(interactionRecord)
			}

			p.monitor.CompleteRequest(monitorReqID, true, "")
			p.markRequestInactive(endpoint.Name)
			// 记录配额使用量（智能路由）
			p.recordQuotaUsage(endpoint.Name, string(clientType), usage)

			// 实际请求成功时，将端点状态设置为可用
			if endpoint.Status != config.EndpointStatusAvailable {
				p.config.SetEndpointStatus(endpoint.Name, string(clientType), config.EndpointStatusAvailable)
				logger.Info("Endpoint %s (client: %s) is now AVAILABLE (via successful request)", endpoint.Name, clientType)
			}

			if p.onEndpointSuccess != nil {
				p.onEndpointSuccess(endpoint.Name, string(clientType))
			}
			logger.Debug("[%s] Request completed successfully (streaming)", endpoint.Name)
			return
		}

		if resp.StatusCode == http.StatusOK {
			usage, rawResp, transformedResp, respBytes, err := p.handleNonStreamingResponse(w, resp, endpoint, trans)
			if err == nil {
				// 缓存成功的非流式响应
				if !streamReq.Stream && p.cache.IsEnabled() {
					cacheKey := cache.GenerateKey(bodyBytes)
					p.cache.Set(cacheKey, respBytes, nil, false)
				}

				// Fallback: estimate tokens when usage is 0
				if usage.TotalInputTokens() == 0 {
					usage.InputTokens = p.estimateInputTokens(bodyBytes)
				}

				// Record daily aggregated stats
				p.stats.RecordTokens(endpoint.Name, string(clientType), usage)

				// Record request-level stats
				// Extract model name from request body
				var reqBody map[string]interface{}
				json.Unmarshal(bodyBytes, &reqBody)
				modelName, _ := reqBody["model"].(string)

				durationMs := time.Since(requestStartTime).Milliseconds()
				p.stats.RecordRequestStat(&RequestStatRecord{
					EndpointName:        endpoint.Name,
					ClientType:          string(clientType),
					ClientIP:            clientIP,
					Timestamp:           time.Now(),
					InputTokens:         usage.InputTokens,
					CacheCreationTokens: usage.CacheCreationInputTokens,
					CacheReadTokens:     usage.CacheReadInputTokens,
					OutputTokens:        usage.OutputTokens,
					Model:               modelName,
					IsStreaming:         false,
					Success:             true,
					DurationMs:          durationMs,
				})

				// Save interaction record (success)
				if interactionRecord != nil {
					interactionRecord.Response = interaction.ResponseData{
						Status:      resp.StatusCode,
						Raw:         rawResp,
						Transformed: transformedResp,
					}
					requestType := interactionRecord.Stats.RequestType // preserve RequestType
					interactionRecord.Stats = interaction.StatsData{
						DurationMs:          time.Since(requestStartTime).Milliseconds(),
						IsStreaming:         false,
						InputTokens:         usage.InputTokens,
						CacheCreationTokens: usage.CacheCreationInputTokens,
						CacheReadTokens:     usage.CacheReadInputTokens,
						OutputTokens:        usage.OutputTokens,
						Success:             true,
						RequestType:         requestType,
					}
					go p.interactionStorage.Save(interactionRecord)
				}

				p.monitor.CompleteRequest(monitorReqID, true, "")
				p.markRequestInactive(endpoint.Name)
				// 记录配额使用量（智能路由）
				p.recordQuotaUsage(endpoint.Name, string(clientType), usage)

				// 实际请求成功时，将端点状态设置为可用
				if endpoint.Status != config.EndpointStatusAvailable {
					p.config.SetEndpointStatus(endpoint.Name, string(clientType), config.EndpointStatusAvailable)
					logger.Info("Endpoint %s (client: %s) is now AVAILABLE (via successful request)", endpoint.Name, clientType)
				}

				if p.onEndpointSuccess != nil {
					p.onEndpointSuccess(endpoint.Name, string(clientType))
				}
				logger.Debug("[%s] Request completed successfully", endpoint.Name)
				return
			}
		}

		if shouldRetry(resp.StatusCode) {
			var errBody []byte
			if resp.Header.Get("Content-Encoding") == "gzip" {
				errBody, _ = decompressGzip(resp.Body)
			} else {
				errBody, _ = io.ReadAll(resp.Body)
			}
			resp.Body.Close()
			errMsg := string(errBody)
			if errMsg == "" {
				errMsg = "(empty response body)"
			} else if len(errMsg) > 200 {
				errMsg = errMsg[:200] + "..."
			}
			lastError = fmt.Sprintf("[%s] HTTP %d: %s", endpoint.Name, resp.StatusCode, errMsg)
			logger.Warn("[%s:%s] Request failed %d: %s (URL: %s, Model: %s)", clientType, endpoint.Name, resp.StatusCode, errMsg, endpoint.APIUrl, streamReq.Model)
			logger.DebugLog("[%s:%s] Request failed %d: %s (URL: %s, Model: %s)", clientType, endpoint.Name, resp.StatusCode, errMsg, endpoint.APIUrl, streamReq.Model)
			p.stats.RecordError(endpoint.Name, string(clientType))
			p.monitor.CompleteRequest(monitorReqID, false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, errMsg))
			p.markRequestInactive(endpoint.Name)
			if p.handleEndpointRotation(fixedEndpoint, clientType, endpoint, endpointAttempts) {
				endpointAttempts = 0
			}
			continue
		}

		var respBody []byte
		if resp.Header.Get("Content-Encoding") == "gzip" {
			respBody, _ = decompressGzip(resp.Body)
		} else {
			respBody, _ = io.ReadAll(resp.Body)
		}
		resp.Body.Close()
		// Complete monitoring - this is a pass-through response (non-retryable)
		if resp.StatusCode == http.StatusOK {
			p.monitor.CompleteRequest(monitorReqID, true, "")
		} else {
			p.monitor.CompleteRequest(monitorReqID, false, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
		p.markRequestInactive(endpoint.Name)
		// Log non-200 responses for debugging
		if resp.StatusCode != http.StatusOK {
			errMsg := string(respBody)
			if errMsg == "" {
				errMsg = "(empty response body)"
			} else if len(errMsg) > 500 {
				errMsg = errMsg[:500] + "..."
			}
			logger.Warn("[%s] Response %d: %s (URL: %s, Model: %s)", endpoint.Name, resp.StatusCode, errMsg, endpoint.APIUrl, streamReq.Model)
			logger.DebugLog("[%s] Response %d: %s (URL: %s, Model: %s)", endpoint.Name, resp.StatusCode, errMsg, endpoint.APIUrl, streamReq.Model)
		}
		// Remove Content-Encoding header since we've decompressed
		for key, values := range resp.Header {
			if key == "Content-Encoding" || key == "Content-Length" {
				continue
			}
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	// Save interaction record for failed requests
	if interactionRecord != nil {
		interactionRecord.Stats.Success = false
		if lastError != "" {
			interactionRecord.Stats.ErrorMessage = lastError
		} else {
			interactionRecord.Stats.ErrorMessage = "All endpoints failed"
		}
		interactionRecord.Stats.DurationMs = time.Since(requestStartTime).Milliseconds()
		go p.interactionStorage.Save(interactionRecord)
	}

	// Return detailed error message
	errorMsg := "All endpoints failed"
	if lastError != "" {
		errorMsg = lastError
	}
	http.Error(w, errorMsg, http.StatusServiceUnavailable)
}

// handleEndpointRotation handles endpoint rotation logic for retry scenarios
// Returns true if rotation occurred, false if endpoint was fixed (test mode)
func (p *Proxy) handleEndpointRotation(fixedEndpoint *config.Endpoint, clientType ClientType, endpoint config.Endpoint, attempts int) bool {
	if attempts < 2 {
		return false
	}

	// 请求失败时，将 untested 状态的端点标记为 unavailable
	if endpoint.Status == config.EndpointStatusUntested {
		p.config.SetEndpointStatus(endpoint.Name, string(clientType), config.EndpointStatusUnavailable)
		logger.Info("Endpoint %s (client: %s) marked as UNAVAILABLE after failed attempt", endpoint.Name, clientType)
	}

	if fixedEndpoint == nil {
		// Normal mode: rotate to next endpoint
		p.rotateEndpointForClient(clientType)
		return true
	} else {
		// Test mode: endpoint is fixed, don't rotate
		logger.Warn("[TEST:%s][%s] Test failed after %d attempts (endpoint fixed, not rotating)",
			clientType, endpoint.Name, attempts)
		return false
	}
}
