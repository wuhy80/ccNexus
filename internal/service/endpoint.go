package service

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/lich0821/ccNexus/internal/config"
    "github.com/lich0821/ccNexus/internal/logger"
    "github.com/lich0821/ccNexus/internal/proxy"
    "github.com/lich0821/ccNexus/internal/storage"
)

// getHTTPClient returns a cached HTTP client or creates a new one if needed
func (e *EndpointService) getHTTPClient(timeout time.Duration) *http.Client {
    // Get current proxy URL
    var currentProxyURL string
    if proxyCfg := e.config.GetProxy(); proxyCfg != nil {
        currentProxyURL = proxyCfg.URL
    }

    e.clientCache.mu.RLock()
    // Check if proxy config changed - if so, we need to invalidate cache
    if e.clientCache.proxyURL != currentProxyURL {
        e.clientCache.mu.RUnlock()
        e.clientCache.mu.Lock()
        // Double-check after acquiring write lock
        if e.clientCache.proxyURL != currentProxyURL {
            e.clientCache.clients = make(map[time.Duration]*http.Client)
            e.clientCache.proxyURL = currentProxyURL
        }
        e.clientCache.mu.Unlock()
        e.clientCache.mu.RLock()
    }

    // Check if we have a cached client
    if client, ok := e.clientCache.clients[timeout]; ok {
        e.clientCache.mu.RUnlock()
        return client
    }
    e.clientCache.mu.RUnlock()

    // Create new client
    e.clientCache.mu.Lock()
    defer e.clientCache.mu.Unlock()

    // Double-check after acquiring write lock
    if client, ok := e.clientCache.clients[timeout]; ok {
        return client
    }

    client := e.createHTTPClient(timeout)
    e.clientCache.clients[timeout] = client
    return client
}

// createHTTPClient creates an HTTP client with optional proxy support
func (e *EndpointService) createHTTPClient(timeout time.Duration) *http.Client {
    client := &http.Client{Timeout: timeout}
    if proxyCfg := e.config.GetProxy(); proxyCfg != nil && proxyCfg.URL != "" {
        if transport, err := proxy.CreateProxyTransport(proxyCfg.URL); err == nil {
            client.Transport = transport
        }
    }
    return client
}

// Test endpoint constants
const (
    testMessage   = "你是什么模型?"
    testMaxTokens = 16
)

// httpClientCache holds cached HTTP clients by timeout duration
type httpClientCache struct {
    clients  map[time.Duration]*http.Client
    proxyURL string // track proxy URL to invalidate cache when it changes
    mu       sync.RWMutex
}

// EndpointService handles endpoint management operations
type EndpointService struct {
    config      *config.Config
    proxy       *proxy.Proxy
    storage     *storage.SQLiteStorage
    clientCache *httpClientCache
}

// NewEndpointService creates a new EndpointService
func NewEndpointService(cfg *config.Config, p *proxy.Proxy, s *storage.SQLiteStorage) *EndpointService {
    return &EndpointService{
        config:  cfg,
        proxy:   p,
        storage: s,
        clientCache: &httpClientCache{
            clients: make(map[time.Duration]*http.Client),
        },
    }
}

// normalizeAPIUrl ensures the API URL has the correct format
func normalizeAPIUrl(apiUrl string) string {
    return strings.TrimSuffix(apiUrl, "/")
}

// AddEndpoint adds a new endpoint for a specific client type
func (e *EndpointService) AddEndpoint(clientType, name, apiUrl, apiKey, transformer, model, remark, tags string,
    modelPatterns string, costPerInputToken, costPerOutputToken float64, quotaLimit int64, quotaResetCycle string, priority int) error {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)
    for _, ep := range endpoints {
        if ep.Name == name {
            return fmt.Errorf("endpoint name '%s' already exists for client type '%s'", name, clientType)
        }
    }

    transformer = normalizeTransformer(transformer)

    apiUrl = normalizeAPIUrl(apiUrl)

    // 默认优先级
    if priority <= 0 {
        priority = 100
    }

    newEndpoint := config.Endpoint{
        Name:               name,
        ClientType:         clientType,
        APIUrl:             apiUrl,
        APIKey:             apiKey,
        Enabled:            true,
        Transformer:        transformer,
        Model:              model,
        Remark:             remark,
        Tags:               tags,
        ModelPatterns:      modelPatterns,
        CostPerInputToken:  costPerInputToken,
        CostPerOutputToken: costPerOutputToken,
        QuotaLimit:         quotaLimit,
        QuotaResetCycle:    quotaResetCycle,
        Priority:           priority,
    }

    // Get all endpoints and add the new one
    allEndpoints := e.config.GetEndpoints()
    allEndpoints = append(allEndpoints, newEndpoint)
    e.config.UpdateEndpoints(allEndpoints)

    if err := e.config.Validate(); err != nil {
        return err
    }

    if err := e.proxy.UpdateConfig(e.config); err != nil {
        return err
    }

    if e.storage != nil {
        configAdapter := storage.NewConfigStorageAdapter(e.storage)
        if err := e.config.SaveToStorage(configAdapter); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
    }

    if model != "" {
        logger.Info("Endpoint added: %s (%s) [%s/%s] for client %s", name, apiUrl, transformer, model, clientType)
    } else {
        logger.Info("Endpoint added: %s (%s) [%s] for client %s", name, apiUrl, transformer, clientType)
    }

    return nil
}

// RemoveEndpoint removes an endpoint by index for a specific client type
func (e *EndpointService) RemoveEndpoint(clientType string, index int) error {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        return fmt.Errorf("invalid endpoint index: %d", index)
    }

    removedName := endpoints[index].Name

    // Remove from all endpoints
    allEndpoints := e.config.GetEndpoints()
    newEndpoints := make([]config.Endpoint, 0, len(allEndpoints)-1)
    for _, ep := range allEndpoints {
        if !(ep.Name == removedName && ep.ClientType == clientType) {
            newEndpoints = append(newEndpoints, ep)
        }
    }
    e.config.UpdateEndpoints(newEndpoints)

    if len(e.config.GetEndpointsByClient(clientType)) > 0 {
        if err := e.config.Validate(); err != nil {
            return err
        }
    }

    if err := e.proxy.UpdateConfig(e.config); err != nil {
        return err
    }

    if e.storage != nil {
        configAdapter := storage.NewConfigStorageAdapter(e.storage)
        if err := e.config.SaveToStorage(configAdapter); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
    }

    logger.Info("Endpoint removed: %s (client: %s)", removedName, clientType)
    return nil
}

// UpdateEndpoint updates an endpoint by index for a specific client type
func (e *EndpointService) UpdateEndpoint(clientType string, index int, name, apiUrl, apiKey, transformer, model, remark, tags string,
    modelPatterns string, costPerInputToken, costPerOutputToken float64, quotaLimit int64, quotaResetCycle string, priority int) error {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        return fmt.Errorf("invalid endpoint index: %d", index)
    }

    oldName := endpoints[index].Name

    if oldName != name {
        for i, ep := range endpoints {
            if i != index && ep.Name == name {
                return fmt.Errorf("endpoint name '%s' already exists for client type '%s'", name, clientType)
            }
        }
    }

    enabled := endpoints[index].Enabled

    transformer = normalizeTransformer(transformer)

    apiUrl = normalizeAPIUrl(apiUrl)

    // 默认优先级
    if priority <= 0 {
        priority = 100
    }

    updatedEndpoint := config.Endpoint{
        Name:               name,
        ClientType:         clientType,
        APIUrl:             apiUrl,
        APIKey:             apiKey,
        Enabled:            enabled,
        Transformer:        transformer,
        Model:              model,
        Remark:             remark,
        Tags:               tags,
        ModelPatterns:      modelPatterns,
        CostPerInputToken:  costPerInputToken,
        CostPerOutputToken: costPerOutputToken,
        QuotaLimit:         quotaLimit,
        QuotaResetCycle:    quotaResetCycle,
        Priority:           priority,
    }

    // Update in all endpoints
    allEndpoints := e.config.GetEndpoints()
    for i, ep := range allEndpoints {
        if ep.Name == oldName && ep.ClientType == clientType {
            allEndpoints[i] = updatedEndpoint
            break
        }
    }
    e.config.UpdateEndpoints(allEndpoints)

    if err := e.config.Validate(); err != nil {
        return err
    }

    if err := e.proxy.UpdateConfig(e.config); err != nil {
        return err
    }

    if e.storage != nil {
        configAdapter := storage.NewConfigStorageAdapter(e.storage)
        if err := e.config.SaveToStorage(configAdapter); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
    }

    if oldName != name {
        if model != "" {
            logger.Info("Endpoint updated: %s → %s (%s) [%s/%s] for client %s", oldName, name, apiUrl, transformer, model, clientType)
        } else {
            logger.Info("Endpoint updated: %s → %s (%s) [%s] for client %s", oldName, name, apiUrl, transformer, clientType)
        }
    } else {
        if model != "" {
            logger.Info("Endpoint updated: %s (%s) [%s/%s] for client %s", name, apiUrl, transformer, model, clientType)
        } else {
            logger.Info("Endpoint updated: %s (%s) [%s] for client %s", name, apiUrl, transformer, clientType)
        }
    }

    return nil
}

// ToggleEndpoint toggles the enabled/disabled state of an endpoint for a specific client type
// 注意：启用时设置为 unavailable 状态，等待健康检查；禁用时设置为 disabled 状态
func (e *EndpointService) ToggleEndpoint(clientType string, index int, enabled bool) error {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        return fmt.Errorf("invalid endpoint index: %d", index)
    }

    endpointName := endpoints[index].Name

    // 根据 enabled 参数设置状态
    var newStatus config.EndpointStatus
    if enabled {
        // 启用：设置为不可用，等待健康检查
        newStatus = config.EndpointStatusUnavailable
    } else {
        // 禁用
        newStatus = config.EndpointStatusDisabled
    }

    // 更新状态
    if err := e.config.SetEndpointStatus(endpointName, clientType, newStatus); err != nil {
        return err
    }

    // 更新代理配置
    if err := e.proxy.UpdateConfig(e.config); err != nil {
        return err
    }

    // 保存到数据库
    if e.storage != nil {
        configAdapter := storage.NewConfigStorageAdapter(e.storage)
        if err := e.config.SaveToStorage(configAdapter); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
    }

    if enabled {
        logger.Info("Endpoint enabled: %s (client: %s), waiting for health check", endpointName, clientType)
    } else {
        logger.Info("Endpoint disabled: %s (client: %s)", endpointName, clientType)
    }

    return nil
}

// ReorderEndpoints reorders endpoints based on the provided name array for a specific client type
func (e *EndpointService) ReorderEndpoints(clientType string, names []string) error {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)

    if len(names) != len(endpoints) {
        return fmt.Errorf("names array length (%d) doesn't match endpoints count (%d) for client type '%s'", len(names), len(endpoints), clientType)
    }

    seen := make(map[string]bool)
    for _, name := range names {
        if seen[name] {
            return fmt.Errorf("duplicate endpoint name in reorder request: %s", name)
        }
        seen[name] = true
    }

    endpointMap := make(map[string]config.Endpoint)
    for _, ep := range endpoints {
        endpointMap[ep.Name] = ep
    }

    reorderedEndpoints := make([]config.Endpoint, 0, len(names))
    for _, name := range names {
        ep, exists := endpointMap[name]
        if !exists {
            return fmt.Errorf("endpoint not found: %s", name)
        }
        reorderedEndpoints = append(reorderedEndpoints, ep)
    }

    // Rebuild all endpoints: other client types + reordered ones
    allEndpoints := e.config.GetEndpoints()
    newEndpoints := make([]config.Endpoint, 0, len(allEndpoints))
    for _, ep := range allEndpoints {
        if ep.ClientType != clientType {
            newEndpoints = append(newEndpoints, ep)
        }
    }
    newEndpoints = append(newEndpoints, reorderedEndpoints...)
    e.config.UpdateEndpoints(newEndpoints)

    if err := e.config.Validate(); err != nil {
        return err
    }

    if err := e.proxy.UpdateConfig(e.config); err != nil {
        return err
    }

    if e.storage != nil {
        configAdapter := storage.NewConfigStorageAdapter(e.storage)
        if err := e.config.SaveToStorage(configAdapter); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
    }

    logger.Info("Endpoints reordered for client %s: %v", clientType, names)
    return nil
}

// GetCurrentEndpoint returns the current active endpoint name for a specific client type
func (e *EndpointService) GetCurrentEndpoint(clientType string) string {
    if e.proxy == nil {
        return ""
    }
    clientType = normalizeClientType(clientType)
    return e.proxy.GetCurrentEndpointNameForClient(clientType)
}

// SwitchToEndpoint manually switches to a specific endpoint by name for a specific client type
func (e *EndpointService) SwitchToEndpoint(clientType, endpointName string) error {
    if e.proxy == nil {
        return fmt.Errorf("proxy not initialized")
    }
    clientType = normalizeClientType(clientType)
    return e.proxy.SetCurrentEndpointForClient(clientType, endpointName)
}

// TestEndpoint tests an endpoint by sending a simple request for a specific client type
func (e *EndpointService) TestEndpoint(clientType string, index int) string {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        return errorJSON(fmt.Sprintf("Invalid endpoint index: %d", index))
    }

    endpoint := endpoints[index]
    logger.Info("Testing endpoint: %s (%s)", endpoint.Name, endpoint.APIUrl)

    var requestBody []byte
    var err error
    var apiPath string

    transformer := endpoint.Transformer
    transformer = normalizeTransformer(transformer)

    switch transformer {
    case "claude":
        apiPath = "/v1/messages"
        model := endpoint.Model
        if model == "" {
            model = "claude-sonnet-4-5-20250929"
        }
        requestBody, err = json.Marshal(map[string]interface{}{
            "model":      model,
            "max_tokens": testMaxTokens,
            "messages": []map[string]string{
                {"role": "user", "content": testMessage},
            },
        })

    case "openai":
        apiPath = "/v1/chat/completions"
        model := endpoint.Model
        if model == "" {
            model = "gpt-4-turbo"
        }
        requestBody, err = json.Marshal(map[string]interface{}{
            "model":      model,
            "max_tokens": testMaxTokens,
            "messages": []map[string]interface{}{
                {"role": "user", "content": testMessage},
            },
        })

    case "openai2":
        apiPath = "/v1/responses"
        model := endpoint.Model
        if model == "" {
            model = "gpt-5-codex"
        }
        requestBody, err = json.Marshal(map[string]interface{}{
            "model": model,
            "input": []map[string]interface{}{
                {
                    "type": "message",
                    "role": "user",
                    "content": []map[string]interface{}{
                        {"type": "input_text", "text": testMessage},
                    },
                },
            },
        })

    case "gemini":
        // Gemini uses its native API format directly
        model := endpoint.Model
        if model == "" {
            model = "gemini-2.0-flash"
        }
        apiPath = fmt.Sprintf("/v1beta/models/%s:generateContent", model)
        requestBody, err = json.Marshal(map[string]interface{}{
            "contents": []map[string]interface{}{
                {
                    "parts": []map[string]string{
                        {"text": testMessage},
                    },
                },
            },
            "generationConfig": map[string]int{
                "maxOutputTokens": testMaxTokens,
            },
        })

    default:
        return errorJSON(fmt.Sprintf("Unsupported transformer: %s", transformer))
    }

    if err != nil {
        return errorJSON(fmt.Sprintf("Failed to build request: %v", err))
    }

    // 直接发送请求到目标API
    normalizedURL := normalizeAPIUrlWithScheme(endpoint.APIUrl)
    var url string
    if transformer == "gemini" {
        url = fmt.Sprintf("%s%s?key=%s", normalizedURL, apiPath, endpoint.APIKey)
    } else {
        url = fmt.Sprintf("%s%s", normalizedURL, apiPath)
    }

    req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody))
    if err != nil {
        return errorJSON(fmt.Sprintf("Failed to create request: %v", err))
    }

    req.Header.Set("Content-Type", "application/json")
    switch transformer {
    case "claude":
        req.Header.Set("x-api-key", endpoint.APIKey)
        req.Header.Set("anthropic-version", "2023-06-01")
    case "openai", "openai2":
        req.Header.Set("Authorization", "Bearer "+endpoint.APIKey)
    // gemini uses query parameter, already set in URL
    }

    client := e.getHTTPClient(30 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        logger.Error("Test failed for %s: %v", endpoint.Name, err)
        return errorJSON(fmt.Sprintf("Request failed: %v", err))
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return errorJSON(fmt.Sprintf("Failed to read response: %v", err))
    }

    if resp.StatusCode != http.StatusOK {
        logger.Error("Test failed for %s: HTTP %d - %s", endpoint.Name, resp.StatusCode, string(respBody))
        return toJSON(map[string]interface{}{
            "success":    false,
            "statusCode": resp.StatusCode,
            "message":    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
        })
    }

    var responseData map[string]interface{}
    if err := json.Unmarshal(respBody, &responseData); err != nil {
        logger.Info("Test successful for %s", endpoint.Name)
        return successJSON(map[string]interface{}{
            "message": string(respBody),
        })
    }

    var message string
    switch transformer {
    case "claude":
        if content, ok := responseData["content"].([]interface{}); ok && len(content) > 0 {
            if textBlock, ok := content[0].(map[string]interface{}); ok {
                if text, ok := textBlock["text"].(string); ok {
                    message = text
                }
            }
        }
    case "openai":
        if choices, ok := responseData["choices"].([]interface{}); ok && len(choices) > 0 {
            if choice, ok := choices[0].(map[string]interface{}); ok {
                if msg, ok := choice["message"].(map[string]interface{}); ok {
                    if content, ok := msg["content"].(string); ok {
                        message = content
                    }
                }
            }
        }
    case "openai2":
        // OpenAI Responses API format: output[].content[].text where type="output_text"
        if output, ok := responseData["output"].([]interface{}); ok && len(output) > 0 {
            for _, item := range output {
                if itemMap, ok := item.(map[string]interface{}); ok {
                    if content, ok := itemMap["content"].([]interface{}); ok {
                        for _, part := range content {
                            if partMap, ok := part.(map[string]interface{}); ok {
                                if partMap["type"] == "output_text" {
                                    if text, ok := partMap["text"].(string); ok {
                                        message += text
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    case "gemini":
        // Gemini native response format: candidates[].content.parts[].text
        if candidates, ok := responseData["candidates"].([]interface{}); ok && len(candidates) > 0 {
            if candidate, ok := candidates[0].(map[string]interface{}); ok {
                if content, ok := candidate["content"].(map[string]interface{}); ok {
                    if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
                        if part, ok := parts[0].(map[string]interface{}); ok {
                            if text, ok := part["text"].(string); ok {
                                message = text
                            }
                        }
                    }
                }
            }
        }
    }

    if message == "" {
        message = string(respBody)
    }

    logger.Info("Test successful for %s", endpoint.Name)
    return successJSON(map[string]interface{}{
        "message": message,
    })
}

// TestEndpointLight tests endpoint availability with minimal token consumption for a specific client type
func (e *EndpointService) TestEndpointLight(clientType string, index int) string {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        return e.testResult(false, "invalid_index", "models", fmt.Sprintf("Invalid endpoint index: %d", index))
    }

    endpoint := endpoints[index]
    logger.Info("Testing endpoint (light): %s (%s)", endpoint.Name, endpoint.APIUrl)

    transformer := endpoint.Transformer
    transformer = normalizeTransformer(transformer)

    normalizedURL := normalizeAPIUrlWithScheme(endpoint.APIUrl)

    // Step 1: Try models API
    statusCode, err := e.testModelsAPI(normalizedURL, endpoint.APIKey, transformer)
    if err == nil {
        return e.testResult(true, "ok", "models", "Models API accessible")
    }
    if statusCode == 401 || statusCode == 403 {
        return e.testResult(false, "invalid_key", "models", fmt.Sprintf("Authentication failed: HTTP %d", statusCode))
    }

    // Step 2: Try token count (Claude) or billing API (OpenAI)
    if transformer == "claude" {
        statusCode, err = e.testTokenCountAPI(normalizedURL, endpoint.APIKey)
        if err == nil {
            return e.testResult(true, "ok", "token_count", "Token count API accessible")
        }
        if statusCode == 401 || statusCode == 403 {
            return e.testResult(false, "invalid_key", "token_count", fmt.Sprintf("Authentication failed: HTTP %d", statusCode))
        }
    } else if transformer == "openai" || transformer == "openai2" {
        statusCode, err = e.testBillingAPI(normalizedURL, endpoint.APIKey)
        if err == nil {
            return e.testResult(true, "ok", "billing", "Billing API accessible")
        }
        if statusCode == 401 || statusCode == 403 {
            return e.testResult(false, "invalid_key", "billing", fmt.Sprintf("Authentication failed: HTTP %d", statusCode))
        }
    }

    // Step 3: Minimal request (fallback)
    statusCode, err = e.testMinimalRequest(normalizedURL, endpoint.APIKey, transformer, endpoint.Model)
    if err == nil {
        return e.testResult(true, "ok", "minimal", "Minimal request successful")
    }
    if statusCode == 401 || statusCode == 403 {
        return e.testResult(false, "invalid_key", "minimal", fmt.Sprintf("Authentication failed: HTTP %d", statusCode))
    }
    if statusCode == 405 {
        return e.testResult(false, "unknown", "minimal", "Method not allowed (may work in real client)")
    }

    return e.testResult(false, "error", "minimal", fmt.Sprintf("Test failed: %v", err))
}

func (e *EndpointService) testResult(success bool, status, method, message string) string {
    return toJSON(map[string]interface{}{
        "success": success,
        "status":  status,
        "method":  method,
        "message": message,
    })
}

// TestAllEndpointsZeroCost tests all endpoints using zero-cost methods only for a specific client type
func (e *EndpointService) TestAllEndpointsZeroCost(clientType string) string {
    clientType = normalizeClientType(clientType)

    endpoints := e.config.GetEndpointsByClient(clientType)
    results := make(map[string]string)

    for _, endpoint := range endpoints {
        transformer := normalizeTransformer(endpoint.Transformer)

        normalizedURL := normalizeAPIUrlWithScheme(endpoint.APIUrl)

        status := "unknown"

        statusCode, err := e.testModelsAPI(normalizedURL, endpoint.APIKey, transformer)
        if err == nil {
            status = "ok"
        } else if statusCode == 401 || statusCode == 403 {
            status = "invalid_key"
        } else {
            if transformer == "claude" {
                statusCode, err = e.testTokenCountAPI(normalizedURL, endpoint.APIKey)
                if err == nil {
                    status = "ok"
                } else if statusCode == 401 || statusCode == 403 {
                    status = "invalid_key"
                }
            } else if transformer == "openai" || transformer == "openai2" {
                statusCode, err = e.testBillingAPI(normalizedURL, endpoint.APIKey)
                if err == nil {
                    status = "ok"
                } else if statusCode == 401 || statusCode == 403 {
                    status = "invalid_key"
                }
            }
        }

        results[endpoint.Name] = status
    }

    return toJSON(results)
}

func (e *EndpointService) testModelsAPI(apiUrl, apiKey, transformer string) (int, error) {
    var url string
    if transformer == "gemini" {
        url = fmt.Sprintf("%s/v1beta/models?key=%s", apiUrl, apiKey)
    } else {
        url = fmt.Sprintf("%s/v1/models", apiUrl)
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return 0, err
    }

    // Set authentication headers based on transformer type
    switch transformer {
    case "claude":
        req.Header.Set("x-api-key", apiKey)
        req.Header.Set("anthropic-version", "2023-06-01")
    case "openai", "openai2":
        req.Header.Set("Authorization", "Bearer "+apiKey)
    // gemini uses query parameter, already set in URL
    }

    client := e.getHTTPClient(15 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return resp.StatusCode, fmt.Errorf("failed to read response")
    }

    var result map[string]interface{}
    if err := json.Unmarshal(body, &result); err != nil {
        return resp.StatusCode, fmt.Errorf("failed to parse response")
    }

    if data, ok := result["data"].([]interface{}); ok {
        if len(data) == 0 {
            return resp.StatusCode, fmt.Errorf("no models found")
        }
        return resp.StatusCode, nil
    }

    if models, ok := result["models"].([]interface{}); ok {
        if len(models) == 0 {
            return resp.StatusCode, fmt.Errorf("no models found")
        }
        return resp.StatusCode, nil
    }

    return resp.StatusCode, fmt.Errorf("unexpected response format")
}

func (e *EndpointService) testTokenCountAPI(apiUrl, apiKey string) (int, error) {
    url := fmt.Sprintf("%s/v1/messages/count_tokens", apiUrl)

    body, _ := json.Marshal(map[string]interface{}{
        "model": "claude-sonnet-4-5-20250929",
        "messages": []map[string]string{
            {"role": "user", "content": "Hi"},
        },
    })

    req, err := http.NewRequest("POST", url, bytes.NewReader(body))
    if err != nil {
        return 0, err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", apiKey)
    req.Header.Set("anthropic-version", "2023-06-01")
    req.Header.Set("anthropic-beta", "token-counting-2024-11-01")

    client := e.getHTTPClient(15 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
    }

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return resp.StatusCode, fmt.Errorf("failed to read response")
    }

    var result map[string]interface{}
    if err := json.Unmarshal(respBody, &result); err != nil {
        return resp.StatusCode, fmt.Errorf("failed to parse response")
    }

    if _, ok := result["input_tokens"]; !ok {
        return resp.StatusCode, fmt.Errorf("invalid response: no input_tokens")
    }

    return resp.StatusCode, nil
}

func (e *EndpointService) testBillingAPI(apiUrl, apiKey string) (int, error) {
    url := fmt.Sprintf("%s/v1/dashboard/billing/credit_grants", apiUrl)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return 0, err
    }

    req.Header.Set("Authorization", "Bearer "+apiKey)

    client := e.getHTTPClient(15 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
    }

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return resp.StatusCode, fmt.Errorf("failed to read response")
    }

    var result map[string]interface{}
    if err := json.Unmarshal(respBody, &result); err != nil {
        return resp.StatusCode, fmt.Errorf("failed to parse response")
    }

    return resp.StatusCode, nil
}

func (e *EndpointService) testMinimalRequest(apiUrl, apiKey, transformer, model string) (int, error) {
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
    case "openai":
        url = fmt.Sprintf("%s/v1/chat/completions", apiUrl)
        if model == "" {
            model = "gpt-4-turbo"
        }
        body, _ = json.Marshal(map[string]interface{}{
            "model":      model,
            "max_tokens": 1,
            "messages":   []map[string]interface{}{{"role": "user", "content": "Hi"}},
        })
    case "openai2":
        url = fmt.Sprintf("%s/v1/responses", apiUrl)
        if model == "" {
            model = "gpt-4-turbo"
        }
        body, _ = json.Marshal(map[string]interface{}{
            "model": model,
            "input": []map[string]interface{}{
                {"type": "message", "role": "user", "content": []map[string]interface{}{{"type": "input_text", "text": "Hi"}}},
            },
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
    if transformer == "claude" {
        req.Header.Set("x-api-key", apiKey)
        req.Header.Set("anthropic-version", "2023-06-01")
    } else if transformer != "gemini" {
        req.Header.Set("Authorization", "Bearer "+apiKey)
    }

    client := e.getHTTPClient(30 * time.Second)
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

// FetchModels fetches available models from the API provider
func (e *EndpointService) FetchModels(apiUrl, apiKey, transformer string) string {
    logger.Info("Fetching models for transformer: %s", transformer)

    transformer = normalizeTransformer(transformer)

    normalizedAPIUrl := normalizeAPIUrlWithScheme(apiUrl)

    var models []string
    var err error

    switch transformer {
    case "claude":
        models, err = e.fetchOpenAIModels(normalizedAPIUrl, apiKey)
    case "openai", "openai2":
        models, err = e.fetchOpenAIModels(normalizedAPIUrl, apiKey)
    case "gemini":
        models, err = e.fetchGeminiModels(normalizedAPIUrl, apiKey)
    default:
        return toJSON(map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Unsupported transformer: %s", transformer),
            "models":  []string{},
        })
    }

    if err != nil {
        return toJSON(map[string]interface{}{
            "success": false,
            "message": err.Error(),
            "models":  []string{},
        })
    }

    logger.Info("Fetched %d models for %s", len(models), transformer)
    return successJSON(map[string]interface{}{
        "message": fmt.Sprintf("Found %d models", len(models)),
        "models":  models,
    })
}

func (e *EndpointService) fetchOpenAIModels(apiUrl, apiKey string) ([]string, error) {
    url := fmt.Sprintf("%s/v1/models", apiUrl)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Authorization", "Bearer "+apiKey)

    client := e.getHTTPClient(30 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("no_models_found")
    }

    var result struct {
        Data []struct {
            ID string `json:"id"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %v", err)
    }

    seen := make(map[string]bool)
    models := make([]string, 0, len(result.Data))
    for _, m := range result.Data {
        id := strings.TrimSpace(m.ID)
        if id != "" && !seen[id] {
            seen[id] = true
            models = append(models, id)
        }
    }

    return models, nil
}

func (e *EndpointService) fetchGeminiModels(apiUrl, apiKey string) ([]string, error) {
    url := fmt.Sprintf("%s/v1beta/models?key=%s", apiUrl, apiKey)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    client := e.getHTTPClient(30 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }

    var result struct {
        Models []struct {
            Name string `json:"name"`
        } `json:"models"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %v", err)
    }

    models := make([]string, 0, len(result.Models))
    for _, m := range result.Models {
        name := m.Name
        if strings.HasPrefix(name, "models/") {
            name = strings.TrimPrefix(name, "models/")
        }
        models = append(models, name)
    }

    return models, nil
}

// ExportEndpoint represents an endpoint for export (without sensitive data option)
type ExportEndpoint struct {
	Name               string  `json:"name"`
	ClientType         string  `json:"clientType,omitempty"`
	APIUrl             string  `json:"apiUrl"`
	APIKey             string  `json:"apiKey,omitempty"`
	Enabled            bool    `json:"enabled"`
	Transformer        string  `json:"transformer,omitempty"`
	Model              string  `json:"model,omitempty"`
	Remark             string  `json:"remark,omitempty"`
	Tags               string  `json:"tags,omitempty"`
	ModelPatterns      string  `json:"modelPatterns,omitempty"`
	CostPerInputToken  float64 `json:"costPerInputToken,omitempty"`
	CostPerOutputToken float64 `json:"costPerOutputToken,omitempty"`
	QuotaLimit         int64   `json:"quotaLimit,omitempty"`
	QuotaResetCycle    string  `json:"quotaResetCycle,omitempty"`
	Priority           int     `json:"priority,omitempty"`
}

// ExportData represents the exported data structure
type ExportData struct {
	Version     string           `json:"version"`
	ExportTime  string           `json:"exportTime"`
	ClientType  string           `json:"clientType,omitempty"`
	Endpoints   []ExportEndpoint `json:"endpoints"`
	IncludeKeys bool             `json:"includeKeys"`
}

// ExportEndpoints exports endpoints for a specific client type
func (e *EndpointService) ExportEndpoints(clientType string, includeKeys bool) string {
	clientType = normalizeClientType(clientType)

	endpoints := e.config.GetEndpointsByClient(clientType)

	exportEndpoints := make([]ExportEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		exportEp := ExportEndpoint{
			Name:               ep.Name,
			ClientType:         ep.ClientType,
			APIUrl:             ep.APIUrl,
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

		if includeKeys {
			exportEp.APIKey = ep.APIKey
		} else {
			if len(ep.APIKey) > 8 {
				exportEp.APIKey = ep.APIKey[:4] + "****" + ep.APIKey[len(ep.APIKey)-4:]
			} else {
				exportEp.APIKey = "****"
			}
		}

		exportEndpoints = append(exportEndpoints, exportEp)
	}

	exportData := ExportData{
		Version:     "1.0",
		ExportTime:  time.Now().Format(time.RFC3339),
		ClientType:  clientType,
		Endpoints:   exportEndpoints,
		IncludeKeys: includeKeys,
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return errorJSON(fmt.Sprintf("Failed to export: %v", err))
	}

	logger.Info("Exported %d endpoints for client %s (includeKeys=%v)", len(exportEndpoints), clientType, includeKeys)
	return string(jsonData)
}

// ExportAllEndpoints exports all endpoints across all client types
func (e *EndpointService) ExportAllEndpoints(includeKeys bool) string {
	endpoints := e.config.GetEndpoints()

	exportEndpoints := make([]ExportEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		exportEp := ExportEndpoint{
			Name:               ep.Name,
			ClientType:         ep.ClientType,
			APIUrl:             ep.APIUrl,
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

		if includeKeys {
			exportEp.APIKey = ep.APIKey
		} else {
			if len(ep.APIKey) > 8 {
				exportEp.APIKey = ep.APIKey[:4] + "****" + ep.APIKey[len(ep.APIKey)-4:]
			} else {
				exportEp.APIKey = "****"
			}
		}

		exportEndpoints = append(exportEndpoints, exportEp)
	}

	exportData := ExportData{
		Version:     "1.0",
		ExportTime:  time.Now().Format(time.RFC3339),
		Endpoints:   exportEndpoints,
		IncludeKeys: includeKeys,
	}

	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return errorJSON(fmt.Sprintf("Failed to export: %v", err))
	}

	logger.Info("Exported %d endpoints (all clients, includeKeys=%v)", len(exportEndpoints), includeKeys)
	return string(jsonData)
}

// ImportResult represents the result of an import operation
type ImportResult struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message"`
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// ImportEndpoints imports endpoints from JSON data
// mode: "skip" (skip existing), "overwrite" (overwrite existing), "rename" (add suffix to duplicates)
func (e *EndpointService) ImportEndpoints(jsonData string, mode string) string {
	var exportData ExportData
	if err := json.Unmarshal([]byte(jsonData), &exportData); err != nil {
		return toJSON(ImportResult{
			Success: false,
			Message: fmt.Sprintf("Invalid JSON format: %v", err),
		})
	}

	if len(exportData.Endpoints) == 0 {
		return toJSON(ImportResult{
			Success: false,
			Message: "No endpoints found in import data",
		})
	}

	if mode != "skip" && mode != "overwrite" && mode != "rename" {
		mode = "skip"
	}

	imported := 0
	skipped := 0
	var errors []string

	for _, importEp := range exportData.Endpoints {
		if importEp.Name == "" {
			errors = append(errors, "Endpoint with empty name skipped")
			skipped++
			continue
		}
		if importEp.APIUrl == "" {
			errors = append(errors, fmt.Sprintf("Endpoint '%s': missing API URL", importEp.Name))
			skipped++
			continue
		}
		if importEp.APIKey == "" || strings.Contains(importEp.APIKey, "****") {
			errors = append(errors, fmt.Sprintf("Endpoint '%s': missing or masked API key", importEp.Name))
			skipped++
			continue
		}

		clientType := normalizeClientType(importEp.ClientType)
		transformer := normalizeTransformer(importEp.Transformer)

		existingEndpoints := e.config.GetEndpointsByClient(clientType)
		exists := false
		existingIndex := -1
		for i, ep := range existingEndpoints {
			if ep.Name == importEp.Name {
				exists = true
				existingIndex = i
				break
			}
		}

		if exists {
			switch mode {
			case "skip":
				skipped++
				continue
			case "overwrite":
				err := e.UpdateEndpoint(clientType, existingIndex, importEp.Name, importEp.APIUrl, importEp.APIKey, transformer, importEp.Model, importEp.Remark, importEp.Tags,
					importEp.ModelPatterns, importEp.CostPerInputToken, importEp.CostPerOutputToken, importEp.QuotaLimit, importEp.QuotaResetCycle, importEp.Priority)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to update '%s': %v", importEp.Name, err))
					skipped++
				} else {
					imported++
				}
				continue
			case "rename":
				baseName := importEp.Name
				suffix := 1
				for suffix <= 100 {
					newName := fmt.Sprintf("%s_%d", baseName, suffix)
					nameExists := false
					for _, ep := range existingEndpoints {
						if ep.Name == newName {
							nameExists = true
							break
						}
					}
					if !nameExists {
						importEp.Name = newName
						break
					}
					suffix++
				}
				if suffix > 100 {
					errors = append(errors, fmt.Sprintf("Could not find unique name for '%s'", baseName))
					skipped++
					continue
				}
			}
		}

		err := e.AddEndpoint(clientType, importEp.Name, importEp.APIUrl, importEp.APIKey, transformer, importEp.Model, importEp.Remark, importEp.Tags,
			importEp.ModelPatterns, importEp.CostPerInputToken, importEp.CostPerOutputToken, importEp.QuotaLimit, importEp.QuotaResetCycle, importEp.Priority)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to add '%s': %v", importEp.Name, err))
			skipped++
		} else {
			imported++
		}
	}

	result := ImportResult{
		Success:  imported > 0 || (imported == 0 && skipped > 0 && len(errors) == 0),
		Message:  fmt.Sprintf("Imported %d endpoints, skipped %d", imported, skipped),
		Imported: imported,
		Skipped:  skipped,
		Errors:   errors,
	}

	if imported == 0 && len(errors) > 0 {
		result.Success = false
		result.Message = "Import failed"
	}

	logger.Info("Import completed: %d imported, %d skipped, %d errors", imported, skipped, len(errors))
	return toJSON(result)
}

// GetAllEndpointTags returns all unique tags used across all endpoints
func (e *EndpointService) GetAllEndpointTags() ([]string, error) {
	if e.storage == nil {
		return []string{}, nil
	}
	return e.storage.GetAllEndpointTags()
}

// GetHealthHistory returns health history for an endpoint
func (e *EndpointService) GetHealthHistory(endpointName, clientType string, hours int) ([]storage.HealthHistoryRecord, error) {
	if e.storage == nil {
		return []storage.HealthHistoryRecord{}, nil
	}

	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(hours) * time.Hour)

	return e.storage.GetHealthHistory(endpointName, clientType, startTime, endTime, 1000)
}

// GetHealthHistoryRetentionDays returns the health history retention days
func (e *EndpointService) GetHealthHistoryRetentionDays() int {
	return e.config.GetHealthHistoryRetentionDays()
}

// SetHealthHistoryRetentionDays sets the health history retention days
func (e *EndpointService) SetHealthHistoryRetentionDays(days int) error {
	if days < 1 {
		return fmt.Errorf("retention days must be at least 1")
	}
	if days > 365 {
		return fmt.Errorf("retention days cannot exceed 365")
	}

	e.config.UpdateHealthHistoryRetentionDays(days)

	if e.storage != nil {
		configAdapter := storage.NewConfigStorageAdapter(e.storage)
		if err := e.config.SaveToStorage(configAdapter); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	logger.Info("Health history retention days updated to %d", days)
	return nil
}

// CleanupOldHealthHistory removes old health history records
func (e *EndpointService) CleanupOldHealthHistory() error {
	if e.storage == nil {
		return nil
	}

	days := e.config.GetHealthHistoryRetentionDays()
	return e.storage.CleanupOldHealthHistory(days)
}

// EndpointTestResult 单个端点的检测结果
type EndpointTestResult struct {
	Name         string  `json:"name"`
	Success      bool    `json:"success"`
	LatencyMs    float64 `json:"latencyMs"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
	Action       string  `json:"action"` // "enabled", "disabled", "set_current", "unchanged"
	WasEnabled   bool    `json:"wasEnabled"`
}

// TestAllEndpointsResult 一键检测的结果
type TestAllEndpointsResult struct {
	Success       bool                 `json:"success"`
	Message       string               `json:"message"`
	Results       []EndpointTestResult `json:"results"`
	BestEndpoint  string               `json:"bestEndpoint,omitempty"`
	EnabledCount  int                  `json:"enabledCount"`
	DisabledCount int                  `json:"disabledCount"`
}

// TestAllEndpointsAndOptimize 一键检测所有端点并优化配置
// 1. 检测所有端点（包括禁用的）
// 2. 将检测成功且延迟最低的端点设为当前使用（移到第一位）
// 3. 将检测成功但被禁用的端点启用
// 4. 将检测失败的端点禁用
func (e *EndpointService) TestAllEndpointsAndOptimize(clientType string) string {
	clientType = normalizeClientType(clientType)

	// 获取所有端点（包括禁用的）
	allEndpoints := e.config.GetEndpointsByClient(clientType)
	if len(allEndpoints) == 0 {
		return toJSON(TestAllEndpointsResult{
			Success: false,
			Message: "没有找到端点",
		})
	}

	// 并发测试所有端点
	type testResult struct {
		index     int
		endpoint  config.Endpoint
		success   bool
		latencyMs float64
		errorMsg  string
	}

	results := make([]testResult, len(allEndpoints))
	var wg sync.WaitGroup

	for i, ep := range allEndpoints {
		wg.Add(1)
		go func(idx int, endpoint config.Endpoint) {
			defer wg.Done()

			transformer := normalizeTransformer(endpoint.Transformer)
			normalizedURL := normalizeAPIUrlWithScheme(endpoint.APIUrl)

			start := time.Now()
			statusCode, err := e.testMinimalRequest(normalizedURL, endpoint.APIKey, transformer, endpoint.Model)
			latencyMs := float64(time.Since(start).Milliseconds())

			success := err == nil
			errorMsg := ""
			if err != nil {
				if statusCode > 0 {
					errorMsg = fmt.Sprintf("HTTP %d", statusCode)
				} else {
					errorMsg = err.Error()
				}
			}

			results[idx] = testResult{
				index:     idx,
				endpoint:  endpoint,
				success:   success,
				latencyMs: latencyMs,
				errorMsg:  errorMsg,
			}

			// 记录检测结果到 Monitor
			if e.proxy != nil {
				e.proxy.GetMonitor().RecordCheckResult(endpoint.Name, success, latencyMs, errorMsg)
				if success {
					e.proxy.GetMonitor().RecordHealthCheckLatency(endpoint.Name, latencyMs)
				} else {
					e.proxy.GetMonitor().ClearHealthCheckLatency(endpoint.Name)
				}
			}
		}(i, ep)
	}

	wg.Wait()

	// 找出检测成功且延迟最低的端点
	var bestResult *testResult
	for i := range results {
		if results[i].success {
			if bestResult == nil || results[i].latencyMs < bestResult.latencyMs {
				bestResult = &results[i]
			}
		}
	}

	// 构建结果并调整端点状态
	testResults := make([]EndpointTestResult, len(results))
	enabledCount := 0
	disabledCount := 0

	for i, r := range results {
		action := "unchanged"
		wasEnabled := r.endpoint.Enabled

		if r.success {
			// 检测成功
			if !r.endpoint.Enabled {
				// 之前禁用的，现在启用
				action = "enabled"
				enabledCount++
			}
			if bestResult != nil && r.index == bestResult.index {
				action = "set_current"
			}
		} else {
			// 检测失败
			if r.endpoint.Enabled {
				// 之前启用的，现在禁用
				action = "disabled"
				disabledCount++
			}
		}

		testResults[i] = EndpointTestResult{
			Name:         r.endpoint.Name,
			Success:      r.success,
			LatencyMs:    r.latencyMs,
			ErrorMessage: r.errorMsg,
			Action:       action,
			WasEnabled:   wasEnabled,
		}
	}

	// 应用更改：根据检测结果设置状态
	// 注意：跳过禁用状态的端点，不自动启用
	for _, r := range results {
		// 跳过禁用状态的端点
		if r.endpoint.Status == config.EndpointStatusDisabled {
			continue
		}

		if r.success {
			// 检测成功：设置为可用
			e.config.SetEndpointStatus(r.endpoint.Name, clientType, config.EndpointStatusAvailable)
		} else {
			// 检测失败：设置为不可用（如果之前是可用的）
			if r.endpoint.Status == config.EndpointStatusAvailable {
				e.config.SetEndpointStatus(r.endpoint.Name, clientType, config.EndpointStatusUnavailable)
			}
		}
	}

	// 2. 将最佳端点移到第一位
	bestEndpointName := ""
	if bestResult != nil {
		bestEndpointName = bestResult.endpoint.Name
		if bestResult.index != 0 {
			// 移动到第一位
			e.config.MoveEndpoint(clientType, bestResult.index, 0)
		}
		// 设置为当前端点
		if e.proxy != nil {
			e.proxy.SetCurrentEndpointForClient(clientType, bestEndpointName)
		}
	}

	// 保存配置
	if e.storage != nil {
		configAdapter := storage.NewConfigStorageAdapter(e.storage)
		if err := e.config.SaveToStorage(configAdapter); err != nil {
			logger.Warn("Failed to save config after endpoint optimization: %v", err)
		}
	}

	message := fmt.Sprintf("检测完成：%d 个成功，%d 个失败", len(results)-disabledCount, disabledCount)
	if bestEndpointName != "" {
		message += fmt.Sprintf("，最佳端点：%s", bestEndpointName)
	}

	logger.Info("TestAllEndpointsAndOptimize: %s (enabled=%d, disabled=%d)", message, enabledCount, disabledCount)

	return toJSON(TestAllEndpointsResult{
		Success:       true,
		Message:       message,
		Results:       testResults,
		BestEndpoint:  bestEndpointName,
		EnabledCount:  enabledCount,
		DisabledCount: disabledCount,
	})
}
