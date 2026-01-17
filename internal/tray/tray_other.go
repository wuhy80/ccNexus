//go:build !darwin && !windows && !linux
// +build !darwin,!windows,!linux

package tray

// Setup 在不支持的平台上不执行任何操作（如 FreeBSD 等）
func Setup(icon []byte, showFunc func(), hideFunc func(), quitFunc func(), language string) {
}

// Quit 退出系统托盘
func Quit() {
}

// UpdateLanguage 更新托盘菜单语言
func UpdateLanguage(language string) {
}
