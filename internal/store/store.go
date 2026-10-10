// Package store 负责本地 SQLite 持久化，数据只写入本机，不进行任何网络传输。
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Usage 一条 token 消耗记录
type Usage struct {
	Hash          string    // 去重哈希：消息ID + 各 token 数
	Ts            time.Time // 发生时间（本地时区）
	Date          string    // YYYY-MM-DD（本地时区），冗余便于按天查询
	Model         string
	Project       string
	SessionID     string
	InputTokens   int
	OutputTokens  int
	CacheCreate   int
	CacheRead     int
	Cost          float64 // 按本地价格表折算的美元，未知模型为 0
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
			cache_read    INTEGER NOT NULL DEFAULT 0,
			cost          REAL NOT NULL DEFAULT 0
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
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// AllForReprice 读取全部记录（重算费用用），含当前 cost 以便跳过未变化的行
func (d *DB) AllForReprice() ([]Usage, error) {
	rows, err := d.db.Query(`SELECT hash, model, input_tokens, output_tokens, cache_create, cache_read, cost FROM usages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Usage
	for rows.Next() {
		var u Usage
		if err := rows.Scan(&u.Hash, &u.Model, &u.InputTokens, &u.OutputTokens, &u.CacheCreate, &u.CacheRead, &u.Cost); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CostUpdate 一条费用更新
type CostUpdate struct {
	Hash string
	Cost float64
}

// UpdateCosts 批量更新费用（分片事务，避免大表单事务锁库过久）。返回更新条数。
func (d *DB) UpdateCosts(items []CostUpdate) (int, error) {
	const chunk = 2000
	total := 0
	for start := 0; start < len(items); start += chunk {
		end := min(start+chunk, len(items))
		n, err := d.updateCostsChunk(items[start:end])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (d *DB) updateCostsChunk(items []CostUpdate) (int, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE usages SET cost = ? WHERE hash = ?`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	n := 0
	for _, it := range items {
		if _, err := stmt.Exec(it.Cost, it.Hash); err != nil {
			return n, err
		}
		n++
	}
	return n, tx.Commit()
}

// ResetAll 清空全部用量记录和扫描偏移（完全重新扫描用），meta 等配置保留
func (d *DB) ResetAll() error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"usages", "file_offsets"} {
		if _, err := tx.Exec(`DELETE FROM ` + t); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) Close() error { return d.db.Close() }

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
		 (hash, ts, date, model, project, session_id, input_tokens, output_tokens, cache_create, cache_read, cost)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(hash) DO UPDATE SET
		   ts = excluded.ts, date = excluded.date, model = excluded.model,
		   project = excluded.project, session_id = excluded.session_id,
		   input_tokens = excluded.input_tokens, output_tokens = excluded.output_tokens,
		   cache_create = excluded.cache_create, cache_read = excluded.cache_read,
		   cost = excluded.cost
		 WHERE excluded.input_tokens + excluded.output_tokens + excluded.cache_create + excluded.cache_read
		     > usages.input_tokens + usages.output_tokens + usages.cache_create + usages.cache_read`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, u := range list {
		res, err := stmt.Exec(u.Hash, u.Ts, u.Date, u.Model, u.Project, u.SessionID,
			u.InputTokens, u.OutputTokens, u.CacheCreate, u.CacheRead, u.Cost)
		if err != nil {
			return inserted, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			inserted++
		}
	}
	return inserted, tx.Commit()
}

// ModelStat 单个模型的汇总
type ModelStat struct {
	Model        string  `json:"model"`
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CacheCreate  int     `json:"cacheCreate"`
	CacheRead    int     `json:"cacheRead"`
	Cost         float64 `json:"cost"`
}

// Totals 总量
type Totals struct {
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CacheCreate  int     `json:"cacheCreate"`
	CacheRead    int     `json:"cacheRead"`
	TotalTokens  int     `json:"totalTokens"`
	Cost         float64 `json:"cost"`
}

func (d *DB) totalsWhere(where string, args ...any) (Totals, error) {
	var t Totals
	row := d.db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
		        COALESCE(SUM(cache_create),0), COALESCE(SUM(cache_read),0),
		        COALESCE(SUM(cost),0)
		 FROM usages `+where, args...)
	err := row.Scan(&t.Requests, &t.InputTokens, &t.OutputTokens, &t.CacheCreate, &t.CacheRead, &t.Cost)
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
		        SUM(input_tokens), SUM(output_tokens), SUM(cache_create), SUM(cache_read), SUM(cost)
		 FROM usages WHERE date BETWEEN ? AND ?
		 GROUP BY model ORDER BY SUM(input_tokens)+SUM(output_tokens)+SUM(cache_create)+SUM(cache_read) DESC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModelStats(rows)
}

func scanModelStats(rows *sql.Rows) ([]ModelStat, error) {
	var out []ModelStat
	for rows.Next() {
		var m ModelStat
		if err := rows.Scan(&m.Model, &m.Requests, &m.InputTokens, &m.OutputTokens, &m.CacheCreate, &m.CacheRead, &m.Cost); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DayStat 一天的总量（用于趋势图）
type DayStat struct {
	Date  string `json:"date"`
	Totals
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
		        COALESCE(SUM(cache_create),0), COALESCE(SUM(cache_read),0), COALESCE(SUM(cost),0)
		 FROM usages WHERE date BETWEEN ? AND ?
		 GROUP BY date ORDER BY date`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDate := map[string]DayStat{}
	for rows.Next() {
		var s DayStat
		if err := rows.Scan(&s.Date, &s.Requests, &s.InputTokens, &s.OutputTokens, &s.CacheCreate, &s.CacheRead, &s.Cost); err != nil {
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

// SessionStat 会话维度汇总
type SessionStat struct {
	SessionID   string  `json:"sessionId"`
	Project     string  `json:"project"`
	Models      string  `json:"models"`
	Requests    int     `json:"requests"`
	TotalTokens int     `json:"totalTokens"`
	Cost        float64 `json:"cost"`
	FirstTs     string  `json:"firstTs"`
	LastTs      string  `json:"lastTs"`
}

// SessionsBetween 日期区间内的会话列表，按最后活跃时间倒序
func (d *DB) SessionsBetween(start, end string, limit int) ([]SessionStat, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := d.db.Query(
		`SELECT session_id, project, GROUP_CONCAT(DISTINCT model), COUNT(*),
		        SUM(input_tokens)+SUM(output_tokens)+SUM(cache_create)+SUM(cache_read),
		        COALESCE(SUM(cost),0), MIN(ts), MAX(ts)
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
			&s.TotalTokens, &s.Cost, &first, &last); err != nil {
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
type ProjectStat struct {
	Project string `json:"project"`
	Totals
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
		        COALESCE(SUM(cache_create),0), COALESCE(SUM(cache_read),0), COALESCE(SUM(cost),0)
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
		if err := rows.Scan(&p.Project, &p.Requests, &p.InputTokens, &p.OutputTokens, &p.CacheCreate, &p.CacheRead, &p.Cost); err != nil {
			return nil, err
		}
		p.TotalTokens = p.InputTokens + p.OutputTokens + p.CacheCreate + p.CacheRead
		out = append(out, p)
	}
	return out, rows.Err()
}
