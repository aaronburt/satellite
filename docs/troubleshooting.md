# Troubleshooting

Start by identifying the app, version, transport, and affected device ID. Verify
traffic at the broker or webhook receiver as well as the app's displayed status.

## Cannot connect to the broker

1. Confirm the broker is running and reachable from the device. `localhost` on a
   phone refers to the phone. The standard Android emulator reaches the host
   through `10.0.2.2`; a physical phone needs the broker's LAN address.
2. Check the port, TLS selection, username, password, and broker access rules.
   Desktop accepts broker URLs such as `tcp://broker.example:1883` or
   `tls://broker.example:8883`. Android has separate host, port, and TLS fields.
3. Check the broker logs for authentication, authorization, or duplicate client-ID
   errors. Give each desktop installation a distinct node/client ID.
4. Check name resolution from the same environment that runs the app. An agent
   sandbox may have different network permissions from an ordinary terminal.

Desktop TLS verifies certificates by default. `insecure_tls` disables verification;
use the correct hostname and a certificate trusted by the client instead of
treating that setting as a general connectivity fix.

## Connected, but Home Assistant has no entities

- Confirm Home Assistant's MQTT integration uses the same broker and listens to
  the `homeassistant` discovery prefix.
- Check retained discovery messages using a broker client. Desktop sends
  per-entity configs; Android sends a single device config. See the
  [topic reference](mqtt-and-webhooks.md).
- On desktop, check sensor toggles and capability detection. Unsupported or
  disabled sensors are intentionally omitted/removed.
- For Android controls, enable the entity in the app first. MQTT commands are
  ignored while the corresponding control is disabled.
- Desktop sends discovery when it connects; restart the agent to resend it.
  Android also refreshes discovery on Home Assistant's `online` birth message.

## Android reconnects but remains unavailable

Builds from current `main` publish retained `online` on every successful
connect, including automatic reconnects, and keep retrying a broker that was
unreachable at start-up. The published v0.26.0 release predates that fix: on
it, a reconnect after a connection loss could leave the retained `offline` Last
Will in place. On v0.26.0, pause and resume reporting as a workaround, or
install a newer build.

To verify, watch `satellite/<device_id>/availability` with a broker client,
restart the broker or interrupt the network, then restore it. Availability
should return to `online` without touching the app. If it does not, check the
app version and the broker logs.

## Reporting appears successful but values do not arrive

On current `main`, the Android MQTT last-published timestamp and test result
update only when the broker accepted every publish. v0.26.0 could report
success after a rejected publish. Either way, acceptance by the broker is not
proof Home Assistant processed the values: check received state topics.
Webhook reporting updates its timestamp after an HTTP 2xx response.

For desktop, unchanged values can be suppressed until the next heartbeat. Check
`interval_sec`, `heartbeat_sec`, and the telemetry webhook's `min_interval_sec`.
A saved `interval_sec` applies to the running collection timer straight away;
the next collection follows one full new interval after the save.

## Desktop web UI or `/json` cannot be opened

See the [HTTP API reference](http-api.md) for endpoint paths, credentials, and
request examples.

- On Windows, enable the web UI from the tray menu, then use **Open WebUI**.
  On Linux, launch `./satellite --debug` for setup. `--headless` does not enable
  the management UI.
- Use the URL Satellite provides. The default port is dynamic. A requested port
  that cannot be bound may fall back to an available port.
- An HTTP 401 on the web UI usually means the token is missing or expired.
  Reopen it from the tray or use the URL from the current debug session.
- `/json` must be enabled separately. It uses `X-API-Key: <api_key>` or the
  `api_key` query parameter, rather than the web UI's ephemeral token.
- The default bind address is `127.0.0.1`. A remote machine cannot reach that
  listener. Configuring a LAN bind also changes who can reach the HTTP service;
  retain authentication and restrict network access accordingly.

`/json` returns the last collected snapshot, not a fresh OS probe on each request.

## Android readings or controls are missing

Check the app's Permissions screen and grant the permission for the affected
reading. Wi-Fi and carrier data depend on the Android version and permissions.
Some OEMs restrict microphone muting; an unsupported control is disabled in the
UI. Use the observed device state to verify a command took effect.

If background reporting stops, check the foreground notification, reporting
switch, and device battery/background restrictions. Test backgrounding, network
changes, and reboot on the actual target device; see
[Android testing](../android/docs/testing.md).

## Android APK update will not install

An installed app needs an update with the same application ID and signing key.
Locally built APKs can be signed with a development key; GitHub release APKs use
the release key. Use a consistently signed build stream. Uninstalling to change
signing keys removes app data, so account for saved settings first.

## Development checks fail

Dependency downloads need network access and writable Go/Gradle caches. Desktop
integration tests also open loopback sockets, and some Windows tests call native
notification APIs. Permission errors or unavailable desktop services must be
distinguished from assertion failures in ordinary application logic.

Use the checks in [Contributing](../CONTRIBUTING.md) and report the exact failing
command and error. Include OS/app versions, transport, steps to reproduce, and
sanitized logs. Remove passwords, tokens, and token-bearing UI URLs. Report
security issues through the [private security process](../SECURITY.md).
