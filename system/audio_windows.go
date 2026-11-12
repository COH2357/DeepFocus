//go:build windows

package system

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The idle clock only knows about keyboard and mouse input. Someone watching a
// long video is legitimately at the computer with both hands off the desk, so
// the tracker also listens to the default playback device: while audio is
// actually audible, the user is treated as present. This uses WASAPI's peak
// meter, which reports the post-mix signal without touching the audio stream.

type comGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")

	clsidMMDeviceEnumerator = comGUID{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = comGUID{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioMeter          = comGUID{0xC02216F6, 0x8C67, 0x4B5B, [8]byte{0x9D, 0x00, 0xD0, 0x08, 0xE7, 0x3E, 0x00, 0x64}}
)

const (
	clsctxAll           = 0x17
	coinitMultithreaded = 0x0
	eRender             = 0
	eConsole            = 0

	// vtable slots (IUnknown occupies 0..2 in every interface).
	slotRelease            = 2
	slotGetDefaultEndpoint = 4
	slotActivate           = 3
	slotGetPeakValue       = 3

	// A peak above this is real signal rather than mixer noise. Quiet dialogue
	// still clears it by orders of magnitude.
	audiblePeakThreshold = 0.0005
)

var (
	audioOnce   sync.Once
	audioActive atomic.Bool
)

// IsAudioPlaying reports whether the default playback device is emitting
// sound. The first call starts a lightweight background sampler; failures are
// reported as "not playing" so a broken audio stack never freezes tracking.
func IsAudioPlaying() bool {
	audioOnce.Do(func() {
		go sampleAudioPlayback()
	})
	return audioActive.Load()
}

func sampleAudioPlayback() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// COINIT_MULTITHREADED is fine for a thread without a message pump. If COM
	// was already initialized in another mode we still keep going; the objects
	// below live entirely on this goroutine.
	_, _, _ = procCoInitializeEx.Call(0, coinitMultithreaded)

	var enumerator unsafe.Pointer
	for {
		if enumerator == nil {
			enumerator = newMMDeviceEnumerator()
			if enumerator == nil {
				audioActive.Store(false)
				time.Sleep(3 * time.Second)
				continue
			}
		}

		peak, ok := currentPeak(enumerator)
		if !ok {
			// The default endpoint may have changed or the device was removed.
			// Rebuild the enumerator next round.
			releaseCOM(enumerator)
			enumerator = nil
			audioActive.Store(false)
			time.Sleep(2 * time.Second)
			continue
		}

		audioActive.Store(peak > audiblePeakThreshold)
		time.Sleep(900 * time.Millisecond)
	}
}

func newMMDeviceEnumerator() unsafe.Pointer {
	var ptr unsafe.Pointer
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)),
		0,
		clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&ptr)),
	)
	if hrFailed(hr) {
		return nil
	}
	return ptr
}

func currentPeak(enumerator unsafe.Pointer) (float32, bool) {
	var device unsafe.Pointer
	hr := comInvoke(enumerator, slotGetDefaultEndpoint,
		uintptr(eRender), uintptr(eConsole), uintptr(unsafe.Pointer(&device)))
	if hrFailed(hr) || device == nil {
		return 0, false
	}
	defer releaseCOM(device)

	var meter unsafe.Pointer
	hr = comInvoke(device, slotActivate,
		uintptr(unsafe.Pointer(&iidIAudioMeter)), clsctxAll, 0,
		uintptr(unsafe.Pointer(&meter)))
	if hrFailed(hr) || meter == nil {
		return 0, false
	}
	defer releaseCOM(meter)

	var peak float32
	hr = comInvoke(meter, slotGetPeakValue, uintptr(unsafe.Pointer(&peak)))
	if hrFailed(hr) {
		return 0, false
	}
	return peak, true
}

func comInvoke(obj unsafe.Pointer, slot int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(obj)
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, uintptr(obj))
	callArgs = append(callArgs, args...)
	r, _, _ := syscall.SyscallN(fn, callArgs...)
	return r
}

func releaseCOM(obj unsafe.Pointer) {
	if obj != nil {
		_ = comInvoke(obj, slotRelease)
	}
}

func hrFailed(hr uintptr) bool {
	return int32(hr) < 0
}
