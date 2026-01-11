package interaction

import (
	"strings"
)

// AnalyzeIntent 分析请求的意图，返回消息摘要、工具调用列表和意图类型
func AnalyzeIntent(rawRequest interface{}) (preview string, toolCalls []string, intentType string) {
	reqMap, ok := rawRequest.(map[string]interface{})
	if !ok {
		return "", nil, IntentUnknown
	}

	// 提取消息摘要
	preview = extractMessagePreview(reqMap, 300)

	// 提取工具调用
	toolCalls = extractToolCalls(reqMap)

	// 分类意图
	intentType = classifyIntent(preview, toolCalls)

	return
}

// extractMessagePreview 提取最后一条用户消息的摘要
func extractMessagePreview(req map[string]interface{}, maxLen int) string {
	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return ""
	}

	// 从后往前找最后一条用户消息
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

		// 过滤系统内容
		content = filterSystemContent(content)
		if content == "" {
			continue
		}

		// 截断到 maxLen 字符
		runes := []rune(content)
		if len(runes) > maxLen {
			return string(runes[:maxLen]) + "..."
		}
		return content
	}

	return ""
}

// extractTextContent 从 content 字段提取文本（支持字符串和数组格式）
func extractTextContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		// 处理数组格式: [{type: "text", text: "..."}]
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

// filterSystemContent 移除系统标签，提取用户实际输入
func filterSystemContent(content string) string {
	// 移除 <system-reminder>...</system-reminder> 块
	result := removeTagBlocks(content, "system-reminder")
	// 移除 <env>...</env> 块
	result = removeTagBlocks(result, "env")
	// 移除 <claude_background_info>...</claude_background_info> 块
	result = removeTagBlocks(result, "claude_background_info")
	// 移除 <claudeMd>...</claudeMd> 块
	result = removeTagBlocks(result, "claudeMd")
	// 去除首尾空白
	return strings.TrimSpace(result)
}

// removeTagBlocks 移除所有 <tag>...</tag> 块
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
			// 没有闭合标签，从 openTag 到末尾都移除
			content = content[:startIdx]
			break
		}

		// 移除整个标签块
		content = content[:startIdx] + content[startIdx+endIdx+len(closeTag):]
	}

	return content
}

// extractToolCalls 从请求中提取工具调用名称列表
func extractToolCalls(req map[string]interface{}) []string {
	messages, ok := req["messages"].([]interface{})
	if !ok {
		return nil
	}

	var toolNames []string
	seen := make(map[string]bool)

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		// 检查 assistant 消息中的 tool_use 块
		role, _ := msgMap["role"].(string)
		if role != "assistant" {
			continue
		}

		content, ok := msgMap["content"].([]interface{})
		if !ok {
			continue
		}

		for _, block := range content {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}

			blockType, _ := blockMap["type"].(string)
			if blockType == "tool_use" {
				if name, ok := blockMap["name"].(string); ok && !seen[name] {
					toolNames = append(toolNames, name)
					seen[name] = true
				}
			}
		}
	}

	return toolNames
}

// classifyIntent 根据消息内容和工具调用分类意图
func classifyIntent(preview string, toolCalls []string) string {
	// 优先根据工具调用分类
	if len(toolCalls) > 0 {
		return classifyByTools(toolCalls)
	}

	// 其次根据消息内容分类
	return classifyByContent(preview)
}

// classifyByTools 根据工具名称分类意图
func classifyByTools(toolCalls []string) string {
	for _, tool := range toolCalls {
		toolLower := strings.ToLower(tool)
		switch {
		case strings.Contains(toolLower, "write") || strings.Contains(toolLower, "edit"):
			return IntentFileOp
		case strings.Contains(toolLower, "read"):
			return IntentFileOp
		case strings.Contains(toolLower, "bash") || strings.Contains(toolLower, "execute"):
			return IntentToolExec
		case strings.Contains(toolLower, "grep") || strings.Contains(toolLower, "glob") || strings.Contains(toolLower, "search"):
			return IntentSearch
		}
	}
	return IntentToolExec
}

// classifyByContent 根据消息内容分类意图
func classifyByContent(preview string) string {
	if preview == "" {
		return IntentUnknown
	}

	previewLower := strings.ToLower(preview)

	switch {
	case containsAny(previewLower, []string{"写", "创建", "生成", "实现", "添加", "write", "create", "generate", "implement", "add"}):
		return IntentCodeGen
	case containsAny(previewLower, []string{"修复", "错误", "bug", "fix", "error", "debug", "问题"}):
		return IntentDebug
	case containsAny(previewLower, []string{"重构", "优化", "refactor", "optimize", "改进"}):
		return IntentRefactor
	case containsAny(previewLower, []string{"解释", "什么是", "为什么", "explain", "what is", "why", "how does"}):
		return IntentExplain
	case containsAny(previewLower, []string{"查找", "搜索", "find", "search", "grep", "在哪"}):
		return IntentSearch
	case strings.HasSuffix(strings.TrimSpace(preview), "?") || strings.HasSuffix(strings.TrimSpace(preview), "？"):
		return IntentQA
	}

	return IntentUnknown
}

// containsAny 检查字符串是否包含任意一个子串
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
