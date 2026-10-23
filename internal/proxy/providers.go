// Package proxy 观测代理：本地反向代理，透明转发 AI API 流量并记录用量。
// 只观测、不修改任何请求/响应内容；关闭开关后完全不介入实际使用。
package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Provider 一个服务商定义：按上游域名匹配，adapter 决定用量解析方式。
// adapter 目前内置 anthropic / openai / gemini 三种 API 形态，
// 市面上绝大多数服务商（DeepSeek、Kimi、GLM、通义等）都是 openai 形态，
// 因此新增服务商通常只需在注册表里加一条记录，不需要改代码。
type Provider struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	Adapter      string   `json:"adapter"` // anthropic | openai | gemini
	HostContains []string `json:"hostContains"`
}

// Registry 服务商注册表，持久化在数据目录 providers.json，可在线更新
type Registry struct {
	Version   int        `json:"version"`
	Providers []Provider `json:"providers"`

	path string
	mu   sync.RWMutex
}

// defaultRegistry 内置注册表（首次启动写入 providers.json）
const defaultRegistry = `{
  "version": 1,
  "providers": [
    {"name": "anthropic",  "displayName": "Anthropic 官方", "adapter": "anthropic", "hostContains": ["api.anthropic.com"]},
    {"name": "openai",     "displayName": "OpenAI",         "adapter": "openai",    "hostContains": ["api.openai.com"]},
    {"name": "deepseek",   "displayName": "DeepSeek",       "adapter": "openai",    "hostContains": ["api.deepseek.com"]},
    {"name": "moonshot",   "displayName": "Moonshot Kimi",  "adapter": "openai",    "hostContains": ["api.moonshot.cn"]},
    {"name": "zhipu",      "displayName": "智谱 GLM",       "adapter": "openai",    "hostContains": ["open.bigmodel.cn"]},
    {"name": "qwen",       "displayName": "通义千问",       "adapter": "openai",    "hostContains": ["dashscope.aliyuncs.com"]},
    {"name": "gemini",     "displayName": "Google Gemini",  "adapter": "gemini",    "hostContains": ["generativelanguage.googleapis.com"]},
    {"name": "openrouter", "displayName": "OpenRouter",     "adapter": "openai",    "hostContains": ["openrouter.ai"]}
  ]
}`

// LoadRegistry 读取注册表；不存在时用内置默认建一份
func LoadRegistry(dataDir string) (*Registry, error) {
	path := filepath.Join(dataDir, "providers.json")
	r := &Registry{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		data = []byte(defaultRegistry)
		if werr := os.WriteFile(path, []byte(defaultRegistry), 0644); werr != nil {
			return nil, werr
		}
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("providers.json 格式错误: %w", err)
	}
	return r, nil
}

// List 返回全部服务商定义
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Provider(nil), r.Providers...)
}

// Match 显式指定优先，否则按上游域名匹配；都不中退化为 anthropic 形态
// （代理主要服务 Claude Code 流量，Anthropic 形态是最安全的默认）
func (r *Registry) Match(explicit, upstream string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if explicit != "" {
		for _, p := range r.Providers {
			if p.Name == explicit {
				return p
			}
		}
	}
	host := upstream
	if u, err := url.Parse(upstream); err == nil && u.Host != "" {
		host = u.Host
	}
	host = strings.ToLower(host)
	for _, p := range r.Providers {
		for _, h := range p.HostContains {
			if strings.Contains(host, strings.ToLower(h)) {
				return p
			}
		}
	}
	return Provider{Name: "unknown", DisplayName: "未知服务商", Adapter: "anthropic"}
}

// Fetch 从 URL 下载新的注册表并合并（同名覆盖），落盘后返回最新列表。
// 用于"支持去下新的"：社区/自己维护的服务商定义可以直接更新进来。
func (r *Registry) Fetch(rawURL string) ([]Provider, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("URL 无效")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var remote Registry
	if err := json.Unmarshal(data, &remote); err != nil || len(remote.Providers) == 0 {
		return nil, fmt.Errorf("注册表格式错误（需要 {\"providers\": [...]}）")
	}
	for _, p := range remote.Providers {
		if p.Name == "" || p.Adapter == "" {
			return nil, fmt.Errorf("注册表条目缺少 name 或 adapter")
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	byName := map[string]int{}
	for i, p := range r.Providers {
		byName[p.Name] = i
	}
	for _, p := range remote.Providers {
		if i, ok := byName[p.Name]; ok {
			r.Providers[i] = p
		} else {
			r.Providers = append(r.Providers, p)
		}
	}
	if err := r.saveLocked(); err != nil {
		return nil, err
	}
	return append([]Provider(nil), r.Providers...), nil
}

// Add 手动添加/覆盖一个服务商定义
func (r *Registry) Add(p Provider) error {
	if p.Name == "" || p.Adapter == "" {
		return fmt.Errorf("name 和 adapter 不能为空")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, old := range r.Providers {
		if old.Name == p.Name {
			r.Providers[i] = p
			return r.saveLocked()
		}
	}
	r.Providers = append(r.Providers, p)
	return r.saveLocked()
}

func (r *Registry) saveLocked() error {
	data, err := json.MarshalIndent(struct {
		Version   int        `json:"version"`
		Providers []Provider `json:"providers"`
	}{r.Version, r.Providers}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0644)
}
