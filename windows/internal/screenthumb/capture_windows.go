//go:build windows

package screenthumb

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	modUser32 = syscall.NewLazyDLL("user32.dll")
	modGdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC               = modUser32.NewProc("GetDC")
	procReleaseDC           = modUser32.NewProc("ReleaseDC")
	procGetSystemMetrics    = modUser32.NewProc("GetSystemMetrics")
	procEnumDisplayMonitors = modUser32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfo      = modUser32.NewProc("GetMonitorInfoW")
	procSetThreadDPI        = modUser32.NewProc("SetThreadDpiAwarenessContext")

	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procSetStretchBltMode      = modGdi32.NewProc("SetStretchBltMode")
	procSetBrushOrgEx          = modGdi32.NewProc("SetBrushOrgEx")
	procStretchBlt             = modGdi32.NewProc("StretchBlt")
	procGetDIBits              = modGdi32.NewProc("GetDIBits")
)

const (
	smCxScreen   = 0
	smCyScreen   = 1
	halftone     = 4
	srcCopy      = 0x00CC0020
	dibRGBColors = 0
	biRGB        = 0
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

func Capture(maxSide int) (image.Image, error) {
	displays, err := Displays()
	if err != nil {
		return nil, err
	}
	display, ok := ResolveDisplay(displays, 0)
	if !ok {
		return nil, errors.New("primary display is not available")
	}
	return CaptureDisplay(display, maxSide)
}

func CaptureDisplay(display Display, maxSide int) (image.Image, error) {
	restore := physicalCoordinates()
	defer restore()
	w, h, err := ThumbSize(display.Width, display.Height, maxSide)
	if err != nil {
		return nil, err
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, errors.New("GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	bmp, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(w), uintptr(h))
	if bmp == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bmp)

	old, _, _ := procSelectObject.Call(memDC, bmp)
	defer procSelectObject.Call(memDC, old)

	procSetStretchBltMode.Call(memDC, halftone)
	procSetBrushOrgEx.Call(memDC, 0, 0, 0)
	ok, _, callErr := procStretchBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h),
		screenDC, uintptr(display.X), uintptr(display.Y), uintptr(display.Width), uintptr(display.Height), srcCopy)
	if ok == 0 {
		return nil, fmt.Errorf("StretchBlt failed: %v", callErr)
	}

	procSelectObject.Call(memDC, old)

	bi := bitmapInfo{Header: bitmapInfoHeader{
		Width:       int32(w),
		Height:      -int32(h),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}}
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	lines, _, _ := procGetDIBits.Call(memDC, bmp, 0, uintptr(h),
		uintptr(unsafe.Pointer(&img.Pix[0])), uintptr(unsafe.Pointer(&bi)), dibRGBColors)
	if int(lines) != h {
		return nil, fmt.Errorf("GetDIBits copied %d of %d rows", lines, h)
	}
	convertBGRA(img.Pix, img.Pix)
	return img, nil
}

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
	Device  [32]uint16
}

var displayEnumeration struct {
	sync.Mutex
	displays []Display
	err      error
}

var displayCallback = syscall.NewCallback(func(monitor, _, _, _ uintptr) uintptr {
	info := monitorInfo{}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, err := procGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		displayEnumeration.err = fmt.Errorf("GetMonitorInfo failed: %v", err)
		return 0
	}
	device := syscall.UTF16ToString(info.Device[:])
	id, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(device), `\\.\DISPLAY`))
	if err != nil || id <= 0 || id > MaxDisplays {
		displayEnumeration.err = fmt.Errorf("unsupported display identifier %q", device)
		return 0
	}
	displayEnumeration.displays = append(displayEnumeration.displays, Display{
		ID: id, Name: fmt.Sprintf("Display %d", id), Device: device,
		Primary: info.Flags&1 != 0,
		X:       int(info.Monitor.Left), Y: int(info.Monitor.Top),
		Width: int(info.Monitor.Right - info.Monitor.Left), Height: int(info.Monitor.Bottom - info.Monitor.Top),
	})
	return 1
})

func platformDisplays() ([]Display, error) {
	displayEnumeration.Lock()
	defer displayEnumeration.Unlock()
	restore := physicalCoordinates()
	defer restore()
	displayEnumeration.displays = nil
	displayEnumeration.err = nil
	ok, _, err := procEnumDisplayMonitors.Call(0, 0, displayCallback, 0)
	if displayEnumeration.err != nil {
		return nil, displayEnumeration.err
	}
	if ok == 0 {
		return nil, fmt.Errorf("EnumDisplayMonitors failed: %v", err)
	}
	displays := displayEnumeration.displays
	displayEnumeration.displays = nil
	slices.SortFunc(displays, func(a, b Display) int { return a.ID - b.ID })
	return displays, nil
}

func physicalCoordinates() func() {
	runtime.LockOSThread()
	var previous uintptr
	if procSetThreadDPI.Find() == nil {
		previous, _, _ = procSetThreadDPI.Call(^uintptr(3))
	}
	return func() {
		if previous != 0 {
			procSetThreadDPI.Call(previous)
		}
		runtime.UnlockOSThread()
	}
}
