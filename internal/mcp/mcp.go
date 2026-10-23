// Package mcp 把 MapleBurn 的统计数据暴露为 MCP 工具（Streamable HTTP，单端点无会话），
// 让 Claude Code 可以直接查询本机的用量数据。
// 只实现协议最小子集：initialize / notifications / ping / tools/list / tools/call。
package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YFFMaple/MapleBurn/internal/service"
)

type Server struct {
	svc *service.Service
}

func New(svc *service.Service) *Server {
	return &Server{svc: svc}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// 不支持 SSE 流，按 Streamable HTTP 规范对 GET 返回 405
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// 通知不需要响应体
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	fail := func(code int, msg string) {
		resp.Error = &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{code, msg}
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "mapleburn", "version": "2.0.0"},
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": toolDefs()}
	case "tools/call":
		resp.Result = s.callTool(req.Params)
	default:
		fail(-32601, "method not found: "+req.Method)
	}
	json.NewEncoder(w).Encode(resp)
}

func toolDefs() []map[string]any {
	dateRange := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"start": map[string]any{"type": "string", "description": "开始日期 YYYY-MM-DD，缺省 30 天前"},
			"end":   map[string]any{"type": "string", "description": "结束日期 YYYY-MM-DD，缺省今天"},
		},
	}
	return []map[string]any{
		{
			"name":        "usage_overview",
			"description": "本机 Claude Code 用量概览：今日/本周/本月/累计的 token 与请求数",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "model_stats",
			"description": "按模型分组的 token 用量统计（可指定日期区间）",
			"inputSchema": dateRange,
		},
		{
			"name":        "tool_stats",
			"description": "Claude Code 工具调用次数与失败次数统计（Bash/Read/Edit 等）",
			"inputSchema": dateRange,
		},
		{
			"name":        "search_sessions",
			"description": "按内容全文搜索历史会话消息，返回匹配的会话与片段",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索关键词（中英文均可）"},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "proxy_status",
			"description": "观测代理的运行状态与近 30 天代理流量汇总",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

func (s *Server) callTool(params json.RawMessage) any {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(params, &p) != nil {
		return toolError("invalid params")
	}

	text, err := s.runTool(p.Name, p.Arguments)
	if err != nil {
		return toolError(err.Error())
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	}
}

func toolError(msg string) any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": "错误: " + msg}},
		"isError": true,
	}
}

func (s *Server) runTool(name string, args json.RawMessage) (string, error) {
	var rangeArgs struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	json.Unmarshal(args, &rangeArgs)
	if rangeArgs.End == "" {
		rangeArgs.End = time.Now().Format("2006-01-02")
	}
	if rangeArgs.Start == "" {
		rangeArgs.Start = time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	}

	switch name {
	case "usage_overview":
		ov, err := s.svc.Overview()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(
			"今日: %s tokens / %d 次请求\n本周: %s tokens / %d 次\n本月: %s tokens / %d 次\n累计: %s tokens / %d 次",
			num(ov.Today.TotalTokens), ov.Today.Requests,
			num(ov.Week.TotalTokens), ov.Week.Requests,
			num(ov.Month.TotalTokens), ov.Month.Requests,
			num(ov.AllTime.TotalTokens), ov.AllTime.Requests), nil

	case "model_stats":
		totals, models, err := s.svc.ModelsBetween(rangeArgs.Start, rangeArgs.End)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s ~ %s 共 %s tokens / %d 次请求\n\n", rangeArgs.Start, rangeArgs.End, num(totals.TotalTokens), totals.Requests)
		for _, m := range models {
			total := m.InputTokens + m.OutputTokens + m.CacheCreate + m.CacheRead
			fmt.Fprintf(&b, "- %s: %s tokens / %d 次\n", m.Model, num(total), m.Requests)
		}
		return b.String(), nil

	case "tool_stats":
		tools, err := s.svc.Tools(rangeArgs.Start, rangeArgs.End)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s ~ %s 工具调用:\n\n", rangeArgs.Start, rangeArgs.End)
		for _, t := range tools {
			line := fmt.Sprintf("- %s: %d 次", t.Tool, t.Count)
			if t.Errors > 0 {
				line += fmt.Sprintf("（失败 %d 次）", t.Errors)
			}
			b.WriteString(line + "\n")
		}
		return b.String(), nil

	case "search_sessions":
		var q struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(args, &q) != nil || q.Query == "" {
			return "", fmt.Errorf("缺少 query 参数")
		}
		hits, err := s.svc.Search(q.Query, 10)
		if err != nil {
			return "", err
		}
		if len(hits) == 0 {
			return "没有找到包含该内容的会话消息", nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "找到 %d 条匹配:\n\n", len(hits))
		for _, h := range hits {
			snippet := h.Text
			if len(snippet) > 150 {
				snippet = snippet[:150] + "…"
			}
			fmt.Fprintf(&b, "- [%s] %s / %s（%s）\n  %s\n", h.Role, h.Project, h.SessionID[:min(8, len(h.SessionID))], h.Ts, snippet)
		}
		return b.String(), nil

	case "proxy_status":
		st := s.svc.ProxyStatus()
		state := "关闭"
		if st.Running {
			state = "运行中"
		}
		out := fmt.Sprintf("观测代理: %s（开关 %v）\n地址: %s\n上游: %s\n服务商: %s",
			state, st.Enabled, st.Addr, st.Upstream, st.ProviderDisplay)
		end := time.Now().Format("2006-01-02")
		start := time.Now().AddDate(0, 0, -29).Format("2006-01-02")
		if models, total, err := s.svc.ProxySummary(start, end); err == nil && total > 0 {
			out += fmt.Sprintf("\n\n近 30 天代理流量: %d 次请求", total)
			for _, m := range models {
				t := m.InputTokens + m.OutputTokens + m.CacheCreate + m.CacheRead
				out += fmt.Sprintf("\n- %s: %s tokens / %d 次", m.Model, num(t), m.Requests)
			}
		}
		return out, nil
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

// num 把大数字格式化成 1.2M / 3.4B 这种短形式
func num(n int) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}
