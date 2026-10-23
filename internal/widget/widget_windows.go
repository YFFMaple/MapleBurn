//go:build windows

// Package widget 桌面小组件窗口：用系统 WebView2 渲染内嵌的精简面板页面，
// 与浏览器访问的是同一个本地服务，数据天然一致。
package widget

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"

	"github.com/YFFMaple/MapleBurn/internal/config"
	"github.com/YFFMaple/MapleBurn/internal/tray"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	dwmapi            = windows.NewLazySystemDLL("dwmapi.dll")
	comctl32          = windows.NewLazySystemDLL("comctl32.dll")
	gdi32             = windows.NewLazySystemDLL("gdi32.dll")
	setWindowPos      = user32.NewProc("SetWindowPos")
	getWindowRect     = user32.NewProc("GetWindowRect")
	isIconic          = user32.NewProc("IsIconic")
	getWindowLong     = user32.NewProc("GetWindowLongPtrW")
	setWindowLong     = user32.NewProc("SetWindowLongPtrW")
	releaseCapture    = user32.NewProc("ReleaseCapture")
	sendMessage       = user32.NewProc("SendMessageW")
	showWindow        = user32.NewProc("ShowWindow")
	getKeyState       = user32.NewProc("GetKeyState")
	setWindowRgn      = user32.NewProc("SetWindowRgn")
	loadImage         = user32.NewProc("LoadImageW")
	setWinCompAttr    = user32.NewProc("SetWindowCompositionAttribute")
	createRoundRect   = gdi32.NewProc("CreateRoundRectRgn")
	dwmSetAttribute   = dwmapi.NewProc("DwmSetWindowAttribute")
	dwmExtendFrame    = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
	setSubclass       = comctl32.NewProc("SetWindowSubclass")
	defSubclassProc   = comctl32.NewProc("DefSubclassProc")
)

const (
	swpNosize       = 0x1
	swpNomove       = 0x2
	swpNozorder     = 0x4
	swpNoactivate   = 0x10
	swpFramechanged = 0x20
	hwndTopmost     = ^uintptr(0)     // -1
	hwndNotopmost   = ^uintptr(0) - 1 // -2

	gwlStyle        = ^uintptr(15) // -16
	wmNclbuttondown = 0xA1
	wmNccalcsize    = 0x83
	wmNchittest     = 0x84
	wmSize          = 0x05
	wmSeticon       = 0x80
	htcaption       = 2
	swMinimize      = 6
	cornerRadius    = 16 // 物理裁剪圆角半径（Win10 没有 DWM 圆角，用窗口区域实现）

	// NCHITTEST 返回值
	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopleft     = 13
	htTopright    = 14
	htBottom      = 15
	htBottomleft  = 16
	htBottomright = 17

	resizeBorder = 6 // 边缘多少像素内算"在拉边框"
)

// subclassProc 无边框窗口的子类化回调：
// WM_NCCALCSIZE 返回 0 把非客户区清零（消灭白色边框，客户区铺满整个窗口），
// WM_NCHITTEST 手动做边缘缩放判定（非客户区没了之后粗边框不再自动生效）
var subclassProc = windows.NewCallback(func(hwnd, msg, wp, lp, _, _ uintptr) uintptr {
	switch msg {
	case wmNccalcsize:
		if wp != 0 {
			return 0
		}
	case wmSize:
		// 尺寸变化后重贴圆角区域（SetWindowRgn 不跟随缩放）
		wdt := int32(uint16(lp))
		hgt := int32(uint16(lp >> 16))
		if wdt > 0 && hgt > 0 {
			applyRoundRgn(hwnd, wdt, hgt)
		}
	case wmNchittest:
		x := int(int16(lp & 0xffff))
		y := int(int16(lp >> 16))
		var r windows.Rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		l := x < int(r.Left)+resizeBorder
		rt := x >= int(r.Right)-resizeBorder
		tp := y < int(r.Top)+resizeBorder
		bt := y >= int(r.Bottom)-resizeBorder
		switch {
		case tp && l:
			return htTopleft
		case tp && rt:
			return htTopright
		case bt && l:
			return htBottomleft
		case bt && rt:
			return htBottomright
		case tp:
			return htTop
		case bt:
			return htBottom
		case l:
			return htLeft
		case rt:
			return htRight
		}
	}
	ret, _, _ := defSubclassProc.Call(hwnd, msg, wp, lp)
	return ret
})

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
		w.Dispatch(w.Destroy)
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
	hwnd := uintptr(w.Window())
	makeFrameless(hwnd)
	setIcon(hwnd)
	if r := getRect(hwnd); r != nil {
		applyRoundRgn(hwnd, r.Right-r.Left, r.Bottom-r.Top)
	}

	// 绑定必须在 Navigate 之前注册——Init 脚本只对之后的页面加载生效，
	// 顺序反了页面里可能拿不到 widgetDrag 等函数（表现为最小化/关闭按钮不出现）
	w.Bind("widgetDrag", func() {
		w.Dispatch(func() {
			if down, _, _ := getKeyState.Call(1); down&0x8000 == 0 { // VK_LBUTTON
				return
			}
			releaseCapture.Call()
			sendMessage.Call(hwnd, wmNclbuttondown, htcaption, 0)
		})
	})
	w.Bind("widgetMinimize", func() {
		w.Dispatch(func() { showWindow.Call(hwnd, swMinimize) })
	})
	// 关闭必须走 WMClose → DestroyWindow 的完整销毁路径，窗口才会真正消失。
	// 直接 Terminate 只往线程队列扔 WM_QUIT：消息循环退了但窗口还在，
	// 留下一个永远不重绘的僵尸窗口（表现就是"点 X 卡死"）
	w.Bind("widgetClose", func() {
		w.Dispatch(w.Destroy)
	})
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
	w.Bind("widgetSetGlass", func(on bool) {
		applyGlass(w, hwnd, on)
		c := m.cfg.WidgetCfg()
		c.Glass = on
		m.cfg.SetWidget(c)
		if err := m.cfg.Save(); err != nil {
			log.Println("保存小组件配置失败:", err)
		}
	})
	w.Bind("widgetGetGlass", func() bool {
		return m.cfg.WidgetCfg().Glass
	})

	w.SetSize(320, 520, webview2.HintNone)
	w.SetSize(300, 420, webview2.HintMin) // 最小尺寸，避免被拖成一条
	w.Navigate(url)

	// 上次开着毛玻璃就恢复（等页面加载完再 Eval，否则 class 加不上）
	if wc.Glass {
		time.AfterFunc(1500*time.Millisecond, func() {
			w.Dispatch(func() { applyGlass(w, hwnd, true) })
		})
	}

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

// getRect 取窗口外框矩形
func getRect(hwnd uintptr) *windows.Rect {
	var r windows.Rect
	if ret, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return nil
	}
	return &r
}

func setTopmost(hwnd uintptr, top bool) {
	after := hwndNotopmost
	if top {
		after = hwndTopmost
	}
	setWindowPos.Call(hwnd, after, 0, 0, 0, 0, swpNosize|swpNomove|swpNoactivate)
}

// applyRoundRgn 用圆角矩形区域物理裁剪窗口（Win10 没有 DWM 圆角，只能这样真圆）
func applyRoundRgn(hwnd uintptr, wdt, hgt int32) {
	rgn, _, _ := createRoundRect.Call(0, 0, uintptr(wdt)+1, uintptr(hgt)+1, cornerRadius, cornerRadius)
	setWindowRgn.Call(hwnd, rgn, 1) // 系统接管 rgn 句柄，不能删
}

// setIcon 任务栏/窗口图标换成枫叶（默认是白色方块）。
// CreateIconFromResourceEx 吃不进 .ico 文件字节（它要的是资源段格式），
// 所以先落地成临时文件再走 LoadImage，跟系统托盘用的是同一条可靠路径
func setIcon(hwnd uintptr) {
	icoPath := filepath.Join(os.TempDir(), "mapleburn.ico")
	if _, err := os.Stat(icoPath); err != nil {
		if err := os.WriteFile(icoPath, tray.IconBytes(), 0644); err != nil {
			log.Println("小组件图标写入失败:", err)
			return
		}
	}
	pathPtr, err := windows.UTF16PtrFromString(icoPath)
	if err != nil {
		return
	}
	const (imageIcon = 1; lrLoadFromFile = 0x10)
	h, _, _ := loadImage.Call(0, uintptr(unsafe.Pointer(pathPtr)), imageIcon, 32, 32, lrLoadFromFile)
	if h == 0 {
		log.Println("小组件图标加载失败")
		return
	}
	sendMessage.Call(hwnd, wmSeticon, 0, h) // ICON_SMALL
	sendMessage.Call(hwnd, wmSeticon, 1, h) // ICON_BIG
}

// ---- 毛玻璃 ----

// accentPolicy / winCompAttrData 对应未公开但沿用多年的 SetWindowCompositionAttribute
type accentPolicy struct {
	accentState   uint32
	accentFlags   uint32
	gradientColor uint32 // ABGR
	animationId   uint32
}

type winCompAttrData struct {
	attrib uint32
	data   unsafe.Pointer
	size   uintptr
}

// setAcrylic 开/关窗口亚克力模糊背景（Win10 1803+ 可用）
func setAcrylic(hwnd uintptr, on bool) {
	var p accentPolicy
	if on {
		p.accentState = 4          // ACCENT_ENABLE_ACRYLICBLURBEHIND
		p.accentFlags = 2          // 四周边框描边
		p.gradientColor = 0x66000000 // ABGR，只留轻微暗色 tint，模糊靠系统
	}
	data := winCompAttrData{attrib: 19, data: unsafe.Pointer(&p), size: unsafe.Sizeof(p)} // WCA_ACCENT_POLICY
	setWinCompAttr.Call(hwnd, uintptr(unsafe.Pointer(&data)))
}

// setWebviewTransparent 把 WebView2 的默认背景改成全透明，亚克力才能透出来。
// go-webview2 没有暴露 controller，只能反射拿（字段名随库版本固定，失败就放弃，不影响功能）
func setWebviewTransparent(w webview2.WebView) {
	defer func() { _ = recover() }()
	bf := reflect.ValueOf(w).Elem().FieldByName("browser")
	if !bf.IsValid() {
		return
	}
	browser := reflect.NewAt(bf.Type(), unsafe.Pointer(bf.UnsafeAddr())).Elem().Interface()
	chromium, ok := browser.(*edge.Chromium)
	if !ok {
		return
	}
	cf := reflect.ValueOf(chromium).Elem().FieldByName("controller")
	if !cf.IsValid() {
		return
	}
	ctrl := *(**edge.ICoreWebView2Controller)(unsafe.Pointer(cf.UnsafeAddr()))
	if ctrl == nil {
		return
	}
	c2 := ctrl.GetICoreWebView2Controller2()
	if c2 == nil {
		return
	}
	_ = c2.PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{A: 0, R: 0, G: 0, B: 0})
}

// applyGlass 毛玻璃三件套：窗口亚克力 + WebView2 透明 + 通知页面换半透明配色
func applyGlass(w webview2.WebView, hwnd uintptr, on bool) {
	setAcrylic(hwnd, on)
	if on {
		setWebviewTransparent(w)
	}
	if on {
		w.Eval(`document.documentElement.classList.add("glass")`)
	} else {
		w.Eval(`document.documentElement.classList.remove("glass")`)
	}
}

// makeFrameless 无边框化：保留 WS_OVERLAPPEDWINDOW 骨架，去掉标题栏/系统菜单/
// 粗边框（这台机器上 WS_POPUP 会让 WebView2 不绘制，不能用），白色边框区靠子类化
// 拦截 WM_NCCALCSIZE 清零非客户区消灭，边缘缩放由 subclassProc 手动命中测试
// （不依赖 WS_THICKFRAME）。WS_THICKFRAME 必须去掉：Win10 的 DWM 会沿窗口区域
// 画一圈 1px 的亮色描边，圆角裁剪后它贴着圆弧显示成"白边"。
// Win11 上顺便请求 DWM 圆角，Win10 忽略失败保持直角。
func makeFrameless(hwnd uintptr) {
	const (
		wsCaption    = 0xC00000
		wsSysmenu    = 0x80000
		wsMaximize   = 0x10000
		wsThickframe = 0x40000
	)
	style, _, _ := getWindowLong.Call(hwnd, gwlStyle)
	style &^= (wsCaption | wsSysmenu | wsMaximize | wsThickframe)
	setWindowLong.Call(hwnd, gwlStyle, style)
	setSubclass.Call(hwnd, subclassProc, 1, 0)
	setWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNosize|swpNomove|swpNozorder|swpNoactivate|swpFramechanged)

	// 延伸 1px DWM 边框进客户区，保住窗口阴影
	type margins struct{ left, right, top, bottom int32 }
	m := margins{0, 0, 0, 1}
	dwmExtendFrame.Call(hwnd, uintptr(unsafe.Pointer(&m)), unsafe.Sizeof(m))

	val := uint32(2) // DWMWCP_ROUND
	dwmSetAttribute.Call(hwnd, 33, uintptr(unsafe.Pointer(&val)), 4) // DWMWA_WINDOW_CORNER_PREFERENCE
}
