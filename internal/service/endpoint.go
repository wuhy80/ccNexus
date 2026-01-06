package service

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/lich0821/ccNexus/internal/config"
    "github.com/lich0821/ccNexus/internal/logger"
    "github.com/lich0821/ccNexus/internal/proxy"
    "github.com/lich0821/ccNexus/internal/storage"
)

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

// EndpointService handles endpoint management operations
type EndpointService struct {
    config  *config.Config
    proxy   *proxy.Proxy
    storage *storage.SQLiteStorage
}

// NewEndpointService creates a new EndpointService
func NewEndpointService(cfg *config.Config, p *proxy.Proxy, s *storage.SQLiteStorage) *EndpointService {
    return &EndpointService{
        config:  cfg,
        proxy:   p,
        storage: s,
    }
}

// normalizeAPIUrl ensures the API URL has the correct format
func normalizeAPIUrl(apiUrl string) string {
    return strings.TrimSuffix(apiUrl, "/")
}

// AddEndpoint adds a new endpoint for a specific client type
func (e *EndpointService) AddEndpoint(clientType, name, apiUrl, apiKey, transformer, model, remark string) error {
    if clientType == "" {
        clientType = "claude"
    }

    endpoints := e.config.GetEndpointsByClient(clientType)
    for _, ep := range endpoints {
        if ep.Name == name {
            return fmt.Errorf("endpoint name '%s' already exists for client type '%s'", name, clientType)
        }
    }

    if transformer == "" {
        transformer = "claude"
    }

    apiUrl = normalizeAPIUrl(apiUrl)

    newEndpoint := config.Endpoint{
        Name:        name,
        ClientType:  clientType,
        APIUrl:      apiUrl,
        APIKey:      apiKey,
        Enabled:     true,
        Transformer: transformer,
        Model:       model,
        Remark:      remark,
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
    if clientType == "" {
        clientType = "claude"
    }

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
func (e *EndpointService) UpdateEndpoint(clientType string, index int, name, apiUrl, apiKey, transformer, model, remark string) error {
    if clientType == "" {
        clientType = "claude"
    }

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

    if transformer == "" {
        transformer = "claude"
    }

    apiUrl = normalizeAPIUrl(apiUrl)

    updatedEndpoint := config.Endpoint{
        Name:        name,
        ClientType:  clientType,
        APIUrl:      apiUrl,
        APIKey:      apiKey,
        Enabled:     enabled,
        Transformer: transformer,
        Model:       model,
        Remark:      remark,
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

// ToggleEndpoint toggles the enabled state of an endpoint for a specific client type
func (e *EndpointService) ToggleEndpoint(clientType string, index int, enabled bool) error {
    if clientType == "" {
        clientType = "claude"
    }

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        return fmt.Errorf("invalid endpoint index: %d", index)
    }

    endpointName := endpoints[index].Name

    // Update in all endpoints
    allEndpoints := e.config.GetEndpoints()
    for i, ep := range allEndpoints {
        if ep.Name == endpointName && ep.ClientType == clientType {
            allEndpoints[i].Enabled = enabled
            break
        }
    }
    e.config.UpdateEndpoints(allEndpoints)

    if err := e.proxy.UpdateConfig(e.config); err != nil {
        return err
    }

    if e.storage != nil {
        configAdapter := storage.NewConfigStorageAdapter(e.storage)
        if err := e.config.SaveToStorage(configAdapter); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
    }

    if enabled {
        logger.Info("Endpoint enabled: %s (client: %s)", endpointName, clientType)
    } else {
        logger.Info("Endpoint disabled: %s (client: %s)", endpointName, clientType)
    }

    return nil
}

// ReorderEndpoints reorders endpoints based on the provided name array for a specific client type
func (e *EndpointService) ReorderEndpoints(clientType string, names []string) error {
    if clientType == "" {
        clientType = "claude"
    }

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
    if clientType == "" {
        clientType = "claude"
    }
    return e.proxy.GetCurrentEndpointNameForClient(clientType)
}

// SwitchToEndpoint manually switches to a specific endpoint by name for a specific client type
func (e *EndpointService) SwitchToEndpoint(clientType, endpointName string) error {
    if e.proxy == nil {
        return fmt.Errorf("proxy not initialized")
    }
    if clientType == "" {
        clientType = "claude"
    }
    return e.proxy.SetCurrentEndpointForClient(clientType, endpointName)
}

// TestEndpoint tests an endpoint by sending a simple request for a specific client type
func (e *EndpointService) TestEndpoint(clientType string, index int) string {
    if clientType == "" {
        clientType = "claude"
    }

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        result := map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Invalid endpoint index: %d", index),
        }
        data, _ := json.Marshal(result)
        return string(data)
    }

    endpoint := endpoints[index]
    logger.Info("Testing endpoint: %s (%s)", endpoint.Name, endpoint.APIUrl)

    var requestBody []byte
    var err error
    var apiPath string

    transformer := endpoint.Transformer
    if transformer == "" {
        transformer = "claude"
    }

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
        // Gemini endpoints use Claude format through proxy with /gemini/ prefix
        // The proxy will convert Claude format to Gemini format
        apiPath = "/gemini/v1/messages"
        model := endpoint.Model
        if model == "" {
            model = "gemini-2.0-flash"
        }
        requestBody, err = json.Marshal(map[string]interface{}{
            "model":      model,
            "max_tokens": testMaxTokens,
            "messages": []map[string]string{
                {"role": "user", "content": testMessage},
            },
        })

    default:
        result := map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Unsupported transformer: %s", transformer),
        }
        data, _ := json.Marshal(result)
        return string(data)
    }

    if err != nil {
        result := map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Failed to build request: %v", err),
        }
        data, _ := json.Marshal(result)
        return string(data)
    }

    // 通过ccNexus代理发送请求，而不是直接发送到目标API
    // 这样请求会被识别为Claude Code客户端
    port := e.config.GetPort()
    url := fmt.Sprintf("http://localhost:%d%s", port, apiPath)

    req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody))
    if err != nil {
        result := map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Failed to create request: %v", err),
        }
        data, _ := json.Marshal(result)
        return string(data)
    }

    req.Header.Set("Content-Type", "application/json")
    // Specify which endpoint to use for this test request
    req.Header.Set("X-CCNexus-Endpoint", endpoint.Name)
    switch transformer {
    case "claude", "gemini":
        // Both use Claude format through proxy
        req.Header.Set("x-api-key", endpoint.APIKey)
        req.Header.Set("anthropic-version", "2023-06-01")
    case "openai", "openai2":
        req.Header.Set("Authorization", "Bearer "+endpoint.APIKey)
    }

    client := e.createHTTPClient(30 * time.Second)
    resp, err := client.Do(req)
    if err != nil {
        result := map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Request failed: %v", err),
        }
        data, _ := json.Marshal(result)
        logger.Error("Test failed for %s: %v", endpoint.Name, err)
        return string(data)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        result := map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Failed to read response: %v", err),
        }
        data, _ := json.Marshal(result)
        return string(data)
    }

    if resp.StatusCode != http.StatusOK {
        result := map[string]interface{}{
            "success":    false,
            "statusCode": resp.StatusCode,
            "message":    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
        }
        data, _ := json.Marshal(result)
        logger.Error("Test failed for %s: HTTP %d", endpoint.Name, resp.StatusCode)
        return string(data)
    }

    var responseData map[string]interface{}
    if err := json.Unmarshal(respBody, &responseData); err != nil {
        result := map[string]interface{}{
            "success": true,
            "message": string(respBody),
        }
        data, _ := json.Marshal(result)
        logger.Info("Test successful for %s", endpoint.Name)
        return string(data)
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
        // Gemini response comes back in Claude format through proxy
        if content, ok := responseData["content"].([]interface{}); ok && len(content) > 0 {
            if textBlock, ok := content[0].(map[string]interface{}); ok {
                if text, ok := textBlock["text"].(string); ok {
                    message = text
                }
            }
        }
    }

    if message == "" {
        message = string(respBody)
    }

    result := map[string]interface{}{
        "success": true,
        "message": message,
    }
    data, _ := json.Marshal(result)
    logger.Info("Test successful for %s", endpoint.Name)
    return string(data)
}

// Remaining methods (TestEndpointLight, TestAllEndpointsZeroCost, FetchModels, etc.) 
// will be added in the next part due to size constraints

// TestEndpointLight tests endpoint availability with minimal token consumption for a specific client type
func (e *EndpointService) TestEndpointLight(clientType string, index int) string {
    if clientType == "" {
        clientType = "claude"
    }

    endpoints := e.config.GetEndpointsByClient(clientType)

    if index < 0 || index >= len(endpoints) {
        return e.testResult(false, "invalid_index", "models", fmt.Sprintf("Invalid endpoint index: %d", index))
    }

    endpoint := endpoints[index]
    logger.Info("Testing endpoint (light): %s (%s)", endpoint.Name, endpoint.APIUrl)

    transformer := endpoint.Transformer
    if transformer == "" {
        transformer = "claude"
    }

    normalizedURL := normalizeAPIUrl(endpoint.APIUrl)
    if !strings.HasPrefix(normalizedURL, "http://") && !strings.HasPrefix(normalizedURL, "https://") {
        normalizedURL = "https://" + normalizedURL
    }

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
    result := map[string]interface{}{
        "success": success,
        "status":  status,
        "method":  method,
        "message": message,
    }
    data, _ := json.Marshal(result)
    return string(data)
}

// TestAllEndpointsZeroCost tests all endpoints using zero-cost methods only for a specific client type
func (e *EndpointService) TestAllEndpointsZeroCost(clientType string) string {
    if clientType == "" {
        clientType = "claude"
    }

    endpoints := e.config.GetEndpointsByClient(clientType)
    results := make(map[string]string)

    for _, endpoint := range endpoints {
        transformer := endpoint.Transformer
        if transformer == "" {
            transformer = "claude"
        }

        normalizedURL := normalizeAPIUrl(endpoint.APIUrl)
        if !strings.HasPrefix(normalizedURL, "http://") && !strings.HasPrefix(normalizedURL, "https://") {
            normalizedURL = "https://" + normalizedURL
        }

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

    data, _ := json.Marshal(results)
    return string(data)
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

    client := e.createHTTPClient(15 * time.Second)
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

    client := e.createHTTPClient(15 * time.Second)
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

    client := e.createHTTPClient(15 * time.Second)
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

    client := e.createHTTPClient(30 * time.Second)
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

    if transformer == "" {
        transformer = "claude"
    }

    normalizedAPIUrl := normalizeAPIUrl(apiUrl)
    if !strings.HasPrefix(normalizedAPIUrl, "http://") && !strings.HasPrefix(normalizedAPIUrl, "https://") {
        normalizedAPIUrl = "https://" + normalizedAPIUrl
    }

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
        result := map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Unsupported transformer: %s", transformer),
            "models":  []string{},
        }
        data, _ := json.Marshal(result)
        return string(data)
    }

    if err != nil {
        result := map[string]interface{}{
            "success": false,
            "message": err.Error(),
            "models":  []string{},
        }
        data, _ := json.Marshal(result)
        return string(data)
    }

    result := map[string]interface{}{
        "success": true,
        "message": fmt.Sprintf("Found %d models", len(models)),
        "models":  models,
    }
    data, _ := json.Marshal(result)
    logger.Info("Fetched %d models for %s", len(models), transformer)
    return string(data)
}

func (e *EndpointService) fetchOpenAIModels(apiUrl, apiKey string) ([]string, error) {
    url := fmt.Sprintf("%s/v1/models", apiUrl)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Authorization", "Bearer "+apiKey)

    client := e.createHTTPClient(30 * time.Second)
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

    client := e.createHTTPClient(30 * time.Second)
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
