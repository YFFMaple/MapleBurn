package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

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

// adjustment 读取调整状态并计算缩放系数
func (s *Server) adjustment() adjustment {
	adj := adjustment{}
	if s.db.GetMeta("adjust_active") != "1" {
		return adj
	}
	target, err := strconv.ParseFloat(s.db.GetMeta("adjust_target"), 64)
	if err != nil || target <= 0 {
		return adj
	}
	total, err := s.db.AllTimeTotals()
	if err != nil || total.TotalTokens == 0 {
		return adj
	}
	adj.Active = true
	adj.Target = target
	adj.RealTotal = total.TotalTokens
	adj.Factor = target / float64(total.TotalTokens)
	return adj
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
	adj := s.adjustment()
	if !adj.Active {
		if total, err := s.db.AllTimeTotals(); err == nil {
			adj.RealTotal = total.TotalTokens
		}
	}
	writeJSON(w, adj)
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
	writeJSON(w, s.adjustment())
}

// handleResetAdjust 回滚到真实数据
func (s *Server) handleResetAdjust(w http.ResponseWriter, r *http.Request) {
	if err := s.db.SetMeta("adjust_active", "0"); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
