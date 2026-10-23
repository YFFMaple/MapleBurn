package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Usage 从响应中提取出的用量（各 adapter 统一到这个结构）
type Usage struct {
	Model       string
	Input       int
	Output      int
	CacheCreate int
	CacheRead   int
}

// sseTap 流式响应的旁路解析器：数据流原样透传给客户端，
// 这里只把含 "usage" 的 SSE data 行攒下来，流结束后统一解析。
// 不缓存完整响应体，长流式响应也没有内存压力。
type sseTap struct {
	partial []byte   // 未换行的残行
	lines   [][]byte // 捕获到的 data 行（通常只有 2~3 条 usage 事件）
}

func (t *sseTap) Feed(p []byte) {
	buf := append(t.partial, p...)
	for {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			break
		}
		line := bytes.TrimRight(buf[:i], "\r")
		buf = buf[i+1:]
		if bytes.HasPrefix(line, []byte("data:")) && bytes.Contains(line, []byte(`"usage"`)) {
			data := bytes.TrimSpace(line[len("data:"):])
			if len(data) > 0 && !bytes.Equal(data, []byte("[DONE]")) {
				t.lines = append(t.lines, append([]byte(nil), data...))
			}
		}
	}
	t.partial = buf
}

// usage 把捕获到的所有 data 行交给对应 adapter 归并
func (t *sseTap) usage(adapter string) Usage {
	var u Usage
	for _, line := range t.lines {
		mergePayload(adapter, line, &u)
	}
	return u
}

// parseJSONUsage 非流式响应：整个响应体就是一个 JSON，按 adapter 解析
func parseJSONUsage(adapter string, body []byte) Usage {
	var u Usage
	mergePayload(adapter, body, &u)
	return u
}

// mergePayload 解析一条 JSON 负载并归并进 u（同字段取最后一次非零值）
func mergePayload(adapter string, payload []byte, u *Usage) {
	switch adapter {
	case "openai":
		mergeOpenAI(payload, u)
	case "gemini":
		mergeGemini(payload, u)
	default: // anthropic
		mergeAnthropic(payload, u)
	}
}

// ---- Anthropic 形态 ----
// message_start: {"type":"message_start","message":{"model":"...","usage":{"input_tokens":N,"cache_creation_input_tokens":N,"cache_read_input_tokens":N}}}
// message_delta: {"type":"message_delta","usage":{"output_tokens":N}}
// 非流式:      {"model":"...","usage":{"input_tokens":N,"output_tokens":N,...}}
func mergeAnthropic(payload []byte, u *Usage) {
	var p struct {
		Model   string `json:"model"`
		Message *struct {
			Model string `json:"model"`
			Usage *struct {
				Input       int `json:"input_tokens"`
				Output      int `json:"output_tokens"`
				CacheCreate int `json:"cache_creation_input_tokens"`
				CacheRead   int `json:"cache_read_input_tokens"`
			} `json:"usage"`
		} `json:"message"`
		Usage *struct {
			Input       int `json:"input_tokens"`
			Output      int `json:"output_tokens"`
			CacheCreate int `json:"cache_creation_input_tokens"`
			CacheRead   int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return
	}
	if p.Message != nil {
		if p.Message.Model != "" {
			u.Model = p.Message.Model
		}
		if m := p.Message.Usage; m != nil {
			if m.Input > 0 {
				u.Input = m.Input
			}
			if m.CacheCreate > 0 {
				u.CacheCreate = m.CacheCreate
			}
			if m.CacheRead > 0 {
				u.CacheRead = m.CacheRead
			}
			if m.Output > u.Output {
				u.Output = m.Output
			}
		}
	}
	if p.Model != "" && u.Model == "" {
		u.Model = p.Model
	}
	if m := p.Usage; m != nil {
		if m.Input > 0 {
			u.Input = m.Input
		}
		if m.Output > u.Output {
			u.Output = m.Output
		}
		if m.CacheCreate > 0 {
			u.CacheCreate = m.CacheCreate
		}
		if m.CacheRead > 0 {
			u.CacheRead = m.CacheRead
		}
	}
}

// ---- OpenAI 形态 ----
// {"model":"...","usage":{"prompt_tokens":N,"completion_tokens":N,"prompt_tokens_details":{"cached_tokens":N}}}
func mergeOpenAI(payload []byte, u *Usage) {
	var p struct {
		Model string `json:"model"`
		Usage *struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
			Details    *struct {
				Cached int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return
	}
	if p.Model != "" {
		u.Model = p.Model
	}
	if m := p.Usage; m != nil {
		if m.Prompt > 0 {
			u.Input = m.Prompt
		}
		if m.Completion > 0 {
			u.Output = m.Completion
		}
		if m.Details != nil && m.Details.Cached > 0 {
			u.CacheRead = m.Details.Cached
		}
	}
}

// ---- Gemini 形态 ----
// {"usageMetadata":{"promptTokenCount":N,"candidatesTokenCount":N,"cachedContentTokenCount":N}}
func mergeGemini(payload []byte, u *Usage) {
	var p struct {
		Usage *struct {
			Prompt     int `json:"promptTokenCount"`
			Candidates int `json:"candidatesTokenCount"`
			Cached     int `json:"cachedContentTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(payload, &p) != nil || p.Usage == nil {
		return
	}
	if p.Usage.Prompt > 0 {
		u.Input = p.Usage.Prompt
	}
	if p.Usage.Candidates > 0 {
		u.Output = p.Usage.Candidates
	}
	if p.Usage.Cached > 0 {
		u.CacheRead = p.Usage.Cached
	}
}

// modelFromBody 从请求体里取 model 字段（anthropic / openai 形态都有）
func modelFromBody(body []byte) string {
	var p struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &p) == nil {
		return p.Model
	}
	return ""
}

// modelFromPath Gemini 把模型放在路径里：/v1beta/models/{model}:generateContent
func modelFromPath(path string) string {
	i := strings.Index(path, "/models/")
	if i < 0 {
		return ""
	}
	rest := path[i+len("/models/"):]
	if j := strings.IndexAny(rest, ":/?"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}
