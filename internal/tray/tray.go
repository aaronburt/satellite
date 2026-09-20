//go:build windows

package tray

import (
	"context"
	_ "embed"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"satellite/internal/config"
	"satellite/internal/mqtt"
	"satellite/internal/toast"
	"satellite/internal/updater"
	"satellite/internal/webui"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

type Tray struct {
	server          *webui.Server
	mqttClient      *mqtt.Client
	updaterInstance *updater.Checker
	statusItem      *systray.MenuItem
	updateItem      *systray.MenuItem
	checkUpdateItem *systray.MenuItem
	openItem        *systray.MenuItem
	toggleItem      *systray.MenuItem
	quitItem        *systray.MenuItem
	onExit          func()
	cancel          context.CancelFunc
	mu              sync.Mutex
}

func NewTray(server *webui.Server, mqttClient *mqtt.Client, updaterInstance *updater.Checker, onExit func()) *Tray {
	return &Tray{
		server:          server,
		mqttClient:      mqttClient,
		updaterInstance: updaterInstance,
		onExit:          onExit,
	}
}

func (t *Tray) Run() {
	systray.Run(t.onReady, t.onSystrayExit)
}

func (t *Tray) onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("Satellite Agent")
	systray.SetTooltip("Satellite Agent - Home Assistant")

	t.statusItem = systray.AddMenuItem("Status: Checking...", "MQTT Connection Status")
	t.statusItem.Disable()

	t.updateItem = systray.AddMenuItem("Update Available", "Click to view release on GitHub")
	t.updateItem.Hide()

	systray.AddSeparator()

	t.openItem = systray.AddMenuItem("Open WebUI", "Open WebUI configuration in browser")
	t.openItem.Disable()
	t.toggleItem = systray.AddMenuItem("WebUI: Disabled (Click to Enable)", "Enable or disable embedded WebUI HTTP server")
	t.checkUpdateItem = systray.AddMenuItem("Check for Updates", "Check GitHub for newer versions")

	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("Exit", "Exit Satellite Agent")

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	go t.statusUpdater(ctx)
	go t.eventLoop()
}

func (t *Tray) onSystrayExit() {
	if t.cancel != nil {
		t.cancel()
	}
	if t.onExit != nil {
		t.onExit()
	}
}

func (t *Tray) statusUpdater(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := t.mqttClient.Status()
			title := fmt.Sprintf("Status: %s", status)
			switch status {
			case mqtt.StatusConnected:
				title = "Status: Connected"
			case mqtt.StatusConnecting:
				title = "Status: Connecting"
			case mqtt.StatusDisconnected:
				title = "Status: Disconnected"
			}
			t.statusItem.SetTitle(title)

			if t.server.IsWebUIEnabled() && t.server.IsRunning() {
				t.openItem.Enable()
				t.toggleItem.SetTitle("WebUI: Enabled (Click to Disable)")
			} else {
				t.openItem.Disable()
				t.toggleItem.SetTitle("WebUI: Disabled (Click to Enable)")
			}

			if t.updaterInstance != nil {
				updateStatus := t.updaterInstance.Status()
				if updateStatus.Available {
					t.updateItem.SetTitle(fmt.Sprintf("Update Available: %s (Click to View)", updateStatus.LatestVersion))
					t.updateItem.Show()
				} else {
					t.updateItem.Hide()
				}
			}
		}
	}
}

func (t *Tray) eventLoop() {
	for {
		select {
		case <-t.updateItem.ClickedCh:
			if t.updaterInstance != nil {
				status := t.updaterInstance.Status()
				if status.ReleaseURL != "" {
					_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", status.ReleaseURL).Start()
				}
			}
		case <-t.checkUpdateItem.ClickedCh:
			go t.manualCheckUpdate()
		case <-t.openItem.ClickedCh:
			if t.server.IsWebUIEnabled() && t.server.IsRunning() {
				url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", t.server.Port(), t.server.Token())
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
			}
		case <-t.toggleItem.ClickedCh:
			if t.server.IsWebUIEnabled() {
				t.server.SetWebUIEnabled(false)
				if !t.server.IsJSONEnabled() {
					t.server.Stop()
				}
				t.openItem.Disable()
				t.toggleItem.SetTitle("WebUI: Disabled (Click to Enable)")
			} else {
				t.server.SetWebUIEnabled(true)
				if !t.server.IsRunning() {
					_, _ = t.server.Start(config.Get().GetPort())
				}
				if t.server.IsRunning() {
					t.openItem.Enable()
					t.toggleItem.SetTitle("WebUI: Enabled (Click to Disable)")
				}
			}
		case <-t.quitItem.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (t *Tray) manualCheckUpdate() {
	if t.updaterInstance == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	status, err := t.updaterInstance.Check(ctx)
	if err != nil {
		_ = toast.Show(toast.Notification{
			Title:   "Satellite Update",
			Message: fmt.Sprintf("Update check failed: %v", err),
		})
		return
	}

	if status.Available {
		_ = toast.Show(toast.Notification{
			Title:   "Satellite Update Available",
			Message: fmt.Sprintf("Version %s is available. Click to download.", status.LatestVersion),
			URL:     status.ReleaseURL,
		})
	} else {
		_ = toast.Show(toast.Notification{
			Title:   "Satellite Up to Date",
			Message: fmt.Sprintf("Satellite is up to date (%s).", config.Version),
		})
	}
}
