package system

import (
	"path/filepath"
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

// CloseActiveTab restores a browser window and sends Ctrl+W. The site blocker
// uses it for its "leave this page" action so the user is not left staring at
// the page that hit the limit; the limit itself is not bypassed.
func CloseActiveTab(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	RestoreWindow(hwnd)
	procSetForegroundWindow.Call(hwnd)
	time.Sleep(180 * time.Millisecond)
	const vkControl = 0x11
	const keyUp = 0x0002
	procKeybdEvent.Call(vkControl, 0, 0, 0)
	procKeybdEvent.Call('W', 0, 0, 0)
	procKeybdEvent.Call('W', 0, keyUp, 0)
	procKeybdEvent.Call(vkControl, 0, keyUp, 0)
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
