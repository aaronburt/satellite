//go:build windows

package toast

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/gen2brain/beeep"
	"golang.org/x/sys/windows"
)

//go:embed icon.ico
var embeddedIcon []byte

var (
	iconOnce      sync.Once
	savedIconPath string
)

// Seams so tests don't show real notifications or open a browser.
var (
	notify  = beeep.Notify
	openURL = OpenBrowser
)

func OpenBrowser(rawURL string) error {
	target, ok := SafeURL(rawURL)
	if !ok {
		return fmt.Errorf("unsafe or invalid URL")
	}

	runtime.LockOSThread()
	hr := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
	comReady := hr == nil || hr == syscall.Errno(1)
	verb, _ := windows.UTF16PtrFromString("open")
	urlPtr, ptrErr := windows.UTF16PtrFromString(target)
	var shellErr error
	if ptrErr != nil {
		shellErr = ptrErr
	} else {
		shellErr = windows.ShellExecute(0, verb, urlPtr, nil, nil, windows.SW_SHOWNORMAL)
	}
	if comReady {
		windows.CoUninitialize()
	}
	runtime.UnlockOSThread()

	if shellErr == nil {
		return nil
	}

	cmd := exec.Command("cmd", "/c", "start", "", target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return shellErr
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func getIconPath() string {
	iconOnce.Do(func() {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = "."
		}
		dir := filepath.Join(appData, "satellite")
		_ = os.MkdirAll(dir, 0755)
		target := filepath.Join(dir, "icon.ico")
		if len(embeddedIcon) > 0 {
			if _, err := os.Stat(target); os.IsNotExist(err) {
				_ = os.WriteFile(target, embeddedIcon, 0644)
			}
		}
		savedIconPath = target
	})
	return savedIconPath
}

func Show(n Notification) error {
	title := strings.TrimSpace(n.Title)
	if title == "" {
		title = "Satellite"
	}

	if err := notify(title, n.Message, getIconPath()); err != nil {
		return fmt.Errorf("notification failed: %w", err)
	}

	if target, ok := SafeURL(n.URL); ok {
		_ = openURL(target)
	}
	return nil
}
