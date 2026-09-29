#!/usr/bin/env bash
# =============================================================================
# coverage-floor.sh  —  fail when total unit-test coverage drops below the floor
#
# Usage:   bash script/coverage-floor.sh [coverage.out]
#          (the profile comes from: go test -short -coverprofile=coverage.out ./...)
#
# The floor is the measured total, rounded down, when it was last raised. Raise
# it when coverage climbs; never lower it to get a pull request green.
# =============================================================================
set -euo pipefail

FLOOR=75.0

profile="${1:-coverage.out}"
if [ ! -s "$profile" ]; then
  echo "coverage-floor: no coverage profile at $profile" >&2
  exit 2
fi

total=$(go tool cover -func="$profile" | awk '/^total:/ { sub(/%/, "", $NF); print $NF }')
if [ -z "$total" ]; then
  echo "coverage-floor: could not read the total from $profile" >&2
  exit 2
fi

if awk -v t="$total" -v f="$FLOOR" 'BEGIN { exit !(t + 0 < f + 0) }'; then
  echo "::error::Unit-test coverage ${total}% is below the ${FLOOR}% floor (script/coverage-floor.sh). Add tests for the code you changed."
  exit 1
fi
echo "Unit-test coverage ${total}% (floor ${FLOOR}%)"
