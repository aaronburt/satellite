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

	"github.com/gen2brain/beeep"
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
	openURL = func(u string) error {
		// FileProtocolHandler opens the URL with the default browser without
		// going through cmd.exe, so characters like & cannot run commands.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	}
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
