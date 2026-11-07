//go:build windows

package screenthumb

import (
	"errors"
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

var (
	modUser32 = syscall.NewLazyDLL("user32.dll")
	modGdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC            = modUser32.NewProc("GetDC")
	procReleaseDC        = modUser32.NewProc("ReleaseDC")
	procGetSystemMetrics = modUser32.NewProc("GetSystemMetrics")

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

// Capture copies the primary monitor straight into a bitmap of thumbnail
// size. Windows scales it during the copy (StretchBlt with HALFTONE), so
// the full-resolution screen is never read into this process.
func Capture(maxSide int) (image.Image, error) {
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	w, h, err := ThumbSize(int(sw), int(sh), maxSide)
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
	procSetBrushOrgEx.Call(memDC, 0, 0, 0) // required after selecting HALFTONE
	ok, _, callErr := procStretchBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h),
		screenDC, 0, 0, sw, sh, srcCopy)
	if ok == 0 {
		// Fails on the secure desktop (lock screen, UAC prompt).
		return nil, fmt.Errorf("StretchBlt failed: %v", callErr)
	}

	// Deselect the bitmap before reading it, as GetDIBits requires.
	procSelectObject.Call(memDC, old)

	bi := bitmapInfo{Header: bitmapInfoHeader{
		Width:       int32(w),
		Height:      -int32(h), // negative: top-down rows
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}}
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	buf := make([]byte, w*h*4)
	lines, _, _ := procGetDIBits.Call(memDC, bmp, 0, uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), dibRGBColors)
	if int(lines) != h {
		return nil, fmt.Errorf("GetDIBits copied %d of %d rows", lines, h)
	}
	return FromBGRA(buf, w, h)
}
