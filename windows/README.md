# Satellite

Lightweight Windows & Linux telemetry agent and remote controller for Home Assistant over MQTT.

---

## Screenshots

The local web UI (`satellite v0.23.0`) lets you choose which sensors are published and shows what the agent is reporting:

| Processes | Performance | Settings |
| --- | --- | --- |
| ![Processes view](docs/screenshots/processes.jpg) | ![Performance view](docs/screenshots/performance.jpg) | ![Settings view](docs/screenshots/settings.jpg) |

## Features

- **Telemetry:** CPU, GPU (Core Load, VRAM, Temperature, Power), RAM, Disk, Network, Battery, Uptime, Local IP, Wi-Fi SSID & Signal Strength.
- **Sensors:** Workstation Lock, User Presence/Idle, Microphone Active, Webcam Active, Display Power State, Audio Output Device, Fullscreen/Gaming, System Dark/Light Theme, Pending Reboot.
- **Media Tracking:** Now Playing track title, artist, and app via Windows Media Transport Controls or Linux MPRIS D-Bus.
- **Remote Controls:** Workstation Lock, Sleep Displays, Play/Pause, Next/Prev Track, Mute, Volume Up/Down.
- **Desktop Notifications:** Native Windows 10/11 toast alerts and Linux FreeDesktop D-Bus notifications with titles, messages, clickable URLs, and silent chime option.
- **Home Assistant:** Instant MQTT Auto-Discovery for sensors, media player, and buttons. Only sensors supported by the host OS/hardware are discovered.
- **Generic Webhook:** Optionally POST every telemetry snapshot as JSON to any HTTP endpoint, alongside (or instead of) MQTT.
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
Right-click the tray icon and select **Open Web UI** (or launch Satellite in debug/headless mode to configure via browser) to connect your MQTT broker, toggle exposed sensors, configure the `/json` snapshot endpoint, set up the telemetry webhook, or enable webhook log pushing.

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
 
Send native Windows toast alerts from Home Assistant by publishing JSON to:
- **Targeted Machine**: `satellite/<node_id>/notify` (e.g. `satellite/my-pc/notify`)
- **All Machines (Broadcast)**: `satellite/all/notify` (delivered to all connected Satellite agents simultaneously)
 
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
 
### Home Assistant Automation Examples

**Broadcast to All PCs:**
```yaml
action: mqtt.publish
data:
  topic: satellite/all/notify
  payload: >
    {
      "title": "Doorbell",
      "message": "Someone is at the front door.",
      "url": "https://homeassistant.local:8123/dashboard-cameras"
    }
```

**Targeted to a Specific PC:**
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

## Telemetry Webhook

Satellite can also send its data to anything that accepts an HTTP POST, such as Node-RED, n8n, a Home Assistant webhook trigger, or your own server. It runs alongside MQTT, so Home Assistant keeps working as before. Leave the MQTT broker blank if you only want the webhook.

Enable it in the Web UI under **Telemetry Webhook**, or in `config.json`:

```json
"telemetry_webhook": {
  "enabled": true,
  "url": "https://example.com/satellite",
  "bearer_token": "optional-secret",
  "min_interval_sec": 0
}
```

- A snapshot is sent when a value changes noticeably, and at least once per heartbeat (`heartbeat_sec`, default 60s).
- `min_interval_sec` sets a minimum gap between posts (0 means post on every change).
- If `bearer_token` is set, it is sent as `Authorization: Bearer <token>`.
- Any 2xx response counts as success. Failures are logged and retried on the next change or heartbeat.

The body uses the same envelope as the Android app, so one endpoint can receive both:

```json
{
  "device_id": "my-pc",
  "device_name": "MY-PC",
  "platform": "windows",
  "app_version": "0.26.0",
  "timestamp": 1761259200000,
  "state": { "cpu_percent": 12.5, "memory_percent": 48.1, "timestamp": 1761259200 }
}
```

`state` holds the same fields as the MQTT state topic and the `/json` endpoint. Only sensors you have enabled are included. The outer `timestamp` is in milliseconds, and `state.timestamp` is in seconds.

The webhook only sends data out. Remote commands such as lock and media control still need MQTT.

---

## Disclaimer

This project was built with AI assistance and reviewed by a human.
