package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	archiveDirName   = "archive"
	archiveFileGlob  = "deepfocus-week-"
	archiveExtension = ".db"
)

// GetArchiveDir returns the directory that holds the weekly archive databases.
func GetArchiveDir() (string, error) {
	dataDir, err := getDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, archiveDirName), nil
}

// ArchiveSummary reports how many weekly archives exist and the newest one.
// The information is display-only; it never creates or changes a file.
func ArchiveSummary() (count int, latest string) {
	dir, err := GetArchiveDir()
	if err != nil {
		return 0, ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, archiveFileGlob) || !strings.HasSuffix(name, archiveExtension) {
			continue
		}
		count++
		if name > latest {
			latest = name
		}
	}
	return count, latest
}

// CompletedWeeks returns the Monday of the current week at local midnight.
// Everything before it is complete and safe to archive.
func currentWeekStart(now time.Time) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	offset := (int(day.Weekday()) + 6) % 7 // Monday = 0, Sunday = 6
	return day.AddDate(0, 0, -offset)
}

func weekStartOf(day time.Time) time.Time {
	d := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}

func dateString(t time.Time) string {
	return t.Format("2006-01-02")
}

func earliestTrackingDate() (string, error) {
	var earliest sql.NullString
	err := db.QueryRow(`
		SELECT MIN(date) FROM (
			SELECT date FROM sessions
			UNION ALL SELECT date FROM apps_daily
			UNION ALL SELECT date FROM browsing_daily
		)
	`).Scan(&earliest)
	if err != nil {
		return "", err
	}
	if !earliest.Valid {
		return "", nil
	}
	return earliest.String, nil
}

func rowsInRange(from, to string) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM sessions WHERE date >= ? AND date <= ?) +
			(SELECT COUNT(*) FROM apps_daily WHERE date >= ? AND date <= ?) +
			(SELECT COUNT(*) FROM browsing_daily WHERE date >= ? AND date <= ?)
	`, from, to, from, to, from, to).Scan(&count)
	return count, err
}

func quoteSQLitePath(path string) string {
	return "'" + strings.ReplaceAll(filepath.ToSlash(path), "'", "''") + "'"
}

func sqliteFilePath(path string) string {
	return "file:" + filepath.ToSlash(path)
}

// writeWeekArchive snapshots the current database, trims the copy to one week
// and atomically publishes it. The main database keeps the same rows; the
// archive is an independent, long-term copy. A temporary file means a crash
// can never leave a half-written archive under the final name.
func writeWeekArchive(weekStart, weekEnd time.Time, dest string) error {
	from := dateString(weekStart)
	to := dateString(weekEnd)

	tmp := dest + ".tmp"
	_ = os.Remove(tmp)

	if _, err := db.Exec("VACUUM INTO " + quoteSQLitePath(tmp)); err != nil {
		return fmt.Errorf("snapshot database: %w", err)
	}

	arch, err := sql.Open("sqlite", sqliteFilePath(tmp)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("open archive: %w", err)
	}
	closeArchive := func() {
		_ = arch.Close()
	}

	// The archive only needs tracking rows. Config and the live session belong
	// to the working database, not to a historical record.
	for _, table := range []string{"sessions", "apps_daily", "browsing_daily"} {
		if _, err := arch.Exec("DELETE FROM "+table+" WHERE date < ? OR date > ?", from, to); err != nil {
			closeArchive()
			_ = os.Remove(tmp)
			return fmt.Errorf("trim %s: %w", table, err)
		}
	}
	for _, table := range []string{"active_session", "config"} {
		if _, err := arch.Exec("DELETE FROM " + table); err != nil {
			closeArchive()
			_ = os.Remove(tmp)
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	if _, err := arch.Exec(`
		CREATE TABLE IF NOT EXISTS archive_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`); err != nil {
		closeArchive()
		_ = os.Remove(tmp)
		return fmt.Errorf("create archive metadata: %w", err)
	}
	meta := map[string]string{
		"week_start": from,
		"week_end":   to,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"format":     "1",
	}
	for key, value := range meta {
		if _, err := arch.Exec("INSERT OR REPLACE INTO archive_meta (key, value) VALUES (?, ?)", key, value); err != nil {
			closeArchive()
			_ = os.Remove(tmp)
			return fmt.Errorf("write archive metadata: %w", err)
		}
	}

	// VACUUM INTO inherits WAL mode; switch the archive to a single file so no
	// -wal/-shm companion is left behind after the rename.
	if _, err := arch.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		closeArchive()
		_ = os.Remove(tmp)
		return fmt.Errorf("set archive journal mode: %w", err)
	}
	if _, err := arch.Exec("VACUUM"); err != nil {
		closeArchive()
		_ = os.Remove(tmp)
		return fmt.Errorf("compact archive: %w", err)
	}
	closeArchive()

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("publish archive: %w", err)
	}
	return nil
}

// ArchiveCompletedWeeks writes one archive file for every completed ISO week
// that contains tracking rows and does not have an archive yet. The operation is
// idempotent: rerunning it never duplicates or overwrites a finished week.
func ArchiveCompletedWeeks(now time.Time) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if !GetKeepHistory() {
		return 0, nil
	}

	earliest, err := earliestTrackingDate()
	if err != nil {
		return 0, err
	}
	if earliest == "" {
		return 0, nil
	}
	first, err := time.ParseInLocation("2006-01-02", earliest, time.Local)
	if err != nil {
		return 0, fmt.Errorf("parse earliest date: %w", err)
	}

	dir, err := GetArchiveDir()
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return 0, fmt.Errorf("create archive directory: %w", err)
	}

	current := currentWeekStart(now)
	created := 0
	for week := weekStartOf(first); week.Before(current); week = week.AddDate(0, 0, 7) {
		weekEnd := week.AddDate(0, 0, 6)
		rows, err := rowsInRange(dateString(week), dateString(weekEnd))
		if err != nil {
			return created, err
		}
		if rows == 0 {
			continue
		}

		isoYear, isoWeek := week.ISOWeek()
		name := fmt.Sprintf("%s%04d-W%02d%s", archiveFileGlob, isoYear, isoWeek, archiveExtension)
		dest := filepath.Join(dir, name)
		if _, err := os.Stat(dest); err == nil {
			continue
		}

		if err := writeWeekArchive(week, weekEnd, dest); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// MaintainHistory is the maintenance entry point used by the daemon. With
// long-term preservation on, completed weeks are archived and retention never
// deletes anything. With it off, the normal rolling retention applies.
func MaintainHistory() {
	if GetKeepHistory() {
		if _, err := ArchiveCompletedWeeks(time.Now()); err != nil {
			log.Printf("WARN: weekly archive failed: %v", err)
		}
		return
	}
	EnforceRetention()
}

// ListArchiveFiles returns archive paths sorted oldest to newest. Used by
// diagnostics and tests; the settings UI only needs the count.
func ListArchiveFiles() ([]string, error) {
	dir, err := GetArchiveDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, archiveFileGlob) || !strings.HasSuffix(name, archiveExtension) {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}
