//go:build windows

package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32dll = windows.NewLazySystemDLL("user32.dll")
	dwmapi    = windows.NewLazySystemDLL("dwmapi.dll")

	procSetProcessDpiAwarenessContext = user32dll.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForSystem               = user32dll.NewProc("GetDpiForSystem")
	procSetProcessDPIAware            = user32dll.NewProc("SetProcessDPIAware")
	procMessageBoxW                   = user32dll.NewProc("MessageBoxW")
	procSystemParametersInfoW         = user32dll.NewProc("SystemParametersInfoW")
	procSetWindowPos                  = user32dll.NewProc("SetWindowPos")
	procFindWindowW                   = user32dll.NewProc("FindWindowW")
	procShowWindowW                   = user32dll.NewProc("ShowWindow")
	procSetForegroundWindowW          = user32dll.NewProc("SetForegroundWindow")
	procDwmSetWindowAttribute         = dwmapi.NewProc("DwmSetWindowAttribute")
	procGetWindowLongPtrW             = user32dll.NewProc("GetWindowLongPtrW")
	procSendMessageW                  = user32dll.NewProc("SendMessageW")
	procReleaseCapture                = user32dll.NewProc("ReleaseCapture")
	procGetWindowRect                 = user32dll.NewProc("GetWindowRect")
	procGetAsyncKeyState              = user32dll.NewProc("GetAsyncKeyState")
)

const (
	swRestoreWindow  = 9
	swMaximizeWindow = 3
)

const (
	wmSysCommand    = 0x0112
	scMinimize      = 0xf020
	scClose         = 0xf060
	scMove          = 0xf010
	gwlStyle        = ^uintptr(15) // -16
	gwlExStyle      = ^uintptr(19) // -20
	gwlpWndProc     = ^uintptr(3)  // -4
	wsPopup         = 0x80000000
	wsVisible       = 0x10000000
	wsCaption       = 0x00c00000
	wsThickFrame    = 0x00040000
	wsMinimizeBox   = 0x00020000
	wsMaximizeBox   = 0x00010000
	wsSysMenu       = 0x00080000
	wsExToolWin     = 0x00000080
	swpFrameChanged = 0x0020
	wmNCLButtonDown = 0x00a1
	htCaption       = 2
)

const (
	dwmwaUseImmersiveDarkMode    = 20
	dwmwaUseImmersiveDarkModeOld = 19
	dwmwaCaptionColor            = 35
	dwmwaBorderColor             = 34
	dwmwaTextColor               = 36
	dwmwaWindowCorner            = 33

	dwmcpRound = 2

	spiGetWorkArea = 0x0030
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
)

var activeWindow uintptr

// openWindow renders the UI in a native WebView2 window. If the runtime is
// missing we fall back to the system browser instead of failing outright.
func openWindow(url string, a *app) (err error) {
	// go-webview2 dereferences a nil controller when WebView2's process
	// creation is denied (for example by a restricted job object or a broken
	// runtime install). Recover so the user still gets the web UI instead of
	// a crash dialog.
	defer func() {
		if r := recover(); r != nil {
			openInBrowser(url)
			showMessage("DeepFocus",
				"The embedded web view could not start on this system,\n"+
					"so DeepFocus is running in your default browser.")
			err = errBrowserFallback
		}
	}()
	if a != nil && a.debug {
		fmt.Println(url)
	}
	if hwnd := existingWindow(); hwnd != 0 {
		procShowWindowW.Call(hwnd, swRestoreWindow)
		procSetForegroundWindowW.Call(hwnd)
		return errAlreadyRunning
	}
	enableDPIAwareness()
	scale := float64(systemDPI()) / 96.0
	width, height, minWidth, minHeight := windowSize(scale)

	view := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  webviewDataPath(),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "DeepFocus",
			Width:  uint(width),
			Height: uint(height),
			IconId: 1,
			Center: true,
		},
	})
	if view == nil {
		openInBrowser(url)
		showMessage("DeepFocus",
			"The Microsoft Edge WebView2 runtime is not available on this system,\n"+
				"so DeepFocus is running in your default browser.")
		return errBrowserFallback
	}
	defer view.Destroy()
	defer removeTray()

	hwnd := uintptr(view.Window())
	activeWindow = hwnd
	placeWindow(hwnd, width, height)
	go func() {
		time.Sleep(250 * time.Millisecond)
		view.Dispatch(func() {
			if !installTray(hwnd) && a != nil && a.debug {
				fmt.Fprintln(os.Stderr, "tray icon unavailable:", trayDebug)
			}
		})
	}()
	view.SetSize(minWidth, minHeight, webview2.HintMin)
	styleWindow(hwnd, systemUsesDarkTheme())
	view.Navigate(url + "#dashboard")
	if a != nil && a.debug {
		go func() {
			time.Sleep(3 * time.Second)
			view.Dispatch(func() {
				view.Eval("document.title = 'DF ' + innerWidth + ' / ' + document.documentElement.scrollWidth + ' / ' + document.body.scrollWidth")
			})
		}()
	}
	view.Run()
	activeWindow = 0
	return nil
}

// windowSize keeps the first window comfortably inside the monitor work area.
// A fixed 1440x900 scaled by DPI can overflow smaller or scaled displays,
// which puts the title bar above the screen and clips the dashboard.
func windowSize(scale float64) (width, height, minWidth, minHeight int) {
	w := int(960 * scale)
	h := int(640 * scale)
	minWidth = int(780 * scale)
	minHeight = int(500 * scale)
	if area, ok := workArea(); ok {
		maxW := int(area.right - area.left)
		maxH := int(area.bottom - area.top)
		limitW := maxW * 9 / 10
		limitH := maxH * 84 / 100
		if limitW > 0 && w > limitW {
			w = limitW
		}
		if limitH > 0 && h > limitH {
			h = limitH
		}
	}
	if w > 2200 {
		w = 2200
	}
	if h > 1400 {
		h = 1400
	}
	if minWidth > w {
		minWidth = w
	}
	if minHeight > h {
		minHeight = h
	}
	return w, h, minWidth, minHeight
}

// existingWindow returns an already-running DeepFocus window. Reopening the
// shortcut while the app is hidden in the notification area should reveal the
// existing window instead of starting a second copy.
func existingWindow() uintptr {
	class, _ := windows.UTF16PtrFromString("webview")
	title, _ := windows.UTF16PtrFromString("DeepFocus")
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)))
	return hwnd
}

// placeWindow centers the frame in the work area (not the full screen), so
// the minimize/maximize/close buttons are always reachable.
func placeWindow(hwnd uintptr, width, height int) {
	area, ok := workArea()
	if !ok || hwnd == 0 {
		return
	}
	aw := int(area.right - area.left)
	ah := int(area.bottom - area.top)
	x := int(area.left) + (aw-width)/2
	y := int(area.top) + (ah-height)/2
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), swpNoZOrder|swpNoActivate)
}

type winRect struct {
	left, top, right, bottom int32
}

func workArea() (winRect, bool) {
	var r winRect
	ok, _, _ := procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0)
	if ok == 0 || r.right <= r.left || r.bottom <= r.top {
		return winRect{}, false
	}
	return r, true
}

func openInBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func webviewDataPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return ""
	}
	path := filepath.Join(base, "focusd", "webview2")
	if err := os.MkdirAll(path, 0755); err != nil {
		return ""
	}
	return path
}

func enableDPIAwareness() {
	// PER_MONITOR_AWARE_V2 is -4; the conversion is deliberate.
	_, _, _ = procSetProcessDpiAwarenessContext.Call(^uintptr(3))
	_, _, _ = procSetProcessDPIAware.Call()
}

func systemDPI() int {
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		return 96
	}
	return int(dpi)
}

// styleWindow blends the native frame with the app's paper/ink palette and
// opts into Windows 11 rounded corners where available.
func styleWindow(hwnd uintptr, dark bool) {
	applyTitleBarTheme(dark)
	pref := int32(dwmcpRound)
	_, _, _ = procDwmSetWindowAttribute.Call(hwnd, dwmwaWindowCorner,
		uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
}

// applyTitleBarTheme can be called later when the user switches themes.
func applyTitleBarTheme(dark bool) {
	if activeWindow == 0 {
		return
	}
	var enabled int32
	if dark {
		enabled = 1
	}
	_, _, _ = procDwmSetWindowAttribute.Call(activeWindow, dwmwaUseImmersiveDarkMode,
		uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))
	_, _, _ = procDwmSetWindowAttribute.Call(activeWindow, dwmwaUseImmersiveDarkModeOld,
		uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))

	var caption, text uint32
	if dark {
		caption = rgb(14, 14, 14)
		text = rgb(236, 232, 222)
	} else {
		caption = rgb(240, 237, 228)
		text = rgb(14, 14, 14)
	}
	_, _, _ = procDwmSetWindowAttribute.Call(activeWindow, dwmwaCaptionColor,
		uintptr(unsafe.Pointer(&caption)), unsafe.Sizeof(caption))
	_, _, _ = procDwmSetWindowAttribute.Call(activeWindow, dwmwaBorderColor,
		uintptr(unsafe.Pointer(&caption)), unsafe.Sizeof(caption))
	_, _, _ = procDwmSetWindowAttribute.Call(activeWindow, dwmwaTextColor,
		uintptr(unsafe.Pointer(&text)), unsafe.Sizeof(text))
}

// rgb builds a COLORREF (0x00BBGGRR).
func rgb(r, g, b uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func systemUsesDarkTheme() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return value == 0
}

func showMessage(title, body string) {
	t, _ := windows.UTF16PtrFromString(title)
	b, _ := windows.UTF16PtrFromString(body)
	_, _, _ = procMessageBoxW.Call(0, uintptr(unsafe.Pointer(b)),
		uintptr(unsafe.Pointer(t)), 0x00000040)
}

// ShowFatal reports a startup failure through a modal dialog, because the GUI
// binary has no console attached.
func ShowFatal(err error) {
	showMessage("DeepFocus", "DeepFocus could not start:\n\n"+err.Error())
}

// setTitleBarTheme keeps the native frame in sync with the in-app theme.
func setTitleBarTheme(dark bool) {
	applyTitleBarTheme(dark)
}

type blockRect struct {
	left, top, right, bottom int32
}

var (
	blockWindowHWND uintptr
	blockOldProc    uintptr
	blockCanClose   atomic.Bool
	blockDragging   atomic.Bool
)

func blockerWindowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmClose:
		if !blockCanClose.Load() {
			return 0
		}
	case wmSysCommand:
		cmd := wParam & 0xfff0
		if cmd == scMinimize || cmd == scClose {
			return 0
		}
	}
	return callWindowProc(blockOldProc, hwnd, msg, wParam, lParam)
}

func removeBlockerFrame(hwnd uintptr) {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	style &^= wsCaption | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu
	style |= wsPopup | wsVisible
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, style)
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, ex|wsExToolWin)
	procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002|0x0040|swpFrameChanged)
}

// openBlockerWindow is deliberately separate from the main window: it is
// topmost, fills the work area, and never calls the single-instance check.
func openBlockerWindow(url string, a *app) (err error) {
	defer func() {
		if r := recover(); r != nil {
			showMessage("DeepFocus", "DeepFocus could not open its blocker window.")
			err = errNoWindow
		}
	}()
	enableDPIAwareness()
	scale := float64(systemDPI()) / 96.0
	width, height := 0, 0
	compact := a != nil && a.block != nil && a.block.Kind == "app"
	if compact {
		width = int(560 * scale)
		height = int(700 * scale)
	} else if area, ok := workArea(); ok {
		width = int(area.right - area.left)
		height = int(area.bottom - area.top)
	} else {
		width, height = int(1120*scale), int(760*scale)
	}
	title := "DeepFocus · 停一下"
	view := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  webviewDataPath(),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  title,
			Width:  uint(width),
			Height: uint(height),
			IconId: 1,
			Center: true,
		},
	})
	if view == nil {
		return errNoWindow
	}
	defer view.Destroy()
	hwnd := uintptr(view.Window())
	blockWindowHWND = hwnd
	blockCanClose.Store(false)
	defer func() {
		blockWindowHWND = 0
		blockOldProc = 0
	}()
	blockOldProc, _, _ = procSetWindowLongPtrW.Call(hwnd, gwlpWndProc, windows.NewCallback(blockerWindowProc))
	removeBlockerFrame(hwnd)
	styleWindow(hwnd, false)
	view.SetSize(440, 560, webview2.HintMin)
	if compact {
		x := 24
		y := 24
		w := int(560 * scale)
		h := int(700 * scale)
		if area, ok := workArea(); ok {
			x = int(area.right) - w - 18
			y = int(area.top) + 18
			if x < 0 {
				x = 18
			}
		}
		procSetWindowPos.Call(hwnd, ^uintptr(0), uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0x0040)
	} else {
		procShowWindowW.Call(hwnd, swMaximizeWindow)
		procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002|0x0040)
	}
	view.Navigate(url)
	view.Run()
	return nil
}

func hideBlockWindow() {
	if blockWindowHWND != 0 {
		procShowWindowW.Call(blockWindowHWND, 0)
	}
}

func closeBlockWindow() {
	if blockWindowHWND != 0 {
		blockCanClose.Store(true)
		procPostMessageW.Call(blockWindowHWND, wmClose, 0, 0)
	}
}
