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

// The leave action sets a hold while it restores the browser and closes all
// matching tabs. The guard must not start a blocker or minimize the browser
// during that window.
func TestBlockHoldSuppressesSiteBlocker(t *testing.T) {
	setupTestDB(t)

	if err := storage.SetBlockHold("site", "bilibili", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("set site hold: %v", err)
	}
	g := &blockGuard{}
	g.startSiteLocked(violation{kind: "site", target: "bilibili", label: "Bilibili", limit: 1, used: 120})
	if g.active != nil || g.lastCmd != nil {
		t.Fatal("site blocker started during an active hold")
	}
	if !storage.ActiveBlockHold("site", "bilibili", time.Now()) {
		t.Fatal("hold should still be active")
	}
	if err := storage.ClearBlockHold("site", "bilibili"); err != nil {
		t.Fatalf("clear site hold: %v", err)
	}
	if storage.ActiveBlockHold("site", "bilibili", time.Now()) {
		t.Fatal("hold should be cleared")
	}
}
