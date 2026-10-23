// Package store 负责本地 SQLite 持久化，数据只写入本机，不进行任何网络传输。
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

// Usage 一条 token 消耗记录
type Usage struct {
	Hash         string    // 去重哈希：消息ID + 各 token 数
	Ts           time.Time // 发生时间（本地时区）
	Date         string    // YYYY-MM-DD（本地时区），冗余便于按天查询
	Model        string
	Project      string
	SessionID    string
	InputTokens  int
	OutputTokens int
	CacheCreate  int
	CacheRead    int
}

// UsageRow usages 表的一行（请求流水展示用）
type UsageRow struct {
	Ts           int64  `json:"ts"` // unix 毫秒（本地时区）
	Model        string `json:"model"`
	Project      string `json:"project"`
	SessionID    string `json:"sessionId"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	CacheCreate  int    `json:"cacheCreate"`
	CacheRead    int    `json:"cacheRead"`
}

// EventFilter 请求流水查询条件
type EventFilter struct {
	Start, End string
	Model      string
	Project    string
	Session    string
	Limit      int
	Offset     int
}

// Events 请求流水：按时间倒序，返回本页和总数
func (d *DB) Events(f EventFilter) ([]UsageRow, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where := `date BETWEEN ? AND ?`
	args := []any{f.Start, f.End}
	if f.Model != "" {
		where += ` AND model = ?`
		args = append(args, f.Model)
	}
	if f.Project != "" {
		where += ` AND project = ?`
		args = append(args, f.Project)
	}
	if f.Session != "" {
		where += ` AND session_id = ?`
		args = append(args, f.Session)
	}

	var total int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM usages WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := d.db.Query(
		`SELECT ts, model, project, session_id, input_tokens, output_tokens, cache_create, cache_read
		 FROM usages WHERE `+where+` ORDER BY ts DESC LIMIT ? OFFSET ?`,
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []UsageRow{}
	for rows.Next() {
		var r UsageRow
		var ts any
		if err := rows.Scan(&ts, &r.Model, &r.Project, &r.SessionID,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreate, &r.CacheRead); err != nil {
			return nil, 0, err
		}
		if t, ok := parseTs(ts); ok {
			r.Ts = t.UnixMilli()
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// Heatmap 时段热力图：[7][24]，周一到周日 × 0-23 时的 token 总量
func (d *DB) Heatmap(start, end string) ([][]int64, error) {
	rows, err := d.db.Query(
		`SELECT ts, input_tokens + output_tokens + cache_create + cache_read
		 FROM usages WHERE date BETWEEN ? AND ?`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grid := make([][]int64, 7)
	for i := range grid {
		grid[i] = make([]int64, 24)
	}
	for rows.Next() {
		var ts any
		var tokens int64
		if err := rows.Scan(&ts, &tokens); err != nil {
			return nil, err
		}
		t, ok := parseTs(ts)
		if !ok {
			continue
		}
		weekday := (int(t.Weekday()) + 6) % 7 // 周一 = 0
		grid[weekday][t.Hour()] += tokens
	}
	return grid, rows.Err()
}

// parseTs 兼容驱动返回 time.Time 或字符串两种情况的时间解析
func parseTs(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t.Local(), true
	case string:
		for _, layout := range []string{
			time.RFC3339Nano,
			"2006-01-02 15:04:05.999999999 -0700 MST",
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05",
		} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed.Local(), true
			}
		}
	}
	return time.Time{}, false
}

// ToolStatKey 工具统计的聚合键
type ToolStatKey struct {
	Date    string
	Project string
	Tool    string
}

// ToolStatDelta 一次扫描中某个键的增量
type ToolStatDelta struct {
	Count  int
	Errors int
}

// UpsertToolStats 累加工具调用计数（增量扫描只处理新行，不会重复计数）
func (d *DB) UpsertToolStats(counts map[ToolStatKey]ToolStatDelta) error {
	if len(counts) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO tool_stats (date, project, tool, count, errors) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(date, project, tool) DO UPDATE SET
		 count = count + excluded.count, errors = errors + excluded.errors`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, n := range counts {
		if _, err := stmt.Exec(k.Date, k.Project, k.Tool, n.Count, n.Errors); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ToolStat 单个工具的调用统计
type ToolStat struct {
	Tool   string `json:"tool"`
	Count  int    `json:"count"`
	Errors int    `json:"errors"`
}

// ToolStats 区间内按工具汇总，按次数倒序
func (d *DB) ToolStats(start, end string) ([]ToolStat, error) {
	rows, err := d.db.Query(
		`SELECT tool, SUM(count), SUM(errors) FROM tool_stats
		 WHERE date BETWEEN ? AND ?
		 GROUP BY tool ORDER BY SUM(count) DESC LIMIT 30`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ToolStat
	for rows.Next() {
		var s ToolStat
		if err := rows.Scan(&s.Tool, &s.Count, &s.Errors); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// FileStatKey 文件操作的聚合键
type FileStatKey struct {
	Date    string
	Project string
	Path    string
	Ops     string
}

func (d *DB) UpsertFileStats(counts map[FileStatKey]int) error {
	if len(counts) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO file_stats (date, project, path, ops, count) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(date, project, path, ops) DO UPDATE SET count = count + excluded.count`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, n := range counts {
		if _, err := stmt.Exec(k.Date, k.Project, k.Path, k.Ops, n); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FileStat 单个文件的操作统计
type FileStat struct {
	Path  string `json:"path"`
	Ops   string `json:"ops"` // 出现过的操作，如 "read,edit"
	Count int    `json:"count"`
}

// FileStats 区间内按文件汇总操作次数，取 top 30
func (d *DB) FileStats(start, end string) ([]FileStat, error) {
	rows, err := d.db.Query(
		`SELECT path, GROUP_CONCAT(DISTINCT ops), SUM(count) FROM file_stats
		 WHERE date BETWEEN ? AND ?
		 GROUP BY path ORDER BY SUM(count) DESC LIMIT 30`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FileStat
	for rows.Next() {
		var s FileStat
		if err := rows.Scan(&s.Path, &s.Ops, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Message 一条可被全文搜索的消息文本
type Message struct {
	Hash      string
	SessionID string
	Project   string
	Role      string
	Ts        time.Time
	Text      string
}

// UpsertMessages 批量写入消息文本；hash 冲突时替换（流式快照只留最终版，FTS 由触发器同步）
func (d *DB) UpsertMessages(list []Message) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO messages (hash, session_id, project, role, ts, text) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(hash) DO UPDATE SET
		 ts = excluded.ts, text = excluded.text
		 WHERE length(excluded.text) >= length(messages.text)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range list {
		if _, err := stmt.Exec(m.Hash, m.SessionID, m.Project, m.Role, m.Ts, m.Text); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SearchHit 一条搜索结果
type SearchHit struct {
	SessionID string `json:"sessionId"`
	Project   string `json:"project"`
	Role      string `json:"role"`
	Ts        string `json:"ts"`
	Text      string `json:"text"` // 截断后的全文（前 600 字符），高亮在前端做
}

// SearchMessages 全文搜索：>=3 字符走 FTS trigram（中英混合子串），
// 更短的查询降级 LIKE。按时间倒序返回。
func (d *DB) SearchMessages(q string, limit int) ([]SearchHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	var rows *sql.Rows
	var err error
	if utf8.RuneCountInString(q) >= 3 {
		// 双引号包裹按短语匹配，同时转义查询里自带的引号
		phrase := `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
		rows, err = d.db.Query(
			`SELECT m.session_id, m.project, m.role, m.ts, m.text
			 FROM messages_fts f JOIN messages m ON m.rowid = f.rowid
			 WHERE messages_fts MATCH ?
			 ORDER BY m.ts DESC LIMIT ?`, phrase, limit)
	} else {
		rows, err = d.db.Query(
			`SELECT session_id, project, role, ts, text FROM messages
			 WHERE text LIKE '%' || ? || '%' ESCAPE '\'
			 ORDER BY ts DESC LIMIT ?`, escapeLike(q), limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		var ts any
		if err := rows.Scan(&h.SessionID, &h.Project, &h.Role, &ts, &h.Text); err != nil {
			return nil, err
		}
		h.Ts = formatTs(ts)
		if len(h.Text) > 600 {
			h.Text = h.Text[:600] + "…"
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// escapeLike 转义 LIKE 通配符，保证按字面量搜索
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ActiveDays 区间内有记录的天数（周期报告用）
func (d *DB) ActiveDays(start, end string) (int, error) {
	var n int
	err := d.db.QueryRow(
		`SELECT COUNT(DISTINCT date) FROM usages WHERE date BETWEEN ? AND ?`, start, end).Scan(&n)
	return n, err
}

// Totals 总量
type Totals struct {
	Requests     int `json:"requests"`
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	CacheCreate  int `json:"cacheCreate"`
	CacheRead    int `json:"cacheRead"`
	TotalTokens  int `json:"totalTokens"`
}

// ModelStat 单个模型的汇总
type ModelStat struct {
	Model        string `json:"model"`
	Requests     int    `json:"requests"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	CacheCreate  int    `json:"cacheCreate"`
	CacheRead    int    `json:"cacheRead"`
}

// DayStat 一天的总量（用于趋势图）
type DayStat struct {
	Date string `json:"date"`
	Totals
}

// SessionStat 会话维度汇总
type SessionStat struct {
	SessionID   string `json:"sessionId"`
	Project     string `json:"project"`
	Models      string `json:"models"`
	Requests    int    `json:"requests"`
	TotalTokens int    `json:"totalTokens"`
	FirstTs     string `json:"firstTs"`
	LastTs      string `json:"lastTs"`
}

// ProjectStat 项目维度汇总
type ProjectStat struct {
	Project string `json:"project"`
	Totals
}

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // 串行化写入，避免并发请求触发 SQLITE_BUSY
	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS usages (
			hash          TEXT PRIMARY KEY,
			ts            DATETIME NOT NULL,
			date          TEXT NOT NULL,
			model         TEXT NOT NULL,
			project       TEXT NOT NULL DEFAULT '',
			session_id    TEXT NOT NULL DEFAULT '',
			input_tokens  INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create  INTEGER NOT NULL DEFAULT 0,
			cache_read    INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usages_date ON usages(date)`,
		// 早期版本建过 idx_usages_model，但所有按模型的查询都先按 date 过滤，用不上，清掉
		`DROP INDEX IF EXISTS idx_usages_model`,
		// 记录每个日志文件已扫描到的字节偏移，实现增量扫描
		`CREATE TABLE IF NOT EXISTS file_offsets (
			path   TEXT PRIMARY KEY,
			offset INTEGER NOT NULL DEFAULT 0
		)`,
		// 键值元数据（显示调整开关等）
		`CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,
		// 工具调用统计：按 天+项目+工具 聚合计数（errors 为 tool_result 失败数）
		`CREATE TABLE IF NOT EXISTS tool_stats (
			date    TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			tool    TEXT NOT NULL,
			count   INTEGER NOT NULL DEFAULT 0,
			errors  INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (date, project, tool)
		)`,
		// 文件操作统计：AI 最常读写的文件
		`CREATE TABLE IF NOT EXISTS file_stats (
			date    TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			path    TEXT NOT NULL,
			ops     TEXT NOT NULL DEFAULT '',
			count   INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (date, project, path, ops)
		)`,
		// 消息文本（全文搜索源）：user/assistant 的纯文本，不含工具输出
		`CREATE TABLE IF NOT EXISTS messages (
			hash       TEXT PRIMARY KEY,
			session_id TEXT NOT NULL DEFAULT '',
			project    TEXT NOT NULL DEFAULT '',
			role       TEXT NOT NULL DEFAULT '',
			ts         DATETIME,
			text       TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id)`,
		// trigram 分词支持中英混合子串匹配（<3 字符的查询由应用层降级 LIKE）
		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			text, content='messages', content_rowid='rowid', tokenize='trigram'
		)`,
		`CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
			INSERT INTO messages_fts(rowid, text) VALUES (new.rowid, new.text); END`,
		`CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, text) VALUES('delete', old.rowid, old.text); END`,
		`CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
			INSERT INTO messages_fts(messages_fts, rowid, text) VALUES('delete', old.rowid, old.text);
			INSERT INTO messages_fts(rowid, text) VALUES (new.rowid, new.text); END`,
		// 观测代理记录的网络层请求（独立于日志扫描的 usages）
		`CREATE TABLE IF NOT EXISTS proxy_requests (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			ts            DATETIME NOT NULL,
			provider      TEXT NOT NULL DEFAULT '',
			model         TEXT NOT NULL DEFAULT '',
			path          TEXT NOT NULL DEFAULT '',
			status        INTEGER NOT NULL DEFAULT 0,
			latency_ms    INTEGER NOT NULL DEFAULT 0,
			input_tokens  INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_create  INTEGER NOT NULL DEFAULT 0,
			cache_read    INTEGER NOT NULL DEFAULT 0,
			error         TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_proxy_requests_ts ON proxy_requests(ts)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return err
		}
	}
	if err := d.dropCostColumn(); err != nil {
		return err
	}
	// 老库补 tool_stats.errors 列（新库建表时已带）
	return d.ensureColumn("tool_stats", "errors",
		`ALTER TABLE tool_stats ADD COLUMN errors INTEGER NOT NULL DEFAULT 0`)
}

// ensureColumn 列不存在时执行补列 DDL
func (d *DB) ensureColumn(table, column, ddl string) error {
	rows, err := d.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	has := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			has = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = d.db.Exec(ddl)
	return err
}

// dropCostColumn 费用折算功能已下线，删掉旧库里的 cost 列
func (d *DB) dropCostColumn() error {
	rows, err := d.db.Query(`PRAGMA table_info(usages)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasCost := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "cost" {
			hasCost = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !hasCost {
		return nil
	}
	_, err = d.db.Exec(`ALTER TABLE usages DROP COLUMN cost`)
	return err
}

func (d *DB) Close() error { return d.db.Close() }

// ---- 观测代理请求记录 ----

// ProxyRequest 一条经代理转发的请求记录
type ProxyRequest struct {
	ID           int64  `json:"id"`
	Ts           int64  `json:"ts"` // unix 毫秒
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Path         string `json:"path"`
	Status       int    `json:"status"`
	LatencyMs    int64  `json:"latencyMs"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	CacheCreate  int    `json:"cacheCreate"`
	CacheRead    int    `json:"cacheRead"`
	Error        string `json:"error"`
}

func (d *DB) InsertProxyRequest(r *ProxyRequest) error {
	res, err := d.db.Exec(
		`INSERT INTO proxy_requests
		 (ts, provider, model, path, status, latency_ms, input_tokens, output_tokens, cache_create, cache_read, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		time.UnixMilli(r.Ts).Local(), r.Provider, r.Model, r.Path, r.Status, r.LatencyMs,
		r.InputTokens, r.OutputTokens, r.CacheCreate, r.CacheRead, r.Error)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

// ProxyFilter 代理请求查询条件
type ProxyFilter struct {
	Start, End string // 日期 YYYY-MM-DD，空为不限
	Provider   string
	Model      string
	Limit      int
	Offset     int
}

// ProxyRequests 按时间倒序返回本页和总数
func (d *DB) ProxyRequests(f ProxyFilter) ([]ProxyRequest, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where, args := `1=1`, []any{}
	if f.Start != "" {
		where += ` AND ts >= ?`
		args = append(args, f.Start+" 00:00:00")
	}
	if f.End != "" {
		where += ` AND ts <= ?`
		args = append(args, f.End+" 23:59:59.999999999")
	}
	if f.Provider != "" {
		where += ` AND provider = ?`
		args = append(args, f.Provider)
	}
	if f.Model != "" {
		where += ` AND model = ?`
		args = append(args, f.Model)
	}

	var total int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM proxy_requests WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := d.db.Query(
		`SELECT id, ts, provider, model, path, status, latency_ms, input_tokens, output_tokens, cache_create, cache_read, error
		 FROM proxy_requests WHERE `+where+` ORDER BY ts DESC LIMIT ? OFFSET ?`,
		append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []ProxyRequest{}
	for rows.Next() {
		var r ProxyRequest
		var ts any
		if err := rows.Scan(&r.ID, &ts, &r.Provider, &r.Model, &r.Path, &r.Status, &r.LatencyMs,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreate, &r.CacheRead, &r.Error); err != nil {
			return nil, 0, err
		}
		if t, ok := parseTs(ts); ok {
			r.Ts = t.UnixMilli()
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// ProxySummary 区间内按 服务商+模型 汇总（观测代理页顶部统计用）
func (d *DB) ProxySummary(start, end string) ([]ModelStat, int, error) {
	rows, err := d.db.Query(
		`SELECT provider || ' / ' || model, COUNT(*),
		        COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
		        COALESCE(SUM(cache_create),0), COALESCE(SUM(cache_read),0)
		 FROM proxy_requests
		 WHERE ts >= ? AND ts <= ? AND error = ''
		 GROUP BY provider, model
		 ORDER BY SUM(input_tokens)+SUM(output_tokens)+SUM(cache_create)+SUM(cache_read) DESC
		 LIMIT 20`, start+" 00:00:00", end+" 23:59:59.999999999")
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []ModelStat
	totalReq := 0
	for rows.Next() {
		var m ModelStat
		if err := rows.Scan(&m.Model, &m.Requests, &m.InputTokens, &m.OutputTokens, &m.CacheCreate, &m.CacheRead); err != nil {
			return nil, 0, err
		}
		totalReq += m.Requests
		out = append(out, m)
	}
	return out, totalReq, rows.Err()
}

// GetMeta 读取元数据；不存在返回空串
func (d *DB) GetMeta(key string) string {
	var v string
	if err := d.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v); err != nil {
		return ""
	}
	return v
}

func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// GetOffset 返回日志文件上次扫描到的位置
func (d *DB) GetOffset(path string) (int64, error) {
	var off int64
	err := d.db.QueryRow(`SELECT offset FROM file_offsets WHERE path = ?`, path).Scan(&off)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return off, err
}

func (d *DB) SetOffset(path string, off int64) error {
	_, err := d.db.Exec(
		`INSERT INTO file_offsets (path, offset) VALUES (?, ?)
		 ON CONFLICT(path) DO UPDATE SET offset = excluded.offset`, path, off)
	return err
}

// ResetAll 清空全部用量记录和扫描偏移（完全重新扫描用），meta 等配置保留
func (d *DB) ResetAll() error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"usages", "file_offsets", "tool_stats", "messages", "file_stats"} {
		if _, err := tx.Exec(`DELETE FROM ` + t); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// InsertUsages 批量写入。hash 相同视为同一条消息的不同快照（流式写入会产生多行），
// 保留 token 数更大的一行；完全相同的重复行不受影响。返回实际新增/更新条数。
func (d *DB) InsertUsages(list []Usage) (int, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO usages
		 (hash, ts, date, model, project, session_id, input_tokens, output_tokens, cache_create, cache_read)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(hash) DO UPDATE SET
		   ts = excluded.ts, date = excluded.date, model = excluded.model,
		   project = excluded.project, session_id = excluded.session_id,
		   input_tokens = excluded.input_tokens, output_tokens = excluded.output_tokens,
		   cache_create = excluded.cache_create, cache_read = excluded.cache_read
		 WHERE excluded.input_tokens + excluded.output_tokens + excluded.cache_create + excluded.cache_read
		     > usages.input_tokens + usages.output_tokens + usages.cache_create + usages.cache_read`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, u := range list {
		res, err := stmt.Exec(u.Hash, u.Ts, u.Date, u.Model, u.Project, u.SessionID,
			u.InputTokens, u.OutputTokens, u.CacheCreate, u.CacheRead)
		if err != nil {
			return inserted, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			inserted++
		}
	}
	return inserted, tx.Commit()
}

func (d *DB) totalsWhere(where string, args ...any) (Totals, error) {
	var t Totals
	row := d.db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
		        COALESCE(SUM(cache_create),0), COALESCE(SUM(cache_read),0)
		 FROM usages `+where, args...)
	err := row.Scan(&t.Requests, &t.InputTokens, &t.OutputTokens, &t.CacheCreate, &t.CacheRead)
	t.TotalTokens = t.InputTokens + t.OutputTokens + t.CacheCreate + t.CacheRead
	return t, err
}

// TotalsForDate 某一天的总计
func (d *DB) TotalsForDate(date string) (Totals, error) {
	return d.totalsWhere(`WHERE date = ?`, date)
}

// TotalsBetween 日期区间总计（闭区间）
func (d *DB) TotalsBetween(start, end string) (Totals, error) {
	return d.totalsWhere(`WHERE date BETWEEN ? AND ?`, start, end)
}

// AllTimeTotals 全量总计
func (d *DB) AllTimeTotals() (Totals, error) {
	return d.totalsWhere(``)
}

// ModelsForDate 某天按模型分组的汇总
func (d *DB) ModelsForDate(date string) ([]ModelStat, error) {
	return d.ModelsBetween(date, date)
}

// ModelsBetween 日期区间按模型分组的汇总
func (d *DB) ModelsBetween(start, end string) ([]ModelStat, error) {
	rows, err := d.db.Query(
		`SELECT model, COUNT(*),
		        SUM(input_tokens), SUM(output_tokens), SUM(cache_create), SUM(cache_read)
		 FROM usages WHERE date BETWEEN ? AND ?
		 GROUP BY model ORDER BY SUM(input_tokens)+SUM(output_tokens)+SUM(cache_create)+SUM(cache_read) DESC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModelStat
	for rows.Next() {
		var m ModelStat
		if err := rows.Scan(&m.Model, &m.Requests, &m.InputTokens, &m.OutputTokens, &m.CacheCreate, &m.CacheRead); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DailySeries 最近 N 天逐日汇总
func (d *DB) DailySeries(days int) ([]DayStat, error) {
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	return d.DailySeriesBetween(start, end)
}

// DailySeriesBetween 日期区间逐日汇总（闭区间，补齐无数据的日期）
func (d *DB) DailySeriesBetween(start, end string) ([]DayStat, error) {
	rows, err := d.db.Query(
		`SELECT date, COUNT(*),
		        COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
		        COALESCE(SUM(cache_create),0), COALESCE(SUM(cache_read),0)
		 FROM usages WHERE date BETWEEN ? AND ?
		 GROUP BY date ORDER BY date`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDate := map[string]DayStat{}
	for rows.Next() {
		var s DayStat
		if err := rows.Scan(&s.Date, &s.Requests, &s.InputTokens, &s.OutputTokens, &s.CacheCreate, &s.CacheRead); err != nil {
			return nil, err
		}
		s.TotalTokens = s.InputTokens + s.OutputTokens + s.CacheCreate + s.CacheRead
		byDate[s.Date] = s
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 补齐没有数据的日期，保证图表连续
	startT, err1 := time.Parse("2006-01-02", start)
	endT, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil || endT.Before(startT) {
		return nil, fmt.Errorf("日期区间无效: %s ~ %s", start, end)
	}
	var out []DayStat
	for t := startT; !t.After(endT); t = t.AddDate(0, 0, 1) {
		date := t.Format("2006-01-02")
		if s, ok := byDate[date]; ok {
			out = append(out, s)
		} else {
			out = append(out, DayStat{Date: date})
		}
	}
	return out, nil
}

// SessionsBetween 日期区间内的会话列表，按最后活跃时间倒序
func (d *DB) SessionsBetween(start, end string, limit int) ([]SessionStat, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := d.db.Query(
		`SELECT session_id, project, GROUP_CONCAT(DISTINCT model), COUNT(*),
		        SUM(input_tokens)+SUM(output_tokens)+SUM(cache_create)+SUM(cache_read),
		        MIN(ts), MAX(ts)
		 FROM usages WHERE date BETWEEN ? AND ?
		 GROUP BY session_id, project
		 ORDER BY MAX(ts) DESC LIMIT ?`, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionStat
	for rows.Next() {
		var s SessionStat
		var first, last any
		if err := rows.Scan(&s.SessionID, &s.Project, &s.Models, &s.Requests,
			&s.TotalTokens, &first, &last); err != nil {
			return nil, err
		}
		s.FirstTs = formatTs(first)
		s.LastTs = formatTs(last)
		out = append(out, s)
	}
	return out, rows.Err()
}

// formatTs 兼容驱动返回 time.Time 或字符串两种情况的 ts 格式化
func formatTs(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.Local().Format("2006-01-02 15:04")
	case string:
		// 可能是 RFC3339，也可能是驱动存 time.Time 时的默认格式 "2006-01-02 15:04:05.999 +0800 CST"
		for _, layout := range []string{
			time.RFC3339Nano,
			"2006-01-02 15:04:05.999999999 -0700 MST",
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05",
		} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed.Local().Format("2006-01-02 15:04")
			}
		}
		return t
	default:
		return ""
	}
}

// ProjectTotals 最近 N 天按项目汇总
func (d *DB) ProjectTotals(days int) ([]ProjectStat, error) {
	start := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	end := time.Now().Format("2006-01-02")
	return d.ProjectTotalsBetween(start, end)
}

// ProjectTotalsBetween 日期区间按项目汇总
func (d *DB) ProjectTotalsBetween(start, end string) ([]ProjectStat, error) {
	rows, err := d.db.Query(
		`SELECT project, COUNT(*),
		        COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
		        COALESCE(SUM(cache_create),0), COALESCE(SUM(cache_read),0)
		 FROM usages WHERE date BETWEEN ? AND ?
		 GROUP BY project
		 ORDER BY SUM(input_tokens)+SUM(output_tokens)+SUM(cache_create)+SUM(cache_read) DESC
		 LIMIT 20`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProjectStat
	for rows.Next() {
		var p ProjectStat
		if err := rows.Scan(&p.Project, &p.Requests, &p.InputTokens, &p.OutputTokens, &p.CacheCreate, &p.CacheRead); err != nil {
			return nil, err
		}
		p.TotalTokens = p.InputTokens + p.OutputTokens + p.CacheCreate + p.CacheRead
		out = append(out, p)
	}
	return out, rows.Err()
}
