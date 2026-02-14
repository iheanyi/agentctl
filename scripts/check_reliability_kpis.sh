#!/usr/bin/env bash

set -euo pipefail

MIN_RELIABILITY_COVERAGE="${MIN_RELIABILITY_COVERAGE:-15}"
TARGETS=(
  "./internal/cli/..."
  "./internal/tui/..."
  "./pkg/config/..."
  "./pkg/discovery/..."
  "./pkg/sync/..."
)

cover_file="$(mktemp)"
trap 'rm -f "$cover_file"' EXIT

echo "Running reliability-critical tests:"
printf '  - %s\n' "${TARGETS[@]}"

go test -race -covermode=atomic -coverprofile="$cover_file" "${TARGETS[@]}"

total_coverage="$(go tool cover -func="$cover_file" | awk '/^total:/{gsub("%","",$3); print $3}')"

echo "Reliability KPI: coverage=${total_coverage}% min=${MIN_RELIABILITY_COVERAGE}%"

awk -v actual="$total_coverage" -v min="$MIN_RELIABILITY_COVERAGE" 'BEGIN { exit (actual+0 >= min+0 ? 0 : 1) }'
echo "Reliability KPI gate passed."
