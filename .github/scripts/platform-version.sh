#!/usr/bin/env bash
set -euo pipefail

platform=${1:?Choose desktop or android}
case "$platform" in
  desktop)
    directory=windows
    floor=$(sed -nE 's/^var[[:space:]]+Version[[:space:]]*=[[:space:]]*"([^"]+)".*/\1/p' windows/internal/config/config.go | head -n1)
    ;;
  android)
    directory=android
    floor=$(sed -nE 's/.*versionName = .*\?: "([^"]+)".*/\1/p' android/app/build.gradle.kts | head -n1)
    ;;
  *) echo "Unknown platform: $platform" >&2; exit 1 ;;
esac

if ! git rev-parse --verify refs/tags/release-streams-ready >/dev/null 2>&1; then
  echo "exists=true"
  echo "The combined updater bridge must publish first." >&2
  exit 0
fi

mapfile -t own_tags < <(git tag --list "$platform-v*" --sort=-version:refname | grep -E "^$platform-v[0-9]+\.[0-9]+$" || true)
base=${own_tags[0]:-release-streams-ready}
if git diff --quiet "$base" HEAD -- "$directory" ".github/workflows/$platform-release.yml" .github/scripts/platform-version.sh .github/scripts/next-version.sh ':(glob,exclude)**/*.md' ":(exclude)$directory/docs"; then
  echo "exists=true"
  echo "No unreleased $platform build changes." >&2
  exit 0
fi

mapfile -t legacy_tags < <(git tag --list 'v*')
normalized=()
for tag in "${own_tags[@]}"; do normalized+=("${tag#"$platform-"}"); done
if ((${#normalized[@]})); then
  version=$(.github/scripts/next-version.sh "$floor" "${normalized[@]}")
else
  version=$(.github/scripts/next-version.sh "$floor" "${legacy_tags[@]}")
fi
IFS='.' read -r major minor <<< "$version"
if ((10#$major > 2099 || 10#$minor > 9999)); then
  echo "Version exceeds the Android version-code range." >&2
  exit 1
fi
printf 'exists=false\nversion=%s\ntag=%s-v%s\ncode=%s\nsha=%s\n' "$version" "$platform" "$version" "$((10#$major * 1000000 + 10#$minor * 100))" "$(git rev-parse HEAD)"
