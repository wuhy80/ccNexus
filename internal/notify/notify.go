// Package notify 提供跨平台的系统通知功能
package notify

import (
	"fmt"
	"sync"
	"time"
)

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

// batchedNotification 批量通知项
type batchedNotification struct {
	title      string
	message    string
	notifyType string
}

// NotificationBatcher 通知批处理器
type NotificationBatcher struct {
	notifications []batchedNotification
	mu            sync.Mutex
	timer         *time.Timer
	batchWindow   time.Duration // 批处理窗口时间
}

// defaultBatcher 默认批处理器
var defaultBatcher *NotificationBatcher

func init() {
	// 创建默认批处理器，批处理窗口为 2 秒
	defaultBatcher = &NotificationBatcher{
		notifications: make([]batchedNotification, 0),
		batchWindow:   2 * time.Second,
	}
}

// addNotification 添加通知到批处理队列
func (b *NotificationBatcher) addNotification(title, message, notifyType string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 添加到队列
	b.notifications = append(b.notifications, batchedNotification{
		title:      title,
		message:    message,
		notifyType: notifyType,
	})

	// 如果定时器已存在，重置它
	if b.timer != nil {
		b.timer.Stop()
	}

	// 启动新的定时器
	b.timer = time.AfterFunc(b.batchWindow, func() {
		b.flush()
	})
}

// flush 发送批量通知
func (b *NotificationBatcher) flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.notifications) == 0 {
		return
	}

	// 按类型分组
	recoveries := make([]string, 0)
	failures := make([]string, 0)
	warnings := make([]string, 0)
	others := make([]string, 0)

	for _, n := range b.notifications {
		switch n.notifyType {
		case "info":
			recoveries = append(recoveries, n.message)
		case "error":
			failures = append(failures, n.message)
		case "warning":
			warnings = append(warnings, n.message)
		default:
			others = append(others, n.message)
		}
	}

	// 发送合并后的通知
	if len(recoveries) > 0 {
		if len(recoveries) == 1 {
			sendDirect("ccNexus", recoveries[0], "info")
		} else {
			message := formatBatchMessage("恢复", recoveries)
			sendDirect("ccNexus", message, "info")
		}
	}

	if len(failures) > 0 {
		if len(failures) == 1 {
			sendDirect("ccNexus", failures[0], "error")
		} else {
			message := formatBatchMessage("故障", failures)
			sendDirect("ccNexus", message, "error")
		}
	}

	if len(warnings) > 0 {
		if len(warnings) == 1 {
			sendDirect("ccNexus", warnings[0], "warning")
		} else {
			message := formatBatchMessage("警告", warnings)
			sendDirect("ccNexus", message, "warning")
		}
	}

	if len(others) > 0 {
		for _, msg := range others {
			sendDirect("ccNexus", msg, "info")
		}
	}

	// 清空队列
	b.notifications = b.notifications[:0]
	b.timer = nil
}

// formatBatchMessage 格式化批量消息
func formatBatchMessage(category string, messages []string) string {
	if len(messages) <= 3 {
		// 3个或更少，直接列出
		result := ""
		for i, msg := range messages {
			if i > 0 {
				result += "\n"
			}
			result += msg
		}
		return result
	}
	// 超过3个，显示前2个和总数
	result := messages[0] + "\n" + messages[1] + "\n"
	result += fmt.Sprintf("等共 %d 个端点%s", len(messages), category)
	return result
}

// sendDirect 直接发送通知（不经过批处理）
func sendDirect(title, message, notifyType string) error {
	if DefaultNotifier == nil {
		return nil
	}
	return DefaultNotifier.Send(title, message, notifyType)
}

// Send 使用默认通知器发送通知（经过批处理）
func Send(title, message, notifyType string) error {
	if DefaultNotifier == nil {
		return nil
	}
	// 添加到批处理队列
	defaultBatcher.addNotification(title, message, notifyType)
	return nil
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
