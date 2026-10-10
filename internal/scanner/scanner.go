// Package scanner 增量扫描本机 Claude Code 会话日志（~/.claude/projects/**/*.jsonl），
// 提取每条 assistant 消息的 token 用量。只读本地文件，不访问网络。
package scanner

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aitoken/internal/pricing"
	"aitoken/internal/store"
)

// logLine Claude Code 日志行（只取需要的字段）
type logLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Message   *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			CacheCreate  int `json:"cache_creation_input_tokens"`
			CacheRead    int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// Result 一次扫描的统计
type Result struct {
	FilesScanned int      `json:"filesScanned"`
	NewRecords   int      `json:"newRecords"`
	LinesParsed  int      `json:"linesParsed"`
	SkippedDirs  []string `json:"skippedDirs"` // 配置了但不存在的目录
}

// Scanner 日志扫描器，支持多个日志目录
type Scanner struct {
	db     *store.DB
	prices *pricing.Table
	mu     sync.RWMutex
	dirs   []string
}

func New(db *store.DB, prices *pricing.Table, dirs []string) *Scanner {
	return &Scanner{db: db, prices: prices, dirs: dirs}
}

// Dirs 返回当前配置的日志目录列表（前端展示用）
func (s *Scanner) Dirs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.dirs...)
}

// SetDirs 更新日志目录列表
func (s *Scanner) SetDirs(dirs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirs = append([]string(nil), dirs...)
}

// Scan 增量扫描全部日志文件
func (s *Scanner) Scan() (Result, error) {
	var res Result

	files, skipped := s.listLogFiles()
	res.SkippedDirs = skipped

	for _, f := range files {
		res.FilesScanned++
		offset, err := s.db.GetOffset(f)
		if err != nil {
			return res, err
		}
		project := filepath.Base(filepath.Dir(f))
		sessionID := filepath.Base(f[:len(f)-len(filepath.Ext(f))])

		newOffset, parsed, usages, err := s.scanFile(f, offset, project, sessionID)
		if err != nil {
			// 单个文件失败不中断整体扫描
			continue
		}
		res.LinesParsed += parsed
		// 先落盘再推进 offset：若顺序颠倒，进程在两者之间崩溃会永久丢掉这批记录
		if len(usages) > 0 {
			n, err := s.db.InsertUsages(usages)
			if err != nil {
				return res, err
			}
			res.NewRecords += n
		}
		if err := s.db.SetOffset(f, newOffset); err != nil {
			return res, err
		}
	}
	return res, nil
}

// listLogFiles 遍历所有配置的目录收集日志文件；不存在的目录记入 skipped 而不是报错
func (s *Scanner) listLogFiles() (files []string, skipped []string) {
	s.mu.RLock()
	dirs := append([]string(nil), s.dirs...)
	s.mu.RUnlock()

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			skipped = append(skipped, dir)
			continue
		}
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // 跳过无权限的子目录
			}
			if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
				files = append(files, path)
			}
			return nil
		})
	}
	return files, skipped
}

// scanFile 从 offset 开始读取一个日志文件，返回新偏移、解析行数和提取出的用量
func (s *Scanner) scanFile(path string, offset int64, project, sessionID string) (int64, int, []store.Usage, error) {
	f, err := os.Open(path)
	if err != nil {
		return offset, 0, nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return offset, 0, nil, err
	}
	// 文件变小了说明被重建过，从头扫
	if info.Size() < offset {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, 0, nil, err
	}

	var usages []store.Usage
	parsed := 0
	reader := bufio.NewReaderSize(f, 256*1024)
	pos := offset

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				// 完整行：解析并前进偏移
				pos += int64(len(line))
				if u, ok := s.parseLine(line, project, sessionID); ok {
					usages = append(usages, u)
					parsed++
				}
			}
			// 未以换行结尾的行是写入中的残行：不前进偏移，
			// 留待下次扫描时从行首重读（否则会永久丢记录）
		}
		if err != nil {
			break // io.EOF 或读取错误都结束本文件
		}
	}
	return pos, parsed, usages, nil
}

// parseLine 解析单行日志；不是带用量的 assistant 消息时返回 false
func (s *Scanner) parseLine(line []byte, project, sessionID string) (store.Usage, bool) {
	// 快速预过滤：不含这两个子串的行一定不是带用量的 assistant 消息，
	// 避免对每行（含大 content 的 user/工具行）都做完整 JSON 解析
	if !bytes.Contains(line, []byte(`"assistant"`)) || !bytes.Contains(line, []byte(`"usage"`)) {
		return store.Usage{}, false
	}

	var l logLine
	if json.Unmarshal(line, &l) != nil ||
		l.Type != "assistant" || l.Message == nil || l.Message.Usage == nil || l.Message.Model == "" {
		return store.Usage{}, false
	}

	ts, err := time.Parse(time.RFC3339Nano, l.Timestamp)
	if err != nil {
		return store.Usage{}, false
	}
	ts = ts.Local()

	u := l.Message.Usage
	if u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheCreate == 0 && u.CacheRead == 0 {
		return store.Usage{}, false
	}

	// 去重哈希：同一条消息（message.id）在流式写入时可能出现多行，usage 逐行累积。
	// 只按消息 ID 哈希，入库时冲突保留 token 数最大的一行（最终快照），避免重复计数。
	// 无 ID 时退化为 会话|时间戳|token 数 的指纹。
	var h [sha1.Size]byte
	if id := l.Message.ID; id != "" {
		h = sha1.Sum(fmt.Appendf(nil, "id|%s", id))
	} else {
		h = sha1.Sum(fmt.Appendf(nil, "%s|%s|%d|%d|%d|%d",
			sessionID, l.Timestamp, u.InputTokens, u.OutputTokens, u.CacheCreate, u.CacheRead))
	}

	model := l.Message.Model
	return store.Usage{
		Hash:          hex.EncodeToString(h[:]),
		Ts:            ts,
		Date:          ts.Format("2006-01-02"),
		Model:         model,
		Project:       project,
		SessionID:     sessionID,
		InputTokens:   u.InputTokens,
		OutputTokens:  u.OutputTokens,
		CacheCreate:   u.CacheCreate,
		CacheRead:     u.CacheRead,
		Cost:          s.prices.Cost(model, u.InputTokens, u.OutputTokens, u.CacheCreate, u.CacheRead),
	}, true
}
