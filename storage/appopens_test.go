package storage

import (
	"testing"
	"time"
)

func TestAppOpenSessionsAndSiteAllowances(t *testing.T) {
	setupStorageEnv(t)
	now := time.Now()
	day := now.Format("2006-01-02")

	id, err := OpenAppSession(day, "test.exe", "test.exe", now, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := CountAppOpens(day, "test.exe"); got != 1 {
		t.Fatalf("opens = %d; want 1", got)
	}
	active, err := ActiveAppSession("test.exe")
	if err != nil || active == nil || active.ID != id {
		t.Fatalf("active session = %+v, %v", active, err)
	}
	if err := UpdateAppSessionDeadline(id, now.Add(10*time.Minute), 5); err != nil {
		t.Fatal(err)
	}
	if err := CloseAppSession(id, now); err != nil {
		t.Fatal(err)
	}
	if active, _ := ActiveAppSession("test.exe"); active != nil {
		t.Fatal("closed session still active")
	}
	if got := CountAppOpens(day, "test.exe"); got != 1 {
		t.Fatalf("opens after close = %d; want 1", got)
	}

	until := now.Add(time.Minute)
	if err := SetSiteAllowance("bilibili", until, "reason"); err != nil {
		t.Fatal(err)
	}
	if _, ok := SiteAllowanceUntil("bilibili", now); !ok {
		t.Fatal("allowance should be active")
	}
	if _, ok := SiteAllowanceUntil("bilibili", until.Add(time.Second)); ok {
		t.Fatal("allowance should expire")
	}
	if err := SetExtensionSeen("edge", "1.0", now); err != nil {
		t.Fatal(err)
	}
	if !ExtensionSeen("edge", time.Minute) {
		t.Fatal("extension heartbeat should be fresh")
	}
}
