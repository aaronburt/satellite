# Project TODO

Active work for the Windows, Linux, and Android apps. Reviewed on 24 October
2026 against `main`, including merged [PR #5](https://github.com/aaronburt/satellite/pull/5).
Items are listed in suggested priority order. Check an item when its completion
criteria are met, and link its PR or verification results.
Each open item links to its GitHub issue for suggestions, completion criteria,
and discussion.

## Next

- [ ] **Fix Android DataStore unit tests on Windows.** ([#6](https://github.com/aaronburt/satellite/issues/6)) Investigate the
  `settings.preferences_pb.tmp` rename failures observed during PR #5 validation.
  Keep the persistence/reopen assertions and make
  `:core:datastore:testDebugUnitTest` pass on Windows and Linux.
  [Tests](android/core/datastore/src/test/kotlin/uk/co/aaronburt/satellite/datastore/DataStoreSettingsRepositoryTest.kt)
- [ ] **Refresh documentation after PR #5.** ([#7](https://github.com/aaronburt/satellite/issues/7)) Update the
  [Android testing guide](android/docs/testing.md) to describe the new module
  tests and shared fakes. Update [troubleshooting](docs/troubleshooting.md) and
  the [protocol reference](docs/mqtt-and-webhooks.md) so fixed reconnect and
  publish-result bugs are not described as current limitations.
- [ ] **Verify reconnect recovery with Home Assistant and a real broker.** ([#8](https://github.com/aaronburt/satellite/issues/8))
  Restart the broker and Home Assistant, interrupt the network, and confirm
  retained `online` availability, restored discovery/state, and one command
  delivery after repeated reconnects. Record broker traffic as well as UI state.
  PR #5 includes the code fixes and unit tests; this item is integration validation.
- [ ] **Verify Android lifecycle and microphone safety on a physical device.** ([#9](https://github.com/aaronburt/satellite/issues/9))
  Exercise reboot, backgrounding, Doze, rapid pause/resume, and transport
  switching. Disable an enabled microphone control while offline and verify
  the observed microphone state is restored. Record the device and Android version.
- [ ] **Prepare the next shared release.** ([#10](https://github.com/aaronburt/satellite/issues/10)) Choose the version, review the merged
  Android fixes, update the shared version, and run the release checks. Verify
  Windows/Linux assets and a signed APK, including an upgrade from the previous
  official APK that preserves settings. Follow [Contributing](CONTRIBUTING.md#releases).

## Reliability and maintenance

- [ ] **Apply desktop collection-interval changes while running.** ([#11](https://github.com/aaronburt/satellite/issues/11)) Reconfigure
  the collection timer when `interval_sec` changes through the WebUI/API, so a
  process restart is unnecessary. Verify repeated changes and shutdown do not
  leave extra timers running. [Startup and collection](windows/main.go)
- [ ] **Enforce HTTP methods consistently.** ([#12](https://github.com/aaronburt/satellite/issues/12)) Restrict `/api/status`,
  `/api/capabilities`, and `/api/logs/stream` to the documented GET method and
  return HTTP 405 for unsupported methods. Keep the
  [API reference](docs/http-api.md) and handler checks aligned.
- [ ] **Add an explicit way to clear saved API configuration secrets.** ([#13](https://github.com/aaronburt/satellite/issues/13)) Define
  how clients clear MQTT passwords and webhook secrets while preserving the
  existing behavior for omitted or blank secret fields. Document the request
  format and verify that reads continue to redact those secrets.
- [ ] **Define and implement webhook failure recovery.** ([#14](https://github.com/aaronburt/satellite/issues/14)) Decide whether each
  app needs bounded retries, an offline queue, or only the next snapshot.
  Specify backoff, storage limits, expiry, and shutdown behavior before changing
  the current delivery contract. Verify against a failing receiver.
- [ ] **Protect Android credentials at rest.** ([#15](https://github.com/aaronburt/satellite/issues/15)) Design and implement
  Keystore-backed protection for broker credentials and webhook tokens, with
  migration from existing settings and a defined recovery path for unavailable keys.
- [ ] **Replace legacy Android Wi-Fi reads and verify permission-dependent data.** ([#16](https://github.com/aaronburt/satellite/issues/16))
  Review `WifiManager.connectionInfo` in
  [DeviceStateReader](android/core/telemetry/src/main/kotlin/uk/co/aaronburt/satellite/telemetry/DeviceStateReader.kt).
  Verify SSID and carrier reporting with permissions granted and denied on
  supported Android versions and a physical phone.
- [ ] **Verify retained-message expiry and device cleanup.** ([#17](https://github.com/aaronburt/satellite/issues/17)) Confirm state and
  discovery expiry against a real broker. Document and test how renamed,
  removed, or replaced devices avoid leaving stale Home Assistant entities.

## Tests, CI, and documentation

- [ ] **Remove the unsupported Gradle action input.** ([#18](https://github.com/aaronburt/satellite/issues/18)) Remove
  `build-root-directory` from `gradle/actions/setup-gradle` in
  [Android CI](.github/workflows/android-ci.yml) and
  [releases](.github/workflows/release.yml). Keep the correct working directory
  for each build command and verify the action warning disappears.
- [ ] **Preserve failure reports in CI.** ([#19](https://github.com/aaronburt/satellite/issues/19)) Upload JUnit results and Android Lint
  reports even when verification fails, so failures can be investigated without
  relying on pasted logs.
- [ ] **Add Android unit-test coverage on Windows.** ([#20](https://github.com/aaronburt/satellite/issues/20)) After fixing the DataStore
  tests, add a Windows CI job to catch filesystem and host-platform differences.
  Keep device/emulator checks separate from JVM unit tests.
- [ ] **Automate the Home Assistant integration smoke test.** ([#21](https://github.com/aaronburt/satellite/issues/21)) Use the
  [Docker environment](android/docker/README.md) to assert discovery, retained
  availability, state updates, and an enabled-control round trip. Decide whether
  it runs on relevant PRs or by manual dispatch before adding a schedule.
- [ ] **Introduce Kotlin formatting/static analysis.** ([#22](https://github.com/aaronburt/satellite/issues/22)) Select ktlint and/or
  detekt, establish a clean baseline, and add repeatable local and CI commands
  without weakening Android Lint.
- [ ] **Retire stale Android planning claims.** ([#23](https://github.com/aaronburt/satellite/issues/23)) Reconcile the historical
  [handoff notes](android/TODO.md) and [development plan](android/docs/development-plan.md)
  with this list. Remove obsolete release/repository information and conflicting
  licensing claims; the repository currently contains an [MIT License](LICENSE).

## Feature candidates

These are proposals to scope before implementation, rather than committed
release requirements.

- [ ] **Add Pause/Resume to the Android foreground notification.** ([#24](https://github.com/aaronburt/satellite/issues/24)) Define how
  users resume reporting after the ongoing notification is removed, and verify
  actions respect saved reporting state.
- [ ] **Improve Android settings and accessibility.** ([#25](https://github.com/aaronburt/satellite/issues/25)) Review input validation,
  password reveal controls, save feedback, and hardcoded user-facing strings;
  move strings into resources and verify light/dark themes and screen readers.
- [ ] **Expose Android update information in Home Assistant.** ([#26](https://github.com/aaronburt/satellite/issues/26)) Define the update
  entity and whether an install command is appropriate when Android still
  requires installer confirmation.
- [ ] **Scope Android text-to-speech controls.** ([#27](https://github.com/aaronburt/satellite/issues/27)) Define opt-in commands,
  payload limits, engine/language handling, and service lifecycle behavior.
- [ ] **Scope an Android refresh-discovery control.** ([#28](https://github.com/aaronburt/satellite/issues/28)) Define a Home Assistant
  button that reuses reporter refresh and reports disconnected or failed publishes.
- [ ] **Scope Android location reporting.** ([#29](https://github.com/aaronburt/satellite/issues/29)) Agree on device-tracker behavior,
  permissions, freshness, battery impact, and explicit user enablement.
- [ ] **Scope Android activity and step sensors.** ([#30](https://github.com/aaronburt/satellite/issues/30)) Choose the readings and
  define hardware support, permissions, units, and reboot/reset behavior.
- [ ] **Scope Android connected-Bluetooth reporting.** ([#31](https://github.com/aaronburt/satellite/issues/31)) Define supported
  connections, opt-in permissions, identifiers, and cleanup of stale readings.
- [ ] **Evaluate F-Droid distribution.** ([#32](https://github.com/aaronburt/satellite/issues/32)) Review build reproducibility, signing,
  dependency eligibility, and how self-update behaves in an F-Droid build.

## Completed baseline

- [x] Publish the desktop HTTP API reference with authentication, endpoint
  details, and PowerShell/Go examples.
- [x] Add Android module unit tests and shared test fixtures in PR #5.
- [x] Fix PR #5's Robolectric sticky-broadcast lint failure and verify GitHub CI.
- [x] Merge PR #5, including reconnect availability, publish-result reporting,
  offline microphone restoration, and first-read device-identity fixes.
