package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/YFFMaple/MapleBurn/internal/config"
	"github.com/YFFMaple/MapleBurn/internal/store"
)

// Status 代理运行状态（API 返回用）
type Status struct {
	Enabled          bool   `json:"enabled"`
	Running          bool   `json:"running"`
	Addr             string `json:"addr"` // 给客户端配置的 base URL
	Port             int    `json:"port"`
	Upstream         string `json:"upstream"`
	Provider         string `json:"provider"`         // 实际生效的服务商名
	ProviderDisplay  string `json:"providerDisplay"`  // 展示名
	DetectedUpstream string `json:"detectedUpstream"` // 从 CC 配置读到的上游（建议值）
}

// Manager 代理生命周期管理：随开关启停，配置热生效
type Manager struct {
	cfg      *config.Config
	db       *store.DB
	registry *Registry
	client   *http.Client

	mu      sync.Mutex
	srv     *http.Server
	running bool
	port    int
}

func NewManager(cfg *config.Config, db *store.DB, dataDir string) (*Manager, error) {
	reg, err := LoadRegistry(dataDir)
	if err != nil {
		return nil, err
	}
	return &Manager{
		cfg: cfg, db: db, registry: reg,
		client: &http.Client{}, // 不设超时：流式响应可能持续很久，交给请求自身的 context
	}, nil
}

// Registry 暴露服务商注册表（增删改、在线更新）
func (m *Manager) Registry() *Registry { return m.registry }

// Apply 按当前配置应用状态：开启/关闭/换端口重启；上游和服务商修改无需重启
func (m *Manager) Apply() error {
	p := m.cfg.ProxyCfg()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !p.Enabled {
		m.stopLocked()
		return nil
	}
	if m.running && m.port == p.Port {
		return nil
	}
	m.stopLocked()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p.Port))
	if err != nil {
		return fmt.Errorf("端口 %d 不可用: %w", p.Port, err)
	}
	m.srv = &http.Server{Handler: http.HandlerFunc(m.serveHTTP)}
	go m.srv.Serve(ln)
	m.running = true
	m.port = p.Port
	return nil
}

// Stop 关闭代理（进程退出时调用）
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.srv != nil {
		m.srv.Close()
		m.srv = nil
	}
	m.running = false
}

// Status 当前状态 + 生效的服务商
func (m *Manager) Status() Status {
	p := m.cfg.ProxyCfg()
	prov := m.registry.Match(p.Provider, p.Upstream)
	m.mu.Lock()
	running := m.running
	m.mu.Unlock()
	return Status{
		Enabled:          p.Enabled,
		Running:          running,
		Addr:             fmt.Sprintf("http://127.0.0.1:%d", p.Port),
		Port:             p.Port,
		Upstream:         p.Upstream,
		Provider:         prov.Name,
		ProviderDisplay:  prov.DisplayName,
		DetectedUpstream: config.DetectCCUpstream(),
	}
}

// ---- 转发逻辑 ----

// 逐跳头部不能转发（RFC 7230），Connection 头里点名的也要去掉
var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Upgrade",
	"Te", "Trailer", "Transfer-Encoding",
}

func (m *Manager) serveHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	p := m.cfg.ProxyCfg()
	prov := m.registry.Match(p.Provider, p.Upstream)

	rec := &store.ProxyRequest{
		Ts:       start.UnixMilli(),
		Provider: prov.Name,
		Path:     r.URL.Path,
	}
	// 无论成功失败都落一条记录——失败的请求同样是观测数据
	defer func() {
		rec.LatencyMs = time.Since(start).Milliseconds()
		if rec.Model == "" {
			rec.Model = modelFromPath(r.URL.Path)
		}
		m.db.InsertProxyRequest(rec)
	}()

	if p.Upstream == "" {
		rec.Error = "upstream not configured"
		http.Error(w, "MapleBurn 代理: 未配置上游地址", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		rec.Error = "read request: " + err.Error()
		http.Error(w, "MapleBurn 代理: 读取请求体失败", http.StatusBadRequest)
		return
	}
	rec.Model = modelFromBody(body)

	target := strings.TrimRight(p.Upstream, "/") + r.URL.RequestURI()
	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		rec.Error = "build request: " + err.Error()
		http.Error(w, "MapleBurn 代理: 构造上游请求失败", http.StatusBadGateway)
		return
	}

	// 原样转发客户端头部（含鉴权），只剔除逐跳头部和 Accept-Encoding：
	// 后者让 Go 传输层自己处理 gzip，这样旁路解析拿到的是解压后的内容
	for k, vs := range r.Header {
		upReq.Header[k] = vs
	}
	for _, h := range hopHeaders {
		upReq.Header.Del(h)
	}
	upReq.Header.Del("Accept-Encoding")

	resp, err := m.client.Do(upReq)
	if err != nil {
		rec.Error = "upstream: " + err.Error()
		http.Error(w, "MapleBurn 代理: 上游请求失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	rec.Status = resp.StatusCode

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	for _, h := range hopHeaders {
		w.Header().Del(h)
	}
	w.Header().Del("Content-Length") // 体长可能因解压变化，交给分块传输
	w.WriteHeader(resp.StatusCode)

	// 流式（SSE）：旁路逐行捕获 usage 事件，主体原样透传；
	// 普通 JSON：整体缓冲（上限 8MB）后解析。其他类型只透传不解析。
	contentType := resp.Header.Get("Content-Type")
	var tap *sseTap
	var jbuf *bytes.Buffer
	isSSE := strings.Contains(contentType, "text/event-stream")
	isJSON := strings.Contains(contentType, "json")
	if isSSE {
		tap = &sseTap{}
	} else if isJSON {
		jbuf = &bytes.Buffer{}
	}

	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if tap != nil {
				tap.Feed(chunk)
			}
			if jbuf != nil && jbuf.Len() < 8<<20 {
				jbuf.Write(chunk)
			}
			w.Write(chunk)
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}

	var u Usage
	if tap != nil {
		u = tap.usage(prov.Adapter)
	} else if jbuf != nil {
		u = parseJSONUsage(prov.Adapter, jbuf.Bytes())
	}
	if rec.Model == "" {
		rec.Model = u.Model
	}
	rec.InputTokens = u.Input
	rec.OutputTokens = u.Output
	rec.CacheCreate = u.CacheCreate
	rec.CacheRead = u.CacheRead
}
