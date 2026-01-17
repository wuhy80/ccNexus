package proxy

import (
	"encoding/json"
	"net/http"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/logger"
	"github.com/lich0821/ccNexus/internal/tokencount"
)

// handleHealth handles health check requests
func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	endpoints := p.getEnabledEndpoints()
	response := map[string]interface{}{
		"status":            "healthy",
		"enabled_endpoints": len(endpoints),
		"endpoints":         endpoints,
	}

	json.NewEncoder(w).Encode(response)
}

// handleStats handles statistics requests
func (p *Proxy) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := p.GetStats()
	json.NewEncoder(w).Encode(stats)
}

// GetStats returns current statistics
func (p *Proxy) GetStats() *Stats {
	return p.stats
}

// handleCountTokens handles token counting requests
func (p *Proxy) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Model    string                   `json:"model"`
		System   interface{}              `json:"system,omitempty"`
		Messages []map[string]interface{} `json:"messages"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode count_tokens request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	systemText := ""
	if req.System != nil {
		switch sys := req.System.(type) {
		case string:
			systemText = sys
		case []interface{}:
			for _, block := range sys {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if text, ok := blockMap["text"].(string); ok {
						systemText += text + "\n"
					}
				}
			}
		}
	}

	totalTokens := 0
	if systemText != "" {
		totalTokens += tokencount.EstimateOutputTokens(systemText)
	}

	for _, msg := range req.Messages {
		content, ok := msg["content"]
		if !ok {
			continue
		}

		switch c := content.(type) {
		case string:
			totalTokens += tokencount.EstimateOutputTokens(c)
		case []interface{}:
			for _, block := range c {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if text, ok := blockMap["text"].(string); ok {
						totalTokens += tokencount.EstimateOutputTokens(text)
					}
				}
			}
		}
	}

	response := map[string]interface{}{
		"input_tokens": totalTokens,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateConfig updates the proxy configuration
func (p *Proxy) UpdateConfig(cfg *config.Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Save current endpoint names for each client type
	currentEndpointNames := make(map[ClientType]string)
	if p.config != nil {
		for _, clientType := range []ClientType{ClientTypeClaude, ClientTypeGemini, ClientTypeCodex} {
			endpoints := p.config.GetEnabledEndpointsByClient(string(clientType))
			if len(endpoints) > 0 {
				idx := p.currentIndexByClient[clientType]
				if idx < len(endpoints) {
					currentEndpointNames[clientType] = endpoints[idx].Name
				}
			}
		}
	}

	p.config = cfg

	// 更新路由器和配额跟踪器的配置引用
	if p.router != nil {
		p.router.UpdateConfig(cfg)
	}
	if p.quotaTracker != nil {
		p.quotaTracker.UpdateConfig(cfg)
	}

	// Try to find the previous current endpoints in new config for each client type
	for _, clientType := range []ClientType{ClientTypeClaude, ClientTypeGemini, ClientTypeCodex} {
		newEndpoints := cfg.GetEnabledEndpointsByClient(string(clientType))
		currentName := currentEndpointNames[clientType]
		if currentName != "" && len(newEndpoints) > 0 {
			found := false
			for i, ep := range newEndpoints {
				if ep.Name == currentName {
					p.currentIndexByClient[clientType] = i
					found = true
					logger.Debug("[CONFIG UPDATE] Preserved current endpoint for %s: %s at index %d", clientType, currentName, i)
					break
				}
			}
			if !found {
				p.currentIndexByClient[clientType] = 0
				logger.Debug("[CONFIG UPDATE] Current endpoint '%s' for %s not found, reset to index 0", currentName, clientType)
			}
		} else {
			p.currentIndexByClient[clientType] = 0
		}
	}

	logger.Info("Configuration updated: %d endpoints configured", len(cfg.GetEndpoints()))
	return nil
}
