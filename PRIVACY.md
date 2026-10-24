# Privacy statement

Last reviewed: 24 October 2026. Applies to Satellite for Windows, Linux, and Android.

I do not collect or share your device data through Satellite. The apps do not send
your readings, credentials, configuration, or logs to me. There is no Satellite
account, developer collection server, advertising, usage analytics, or automatic
crash reporting built into the apps.

You choose whether to report information and which MQTT broker or webhook receives
it. Satellite does not send a second copy to its developer.

## Where your information goes

Satellite reads device state to show it locally and report it to destinations you
configure. The word "telemetry" in the app means these device readings.

- **MQTT:** readings, device discovery information, and online/offline status go
  to the broker you enter. Home Assistant and other clients with access to the
  relevant topics can receive them.
- **Webhooks:** readings go to the URL you enter. Desktop also has a separate,
  optional log webhook. Logs can include device names, connection details,
  notification contents, and remote commands. Both desktop webhooks are off by
  default. Android uses either MQTT or a webhook; desktop can use both.
- **Test buttons:** testing a connection or webhook contacts the destination you
  specify. A telemetry test sends real device readings.

Choosing a hosted broker or webhook means sharing information with its operator.
Its access controls, storage, and any onward sharing are outside Satellite's
control. Use TLS/HTTPS where available: unencrypted connections can expose
readings and credentials to others on the network.

## What you control and what can be shared

**Windows and Linux:** no broker or webhook destination is configured initially.
CPU, memory, and network counters are enabled locally by default. Other sensor
and remote-control switches are off. Depending on the switches and platform,
readings can include storage and GPU statistics, battery state, local IP and
Wi-Fi name, presence and idle time, session and display state, process names,
exact window titles, media titles and artists, and microphone/webcam usage.
Snapshots also include a timestamp and whether a battery is present. MQTT
discovery includes the configured device ID and app version. The default ID is
based on your hostname; desktop telemetry webhooks include your actual hostname
even if you change that ID.

**Android:** no broker or webhook is configured initially, but reporting is on by
default and starts sending once a working destination is saved. Read-only
sensors are reported as a group, without individual sensor switches: battery and
charging, power and screen state, uptime, free storage, network type, ringer mode,
and software versions. Wi-Fi name and carrier name require permission; without
it, the app sends a status placeholder. Reports also identify the installation
using a locally generated random ID, device name, manufacturer, and model.
Optional media-volume and microphone-mute controls, including their reported
states, are off until enabled. You can pause reporting and revoke permissions.

The microphone and webcam features do not record or upload audio or video.
Android's location permission is used to read the Wi-Fi network name on older
Android versions; Satellite does not read GPS coordinates.

The desktop web interface is served by the app and listens on your own computer
by default. The separate JSON endpoint is off by default. Changing the listening
address can make these interfaces reachable from other devices; access requires
the relevant token or API key.

Enabling remote controls or notifications also gives clients allowed to publish
to the relevant MQTT topics the ability to trigger those features. On Windows,
notifications containing a web link can open that link automatically in your
default browser. This includes enabled MQTT notifications and notifications
triggered by manual or enabled automatic update checks.

## Update checks and GitHub

Satellite does not poll GitHub for updates by default.

- **Windows and Linux:** automatic checks are off unless you enable them. A
  manual check still contacts GitHub while automatic checks are off.
- **Android:** checking starts from the Check for updates button in Settings.
  If a newer version is found, that action also downloads its APK. Installation
  requires a separate confirmation. There is no scheduled background check.

Checks request release information from GitHub. GitHub and services handling
downloads or opened web links receive ordinary connection information, including
your connection's IP address and request headers. The desktop update request
identifies Satellite and its version; Android identifies itself as
`SatelliteAndroid`. Release checks do not include your sensor readings,
credentials, or installation ID.

## Local storage and deletion

Desktop settings are saved in `satellite/config.json` under your operating
system's user configuration directory. This includes credentials in plaintext.
Exported settings also contain credentials; exports are encrypted only if you
provide a passphrase.

Android keeps settings in app-private storage and encrypts saved broker passwords
and webhook tokens using Android Keystore. Older plaintext secrets are migrated
on opening the settings store. Android app backup is disabled. Downloaded update
APKs are also kept in app-private storage.

Satellite does not keep a persistent history of sensor readings on disk. It
holds current readings and pending webhook deliveries in memory. Brokers, Home
Assistant, and webhook receivers can retain their own copies, including retained
MQTT messages and history. Pausing reporting or uninstalling Satellite does not
erase those copies.

To remove local settings, stop the desktop agent and delete its configuration
directory, or clear Android's app storage/uninstall the app. Delete any exports
and destination-side records separately.

If you choose to send me a bug report, logs, or screenshots, I receive what you
submit through that channel. That is a separate, deliberate action; check its
contents before sharing.

## Checking this statement

The [implementation review](docs/privacy-review.md) links these claims to the
source code and records what was checked and the limits of that verification.
Report privacy-related vulnerabilities through the [security policy](SECURITY.md).
