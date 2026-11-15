package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
)

// blockGuard turns a crossed limit into friction without terminating anything.
//
//   - app limits open a small, always-on-top reminder. The app keeps running so
//     unsaved work is never lost; the popup waits until the process exits and
//     offers a "already closed" button that only enables once the process is
//     actually gone.
//   - site limits notify first, then minimize the browser and open the larger
//     blocker page after a short delay. Restoring the browser on the blocked
//     site brings it straight back.
type blockGuard struct {
	mu        sync.Mutex
	active    *activeBlock
	nextSpawn time.Time
	lastCmd   *exec.Cmd
	warned    map[string]bool
	pending   map[string]time.Time
	day       string
}

type activeBlock struct {
	kind      string
	target    string
	label     string
	cmd       *exec.Cmd
	pid       uint32
	limit     int
	used      int
	hwnd      uintptr
	minimize  bool
	startedAt time.Time
}

type violation struct {
	kind   string
	target string
	label  string
	limit  int
	used   int
	hwnd   uintptr
	pid    uint32
	launch bool
}

func (g *blockGuard) stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closeLocked()
}

func (g *blockGuard) closeLocked() {
	if g.active != nil && g.active.cmd != nil && g.active.cmd.Process != nil {
		_ = g.active.cmd.Process.Kill()
	}
	g.active = nil
}

func (g *blockGuard) resetDay(now time.Time) {
	today := now.Format("2006-01-02")
	if g.day != today {
		g.day = today
		g.warned = make(map[string]bool)
		g.pending = make(map[string]time.Time)
	}
}

func (g *blockGuard) enforce() {
	if storage.IsPaused() {
		g.mu.Lock()
		g.closeLocked()
		g.mu.Unlock()
		return
	}

	fg, err := system.GetForegroundWindowInfo()
	if err != nil {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.resetDay(time.Now())

	if g.active != nil && storage.ActiveBlockBypass(g.active.kind, g.active.target, time.Now()) {
		g.closeLocked()
	}

	if g.active != nil {
		if g.active.kind == "app" {
			g.enforceAppLocked(fg)
			return
		}
		g.enforceSiteLocked(fg)
		return
	}

	if fg == nil || strings.EqualFold(fg.ExeName, "deepfocus_gui.exe") {
		return
	}

	g.warnNearLimit(fg)

	v, ok := g.violationFor(fg)
	if !ok {
		return
	}
	if v.kind == "site" {
		g.startSiteLocked(v)
		return
	}
	g.startAppLocked(v)
}

func (g *blockGuard) enforceAppLocked(fg *system.WindowInfo) {
	if g.active == nil {
		return
	}
	if g.active.minimize && fg != nil && fg.PID == g.active.pid {
		system.MinimizeWindow(fg.HWND)
	}
	// The popup stays while the app is running. If it was closed by the user,
	// bring it back after a short backoff.
	if system.TargetAlive(g.active.hwnd, g.active.pid) {
		if g.active.cmd == nil || !system.ProcessRunning(uint32(g.active.cmd.Process.Pid)) {
			if time.Now().After(g.nextSpawn) {
				if err := g.spawn(g.active.kind, g.active.target, g.active.label, g.active.limit, g.active.used, g.active.hwnd, g.active.pid); err != nil {
					g.nextSpawn = time.Now().Add(3 * time.Second)
				}
			}
		}
		return
	}
	// Process is gone. Keep the popup open until the page's "already closed"
	// button is used, but never respawn it.
	if g.active.cmd == nil || !system.ProcessRunning(uint32(g.active.cmd.Process.Pid)) {
		g.active = nil
	}
}

func (g *blockGuard) enforceSiteLocked(fg *system.WindowInfo) {
	if g.active == nil {
		return
	}
	if fg == nil || strings.EqualFold(fg.ExeName, "deepfocus_gui.exe") {
		return
	}
	v, ok := g.violationFor(fg)
	if !ok || v.kind != "site" || v.target != g.active.target {
		g.closeLocked()
		return
	}
	system.MinimizeWindow(v.hwnd)
	if g.active.cmd == nil || !system.ProcessRunning(uint32(g.active.cmd.Process.Pid)) {
		if time.Now().After(g.nextSpawn) {
			if err := g.spawn(g.active.kind, g.active.target, g.active.label, v.limit, v.used, v.hwnd, v.pid); err != nil {
				g.nextSpawn = time.Now().Add(3 * time.Second)
			}
		}
	}
}

func (g *blockGuard) startAppLocked(v violation) {
	if time.Now().Before(g.nextSpawn) {
		return
	}
	if !storage.HasBlockEventToday(v.kind, v.label) {
		_ = storage.RecordBlockEvent(v.kind, v.label, "blocked", v.limit, v.used, "")
	}
	if v.launch {
		system.MinimizeWindow(v.hwnd)
	}
	if err := g.spawn(v.kind, v.target, v.label, v.limit, v.used, v.hwnd, v.pid); err != nil {
		log.Printf("WARN: could not open blocker: %v", err)
		g.nextSpawn = time.Now().Add(3 * time.Second)
		return
	}
	g.active = &activeBlock{kind: v.kind, target: v.target, label: v.label, cmd: g.lastCmd, pid: v.pid, limit: v.limit, used: v.used, hwnd: v.hwnd, minimize: v.launch}
}

func (g *blockGuard) startSiteLocked(v violation) {
	if time.Now().Before(g.nextSpawn) {
		return
	}
	key := "site:" + v.target
	if _, pending := g.pending[key]; !pending {
		showNotification(notifyContext, tr("Website limit reached", "网站时限已到"), fmt.Sprintf(tr("%s has used today's allowance. The reminder page opens in a moment.", "%s 今天的使用额度已经用完，几秒后会切到提醒页。"), v.label))
		g.pending[key] = time.Now().Add(2 * time.Second)
		return
	}
	if time.Now().Before(g.pending[key]) {
		return
	}
	delete(g.pending, key)
	if !storage.HasBlockEventToday(v.kind, v.label) {
		_ = storage.RecordBlockEvent(v.kind, v.label, "blocked", v.limit, v.used, "")
	}
	system.MinimizeWindow(v.hwnd)
	if err := g.spawn(v.kind, v.target, v.label, v.limit, v.used, v.hwnd, v.pid); err != nil {
		log.Printf("WARN: could not open blocker: %v", err)
		g.nextSpawn = time.Now().Add(3 * time.Second)
		return
	}
	g.active = &activeBlock{kind: v.kind, target: v.target, label: v.label, cmd: g.lastCmd, pid: v.pid, limit: v.limit, used: v.used, hwnd: v.hwnd}
}

func (g *blockGuard) warnNearLimit(fg *system.WindowInfo) {
	if fg == nil || system.IsWhitelisted(fg.ExeName) {
		return
	}
	if system.IsBrowser(fg.ExeName) {
		class := coreapi.SiteClassForTitle(fg.Title)
		if minutes, ok := system.GetSiteTimeLimits()[class]; ok && minutes > 0 {
			used := coreapi.SiteUsageTodaySeconds(class)
			g.warnIfTwenty("site:"+class, used, minutes, coreapi.SiteClassDisplay(class))
		}
	}
	if key, ok := system.MatchLimitKey(system.GetAppTimeLimits(), fg.AppID, fg.ExeName); ok {
		minutes := system.GetAppTimeLimits()[key]
		if minutes > 0 {
			used := storage.GetAppUsageTodaySecondsForConfig(key)
			g.warnIfTwenty("app:"+key, used, minutes, system.ResolveDisplayName(fg.AppID, fg.ExeName, ""))
		}
	}
}

func (g *blockGuard) warnIfTwenty(key string, used, limit int, label string) {
	if limit <= 0 || used >= limit*60 || used*5 < limit*60*4 {
		return
	}
	if g.warned[key+":20"] {
		return
	}
	g.warned[key+":20"] = true
	remaining := limit*60 - used
	if remaining < 0 {
		remaining = 0
	}
	showNotification(notifyContext, tr("Time allowance", "使用额度提醒"), fmt.Sprintf(tr("%s has %d minutes left.", "%s 还剩 %d 分钟。"), label, (remaining+59)/60))
}

func (g *blockGuard) spawn(kind, target, label string, limit, used int, hwnd uintptr, pid uint32) error {
	gui := guiExecutablePath()
	if gui == "" {
		return os.ErrNotExist
	}
	cmd := exec.Command(gui,
		"--block",
		"--kind", kind,
		"--target", target,
		"--label", label,
		"--limit", strconv.Itoa(limit),
		"--used", strconv.Itoa(used),
		"--hwnd", strconv.FormatUint(uint64(hwnd), 10),
		"--pid", strconv.FormatUint(uint64(pid), 10),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Start(); err != nil {
		return err
	}
	g.lastCmd = cmd
	go func() { _ = cmd.Wait() }()
	return nil
}

func guiExecutablePath() string {
	if self, err := os.Executable(); err == nil {
		dir := filepath.Dir(self)
		for _, name := range []string{"deepfocus_gui.exe", "focusd_gui.exe"} {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	dir := system.GetInstallDir()
	for _, name := range []string{"deepfocus_gui.exe", "focusd_gui.exe"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (g *blockGuard) violationFor(fg *system.WindowInfo) (violation, bool) {
	if fg == nil || system.IsWhitelisted(fg.ExeName) {
		return violation{}, false
	}

	if system.IsBrowser(fg.ExeName) {
		class := coreapi.SiteClassForTitle(fg.Title)
		if class != "" {
			if minutes, ok := system.GetSiteTimeLimits()[class]; ok && minutes > 0 {
				used := coreapi.SiteUsageTodaySeconds(class)
				if used >= minutes*60 {
					return violation{"site", class, coreapi.SiteClassDisplay(class), minutes, used, fg.HWND, fg.PID, false}, true
				}
			}
		}
	}

	if key, ok := system.MatchLimitKey(system.GetAppTimeLimits(), fg.AppID, fg.ExeName); ok {
		limits := system.GetAppTimeLimits()
		minutes := limits[key]
		if minutes > 0 {
			used := storage.GetAppUsageTodaySecondsForConfig(key)
			if used >= minutes*60 {
				label := system.ResolveDisplayName(fg.AppID, fg.ExeName, "")
				age := system.ProcessAge(fg.PID)
				launch := age > 0 && age < 20*time.Second
				return violation{"app", key, label, minutes, used, fg.HWND, fg.PID, launch}, true
			}
		}
	}
	return violation{}, false
}
