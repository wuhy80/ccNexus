package proxy

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/logger"
	"github.com/lich0821/ccNexus/internal/transformer"
)

// handleNonStreamingResponse processes non-streaming responses
// Returns: usage, rawResponse, transformedResponse, error
func (p *Proxy) handleNonStreamingResponse(w http.ResponseWriter, resp *http.Response, endpoint config.Endpoint, trans transformer.Transformer) (transformer.TokenUsageDetail, interface{}, interface{}, error) {
	var bodyBytes []byte
	var err error

	if resp.Header.Get("Content-Encoding") == "gzip" {
		bodyBytes, err = decompressGzip(resp.Body)
		if err != nil {
			logger.Error("[%s] Failed to decompress gzip response: %v", endpoint.Name, err)
			return transformer.TokenUsageDetail{}, nil, nil, err
		}
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("[%s] Failed to read response body: %v", endpoint.Name, err)
			return transformer.TokenUsageDetail{}, nil, nil, err
		}
	}
	resp.Body.Close()

	logger.DebugLog("[%s] Response Body: %s", endpoint.Name, string(bodyBytes))

	// Parse raw response as JSON for interaction recording
	var rawResponse interface{}
	json.Unmarshal(bodyBytes, &rawResponse)

	// Transform response back to Claude format
	transformedResp, err := trans.TransformResponse(bodyBytes, false)
	if err != nil {
		logger.Error("[%s] Failed to transform response: %v", endpoint.Name, err)
		return transformer.TokenUsageDetail{}, rawResponse, nil, err
	}

	logger.DebugLog("[%s] Transformed Response: %s", endpoint.Name, string(transformedResp))

	// Parse transformed response as JSON for interaction recording
	var transformedResponse interface{}
	json.Unmarshal(transformedResp, &transformedResponse)

	// Extract token usage
	usage := extractTokenUsage(transformedResp)

	// Copy response headers
	for key, values := range resp.Header {
		if key == "Content-Length" || key == "Content-Encoding" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(transformedResp)

	return usage, rawResponse, transformedResponse, nil
}

// extractTokenUsage extracts detailed token usage from response
func extractTokenUsage(responseBody []byte) transformer.TokenUsageDetail {
	var resp map[string]interface{}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return transformer.TokenUsageDetail{}
	}

	if usageMap, ok := resp["usage"].(map[string]interface{}); ok {
		return transformer.ExtractTokenUsageDetail(usageMap)
	}

	return transformer.TokenUsageDetail{}
}
