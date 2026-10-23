//go:build windows

// Package tray 系统托盘与开机自启（Windows）。
package tray

import (
	"os"
	"path/filepath"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows/registry"
)

// widgetItem 小组件菜单项，窗口开关状态变化时同步勾选
var widgetItem *systray.MenuItem

// Run 在调用方 goroutine 运行托盘图标（会阻塞，通常放在主 goroutine）。
// onOpen 打开面板，onWidget 切换小组件窗口，onExit 退出程序（应先做清理再返回）。
func Run(onOpen, onExit, onWidget func(), widgetOpen bool) {
	systray.Run(func() {
		migrateAutostart()
		systray.SetIcon(iconBytes())
		systray.SetTooltip("燃枫 MapleBurn")
		mOpen := systray.AddMenuItem("打开面板", "在浏览器中打开")
		widgetItem = systray.AddMenuItemCheckbox("小组件", "桌面小窗实时显示今日用量", widgetOpen)
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "还原配置并退出")
		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					onOpen()
				case <-widgetItem.ClickedCh:
					onWidget()
				case <-mQuit.ClickedCh:
					onExit()
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}

// SetWidgetChecked 同步小组件菜单的勾选状态（窗口被直接关掉时也要反勾选）
func SetWidgetChecked(open bool) {
	if widgetItem == nil {
		return
	}
	if open {
		widgetItem.Check()
	} else {
		widgetItem.Uncheck()
	}
}

// SetTooltip 更新托盘悬停提示（显示实时用量）
func SetTooltip(text string) { systray.SetTooltip(text) }

// mapleLeaf 枫叶轮廓（0~100 坐标，对称的多边形，从上尖点顺时针一圈）
var mapleLeaf = [][2]float64{
	{50, 2},
	{56, 18}, {68, 11}, {64, 28}, {82, 21}, {75, 38}, {94, 33},
	{84, 50}, {97, 57}, {72, 64}, {78, 79}, {60, 76}, {57, 88},
	{53, 88}, {53, 98}, {47, 98}, {47, 88}, {43, 88}, {40, 76},
	{22, 79}, {28, 64}, {3, 57}, {16, 50}, {6, 33}, {25, 38},
	{18, 21}, {36, 28}, {32, 11}, {44, 18},
}

// iconBytes 运行时生成一个 32×32 枫叶图标的 ICO（免资源文件）。
// 用 4×4 超采样做边缘抗锯齿。
func iconBytes() []byte {
	const size = 32
	px := make([]byte, size*size*4)
	mask := make([]byte, size*size/8)

	// 采样点在 0~100 坐标系里判断是否在叶子内部（射线法）
	inside := func(px, py float64) bool {
		in := false
		for i, j := 0, len(mapleLeaf)-1; i < len(mapleLeaf); j, i = i, i+1 {
			xi, yi := mapleLeaf[i][0], mapleLeaf[i][1]
			xj, yj := mapleLeaf[j][0], mapleLeaf[j][1]
			if (yi > py) != (yj > py) && px < (xj-xi)*(py-yi)/(yj-yi)+xi {
				in = !in
			}
		}
		return in
	}

	const ss = 4 // 每轴采样数
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					// 像素中心映射到 0~100 坐标
					pxx := (float64(x) + (float64(sx)+0.5)/ss) / size * 100
					pyy := (float64(y) + (float64(sy)+0.5)/ss) / size * 100
					if inside(pxx, pyy) {
						hits++
					}
				}
			}
			alpha := uint8(hits * 255 / (ss * ss))
			i := (y*size + x) * 4
			// 枫叶红 #e2543f
			px[i+0] = 0x3f // B
			px[i+1] = 0x54 // G
			px[i+2] = 0xe2 // R
			px[i+3] = alpha
			if alpha < 128 {
				mask[(y*size+x)/8] |= 1 << (7 - uint((y*size+x)%8))
			}
		}
	}

	// ICO：头部 + 目录 + BITMAPINFOHEADER + XOR 位图 + AND 掩码
	header := []byte{0, 0, 1, 0, 1, 0} // reserved, type=icon, count=1
	bmpHeaderSize := 40
	imgSize := bmpHeaderSize + len(px) + len(mask)
	entry := []byte{
		size, size, // 宽高
		0, 0, // 调色板/保留
		1, 0, // color planes
		32, 0, // bpp
		byte(imgSize), byte(imgSize >> 8), byte(imgSize >> 16), byte(imgSize >> 24),
		byte(22), 0, 0, 0, // 数据偏移 = 6 + 16
	}
	bmp := make([]byte, bmpHeaderSize)
	put := func(off int, v uint32) {
		bmp[off] = byte(v)
		bmp[off+1] = byte(v >> 8)
		bmp[off+2] = byte(v >> 16)
		bmp[off+3] = byte(v >> 24)
	}
	put(0, uint32(bmpHeaderSize))
	put(4, size)
	put(8, size*2) // ICO 高度计为 2 倍（含掩码）
	put(12, 1)     // planes（低 2 字节）
	bmp[14] = 32   // bpp
	// 像素按 BMP 自底向上存储，翻转行序
	flipped := make([]byte, len(px))
	row := size * 4
	for y := 0; y < size; y++ {
		copy(flipped[(size-1-y)*row:], px[y*row:(y+1)*row])
	}
	flipMask := make([]byte, len(mask))
	mrow := size / 8
	for y := 0; y < size; y++ {
		copy(flipMask[(size-1-y)*mrow:], mask[y*mrow:(y+1)*mrow])
	}

	out := append(header, entry...)
	out = append(out, bmp...)
	out = append(out, flipped...)
	out = append(out, flipMask...)
	return out
}

// Quit 从其他 goroutine 结束托盘（服务关闭时调用）
func Quit() { systray.Quit() }

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const runValueName = "mapleburn"
const oldRunValueName = "aitoken"

// migrateAutostart 把旧版本注册的自启项迁移到新名（旧条目指向的 exe 已经不存在）
func migrateAutostart() {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	oldVal, _, err := k.GetStringValue(oldRunValueName)
	if err != nil {
		return
	}
	if _, _, err := k.GetStringValue(runValueName); err != nil {
		k.SetStringValue(runValueName, oldVal)
	}
	k.DeleteValue(oldRunValueName)
}

// AutostartEnabled 是否已注册开机自启
func AutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValueName)
	return err == nil
}

// SetAutostart 注册/取消开机自启（HKCU Run 键，免管理员）
func SetAutostart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		err = k.DeleteValue(runValueName)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return err
	}
	return k.SetStringValue(runValueName, `"`+abs+`"`)
}
