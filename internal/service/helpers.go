package service

import "strings"

// normalizeClientType ensures clientType has a default value
func normalizeClientType(clientType string) string {
	if clientType == "" {
		return "claude"
	}
	return clientType
}

// normalizeTransformer ensures transformer has a default value
func normalizeTransformer(transformer string) string {
	if transformer == "" {
		return "claude"
	}
	return transformer
}

// normalizeAPIUrlWithScheme ensures the API URL has the correct format with scheme
func normalizeAPIUrlWithScheme(apiUrl string) string {
	apiUrl = strings.TrimSuffix(apiUrl, "/")
	if !strings.HasPrefix(apiUrl, "http://") && !strings.HasPrefix(apiUrl, "https://") {
		apiUrl = "https://" + apiUrl
	}
	return apiUrl
}
