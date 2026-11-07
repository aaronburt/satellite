#!/usr/bin/env bash
# Tests for next-version.sh. Run: .github/scripts/next-version-test.sh
set -uo pipefail

script="$(dirname "$0")/next-version.sh"
failures=0

expect() {
  local want=$1
  shift
  local got
  got=$("$script" "$@" 2>/dev/null) || got="<exit $?>"
  if [[ "$got" == "$want" ]]; then
    echo "ok    $want <- $*"
  else
    echo "FAIL  want $want, got $got <- $*"
    failures=$((failures + 1))
  fi
}

# Switch-over from three-part versions: first release is 0.28.
expect 0.28 0.28 v0.26.1 v0.27.0 v0.27.3 v0.27.2
# Every later release bumps the minor number.
expect 0.29 0.28 v0.27.3 v0.28
expect 0.30 0.28 v0.27.3 v0.28 v0.29
# Minor numbers compare as numbers, not text.
expect 0.101 0.28 v0.28 v0.99 v0.100 v0.9
# Raising the floor jumps to it, then bumping carries on from there.
expect 1.0 1.0 v0.27.3 v0.45
expect 1.1 1.0 v0.45 v1.0
# A floor below the latest release is ignored.
expect 0.41 0.28 v0.40
# No releases yet: use the floor.
expect 0.28 0.28
# Tags that are not releases are ignored.
expect 0.29 0.28 v0.28 nightly v9.9-beta v10 latest 1.5
# Leading zeros don't make the numbers octal.
expect 0.10 0.08 v0.09
# Bad floors are rejected.
expect "<exit 1>" 0.28.0 v0.27.3
expect "<exit 1>" v0.28
expect "<exit 2>"

if ((failures > 0)); then
  echo "$failures test(s) failed"
  exit 1
fi
echo "all tests passed"
