# Home Assistant update entity

Status: implemented for issue #26. This is the design that was agreed before the
code, kept as the reference for the entity's schema and behaviour.

## What Home Assistant gets

Two entities on the satellite device, both `diagnostic`:

| Key | Platform | Purpose |
|-----|----------|---------|
| `app_update` | `update` (`device_class: firmware`) | Installed and latest version, release link, download progress. |
| `update_status` | `sensor` (`device_class: enum`) | Where the update flow is, including a failed check. |

The status sensor exists because an `update` entity has no way to say "the last
check failed" or "checking now".

### `app_update` state topic (JSON, retained)

`satellite/<id>/state/app_update`

| Field | When it is sent |
|-------|-----------------|
| `installed_version` | Always (the running `versionName`). |
| `latest_version` | Only after a check produced one: the installed version when up to date, the release version while downloading or ready. |
| `in_progress`, `update_percentage` | While downloading (progress is sent in steps of 10). |
| `title`, `release_url`, `release_summary` | When a verified update is ready. `release_summary` is the release notes cut to 255 characters. |
| `release_summary` | Also carries `Update check failed: <reason>` after a failed check. |

`latest_version` is left out rather than guessed when the app has not checked or
the check failed, so the entity never reports "up to date" without knowing.

### `update_status` values

`idle` (no check since the app started), `checking`, `up_to_date`,
`downloading`, `ready` (downloaded and verified, waiting for the user), `failed`.

## Checking and installing

- **No install command in this version.** Android needs the user to confirm the
  system installer, so Home Assistant could only ever *request* an install.
  Presenting that as an installation would be misleading, and a remote install
  trigger is a bigger trust decision than a read-only sensor. If it is added
  later it must be an opt-in control (off by default, like volume and mute),
  must reuse `UpdateManager.install()`, and must report `ready` until the user
  confirms on the device.
- **No automatic checks.** The existing rule stands: a check only starts from the
  Updates screen, so the app never phones home by itself and stays inside
  GitHub's unauthenticated rate limit. Home Assistant therefore shows `idle`
  after a restart until someone checks.
- **Verification is unchanged.** Everything goes through `UpdateManager`, so the
  download is still HTTPS-only, checked against GitHub's SHA-256 and required to
  be signed by the installed app's key before `ready` is reported.

## Webhook output

The webhook snapshot gets the same two keys under `state` (the `app_update`
value is the JSON object as a string, like the other values).
