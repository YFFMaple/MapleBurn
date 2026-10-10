package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config 应用配置，持久化到数据目录的 config.json（仅本机）
type Config struct {
	LogDirs []string `json:"logDirs"` // 日志目录列表，扫描时全部遍历
	path    string
}

// defaultLogDirs 自动探测默认日志目录：
// 1. CLAUDE_CONFIG_DIR 环境变量（用户自定义过 Claude Code 配置位置时）
// 2. ~/.claude/projects（官方默认位置）
func defaultLogDirs() []string {
	var dirs []string
	if cfg := os.Getenv("CLAUDE_CONFIG_DIR"); cfg != "" {
		dirs = append(dirs, filepath.Join(cfg, "projects"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".claude", "projects"))
	}
	return uniqueStrings(dirs)
}

func loadConfig(dataDir string) *Config {
	c := &Config{path: filepath.Join(dataDir, "config.json")}
	data, err := os.ReadFile(c.path)
	if err == nil && json.Unmarshal(data, c) == nil && len(c.LogDirs) > 0 {
		return c
	}
	c.LogDirs = defaultLogDirs()
	return c
}

func (c *Config) save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

// AddLogDir 添加目录；返回是否真正新增（重复或空则 false）
func (c *Config) AddLogDir(dir string) bool {
	dir = normalizeDir(dir)
	if dir == "" {
		return false
	}
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
