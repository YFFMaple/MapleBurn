// Package config 配置层：应用配置的读取、持久化与日志目录管理。
// 持久化到数据目录的 config.json（仅本机）。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ProxyConfig 观测代理配置：只记录转发流量，不修改任何内容
type ProxyConfig struct {
	Enabled  bool   `json:"enabled"`
	Port     int    `json:"port"`     // 监听端口（127.0.0.1）
	Upstream string `json:"upstream"` // 真实上游 base URL，如 https://api.anthropic.com
	Provider string `json:"provider"` // 服务商注册表名；空 = 按上游域名自动匹配

	// 开启代理前 CC 配置里的 ANTHROPIC_BASE_URL，用于关闭时自动还原
	PrevBaseURL       string `json:"prevBaseURL,omitempty"`
	PrevBaseURLExists bool   `json:"prevBaseURLExists,omitempty"`
}

type Config struct {
	LogDirs []string    `json:"logDirs"` // 日志目录列表，扫描时全部遍历
	Proxy   ProxyConfig `json:"proxy"`
	path    string
	mu      sync.RWMutex
}

// DefaultLogDirs 自动探测默认日志目录：
// 1. CLAUDE_CONFIG_DIR 环境变量（用户自定义过 Claude Code 配置位置时）
// 2. ~/.claude/projects（官方默认位置）
func DefaultLogDirs() []string {
	var dirs []string
	if cfg := os.Getenv("CLAUDE_CONFIG_DIR"); cfg != "" {
		dirs = append(dirs, filepath.Join(cfg, "projects"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".claude", "projects"))
	}
	return uniqueStrings(dirs)
}

func Load(dataDir string) *Config {
	c := &Config{path: filepath.Join(dataDir, "config.json")}
	data, err := os.ReadFile(c.path)
	if err == nil && json.Unmarshal(data, c) == nil && len(c.LogDirs) > 0 {
		return c
	}
	c.LogDirs = DefaultLogDirs()
	return c
}

func (c *Config) Save() error {
	c.mu.RLock()
	data, err := json.MarshalIndent(struct {
		LogDirs []string    `json:"logDirs"`
		Proxy   ProxyConfig `json:"proxy"`
	}{LogDirs: c.LogDirs, Proxy: c.Proxy}, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

// ProxyCfg 返回代理配置副本；端口缺省补 17841
func (c *Config) ProxyCfg() ProxyConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p := c.Proxy
	if p.Port == 0 {
		p.Port = 17841
	}
	return p
}

// SetProxy 更新代理配置（不落盘，调用方负责 Save）
func (c *Config) SetProxy(p ProxyConfig) {
	c.mu.Lock()
	c.Proxy = p
	c.mu.Unlock()
}

// Dirs 返回当前日志目录列表的副本
func (c *Config) Dirs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.LogDirs...)
}

// AddLogDir 添加目录；返回是否真正新增（重复或空则 false）
func (c *Config) AddLogDir(dir string) bool {
	dir = normalizeDir(dir)
	if dir == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range c.LogDirs {
		if strings.EqualFold(d, dir) {
			return false
		}
	}
	c.LogDirs = append(c.LogDirs, dir)
	return true
}

// RemoveLogDir 删除目录；不允许删光
func (c *Config) RemoveLogDir(dir string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.LogDirs) <= 1 {
		return false
	}
	for i, d := range c.LogDirs {
		if strings.EqualFold(d, dir) {
			c.LogDirs = append(c.LogDirs[:i], c.LogDirs[i+1:]...)
			return true
		}
	}
	return false
}

// ccSettingsPath 返回 Claude Code settings.json 的候选路径
// （第一个存在的；都不存在时返回默认位置，供新建用）
func ccSettingsPath() string {
	def := ""
	if home, err := os.UserHomeDir(); err == nil {
		def = filepath.Join(home, ".claude", "settings.json")
	}
	var candidates []string
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "settings.json"))
	}
	if def != "" {
		candidates = append(candidates, def)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return def
}

// readCCSettings 读取 settings.json 为 map（保留未知字段，写回时不丢配置）
func readCCSettings() (map[string]any, string, error) {
	path := ccSettingsPath()
	m := map[string]any{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return m, path, nil
	}
	if err != nil {
		return nil, path, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, path, fmt.Errorf("settings.json 不是合法 JSON: %w", err)
	}
	return m, path, nil
}

// CCBaseURL 读取 Claude Code 当前的 ANTHROPIC_BASE_URL
func CCBaseURL() (url string, exists bool) {
	m, _, err := readCCSettings()
	if err != nil {
		return "", false
	}
	env, _ := m["env"].(map[string]any)
	v, _ := env["ANTHROPIC_BASE_URL"].(string)
	v = strings.TrimRight(strings.TrimSpace(v), "/")
	return v, v != ""
}

// writeCCBaseURL 修改 settings.json 的 env.ANTHROPIC_BASE_URL；
// url 为空则删除该键。首次修改前备份原文件为 settings.json.aitoken-backup
// （备份只在不存在时创建，保证留的是最初的手工配置）
func writeCCBaseURL(url string) error {
	m, path, err := readCCSettings()
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("无法定位 Claude Code 配置目录")
	}
	if _, err := os.Stat(path); err == nil {
		bak := path + ".aitoken-backup"
		if _, err := os.Stat(bak); os.IsNotExist(err) {
			if data, err := os.ReadFile(path); err == nil {
				os.WriteFile(bak, data, 0644)
			}
		}
	}
	env, _ := m["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
		m["env"] = env
	}
	if url == "" {
		delete(env, "ANTHROPIC_BASE_URL")
	} else {
		env["ANTHROPIC_BASE_URL"] = url
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SetCCBaseURL 把 CC 的 base URL 指向指定地址
func SetCCBaseURL(url string) error { return writeCCBaseURL(url) }

// RestoreCCBaseURL 还原开启代理前的值；原来没有该键则删除
func RestoreCCBaseURL(prev string, existed bool) error {
	if !existed {
		return writeCCBaseURL("")
	}
	return writeCCBaseURL(prev)
}

// DetectCCUpstream 读 Claude Code 的 settings.json（只读，不修改），
// 返回其中配置的 ANTHROPIC_BASE_URL；未配置时返回官方地址
func DetectCCUpstream() string {
	const official = "https://api.anthropic.com"
	if u, ok := CCBaseURL(); ok {
		return u
	}
	return official
}

// normalizeDir 展开 ~、转为绝对路径、清理分隔符
func normalizeDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[1:])
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return filepath.Clean(dir)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
