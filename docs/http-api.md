# Desktop HTTP API

The Windows and Linux Go agent exposes a local HTTP API for status,
configuration, actions, connection tests, updates, and logs. The Android app
does not expose these HTTP endpoints. Routes and handlers are implemented in
[`server.go`](../windows/internal/webui/server.go).

## Connect and authenticate

1. On Windows, enable **WebUI** in the Satellite tray menu, then select
   **Open WebUI**. Alternatively, start `satellite.exe --debug`. On Linux, start
   `./satellite --debug`; `--headless` alone does not enable the management API.
2. Use the address printed in debug output or opened by the tray menu, for
   example `http://127.0.0.1:12345/?token=YOUR_WEB_UI_TOKEN`. The default bind
   address is `127.0.0.1`, and port `0` selects an available port. Replace the
   example port with the actual port from your running agent.
3. Send the URL's `token` value as the `X-API-Token` header on every `/api/*`
   request. These routes are available only while WebUI is enabled. If the
   listener remains running with WebUI disabled, they return HTTP 404.

The token is temporary. Stopping the HTTP server or restarting Satellite
invalidates it; reopen the WebUI to obtain the current address and token.
The API also accepts `?token=YOUR_WEB_UI_TOKEN`, which is useful for browser
clients such as `EventSource`. Prefer the header for scripts so the token stays
out of request URLs. Keep tokens and token-bearing URLs out of shared logs.

| Endpoint family | Header | Query alternative | Availability |
| --- | --- | --- | --- |
| `/api/*` | `X-API-Token: <WebUI token>` | `token=<WebUI token>` | WebUI enabled |
| `/json` | `X-API-Key: <configured API key>` | `api_key=<configured API key>` | JSON endpoint enabled |

The `/json` API key is a separate, persistent configuration value. It does not
authenticate `/api/*`. The default loopback listener is reachable only from the
same machine; a LAN bind changes who can connect to these management endpoints.

### PowerShell quick start

Replace both placeholders with values from your current WebUI URL:

```powershell
$base = 'http://127.0.0.1:12345'
$headers = @{ 'X-API-Token' = 'YOUR_WEB_UI_TOKEN' }

$status = Invoke-RestMethod -Uri "$base/api/status" -Headers $headers
$status.mqtt_status
$status.telemetry

Invoke-RestMethod -Uri "$base/api/capabilities" -Headers $headers
Invoke-RestMethod -Uri "$base/api/config" -Headers $headers
```

With curl on Linux/macOS:

```sh
SATELLITE_URL='http://127.0.0.1:12345'
SATELLITE_TOKEN='YOUR_WEB_UI_TOKEN'
curl --silent --show-error --fail-with-body \
  -H "X-API-Token: $SATELLITE_TOKEN" "$SATELLITE_URL/api/status"
```

## Endpoint overview

Use the following HTTP methods. JSON request bodies should use
`Content-Type: application/json`. `/api/*` request bodies are limited to 1 MiB.

| Method | Path | Request body | Result |
| --- | --- | --- | --- |
| GET | `/api/status` | None | MQTT status, last telemetry snapshot, update status |
| GET | `/api/capabilities` | None | OS name and supported-feature booleans |
| GET | `/api/config` | None | Current configuration, with selected secrets blanked |
| POST | `/api/config` | Partial configuration object | Save and apply configuration |
| POST | `/api/config/export` | `{"passphrase":"optional passphrase"}` or `{}` | Download a `.satconfig` file |
| POST | `/api/config/import` | JSON import object or multipart form | Import and apply configuration |
| POST | `/api/action` | `{"action":"test_toast"}` | Execute a local action |
| POST | `/api/test-connection` | MQTT settings object | Test a broker connection |
| POST | `/api/test-webhook` | Log webhook settings object | Send a test log event |
| POST | `/api/test-telemetry-webhook` | Telemetry webhook settings object | Send a live telemetry snapshot |
| GET | `/api/update` | None | Current update-check status |
| POST | `/api/update/check` | None | Check GitHub for a release now |
| GET | `/api/logs/stream` | None | Server-sent event stream of new logs |
| GET | `/json` | None | Last telemetry snapshot; uses `X-API-Key` |

## Status and capabilities

`GET /api/status` returns an object with:

| Field | Meaning |
| --- | --- |
| `mqtt_status` | `Disconnected`, `Connecting`, or `Connected` |
| `telemetry` | Last collected snapshot; fields depend on enabled sensors and platform support |
| `update_status` | Update-check status, or `null` if no checker is configured |

Snapshot fields, including `cpu_percent`, `memory_percent`, `has_battery`, and
the Unix-seconds `timestamp`, are defined in
[`types.go`](../windows/internal/telemetry/types.go). This endpoint reads the
last snapshot rather than triggering a fresh collection.

`GET /api/capabilities` returns `os` and feature flags such as `cpu`, `battery`,
`media_control`, and `notifications`. These describe platform support, not
whether each feature is enabled in configuration. The complete fields are in
[`capabilities.go`](../windows/internal/capabilities/capabilities.go).

## Read and save configuration

`GET /api/config` returns the configuration model from
[`config.go`](../windows/internal/config/config.go). The saved MQTT password,
telemetry webhook bearer token, and log webhook secret are returned as empty
strings. The `/json` API key remains present, so treat the response as sensitive.

`POST /api/config` accepts a partial object. Omitted fields retain their saved
values, including omitted fields inside nested objects. Empty strings for the
three secrets listed above also retain their saved values, so an empty string
never clears a secret.

To clear a saved secret, list it in `clear_secrets`. The allowed names are
`mqtt.password`, `telemetry_webhook.bearer_token`, and `webhook.secret`;
listing a name twice is harmless. `clear_secrets` can be combined with other
changes in the same request:

```sh
curl --silent --show-error --fail-with-body -X POST \
  -H "X-API-Token: $SATELLITE_TOKEN" -H 'Content-Type: application/json' \
  -d '{"clear_secrets":["mqtt.password","webhook.secret"]}' \
  "$SATELLITE_URL/api/config"
```

The whole request is checked before anything is saved. It is rejected with
`400` and nothing changes when `clear_secrets` is not a list of strings, names
an unknown secret, or names a secret that the same request also sets to a
non-empty value. Error messages name the field but never include secret values.

For example, enable background update checks and set the MQTT heartbeat:

```powershell
$body = @{ check_updates = $true; heartbeat_sec = 90 } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "$base/api/config" -Headers $headers `
    -ContentType 'application/json' -Body $body
```

The normal response is `{"success":true}`. Saving also reapplies the running
settings and restarts MQTT. Changing `interval_sec` still requires restarting
the process to change its collection timer.

Changing `bind_address`, `port`, or `webui_port` can move the listener. For a port
change, supply the same desired value for both `port` and `webui_port`. A move
adds a `listen` object to the response, for example:

```json
{
  "success": true,
  "listen": { "moved": true, "host": "127.0.0.1", "port": 12346 }
}
```

Use the returned host and port for subsequent requests. A listener move retains
the current token. Invalid or unavailable listen settings return HTTP 400.

## Export and import configuration

Exports include credentials. Supply a nonempty `passphrase` to encrypt the
export; `{}` produces an unencrypted export. The response is an
`application/octet-stream` attachment named `satellite-config-<node_id>.satconfig`.

```powershell
$exportBody = @{ passphrase = 'YOUR_EXPORT_PASSPHRASE' } | ConvertTo-Json
Invoke-WebRequest -Method Post -Uri "$base/api/config/export" -Headers $headers `
    -ContentType 'application/json' -Body $exportBody -OutFile './device.satconfig'
```

Import the exported file as a JSON string in `content`:

```powershell
$importBody = @{
    content = Get-Content -LiteralPath './device.satconfig' -Raw
    passphrase = 'YOUR_EXPORT_PASSPHRASE'
    reset_node_id = $false
} | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "$base/api/config/import" -Headers $headers `
    -ContentType 'application/json' -Body $importBody
```

Alternatively, send a multipart form with `file`, optional `passphrase`, and
optional `reset_node_id` fields. `reset_node_id` defaults to `true`, adapting the
imported node/client identity to the current machine. Set it to `false` to keep
the imported identity, as in the example.

Successful imports return `success`, `source_app_version`, `is_newer_version`,
and `node_id`, plus `listen` if the listener moved. They save and apply the
imported configuration, including restarting MQTT. A missing or incorrect
encryption passphrase returns HTTP 422; invalid import data returns HTTP 400.

## Execute an action

`POST /api/action` accepts an action name in a JSON object:

```powershell
Invoke-RestMethod -Method Post -Uri "$base/api/action" -Headers $headers `
    -ContentType 'application/json' -Body '{"action":"test_toast"}'
```

`test_toast` queues a desktop test notification and returns `{"success":true}`;
that response confirms the request was accepted, not that the OS displayed it.
Other canonical action names are `lock`, `display_sleep`, `media_play_pause`,
`media_play`, `media_pause`, `media_next`, `media_prev`, `media_stop`,
`volume_mute`, `volume_up`, and `volume_down`. They act on the machine running
Satellite, and support depends on its OS and desktop session.

The authenticated HTTP action handler calls the action registry directly; it
does not check the MQTT `expose` switches. Unknown or failed actions return
HTTP 400 with `{"success":false,"error":"..."}`. See
[`actions.go`](../windows/internal/actions/actions.go) for action aliases.

## Test MQTT and webhooks

These endpoints perform a test without saving the supplied settings:

| Path | Example JSON body |
| --- | --- |
| `/api/test-connection` | `{"broker":"tcp://127.0.0.1:1883","username":"satellite","password":"YOUR_BROKER_PASSWORD","client_id":"satellite-api-test","topic_prefix":"satellite","insecure_tls":false}` |
| `/api/test-webhook` | `{"url":"https://example.com/logs","secret":"YOUR_LOG_SECRET"}` |
| `/api/test-telemetry-webhook` | `{"url":"https://example.com/telemetry","bearer_token":"YOUR_WEBHOOK_TOKEN"}` |

For example, test an MQTT connection:

```powershell
$mqttBody = @{
    broker = 'tcp://127.0.0.1:1883'
    username = 'satellite'
    password = 'YOUR_BROKER_PASSWORD'
    client_id = 'satellite-api-test'
    topic_prefix = 'satellite'
    insecure_tls = $false
} | ConvertTo-Json
$result = Invoke-RestMethod -Method Post -Uri "$base/api/test-connection" `
    -Headers $headers -ContentType 'application/json' -Body $mqttBody
if (-not $result.success) { throw $result.error }
```

A blank MQTT password reuses the saved password only when the supplied broker
and username match the saved configuration. For webhook tests, a blank URL
uses the saved URL; a blank secret/token reuses the saved value only when the
resolved URL matches the saved URL. Sending `{}` therefore tests a saved
webhook destination. The telemetry test collects and sends a fresh snapshot.

Tests return HTTP 200 for both success and a connection/delivery failure:
inspect the JSON `success` boolean and `error` string. Invalid JSON returns
HTTP 400. Test webhook requests send real HTTP requests to the chosen destination.

## Check for updates

`GET /api/update` returns cached update information. `POST /api/update/check`
performs a new GitHub release check and returns its result:

```powershell
Invoke-RestMethod -Uri "$base/api/update" -Headers $headers
Invoke-RestMethod -Method Post -Uri "$base/api/update/check" -Headers $headers
```

The normal status fields are `checked_at` (RFC 3339 timestamp), `available`,
`current_version`, `latest_version`, `release_url`, and optional `error`.
Checking does not download or install a desktop update. A failed check returns
HTTP 500 with the status object and its `error` field. Without a configured
checker, GET returns `available: false` and `current_version`; POST returns
HTTP 503. See [`updater.go`](../windows/internal/updater/updater.go).

## Stream logs

`GET /api/logs/stream` returns `Content-Type: text/event-stream`. Each new log
event is a JSON object in an SSE `data:` line with `timestamp` (Unix seconds),
`time` (RFC 3339), `node_id`, `level`, `category`, and `message`. Comment lines
keep the connection alive. The stream sends new events; it has no replay, and
a slow client can miss events.

Use curl's unbuffered mode in PowerShell (press Ctrl+C to stop):

```powershell
curl.exe --no-buffer --header "X-API-Token: $($headers['X-API-Token'])" "$base/api/logs/stream"
```

On Linux/macOS:

```sh
curl --no-buffer -H "X-API-Token: $SATELLITE_TOKEN" \
  "$SATELLITE_URL/api/logs/stream"
```

Browser `EventSource` clients can use
`/api/logs/stream?token=YOUR_WEB_UI_TOKEN` because they cannot set a custom
authentication header.

## Read the separate JSON snapshot endpoint

Enable **JSON endpoint** in WebUI settings (`json_enabled: true`) and obtain its
configured `api_key`. This endpoint can remain available when WebUI is disabled:

```powershell
$jsonHeaders = @{ 'X-API-Key' = 'YOUR_CONFIGURED_API_KEY' }
Invoke-RestMethod -Uri "$base/json" -Headers $jsonHeaders
```

`GET /json` returns the snapshot directly, without the `mqtt_status` or
`update_status` wrapper from `/api/status`. `/json/` is also accepted. The
snapshot is the last collected value. A disabled endpoint returns HTTP 404;
an invalid or missing API key returns HTTP 401. The `api_key` query parameter
is also accepted and takes precedence over the header when nonempty.

## Go client example

This standalone program reads status and configuration using the standard
library. Save it as `api-example.go`. Using the PowerShell quick-start variables,
run it with:

```powershell
$env:SATELLITE_URL = $base
$env:SATELLITE_TOKEN = $headers['X-API-Token']
go run ./api-example.go
```

On Linux/macOS, export the curl example's variables with
`export SATELLITE_URL SATELLITE_TOKEN`, then run `go run ./api-example.go`.
The program prints configuration, including the `/json` API key when configured.

```go
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func callAPI(client *http.Client, method, endpoint, token string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Token", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, data)
	}
	return data, nil
}

func main() {
	base := strings.TrimRight(os.Getenv("SATELLITE_URL"), "/")
	token := os.Getenv("SATELLITE_TOKEN")
	if base == "" || token == "" {
		log.Fatal("Set SATELLITE_URL and SATELLITE_TOKEN from the current WebUI URL")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	for _, path := range []string{"/api/status", "/api/config"} {
		data, err := callAPI(client, http.MethodGet, base+path, token, nil)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %s\n", path, data)
	}
}
```

To save a setting instead, make this call inside `main` using the same client:

```go
body := strings.NewReader(`{"check_updates":true}`)
data, err := callAPI(client, http.MethodPost, base+"/api/config", token, body)
if err != nil {
	log.Fatal(err)
}
fmt.Println(string(data))
```

`callAPI` checks HTTP errors. When using it for the connection-test endpoints,
also decode the response JSON and check `success`, even when HTTP is 200.

## Errors and limits

| HTTP status | Meaning |
| --- | --- |
| 200 | Successful request, or a completed connection/webhook test whose JSON may report failure |
| 400 | Invalid JSON/settings/import data, or an unknown/failed action |
| 401 | Missing or invalid WebUI token or `/json` API key |
| 404 | WebUI/API disabled, JSON endpoint disabled, or unknown route |
| 405 | Unsupported method on handlers that enforce methods |
| 422 | Configuration import requires a passphrase, or decryption failed |
| 500 | Configuration save/export failure, unavailable action registry, or failed update check |
| 503 | Update checker unavailable for a manual check |

Most operation errors return `{"success":false,"error":"..."}`. Authentication
errors return an `error` field without `success`. Update-check failures use the
update-status object. Some method and routing errors are plain text, so inspect
the HTTP status and content type before decoding every response as JSON.
Oversized bodies are capped at 1 MiB; handlers that report a JSON decode failure
return HTTP 400 rather than a uniform HTTP 413.

See [troubleshooting](troubleshooting.md#desktop-web-ui-or-json-cannot-be-opened)
for listener and authentication problems.
