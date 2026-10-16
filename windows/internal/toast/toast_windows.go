//go:build windows

package toast

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/gen2brain/beeep"
)

//go:embed icon.ico
var embeddedIcon []byte

var (
	iconOnce     sync.Once
	savedIconPath string
)

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
	iconPath := getIconPath()
	title := strings.TrimSpace(n.Title)
	if title == "" {
		title = "Satellite"
	}

	err := beeep.Notify(title, n.Message, iconPath)
	if err != nil {
		return fmt.Errorf("notification failed: %w", err)
	}

	if strings.TrimSpace(n.URL) != "" {
		go func(targetURL string) {
			cmd := exec.Command("cmd", "/c", "start", "", targetURL)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			_ = cmd.Start()
		}(strings.TrimSpace(n.URL))
	}

	return nil
}
