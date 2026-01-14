//go:build !windows && !darwin
// +build !windows,!darwin

package notify

import (
	"os/exec"
	"strings"

	"github.com/lich0821/ccNexus/internal/logger"
)

// LinuxNotifier Linux 平台通知实现
type LinuxNotifier struct {
	appName string
}

// NewLinuxNotifier 创建 Linux 通知器
func NewLinuxNotifier(appName string) *LinuxNotifier {
	return &LinuxNotifier{appName: appName}
}

// Send 发送 Linux 系统通知（使用 notify-send）
func (n *LinuxNotifier) Send(title, message, notifyType string) error {
	// 检查 notify-send 是否可用
	_, err := exec.LookPath("notify-send")
	if err != nil {
		logger.Debug("notify-send not available, skipping notification")
		return nil
	}

	// 根据通知类型设置紧急程度
	urgency := "normal"
	switch notifyType {
	case "error":
		urgency = "critical"
	case "warning":
		urgency = "normal"
	case "info":
		urgency = "low"
	}

	// 转义特殊字符
	title = escapeForShell(title)
	message = escapeForShell(message)

	cmd := exec.Command("notify-send",
		"-a", n.appName,
		"-u", urgency,
		title,
		message,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("Failed to send Linux notification: %v, output: %s", err, string(output))
		return err
	}

	logger.Debug("Linux notification sent: %s - %s", title, message)
	return nil
}

// escapeForShell 转义 Shell 特殊字符
func escapeForShell(s string) string {
	// 移除可能导致问题的字符
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	return s
}

func init() {
	// 设置默认通知器
	DefaultNotifier = NewLinuxNotifier("ccNexus")
}
