#!/usr/bin/env bash
# Run one test-linux shard. ACT_TEST_LINUX_SHARD is "INDEX/TOTAL".
# Shard 0 also runs every package other than the sharded runner integration tests.
set -euo pipefail

shard="${ACT_TEST_LINUX_SHARD:?set ACT_TEST_LINUX_SHARD to INDEX/TOTAL}"
index="${shard%%/*}"
jsonfile="${TIMING_JSON:-timing.json}"
cover="${COVERAGE_FILE:-coverage.txt}"

packages=(./pkg/runner)
if [ "$index" = "0" ]; then
  packages=(./...)
fi

status=0
# --packages is required when --rerun-fails is set. It is a space-separated list.
go run gotest.tools/gotestsum@v1.13.0 \
  --rerun-fails=2 \
  --junitfile unit-tests.xml \
  --jsonfile-timing-events "$jsonfile" \
  --format pkgname \
  --packages="${packages[*]}" \
  -- \
  -v -cover -coverpkg=./... -coverprofile="$cover" -covermode=atomic -timeout 20m || status=$?

echo "::group::slowest tests (go test -json timing)"
go run gotest.tools/gotestsum@v1.13.0 tool slowest --threshold 1s --jsonfile "$jsonfile" || true
echo "::endgroup::"
exit "$status"
