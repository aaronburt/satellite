# Documentation

Satellite turns Windows, Linux, and Android devices into Home Assistant sensors
and controls. MQTT provides discovery, state reporting, and remote commands;
webhooks provide an alternative destination for telemetry.

## Start here

| Task | Guide |
| --- | --- |
| Find current priorities and planned work | [GitHub issues](https://github.com/aaronburt/satellite/issues) |
| Install, build, or configure the desktop agent | [Windows and Linux](../windows/README.md) |
| Install, build, or configure the Android app | [Android](../android/README.md) |
| Understand components and runtime behavior | [Architecture](architecture.md) |
| Access the desktop HTTP endpoints from scripts or Go | [HTTP API reference](http-api.md) |
| Integrate with MQTT or consume webhook payloads | [MQTT and webhook reference](mqtt-and-webhooks.md) |
| Diagnose connection, discovery, or reporting problems | [Troubleshooting](troubleshooting.md) |
| Develop, test, or release a change | [Contributing](../CONTRIBUTING.md) |
| Run Android device and integration tests | [Android testing](../android/docs/testing.md) |
| Start a development broker and Home Assistant | [Docker test environment](../android/docker/README.md) |
| Report a vulnerability privately | [Security policy](../SECURITY.md) |

## Planning and research

The Android [development plan](../android/docs/development-plan.md),
[research notes](../android/docs/research.md), [sensor catalog](../android/docs/sensors.md),
and [handoff notes](../android/TODO.md) contain historical decisions and proposed
features. They are background material, not a list of everything currently
implemented. Use the app guides and source code to establish current behavior.

Update the relevant guide alongside changes to configuration, topics, payloads,
commands, build requirements, or release behavior. The repository is licensed
under the [MIT License](../LICENSE).
