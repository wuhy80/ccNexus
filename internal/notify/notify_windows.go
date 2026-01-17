//go:build windows
// +build windows

package notify

import (
	"os/exec"
	"strings"

	"github.com/lich0821/ccNexus/internal/logger"
)

// WindowsNotifier Windows 平台通知实现
type WindowsNotifier struct {
	appName string
}

// NewWindowsNotifier 创建 Windows 通知器
func NewWindowsNotifier(appName string) *WindowsNotifier {
	return &WindowsNotifier{appName: appName}
}

// Send 发送 Windows 系统通知（使用 PowerShell Toast 通知）
func (n *WindowsNotifier) Send(title, message, notifyType string) error {
	// 使用 PowerShell 发送 Toast 通知
	// 这是 Windows 10/11 原生支持的方式
	script := buildToastScript(n.appName, title, message)

	// 使用 -WindowStyle Hidden 隐藏 PowerShell 窗口，避免黑框闪现
	cmd := exec.Command("powershell", "-WindowStyle", "Hidden", "-NoProfile", "-NonInteractive", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("Failed to send Windows notification: %v, output: %s", err, string(output))
		return err
	}

	logger.Debug("Windows notification sent: %s - %s", title, message)
	return nil
}

// buildToastScript 构建 PowerShell Toast 通知脚本
func buildToastScript(appName, title, message string) string {
	// 转义特殊字符
	title = escapeForPowerShell(title)
	message = escapeForPowerShell(message)
	appName = escapeForPowerShell(appName)

	// 使用 Windows Toast 通知 API
	script := `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null

$template = @"
<toast>
    <visual>
        <binding template="ToastText02">
            <text id="1">` + title + `</text>
            <text id="2">` + message + `</text>
        </binding>
    </visual>
</toast>
"@

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("` + appName + `").Show($toast)
`
	return script
}

// escapeForPowerShell 转义 PowerShell 特殊字符
func escapeForPowerShell(s string) string {
	// 转义可能导致问题的字符
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, "$", "`$")
	s = strings.ReplaceAll(s, "\"", "`\"")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "&", "&amp;")
	return s
}

func init() {
	// 设置默认通知器
	DefaultNotifier = NewWindowsNotifier("ccNexus")
}
