package scanner_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YFFMaple/aitoken/internal/pricing"
	"github.com/YFFMaple/aitoken/internal/scanner"
	"github.com/YFFMaple/aitoken/internal/store"
)

// 用真实的 ~/.claude/projects 日志验证扫描、入库与去重
func TestScanRealLogs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := scanner.New(db, pricing.Load(t.TempDir()), []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".claude", "projects"),
	})

	res, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("首次扫描: %d 个文件, %d 条新记录", res.FilesScanned, res.NewRecords)
	if res.NewRecords == 0 {
		t.Skip("本机日志中没有 assistant usage 记录，跳过断言")
	}

	// 再扫一次，增量去重后应该没有新记录
	res2, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if res2.NewRecords != 0 {
		t.Errorf("二次扫描应有 0 条新记录，实际 %d 条（去重失效）", res2.NewRecords)
	}

	// 今天的日报应该能查到数据
	today := time.Now().Format("2006-01-02")
	totals, err := db.TotalsForDate(today)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("今日: %d 次请求, %d tokens", totals.Requests, totals.TotalTokens)

	models, err := db.ModelsForDate(today)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		t.Logf("  模型 %s: 输入 %d, 输出 %d, 缓存读 %d", m.Model, m.InputTokens, m.OutputTokens, m.CacheRead)
	}

	series, err := db.DailySeries(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 7 {
		t.Errorf("DailySeries(7) 应返回 7 天（含空日期），实际 %d 天", len(series))
	}
}
