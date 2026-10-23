//go:build !windows

// Package widget 非 Windows 平台没有桌面小组件，退化为浏览器打开。
package widget

import (
	"os/exec"

	"github.com/YFFMaple/MapleBurn/internal/config"
)

type Manager struct{}

func New(cfg *config.Config) *Manager { return &Manager{} }

func (m *Manager) OnStateChange(f func(open bool)) {}
func (m *Manager) IsOpen() bool                   { return false }
func (m *Manager) Close()                         {}

func (m *Manager) Toggle(url string) { m.Show(url) }

func (m *Manager) Show(url string) {
	exec.Command("open", url).Start()
}
