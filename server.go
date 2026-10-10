package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"aitoken/internal/pricing"
	"aitoken/internal/scanner"
	"aitoken/internal/store"
)

// Server 本地 HTTP API，只服务 127.0.0.1
type Server struct {
	db      *store.DB
	scanner *scanner.Scanner
	prices  *pricing.Table
	dataDir string
	config  *Config
	scanMu  sync.Mutex // 扫描串行化，防止并发触发
}

func (s *Server) registerAPI(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/scan", s.handleScan)
	mux.HandleFunc("POST /api/rescan", s.handleRescan)
	mux.HandleFunc("GET /api/overview", s.handleOverview)
	mux.HandleFunc("GET /api/range", s.handleRange)
	mux.HandleFunc("GET /api/daily", s.handleDaily)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("GET /api/series", s.handleSeries)
	mux.HandleFunc("GET /api/projects", s.handleProjects)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/session-messages", s.handleSessionMessages)
	mux.HandleFunc("GET /api/logdirs", s.handleGetLogDirs)
	mux.HandleFunc("POST /api/logdirs", s.handleAddLogDir)
	mux.HandleFunc("DELETE /api/logdirs", s.handleRemoveLogDir)
	mux.HandleFunc("GET /api/pricing", s.handleGetPricing)
	mux.HandleFunc("POST /api/pricing", s.handleSetPrice)
	mux.HandleFunc("DELETE /api/pricing", s.handleDeletePrice)
	mux.HandleFunc("GET /api/pricing/community", s.handleCommunityInfo)
	mux.HandleFunc("POST /api/pricing/update", s.handleUpdatePricing)
	mux.HandleFunc("POST /api/reprice", s.handleReprice)
	mux.HandleFunc("GET /api/adjust", s.handleGetAdjust)
	mux.HandleFunc("POST /api/adjust", s.handleSetAdjust)
	mux.HandleFunc("DELETE /api/adjust", s.handleResetAdjust)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// ---- 扫描 ----

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	res, err := s.scanner.Scan()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, res)
}

// ---- 完全重新扫描 ----

func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if err := s.db.ResetAll(); err != nil {
		writeErr(w, err)
		return
	}
	res, err := s.scanner.Scan()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, res)
}

// ---- 概览 ----

type overview struct {
	Today   store.Totals `json:"today"`
	Week    store.Totals `json:"week"`
	Month   store.Totals `json:"month"`
	AllTime store.Totals `json:"allTime"`
	DataDir string       `json:"dataDir"`
	LogDir  string       `json:"logDir"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	today := now.Format("2006-01-02")
	weekStart := now.AddDate(0, 0, -int(now.Weekday()+6)%7).Format("2006-01-02") // 本周一
	monthStart := now.Format("2006-01") + "-01"

	var ov overview
	var err error
	if ov.Today, err = s.db.TotalsForDate(today); err != nil {
		writeErr(w, err)
		return
	}
	if ov.Week, err = s.db.TotalsBetween(weekStart, today); err != nil {
		writeErr(w, err)
		return
	}
	if ov.Month, err = s.db.TotalsBetween(monthStart, today); err != nil {
		writeErr(w, err)
		return
	}
	if ov.AllTime, err = s.db.AllTimeTotals(); err != nil {
		writeErr(w, err)
		return
	}
	if adj := s.adjustment(); adj.Active {
		scaleTotals(&ov.Today, adj.Factor)
		scaleTotals(&ov.Week, adj.Factor)
		scaleTotals(&ov.Month, adj.Factor)
		scaleTotals(&ov.AllTime, adj.Factor)
	}
	ov.DataDir = s.dataDir
	ov.LogDir = strings.Join(s.scanner.Dirs(), " ; ")
	writeJSON(w, ov)
}

// ---- 区间统计 ----

// dateRange 从查询参数解析日期区间；缺省为最近 30 天
func dateRange(r *http.Request) (string, string, error) {
	start, end := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	if start == "" && end == "" {
		end = time.Now().Format("2006-01-02")
		start = time.Now().AddDate(0, 0, -29).Format("2006-01-02")
		return start, end, nil
	}
	if _, err := time.Parse("2006-01-02", start); err != nil {
		return "", "", fmt.Errorf("start 日期格式应为 YYYY-MM-DD")
	}
	if _, err := time.Parse("2006-01-02", end); err != nil {
		return "", "", fmt.Errorf("end 日期格式应为 YYYY-MM-DD")
	}
	return start, end, nil
}

func (s *Server) handleRange(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t, err := s.db.TotalsBetween(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	if adj := s.adjustment(); adj.Active {
		scaleTotals(&t, adj.Factor)
	}
	writeJSON(w, map[string]any{"start": start, "end": end, "totals": t})
}

// ---- 日报 ----

type dailyReport struct {
	Date   string            `json:"date"`
	Totals store.Totals      `json:"totals"`
	Models []store.ModelStat `json:"models"`
}

func (s *Server) handleDaily(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	rep := dailyReport{Date: date}
	var err error
	if rep.Totals, err = s.db.TotalsForDate(date); err != nil {
		writeErr(w, err)
		return
	}
	if rep.Models, err = s.db.ModelsForDate(date); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, rep)
}

// handleModels 日期区间的模型明细（总量 + 按模型分组）
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	totals, err := s.db.TotalsBetween(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	models, err := s.db.ModelsBetween(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	if adj := s.adjustment(); adj.Active {
		scaleTotals(&totals, adj.Factor)
		for i := range models {
			scaleModel(&models[i], adj.Factor)
		}
	}
	writeJSON(w, map[string]any{"start": start, "end": end, "totals": totals, "models": models})
}

// ---- 趋势 ----

func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("start") != "" || r.URL.Query().Get("end") != "" {
		start, end, err := dateRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		series, err := s.db.DailySeriesBetween(start, end)
		if err != nil {
			writeErr(w, err)
			return
		}
		if adj := s.adjustment(); adj.Active {
			for i := range series {
				scaleDay(&series[i], adj.Factor)
			}
		}
		writeJSON(w, series)
		return
	}
	days := queryInt(r, "days", 30)
	if days <= 0 || days > 365 {
		days = 30
	}
	series, err := s.db.DailySeries(days)
	if err != nil {
		writeErr(w, err)
		return
	}
	if adj := s.adjustment(); adj.Active {
		for i := range series {
			scaleDay(&series[i], adj.Factor)
		}
	}
	writeJSON(w, series)
}

// ---- 项目 ----

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("start") != "" || r.URL.Query().Get("end") != "" {
		start, end, err := dateRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		list, err := s.db.ProjectTotalsBetween(start, end)
		if err != nil {
			writeErr(w, err)
			return
		}
		if adj := s.adjustment(); adj.Active {
			for i := range list {
				scaleProject(&list[i], adj.Factor)
			}
		}
		writeJSON(w, list)
		return
	}
	days := queryInt(r, "days", 30)
	if days <= 0 || days > 365 {
		days = 30
	}
	list, err := s.db.ProjectTotals(days)
	if err != nil {
		writeErr(w, err)
		return
	}
	if adj := s.adjustment(); adj.Active {
		for i := range list {
			scaleProject(&list[i], adj.Factor)
		}
	}
	writeJSON(w, list)
}

// ---- 会话 ----

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	list, err := s.db.SessionsBetween(start, end, queryInt(r, "limit", 50))
	if err != nil {
		writeErr(w, err)
		return
	}
	if adj := s.adjustment(); adj.Active {
		for i := range list {
			scaleSession(&list[i], adj.Factor)
		}
	}
	writeJSON(w, list)
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	session := r.URL.Query().Get("session")
	msgs, err := s.scanner.ReadSessionMessages(project, session)
	if err != nil {
		writeErr(w, err)
		return
	}
	// 校准模式下消息级 token 也按同一系数缩放，与会话汇总保持一致
	if adj := s.adjustment(); adj.Active {
		for i := range msgs {
			msgs[i].InputTokens = int(math.Round(float64(msgs[i].InputTokens) * adj.Factor))
			msgs[i].OutputTokens = int(math.Round(float64(msgs[i].OutputTokens) * adj.Factor))
		}
	}
	writeJSON(w, msgs)
}

// ---- 日志目录管理 ----

// LogDirInfo 目录及其状态
type LogDirInfo struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

func (s *Server) logDirInfos() []LogDirInfo {
	dirs := s.scanner.Dirs()
	infos := make([]LogDirInfo, 0, len(dirs))
	for _, d := range dirs {
		_, err := os.Stat(d)
		infos = append(infos, LogDirInfo{Path: d, Exists: err == nil})
	}
	return infos
}

func (s *Server) handleGetLogDirs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"dirs":       s.logDirInfos(),
		"defaultDir": defaultLogDirs(),
	})
}

func (s *Server) handleAddLogDir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}
	if !s.config.AddLogDir(req.Path) {
		http.Error(w, "目录已存在或为空", http.StatusBadRequest)
		return
	}
	if err := s.config.save(); err != nil {
		writeErr(w, err)
		return
	}
	s.scanner.SetDirs(s.config.LogDirs)
	writeJSON(w, map[string]any{"dirs": s.logDirInfos()})
}

func (s *Server) handleRemoveLogDir(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	if !s.config.RemoveLogDir(dir) {
		http.Error(w, "目录不存在，或至少需要保留一个目录", http.StatusBadRequest)
		return
	}
	if err := s.config.save(); err != nil {
		writeErr(w, err)
		return
	}
	s.scanner.SetDirs(s.config.LogDirs)
	writeJSON(w, map[string]any{"dirs": s.logDirInfos()})
}

// ---- 价格表 ----

func (s *Server) handleGetPricing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.prices.All())
}

type setPriceReq struct {
	Model       string  `json:"model"`
	Input       float64 `json:"input"`
	Output      float64 `json:"output"`
	CacheCreate float64 `json:"cacheCreate"`
	CacheRead   float64 `json:"cacheRead"`
}

func (s *Server) handleSetPrice(w http.ResponseWriter, r *http.Request) {
	var req setPriceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Model == "" {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}
	if err := s.prices.Set(req.Model, pricing.Price{
		Input: req.Input, Output: req.Output,
		CacheCreate: req.CacheCreate, CacheRead: req.CacheRead,
	}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleDeletePrice(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	if model == "" {
		http.Error(w, "缺少 model 参数", http.StatusBadRequest)
		return
	}
	if err := s.prices.Delete(model); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleCommunityInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.prices.CommunityInfo())
}

// handleUpdatePricing 联网拉取社区价格表（全程序唯一访问网络的接口，且只下载不上传）
func (s *Server) handleUpdatePricing(w http.ResponseWriter, r *http.Request) {
	count, source, err := s.prices.Update()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"count": count, "source": source})
}

// ---- 重算费用 ----

func (s *Server) handleReprice(w http.ResponseWriter, r *http.Request) {
	usages, err := s.db.AllForReprice()
	if err != nil {
		writeErr(w, err)
		return
	}
	updates := make([]store.CostUpdate, 0, len(usages))
	for _, u := range usages {
		updates = append(updates, store.CostUpdate{
			Hash: u.Hash,
			Cost: s.prices.Cost(u.Model, u.InputTokens, u.OutputTokens, u.CacheCreate, u.CacheRead),
		})
	}
	n, err := s.db.UpdateCosts(updates)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]int{"updated": n})
}

func queryInt(r *http.Request, key string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil {
		return v
	}
	return def
}
