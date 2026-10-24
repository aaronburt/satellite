#!/usr/bin/env bash
# Kotlin formatting (ktlint) and static analysis (detekt) for the Android project.
# Usage (from android/):  scripts/kotlin-checks.sh          check only
#                         scripts/kotlin-checks.sh --format  fix formatting first
# Pinned tools are downloaded once to android/.tools/ (git-ignored). Needs Java 17+.
set -euo pipefail

KTLINT_VERSION=1.7.1
DETEKT_VERSION=1.23.8

cd "$(dirname "$0")/.."
TOOLS=.tools
mkdir -p "$TOOLS"

ktlint="$TOOLS/ktlint-$KTLINT_VERSION"
detekt="$TOOLS/detekt-cli-$DETEKT_VERSION-all.jar"
[ -f "$ktlint" ] || curl -fsSL -o "$ktlint" \
  "https://github.com/pinterest/ktlint/releases/download/$KTLINT_VERSION/ktlint"
[ -f "$detekt" ] || curl -fsSL -o "$detekt" \
  "https://github.com/detekt/detekt/releases/download/v$DETEKT_VERSION/detekt-cli-$DETEKT_VERSION-all.jar"

sources=('**/*.kt' '**/*.kts' '!**/build/**' '!.tools/**')

status=0
if [ "${1:-}" = "--format" ]; then
  java -jar "$ktlint" --format "${sources[@]}" || status=1
else
  java -jar "$ktlint" "${sources[@]}" || status=1
fi

java -jar "$detekt" \
  --build-upon-default-config \
  --config config/detekt/detekt.yml \
  --baseline config/detekt/baseline.xml \
  --input . \
  --excludes '**/build/**,**/.tools/**' || status=1

exit $status
