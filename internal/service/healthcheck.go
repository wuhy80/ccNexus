package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/logger"
	"github.com/lich0821/ccNexus/internal/proxy"
)

// HealthCheckService handles periodic health checks for all enabled endpoints
type HealthCheckService struct {
	config  *config.Config
	monitor *proxy.Monitor

	mu       sync.Mutex
	ticker   *time.Ticker
	stopChan chan struct{}
	running  bool

	// HTTP client cache
	clientCache *httpClientCache
}

// NewHealthCheckService creates a new HealthCheckService
func NewHealthCheckService(cfg *config.Config, monitor *proxy.Monitor) *HealthCheckService {
	return &HealthCheckService{
		config:  cfg,
		monitor: monitor,
		clientCache: &httpClientCache{
			clients: make(map[time.Duration]*http.Client),
		},
	}
}

// Start starts the health check service with the configured interval
// If interval is 0, health checks are disabled
func (h *HealthCheckService) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return
	}

	interval := h.config.GetHealthCheckInterval()
	if interval <= 0 {
		logger.Info("Health check disabled (interval=0)")
		return
	}

	h.stopChan = make(chan struct{})
	h.ticker = time.NewTicker(time.Duration(interval) * time.Second)
	h.running = true

	logger.Info("Health check service started with interval %d seconds", interval)

	go h.run()
}

// Stop stops the health check service
func (h *HealthCheckService) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	if h.ticker != nil {
		h.ticker.Stop()
	}
	close(h.stopChan)
	h.running = false

	logger.Info("Health check service stopped")
}

// Restart restarts the health check service with the new interval
func (h *HealthCheckService) Restart() {
	h.Stop()
	h.Start()
}

// IsRunning returns whether the health check service is running
func (h *HealthCheckService) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// run is the main loop for health checks
func (h *HealthCheckService) run() {
	// Run immediately on start
	h.checkAllEndpoints()

	for {
		select {
		case <-h.ticker.C:
			h.checkAllEndpoints()
		case <-h.stopChan:
			return
		}
	}
}

// checkAllEndpoints checks all enabled endpoints
func (h *HealthCheckService) checkAllEndpoints() {
	endpoints := h.config.GetEndpoints()

	var wg sync.WaitGroup
	for _, ep := range endpoints {
		if !ep.Enabled {
			continue
		}

		wg.Add(1)
		go func(endpoint config.Endpoint) {
			defer wg.Done()
			h.checkEndpoint(endpoint)
		}(ep)
	}
	wg.Wait()
}

// checkEndpoint checks a single endpoint and records the latency
func (h *HealthCheckService) checkEndpoint(endpoint config.Endpoint) {
	transformer := endpoint.Transformer
	if transformer == "" {
		transformer = "claude"
	}

	normalizedURL := normalizeAPIUrlWithScheme(endpoint.APIUrl)

	start := time.Now()
	statusCode, err := h.testMinimalRequest(normalizedURL, endpoint.APIKey, transformer, endpoint.Model)
	latencyMs := float64(time.Since(start).Milliseconds())

	if err == nil {
		// Success - record latency
		h.monitor.RecordHealthCheckLatency(endpoint.Name, latencyMs)
		logger.Debug("Health check OK for %s: %.0fms", endpoint.Name, latencyMs)
	} else if statusCode == 401 || statusCode == 403 {
		// Auth error - still record latency but log warning
		h.monitor.RecordHealthCheckLatency(endpoint.Name, latencyMs)
		logger.Warn("Health check auth error for %s: HTTP %d (%.0fms)", endpoint.Name, statusCode, latencyMs)
	} else {
		// Other error - clear latency
		h.monitor.ClearHealthCheckLatency(endpoint.Name)
		logger.Warn("Health check failed for %s: %v", endpoint.Name, err)
	}
}

// testMinimalRequest sends a minimal request to test if the LLM service is available
// This consumes approximately 1-2 output tokens per check
func (h *HealthCheckService) testMinimalRequest(apiUrl, apiKey, transformer, model string) (int, error) {
	var url string
	var body []byte

	switch transformer {
	case "claude":
		url = fmt.Sprintf("%s/v1/messages", apiUrl)
		if model == "" {
			model = "claude-sonnet-4-5-20250929"
		}
		body, _ = json.Marshal(map[string]interface{}{
			"model":      model,
			"max_tokens": 1,
			"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
		})
	case "openai", "openai2":
		url = fmt.Sprintf("%s/v1/chat/completions", apiUrl)
		if model == "" {
			model = "gpt-4o-mini"
		}
		body, _ = json.Marshal(map[string]interface{}{
			"model":      model,
			"max_tokens": 1,
			"messages":   []map[string]interface{}{{"role": "user", "content": "Hi"}},
		})
	case "gemini":
		if model == "" {
			model = "gemini-2.0-flash"
		}
		url = fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", apiUrl, model, apiKey)
		body, _ = json.Marshal(map[string]interface{}{
			"contents":         []map[string]interface{}{{"parts": []map[string]string{{"text": "Hi"}}}},
			"generationConfig": map[string]int{"maxOutputTokens": 1},
		})
	default:
		return 0, fmt.Errorf("unsupported transformer: %s", transformer)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	switch transformer {
	case "claude":
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "openai", "openai2":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	// gemini uses query parameter, already set in URL
	}

	client := h.getHTTPClient(30 * time.Second) // Longer timeout for actual LLM request
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

// getHTTPClient returns a cached HTTP client or creates a new one
func (h *HealthCheckService) getHTTPClient(timeout time.Duration) *http.Client {
	// Get current proxy URL
	var currentProxyURL string
	if proxyCfg := h.config.GetProxy(); proxyCfg != nil {
		currentProxyURL = proxyCfg.URL
	}

	h.clientCache.mu.RLock()
	// Check if proxy config changed
	if h.clientCache.proxyURL != currentProxyURL {
		h.clientCache.mu.RUnlock()
		h.clientCache.mu.Lock()
		if h.clientCache.proxyURL != currentProxyURL {
			h.clientCache.clients = make(map[time.Duration]*http.Client)
			h.clientCache.proxyURL = currentProxyURL
		}
		h.clientCache.mu.Unlock()
		h.clientCache.mu.RLock()
	}

	if client, ok := h.clientCache.clients[timeout]; ok {
		h.clientCache.mu.RUnlock()
		return client
	}
	h.clientCache.mu.RUnlock()

	h.clientCache.mu.Lock()
	defer h.clientCache.mu.Unlock()

	if client, ok := h.clientCache.clients[timeout]; ok {
		return client
	}

	client := h.createHTTPClient(timeout)
	h.clientCache.clients[timeout] = client
	return client
}

// createHTTPClient creates an HTTP client with optional proxy support
func (h *HealthCheckService) createHTTPClient(timeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}
	if proxyCfg := h.config.GetProxy(); proxyCfg != nil && proxyCfg.URL != "" {
		if transport, err := proxy.CreateProxyTransport(proxyCfg.URL); err == nil {
			client.Transport = transport
		}
	}
	return client
}
