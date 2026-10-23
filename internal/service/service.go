// Package service 业务逻辑层：编排 store / scanner / config 完成用例，
// 包含校准缩放等业务规则。不感知 HTTP，向上由 server 层调用。
package service

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YFFMaple/MapleBurn/internal/config"
	"github.com/YFFMaple/MapleBurn/internal/proxy"
	"github.com/YFFMaple/MapleBurn/internal/scanner"
	"github.com/YFFMaple/MapleBurn/internal/store"
)

type Service struct {
	db      *store.DB
	scanner *scanner.Scanner
	config  *config.Config
	dataDir string
	proxy   *proxy.Manager

	scanMu sync.Mutex // 扫描串行化，防止并发触发

	adjMu    sync.Mutex // 校准系数短 TTL 缓存
	adjCache Adjustment
	adjAt    time.Time
}

func New(db *store.DB, sc *scanner.Scanner, cfg *config.Config, dataDir string, pm *proxy.Manager) *Service {
	return &Service{db: db, scanner: sc, config: cfg, dataDir: dataDir, proxy: pm}
}

// ---- 扫描 ----

// Scan 增量扫描全部日志目录
func (s *Service) Scan() (scanner.Result, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	return s.scanner.Scan()
}

// Rescan 清空记录后完全重新扫描
func (s *Service) Rescan() (scanner.Result, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if err := s.db.ResetAll(); err != nil {
		return scanner.Result{}, err
	}
	return s.scanner.Scan()
}

// ---- 统计查询：从 store 取数，按当前校准状态缩放后返回 ----

// Overview 概览页数据
type Overview struct {
	Today   store.Totals `json:"today"`
	Week    store.Totals `json:"week"`
	Month   store.Totals `json:"month"`
	AllTime store.Totals `json:"allTime"`
	DataDir string       `json:"dataDir"`
	LogDir  string       `json:"logDir"`
}

func (s *Service) Overview() (*Overview, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	weekStart := now.AddDate(0, 0, -int(now.Weekday()+6)%7).Format("2006-01-02") // 本周一
	monthStart := now.Format("2006-01") + "-01"

	ov := &Overview{}
	var err error
	if ov.Today, err = s.db.TotalsForDate(today); err != nil {
		return nil, err
	}
	if ov.Week, err = s.db.TotalsBetween(weekStart, today); err != nil {
		return nil, err
	}
	if ov.Month, err = s.db.TotalsBetween(monthStart, today); err != nil {
		return nil, err
	}
	if ov.AllTime, err = s.db.AllTimeTotals(); err != nil {
		return nil, err
	}
	// 复用刚查出的 AllTime，避免 adjustment 内部再全表聚合一次
	if adj := s.adjustmentFor(ov.AllTime); adj.Active {
		scaleTotals(&ov.Today, adj.Factor)
		scaleTotals(&ov.Week, adj.Factor)
		scaleTotals(&ov.Month, adj.Factor)
		scaleTotals(&ov.AllTime, adj.Factor)
	}
	ov.DataDir = s.dataDir
	ov.LogDir = strings.Join(s.scanner.Dirs(), " ; ")
	return ov, nil
}

// RangeTotals 日期区间总计
func (s *Service) RangeTotals(start, end string) (store.Totals, error) {
	t, err := s.db.TotalsBetween(start, end)
	if err != nil {
		return t, err
	}
	if adj := s.Adjustment(); adj.Active {
		scaleTotals(&t, adj.Factor)
	}
	return t, nil
}

// DailyReport 某一天的日报
type DailyReport struct {
	Date   string            `json:"date"`
	Totals store.Totals      `json:"totals"`
	Models []store.ModelStat `json:"models"`
}

func (s *Service) DailyReport(date string) (*DailyReport, error) {
	rep := &DailyReport{Date: date}
	var err error
	if rep.Totals, err = s.db.TotalsForDate(date); err != nil {
		return nil, err
	}
	if rep.Models, err = s.db.ModelsForDate(date); err != nil {
		return nil, err
	}
	return rep, nil
}

// ModelsBetween 日期区间的模型明细（总量 + 按模型分组）
func (s *Service) ModelsBetween(start, end string) (store.Totals, []store.ModelStat, error) {
	totals, err := s.db.TotalsBetween(start, end)
	if err != nil {
		return totals, nil, err
	}
	models, err := s.db.ModelsBetween(start, end)
	if err != nil {
		return totals, nil, err
	}
	if adj := s.Adjustment(); adj.Active {
		scaleTotals(&totals, adj.Factor)
		for i := range models {
			scaleModel(&models[i], adj.Factor)
		}
	}
	return totals, models, nil
}

// Series 日期区间逐日趋势
func (s *Service) Series(start, end string) ([]store.DayStat, error) {
	series, err := s.db.DailySeriesBetween(start, end)
	if err != nil {
		return nil, err
	}
	if adj := s.Adjustment(); adj.Active {
		for i := range series {
			scaleTotals(&series[i].Totals, adj.Factor)
		}
	}
	return series, nil
}

// Projects 日期区间按项目汇总
func (s *Service) Projects(start, end string) ([]store.ProjectStat, error) {
	list, err := s.db.ProjectTotalsBetween(start, end)
	if err != nil {
		return nil, err
	}
	if adj := s.Adjustment(); adj.Active {
		for i := range list {
			scaleTotals(&list[i].Totals, adj.Factor)
		}
	}
	return list, nil
}

// Sessions 日期区间内的会话列表
func (s *Service) Sessions(start, end string, limit int) ([]store.SessionStat, error) {
	list, err := s.db.SessionsBetween(start, end, limit)
	if err != nil {
		return nil, err
	}
	if adj := s.Adjustment(); adj.Active {
		for i := range list {
			list[i].TotalTokens = int(math.Round(float64(list[i].TotalTokens) * adj.Factor))
		}
	}
	return list, nil
}

// SessionMessages 读取会话消息；校准模式下消息级 token 也按同一系数缩放，与会话汇总保持一致
func (s *Service) SessionMessages(project, session string) ([]scanner.SessionMessage, error) {
	msgs, err := s.scanner.ReadSessionMessages(project, session)
	if err != nil {
		return nil, err
	}
	if adj := s.Adjustment(); adj.Active {
		for i := range msgs {
			msgs[i].InputTokens = int(math.Round(float64(msgs[i].InputTokens) * adj.Factor))
			msgs[i].OutputTokens = int(math.Round(float64(msgs[i].OutputTokens) * adj.Factor))
		}
	}
	return msgs, nil
}

// ---- 请求流水 ----

// Events 逐条请求记录（按时间倒序）
func (s *Service) Events(f store.EventFilter) ([]store.UsageRow, int, error) {
	list, total, err := s.db.Events(f)
	if err != nil {
		return nil, 0, err
	}
	if adj := s.Adjustment(); adj.Active {
		for i := range list {
			r := &list[i]
			r.InputTokens = int(math.Round(float64(r.InputTokens) * adj.Factor))
			r.OutputTokens = int(math.Round(float64(r.OutputTokens) * adj.Factor))
			r.CacheCreate = int(math.Round(float64(r.CacheCreate) * adj.Factor))
			r.CacheRead = int(math.Round(float64(r.CacheRead) * adj.Factor))
		}
	}
	return list, total, nil
}

// ---- 时段热力图 ----

// Heatmap 周一到周日 × 0-23 时的 token 分布
func (s *Service) Heatmap(start, end string) ([][]int64, error) {
	grid, err := s.db.Heatmap(start, end)
	if err != nil {
		return nil, err
	}
	if adj := s.Adjustment(); adj.Active {
		for d := range grid {
			for h := range grid[d] {
				grid[d][h] = int64(math.Round(float64(grid[d][h]) * adj.Factor))
			}
		}
	}
	return grid, nil
}

// ---- 工具使用统计 ----

// Tools 区间内按工具汇总（调用次数是真实计数，不随校准缩放）
func (s *Service) Tools(start, end string) ([]store.ToolStat, error) {
	return s.db.ToolStats(start, end)
}

// ---- 周期报告 ----

// Report 本周/本月的对比报告数据
type Report struct {
	Period     string            `json:"period"` // week | month
	Start      string            `json:"start"`
	End        string            `json:"end"`
	Totals     store.Totals      `json:"totals"`
	PrevTotals store.Totals      `json:"prevTotals"`
	ActiveDays int               `json:"activeDays"`
	Models     []store.ModelStat `json:"models"` // top 模型
}

func (s *Service) Report(period string) (*Report, error) {
	now := time.Now()
	var start, prevStart, prevEnd time.Time
	switch period {
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		prevStart = start.AddDate(0, -1, 0)
		prevEnd = start.AddDate(0, 0, -1)
	default: // week
		period = "week"
		monday := now.AddDate(0, 0, -int(now.Weekday()+6)%7)
		start = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.Local)
		prevStart = start.AddDate(0, 0, -7)
		prevEnd = start.AddDate(0, 0, -1)
	}
	fmtDate := func(t time.Time) string { return t.Format("2006-01-02") }
	end := fmtDate(now)

	rep := &Report{Period: period, Start: fmtDate(start), End: end}
	var err error
	if rep.Totals, err = s.db.TotalsBetween(fmtDate(start), end); err != nil {
		return nil, err
	}
	if rep.PrevTotals, err = s.db.TotalsBetween(fmtDate(prevStart), fmtDate(prevEnd)); err != nil {
		return nil, err
	}
	if rep.ActiveDays, err = s.db.ActiveDays(fmtDate(start), end); err != nil {
		return nil, err
	}
	if rep.Models, err = s.db.ModelsBetween(fmtDate(start), end); err != nil {
		return nil, err
	}
	if len(rep.Models) > 5 {
		rep.Models = rep.Models[:5]
	}
	if adj := s.Adjustment(); adj.Active {
		scaleTotals(&rep.Totals, adj.Factor)
		scaleTotals(&rep.PrevTotals, adj.Factor)
		for i := range rep.Models {
			scaleModel(&rep.Models[i], adj.Factor)
		}
	}
	return rep, nil
}

// ---- 日志目录管理 ----

// LogDirInfo 目录及其状态
type LogDirInfo struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

// LogDirs 当前日志目录（含存在性状态）和自动探测的默认目录
func (s *Service) LogDirs() (dirs []LogDirInfo, defaults []string) {
	return s.logDirInfos(), config.DefaultLogDirs()
}

func (s *Service) logDirInfos() []LogDirInfo {
	dirs := s.scanner.Dirs()
	infos := make([]LogDirInfo, 0, len(dirs))
	for _, d := range dirs {
		_, err := os.Stat(d)
		infos = append(infos, LogDirInfo{Path: d, Exists: err == nil})
	}
	return infos
}

// AddLogDir 添加日志目录并持久化，返回最新的目录列表
func (s *Service) AddLogDir(path string) ([]LogDirInfo, error) {
	if !s.config.AddLogDir(path) {
		return nil, fmt.Errorf("目录已存在或为空")
	}
	if err := s.config.Save(); err != nil {
		return nil, err
	}
	s.scanner.SetDirs(s.config.Dirs())
	return s.logDirInfos(), nil
}

// RemoveLogDir 移除日志目录并持久化，返回最新的目录列表
func (s *Service) RemoveLogDir(path string) ([]LogDirInfo, error) {
	if !s.config.RemoveLogDir(path) {
		return nil, fmt.Errorf("目录不存在，或至少需要保留一个目录")
	}
	if err := s.config.Save(); err != nil {
		return nil, err
	}
	s.scanner.SetDirs(s.config.Dirs())
	return s.logDirInfos(), nil
}

// ---- 观测代理 ----

// ProxyStatus 代理当前状态
func (s *Service) ProxyStatus() proxy.Status {
	return s.proxy.Status()
}

// SetProxy 更新代理配置并立即生效。
// 开启：备份 CC 当前的 ANTHROPIC_BASE_URL，并自动把它指向代理地址；
// 关闭：自动还原备份的值（用户中途改过则不覆盖）。
// 若未填上游，自动采用开启前 CC 配置里的地址。
func (s *Service) SetProxy(p config.ProxyConfig) (proxy.Status, error) {
	if p.Port == 0 {
		p.Port = 17841
	}
	if p.Port < 1024 || p.Port > 65535 {
		return proxy.Status{}, fmt.Errorf("端口无效: %d", p.Port)
	}
	p.Upstream = strings.TrimRight(strings.TrimSpace(p.Upstream), "/")
	cur := s.config.ProxyCfg()
	addr := fmt.Sprintf("http://127.0.0.1:%d", p.Port)

	// 还原点默认沿用上次的，防止重复开关时把代理地址自己存成还原点
	p.PrevBaseURL, p.PrevBaseURLExists = cur.PrevBaseURL, cur.PrevBaseURLExists

	if p.Enabled && !cur.Enabled {
		// 开启瞬间：记录 CC 当前 base URL 作为还原点；上游缺省取它
		if ccURL, ccExists := config.CCBaseURL(); ccURL != addr {
			p.PrevBaseURL, p.PrevBaseURLExists = ccURL, ccExists
			if p.Upstream == "" && ccExists {
				p.Upstream = ccURL
			}
		}
	}
	if p.Enabled {
		if p.Upstream == "" {
			// CC 已经指向我们（如崩溃后重启）：沿用上次的真实上游
			p.Upstream = cur.Upstream
		}
		if p.Upstream == "" {
			p.Upstream = config.DetectCCUpstream()
		}
		// 上游指回代理自己会形成转发死循环，直接拒绝
		if strings.Contains(p.Upstream, fmt.Sprintf("127.0.0.1:%d", p.Port)) ||
			strings.Contains(p.Upstream, fmt.Sprintf("localhost:%d", p.Port)) {
			return proxy.Status{}, fmt.Errorf("上游不能指向代理自身（%s）", p.Upstream)
		}
	}

	s.config.SetProxy(p)
	if err := s.config.Save(); err != nil {
		return proxy.Status{}, err
	}
	// 先让代理进入目标状态，成功后再动 CC 配置，避免指向一个没起来的代理
	if err := s.proxy.Apply(); err != nil {
		return proxy.Status{}, err
	}

	if p.Enabled && !cur.Enabled {
		if err := config.SetCCBaseURL(addr); err != nil {
			return s.proxy.Status(), fmt.Errorf("代理已开启，但修改 Claude Code 配置失败: %w（请手动把 ANTHROPIC_BASE_URL 改为 %s）", err, addr)
		}
	}
	if !p.Enabled && cur.Enabled {
		// 只有 CC 仍指着我们时才还原；用户中途用别的工具改过就不动
		if ccURL, _ := config.CCBaseURL(); ccURL == addr {
			config.RestoreCCBaseURL(cur.PrevBaseURL, cur.PrevBaseURLExists)
		}
	}
	return s.proxy.Status(), nil
}

// ProxyRequests 代理转发的请求流水
func (s *Service) ProxyRequests(f store.ProxyFilter) ([]store.ProxyRequest, int, error) {
	return s.db.ProxyRequests(f)
}

// ProxySummary 区间内按 服务商+模型 汇总
func (s *Service) ProxySummary(start, end string) ([]store.ModelStat, int, error) {
	return s.db.ProxySummary(start, end)
}

// ProxyProviders 服务商注册表
func (s *Service) ProxyProviders() []proxy.Provider {
	return s.proxy.Registry().List()
}

// FetchProxyProviders 从 URL 下载注册表并合并
func (s *Service) FetchProxyProviders(url string) ([]proxy.Provider, error) {
	return s.proxy.Registry().Fetch(url)
}

// AddProxyProvider 手动添加/覆盖一个服务商
func (s *Service) AddProxyProvider(p proxy.Provider) ([]proxy.Provider, error) {
	if err := s.proxy.Registry().Add(p); err != nil {
		return nil, err
	}
	return s.proxy.Registry().List(), nil
}

// ---- 全文搜索 ----

// Search 按内容搜索历史会话消息
func (s *Service) Search(q string, limit int) ([]store.SearchHit, error) {
	return s.db.SearchMessages(q, limit)
}

// ---- 缓存命中率 / 效率 ----

// ModelEfficiency 单模型的效率指标
type ModelEfficiency struct {
	Model        string  `json:"model"`
	Requests     int     `json:"requests"`
	CacheHitRate float64 `json:"cacheHitRate"` // 0~1，缓存读占全部输入侧 token 的比例
	AvgOutput    int     `json:"avgOutput"`    // 平均每请求输出
	AvgTotal     int     `json:"avgTotal"`     // 平均每请求总 token
}

// Efficiency 区间效率汇总（用原始值计算，比率不受显示校准影响）
type Efficiency struct {
	CacheHitRate  float64           `json:"cacheHitRate"`
	AvgPerRequest int               `json:"avgPerRequest"`
	Models        []ModelEfficiency `json:"models"`
}

func (s *Service) Efficiency(start, end string) (*Efficiency, error) {
	totals, err := s.db.TotalsBetween(start, end)
	if err != nil {
		return nil, err
	}
	models, err := s.db.ModelsBetween(start, end)
	if err != nil {
		return nil, err
	}

	hitRate := func(in, cc, cr int) float64 {
		den := in + cc + cr
		if den == 0 {
			return 0
		}
		return float64(cr) / float64(den)
	}

	e := &Efficiency{CacheHitRate: hitRate(totals.InputTokens, totals.CacheCreate, totals.CacheRead)}
	if totals.Requests > 0 {
		e.AvgPerRequest = totals.TotalTokens / totals.Requests
	}
	for _, m := range models {
		me := ModelEfficiency{
			Model:        m.Model,
			Requests:     m.Requests,
			CacheHitRate: hitRate(m.InputTokens, m.CacheCreate, m.CacheRead),
		}
		if m.Requests > 0 {
			me.AvgOutput = m.OutputTokens / m.Requests
			me.AvgTotal = (m.InputTokens + m.OutputTokens + m.CacheCreate + m.CacheRead) / m.Requests
		}
		e.Models = append(e.Models, me)
	}
	return e, nil
}

// ---- 文件热点 ----

// FileStats 区间内 AI 操作最多的文件
func (s *Service) FileStats(start, end string) ([]store.FileStat, error) {
	return s.db.FileStats(start, end)
}

// ---- 显示层数据调整（不修改真实记录，查询时按比例缩放，可随时回滚） ----

// Adjustment 当前调整状态
type Adjustment struct {
	Active    bool    `json:"active"`
	Target    float64 `json:"target"`    // 目标总消耗
	RealTotal int     `json:"realTotal"` // 真实总消耗
	Factor    float64 `json:"factor"`    // 缩放系数
}

// adjustTTL 系数缓存有效期：扫描等操作改变总量后，最多延迟该时间反映到系数上
const adjustTTL = 3 * time.Second

// Adjustment 读取调整状态并计算缩放系数（带短 TTL 缓存，避免每个请求都全表聚合）
func (s *Service) Adjustment() Adjustment {
	s.adjMu.Lock()
	defer s.adjMu.Unlock()
	if time.Since(s.adjAt) < adjustTTL {
		return s.adjCache
	}
	total, err := s.db.AllTimeTotals()
	if err != nil {
		return Adjustment{}
	}
	s.adjCache = s.adjustmentFor(total)
	s.adjAt = time.Now()
	return s.adjCache
}

// adjustmentFor 用已知的全量总计计算缩放系数（调用方已查过 AllTimeTotals 时避免重复聚合）
func (s *Service) adjustmentFor(total store.Totals) Adjustment {
	adj := Adjustment{RealTotal: total.TotalTokens}
	if s.db.GetMeta("adjust_active") != "1" {
		return adj
	}
	target, err := strconv.ParseFloat(s.db.GetMeta("adjust_target"), 64)
	if err != nil || target <= 0 || total.TotalTokens == 0 {
		return adj
	}
	adj.Active = true
	adj.Target = target
	adj.Factor = target / float64(total.TotalTokens)
	return adj
}

// invalidateAdjust 设置/重置后让缓存立即失效
func (s *Service) invalidateAdjust() {
	s.adjMu.Lock()
	s.adjCache = Adjustment{}
	s.adjAt = time.Time{}
	s.adjMu.Unlock()
}

// SetAdjust 解析目标值并启用校准，返回生效后的状态
func (s *Service) SetAdjust(target string) (Adjustment, error) {
	amount, err := parseTokenAmount(target)
	if err != nil {
		return Adjustment{}, err
	}
	if err := s.db.SetMeta("adjust_target", strconv.FormatFloat(amount, 'f', 0, 64)); err != nil {
		return Adjustment{}, err
	}
	if err := s.db.SetMeta("adjust_active", "1"); err != nil {
		return Adjustment{}, err
	}
	s.invalidateAdjust()
	return s.Adjustment(), nil
}

// ResetAdjust 回滚到真实数据
func (s *Service) ResetAdjust() error {
	if err := s.db.SetMeta("adjust_active", "0"); err != nil {
		return err
	}
	s.invalidateAdjust()
	return nil
}

// parseTokenAmount 解析人类可读的 token 数：3B / 2.5M / 500k / 30亿 / 500万 / 3000000000
func parseTokenAmount(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("不能为空")
	}
	multipliers := []struct {
		suffix string
		mult   float64
	}{
		{"亿", 1e8}, {"万", 1e4},
		{"b", 1e9}, {"m", 1e6}, {"k", 1e3},
	}
	mult := 1.0
	lower := strings.ToLower(s)
	for _, m := range multipliers {
		if strings.HasSuffix(lower, m.suffix) {
			mult = m.mult
			s = s[:len(s)-len(m.suffix)]
			break
		}
	}
	num, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || num <= 0 {
		return 0, fmt.Errorf("无法解析数值: %q", s)
	}
	return num * mult, nil
}

// scaleTotals 等比缩放汇总（请求次数是真实调用计数，保持不变）
func scaleTotals(t *store.Totals, f float64) {
	t.InputTokens = int(math.Round(float64(t.InputTokens) * f))
	t.OutputTokens = int(math.Round(float64(t.OutputTokens) * f))
	t.CacheCreate = int(math.Round(float64(t.CacheCreate) * f))
	t.CacheRead = int(math.Round(float64(t.CacheRead) * f))
	t.TotalTokens = t.InputTokens + t.OutputTokens + t.CacheCreate + t.CacheRead
}

func scaleModel(m *store.ModelStat, f float64) {
	m.InputTokens = int(math.Round(float64(m.InputTokens) * f))
	m.OutputTokens = int(math.Round(float64(m.OutputTokens) * f))
	m.CacheCreate = int(math.Round(float64(m.CacheCreate) * f))
	m.CacheRead = int(math.Round(float64(m.CacheRead) * f))
}
