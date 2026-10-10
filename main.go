package main

import (
	"context"
	"embed"
	"fmt"
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

	"aitoken/internal/pricing"
	"aitoken/internal/scanner"
	"aitoken/internal/store"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func main() {
	// 数据目录：%LOCALAPPDATA%\aitoken（仅本机）
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local, _ = os.UserHomeDir()
	}
	dataDir := filepath.Join(local, "aitoken")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("无法创建数据目录: ", err)
	}

	db, err := store.Open(filepath.Join(dataDir, "aitoken.db"))
	if err != nil {
		log.Fatal("无法打开本地数据库: ", err)
	}
	defer db.Close()

	srv := &Server{
		db:      db,
		prices:  pricing.Load(dataDir),
		dataDir: dataDir,
		config:  loadConfig(dataDir),
	}
	srv.scanner = scanner.New(db, srv.prices, srv.config.Dirs())

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
	srv.registerAPI(mux)

	fmt.Println("┌──────────────────────────────────────────┐")
	fmt.Println("│        AI Token 统计（本地版）            │")
	fmt.Println("└──────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("  访问地址:  ", url)
	fmt.Println("  日志目录:  ", strings.Join(srv.scanner.Dirs(), " ; "))
	fmt.Println("  数据目录:  ", dataDir)
	fmt.Println()
	fmt.Println("  浏览器会自动打开；关闭此窗口即退出程序。")
	fmt.Println()

	go openBrowser(url)

	httpSrv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	// Ctrl+C 时优雅退出，让 defer db.Close() 有机会执行
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		fmt.Println("\n正在退出…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
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
