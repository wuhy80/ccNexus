// Package notify 提供跨平台的系统通知功能
package notify

// Notifier 通知接口
type Notifier interface {
	// Send 发送系统通知
	// title: 通知标题
	// message: 通知内容
	// notifyType: 通知类型 ("info", "warning", "error")
	Send(title, message, notifyType string) error
}

// DefaultNotifier 默认通知器（由平台特定实现提供）
var DefaultNotifier Notifier

// Send 使用默认通知器发送通知
func Send(title, message, notifyType string) error {
	if DefaultNotifier == nil {
		return nil
	}
	return DefaultNotifier.Send(title, message, notifyType)
}

// SendAlert 发送告警通知（故障）
func SendAlert(title, message string) error {
	return Send(title, message, "error")
}

// SendRecovery 发送恢复通知
func SendRecovery(title, message string) error {
	return Send(title, message, "info")
}

// SendWarning 发送警告通知
func SendWarning(title, message string) error {
	return Send(title, message, "warning")
}
