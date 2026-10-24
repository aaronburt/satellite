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

JVM tests that touch DataStore files must give the store an explicit
`OkioStorage` (see `DataStoreSettingsRepositoryTest`). The default JVM
`FileStorage`, used by both `create` and `createWithPath`, commits writes with
`File.renameTo`, which cannot replace an
existing file on Windows, so those tests pass on Linux and fail on Windows.
CI runs `:core:datastore:testDebugUnitTest` on a Windows runner to catch this.

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

The root [release workflow](.github/workflows/release.yml) builds one release
containing desktop and Android artifacts:

1. Update `const Version` in `windows/internal/config/config.go` to `X.Y.Z`.
2. Run the relevant application checks and review the changes.
3. Push the approved change to `main`. The workflow also supports manual dispatch.
4. Check the workflow result and the assets on the generated `vX.Y.Z` release.

The workflow skips a version if its release already exists. Build jobs run Go
and Android unit tests before publication; separate CI jobs provide additional
checks. Linux tests run in Go CI, while the release workflow cross-compiles the
Linux binary on Windows.

Android release `versionName` comes from the shared version. `versionCode` is
`major * 10000 + minor * 100 + patch`; keep minor and patch below 100 so versions
remain ordered within that encoding. Local Android builds use the defaults in
`app/build.gradle.kts` unless overridden with `-PversionName` and `-PversionCode`.

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
