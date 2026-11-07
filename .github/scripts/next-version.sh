#!/usr/bin/env bash
# Prints the next release version (MAJOR.MINOR) for the release workflow.
#
# Usage: next-version.sh FLOOR [TAG...]
#   FLOOR  the version in windows/internal/config/config.go (MAJOR.MINOR).
#   TAG    existing release tags, e.g. v0.27.3 or v0.28. Others are ignored.
#
# The next version is the latest tag with its minor number bumped by one, or
# FLOOR if that is higher. Raising FLOOR is how a major bump (or a jump)
# happens: set it to 1.0 and the next release is 1.0, then 1.1, and so on.
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 FLOOR [TAG...]" >&2
  exit 2
fi

floor=$1
shift
if [[ ! "$floor" =~ ^([0-9]+)\.([0-9]+)$ ]]; then
  echo "floor version '$floor' is not MAJOR.MINOR" >&2
  exit 1
fi
floor_major=$((10#${BASH_REMATCH[1]}))
floor_minor=$((10#${BASH_REMATCH[2]}))

# Highest existing release, comparing major then minor. Old three-part tags
# (v0.27.3) count by their major and minor only.
have_tag=false
latest_major=0
latest_minor=0
for tag in "$@"; do
  if [[ "$tag" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+)?$ ]]; then
    major=$((10#${BASH_REMATCH[1]}))
    minor=$((10#${BASH_REMATCH[2]}))
    if ! $have_tag || ((major > latest_major || (major == latest_major && minor > latest_minor))); then
      latest_major=$major
      latest_minor=$minor
      have_tag=true
    fi
  fi
done

next_major=$floor_major
next_minor=$floor_minor
if $have_tag; then
  bumped_major=$latest_major
  bumped_minor=$((latest_minor + 1))
  if ((bumped_major > floor_major || (bumped_major == floor_major && bumped_minor > floor_minor))); then
    next_major=$bumped_major
    next_minor=$bumped_minor
  fi
fi

echo "$next_major.$next_minor"
