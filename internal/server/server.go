// Package server HTTP 层：路由、参数解析与响应序列化。
// 只做协议适配，业务规则一律调用 service 层完成。只服务 127.0.0.1。
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/YFFMaple/MapleBurn/internal/config"
	"github.com/YFFMaple/MapleBurn/internal/proxy"
	"github.com/YFFMaple/MapleBurn/internal/service"
	"github.com/YFFMaple/MapleBurn/internal/store"
	"github.com/YFFMaple/MapleBurn/internal/tray"
)

type Server struct {
	svc *service.Service
}

func New(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) RegisterAPI(mux *http.ServeMux) {
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
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/heatmap", s.handleHeatmap)
	mux.HandleFunc("GET /api/tools", s.handleTools)
	mux.HandleFunc("GET /api/report", s.handleReport)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/efficiency", s.handleEfficiency)
	mux.HandleFunc("GET /api/files", s.handleFiles)
	mux.HandleFunc("GET /api/logdirs", s.handleGetLogDirs)
	mux.HandleFunc("POST /api/logdirs", s.handleAddLogDir)
	mux.HandleFunc("DELETE /api/logdirs", s.handleRemoveLogDir)
	mux.HandleFunc("GET /api/adjust", s.handleGetAdjust)
	mux.HandleFunc("POST /api/adjust", s.handleSetAdjust)
	mux.HandleFunc("DELETE /api/adjust", s.handleResetAdjust)
	mux.HandleFunc("GET /api/proxy", s.handleGetProxy)
	mux.HandleFunc("POST /api/proxy", s.handleSetProxy)
	mux.HandleFunc("GET /api/proxy/requests", s.handleProxyRequests)
	mux.HandleFunc("GET /api/proxy/summary", s.handleProxySummary)
	mux.HandleFunc("GET /api/proxy/providers", s.handleProxyProviders)
	mux.HandleFunc("POST /api/proxy/providers", s.handleAddProxyProvider)
	mux.HandleFunc("POST /api/proxy/providers/fetch", s.handleFetchProxyProviders)
	mux.HandleFunc("GET /api/autostart", s.handleGetAutostart)
	mux.HandleFunc("POST /api/autostart", s.handleSetAutostart)
}

// ---- 扫描 ----

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.Scan()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.Rescan()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, res)
}

// ---- 概览 / 区间 / 日报 / 模型 / 趋势 / 项目 / 会话 ----

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := s.svc.Overview()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, ov)
}

func (s *Server) handleRange(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	t, err := s.svc.RangeTotals(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"start": start, "end": end, "totals": t})
}

func (s *Server) handleDaily(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	rep, err := s.svc.DailyReport(date)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, rep)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	totals, models, err := s.svc.ModelsBetween(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"start": start, "end": end, "totals": totals, "models": models})
}

func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	// series 按 days-1 天前起算（近 N 天含今天共 N 天）
	start, end, err := rangeOrDays(r, 1)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	series, err := s.svc.Series(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, series)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	// projects 按 days 天前起算，保持原有口径
	start, end, err := rangeOrDays(r, 0)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	list, err := s.svc.Projects(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, list)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	list, err := s.svc.Sessions(start, end, queryInt(r, "limit", 50))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, list)
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	msgs, err := s.svc.SessionMessages(r.URL.Query().Get("project"), r.URL.Query().Get("session"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, msgs)
}

// ---- 请求流水 ----

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	q := r.URL.Query()
	f := store.EventFilter{
		Start:   start,
		End:     end,
		Model:   q.Get("model"),
		Project: q.Get("project"),
		Session: q.Get("session"),
		Limit:   queryInt(r, "limit", 50),
		Offset:  queryInt(r, "offset", 0),
	}
	events, total, err := s.svc.Events(f)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"events": events, "total": total})
}

// ---- 时段热力图 ----

func (s *Server) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	grid, err := s.svc.Heatmap(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, grid)
}

// ---- 工具使用统计 ----

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	tools, err := s.svc.Tools(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, tools)
}

// ---- 全文搜索 / 效率 / 文件热点 ----

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	hits, err := s.svc.Search(r.URL.Query().Get("q"), queryInt(r, "limit", 30))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"hits": hits})
}

func (s *Server) handleEfficiency(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	e, err := s.svc.Efficiency(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, e)
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	list, err := s.svc.FileStats(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, list)
}

// ---- 周期报告 ----

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	rep, err := s.svc.Report(r.URL.Query().Get("period"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, rep)
}

// ---- 日志目录管理 ----

func (s *Server) handleGetLogDirs(w http.ResponseWriter, r *http.Request) {
	dirs, defaults := s.svc.LogDirs()
	writeJSON(w, map[string]any{
		"dirs":       dirs,
		"defaultDir": defaults,
	})
}

func (s *Server) handleAddLogDir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		badRequest(w, "参数错误")
		return
	}
	dirs, err := s.svc.AddLogDir(req.Path)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, map[string]any{"dirs": dirs})
}

func (s *Server) handleRemoveLogDir(w http.ResponseWriter, r *http.Request) {
	dirs, err := s.svc.RemoveLogDir(r.URL.Query().Get("path"))
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, map[string]any{"dirs": dirs})
}

// ---- 显示调整 ----

func (s *Server) handleGetAdjust(w http.ResponseWriter, r *http.Request) {
	// Adjustment 的 RealTotal 在未启用时也有值（缓存自全量聚合）
	writeJSON(w, s.svc.Adjustment())
}

func (s *Server) handleSetAdjust(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "参数错误")
		return
	}
	adj, err := s.svc.SetAdjust(req.Target)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, adj)
}

// handleResetAdjust 回滚到真实数据
func (s *Server) handleResetAdjust(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.ResetAdjust(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// ---- 观测代理 ----

func (s *Server) handleGetProxy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.svc.ProxyStatus())
}

func (s *Server) handleSetProxy(w http.ResponseWriter, r *http.Request) {
	var req config.ProxyConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "参数错误")
		return
	}
	st, err := s.svc.SetProxy(req)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, st)
}

func (s *Server) handleProxyRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ProxyFilter{
		Start:    q.Get("start"),
		End:      q.Get("end"),
		Provider: q.Get("provider"),
		Model:    q.Get("model"),
		Limit:    queryInt(r, "limit", 50),
		Offset:   queryInt(r, "offset", 0),
	}
	if (f.Start != "" && !validDate(f.Start)) || (f.End != "" && !validDate(f.End)) {
		badRequest(w, "日期格式应为 YYYY-MM-DD")
		return
	}
	list, total, err := s.svc.ProxyRequests(f)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"requests": list, "total": total})
}

func (s *Server) handleProxySummary(w http.ResponseWriter, r *http.Request) {
	start, end, err := dateRange(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	models, totalReq, err := s.svc.ProxySummary(start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"models": models, "requests": totalReq})
}

func (s *Server) handleProxyProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.svc.ProxyProviders())
}

func (s *Server) handleAddProxyProvider(w http.ResponseWriter, r *http.Request) {
	var req proxy.Provider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "参数错误")
		return
	}
	list, err := s.svc.AddProxyProvider(req)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, list)
}

func (s *Server) handleFetchProxyProviders(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		badRequest(w, "参数错误")
		return
	}
	list, err := s.svc.FetchProxyProviders(req.URL)
	if err != nil {
		badRequest(w, err.Error())
		return
	}
	writeJSON(w, list)
}

// ---- 开机自启 ----

func (s *Server) handleGetAutostart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"enabled": tray.AutostartEnabled()})
}

func (s *Server) handleSetAutostart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "参数错误")
		return
	}
	if err := tray.SetAutostart(req.Enabled); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]bool{"enabled": tray.AutostartEnabled()})
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// ---- 响应与参数工具 ----

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func badRequest(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

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
	if start > end {
		return "", "", fmt.Errorf("start 不能晚于 end")
	}
	return start, end, nil
}

// rangeOrDays 有 start/end 时解析区间，否则按最近 N 天换算成区间
func rangeOrDays(r *http.Request, back int) (string, string, error) {
	if r.URL.Query().Get("start") != "" || r.URL.Query().Get("end") != "" {
		return dateRange(r)
	}
	days := queryInt(r, "days", 30)
	if days <= 0 || days > 365 {
		days = 30
	}
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -(days - back)).Format("2006-01-02")
	return start, end, nil
}

func queryInt(r *http.Request, key string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil {
		return v
	}
	return def
}
