//go:build darwin
// +build darwin

package notify

import (
	"os/exec"
	"strings"

	"github.com/lich0821/ccNexus/internal/logger"
)

// DarwinNotifier macOS 平台通知实现
type DarwinNotifier struct {
	appName string
}

// NewDarwinNotifier 创建 macOS 通知器
func NewDarwinNotifier(appName string) *DarwinNotifier {
	return &DarwinNotifier{appName: appName}
}

// Send 发送 macOS 系统通知（使用 osascript）
func (n *DarwinNotifier) Send(title, message, notifyType string) error {
	// 使用 AppleScript 发送通知
	script := buildAppleScript(n.appName, title, message)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("Failed to send macOS notification: %v, output: %s", err, string(output))
		return err
	}

	logger.Debug("macOS notification sent: %s - %s", title, message)
	return nil
}

// buildAppleScript 构建 AppleScript 通知脚本
func buildAppleScript(appName, title, message string) string {
	// 转义特殊字符
	title = escapeForAppleScript(title)
	message = escapeForAppleScript(message)
	appName = escapeForAppleScript(appName)

	return `display notification "` + message + `" with title "` + appName + `" subtitle "` + title + `"`
}

// escapeForAppleScript 转义 AppleScript 特殊字符
func escapeForAppleScript(s string) string {
	// 转义双引号和反斜杠
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func init() {
	// 设置默认通知器
	DefaultNotifier = NewDarwinNotifier("ccNexus")
}
