#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
task_tmp=$(mktemp -d)
trap 'rm -rf -- "$task_tmp"' EXIT
cp -R "$root/.github" "$task_tmp/.github"
cd "$task_tmp"
mkdir -p windows/internal/config android/app
git init -q
git config user.name ReleaseTest
git config user.email release-test@example.invalid
printf 'var Version = "0.28"\n' > windows/internal/config/config.go
printf 'versionName = (project.findProperty("versionName") as String?) ?: "0.28"\n' > android/app/build.gradle.kts
git add .
git commit -qm initial
git tag v0.47

expect() {
  local platform=$1 expected=$2 output
  output=$(bash .github/scripts/platform-version.sh "$platform")
  if ! grep -Fxq "$expected" <<< "$output"; then
    printf 'Expected %s for %s, got:\n%s\n' "$expected" "$platform" "$output" >&2
    exit 1
  fi
}

expect desktop exists=true
expect android exists=true
git tag release-streams-ready
expect desktop exists=true
expect android exists=true
printf 'documentation\n' > windows/README.md
git add . && git commit -qm docs
expect desktop exists=true
expect android exists=true
printf 'desktop change\n' > windows/main.go
git add . && git commit -qm desktop
expect desktop tag=desktop-v0.48
expect android exists=true
git tag desktop-v0.48
expect desktop exists=true
printf 'android change\n' > android/app/source.kt
git add . && git commit -qm android
expect desktop exists=true
expect android tag=android-v0.48
git tag android-v0.48
printf 'desktop second change\n' >> windows/main.go
git add . && git commit -qm desktop-second
expect desktop tag=desktop-v0.49
expect android exists=true
printf 'android second change\n' >> android/app/source.kt
git add . && git commit -qm android-second
expect desktop tag=desktop-v0.49
expect android tag=android-v0.49
expect android code=4900
printf 'windows docs\n' > windows/docs-file.md
git add . && git commit -qm more-docs
git tag desktop-v0.49
git tag android-v0.49
expect desktop exists=true
expect android exists=true
printf 'var Version = "1.0"\n' > windows/internal/config/config.go
git add . && git commit -qm desktop-floor
expect desktop tag=desktop-v1.0
expect android exists=true
git tag desktop-v1.0
printf 'workflow change\n' >> .github/workflows/android-release.yml
git add . && git commit -qm android-workflow
expect desktop exists=true
expect android tag=android-v0.50
git tag android-v0.50
printf 'workflow change\n' >> .github/workflows/desktop-release.yml
git add . && git commit -qm desktop-workflow
expect desktop tag=desktop-v1.1
expect android exists=true
git tag desktop-v1.1
printf 'test change\n' >> .github/scripts/platform-version-test.sh
git add . && git commit -qm release-tests
expect desktop exists=true
expect android exists=true
printf '\n' >> .github/scripts/platform-version.sh
git add . && git commit -qm shared-version-script
expect desktop tag=desktop-v1.2
expect android tag=android-v0.51
git tag v9.99
expect desktop tag=desktop-v1.2
expect android tag=android-v0.51
echo 'Platform release tests passed'
