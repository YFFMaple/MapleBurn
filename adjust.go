package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aitoken/internal/store"
)

// ---- 显示层数据调整（不修改真实记录，查询时按比例缩放，可随时回滚） ----

// adjustment 当前调整状态
type adjustment struct {
	Active    bool    `json:"active"`
	Target    float64 `json:"target"`    // 目标总消耗
	RealTotal int     `json:"realTotal"` // 真实总消耗
	Factor    float64 `json:"factor"`    // 缩放系数
}

// adjustTTL 系数缓存有效期：扫描等操作改变总量后，最多延迟该时间反映到系数上
const adjustTTL = 3 * time.Second

// adjustment 读取调整状态并计算缩放系数（带短 TTL 缓存，避免每个请求都全表聚合）
func (s *Server) adjustment() adjustment {
	s.adjMu.Lock()
	defer s.adjMu.Unlock()
	if time.Since(s.adjAt) < adjustTTL {
		return s.adjCache
	}
	total, err := s.db.AllTimeTotals()
	if err != nil {
		return adjustment{}
	}
	s.adjCache = s.adjustmentFor(total)
	s.adjAt = time.Now()
	return s.adjCache
}

// adjustmentFor 用已知的全量总计计算缩放系数（调用方已查过 AllTimeTotals 时避免重复聚合）
func (s *Server) adjustmentFor(total store.Totals) adjustment {
	adj := adjustment{RealTotal: total.TotalTokens}
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
func (s *Server) invalidateAdjust() {
	s.adjMu.Lock()
	s.adjCache = adjustment{}
	s.adjAt = time.Time{}
	s.adjMu.Unlock()
}

// ---- 各类统计的缩放 ----

// scaleTotals 等比缩放汇总（请求次数是真实调用计数，保持不变）
func scaleTotals(t *store.Totals, f float64) {
	t.InputTokens = int(math.Round(float64(t.InputTokens) * f))
	t.OutputTokens = int(math.Round(float64(t.OutputTokens) * f))
	t.CacheCreate = int(math.Round(float64(t.CacheCreate) * f))
	t.CacheRead = int(math.Round(float64(t.CacheRead) * f))
	t.TotalTokens = t.InputTokens + t.OutputTokens + t.CacheCreate + t.CacheRead
	t.Cost *= f
}

func scaleModel(m *store.ModelStat, f float64) {
	m.InputTokens = int(math.Round(float64(m.InputTokens) * f))
	m.OutputTokens = int(math.Round(float64(m.OutputTokens) * f))
	m.CacheCreate = int(math.Round(float64(m.CacheCreate) * f))
	m.CacheRead = int(math.Round(float64(m.CacheRead) * f))
	m.Cost *= f
}

func scaleDay(d *store.DayStat, f float64) {
	scaleTotals(&d.Totals, f)
}

func scaleProject(p *store.ProjectStat, f float64) {
	scaleTotals(&p.Totals, f)
}

func scaleSession(s2 *store.SessionStat, f float64) {
	s2.TotalTokens = int(math.Round(float64(s2.TotalTokens) * f))
	s2.Cost *= f
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

// ---- API ----

func (s *Server) handleGetAdjust(w http.ResponseWriter, r *http.Request) {
	// adjustment 的 RealTotal 在未启用时也有值（缓存自全量聚合）
	writeJSON(w, s.adjustment())
}

func (s *Server) handleSetAdjust(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}
	amount, err := parseTokenAmount(req.Target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.db.SetMeta("adjust_target", strconv.FormatFloat(amount, 'f', 0, 64)); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.db.SetMeta("adjust_active", "1"); err != nil {
		writeErr(w, err)
		return
	}
	s.invalidateAdjust()
	writeJSON(w, s.adjustment())
}

// handleResetAdjust 回滚到真实数据
func (s *Server) handleResetAdjust(w http.ResponseWriter, r *http.Request) {
	if err := s.db.SetMeta("adjust_active", "0"); err != nil {
		writeErr(w, err)
		return
	}
	s.invalidateAdjust()
	writeJSON(w, map[string]bool{"ok": true})
}
