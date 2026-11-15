package storage

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func insertTestAppDaily(t *testing.T, date, name, exe string, seconds int) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		VALUES (?, ?, ?, ?, ?, 1)
	`, date, name, exe, name+"|test", seconds)
	if err != nil {
		t.Fatalf("insert apps_daily %s: %v", date, err)
	}
}

func archiveRows(t *testing.T, path, table string) int {
	t.Helper()
	conn, err := sql.Open("sqlite", sqliteFilePath(path))
	if err != nil {
		t.Fatalf("open archive %s: %v", path, err)
	}
	defer conn.Close()
	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("count %s in %s: %v", table, filepath.Base(path), err)
	}
	return count
}

func archiveDates(t *testing.T, path string) []string {
	t.Helper()
	conn, err := sql.Open("sqlite", sqliteFilePath(path))
	if err != nil {
		t.Fatalf("open archive %s: %v", path, err)
	}
	defer conn.Close()
	rows, err := conn.Query("SELECT DISTINCT date FROM apps_daily ORDER BY date")
	if err != nil {
		t.Fatalf("query dates: %v", err)
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			t.Fatal(err)
		}
		dates = append(dates, date)
	}
	return dates
}

func TestKeepHistoryDefaultsOff(t *testing.T) {
	setupStorageEnv(t)
	if GetKeepHistory() {
		t.Fatal("keep history must be off on a fresh database")
	}
}

func TestArchiveCompletedWeeksIsIdempotentAndSplitsWeeks(t *testing.T) {
	setupStorageEnv(t)
	if err := SetKeepHistory(true); err != nil {
		t.Fatal(err)
	}

	insertTestAppDaily(t, "2026-10-27", "Old", "old.exe", 60)       // week 44
	insertTestAppDaily(t, "2026-11-03", "Recent", "recent.exe", 90) // week 45
	insertTestAppDaily(t, "2026-11-10", "Current", "current.exe", 120)

	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.Local) // Thursday, week 46
	created, err := ArchiveCompletedWeeks(now)
	if err != nil {
		t.Fatalf("ArchiveCompletedWeeks: %v", err)
	}
	if created != 2 {
		t.Fatalf("created %d archives, want 2", created)
	}

	files, err := ListArchiveFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d archive files, want 2: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "deepfocus-week-2026-W44.db" || filepath.Base(files[1]) != "deepfocus-week-2026-W45.db" {
		t.Fatalf("unexpected archive names: %v", files)
	}

	dates := archiveDates(t, files[0])
	if len(dates) != 1 || dates[0] != "2026-10-27" {
		t.Fatalf("week 44 archive dates = %v, want [2026-10-27]", dates)
	}
	dates = archiveDates(t, files[1])
	if len(dates) != 1 || dates[0] != "2026-11-03" {
		t.Fatalf("week 45 archive dates = %v, want [2026-11-03]", dates)
	}
	if got := archiveRows(t, files[1], "active_session"); got != 0 {
		t.Fatalf("archive kept %d active_session rows, want 0", got)
	}

	// The working database still has every row; archives are copies, not moves.
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM apps_daily").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("working database has %d rows, want 3", total)
	}

	// Running again must not duplicate, overwrite or add the unfinished week.
	created, err = ArchiveCompletedWeeks(now)
	if err != nil {
		t.Fatalf("second ArchiveCompletedWeeks: %v", err)
	}
	if created != 0 {
		t.Fatalf("second run created %d archives, want 0", created)
	}
	if files, _ = ListArchiveFiles(); len(files) != 2 {
		t.Fatalf("second run left %d archive files, want 2", len(files))
	}
}

func TestArchiveSkipsEmptyWeeks(t *testing.T) {
	setupStorageEnv(t)
	if err := SetKeepHistory(true); err != nil {
		t.Fatal(err)
	}
	insertTestAppDaily(t, "2026-10-20", "Only", "only.exe", 30) // week 43

	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.Local) // week 46
	created, err := ArchiveCompletedWeeks(now)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("created %d archives, want 1 (empty weeks are skipped)", created)
	}
	files, _ := ListArchiveFiles()
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if _, err := os.Stat(files[0]); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
}

func TestMaintainHistorySkipsRetentionWhenEnabled(t *testing.T) {
	setupStorageEnv(t)
	if err := SetRetentionDays(1); err != nil {
		t.Fatal(err)
	}
	if err := SetKeepHistory(true); err != nil {
		t.Fatal(err)
	}
	insertTestAppDaily(t, "2026-10-27", "Old", "old.exe", 60)

	MaintainHistory()

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM apps_daily").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("retention deleted old row despite keep history; rows = %d", total)
	}
	if files, _ := ListArchiveFiles(); len(files) != 1 {
		t.Fatalf("got %d archives, want 1", len(files))
	}
}

func TestBlockQuotesSeedAndLastQuoteGuard(t *testing.T) {
	setupStorageEnv(t)
	quotes, err := ListBlockQuotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) == 0 {
		t.Fatal("default quotes were not seeded")
	}
	text := "测试语录"
	id, err := AddBlockQuote(text)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateBlockQuote(id, text+"更新"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBlockQuote(id); err != nil {
		t.Fatal(err)
	}
	// Delete all but one, then the last deletion must be refused.
	left, _ := ListBlockQuotes()
	for len(left) > 1 {
		if err := DeleteBlockQuote(left[0].ID); err != nil {
			t.Fatal(err)
		}
		left, _ = ListBlockQuotes()
	}
	if err := DeleteBlockQuote(left[0].ID); !errors.Is(err, ErrLastQuote) {
		t.Fatalf("last quote delete = %v; want ErrLastQuote", err)
	}
}
