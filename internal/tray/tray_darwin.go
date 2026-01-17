package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"
import (
	"unsafe"
)

var (
	showWindow func()
	hideWindow func()
	quitApp    func()
)

// goShowWindow 由 Objective-C 代码调用，显示主窗口
//
//export goShowWindow
func goShowWindow() {
	if showWindow != nil {
		showWindow()
	}
}

// goHideWindow 由 Objective-C 代码调用，隐藏主窗口
//
//export goHideWindow
func goHideWindow() {
	if hideWindow != nil {
		hideWindow()
	}
}

// goQuitApp 由 Objective-C 代码调用，退出应用程序
//
//export goQuitApp
func goQuitApp() {
	if quitApp != nil {
		quitApp()
	}
}

// Setup 初始化系统托盘，使用原生 macOS API
func Setup(icon []byte, showFunc func(), hideFunc func(), quitFunc func(), language string) {
	showWindow = showFunc
	hideWindow = hideFunc
	quitApp = quitFunc

	if len(icon) > 0 {
		cLang := C.CString(language)
		defer C.free(unsafe.Pointer(cLang))
		C.setupTray(unsafe.Pointer(&icon[0]), C.int(len(icon)), cLang)
	}
}

// Quit 退出系统托盘
func Quit() {
	// 清理资源（如需要）
}

// UpdateLanguage 更新托盘菜单语言
func UpdateLanguage(language string) {
	cLang := C.CString(language)
	defer C.free(unsafe.Pointer(cLang))
	C.updateTrayLanguage(cLang)
}
