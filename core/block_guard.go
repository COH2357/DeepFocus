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
	openState map[string]*openLimitState
}

type openLimitState struct {
	sessionID int64
	deadline  time.Time
	day       string
	blocked   bool
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
	unit      string
	maxOpens  int
	startedAt time.Time
}

type violation struct {
	kind     string
	target   string
	label    string
	limit    int
	used     int
	hwnd     uintptr
	pid      uint32
	launch   bool
	unit     string
	maxOpens int
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

	if g.active == nil || g.active.kind == "app" {
		g.enforceOpenLimitsLocked(time.Now())
	}

	// The site blocker sets a short hold while it restores the browser and
	// closes the site's tabs. Without this, the guard would minimize the
	// browser again in the middle of that operation.
	if g.active != nil && g.active.kind == "site" && storage.ActiveBlockHold("site", g.active.target, time.Now()) {
		return
	}

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
	// The blocker may still be closing the site's tabs. Do not reopen the
	// pause page (or minimize the browser) during that short operation.
	if v.kind == "site" && storage.ActiveBlockHold("site", v.target, time.Now()) {
		return
	}
	// A bypass is checked here as well as on the active-block path. Otherwise
	// closing the blocker for a bypass clears active, and the very next tick
	// reopens it while the target is still in front: the open/close loop.
	if storage.ActiveBlockBypass(v.kind, v.target, time.Now()) {
		return
	}
	if v.kind == "site" {
		g.startSiteLocked(v)
		return
	}
	g.startAppLocked(v)
}

func (g *blockGuard) enforceOpenLimitsLocked(now time.Time) {
	limits := system.GetAppOpenLimits()
	if len(limits) == 0 {
		return
	}
	if g.openState == nil {
		g.openState = make(map[string]*openLimitState)
	}
	for key, st := range g.openState {
		if _, ok := limits[key]; !ok {
			if st.sessionID > 0 {
				_ = storage.CloseAppSession(st.sessionID, now)
			}
			delete(g.openState, key)
		}
	}

	day := storage.Today()
	for key, limit := range limits {
		if limit.MaxOpens <= 0 || limit.MinutesPerOpen <= 0 {
			continue
		}
		exe := system.ExeNameForLimitKey(key)
		if exe == "" {
			continue
		}
		st := g.openState[key]
		if st == nil {
			st = &openLimitState{}
			if active, err := storage.ActiveAppSession(key); err == nil && active != nil {
				st.sessionID = active.ID
				st.deadline = time.Unix(active.Deadline, 0)
				st.day = active.Date
			}
			g.openState[key] = st
		}

		running := system.GetProcessCount(exe) > 0
		if !running {
			if st.sessionID > 0 {
				_ = storage.CloseAppSession(st.sessionID, now)
			}
			st.sessionID = 0
			st.deadline = time.Time{}
			st.day = ""
			st.blocked = false
			continue
		}
		if st.sessionID > 0 && (st.day == "" || st.day != day) {
			_ = storage.CloseAppSession(st.sessionID, now)
			st.sessionID = 0
			st.deadline = time.Time{}
			st.blocked = false
		}

		label := system.ResolveDisplayName("", exe, "")
		if st.sessionID == 0 {
			if storage.ActiveBlockBypass("app", key, now) {
				continue
			}
			opens := storage.CountAppOpens(day, key)
			if opens >= limit.MaxOpens {
				st.blocked = true
				if g.active == nil {
					g.startAppLocked(violation{
						kind: "app", target: key, label: label,
						limit: limit.MaxOpens, used: opens,
						pid: system.FirstPIDForExe(exe), unit: "opens",
						maxOpens: limit.MaxOpens,
					})
				}
				continue
			}
			id, err := storage.OpenAppSession(day, key, exe, now, limit.MinutesPerOpen)
			if err != nil {
				continue
			}
			st.sessionID = id
			st.deadline = now.Add(time.Duration(limit.MinutesPerOpen) * time.Minute)
			st.day = day
			st.blocked = false
			continue
		}

		if storage.ActiveBlockBypass("app", key, now) {
			if st.blocked {
				until := now.Add(10 * time.Minute)
				_ = storage.UpdateAppSessionDeadline(st.sessionID, until, limit.MinutesPerOpen)
				st.deadline = until
				st.blocked = false
			}
			continue
		}
		if now.After(st.deadline) && !st.blocked {
			st.blocked = true
			if g.active == nil {
				g.startAppLocked(violation{
					kind: "app", target: key, label: label,
					limit: limit.MinutesPerOpen, used: limit.MinutesPerOpen * 60,
					pid: system.FirstPIDForExe(exe), unit: "minutes",
					maxOpens: limit.MaxOpens,
				})
			}
		}
	}
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
				if err := g.spawn(g.active.kind, g.active.target, g.active.label, g.active.limit, g.active.used, g.active.hwnd, g.active.pid, g.active.unit, g.active.maxOpens); err != nil {
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
			if err := g.spawn(g.active.kind, g.active.target, g.active.label, v.limit, v.used, v.hwnd, v.pid, v.unit, v.maxOpens); err != nil {
				g.nextSpawn = time.Now().Add(3 * time.Second)
			}
		}
	}
}

func (g *blockGuard) startAppLocked(v violation) {
	if time.Now().Before(g.nextSpawn) {
		return
	}
	if storage.ActiveBlockBypass(v.kind, v.target, time.Now()) {
		return
	}
	if !storage.HasBlockEventToday(v.kind, v.label) {
		_ = storage.RecordBlockEvent(v.kind, v.label, "blocked", v.limit, v.used, "")
	}
	if v.launch {
		system.MinimizeWindow(v.hwnd)
	}
	unit := v.unit
	if unit == "" {
		unit = "minutes"
	}
	if err := g.spawn(v.kind, v.target, v.label, v.limit, v.used, v.hwnd, v.pid, unit, v.maxOpens); err != nil {
		log.Printf("WARN: could not open blocker: %v", err)
		g.nextSpawn = time.Now().Add(3 * time.Second)
		return
	}
	g.active = &activeBlock{kind: v.kind, target: v.target, label: v.label, cmd: g.lastCmd, pid: v.pid, limit: v.limit, used: v.used, hwnd: v.hwnd, minimize: v.launch, unit: unit, maxOpens: v.maxOpens}
}

func (g *blockGuard) startSiteLocked(v violation) {
	if time.Now().Before(g.nextSpawn) {
		return
	}
	if storage.ActiveBlockHold("site", v.target, time.Now()) {
		return
	}
	if storage.ActiveBlockBypass(v.kind, v.target, time.Now()) {
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
	if err := g.spawn(v.kind, v.target, v.label, v.limit, v.used, v.hwnd, v.pid, "minutes", 0); err != nil {
		log.Printf("WARN: could not open blocker: %v", err)
		g.nextSpawn = time.Now().Add(3 * time.Second)
		return
	}
	g.active = &activeBlock{kind: v.kind, target: v.target, label: v.label, cmd: g.lastCmd, pid: v.pid, limit: v.limit, used: v.used, hwnd: v.hwnd, unit: "minutes"}
}

func extensionBrowserForExe(exe string) string {
	switch strings.ToLower(exe) {
	case "msedge.exe":
		return "edge"
	case "firefox.exe":
		return "firefox"
	case "brave.exe":
		return "brave"
	case "chrome.exe", "chromium.exe", "thorium.exe":
		return "chrome"
	}
	return ""
}

func extensionHandlesBrowser(exe string) bool {
	browser := extensionBrowserForExe(exe)
	return browser != "" && storage.ExtensionSeen(browser, 90*time.Second)
}

func (g *blockGuard) warnNearLimit(fg *system.WindowInfo) {

	if fg == nil || system.IsWhitelisted(fg.ExeName) {
		return
	}
	if system.IsBrowser(fg.ExeName) && !extensionHandlesBrowser(fg.ExeName) {
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

func (g *blockGuard) spawn(kind, target, label string, limit, used int, hwnd uintptr, pid uint32, unit string, maxOpens int) error {
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
		"--unit", unit,
		"--max-opens", strconv.Itoa(maxOpens),
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

	if system.IsBrowser(fg.ExeName) && !extensionHandlesBrowser(fg.ExeName) {
		class := coreapi.SiteClassForTitle(fg.Title)
		if class != "" {
			if minutes, ok := system.GetSiteTimeLimits()[class]; ok && minutes > 0 {
				used := coreapi.SiteUsageTodaySeconds(class)
				if used >= minutes*60 {
					return violation{kind: "site", target: class, label: coreapi.SiteClassDisplay(class), limit: minutes, used: used, hwnd: fg.HWND, pid: fg.PID, unit: "minutes"}, true
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
				return violation{kind: "app", target: key, label: label, limit: minutes, used: used, hwnd: fg.HWND, pid: fg.PID, launch: launch, unit: "minutes"}, true
			}
		}
	}
	return violation{}, false
}
