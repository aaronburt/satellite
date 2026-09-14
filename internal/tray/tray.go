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
	"satellite/internal/webui"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

type Tray struct {
	server     *webui.Server
	mqttClient *mqtt.Client
	statusItem *systray.MenuItem
	openItem   *systray.MenuItem
	toggleItem *systray.MenuItem
	quitItem   *systray.MenuItem
	onExit     func()
	cancel     context.CancelFunc
	mu         sync.Mutex
}

func NewTray(server *webui.Server, mqttClient *mqtt.Client, onExit func()) *Tray {
	return &Tray{
		server:     server,
		mqttClient: mqttClient,
		onExit:     onExit,
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

	systray.AddSeparator()

	t.openItem = systray.AddMenuItem("Open WebUI", "Open WebUI configuration in browser")
	t.openItem.Disable()
	t.toggleItem = systray.AddMenuItem("WebUI: Disabled (Click to Enable)", "Enable or disable embedded WebUI HTTP server")

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
				title = "Status: 🟢 Connected"
			case mqtt.StatusConnecting:
				title = "Status: 🟡 Connecting"
			case mqtt.StatusDisconnected:
				title = "Status: 🔴 Disconnected"
			}
			t.statusItem.SetTitle(title)

			if t.server.IsRunning() {
				t.openItem.Enable()
				t.toggleItem.SetTitle("WebUI: Enabled (Click to Disable)")
			} else {
				t.openItem.Disable()
				t.toggleItem.SetTitle("WebUI: Disabled (Click to Enable)")
			}
		}
	}
}

func (t *Tray) eventLoop() {
	for {
		select {
		case <-t.openItem.ClickedCh:
			if t.server.IsRunning() {
				url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", t.server.Port(), t.server.Token())
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
			}
		case <-t.toggleItem.ClickedCh:
			if t.server.IsRunning() {
				t.server.Stop()
				t.openItem.Disable()
				t.toggleItem.SetTitle("WebUI: Disabled (Click to Enable)")
			} else {
				preferredPort := config.Get().WebUIPort
				_, _ = t.server.Start(preferredPort)
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
