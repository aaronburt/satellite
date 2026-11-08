# Contributing

Read the [architecture overview](docs/architecture.md) before changing runtime
behavior. The applications build independently: desktop is a Go module in
`windows/`; Android is a Gradle build in `android/`. Run commands from the stated
directory, rather than the repository root.

## Desktop development

Install the Go toolchain declared in [`windows/go.mod`](windows/go.mod). From
`windows/`, build a console binary for development:

```powershell
go build -o satellite.exe .
.\satellite.exe --debug
```

On Linux:

```sh
go build -o satellite .
./satellite --debug
```

Debug mode prints the web UI URL and can load
`internal/webui/static/index.html` from disk. Normal builds embed that HTML.
Running the agent uses your real Satellite configuration and may connect to its
configured outputs; use a development broker and settings when exercising controls.

Before submitting desktop changes, run from `windows/`:

```sh
go vet ./...
go test -count=1 ./...
gofmt -l .
```

`gofmt -l .` should print nothing. Format files you changed with `gofmt -w`.
CI also checks `go mod tidy`, runs the Linux tests with `-race`, and vets Windows
and Darwin builds. Run `go test -race -count=1 ./...` on a system with a working
race-detector/CGO toolchain when changing concurrent behavior.

Tests include an in-process MQTT broker, HTTP test servers, and platform-specific
behavior. They need loopback networking and writable dependency caches; some
Windows paths invoke native notifications. Configuration-saving tests use
temporary configuration directories through `internal/testutil`.

## Android development

Use JDK 21 to match CI and the committed Gradle wrapper. Configure `ANDROID_HOME`
or an untracked `local.properties` with `sdk.dir`. Build requirements live in
[`app/build.gradle.kts`](android/app/build.gradle.kts), the
[version catalog](android/gradle/libs.versions.toml), and
[CI workflow](.github/workflows/android-ci.yml). The app declares compile SDK 37,
target SDK 36, and minimum SDK 26; CI currently installs `platforms;android-37.2`
and `build-tools;37.0.0`.

From `android/` on Windows:

```powershell
.\gradlew.bat test lint :app:assembleDebug
```

Before pushing Kotlin changes, run `scripts/kotlin-checks.sh` from `android/`
(ktlint formatting and detekt; add `--format` to fix formatting). See
[Kotlin code quality](android/docs/code-quality.md) for the rules.

JVM tests that touch DataStore files must give the store an explicit
`OkioStorage` (see `DataStoreSettingsRepositoryTest`). The default JVM
`FileStorage`, used by both `create` and `createWithPath`, commits writes with
`File.renameTo`, which cannot replace an
existing file on Windows, so those tests pass on Linux and fail on Windows.
Android CI runs the full JVM unit-test suite (`test`) on both Linux
(`ubuntu-latest`) and Windows (`windows-latest`) runners to catch this.

On Linux/macOS, use `./gradlew` instead. These checks do not require a running
device. Device/UI tests require an emulator or phone; the broker configuration
test additionally needs the Docker broker and a fresh app setup. See
[Android testing](android/docs/testing.md) for commands and scope.

## Integration changes

- Keep node IDs, device IDs, entity unique IDs, topics, and state keys stable
  unless a deliberate compatibility change is required. Document any migration.
- For desktop sensors, consider platform capabilities, configuration toggles,
  collection, discovery, and significant-change detection together.
- For Android entities, keep the catalog, state reader, discovery, and active-key
  filtering consistent. Controls also need parsing, opt-in checks, and observed
  state reporting.
- Exercise disconnect/reconnect, broker or Home Assistant restart, configuration
  changes, and shutdown when changing connection/reporting code. Verify broker
  messages as well as displayed connection status.
- Add tests for meaningful behavior and failure paths. Update the relevant
  [documentation](docs/README.md) in the same change.

Use the [Docker environment](android/docker/README.md) for local integration
testing. Its development credentials and exposed ports are documented there.

## Releases

Versions are `MAJOR.MINOR`, with independent `desktop-vMAJOR.MINOR` and
`android-vMAJOR.MINOR` tags. Nothing is committed back to `main`.

- Changes under `windows/` run [Desktop release](.github/workflows/desktop-release.yml),
  which tests Go and builds the Windows executable, installer, and Linux binary.
- Changes under `android/` run [Android release](.github/workflows/android-release.yml),
  which tests Android and builds the signed APK.
- Changes to both folders run both streams. Markdown and platform `docs/` changes
  do not release. A release workflow change affects its own stream; changes to
  `platform-version.sh` or `next-version.sh` affect both.

Each stream compares its current source with its last release tag, so reruns with
no relevant changes skip builds. Runs serialize per platform and resolve current
`main` when they start, allowing queued changes to ship together. Failed builds
publish nothing and remain eligible on the next relevant push or manual run.
Windows and Linux remain together because they share the Go module in `windows/`.
The Linux release binary is cross-compiled on Windows; Go CI also tests on Linux.

The first independent version starts above the legacy `v*` releases. Subsequent
versions advance only from that platform's tags. To raise a platform's version
floor, update `var Version` in `windows/internal/config/config.go` or the default
`versionName` in `android/app/build.gradle.kts`.

The one-time [Release bridge](.github/workflows/release.yml) publishes a combined
`vMAJOR.MINOR` release containing updaters that recognize the independent streams.
It remains GitHub's latest release for older installations. After publishing, it
creates the `release-streams-ready` marker tag; independent workflows wait for
that marker and check for unreleased changes when the bridge completes. Later
bridge runs skip builds. Keep the marker tag and bridge release so older clients
can migrate. Platform releases use `--latest=false` to preserve that route.

Desktop builds embed the numeric version through Go linker flags. Android uses
that stream's numeric `versionName` and `versionCode = major * 1000000 + minor * 100`.
Local builds retain their source defaults. Clients installed before the bridge
must update to it before they can discover platform-specific releases.

Publishing a signed APK requires repository secrets `RELEASE_KEYSTORE_BASE64`,
`RELEASE_STORE_PASSWORD`, `RELEASE_KEY_ALIAS`, and `RELEASE_KEY_PASSWORD`. The
workflow refuses to publish without them. Keep a secure backup of the release
keystore: installed apps require the same signing key for updates. A local
release build falls back to the development keystore when `keystore.properties`
is absent; it is not interchangeable with the official signed release.

## Reports and credentials

Include reproduction steps, affected platform/version, and sanitized logs in bug
reports. Keep broker credentials, API keys, signing material, exported private
configuration, and local build outputs out of commits. Report vulnerabilities
through [SECURITY.md](SECURITY.md).

Contributions use the repository's [MIT License](LICENSE).
