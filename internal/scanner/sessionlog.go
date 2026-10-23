package scanner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionMessage 会话中的一条文本消息
type SessionMessage struct {
	Role         string `json:"role"` // user | assistant
	Text         string `json:"text"`
	Ts           string `json:"ts"`
	Model        string `json:"model,omitempty"`
	InputTokens  int    `json:"inputTokens,omitempty"`
	OutputTokens int    `json:"outputTokens,omitempty"`
}

const (
	maxSessionMessages = 2000 // 单个会话最多返回的消息数
	maxMessageRunes    = 8000 // 单条消息最大字符数（超出截断）
)

// msgLine 日志行的消息部分（content 可能是字符串或块数组）
type msgLine struct {
	Type      string `json:"type"`
	IsMeta    bool   `json:"isMeta"`
	Timestamp string `json:"timestamp"`
	Message   *struct {
		Role    string          `json:"role"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ReadSessionMessages 读取一个会话日志文件，提取用户/助手的文本消息。
// 在所有配置的日志目录中查找，取第一个命中的文件。
func (s *Scanner) ReadSessionMessages(project, session string) ([]SessionMessage, error) {
	// 防路径穿越
	if project == "" || session == "" ||
		strings.ContainsAny(project, `/\`) || strings.ContainsAny(session, `/\`) ||
		strings.Contains(project, "..") || strings.Contains(session, "..") {
		return nil, fmt.Errorf("非法的 project 或 session 参数")
	}

	s.mu.RLock()
	dirs := append([]string(nil), s.dirs...)
	s.mu.RUnlock()

	var path string
	for _, dir := range dirs {
		candidate := filepath.Join(dir, project, session+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("会话日志不存在: %s/%s", project, session)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("会话日志不存在: %w", err)
	}
	defer f.Close()

	var out []SessionMessage
	reader := bufio.NewReaderSize(f, 256*1024)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if m, ok := parseMessage(line); ok {
				out = append(out, m)
				if len(out) >= maxSessionMessages {
					break
				}
			}
		}
		if err != nil {
			break
		}
	}
	return out, nil
}

// parseMessage 从日志行提取一条文本消息；非文本内容（thinking/工具调用等）跳过
func parseMessage(line []byte) (SessionMessage, bool) {
	// 快速预过滤：user/assistant 之外的角色行直接跳过，省去完整 JSON 解析
	if !bytes.Contains(line, []byte(`"user"`)) && !bytes.Contains(line, []byte(`"assistant"`)) {
		return SessionMessage{}, false
	}

	var l msgLine
	if json.Unmarshal(line, &l) != nil || l.Message == nil || l.IsMeta {
		return SessionMessage{}, false
	}
	if l.Type != "user" && l.Type != "assistant" {
		return SessionMessage{}, false
	}

	text := extractText(l.Message.Content)
	if text == "" {
		return SessionMessage{}, false
	}
	if runes := []rune(text); len(runes) > maxMessageRunes {
		text = string(runes[:maxMessageRunes]) + "\n\n…（内容过长已截断）"
	}

	m := SessionMessage{Role: l.Type, Text: text, Model: l.Message.Model}
	if ts, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
		m.Ts = ts.Local().Format("2006-01-02 15:04:05")
	}
	if l.Message.Usage != nil {
		m.InputTokens = l.Message.Usage.InputTokens
		m.OutputTokens = l.Message.Usage.OutputTokens
	}
	return m, true
}

// extractText content 可能是纯字符串，也可能是内容块数组，只取 text 块
func extractText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
