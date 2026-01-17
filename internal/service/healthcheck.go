package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/logger"
	"github.com/lich0821/ccNexus/internal/proxy"
	"github.com/lich0821/ccNexus/internal/storage"
)

// AlertEvent 告警事件类型
type AlertEvent struct {
	EndpointName string    // 端点名称
	ClientType   string    // 客户端类型
	AlertType    string    // 告警类型: "failure" 或 "recovery"
	Message      string    // 告警消息
	Timestamp    time.Time // 事件时间
}

// AlertCallback 告警回调函数类型
type AlertCallback func(event AlertEvent)

// endpointAlertState 端点告警状态
type endpointAlertState struct {
	consecutiveFailures  int         // 连续失败次数
	consecutiveSuccesses int         // 连续成功次数（用于自动启用）
	lastAlertTime        time.Time   // 上次告警时间
	wasHealthy           bool        // 上次检测是否健康
	// 性能告警相关
	latencyHistory      []float64   // 延迟历史记录（最近10次）
	lastPerfAlertTime   time.Time   // 上次性能告警时间
}

// HealthCheckService handles periodic health checks for all enabled endpoints
type HealthCheckService struct {
	config  *config.Config
	monitor *proxy.Monitor
	storage storage.Storage

	mu       sync.Mutex
	ticker   *time.Ticker
	stopChan chan struct{}
	running  bool

	// HTTP client cache
	clientCache *httpClientCache

	// Device ID for health history records
	deviceID string

	// 告警相关
	alertStates   map[string]*endpointAlertState // key: endpointName
	alertStatesMu sync.RWMutex
	alertCallback AlertCallback
}

// NewHealthCheckService creates a new HealthCheckService
func NewHealthCheckService(cfg *config.Config, monitor *proxy.Monitor) *HealthCheckService {
	return &HealthCheckService{
		config:  cfg,
		monitor: monitor,
		clientCache: &httpClientCache{
			clients: make(map[time.Duration]*http.Client),
		},
		deviceID:    "default",
		alertStates: make(map[string]*endpointAlertState),
	}
}

// SetAlertCallback 设置告警回调函数
func (h *HealthCheckService) SetAlertCallback(callback AlertCallback) {
	h.alertCallback = callback
}

// SetStorage sets the storage for recording health history
func (h *HealthCheckService) SetStorage(s storage.Storage) {
	h.storage = s
}

// SetDeviceID sets the device ID for health history records
func (h *HealthCheckService) SetDeviceID(deviceID string) {
	h.deviceID = deviceID
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

// checkAllEndpoints checks all non-disabled endpoints
func (h *HealthCheckService) checkAllEndpoints() {
	endpoints := h.config.GetEndpoints()

	var wg sync.WaitGroup
	for _, ep := range endpoints {
		// 跳过禁用的端点
		if ep.Status == config.EndpointStatusDisabled {
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

	clientType := endpoint.ClientType
	if clientType == "" {
		clientType = "claude"
	}

	normalizedURL := normalizeAPIUrlWithScheme(endpoint.APIUrl)

	start := time.Now()
	statusCode, err := h.testMinimalRequest(normalizedURL, endpoint.APIKey, transformer, endpoint.Model)
	latencyMs := float64(time.Since(start).Milliseconds())

	var status string
	var errorMsg string
	isHealthy := false

	if err == nil {
		// Success - record latency
		h.monitor.RecordHealthCheckLatency(endpoint.Name, latencyMs)
		logger.Debug("Health check OK for %s: %.0fms", endpoint.Name, latencyMs)
		status = "healthy"
		isHealthy = true

		// 自动设置为可用状态
		if endpoint.Status != config.EndpointStatusAvailable {
			h.setEndpointAvailable(endpoint.Name, clientType)
		}
	} else if statusCode == 401 || statusCode == 403 {
		// Auth error - still record latency but log warning
		h.monitor.RecordHealthCheckLatency(endpoint.Name, latencyMs)
		logger.Warn("Health check auth error for %s: HTTP %d (%.0fms)", endpoint.Name, statusCode, latencyMs)
		status = "warning"
		errorMsg = fmt.Sprintf("HTTP %d", statusCode)
	} else {
		// Other error - clear latency
		h.monitor.ClearHealthCheckLatency(endpoint.Name)
		logger.Warn("Health check failed for %s: %v", endpoint.Name, err)
		status = "error"
		if err != nil {
			errorMsg = err.Error()
		}

		// 自动设置为不可用状态
		if endpoint.Status == config.EndpointStatusAvailable {
			h.setEndpointUnavailable(endpoint.Name, clientType)
		}
	}

	// 记录检测结果到 Monitor（用于前端显示检测时间）
	h.monitor.RecordCheckResult(endpoint.Name, isHealthy, latencyMs, errorMsg)

	// Record to health history
	h.recordHealthHistory(endpoint.Name, clientType, status, latencyMs, errorMsg)

	// 处理告警逻辑
	h.processAlert(endpoint.Name, clientType, isHealthy, errorMsg)

	// 处理性能告警逻辑（仅在健康时检查）
	if isHealthy {
		h.processPerformanceAlert(endpoint.Name, clientType, latencyMs)
	} else {
		// 失败时重置连续成功计数
		h.alertStatesMu.Lock()
		if state := h.alertStates[endpoint.Name]; state != nil {
			state.consecutiveSuccesses = 0
		}
		h.alertStatesMu.Unlock()
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

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("failed to read response: %v", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		// 尝试解析错误信息
		var errorMsg string
		var errorData map[string]interface{}
		if json.Unmarshal(respBody, &errorData) == nil {
			if errObj, ok := errorData["error"]; ok {
				if errMap, ok := errObj.(map[string]interface{}); ok {
					if msg, ok := errMap["message"].(string); ok {
						errorMsg = msg
					}
				} else if errStr, ok := errObj.(string); ok {
					errorMsg = errStr
				}
			}
		}
		if errorMsg != "" {
			return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errorMsg)
		}
		return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 验证响应内容是否有效（检查是否包含错误）
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return resp.StatusCode, fmt.Errorf("invalid JSON response: %v", err)
	}

	// 检查响应中是否包含错误字段
	if errorField, hasError := respData["error"]; hasError {
		var errorMsg string
		if errMap, ok := errorField.(map[string]interface{}); ok {
			if msg, ok := errMap["message"].(string); ok {
				errorMsg = msg
			} else if msgType, ok := errMap["type"].(string); ok {
				errorMsg = msgType
			}
		} else if errStr, ok := errorField.(string); ok {
			errorMsg = errStr
		}
		if errorMsg != "" {
			return resp.StatusCode, fmt.Errorf("API error: %s", errorMsg)
		}
		return resp.StatusCode, fmt.Errorf("API returned error")
	}

	// 验证响应包含预期的内容字段，并检查内容是否有效
	switch transformer {
	case "claude":
		// Claude 应该返回 content 字段
		content, hasContent := respData["content"]
		if !hasContent {
			return resp.StatusCode, fmt.Errorf("invalid response: missing 'content' field")
		}
		// 检查 content 是否为空数组
		if contentArray, ok := content.([]interface{}); ok {
			if len(contentArray) == 0 {
				return resp.StatusCode, fmt.Errorf("invalid response: empty content array")
			}
		}
		// 检查 stop_reason 是否异常
		if stopReason, ok := respData["stop_reason"].(string); ok {
			if stopReason == "error" {
				return resp.StatusCode, fmt.Errorf("API error: stop_reason is 'error'")
			}
		}

	case "openai", "openai2":
		// OpenAI 应该返回 choices 字段
		choices, hasChoices := respData["choices"]
		if !hasChoices {
			return resp.StatusCode, fmt.Errorf("invalid response: missing 'choices' field")
		}
		// 检查 choices 是否为空数组
		if choicesArray, ok := choices.([]interface{}); ok {
			if len(choicesArray) == 0 {
				return resp.StatusCode, fmt.Errorf("invalid response: empty choices array")
			}
			// 检查第一个 choice 的 finish_reason
			if len(choicesArray) > 0 {
				if choice, ok := choicesArray[0].(map[string]interface{}); ok {
					if finishReason, ok := choice["finish_reason"].(string); ok {
						// content_filter 表示内容被过滤
						if finishReason == "content_filter" {
							return resp.StatusCode, fmt.Errorf("content filtered by safety system")
						}
					}
					// 检查 message 是否存在且有内容
					if message, ok := choice["message"].(map[string]interface{}); ok {
						if content, ok := message["content"].(string); ok {
							if content == "" {
								return resp.StatusCode, fmt.Errorf("invalid response: empty message content")
							}
						}
					}
				}
			}
		}

	case "gemini":
		// Gemini 应该返回 candidates 字段
		candidates, hasCandidates := respData["candidates"]
		if !hasCandidates {
			return resp.StatusCode, fmt.Errorf("invalid response: missing 'candidates' field")
		}
		// 检查 candidates 是否为空数组
		if candidatesArray, ok := candidates.([]interface{}); ok {
			if len(candidatesArray) == 0 {
				return resp.StatusCode, fmt.Errorf("invalid response: empty candidates array")
			}
			// 检查第一个 candidate 的 finishReason
			if len(candidatesArray) > 0 {
				if candidate, ok := candidatesArray[0].(map[string]interface{}); ok {
					if finishReason, ok := candidate["finishReason"].(string); ok {
						// SAFETY 表示被安全过滤器拦截
						if finishReason == "SAFETY" {
							return resp.StatusCode, fmt.Errorf("content blocked by safety filters")
						}
						// RECITATION 表示可能包含受版权保护的内容
						if finishReason == "RECITATION" {
							return resp.StatusCode, fmt.Errorf("content blocked due to recitation")
						}
					}
					// 检查 content 是否存在且有 parts
					if content, ok := candidate["content"].(map[string]interface{}); ok {
						if parts, ok := content["parts"].([]interface{}); ok {
							if len(parts) == 0 {
								return resp.StatusCode, fmt.Errorf("invalid response: empty content parts")
							}
						}
					}
				}
			}
		}
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

// recordHealthHistory records a health check result to the history table
func (h *HealthCheckService) recordHealthHistory(endpointName, clientType, status string, latencyMs float64, errorMsg string) {
	if h.storage == nil {
		return
	}

	record := &storage.HealthHistoryRecord{
		EndpointName: endpointName,
		ClientType:   clientType,
		Status:       status,
		LatencyMs:    latencyMs,
		ErrorMessage: errorMsg,
		Timestamp:    time.Now(),
		DeviceID:     h.deviceID,
	}

	if err := h.storage.RecordHealthHistory(record); err != nil {
		logger.Warn("Failed to record health history for %s: %v", endpointName, err)
	}
}

// CleanupOldHistory removes old health history records based on retention days
func (h *HealthCheckService) CleanupOldHistory() {
	if h.storage == nil {
		return
	}

	days := h.config.GetHealthHistoryRetentionDays()
	if err := h.storage.CleanupOldHealthHistory(days); err != nil {
		logger.Warn("Failed to cleanup old health history: %v", err)
	} else {
		logger.Debug("Cleaned up health history older than %d days", days)
	}
}

// processAlert 处理告警逻辑
func (h *HealthCheckService) processAlert(endpointName, clientType string, isHealthy bool, errorMsg string) {
	alertConfig := h.config.GetAlert()
	if alertConfig == nil || !alertConfig.Enabled {
		return
	}

	if h.alertCallback == nil {
		return
	}

	h.alertStatesMu.Lock()
	defer h.alertStatesMu.Unlock()

	// 获取或创建端点告警状态
	state, exists := h.alertStates[endpointName]
	if !exists {
		state = &endpointAlertState{
			wasHealthy: true, // 假设初始状态是健康的
		}
		h.alertStates[endpointName] = state
	}

	now := time.Now()
	cooldownDuration := time.Duration(alertConfig.AlertCooldownMinutes) * time.Minute
	if cooldownDuration == 0 {
		cooldownDuration = 5 * time.Minute // 默认5分钟冷却
	}

	consecutiveThreshold := alertConfig.ConsecutiveFailures
	if consecutiveThreshold <= 0 {
		consecutiveThreshold = 3 // 默认连续3次失败触发告警
	}

	if isHealthy {
		// 端点恢复健康
		if !state.wasHealthy && alertConfig.NotifyOnRecovery {
			// 检查冷却时间
			if now.Sub(state.lastAlertTime) >= cooldownDuration {
				// 发送恢复通知
				event := AlertEvent{
					EndpointName: endpointName,
					ClientType:   clientType,
					AlertType:    "recovery",
					Message:      fmt.Sprintf("端点 %s 已恢复正常", endpointName),
					Timestamp:    now,
				}
				h.alertCallback(event)
				state.lastAlertTime = now
				logger.Info("Alert: endpoint %s recovered", endpointName)
			}
		}
		// 重置失败计数
		state.consecutiveFailures = 0
		state.wasHealthy = true
	} else {
		// 端点故障
		state.consecutiveFailures++
		logger.Debug("Endpoint %s consecutive failures: %d", endpointName, state.consecutiveFailures)

		// 检查是否达到告警阈值
		if state.consecutiveFailures >= consecutiveThreshold {
			// 检查冷却时间
			if now.Sub(state.lastAlertTime) >= cooldownDuration {
				// 发送故障告警
				message := fmt.Sprintf("端点 %s 连续 %d 次健康检测失败", endpointName, state.consecutiveFailures)
				if errorMsg != "" {
					message += fmt.Sprintf("，错误: %s", errorMsg)
				}
				event := AlertEvent{
					EndpointName: endpointName,
					ClientType:   clientType,
					AlertType:    "failure",
					Message:      message,
					Timestamp:    now,
				}
				h.alertCallback(event)
				state.lastAlertTime = now
				logger.Warn("Alert: endpoint %s failed %d times consecutively", endpointName, state.consecutiveFailures)
			}
		}
		state.wasHealthy = false
	}
}

// processPerformanceAlert 处理性能异常告警
func (h *HealthCheckService) processPerformanceAlert(endpointName, clientType string, latencyMs float64) {
	alertConfig := h.config.GetAlert()
	if alertConfig == nil || !alertConfig.PerformanceAlertEnabled {
		return
	}

	if h.alertCallback == nil {
		return
	}

	h.alertStatesMu.Lock()
	defer h.alertStatesMu.Unlock()

	// 获取或创建端点告警状态
	state, exists := h.alertStates[endpointName]
	if !exists {
		state = &endpointAlertState{
			wasHealthy:     true,
			latencyHistory: make([]float64, 0, 10),
		}
		h.alertStates[endpointName] = state
	}

	// 初始化延迟历史
	if state.latencyHistory == nil {
		state.latencyHistory = make([]float64, 0, 10)
	}

	now := time.Now()
	cooldownDuration := time.Duration(alertConfig.AlertCooldownMinutes) * time.Minute
	if cooldownDuration == 0 {
		cooldownDuration = 5 * time.Minute
	}

	// 获取配置的阈值
	latencyThreshold := float64(alertConfig.LatencyThresholdMs)
	if latencyThreshold <= 0 {
		latencyThreshold = 5000 // 默认5秒
	}

	increasePercent := float64(alertConfig.LatencyIncreasePercent)
	if increasePercent <= 0 {
		increasePercent = 200 // 默认200%
	}

	// 计算平均延迟
	var avgLatency float64
	if len(state.latencyHistory) > 0 {
		var sum float64
		for _, l := range state.latencyHistory {
			sum += l
		}
		avgLatency = sum / float64(len(state.latencyHistory))
	}

	// 检查是否需要告警
	shouldAlert := false
	var alertReason string

	// 条件1: 延迟超过绝对阈值
	if latencyMs > latencyThreshold {
		shouldAlert = true
		alertReason = fmt.Sprintf("延迟 %.0fms 超过阈值 %.0fms", latencyMs, latencyThreshold)
	}

	// 条件2: 延迟相比平均值增加超过指定百分比（需要有足够的历史数据）
	if !shouldAlert && len(state.latencyHistory) >= 5 && avgLatency > 0 {
		increaseRatio := (latencyMs - avgLatency) / avgLatency * 100
		if increaseRatio > increasePercent {
			shouldAlert = true
			alertReason = fmt.Sprintf("延迟 %.0fms 相比平均值 %.0fms 增加了 %.0f%%", latencyMs, avgLatency, increaseRatio)
		}
	}

	// 发送告警
	if shouldAlert && now.Sub(state.lastPerfAlertTime) >= cooldownDuration {
		event := AlertEvent{
			EndpointName: endpointName,
			ClientType:   clientType,
			AlertType:    "performance",
			Message:      fmt.Sprintf("端点 %s 性能异常: %s", endpointName, alertReason),
			Timestamp:    now,
		}
		h.alertCallback(event)
		state.lastPerfAlertTime = now
		logger.Warn("Performance alert: endpoint %s - %s", endpointName, alertReason)
	}

	// 更新延迟历史（保留最近10次）
	state.latencyHistory = append(state.latencyHistory, latencyMs)
	if len(state.latencyHistory) > 10 {
		state.latencyHistory = state.latencyHistory[1:]
	}
}

// setEndpointAvailable 设置端点为可用状态
func (h *HealthCheckService) setEndpointAvailable(endpointName, clientType string) {
	if err := h.config.SetEndpointStatus(endpointName, clientType, config.EndpointStatusAvailable); err != nil {
		logger.Warn("Failed to set endpoint %s to available: %v", endpointName, err)
		return
	}

	logger.Info("Endpoint %s (client: %s) is now AVAILABLE", endpointName, clientType)

	// 触发恢复通知
	alertConfig := h.config.GetAlert()
	if alertConfig != nil && alertConfig.NotifyOnRecovery && h.alertCallback != nil {
		event := AlertEvent{
			EndpointName: endpointName,
			ClientType:   clientType,
			AlertType:    "recovery",
			Message:      fmt.Sprintf("端点 %s 已恢复可用", endpointName),
			Timestamp:    time.Now(),
		}
		h.alertCallback(event)
	}
}

// setEndpointUnavailable 设置端点为不可用状态
func (h *HealthCheckService) setEndpointUnavailable(endpointName, clientType string) {
	if err := h.config.SetEndpointStatus(endpointName, clientType, config.EndpointStatusUnavailable); err != nil {
		logger.Warn("Failed to set endpoint %s to unavailable: %v", endpointName, err)
		return
	}

	logger.Warn("Endpoint %s (client: %s) is now UNAVAILABLE", endpointName, clientType)
}

// autoEnableEndpoint 自动启用端点（连续成功达到阈值后）
// 已废弃：三状态系统中不再需要此功能
func (h *HealthCheckService) autoEnableEndpoint(endpointName, clientType string) {
	alertConfig := h.config.GetAlert()
	if alertConfig == nil || !alertConfig.AutoEnableOnRecovery {
		return
	}

	h.alertStatesMu.Lock()
	defer h.alertStatesMu.Unlock()

	// 获取或创建端点告警状态
	state, exists := h.alertStates[endpointName]
	if !exists {
		state = &endpointAlertState{
			wasHealthy: true,
		}
		h.alertStates[endpointName] = state
	}

	// 连续成功次数加1
	state.consecutiveSuccesses++

	// 获取阈值（默认3次）
	threshold := alertConfig.AutoEnableSuccessThreshold
	if threshold <= 0 {
		threshold = 3
	}

	// 达到阈值，自动启用端点
	if state.consecutiveSuccesses >= threshold {
		if err := h.config.EnableEndpoint(endpointName, clientType); err != nil {
			logger.Warn("Failed to auto-enable endpoint %s: %v", endpointName, err)
		} else {
			logger.Info("Auto-enabled endpoint %s after %d consecutive successful checks", endpointName, state.consecutiveSuccesses)

			// 触发告警回调通知前端
			if h.alertCallback != nil {
				event := AlertEvent{
					EndpointName: endpointName,
					ClientType:   clientType,
					AlertType:    "auto-enabled",
					Message:      fmt.Sprintf("端点 %s 已自动启用（连续 %d 次检测成功）", endpointName, state.consecutiveSuccesses),
					Timestamp:    time.Now(),
				}
				h.alertCallback(event)
			}
		}
		// 重置计数
		state.consecutiveSuccesses = 0
	}
}
