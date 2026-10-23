//go:build windows

package main

import (
	"sync"

	"golang.org/x/sys/windows"
)

// closeCb 必须持有引用，防止 GC 回收回调
var (
	closeCb     uintptr
	closeCbOnce sync.Once
)

// registerCloseCleanup 挂控制台 Ctrl 事件回调。
// 直接点窗口 X 关闭时 Windows 不发 SIGINT，进程会被直接结束；
// 在这里拦截 CLOSE/LOGOFF/SHUTDOWN，先跑清理（还原 CC 配置）再让系统结束进程。
func registerCloseCleanup(f func()) {
	closeCbOnce.Do(func() {
		closeCb = windows.NewCallback(func(event uint32) uintptr {
			const (
				ctrlCloseEvent    = 2
				ctrlLogoffEvent   = 5
				ctrlShutdownEvent = 6
			)
			if event == ctrlCloseEvent || event == ctrlLogoffEvent || event == ctrlShutdownEvent {
				f()
			}
			return 0 // 交还系统，继续正常退出流程
		})
		windows.NewLazySystemDLL("kernel32.dll").
			NewProc("SetConsoleCtrlHandler").
			Call(closeCb, 1)
	})
}
