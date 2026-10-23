//go:build windows

// Package widget 桌面小组件窗口：用系统 WebView2 渲染内嵌的精简面板页面，
// 与浏览器访问的是同一个本地服务，数据天然一致。
package widget

import (
	"log"
	"runtime"
	"sync"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"github.com/YFFMaple/MapleBurn/internal/config"
)

var (
	user32        = windows.NewLazySystemDLL("user32.dll")
	setWindowPos  = user32.NewProc("SetWindowPos")
	getWindowRect = user32.NewProc("GetWindowRect")
	isIconic      = user32.NewProc("IsIconic")
)

const (
	swpNosize     = 0x1
	swpNomove     = 0x2
	swpNoactivate = 0x10
	hwndTopmost   = ^uintptr(0)     // -1
	hwndNotopmost = ^uintptr(0) - 1 // -2
)

// Manager 小组件窗口生命周期：同一时刻最多一个窗口，开关状态持久化到配置
type Manager struct {
	cfg *config.Config

	mu      sync.Mutex
	w       webview2.WebView
	running bool

	onStateChange func(open bool)
}

func New(cfg *config.Config) *Manager { return &Manager{cfg: cfg} }

// OnStateChange 窗口实际打开/关闭时回调（更新托盘勾选与配置）
func (m *Manager) OnStateChange(f func(open bool)) {
	m.mu.Lock()
	m.onStateChange = f
	m.mu.Unlock()
}

func (m *Manager) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// Toggle 开着就关，关着就开
func (m *Manager) Toggle(url string) {
	if m.IsOpen() {
		m.Close()
	} else {
		m.Show(url)
	}
}

// Show 打开小组件窗口（已打开则忽略）
func (m *Manager) Show(url string) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()
	go m.loop(url)
	m.stateChanged(true)
}

// Close 关闭小组件窗口（未打开则忽略）
func (m *Manager) Close() {
	m.mu.Lock()
	w := m.w
	m.mu.Unlock()
	if w != nil {
		w.Dispatch(w.Terminate)
	}
}

func (m *Manager) stateChanged(open bool) {
	m.mu.Lock()
	f := m.onStateChange
	m.mu.Unlock()
	if f != nil {
		f(open)
	}
}

// loop 窗口线程：webview 的消息循环必须固定在同一个 OS 线程上跑
func (m *Manager) loop(url string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		m.mu.Lock()
		m.w = nil
		m.running = false
		m.mu.Unlock()
		m.stateChanged(false)
	}()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		WindowOptions: webview2.WindowOptions{Title: "MapleBurn", Width: 320, Height: 560, Center: true},
	})
	if w == nil {
		log.Println("小组件启动失败：系统缺少 WebView2 运行时（可在浏览器中使用完整面板）")
		return
	}
	defer w.Destroy()
	log.Println("小组件窗口已打开:", url)

	m.mu.Lock()
	m.w = w
	m.mu.Unlock()

	wc := m.cfg.WidgetCfg()
	w.SetSize(320, 560, webview2.HintNone)
	w.SetSize(300, 480, webview2.HintMin) // 最小尺寸，避免被拖成一条标题栏
	w.Navigate(url)
	hwnd := uintptr(w.Window())

	// 面板里的置顶按钮直接调这个绑定
	w.Bind("widgetSetTopmost", func(top bool) {
		setTopmost(hwnd, top)
		c := m.cfg.WidgetCfg()
		c.Topmost = top
		m.cfg.SetWidget(c)
		if err := m.cfg.Save(); err != nil {
			log.Println("保存小组件配置失败:", err)
		}
	})

	w.Bind("widgetGetTopmost", func() bool {
		return m.cfg.WidgetCfg().Topmost
	})

	if wc.HasPos && wc.X > -10000 && wc.Y > -10000 {
		setWindowPos.Call(hwnd, 0, uintptr(int32(wc.X)), uintptr(int32(wc.Y)), 0, 0, swpNosize|swpNoactivate)
	}
	setTopmost(hwnd, wc.Topmost)

	w.Run() // 阻塞到窗口关闭
	log.Println("小组件窗口已关闭")

	// 记住最后的位置，下次原地打开。
	// 最小化时 GetWindowRect 返回的是 -32000 哨兵坐标，直接跳过，保留上一次的正常位置。
	if ret, _, _ := isIconic.Call(hwnd); ret != 0 {
		return
	}
	var r windows.Rect
	if ret, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret != 0 && r.Left > -10000 && r.Top > -10000 {
		c := m.cfg.WidgetCfg()
		c.X, c.Y, c.HasPos = int(r.Left), int(r.Top), true
		m.cfg.SetWidget(c)
		if err := m.cfg.Save(); err != nil {
			log.Println("保存小组件位置失败:", err)
		}
	}
}

func setTopmost(hwnd uintptr, top bool) {
	after := hwndNotopmost
	if top {
		after = hwndTopmost
	}
	setWindowPos.Call(hwnd, after, 0, 0, 0, 0, swpNosize|swpNomove|swpNoactivate)
}
