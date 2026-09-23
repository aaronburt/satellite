# Satellite

Lightweight Windows & Linux telemetry agent and remote controller for Home Assistant over MQTT.

---

## Features

- **Telemetry:** CPU, GPU (Core Load, VRAM, Temperature, Power), RAM, Disk, Network, Battery, Uptime, Local IP, Wi-Fi SSID & Signal Strength.
- **Sensors:** Workstation Lock, User Presence/Idle, Microphone Active, Webcam Active, Display Power State, Audio Output Device, Fullscreen/Gaming, System Dark/Light Theme, Pending Reboot.
- **Media Tracking:** Now Playing track title, artist, and app via Windows Media Transport Controls or Linux MPRIS D-Bus.
- **Remote Controls:** Workstation Lock, Sleep Displays, Play/Pause, Next/Prev Track, Mute, Volume Up/Down.
- **Desktop Notifications:** Native Windows 10/11 toast alerts and Linux FreeDesktop D-Bus notifications with titles, messages, clickable URLs, and silent chime option.
- **Home Assistant:** Instant MQTT Auto-Discovery for sensors, media player, and buttons. Only sensors supported by the host OS/hardware are discovered.
- **Tray, Web UI & Headless:** Runs in system tray on desktop, provides a local Web UI for setup, or runs headless (`--headless`) as a background systemd daemon.

---

## Quick Start

### 1. Build
- **Windows Binary:**
  ```powershell
  go build -ldflags="-H windowsgui -s -w" -o satellite.exe .
  ```
- **Linux Binary:**
  ```bash
  go build -s -w -o satellite .
  ```
- **Windows Installer (Inno Setup):**
  ```powershell
  & "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe" installer.iss
  ```

### 2. Run
- **Tray Mode (Windows / Linux Desktop):** Run `satellite.exe` or `./satellite`.
- **Headless Mode (Linux Server / Background Service):** Run `./satellite --headless`.
- **CLI Mode:** Run `./satellite --cli` for interactive terminal dashboard.
- **Systemd User Service (Linux):**
  ```bash
  mkdir -p ~/.config/systemd/user
  cp satellite.service ~/.config/systemd/user/
  systemctl --user daemon-reload
  systemctl --user enable --now satellite
  ```

### 3. Configure
Right-click the tray icon and select **Open Web UI** (or launch Satellite in debug/headless mode to configure via browser) to connect your MQTT broker, toggle exposed sensors, configure the `/json` snapshot endpoint, or enable webhook log pushing.

The WebUI automatically probes your operating system and hardware on boot and only displays sensor toggles that are actually supported on your current machine.

Config is saved at:
- **Windows:** `%APPDATA%\satellite\config.json`
- **Linux:** `~/.config/satellite/config.json`

---

## Remote Commands

Publish plain text or JSON `{"action": "<command>"}` to `satellite/<node_id>/command`:

| Command | Action |
| :--- | :--- |
| `LOCK` | Lock Windows session |
| `display_sleep` | Put displays to sleep (turn off monitors) |
| `media_play_pause` | Toggle media play/pause |
| `media_play` | Play media |
| `media_pause` | Pause media |
| `media_next` | Skip to next track |
| `media_prev` | Skip to previous track |
| `volume_mute` | Toggle master mute |
| `volume_up` | Volume up (+2%) |
| `volume_down` | Volume down (-2%) |
 
---
 
## Toast Notifications
 
Send native Windows toast alerts from Home Assistant by publishing JSON to `satellite/<node_id>/notify`:
 
```json
{
  "title": "Front Door",
  "message": "Motion detected on the porch",
  "url": "https://homeassistant.local:8123/dashboard-cameras",
  "silent": false
}
```
 
- `title` *(optional)*: Notification title. Defaults to the PC hostname if omitted.
- `message` *(required)*: Body text of the notification.
- `url` *(optional)*: Clicking the notification launches this URL in your default browser.
- `silent` *(optional)*: Set to `true` to suppress audio chime.
 
### Home Assistant Automation Example
 
```yaml
action: mqtt.publish
data:
  topic: satellite/my-pc/notify
  payload: >
    {
      "title": "Laundry Done",
      "message": "The washing machine cycle has finished.",
      "url": "https://homeassistant.local:8123/lovelace/appliances"
    }
```


---

## Home Assistant

When connected to your MQTT broker, Home Assistant automatically discovers:
- Telemetry sensors and binary sensors (including Webcam, Display Power, Audio Output, and Wi-Fi).
- `media_player.<node_id>_media_player` entity.
- Quick action buttons (Lock PC, Sleep Displays, Play/Pause, Next, Previous, Mute, Volume).

---

## Disclaimer

This project was built with AI assistance and reviewed by a human.
