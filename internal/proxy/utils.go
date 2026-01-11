package proxy

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lich0821/ccNexus/internal/logger"
	"github.com/lich0821/ccNexus/internal/tokencount"
)

// normalizeAPIUrl ensures the API URL has a protocol prefix
func normalizeAPIUrl(apiUrl string) string {
	if !strings.HasPrefix(apiUrl, "http://") && !strings.HasPrefix(apiUrl, "https://") {
		return "https://" + apiUrl
	}
	return apiUrl
}

// shouldRetry determines if a response should trigger a retry
func shouldRetry(statusCode int) bool {
	return statusCode != http.StatusOK &&
		statusCode != http.StatusBadRequest &&
		statusCode != http.StatusUnauthorized
}

// cleanIncompleteToolCalls removes incomplete tool_use blocks from request
func cleanIncompleteToolCalls(bodyBytes []byte) ([]byte, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return bodyBytes, err
	}

	messages, ok := req["messages"].([]interface{})
	if !ok {
		return bodyBytes, nil
	}

	hasIncomplete := false
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msg["role"].(string)
		if role != "assistant" {
			break
		}

		content, ok := msg["content"].([]interface{})
		if !ok {
			break
		}

		var cleanedContent []interface{}
		for _, block := range content {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				cleanedContent = append(cleanedContent, block)
				continue
			}

			blockType, _ := blockMap["type"].(string)
			if blockType == "tool_use" {
				if input, hasInput := blockMap["input"]; !hasInput || input == nil {
					logger.Debug("Removing incomplete tool_use block without input")
					hasIncomplete = true
					continue
				}
			}
			cleanedContent = append(cleanedContent, block)
		}

		if hasIncomplete {
			if len(cleanedContent) == 0 {
				messages = append(messages[:i], messages[i+1:]...)
			} else {
				msg["content"] = cleanedContent
			}
		}
		break
	}

	if !hasIncomplete {
		return bodyBytes, nil
	}

	req["messages"] = messages
	return json.Marshal(req)
}

// estimateInputTokens estimates input tokens from request body
func (p *Proxy) estimateInputTokens(bodyBytes []byte) int {
	var req tokencount.CountTokensRequest
	if json.Unmarshal(bodyBytes, &req) == nil {
		return tokencount.EstimateInputTokens(&req)
	}
	return 0
}

// estimateOutputTokens estimates output tokens from text
func (p *Proxy) estimateOutputTokens(outputText string) int {
	if outputText != "" {
		return tokencount.EstimateOutputTokens(outputText)
	}
	return 0
}

// ExtractMessagePreview extracts the first N characters of the last user message as a preview
// It filters out system content like <system-reminder> tags
func ExtractMessagePreview(bodyBytes []byte, maxLen int) string {
	var req map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return ""
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return ""
	}

	// Find the last user message
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msg["role"].(string)
		if role != "user" {
			continue
		}

		content := extractTextContent(msg["content"])
		if content == "" {
			continue
		}

		// Filter out system content
		content = filterSystemContent(content)
		if content == "" {
			continue
		}

		// Truncate to maxLen characters (using runes for proper Unicode handling)
		runes := []rune(content)
		if len(runes) > maxLen {
			return string(runes[:maxLen]) + "..."
		}
		return content
	}

	return ""
}

// filterSystemContent removes system tags and extracts user's actual input
func filterSystemContent(content string) string {
	// Remove <system-reminder>...</system-reminder> blocks
	result := removeTagBlocks(content, "system-reminder")

	// Remove <env>...</env> blocks
	result = removeTagBlocks(result, "env")

	// Remove <claude_background_info>...</claude_background_info> blocks
	result = removeTagBlocks(result, "claude_background_info")

	// Remove <claudeMd>...</claudeMd> blocks
	result = removeTagBlocks(result, "claudeMd")

	// Trim whitespace and return
	return strings.TrimSpace(result)
}

// removeTagBlocks removes all occurrences of <tag>...</tag> from content
func removeTagBlocks(content, tagName string) string {
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"

	for {
		startIdx := strings.Index(content, openTag)
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(content[startIdx:], closeTag)
		if endIdx == -1 {
			// No closing tag, remove from openTag to end
			content = content[:startIdx]
			break
		}

		// Remove the entire tag block
		content = content[:startIdx] + content[startIdx+endIdx+len(closeTag):]
	}

	return content
}

// extractTextContent extracts text from content field (supports string and array formats)
func extractTextContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		// Handle array format: [{type: "text", text: "..."}]
		for _, block := range c {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					if text, ok := blockMap["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return ""
}
