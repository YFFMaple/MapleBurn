//go:build !windows

// Package tray 系统托盘与开机自启。非 Windows 平台为空实现。
package tray

// Run 非 Windows 平台没有托盘，直接永久阻塞（进程随信号退出）
func Run(onOpen, onExit func()) { select {} }

// Quit 非 Windows 平台为空操作
func Quit() {}

func AutostartEnabled() bool  { return false }
func SetAutostart(bool) error { return nil }
