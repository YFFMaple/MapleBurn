// Package scanner 增量扫描本机 Claude Code 会话日志（~/.claude/projects/**/*.jsonl），
// 提取每条 assistant 消息的 token 用量、工具调用（含失败）、文件操作和消息文本。
// 只读本地文件，不访问网络。
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
	"strings"
	"sync"
	"time"

	"github.com/YFFMaple/MapleBurn/internal/store"
)

// logLine Claude Code 日志行（只取需要的字段）
type logLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	UUID      string `json:"uuid"`
	IsMeta    bool   `json:"isMeta"`
	Message   *struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			CacheCreate  int `json:"cache_creation_input_tokens"`
			CacheRead    int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// contentBlock 消息内容块（text / tool_use / tool_result）
type contentBlock struct {
	Type      string `json:"type"`
	Name      string `json:"name"`        // tool_use 的工具名
	Text      string `json:"text"`        // text 块内容
	ID        string `json:"id"`          // tool_use 的 id
	ToolUseID string `json:"tool_use_id"` // tool_result 回指的 tool_use id
	IsError   bool   `json:"is_error"`    // tool_result 是否失败
	Input     *struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"input"`
}

// ToolCall 一次工具调用或工具失败（统计用）
type ToolCall struct {
	Date string
	Tool string
}

// FileOp 一次文件操作（Read/Write/Edit 的 file_path）
type FileOp struct {
	Date string
	Op   string
	Path string
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
	db   *store.DB
	mu   sync.RWMutex
	dirs []string
}

func New(db *store.DB, dirs []string) *Scanner {
	return &Scanner{db: db, dirs: dirs}
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

// fileScanResult 单文件扫描产物
type fileScanResult struct {
	offset   int64
	parsed   int
	usages   []store.Usage
	tools    []ToolCall
	toolErrs []ToolCall
	files    []FileOp
	texts    []store.Message
}

// Scan 增量扫描全部日志文件
func (s *Scanner) Scan() (Result, error) {
	var res Result
	toolCounts := map[store.ToolStatKey]store.ToolStatDelta{}
	fileCounts := map[store.FileStatKey]int{}
	var texts []store.Message

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

		fr, err := s.scanFile(f, offset, project, sessionID)
		if err != nil {
			// 单个文件失败不中断整体扫描
			continue
		}
		res.LinesParsed += fr.parsed
		// 先落盘再推进 offset：若顺序颠倒，进程在两者之间崩溃会永久丢掉这批记录
		if len(fr.usages) > 0 {
			n, err := s.db.InsertUsages(fr.usages)
			if err != nil {
				return res, err
			}
			res.NewRecords += n
		}
		for _, tc := range fr.tools {
			d := toolCounts[store.ToolStatKey{Date: tc.Date, Project: project, Tool: tc.Tool}]
			d.Count++
			toolCounts[store.ToolStatKey{Date: tc.Date, Project: project, Tool: tc.Tool}] = d
		}
		for _, tc := range fr.toolErrs {
			d := toolCounts[store.ToolStatKey{Date: tc.Date, Project: project, Tool: tc.Tool}]
			d.Errors++
			toolCounts[store.ToolStatKey{Date: tc.Date, Project: project, Tool: tc.Tool}] = d
		}
		for _, fo := range fr.files {
			fileCounts[store.FileStatKey{Date: fo.Date, Project: project, Path: fo.Path, Ops: fo.Op}]++
		}
		texts = append(texts, fr.texts...)
		if err := s.db.SetOffset(f, fr.offset); err != nil {
			return res, err
		}
	}
	if err := s.db.UpsertToolStats(toolCounts); err != nil {
		return res, err
	}
	if err := s.db.UpsertFileStats(fileCounts); err != nil {
		return res, err
	}
	if err := s.db.UpsertMessages(texts); err != nil {
		return res, err
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

// scanFile 从 offset 开始读取一个日志文件
func (s *Scanner) scanFile(path string, offset int64, project, sessionID string) (fileScanResult, error) {
	fr := fileScanResult{offset: offset}
	f, err := os.Open(path)
	if err != nil {
		return fr, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fr, err
	}
	// 文件变小了说明被重建过，从头扫
	if info.Size() < offset {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return fr, err
	}

	// tool_result 只有 tool_use_id，需要同一会话内先出现的 tool_use 行来解析工具名
	toolUseNames := map[string]string{}

	reader := bufio.NewReaderSize(f, 256*1024)
	pos := offset
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 && line[len(line)-1] == '\n' {
			// 完整行：解析并前进偏移。
			// 未以换行结尾的行是写入中的残行：不前进偏移，
			// 留待下次扫描时从行首重读（否则会永久丢记录）
			pos += int64(len(line))
			s.parseLine(line, project, sessionID, toolUseNames, &fr)
		}
		if err != nil {
			break // io.EOF 或读取错误都结束本文件
		}
	}
	fr.offset = pos
	return fr, nil
}

// parseLine 解析单行日志，把用量/工具/文件/文本全部提取到 fr
func (s *Scanner) parseLine(line []byte, project, sessionID string, toolUseNames map[string]string, fr *fileScanResult) {
	// 快速预过滤：必须是 user/assistant 消息行，且含我们关心的内容，
	// 避免对每行（含大 content 的行）都做完整 JSON 解析
	isAssistant := bytes.Contains(line, []byte(`"type":"assistant"`))
	isUser := !isAssistant && bytes.Contains(line, []byte(`"type":"user"`))
	if !isAssistant && !isUser {
		return
	}
	if !bytes.Contains(line, []byte(`"usage"`)) &&
		!bytes.Contains(line, []byte(`"tool_use"`)) &&
		!bytes.Contains(line, []byte(`"tool_result"`)) &&
		!bytes.Contains(line, []byte(`"text"`)) {
		return
	}

	var l logLine
	if json.Unmarshal(line, &l) != nil || l.Message == nil {
		return
	}
	if (isAssistant && l.Type != "assistant") || (isUser && l.Type != "user") {
		return
	}
	if l.IsMeta {
		return // 命令调用等元消息不进任何统计
	}

	ts, err := time.Parse(time.RFC3339Nano, l.Timestamp)
	if err != nil {
		return
	}
	ts = ts.Local()
	date := ts.Format("2006-01-02")

	// content 可能是字符串（user 纯文本）或内容块数组
	var blocks []contentBlock
	if raw := l.Message.Content; len(raw) > 0 {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			if isUser {
				s.addText(sessionID, project, "user", ts, l.UUID, text, fr)
			}
		} else if json.Unmarshal(raw, &blocks) != nil {
			blocks = nil
		}
	}

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if isUser {
				s.addText(sessionID, project, "user", ts, l.UUID, b.Text, fr)
			} else {
				// assistant 文本：流式写入会产生同一 message.id 的多行快照，
				// hash 按消息 id，入库 REPLACE 保留最长（最终）版本
				s.addText(sessionID, project, "assistant", ts, firstNonEmpty(l.Message.ID, l.UUID), b.Text, fr)
			}
		case "tool_use":
			if b.Name == "" {
				continue
			}
			fr.tools = append(fr.tools, ToolCall{Date: date, Tool: b.Name})
			if b.ID != "" {
				toolUseNames[b.ID] = b.Name
			}
			// 文件操作：Read/Write/Edit 系的 file_path
			if b.Input != nil {
				if op, ok := fileOps[b.Name]; ok {
					if p := firstNonEmpty(b.Input.FilePath, b.Input.NotebookPath); p != "" {
						fr.files = append(fr.files, FileOp{Date: date, Op: op, Path: p})
					}
				}
			}
		case "tool_result":
			if b.IsError {
				tool := toolUseNames[b.ToolUseID]
				if tool == "" {
					tool = "unknown"
				}
				fr.toolErrs = append(fr.toolErrs, ToolCall{Date: date, Tool: tool})
			}
		}
	}

	// token 用量（仅 assistant）
	u2 := l.Message.Usage
	if !isAssistant || u2 == nil || l.Message.Model == "" {
		return
	}
	if u2.InputTokens == 0 && u2.OutputTokens == 0 && u2.CacheCreate == 0 && u2.CacheRead == 0 {
		return
	}

	// 去重哈希：同一条消息（message.id）在流式写入时可能出现多行，usage 逐行累积。
	// 只按消息 ID 哈希，入库时冲突保留 token 数最大的一行（最终快照），避免重复计数。
	// 无 ID 时退化为 会话|时间戳|token 数 的指纹。
	var h [sha1.Size]byte
	if id := l.Message.ID; id != "" {
		h = sha1.Sum(fmt.Appendf(nil, "id|%s", id))
	} else {
		h = sha1.Sum(fmt.Appendf(nil, "%s|%s|%d|%d|%d|%d",
			sessionID, l.Timestamp, u2.InputTokens, u2.OutputTokens, u2.CacheCreate, u2.CacheRead))
	}

	fr.usages = append(fr.usages, store.Usage{
		Hash:         hex.EncodeToString(h[:]),
		Ts:           ts,
		Date:         date,
		Model:        l.Message.Model,
		Project:      project,
		SessionID:    sessionID,
		InputTokens:  u2.InputTokens,
		OutputTokens: u2.OutputTokens,
		CacheCreate:  u2.CacheCreate,
		CacheRead:    u2.CacheRead,
	})
	fr.parsed++
}

// fileOps 会产生文件统计的工具名 → 操作名
var fileOps = map[string]string{
	"Read":         "read",
	"Write":        "write",
	"Edit":         "edit",
	"NotebookEdit": "edit",
	"MultiEdit":    "edit",
}

// addText 把一条消息文本加入扫描产物；太短的、XML 包裹的系统味文本不索引
func (s *Scanner) addText(sessionID, project, role string, ts time.Time, id, text string, fr *fileScanResult) {
	text = strings.TrimSpace(text)
	if len(text) < 2 || strings.HasPrefix(text, "<") {
		return
	}
	if id == "" {
		id = fmt.Sprintf("%s|%s|%.32s", sessionID, ts.Format(time.RFC3339Nano), text)
	}
	h := sha1.Sum(fmt.Appendf(nil, "msg|%s|%s", role, id))
	fr.texts = append(fr.texts, store.Message{
		Hash:      hex.EncodeToString(h[:]),
		SessionID: sessionID,
		Project:   project,
		Role:      role,
		Ts:        ts,
		Text:      text,
	})
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
