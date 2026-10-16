# SatelliteAndroid — Development test environment

A local MQTT broker (**Mosquitto**) and **Home Assistant** for developing and
validating the app's MQTT discovery behavior. Nothing here talks to the cloud.

```
docker/
  compose.yaml                      # Mosquitto + Home Assistant
  mosquitto/config/mosquitto.conf   # Broker config (anonymous access, dev only)
  homeassistant/config/             # HA config dir (only configuration.yaml is committed)
  samples/                          # Example discovery payloads for smoke tests
```

## Prerequisites

- Docker Desktop (WSL2 backend) running.
- Ports `1883` (MQTT) and `8123` (Home Assistant) free.

## 1. Start

From this directory:

```powershell
docker compose up -d
```

First start pulls the images (Home Assistant is ~1 GB) and takes a few minutes.
Watch progress with:

```powershell
docker compose ps
docker compose logs -f homeassistant
```

Wait until <http://localhost:8123> serves the onboarding wizard (first boot
takes 2–5 minutes). The committed HA config sets the log level to `warning`,
so routine startup INFO lines are hidden — check the web UI rather than the
logs.

## 2. Onboard Home Assistant (once)

1. Create the owner account in the browser wizard.
2. **Settings → Devices & services → Add integration → MQTT**.
3. Broker: `mosquitto`, port: `1883`, username: `satellite`, password: `satellite`.
4. Submit. Discovery is enabled by default.

> Broker configuration via `configuration.yaml` was removed from Home Assistant
> (deprecated in 2022.3), so the integration is added through the UI once and
> stored in `homeassistant/config/.storage`.

## 3. Smoke test: does discovery work?

All commands run against the broker inside the container, so no host MQTT
client is needed. Run them from this directory.

**Watch traffic** (leave running in a second terminal):

```powershell
docker compose exec mosquitto mosquitto_sub -h localhost -u satellite -P satellite -v -t "homeassistant/#" -t "satellite/#"
```

**Simulate the Home Assistant birth message** (this is what the app will
subscribe to and republish discovery on):

```powershell
docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t "homeassistant/status" -m "online"
```

**Publish the sample device** (retained discovery + availability + states):

```powershell
docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t "homeassistant/device/satellite_sample01/config" -r -f /samples/sample-device-discovery.json
docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t "satellite/sample01/availability" -r -m "online"
docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t "satellite/sample01/state/battery" -r -m "87"
docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t "satellite/sample01/state/charging" -r -m "ON"
```

**Verify in HA:** Settings → Devices & services → MQTT. A device
**Sample Satellite** should appear with a Battery sensor (87 %) and a Charging
binary sensor.

**Remove the sample device** (a zero-length payload deletes a discovered
component / device):

```powershell
docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t "homeassistant/device/satellite_sample01/config" -r -n
docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t "satellite/sample01/availability" -r -n
```

> PowerShell drops `-m ""` before it reaches the container, so use `-n`
> (zero-length message) to clear retained topics.

## 4. Useful commands

| Task | Command |
|---|---|
| Broker health | `docker compose ps` |
| Live broker log | `docker compose logs -f mosquitto` |
| HA log | `docker compose logs -f homeassistant` |
| Publish a message | `docker compose exec mosquitto mosquitto_pub -h localhost -u satellite -P satellite -t <topic> -m <payload>` |
| Subscribe to everything | `docker compose exec mosquitto mosquitto_sub -h localhost -u satellite -P satellite -v -t "#"` |
| Restart everything | `docker compose restart` |
| Stop (keep data) | `docker compose down` |
| Full reset | `docker compose down` then delete `mosquitto/data/*`, `mosquitto/log/*`, and everything under `homeassistant/config/` except `configuration.yaml` |

## 5. Broker credentials

The broker requires authentication (`allow_anonymous false`). The password file
is `mosquitto/data/passwd` (gitignored, on the writable data volume).

Default development account:

| Username | Password |
|---|---|
| `satellite` | `satellite` |

Add or change users:

```powershell
# Add or update a user (no -c, so existing users are kept)
docker compose exec mosquitto mosquitto_passwd -b /mosquitto/data/passwd <user> <password>
# Change the default user's password
docker compose exec mosquitto mosquitto_passwd -b /mosquitto/data/passwd satellite <newpassword>
docker compose restart mosquitto
```

> If you change the password, also update the broker healthcheck in
> `compose.yaml` and the credentials you enter in Home Assistant (and later in
> the app).

## 6. TLS (optional, later milestone)

The broker config has a commented `8883` listener block. Generate a local CA +
server certificate (any `openssl`/`mkcert` workflow), place them in
`mosquitto/config/certs/`, uncomment the block, and restart. The app's TLS
implementation will be tested against this in milestone M1/M5.

## Troubleshooting

- **`Connection Refused: not authorised`** — missing or wrong credentials. All
  broker commands need `-u satellite -P satellite`, and Home Assistant needs the
  same username/password in its MQTT integration.
- **`path ... is not shared from the host`** — Docker Desktop 29+ requires
  explicit file sharing: **Settings → Resources → File sharing → add the
  repository folder**, then restart Docker Desktop.
- **HA starts but no MQTT integration** — it must be added through the UI
  (step 2); it is intentionally not in `configuration.yaml`.
- **Port already in use** — stop the conflicting service or change the port
  mapping in `compose.yaml`.
- **Broker unhealthy** — `docker compose logs mosquitto`; most commonly a bad
  config file or a leftover process on 1883.
- **HA web UI unreachable** — first boot can take several minutes on slower
  machines; check `docker compose logs -f homeassistant`.
