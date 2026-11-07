//go:build windows

package screenthumb

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	modDwmapi    = syscall.NewLazyDLL("dwmapi.dll")
	modKernel32  = syscall.NewLazyDLL("kernel32.dll")
	procEnumWins = modUser32.NewProc("EnumWindows")
	procIsVis    = modUser32.NewProc("IsWindowVisible")
	procIsIconic = modUser32.NewProc("IsIconic")
	procGetRect  = modUser32.NewProc("GetWindowRect")
	procGetPID   = modUser32.NewProc("GetWindowThreadProcessId")
	procDwmAttr  = modDwmapi.NewProc("DwmGetWindowAttribute")
	procOpenProc = modKernel32.NewProc("OpenProcess")
	procCloseH   = modKernel32.NewProc("CloseHandle")
	procImage    = modKernel32.NewProc("QueryFullProcessImageNameW")

	enumCallback = syscall.NewCallback(enumProc)
)

const (
	dwmaExtendedFrameBounds = 9
	dwmaCloaked             = 14
	processQueryLimited     = 0x1000
)

type winRect struct{ Left, Top, Right, Bottom int32 }

// enumState is shared with the EnumWindows callback. EnumWindows runs the
// callback on the calling thread, one call at a time.
var enumState struct {
	apps  []string
	pids  map[uint32]bool
	rects []Rect
}

func enumProc(hwnd, _ uintptr) uintptr {
	if v, _, _ := procIsVis.Call(hwnd); v == 0 {
		return 1
	}
	if v, _, _ := procIsIconic.Call(hwnd); v != 0 {
		return 1
	}
	var cloaked uint32
	procDwmAttr.Call(hwnd, dwmaCloaked, uintptr(unsafe.Pointer(&cloaked)), 4)
	if cloaked != 0 {
		return 1
	}
	var pid uint32
	procGetPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return 1
	}
	match, known := enumState.pids[pid]
	if !known {
		match = MatchApp(processImageName(pid), enumState.apps)
		enumState.pids[pid] = match
	}
	if !match {
		return 1
	}
	var r winRect
	hr, _, _ := procDwmAttr.Call(hwnd, dwmaExtendedFrameBounds, uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r))
	if hr != 0 {
		if ok, _, _ := procGetRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
			return 1
		}
	}
	enumState.rects = append(enumState.rects, Rect{X: int(r.Left), Y: int(r.Top), W: int(r.Right - r.Left), H: int(r.Bottom - r.Top)})
	return 1
}

func processImageName(pid uint32) string {
	h, _, _ := procOpenProc.Call(processQueryLimited, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseH.Call(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	if ok, _, _ := procImage.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}

var enumLock = make(chan struct{}, 1)

// PlatformHiddenRects lists the visible top-level windows of the given apps,
// in coordinates of the primary screen that Capture reads.
func PlatformHiddenRects(apps []string) ([]Rect, int, int, error) {
	enumLock <- struct{}{}
	defer func() { <-enumLock }()
	enumState.apps = apps
	enumState.pids = map[uint32]bool{}
	enumState.rects = nil
	defer func() { enumState.apps, enumState.pids, enumState.rects = nil, nil, nil }()
	if ok, _, err := procEnumWins.Call(enumCallback, 0); ok == 0 {
		return nil, 0, 0, fmt.Errorf("EnumWindows failed: %v", err)
	}
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	return append([]Rect(nil), enumState.rects...), int(sw), int(sh), nil
}
