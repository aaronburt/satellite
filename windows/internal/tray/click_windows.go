package tray

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var trayClickCallback uintptr

func installTrayDoubleClick(open chan struct{}) error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	findWindow := user32.NewProc("FindWindowExW")
	getProcess := user32.NewProc("GetWindowThreadProcessId")
	getName, setName := "GetWindowLongPtrW", "SetWindowLongPtrW"
	if unsafe.Sizeof(uintptr(0)) == 4 {
		getName, setName = "GetWindowLongW", "SetWindowLongW"
	}
	getProc := user32.NewProc(getName)
	setProc := user32.NewProc(setName)
	callProc := user32.NewProc("CallWindowProcW")
	class, err := windows.UTF16PtrFromString("SystrayClass")
	if err != nil {
		return err
	}
	var window uintptr
	for {
		window, _, _ = findWindow.Call(0, window, uintptr(unsafe.Pointer(class)), 0)
		if window == 0 {
			return fmt.Errorf("Satellite tray window not found")
		}
		var pid uint32
		getProcess.Call(window, uintptr(unsafe.Pointer(&pid)))
		if pid == uint32(os.Getpid()) {
			break
		}
	}
	index := ^uintptr(3)
	previous, _, err := getProc.Call(window, index)
	if previous == 0 {
		return fmt.Errorf("read tray window procedure: %w", err)
	}
	trayClickCallback = windows.NewCallback(func(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
		if message == 0x0401 {
			switch lparam {
			case 0x0202:
				return 0
			case 0x0203:
				select {
				case open <- struct{}{}:
				default:
				}
				return 0
			}
		}
		result, _, _ := callProc.Call(previous, hwnd, uintptr(message), wparam, lparam)
		return result
	})
	result, _, err := setProc.Call(window, index, trayClickCallback)
	if result == 0 {
		return fmt.Errorf("install tray double-click handler: %w", err)
	}
	return nil
}
