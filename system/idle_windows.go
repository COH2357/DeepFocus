package system

import (
	"sync"
	"time"
	"unsafe"
)

// lastInputInfo mirrors the Win32 LASTINPUTINFO structure.
type lastInputInfo struct {
	CBSize uint32
	DwTime uint32
}

var (
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount     = kernel32.NewProc("GetTickCount")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
)

// cursor activity is tracked separately from GetLastInputInfo. The Win32 input
// clock already counts mouse movement, but some remote, touchpad and injected
// input paths update the pointer without refreshing it. Sampling the pointer
// position as well makes "the mouse moved" an explicit activity signal rather
// than an assumption, and costs one cheap user32 call per poll.
type cursorPoint struct {
	X int32
	Y int32
}

var (
	cursorMu   sync.Mutex
	cursorLast cursorPoint
	cursorAt   time.Time
	cursorSeen bool
)

func pointerIdleSeconds(now time.Time) int {
	var p cursorPoint
	ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if ret == 0 {
		return 0
	}

	cursorMu.Lock()
	if !cursorSeen || p != cursorLast {
		cursorLast = p
		cursorAt = now
		cursorSeen = true
	}
	idle := now.Sub(cursorAt)
	cursorMu.Unlock()

	if idle < 0 {
		return 0
	}
	return int(idle / time.Second)
}

// GetIdleSeconds returns how many whole seconds the user has been away from the
// keyboard and mouse.
func GetIdleSeconds() (int, error) {
	var info lastInputInfo
	info.CBSize = uint32(unsafe.Sizeof(info))

	ret, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, err
	}

	// uint32 subtraction so a GetTickCount wraparound every 49 days cannot go
	// negative. The result is only compared against a threshold, never summed.
	idleMillis := uint32(GetTickCount()) - info.DwTime
	idle := int(idleMillis / 1000)

	// Whichever signal is fresher wins: a keypress or a pointer movement.
	if pointerIdle := pointerIdleSeconds(time.Now()); pointerIdle < idle {
		idle = pointerIdle
	}
	return idle, nil
}

// GetTickCount returns the number of milliseconds since the system started.
func GetTickCount() uint32 {
	ret, _, _ := procGetTickCount.Call()
	return uint32(ret)
}
