//go:build windows

package gui

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"focusd/system"
)

// A small, dependency-free notification-area icon. It uses a private
// message-only window for tray callbacks and subclasses the WebView2 window
// only to intercept the close button.
const (
	wmDestroy      = 0x0002
	wmClose        = 0x0010
	wmNull         = 0x0000
	wmApp          = 0x8000
	trayCallback   = wmApp + 1
	wmLButtonDbl   = 0x0203
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	wmContextMenu  = 0x007B
	swHide         = 0
	swRestore      = 9
	nimAdd         = 0
	nimDelete      = 2
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	imageIcon      = 1
	lrDefaultSize  = 0x0040
	lrShared       = 0x8000
	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	trayOpenCmd    = 1
	trayQuitCmd    = 2
)

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

type point struct {
	x, y int32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

var (
	shell32               = windows.NewLazySystemDLL("shell32.dll")
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procShellNotifyIconW  = shell32.NewProc("Shell_NotifyIconW")
	procRegisterClassExW  = user32dll.NewProc("RegisterClassExW")
	procCreateWindowExW   = user32dll.NewProc("CreateWindowExW")
	procDestroyWindow     = user32dll.NewProc("DestroyWindow")
	procCreatePopupMenu   = user32dll.NewProc("CreatePopupMenu")
	procAppendMenuW       = user32dll.NewProc("AppendMenuW")
	procDestroyMenu       = user32dll.NewProc("DestroyMenu")
	procTrackPopupMenu    = user32dll.NewProc("TrackPopupMenu")
	procSetForegroundWin  = user32dll.NewProc("SetForegroundWindow")
	procGetCursorPos      = user32dll.NewProc("GetCursorPos")
	procLoadImageW        = user32dll.NewProc("LoadImageW")
	procLoadIconW         = user32dll.NewProc("LoadIconW")
	procSetWindowLongPtrW = user32dll.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32dll.NewProc("CallWindowProcW")
	procShowWindow        = user32dll.NewProc("ShowWindow")
	procPostMessageW      = user32dll.NewProc("PostMessageW")
	procGetModuleHandleW  = kernel32.NewProc("GetModuleHandleW")

	mainHWND     uintptr
	mainOldProc  uintptr
	trayHWND     uintptr
	trayReady    bool
	trayQuitting bool
	trayDebug    string
)

// installTray adds the notification-area icon and asks the WebView2 window to
// hide instead of quit on close. Returning false leaves normal close behaviour.
func installTray(hwnd uintptr) bool {
	if hwnd == 0 || trayReady {
		return trayReady
	}
	if err := createTrayWindow(); err != nil {
		trayDebug = err.Error()
		return false
	}
	hinst, _, _ := procGetModuleHandleW.Call(0)
	icon, _, _ := procLoadImageW.Call(hinst, 1, imageIcon, 0, 0, lrDefaultSize|lrShared)
	if icon == 0 {
		icon, _, _ = procLoadIconW.Call(0, 32512)
	}
	if icon == 0 {
		trayDebug = "icon load failed"
		destroyTrayWindow()
		return false
	}

	var nid notifyIconData
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = trayHWND
	nid.uID = 1
	nid.uFlags = nifMessage | nifIcon | nifTip
	nid.uCallbackMessage = trayCallback
	nid.hIcon = icon
	copy(nid.szTip[:], windows.StringToUTF16("DeepFocus - screen time"))
	ok, _, notifyErr := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ok == 0 {
		trayDebug = fmt.Sprintf("shell_notify add err=%v hwnd=%x", notifyErr, trayHWND)
		destroyTrayWindow()
		return false
	}

	mainHWND = hwnd
	wndProc := windows.NewCallback(mainWindowProc)
	old, _, callErr := procSetWindowLongPtrW.Call(hwnd, ^uintptr(3), wndProc)
	if old == 0 {
		trayDebug = fmt.Sprintf("setwndproc hwnd=%x err=%v", hwnd, callErr)
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		mainHWND = 0
		destroyTrayWindow()
		return false
	}
	mainOldProc = old
	trayReady = true
	return true
}

func createTrayWindow() error {
	hinst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString("DeepFocus tray")
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)), 0x80000000, 0, 0, 0, 0, 0, 0, hinst, 0)
	if hwnd == 0 {
		return fmt.Errorf("create tray window: %v", err)
	}
	proc := windows.NewCallback(trayMessageProc)
	if old, _, callErr := procSetWindowLongPtrW.Call(hwnd, ^uintptr(3), proc); old == 0 {
		procDestroyWindow.Call(hwnd)
		return fmt.Errorf("subclass tray window: %v", callErr)
	}
	trayHWND = hwnd
	return nil
}
func destroyTrayWindow() {
	if trayHWND != 0 {
		hwnd := trayHWND
		trayHWND = 0
		procDestroyWindow.Call(hwnd)
	}
}

// removeTray hides the icon, restores the window procedure and destroys the
// private message window.
func removeTray() {
	if trayReady {
		var nid notifyIconData
		nid.cbSize = uint32(unsafe.Sizeof(nid))
		nid.hWnd = trayHWND
		nid.uID = 1
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	}
	if mainOldProc != 0 && mainHWND != 0 {
		procSetWindowLongPtrW.Call(mainHWND, ^uintptr(3), mainOldProc)
	}
	mainOldProc = 0
	mainHWND = 0
	trayReady = false
	destroyTrayWindow()
}

func mainWindowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == wmClose && !trayQuitting && trayReady && system.GetCloseToTray() {
		procShowWindow.Call(hwnd, swHide)
		return 0
	}
	return callWindowProc(mainOldProc, hwnd, msg, wParam, lParam)
}

func trayMessageProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case trayCallback:
		switch lParam {
		case wmLButtonDbl, wmLButtonUp:
			showFromTray()
		case wmRButtonUp, wmContextMenu:
			showTrayMenu(hwnd)
		}
	case wmDestroy:
		return 0
	}
	return 0
}

func callWindowProc(prev, hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if prev == 0 {
		return 0
	}
	r, _, _ := procCallWindowProcW.Call(prev, hwnd, uintptr(msg), wParam, lParam)
	return r
}

func showFromTray() {
	if mainHWND == 0 {
		return
	}
	procShowWindow.Call(mainHWND, swRestore)
	procSetForegroundWin.Call(mainHWND)
}

func quitFromTray() {
	hwnd := mainHWND
	trayQuitting = true
	removeTray()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
}

func showTrayMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	open, _ := windows.UTF16PtrFromString("Open DeepFocus / 打开")
	quit, _ := windows.UTF16PtrFromString("Quit DeepFocus / 退出")
	procAppendMenuW.Call(menu, mfString, trayOpenCmd, uintptr(unsafe.Pointer(open)))
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	procAppendMenuW.Call(menu, mfString, trayQuitCmd, uintptr(unsafe.Pointer(quit)))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWin.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(pt.x), uintptr(pt.y), 0, hwnd, 0)
	procPostMessageW.Call(hwnd, wmNull, 0, 0)
	switch cmd {
	case trayOpenCmd:
		showFromTray()
	case trayQuitCmd:
		quitFromTray()
	}
}
