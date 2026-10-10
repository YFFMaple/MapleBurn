// Package pricing 维护本地模型价格表（美元 / 百万 token）。
// 三层来源，优先级从高到低：用户自定义 > 社区价格表（LiteLLM，联网拉取后本地缓存）> 内置兜底。
// 仅在用户手动触发更新时访问网络，且只下载价格表，不上传任何数据。
package pricing

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Price 每百万 token 的价格
type Price struct {
	Input       float64 `json:"input"`       // 输入
	Output      float64 `json:"output"`      // 输出
	CacheCreate float64 `json:"cacheCreate"` // 缓存写入
	CacheRead   float64 `json:"cacheRead"`   // 缓存读取
	Custom      bool    `json:"custom"`      // 是否来自用户覆盖
}

// 社区价格表地址（LiteLLM 维护，按可用性排序：jsdelivr 国内可达性更好）
var communityURLs = []string{
	"https://cdn.jsdelivr.net/gh/BerriAI/litellm@main/model_prices_and_context_window.json",
	"https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json",
}

// 内置兜底价格（官方价；缓存价格按惯例：写入=1.25×输入，读取=0.1×输入）
var builtin = map[string]Price{
	"claude-opus-5-5":   {Input: 4, Output: 20, CacheCreate: 5, CacheRead: 0.4},
	"claude-opus-5":     {Input: 5, Output: 25, CacheCreate: 6.25, CacheRead: 0.5},
	"claude-opus-4-8":   {Input: 5, Output: 25, CacheCreate: 6.25, CacheRead: 0.5},
	"claude-opus-4-7":   {Input: 5, Output: 25, CacheCreate: 6.25, CacheRead: 0.5},
	"claude-opus-4-6":   {Input: 5, Output: 25, CacheCreate: 6.25, CacheRead: 0.5},
	"claude-sonnet-5-5": {Input: 2, Output: 10, CacheCreate: 2.5, CacheRead: 0.2},
	"claude-sonnet-5":   {Input: 2, Output: 10, CacheCreate: 2.5, CacheRead: 0.2},
	"claude-sonnet-4-6": {Input: 3, Output: 15, CacheCreate: 3.75, CacheRead: 0.3},
	"claude-sonnet-4-5": {Input: 3, Output: 15, CacheCreate: 3.75, CacheRead: 0.3},
	"claude-haiku-5-5":  {Input: 0.1, Output: 0.5, CacheCreate: 0.125, CacheRead: 0.01},
	"claude-haiku-4-5":  {Input: 1, Output: 5, CacheCreate: 1.25, CacheRead: 0.1},
	"gpt-4o":            {Input: 2.5, Output: 10},
	"gpt-4o-mini":       {Input: 0.15, Output: 0.6},
}

var dateSuffix = regexp.MustCompile(`-\d{8}$`)
var bracketSuffix = regexp.MustCompile(`\[.*\]$`) // 如 kimi-k3[1m]

// 官方渠道前缀（带前缀匹配时优先选这些，避免命中第三方中转价）
var officialProviders = map[string]bool{
	"moonshot": true, "anthropic": true, "openai": true, "google": true,
	"gemini": true, "deepseek": true, "xai": true, "mistral": true,
	"qwen": true, "alibaba": true, "meta": true, "zhipu": true,
}

// Table 价格表
type Table struct {
	mu             sync.RWMutex
	overrides      map[string]Price // pricing.json，用户自定义
	community      map[string]Price // community_pricing.json，社区价格本地缓存
	communityIndex map[string]Price // 社区价格的裸名索引（小写，如 kimi-k3 → moonshot/kimi-k3 的价格）
	fetchedAt      time.Time        // 社区价格最近更新时间
	dir            string
}

type communityCache struct {
	FetchedAt time.Time        `json:"fetchedAt"`
	Prices    map[string]Price `json:"prices"`
}

// Load 加载本地缓存的价格数据；文件不存在时只用内置兜底
func Load(dir string) *Table {
	t := &Table{
		overrides: map[string]Price{},
		community: map[string]Price{},
		dir:       dir,
	}

	if data, err := os.ReadFile(filepath.Join(dir, "pricing.json")); err == nil {
		var raw map[string]Price
		if json.Unmarshal(data, &raw) == nil {
			for k, v := range raw {
				v.Custom = true
				t.overrides[k] = v
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(dir, "community_pricing.json")); err == nil {
		var cache communityCache
		if json.Unmarshal(data, &cache) == nil {
			t.community = cache.Prices
			t.fetchedAt = cache.FetchedAt
		}
	}
	t.rebuildIndex()
	return t
}

// rebuildIndex 把 "provider/model" 形式的社区价格折叠成裸名索引，官方渠道优先。
// 这样日志里的裸模型名（如 kimi-k3）能命中 moonshot/kimi-k3 等官方条目。
func (t *Table) rebuildIndex() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.communityIndex = buildIndexLocked(t.community)
}

func buildIndexLocked(community map[string]Price) map[string]Price {
	type entry struct {
		price    Price
		official bool
	}
	best := map[string]entry{}
	for key, p := range community {
		slash := strings.LastIndex(key, "/")
		if slash < 0 {
			continue
		}
		provider := strings.ToLower(key[:slash])
		bare := strings.ToLower(key[slash+1:])
		e, exists := best[bare]
		if !exists || (officialProviders[provider] && !e.official) {
			best[bare] = entry{price: p, official: officialProviders[provider]}
		}
	}
	out := make(map[string]Price, len(best))
	for bare, e := range best {
		out[bare] = e.price
	}
	return out
}

// candidates 模型名的匹配候选：原名 → 去日期后缀 → 去方括号后缀
func candidates(model string) []string {
	out := []string{model}
	if s := dateSuffix.ReplaceAllString(model, ""); s != model {
		out = append(out, s)
	}
	if s := bracketSuffix.ReplaceAllString(model, ""); s != model {
		out = append(out, s)
	}
	return out
}

// Lookup 查询模型价格：自定义 > 社区 > 内置；先试精确/去后缀，最后试官方渠道前缀匹配
func (t *Table) Lookup(model string) (Price, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cs := candidates(model)
	for _, c := range cs {
		if p, ok := t.overrides[c]; ok {
			return p, true
		}
	}
	for _, c := range cs {
		if p, ok := t.community[c]; ok {
			return p, true
		}
	}
	for _, c := range cs {
		if p, ok := builtin[c]; ok {
			return p, true
		}
	}
	// 裸名索引：kimi-k3 → moonshot/kimi-k3（官方渠道优先）
	for _, c := range cs {
		if p, ok := t.communityIndex[strings.ToLower(c)]; ok {
			return p, true
		}
	}
	return Price{}, false
}

// Cost 按 token 数折算美元
func (t *Table) Cost(model string, input, output, cacheCreate, cacheRead int) float64 {
	p, ok := t.Lookup(model)
	if !ok {
		return 0
	}
	return (float64(input)*p.Input + float64(output)*p.Output +
		float64(cacheCreate)*p.CacheCreate + float64(cacheRead)*p.CacheRead) / 1e6
}

// All 返回合并后的完整价格表（前端展示用）
func (t *Table) All() map[string]Price {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]Price, len(builtin)+len(t.community)+len(t.overrides))
	maps.Copy(out, builtin)
	maps.Copy(out, t.community)
	maps.Copy(out, t.overrides)
	return out
}

// CommunityInfo 社区价格表状态（前端展示用）
type CommunityInfo struct {
	FetchedAt string `json:"fetchedAt"` // 空表示从未更新
	Count     int    `json:"count"`
}

func (t *Table) CommunityInfo() CommunityInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	info := CommunityInfo{Count: len(t.community)}
	if !t.fetchedAt.IsZero() {
		info.FetchedAt = t.fetchedAt.Local().Format("2006-01-02 15:04")
	}
	return info
}

// Update 联网拉取社区价格表并缓存到本地。返回模型数和来源地址。
func (t *Table) Update() (int, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var lastErr error
	for _, url := range communityURLs {
		prices, err := fetchCommunity(client, url)
		if err != nil {
			lastErr = err
			continue
		}

		t.mu.Lock()
		t.community = prices
		t.communityIndex = buildIndexLocked(prices)
		t.fetchedAt = time.Now()
		err = t.saveCommunityLocked()
		t.mu.Unlock()
		if err != nil {
			return 0, url, err
		}
		return len(prices), url, nil
	}
	return 0, "", fmt.Errorf("所有价格源均不可用: %w", lastErr)
}

// fetchCommunity 下载并解析 LiteLLM 价格 JSON（单位：美元/token，转换为美元/百万token）
func fetchCommunity(client *http.Client, url string) (map[string]Price, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}

	var raw map[string]struct {
		Input       float64 `json:"input_cost_per_token"`
		Output      float64 `json:"output_cost_per_token"`
		CacheCreate float64 `json:"cache_creation_input_token_cost"`
		CacheRead   float64 `json:"cache_read_input_token_cost"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	prices := make(map[string]Price, len(raw))
	for model, p := range raw {
		if p.Input == 0 && p.Output == 0 {
			continue // 跳过 sample_spec 等非模型条目和免费条目
		}
		prices[model] = Price{
			Input:       p.Input * 1e6,
			Output:      p.Output * 1e6,
			CacheCreate: p.CacheCreate * 1e6,
			CacheRead:   p.CacheRead * 1e6,
		}
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("价格表解析结果为空")
	}
	return prices, nil
}

func (t *Table) saveCommunityLocked() error {
	data, err := json.Marshal(communityCache{FetchedAt: t.fetchedAt, Prices: t.community})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(t.dir, "community_pricing.json"), data, 0644)
}

// Set 新增/修改一个模型的价格并保存到本地覆盖文件
func (t *Table) Set(model string, p Price) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	p.Custom = true
	t.overrides[model] = p
	return t.saveOverridesLocked()
}

// Delete 删除一个用户覆盖项
func (t *Table) Delete(model string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.overrides, model)
	return t.saveOverridesLocked()
}

func (t *Table) saveOverridesLocked() error {
	data, err := json.MarshalIndent(t.overrides, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(t.dir, "pricing.json"), data, 0644)
}
