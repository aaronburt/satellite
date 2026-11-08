//go:build windows

package tray

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"satellite/internal/config"
	"satellite/internal/desktopui"
	"satellite/internal/logger"
	"satellite/internal/mqtt"
	"satellite/internal/privacy"
	"satellite/internal/toast"
	"satellite/internal/updater"
	"satellite/internal/webui"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

var (
	openBrowser = toast.OpenBrowser
	openClient  = func(client *desktopui.Client, target string) error { return client.Open(target) }
	showToast   = toast.Show
)

type Tray struct {
	server          *webui.Server
	mqttClient      *mqtt.Client
	updaterInstance *updater.Checker
	statusItem      *systray.MenuItem
	updateItem      *systray.MenuItem
	checkUpdateItem *systray.MenuItem
	openItem        *systray.MenuItem
	browserItem     *systray.MenuItem
	uiClient        *desktopui.Client
	toggleItem      *systray.MenuItem
	quitItem        *systray.MenuItem
	privacyItem     *systray.MenuItem
	resumeItem      *systray.MenuItem
	pauseItems      [4]*systray.MenuItem
	onExit          func()
	cancel          context.CancelFunc
	mu              sync.Mutex
	titles          map[*systray.MenuItem]string
	updateVisible   bool
	privacyLabel    string
}

func NewTray(server *webui.Server, mqttClient *mqtt.Client, updaterInstance *updater.Checker, onExit func()) *Tray {
	return &Tray{
		server:          server,
		mqttClient:      mqttClient,
		updaterInstance: updaterInstance,
		onExit:          onExit,
		uiClient:        desktopui.New(),
	}
}

func (t *Tray) Run() {
	systray.Run(t.onReady, t.onSystrayExit)
}

func (t *Tray) onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("Satellite Agent")
	systray.SetTooltip("Satellite Agent - Home Assistant")

	t.statusItem = systray.AddMenuItem("MQTT: Connecting", "MQTT connection status")
	t.statusItem.Disable()
	systray.AddSeparator()
	t.openItem = systray.AddMenuItem("Open Satellite", "Open Satellite configuration window")
	t.updateItem = systray.AddMenuItem("Update available...", "View the latest release")
	t.updateItem.Hide()
	systray.AddSeparator()
	t.privacyItem = systray.AddMenuItem("Privacy Pause", "Temporarily pause sensitive sensors")
	for i, minutes := range privacyMinutes {
		t.pauseItems[i] = t.privacyItem.AddSubMenuItem(fmt.Sprintf("Pause for %d minutes", minutes), "Automatically resume sensitive sensors")
	}
	t.resumeItem = systray.AddMenuItem("Resume sensitive sensors", "End the privacy pause early")
	t.refreshPrivacy()
	systray.AddSeparator()
	more := systray.AddMenuItem("More", "Server and update controls")
	t.browserItem = more.AddSubMenuItem("Open in Browser", "Open Satellite in your default browser")
	t.toggleItem = more.AddSubMenuItem("Enable web server", "Toggle the embedded WebUI HTTP server")
	t.checkUpdateItem = more.AddSubMenuItem("Check for Updates", "Check for newer versions")
	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("Quit Satellite", "Stop Satellite and exit")

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	go t.statusUpdater(ctx)
	go t.eventLoop()
	if err := installTrayDoubleClick(t.openItem.ClickedCh); err != nil {
		logger.Warn("tray", err.Error())
	}
}

func (t *Tray) onSystrayExit() {
	if t.uiClient != nil {
		t.uiClient.Close()
	}
	if t.cancel != nil {
		t.cancel()
	}
	if t.onExit != nil {
		t.onExit()
	}
}

func (t *Tray) statusUpdater(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.refreshPrivacy()
			status := t.mqttClient.Status()
			title := fmt.Sprintf("MQTT: %s", status)
			switch status {
			case mqtt.StatusConnected:
				title = "MQTT: Connected"
			case mqtt.StatusConnecting:
				title = "MQTT: Connecting"
			case mqtt.StatusDisconnected:
				title = "MQTT: Disconnected"
			}
			t.setTitle(t.statusItem, title)

			if t.server.IsWebUIEnabled() && t.server.IsRunning() {
				t.setTitle(t.toggleItem, "Disable web server")
			} else {
				t.setTitle(t.toggleItem, "Enable web server")
			}

			if t.updaterInstance != nil {
				updateStatus := t.updaterInstance.Status()
				if updateStatus.Available {
					t.setTitle(t.updateItem, fmt.Sprintf("Update available: %s...", updateStatus.LatestVersion))
					if !t.updateVisible {
						t.updateItem.Show()
						t.updateVisible = true
					}
				} else {
					if t.updateVisible {
						t.updateItem.Hide()
						t.updateVisible = false
					}
				}
			}
		}
	}
}

func (t *Tray) eventLoop() {
	for {
		select {
		case <-clicked(t.pauseItems[0]):
			t.pausePrivacy(time.Duration(privacyMinutes[0]) * time.Minute)
		case <-clicked(t.pauseItems[1]):
			t.pausePrivacy(time.Duration(privacyMinutes[1]) * time.Minute)
		case <-clicked(t.pauseItems[2]):
			t.pausePrivacy(time.Duration(privacyMinutes[2]) * time.Minute)
		case <-clicked(t.pauseItems[3]):
			t.pausePrivacy(time.Duration(privacyMinutes[3]) * time.Minute)
		case <-clicked(t.resumeItem):
			t.pausePrivacy(0)
		case <-t.updateItem.ClickedCh:
			if t.updaterInstance != nil {
				status := t.updaterInstance.Status()
				if status.ReleaseURL != "" {
					if err := openBrowser(status.ReleaseURL); err != nil {
						logger.Warn("tray", fmt.Sprintf("Failed to open release URL in browser: %v", err))
					}
				}
			}
		case <-t.checkUpdateItem.ClickedCh:
			go t.manualCheckUpdate()
		case <-t.openItem.ClickedCh:
			if t.enableWebUI() {
				if err := openClient(t.uiClient, t.server.URL()); err != nil {
					logger.Warn("tray", err.Error())
					_ = showToast(toast.Notification{Title: "Satellite window unavailable", Message: err.Error()})
				}
			}
		case <-clicked(t.browserItem):
			if t.enableWebUI() {
				if err := openBrowser(t.server.URL()); err != nil {
					logger.Warn("tray", fmt.Sprintf("Failed to open WebUI in browser: %v", err))
				}
			}
		case <-t.toggleItem.ClickedCh:
			if t.server.IsWebUIEnabled() {
				if t.uiClient != nil {
					t.uiClient.Close()
				}
				t.server.SetWebUIEnabled(false)
				if !t.server.IsJSONEnabled() {
					t.server.Stop()
				}
				t.setTitle(t.toggleItem, "Enable web server")
			} else {
				t.server.SetWebUIEnabled(true)
				if !t.server.IsRunning() {
					_, _ = t.server.Start(config.Get().GetPort())
				}
				if t.server.IsRunning() {
					t.setTitle(t.toggleItem, "Disable web server")
				}
			}
		case <-t.quitItem.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (t *Tray) enableWebUI() bool {
	t.server.SetWebUIEnabled(true)
	if !t.server.IsRunning() {
		if _, err := t.server.Start(config.Get().GetPort()); err != nil {
			logger.Warn("tray", fmt.Sprintf("Failed to start WebUI server: %v", err))
			return false
		}
	}
	if t.toggleItem != nil {
		t.setTitle(t.toggleItem, "Disable web server")
	}
	return true
}

var privacyMinutes = [...]int{5, 15, 30, 60}

func clicked(item *systray.MenuItem) <-chan struct{} {
	if item == nil {
		return nil
	}
	return item.ClickedCh
}

func privacyTitle(status privacy.Status) string {
	if !status.Paused {
		return "Privacy Pause"
	}
	return fmt.Sprintf("Privacy paused: %02d:%02d remaining", status.RemainingSeconds/60, status.RemainingSeconds%60)
}

func (t *Tray) pausePrivacy(duration time.Duration) {
	t.server.PausePrivacy(duration)
	t.refreshPrivacy()
}

func (t *Tray) refreshPrivacy() {
	if t.privacyItem == nil || t.resumeItem == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	status := privacy.Default.Status()
	title := privacyTitle(status)
	if title == t.privacyLabel {
		return
	}
	wasPaused := t.privacyLabel != "" && t.privacyLabel != "Privacy Pause"
	t.privacyLabel = title
	t.privacyItem.SetTitle(title)
	if status.Paused {
		if !wasPaused {
			t.resumeItem.Enable()
			t.resumeItem.Show()
		}
		systray.SetTooltip("Satellite Agent - " + title)
	} else {
		t.resumeItem.Disable()
		t.resumeItem.Hide()
		systray.SetTooltip("Satellite Agent - Home Assistant")
	}
}

func (t *Tray) setTitle(item *systray.MenuItem, title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.titles == nil {
		t.titles = make(map[*systray.MenuItem]string)
	}
	if t.titles[item] == title {
		return
	}
	t.titles[item] = title
	item.SetTitle(title)
}

func (t *Tray) manualCheckUpdate() {
	if t.updaterInstance == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	status, err := t.updaterInstance.CheckAndInstall(ctx)
	if err != nil {
		_ = showToast(toast.Notification{
			Title:   "Satellite Update",
			Message: fmt.Sprintf("Update check failed: %v", err),
		})
		return
	}

	if status.Available {
		_ = showToast(toast.Notification{
			Title:   "Satellite Update Available",
			Message: fmt.Sprintf("Version %s installer opened. Follow the installer to finish updating.", status.LatestVersion),
			URL:     status.ReleaseURL,
		})
	} else {
		_ = showToast(toast.Notification{
			Title:   "Satellite Up to Date",
			Message: fmt.Sprintf("Satellite is up to date (%s).", config.Version),
		})
	}
}
