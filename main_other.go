//go:build !windows

package main

// 非 Windows 平台控制台关闭走 SIGINT，无需额外处理
func registerCloseCleanup(func()) {}
