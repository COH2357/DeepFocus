package core

import (
	"testing"
	"time"

	"focusd/storage"
)

// A bypass must be checked before starting a blocker as well as while one is
// active. Otherwise clicking "temporarily allow" closes the blocker, and the
// next guard tick immediately opens it again.
func TestBypassedAppAndSiteDoNotStartBlocker(t *testing.T) {
	setupTestDB(t)

	until := time.Now().Add(time.Minute)
	if err := storage.SetBlockBypass("app", "test.exe", until, "testing"); err != nil {
		t.Fatalf("set app bypass: %v", err)
	}
	g := &blockGuard{}
	g.startAppLocked(violation{kind: "app", target: "test.exe", label: "Test", limit: 1, used: 120})
	if g.active != nil || g.lastCmd != nil {
		t.Fatal("app blocker started during an active bypass")
	}

	if err := storage.SetBlockBypass("site", "bilibili", until, "testing"); err != nil {
		t.Fatalf("set site bypass: %v", err)
	}
	g.startSiteLocked(violation{kind: "site", target: "bilibili", label: "Bilibili", limit: 1, used: 120})
	if g.active != nil || g.lastCmd != nil {
		t.Fatal("site blocker started during an active bypass")
	}
}
