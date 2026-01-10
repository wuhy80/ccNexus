package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/interaction"
	"github.com/lich0821/ccNexus/internal/logger"
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
}

// New creates a new Proxy instance
func New(cfg *config.Config, statsStorage StatsStorage, deviceID string) *Proxy {
	stats := NewStats(statsStorage, deviceID)

	return &Proxy{
		config:              cfg,
		stats:               stats,
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

// GetMonitor returns the monitor instance for real-time request tracking
func (p *Proxy) GetMonitor() *Monitor {
	return p.monitor
}

// Start starts the proxy server
func (p *Proxy) Start() error {
	return p.StartWithMux(nil)
}

// StartWithMux starts the proxy server with an optional custom mux
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

	p.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	logger.Info("ccNexus starting on port %d", port)
	logger.Info("Configured %d endpoints", len(p.config.GetEndpoints()))

	return p.server.ListenAndServe()
}

// Stop stops the proxy server
func (p *Proxy) Stop() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

// getEnabledEndpoints returns only the enabled endpoints
func (p *Proxy) getEnabledEndpoints() []config.Endpoint {
	allEndpoints := p.config.GetEndpoints()
	enabled := make([]config.Endpoint, 0)
	for _, ep := range allEndpoints {
		if ep.Enabled {
			enabled = append(enabled, ep)
		}
	}
	return enabled
}

// getEnabledEndpointsForClient returns enabled endpoints for a specific client type
func (p *Proxy) getEnabledEndpointsForClient(clientType ClientType) []config.Endpoint {
	return p.config.GetEnabledEndpointsByClient(string(clientType))
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

	endpoints := p.getEnabledEndpointsForClient(clientType)
	if len(endpoints) == 0 {
		logger.Error("No enabled endpoints available for client type: %s", clientType)
		http.Error(w, fmt.Sprintf("No enabled endpoints configured for client type: %s", clientType), http.StatusServiceUnavailable)
		return
	}

	maxRetries := len(endpoints) * 2
	endpointAttempts := 0
	lastEndpointName := ""

	// Check if a specific endpoint is requested via header (used for testing)
	specifiedEndpoint := r.Header.Get("X-CCNexus-Endpoint")
	var fixedEndpoint *config.Endpoint
	if specifiedEndpoint != "" {
		for _, ep := range endpoints {
			if ep.Name == specifiedEndpoint {
				fixedEndpoint = &ep
				break
			}
		}
		if fixedEndpoint == nil {
			http.Error(w, fmt.Sprintf("Specified endpoint '%s' not found or not enabled for client type: %s", specifiedEndpoint, clientType), http.StatusBadRequest)
			return
		}
		// For test requests, use reduced retry count
		maxRetries = 3
		logger.Debug("[TEST:%s] Using fixed endpoint: %s (max retries: %d)", clientType, specifiedEndpoint, maxRetries)
		// Mark as test request in interaction record
		if interactionRecord != nil {
			interactionRecord.Stats.RequestType = "test"
		}
	}

	for retry := 0; retry < maxRetries; retry++ {
		var endpoint config.Endpoint
		if fixedEndpoint != nil {
			endpoint = *fixedEndpoint
		} else {
			endpoint = p.getCurrentEndpointForClient(clientType)
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
		p.monitor.StartRequest(monitorReqID, endpoint.Name, string(clientType), streamReq.Model)

		// Log request attempt with test indication if applicable
		if fixedEndpoint != nil {
			logger.Debug("[TEST:%s][%s] Testing endpoint (attempt %d/%d)", clientType, endpoint.Name, endpointAttempts, maxRetries)
		}

		trans, err := prepareTransformerForClient(clientFormat, endpoint)
		if err != nil {
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
			if p.onEndpointSuccess != nil {
				p.onEndpointSuccess(endpoint.Name, string(clientType))
			}
			logger.Debug("[%s] Request completed successfully (streaming)", endpoint.Name)
			return
		}

		if resp.StatusCode == http.StatusOK {
			usage, rawResp, transformedResp, err := p.handleNonStreamingResponse(w, resp, endpoint, trans)
			if err == nil {
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
		interactionRecord.Stats.ErrorMessage = "All endpoints failed"
		interactionRecord.Stats.DurationMs = time.Since(requestStartTime).Milliseconds()
		go p.interactionStorage.Save(interactionRecord)
	}

	http.Error(w, "All endpoints failed", http.StatusServiceUnavailable)
}

// handleEndpointRotation handles endpoint rotation logic for retry scenarios
// Returns true if rotation occurred, false if endpoint was fixed (test mode)
func (p *Proxy) handleEndpointRotation(fixedEndpoint *config.Endpoint, clientType ClientType, endpoint config.Endpoint, attempts int) bool {
	if attempts < 2 {
		return false
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
