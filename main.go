// main 入口：负责装配各层（config → store → scanner → service → server）并启动服务。
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/YFFMaple/MapleBurn/internal/config"
	"github.com/YFFMaple/MapleBurn/internal/mcp"
	"github.com/YFFMaple/MapleBurn/internal/proxy"
	"github.com/YFFMaple/MapleBurn/internal/scanner"
	"github.com/YFFMaple/MapleBurn/internal/server"
	"github.com/YFFMaple/MapleBurn/internal/service"
	"github.com/YFFMaple/MapleBurn/internal/store"
	"github.com/YFFMaple/MapleBurn/internal/tray"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
	// 数据目录：%LOCALAPPDATA%\mapleburn（仅本机）
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local, _ = os.UserHomeDir()
	}
	dataDir := filepath.Join(local, "mapleburn")
	migrateDataDir(local, dataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("无法创建数据目录: ", err)
	}

	// 日志同时写文件：windowsgui 构建没有控制台，排查问题全靠它
	if lf, err := os.OpenFile(filepath.Join(dataDir, "mapleburn.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		defer lf.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, lf))
	}

	// 费用折算已下线，清理旧版本遗留的价格表文件
	for _, f := range []string{"pricing.json", "community_pricing.json"} {
		os.Remove(filepath.Join(dataDir, f))
	}

	db, err := store.Open(filepath.Join(dataDir, "mapleburn.db"))
	if err != nil {
		log.Fatal("无法打开本地数据库: ", err)
	}
	defer db.Close()

	cfg := config.Load(dataDir)
	pm, err := proxy.NewManager(cfg, db, dataDir)
	if err != nil {
		log.Fatal("初始化观测代理失败: ", err)
	}
	defer pm.Stop()
	svc := service.New(db, scanner.New(db, cfg.Dirs()), cfg, dataDir, pm)
	srv := server.New(svc)

	// 观测代理：开关持久化在配置里，上次退出时开着就自动恢复
	if err := pm.Apply(); err != nil {
		log.Println("观测代理启动失败（不影响其他功能）:", err)
	}

	// schema v3 引入消息全文索引/工具错误率/文件统计：老库的历史数据没有这部分，
	// 升级后清空重建一次，让新统计覆盖全部历史
	if db.GetMeta("schema_version") != "3" {
		fmt.Println("数据升级：重建索引以补充搜索/工具/文件统计（约十几秒）…")
		if _, err := svc.Rescan(); err != nil {
			log.Println("重建失败（不影响使用，可稍后在设置里手动重建）:", err)
		}
		if err := db.SetMeta("schema_version", "3"); err != nil {
			log.Println("写入版本标记失败:", err)
		}
	}

	// 找一个可用端口（127.0.0.1 只监听本机回环，外部无法访问）
	listener, port, err := listenLocal(17820, 17840)
	if err != nil {
		log.Fatal("没有可用端口: ", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 静态资源（内嵌在 exe 里）
	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(dist)))
	mux.Handle("/mcp", mcp.New(svc)) // MCP 工具端点：让 Claude Code 直接查询用量
	srv.RegisterAPI(mux)

	fmt.Println("┌──────────────────────────────────────────┐")
	fmt.Println("│          燃枫 MapleBurn（本地版）         │")
	fmt.Println("└──────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("  访问地址:  ", url)
	fmt.Println("  日志目录:  ", strings.Join(cfg.Dirs(), " ; "))
	fmt.Println("  数据目录:  ", dataDir)
	fmt.Println()
	fmt.Println("  浏览器会自动打开；关闭此窗口即退出程序。")
	fmt.Println()

	go openBrowser(url)

	httpSrv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	// Ctrl+C 时优雅退出，让 defer db.Close() 有机会执行
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// 退出清理：代理还开着就自动关闭并还原 Claude Code 配置，
	// 保证退出后 CC 不会指向一个已经不存在的代理
	exiting := false
	restoreProxy := func() {
		if exiting {
			return
		}
		exiting = true
		if cfg.ProxyCfg().Enabled {
			fmt.Println("正在关闭观测代理并还原 Claude Code 配置…")
			if _, err := svc.SetProxy(config.ProxyConfig{Enabled: false, Port: cfg.ProxyCfg().Port}); err != nil {
				log.Println("还原 Claude Code 配置失败（请手动检查 settings.json 的 ANTHROPIC_BASE_URL）:", err)
			}
		}
	}
	registerCloseCleanup(restoreProxy)
	go func() {
		<-ctx.Done()
		fmt.Println("\n正在退出…")
		restoreProxy()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	// HTTP 服务放后台 goroutine，主 goroutine 跑托盘消息循环
	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Println("HTTP 服务异常退出:", err)
		}
		tray.Quit()
	}()
	tray.Run(
		func() { openBrowser(url) },
		func() {
			restoreProxy()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			httpSrv.Shutdown(shutdownCtx)
		},
	)
}

// migrateDataDir 旧版本（aitoken）的数据目录整体迁移到新目录，保留全部历史数据
func migrateDataDir(local, newDir string) {
	oldDir := filepath.Join(local, "aitoken")
	if oldDir != newDir {
		if _, err := os.Stat(oldDir); err == nil {
			if _, err := os.Stat(newDir); err != nil {
				if err := os.Rename(oldDir, newDir); err != nil {
					fmt.Println("数据目录迁移失败（将从空数据开始，旧数据仍在", oldDir, "）:", err)
					return
				}
				fmt.Println("数据目录已迁移：", oldDir, "→", newDir)
			}
		}
	}
	// 数据库/日志文件名也随项目改名，逐文件重命名（含 WAL 附属文件）
	for _, f := range []struct{ old, new string }{
		{"aitoken.db", "mapleburn.db"},
		{"aitoken.db-wal", "mapleburn.db-wal"},
		{"aitoken.db-shm", "mapleburn.db-shm"},
		{"aitoken.log", "mapleburn.log"},
	} {
		if _, err := os.Stat(filepath.Join(newDir, f.old)); err == nil {
			os.Rename(filepath.Join(newDir, f.old), filepath.Join(newDir, f.new))
		}
	}
}

// listenLocal 在 [from, to] 范围内找第一个可用端口，只绑定 127.0.0.1
func listenLocal(from, to int) (net.Listener, int, error) {
	for port := from; port <= to; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return l, port, nil
		}
	}
	return nil, 0, fmt.Errorf("端口 %d-%d 均被占用", from, to)
}

func openBrowser(url string) {
	time.Sleep(300 * time.Millisecond)
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
