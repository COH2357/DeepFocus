package system

import (
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	psapi                        = syscall.NewLazyDLL("psapi.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procGetModuleBaseNameW       = psapi.NewProc("GetModuleBaseNameW")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
	procSendInput                = user32.NewProc("SendInput")
	procSetFocus                 = user32.NewProc("SetFocus")
	procSetActiveWindow          = user32.NewProc("SetActiveWindow")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procSetCursorPos             = user32.NewProc("SetCursorPos")
	procMouseEvent               = user32.NewProc("mouse_event")
	procKeybdEvent               = user32.NewProc("keybd_event")
	procIsWindow                 = user32.NewProc("IsWindow")
)

const (
	processQueryInformation = 0x0400
	processVMRead           = 0x0010
)

type WindowInfo struct {
	HWND    uintptr
	PID     uint32
	Title   string
	ExeName string
	ExePath string
	AppID   string
}

func GetForegroundWindowInfo() (*WindowInfo, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, nil
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	// Modal dialogs, toolbars, and splash screens surface with an empty caption, so
	// resolve the process before treating a blank title as a switch away.
	title := getWindowText(hwnd)

	exeName := ""
	exePath := ""
	if pid != 0 {
		// The full image path is what distinguishes one install from another.
		exePath, _ = getProcessPath(pid)
		if exePath != "" {
			exeName = filepath.Base(exePath)
		} else {
			exeName = getProcessName(pid)
		}
	}

	if title == "" && exeName == "" {
		return nil, nil
	}

	id := ResolveIdentity(exeName, exePath)

	return &WindowInfo{
		HWND:    hwnd,
		PID:     pid,
		Title:   title,
		ExeName: exeName,
		ExePath: exePath,
		AppID:   id.AppID,
	}, nil
}

func getWindowText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}

	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
	return syscall.UTF16ToString(buf)
}

func getProcessName(pid uint32) string {
	// Limited query rights are granted where full query plus VM read are not
	// (elevated and packaged processes), so try the limited handle first and
	// widen only as a fallback.
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		handle, _, _ = procOpenProcess.Call(
			processQueryInformation|processVMRead,
			0,
			uintptr(pid),
		)
	}
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	buf := make([]uint16, 260)
	ret, _, _ := procGetModuleBaseNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		260,
	)
	if ret == 0 {
		// Module enumeration needs VM read; the image path still resolves
		// with only limited query rights.
		if path, err := getProcessPath(pid); err == nil && path != "" {
			return filepath.Base(path)
		}
		return ""
	}
	return syscall.UTF16ToString(buf)
}

const (
	swMinimize = 6
	swRestore  = 9
)

// MinimizeWindow asks Windows to minimize a top-level window. The guard uses
// this instead of terminating the app so unsaved work is never destroyed.
func MinimizeWindow(hwnd uintptr) {
	if hwnd != 0 {
		procShowWindow.Call(hwnd, swMinimize)
	}
}

// RestoreWindow brings a minimized app back after an explicit bypass.
func RestoreWindow(hwnd uintptr) {
	if hwnd != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
}

// ForceForegroundWindow hands the foreground to hwnd even when Windows would
// normally reject SetForegroundWindow from a background process. Attaching the
// current thread to the foreground and target input queues lifts the
// foreground-lock restriction; the caller must keep running on the same OS
// thread until the hand-off completes.
func ForceForegroundWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fg, _, _ := procGetForegroundWindow.Call()
	fgThread, _, _ := procGetWindowThreadProcessId.Call(fg, 0)
	targetThread, _, _ := procGetWindowThreadProcessId.Call(hwnd, 0)
	curThread, _, _ := procGetCurrentThreadId.Call()

	attachedFg := false
	attachedTarget := false
	if fgThread != 0 && fgThread != curThread {
		procAttachThreadInput.Call(curThread, fgThread, 1)
		attachedFg = true
	}
	if targetThread != 0 && targetThread != curThread && targetThread != fgThread {
		procAttachThreadInput.Call(curThread, targetThread, 1)
		attachedTarget = true
	}
	if attachedTarget {
		defer procAttachThreadInput.Call(curThread, targetThread, 0)
	}
	if attachedFg {
		defer procAttachThreadInput.Call(curThread, fgThread, 0)
	}

	RestoreWindow(hwnd)
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	procSetActiveWindow.Call(hwnd)
	procSetFocus.Call(hwnd)
}

// FocusWindow restores a window and makes it the foreground window. It is
// called while the blocker still owns the foreground so Windows permits the
// hand-off; hiding the blocker first would leave the browser behind another
// app and the keyboard tab sweep would never reach it.
func FocusWindow(hwnd uintptr) {
	ForceForegroundWindow(hwnd)
}

const (
	vkControl = 0x11
	vkTab     = 0x09
	vkClose   = 0x57
	keyUp     = 0x0002
)

type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type keyboardInput struct {
	typ uint32
	_   uint32
	ki  keybdInput
	_   [8]byte
}

func sendKey(vk uintptr, flags uint32) {
	in := keyboardInput{typ: 1}
	in.ki.wVk = uint16(vk)
	in.ki.dwFlags = flags
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func sendCtrlKey(vk uintptr) {
	// Chromium ignores key chords whose down/up events arrive in the same
	// message-loop turn, so keep a small gap between each transition.
	sendKey(vkControl, 0)
	time.Sleep(25 * time.Millisecond)
	sendKey(vk, 0)
	time.Sleep(35 * time.Millisecond)
	sendKey(vk, keyUp)
	time.Sleep(20 * time.Millisecond)
	sendKey(vkControl, keyUp)
	time.Sleep(20 * time.Millisecond)
}

// ClickWindowTitle focuses a window with a harmless click on its title bar.
// SetForegroundWindow alone can leave keyboard focus behind on the hidden
// blocker, so browser shortcuts would never reach the tab strip.
func ClickWindowTitle(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	var rect struct {
		left, top, right, bottom int32
	}
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return
	}
	x := (rect.left + rect.right) / 2
	y := rect.top + 12
	procSetCursorPos.Call(uintptr(x), uintptr(y))
	time.Sleep(40 * time.Millisecond)
	procMouseEvent.Call(0x0002, 0, 0, 0, 0)
	time.Sleep(40 * time.Millisecond)
	procMouseEvent.Call(0x0004, 0, 0, 0, 0)
	time.Sleep(120 * time.Millisecond)
}

// CloseBrowserTabs restores a browser window, then walks its tabs and closes
// every tab whose title matches. It is used by "close this site's tabs" so a
// single blocked tab cannot leave more tabs of the same class in front and
// make the guard minimize the browser again immediately.
//
// Chromium browsers do not expose individual tabs as top-level windows, so the
// only portable route is keyboard navigation: close a matching active tab, and
// otherwise move to the next tab. The walk is bounded (48 steps) and stops early
// when Ctrl+Tab no longer changes the page.
func CloseBrowserTabs(hwnd uintptr, match func(string) bool) int {
	if hwnd == 0 || match == nil {
		return 0
	}
	RestoreWindow(hwnd)
	ForceForegroundWindow(hwnd)
	time.Sleep(250 * time.Millisecond)
	ClickWindowTitle(hwnd)

	first, err := GetForegroundWindowInfo()
	if err != nil || first == nil {
		return 0
	}
	pid := first.PID
	closed := 0
	unchanged := 0
	lastTitle := ""

	for step := 0; step < 48; step++ {
		info, err := GetForegroundWindowInfo()
		if err != nil || info == nil || info.PID != pid {
			break
		}
		if match(info.Title) {
			sendCtrlKey(vkClose)
			closed++
			unchanged = 0
			lastTitle = ""
			time.Sleep(220 * time.Millisecond)
			continue
		}
		if info.Title == lastTitle {
			unchanged++
			if unchanged >= 2 {
				break
			}
		} else {
			unchanged = 0
			lastTitle = info.Title
		}
		sendCtrlKey(vkTab)
		time.Sleep(140 * time.Millisecond)
	}
	return closed
}

// WindowExists reports whether a top-level window handle is still alive.
func WindowExists(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procIsWindow.Call(hwnd)
	return ret != 0
}

// TargetAlive reports whether the app or browser window that triggered a
// blocker still exists. Browsers such as Edge keep background processes alive
// after their last window closes, so PID liveness alone would leave the
// reminder's "already closed" button disabled forever.
func TargetAlive(hwnd uintptr, pid uint32) bool {
	if hwnd != 0 {
		if !WindowExists(hwnd) {
			return false
		}
		var owner uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner != 0 && pid != 0 && owner != pid {
			return false
		}
		return true
	}
	return ProcessRunning(pid)
}
