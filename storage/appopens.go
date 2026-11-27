package storage

import (
	"database/sql"
	"errors"
	"time"
)

type AppOpenSession struct {
	ID       int64  `json:"id"`
	Date     string `json:"date"`
	AppKey   string `json:"appKey"`
	ExeName  string `json:"exeName"`
	OpenedAt int64  `json:"openedAt"`
	ClosedAt int64  `json:"closedAt"`
	Deadline int64  `json:"deadline"`
	Minutes  int    `json:"minutes"`
}

func OpenAppSession(date, appKey, exeName string, openedAt time.Time, minutes int) (int64, error) {
	deadline := openedAt.Add(time.Duration(minutes) * time.Minute)
	res, err := db.Exec(`
		INSERT INTO app_open_sessions (date, app_key, exe_name, opened_at, closed_at, deadline, minutes)
		VALUES (?, ?, ?, ?, NULL, ?, ?)
	`, date, appKey, exeName, openedAt.Unix(), deadline.Unix(), minutes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func CloseAppSession(id int64, at time.Time) error {
	_, err := db.Exec(`UPDATE app_open_sessions SET closed_at = ? WHERE id = ? AND closed_at IS NULL`, at.Unix(), id)
	return err
}

func CountAppOpens(date, appKey string) int {
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM app_open_sessions WHERE date = ? AND app_key = ?`, date, appKey).Scan(&count)
	return count
}

func ActiveAppSession(appKey string) (*AppOpenSession, error) {
	var s AppOpenSession
	err := db.QueryRow(`
		SELECT id, date, app_key, COALESCE(exe_name,''), opened_at, COALESCE(closed_at,0), deadline, minutes
		FROM app_open_sessions
		WHERE app_key = ? AND closed_at IS NULL
		ORDER BY id DESC LIMIT 1
	`, appKey).Scan(&s.ID, &s.Date, &s.AppKey, &s.ExeName, &s.OpenedAt, &s.ClosedAt, &s.Deadline, &s.Minutes)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func UpdateAppSessionDeadline(id int64, deadline time.Time, minutes int) error {
	_, err := db.Exec(`UPDATE app_open_sessions SET deadline = ?, minutes = ? WHERE id = ?`, deadline.Unix(), minutes, id)
	return err
}

func SetSiteAllowance(site string, until time.Time, reason string) error {
	_, err := db.Exec(`
		INSERT INTO site_allowances (site, until_ts, reason, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(site) DO UPDATE SET
			until_ts = excluded.until_ts,
			reason = excluded.reason,
			created_at = excluded.created_at
	`, site, until.Unix(), reason, time.Now().Unix())
	return err
}

func SiteAllowanceUntil(site string, now time.Time) (time.Time, bool) {
	var until int64
	err := db.QueryRow(`SELECT until_ts FROM site_allowances WHERE site = ?`, site).Scan(&until)
	if err != nil || until <= now.Unix() {
		return time.Time{}, false
	}
	return time.Unix(until, 0), true
}

func ClearSiteAllowance(site string) error {
	_, err := db.Exec(`DELETE FROM site_allowances WHERE site = ?`, site)
	return err
}

func SetExtensionSeen(browser, version string, at time.Time) error {
	_, err := db.Exec(`
		INSERT INTO extension_status (browser, seen_at, version)
		VALUES (?, ?, ?)
		ON CONFLICT(browser) DO UPDATE SET seen_at = excluded.seen_at, version = excluded.version
	`, browser, at.Unix(), version)
	return err
}

func ExtensionSeen(browser string, within time.Duration) bool {
	var seen int64
	err := db.QueryRow(`SELECT seen_at FROM extension_status WHERE browser = ?`, browser).Scan(&seen)
	return err == nil && time.Since(time.Unix(seen, 0)) <= within
}
