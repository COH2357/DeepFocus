package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrLastQuote is returned when deleting the final quote. The blocker must
// always have at least one line to show.
var ErrLastQuote = errors.New("at least one quote is required")

type BlockQuote struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	CreatedAt int64  `json:"createdAt"`
}

type BlockEvent struct {
	ID           int64  `json:"id"`
	Time         int64  `json:"time"`
	Date         string `json:"date"`
	Kind         string `json:"kind"`
	Target       string `json:"target"`
	Action       string `json:"action"`
	LimitMinutes int    `json:"limitMinutes"`
	UsedSeconds  int    `json:"usedSeconds"`
	Reason       string `json:"reason"`
}

const quotesSeededKey = "block_quotes_seeded_v1"

var defaultBlockQuotes = []string{
	"你不是没有时间，只是把时间交给了屏幕。",
	"先做十分钟，再决定要不要继续。",
	"注意力是你最贵的资产。",
	"放下屏幕，不是失去世界，是拿回自己。",
	"真正想做的事，不需要提醒。",
	"把刷视频的十分钟，还给想成为的自己。",
	"现在关掉它，未来的你会感谢你。",
	"意志力不是天生的，是一次次选择练出来的。",
	"不要让算法替你决定今天。",
	"休息不是刷屏，是让大脑真正安静。",
	"你可以随时回来，但不必现在。",
	"今天少刷一会儿，明天多一点自由。",
	"看完这一条，就到这里。",
	"时间不会提醒你它在走。",
	"选择困难时，选那个更难但更值得的。",
	"你不是在放松，你是在被消耗。",
	"每一次放下，都是一次小小的胜利。",
	"专注一小时，胜过刷屏一整天。",
	"让屏幕为你工作，而不是相反。",
	"你不需要更多信息，你需要一个决定。",
	"现在就停，不用等到整点。",
	"把注意力留给重要的人。",
	"生活不在下一条视频里。",
	"做自己的时间管理者。",
}

// SeedBlockQuotes installs the starter quotes once. Deleting a quote later does
// not re-seed the list, and the UI refuses to delete the final row.
func SeedBlockQuotes() error {
	if db == nil {
		return errors.New("database not initialized")
	}
	if value, err := GetConfig(quotesSeededKey); err == nil && value == "true" {
		return nil
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM block_quotes`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for _, text := range defaultBlockQuotes {
			if _, err := tx.Exec(`INSERT INTO block_quotes (text, created_at) VALUES (?, ?)`, text, time.Now().Unix()); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return SetConfig(quotesSeededKey, "true")
}

func ListBlockQuotes() ([]BlockQuote, error) {
	rows, err := db.Query(`SELECT id, text, created_at FROM block_quotes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlockQuote
	for rows.Next() {
		var quote BlockQuote
		if err := rows.Scan(&quote.ID, &quote.Text, &quote.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, quote)
	}
	return out, rows.Err()
}

func RandomBlockQuote() (string, error) {
	var text string
	err := db.QueryRow(`SELECT text FROM block_quotes ORDER BY RANDOM() LIMIT 1`).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return text, err
}

func AddBlockQuote(text string) (int64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, errors.New("quote is empty")
	}
	if len([]rune(text)) > 240 {
		text = string([]rune(text)[:240])
	}
	res, err := db.Exec(`INSERT INTO block_quotes (text, created_at) VALUES (?, ?)`, text, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateBlockQuote(id int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("quote is empty")
	}
	if len([]rune(text)) > 240 {
		text = string([]rune(text)[:240])
	}
	res, err := db.Exec(`UPDATE block_quotes SET text = ? WHERE id = ?`, text, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func DeleteBlockQuote(id int64) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM block_quotes`).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastQuote
	}
	res, err := db.Exec(`DELETE FROM block_quotes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RecordBlockEvent appends an audit row. It is intentionally append-only: a
// bypass is never hidden by later edits.
func RecordBlockEvent(kind, target, action string, limitMinutes, usedSeconds int, reason string) error {
	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO block_events (ts, date, kind, target, action, limit_minutes, used_seconds, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, now.Unix(), now.Format("2006-01-02"), kind, target, action, limitMinutes, usedSeconds, strings.TrimSpace(reason))
	return err
}

func ListBlockEvents(limit int) ([]BlockEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Query(`
		SELECT id, ts, date, kind, target, action, limit_minutes, used_seconds, reason
		FROM block_events ORDER BY ts DESC, id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BlockEvent, 0, limit)
	for rows.Next() {
		var e BlockEvent
		if err := rows.Scan(&e.ID, &e.Time, &e.Date, &e.Kind, &e.Target, &e.Action, &e.LimitMinutes, &e.UsedSeconds, &e.Reason); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// BlockStatsToday returns blocked and bypassed counts for the current date.
func BlockStatsToday() (blocked, bypassed int) {
	today := Today()
	_ = db.QueryRow(`SELECT COUNT(*) FROM block_events WHERE date = ? AND action = 'blocked'`, today).Scan(&blocked)
	_ = db.QueryRow(`SELECT COUNT(*) FROM block_events WHERE date = ? AND action = 'bypassed'`, today).Scan(&bypassed)
	return blocked, bypassed
}

func HasBlockEventToday(kind, target string) bool {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM block_events WHERE date = ? AND kind = ? AND target = ? AND action = 'blocked'`,
		Today(), kind, target).Scan(&count)
	return err == nil && count > 0
}

func SetBlockBypass(kind, target string, until time.Time, reason string) error {
	_, err := db.Exec(`
		INSERT INTO block_bypasses (kind, target, until_ts, reason, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(kind, target) DO UPDATE SET
			until_ts = excluded.until_ts,
			reason = excluded.reason,
			created_at = excluded.created_at
	`, kind, target, until.Unix(), strings.TrimSpace(reason), time.Now().Unix())
	return err
}

func ActiveBlockBypass(kind, target string, now time.Time) bool {
	var until int64
	err := db.QueryRow(`SELECT until_ts FROM block_bypasses WHERE kind = ? AND target = ?`, kind, target).Scan(&until)
	return err == nil && until > now.Unix()
}

func ClearBlockBypass(kind, target string) error {
	_, err := db.Exec(`DELETE FROM block_bypasses WHERE kind = ? AND target = ?`, kind, target)
	return err
}

// SetBlockHold briefly suppresses enforcement while the blocker itself is
// closing the matching tabs. It is not an allowance: no event is recorded and
// enforcement resumes as soon as the hold expires or is cleared.
func SetBlockHold(kind, target string, until time.Time) error {
	_, err := db.Exec(`
		INSERT INTO block_holds (kind, target, until_ts)
		VALUES (?, ?, ?)
		ON CONFLICT(kind, target) DO UPDATE SET until_ts = excluded.until_ts
	`, kind, target, until.Unix())
	return err
}

func ActiveBlockHold(kind, target string, now time.Time) bool {
	var until int64
	err := db.QueryRow(`SELECT until_ts FROM block_holds WHERE kind = ? AND target = ?`, kind, target).Scan(&until)
	return err == nil && until > now.Unix()
}

func ClearBlockHold(kind, target string) error {
	_, err := db.Exec(`DELETE FROM block_holds WHERE kind = ? AND target = ?`, kind, target)
	return err
}
