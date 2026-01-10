package service

import (
	"encoding/json"

	"github.com/lich0821/ccNexus/internal/logger"
)

// toJSON safely marshals data to JSON string, logging errors if they occur
func toJSON(data interface{}) string {
	result, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to marshal JSON: %v", err)
		// Return a valid error JSON response
		return `{"success":false,"error":"internal serialization error"}`
	}
	return string(result)
}

// errorJSON creates a JSON error response
func errorJSON(message string) string {
	return toJSON(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// successJSON creates a JSON success response with optional data
func successJSON(data map[string]interface{}) string {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["success"] = true
	return toJSON(data)
}
