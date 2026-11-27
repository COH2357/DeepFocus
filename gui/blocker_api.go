package gui

import (
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"net/http"
	"strings"
	"time"
	"unsafe"
)

const blockBypassMinutes = 10

type blockInfoData struct {
	Kind           string   `json:"kind"`
	Target         string   `json:"target"`
	Label          string   `json:"label"`
	LimitMinutes   int      `json:"limitMinutes"`
	UsedSeconds    int      `json:"usedSeconds"`
	BypassMinutes  int      `json:"bypassMinutes"`
	Quote          string   `json:"quote"`
	QuoteCount     int      `json:"quoteCount"`
	ProcessRunning bool     `json:"processRunning"`
	Mode           string   `json:"mode"`
	Unit           string   `json:"unit"`
	MaxOpens       int      `json:"maxOpens"`
	Quotes         []string `json:"quotes"`
}

func (a *app) blockInfo(w http.ResponseWriter, r *http.Request) error {
	quote, err := storage.RandomBlockQuote()
	if err != nil {
		return err
	}
	quotes, _ := storage.ListBlockQuotes()
	texts := make([]string, 0, len(quotes))
	for _, q := range quotes {
		texts = append(texts, q.Text)
	}
	return writeJSON(w, blockInfoData{
		Kind:           a.block.Kind,
		Target:         a.block.Target,
		Label:          a.block.Label,
		LimitMinutes:   a.block.LimitMinutes,
		UsedSeconds:    a.block.UsedSeconds,
		BypassMinutes:  blockBypassMinutes,
		Quote:          quote,
		QuoteCount:     len(quotes),
		ProcessRunning: system.TargetAlive(a.block.HWND, a.block.PID),
		Mode:           a.block.Kind,
		Unit:           a.block.Unit,
		MaxOpens:       a.block.MaxOpens,
		Quotes:         texts,
	})
}

type blockBypassRequest struct {
	Reason  string `json:"reason"`
	Minutes int    `json:"minutes"`
}

func (a *app) blockBypass(w http.ResponseWriter, r *http.Request) error {
	var req blockBypassRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return badRequest("a reason is required")
	}
	minutes := req.Minutes
	if minutes <= 0 {
		minutes = blockBypassMinutes
	}
	if minutes < 1 || minutes > 1440 {
		return badRequest("allow duration must be between 1 and 1440 minutes")
	}
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	if err := storage.SetBlockBypass(a.block.Kind, a.block.Target, until, reason); err != nil {
		return err
	}
	if err := storage.RecordBlockEvent(a.block.Kind, a.block.Label, "bypassed", a.block.LimitMinutes, a.block.UsedSeconds, reason); err != nil {
		return err
	}
	system.RestoreWindow(a.block.HWND)
	if err := writeJSON(w, map[string]interface{}{"ok": true, "until": until.Unix()}); err != nil {
		return err
	}
	closeBlockWindowAfter(250 * time.Millisecond)
	return nil
}

func (a *app) blockClose(w http.ResponseWriter, r *http.Request) error {
	if err := writeJSON(w, map[string]interface{}{"ok": true}); err != nil {
		return err
	}
	closeBlockWindowAfter(120 * time.Millisecond)
	return nil
}

func closeBlockWindowAfter(d time.Duration) {
	go func() {
		time.Sleep(d)
		closeBlockWindow()
	}()
}

type blockPoint struct {
	x, y int32
}

// blockDragStart tracks the physical cursor while the mouse button is held and
// moves the popup by the same delta. This is more reliable than WM_NCLBUTTONDOWN
// for a borderless WS_POPUP window, and it keeps tracking even when Chromium
// loses pointer capture because the window is moving underneath the cursor.
func (a *app) blockDragStart(w http.ResponseWriter, r *http.Request) error {
	if blockWindowHWND == 0 {
		return badRequest("blocker window is not ready")
	}
	if !blockDragging.CompareAndSwap(false, true) {
		return writeJSON(w, map[string]interface{}{"ok": true})
	}
	hwnd := blockWindowHWND
	go func() {
		defer blockDragging.Store(false)
		var start blockRect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&start)))
		var cursor blockPoint
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
		startX, startY := cursor.x, cursor.y
		for {
			var now blockPoint
			procGetCursorPos.Call(uintptr(unsafe.Pointer(&now)))
			dx := now.x - startX
			dy := now.y - startY
			procSetWindowPos.Call(hwnd, ^uintptr(0),
				uintptr(int(start.left)+int(dx)),
				uintptr(int(start.top)+int(dy)),
				0, 0, 0x0001|0x0040)
			key, _, _ := procGetAsyncKeyState.Call(0x01) // VK_LBUTTON
			if uint16(key)&0x8000 == 0 {
				return
			}
			time.Sleep(8 * time.Millisecond)
		}
	}()
	return writeJSON(w, map[string]interface{}{"ok": true})
}

// blockLeave closes every tab of the blocked website, then restores the
// browser. It is different from a bypass: no extra allowance is granted, so
// re-opening the site will trigger the limit again. Closing only the active
// tab would leave other tabs of the same site in front and make the guard
// minimize the browser again as soon as it is opened.
func (a *app) blockLeave(w http.ResponseWriter, r *http.Request) error {
	target := a.block.Target
	// Keep the guard quiet while the browser is being restored and its tabs
	// are being closed; otherwise it minimizes the browser mid-operation.
	_ = storage.SetBlockHold("site", target, time.Now().Add(45*time.Second))
	defer storage.ClearBlockHold("site", target)

	// Hide the blocker before the sweep, then force the browser foreground in
	// CloseBrowserTabs. The force path attaches to the input queues, which is
	// required because simply calling SetForegroundWindow after hiding is
	// rejected by Windows and leaves Task Manager or Task View in front.
	hideBlockWindow()
	time.Sleep(100 * time.Millisecond)
	closed := system.CloseBrowserTabs(a.block.HWND, func(title string) bool {
		return coreapi.SiteClassForTitle(title) == target
	})
	if err := writeJSON(w, map[string]interface{}{"ok": true, "closed": closed}); err != nil {
		return err
	}
	closeBlockWindowAfter(350 * time.Millisecond)
	return nil
}
