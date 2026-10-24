# Satellite

Lightweight Windows & Linux telemetry agent and remote controller for Home Assistant over MQTT.

[Documentation index](../docs/README.md) | [Architecture](../docs/architecture.md) |
[Protocol reference](../docs/mqtt-and-webhooks.md) | [Troubleshooting](../docs/troubleshooting.md)

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
- **Generic Webhook:** Optionally POST telemetry snapshots as JSON on changes or heartbeats, alongside (or instead of) MQTT.
- **Tray, Web UI & Headless:** Windows system tray, local Web UI for setup, and headless operation (`--headless`) including Linux systemd user services. Linux has no tray implementation.

---

## Quick Start

### 1. Download or build

Download `satellite.exe` or the Windows installer, or `satellite-linux-amd64`, from
[GitHub Releases](https://github.com/aaronburt/satellite/releases).
For the downloaded Linux binary, make it executable and rename it before using
the commands below:

```sh
chmod +x satellite-linux-amd64
mv satellite-linux-amd64 satellite
```

To build from source, install the Go version declared in [`go.mod`](go.mod) and run
these commands from `windows/`:

- **Windows Binary:**
  ```powershell
  go build -ldflags="-H windowsgui -s -w" -o satellite.exe .
  ```
- **Linux Binary:**
  ```bash
  go build -ldflags="-s -w" -o satellite .
  ```
- **Windows Installer (Inno Setup):**
  ```powershell
  & "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe" installer.iss
  ```

### 2. Run

- **Tray Mode (Windows):** Run `satellite.exe`. Enable the WebUI from its tray menu, then choose **Open WebUI**.
- **Headless Mode (Linux Server / Background Service):** Run `./satellite --headless`.
- **Setup on Linux:** Run `./satellite --debug` and open the URL printed in the terminal. Stop it after saving, then launch normally or as a service.
- **CLI Mode:** Run `./satellite --cli` for a local terminal dashboard. CLI mode does not start MQTT or webhook reporting.
- **Systemd User Service (Linux):**
  ```bash
  install -Dm755 satellite ~/.local/bin/satellite
  mkdir -p ~/.config/systemd/user
  cp satellite.service ~/.config/systemd/user/
  systemctl --user daemon-reload
  systemctl --user enable --now satellite
  ```

### 3. Configure

Enable the WebUI from the Windows tray menu and select **Open WebUI** (or launch `--debug` for browser setup on Linux) to connect your MQTT broker, toggle exposed sensors, configure the `/json` snapshot endpoint, set up the telemetry webhook, or enable webhook log pushing.

The WebUI automatically probes your operating system and hardware on boot and only displays sensor toggles that are actually supported on your current machine.

Config is saved at:

- **Windows:** `%APPDATA%\satellite\config.json`
- **Linux:** `~/.config/satellite/config.json`

---

## Configuration reference

The default listener is `127.0.0.1` with a dynamically selected port. Use the URL
from the tray or debug output. The WebUI uses an ephemeral token; the optional
`/json` endpoint instead uses `X-API-Key` or the `api_key` query parameter.

| Setting | Default | Meaning |
| --- | --- | --- |
| `mqtt.broker` | Empty | Broker URL; leave empty for webhook-only operation |
| `mqtt.topic_prefix` | `satellite` | Prefix for state, command, and notification topics |
| `interval_sec` | `5` | Collection interval; restart the process after changing it |
| `heartbeat_sec` | `60` | Publish even without a significant change, checked on collection ticks |
| `bind_address` | `127.0.0.1` | HTTP listener address |
| `port` / `webui_port` | `0` | HTTP port; zero selects an available port |
| `json_enabled` | `false` | Enable the last-snapshot JSON endpoint |
| `check_updates` | `false` | Enable background GitHub release checks |
| `telemetry_webhook.enabled` | `false` | Send state snapshots to an HTTP endpoint |
| `webhook.enabled` | `false` | Send application log events to an HTTP endpoint |

CPU, memory, and network telemetry are enabled by default. Other sensors,
notifications, and remote controls are opt-in and depend on platform support.
The full configuration model is in [`config.go`](internal/config/config.go).

Configuration can also be exported and imported. With the agent stopped, run:

```powershell
.\satellite.exe --export satellite-config.json
.\satellite.exe --import satellite-config.json
```

Exports may contain credentials. Supply `--password <passphrase>` on both commands
for an encrypted export/import; the passphrase is supplied as a command-line
argument. Import adapts node/client identity to the current hostname by default;
use `--reset-node-id=false` to retain the imported identity. Restart the agent
after importing. Linux uses `./satellite` with the same flags.

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
| `media_stop` | Stop media playback |
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

The body shares common envelope fields with Android, but metadata and state value types differ. See the [schema comparison](../docs/mqtt-and-webhooks.md#telemetry-webhooks) before handling both in one receiver:

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
