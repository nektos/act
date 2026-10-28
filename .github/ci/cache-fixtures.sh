#!/usr/bin/env bash
# Warm the local docker store from the GitHub Actions cache so test-linux
# shards do not apt-get inside fixture builds, and do not re-pull base images
# that this shard actually runs.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

shard="${ACT_TEST_LINUX_SHARD:-0/4}"
index="${shard%%/*}"

# type=gha reads the cache URL and ACTIONS_RUNTIME_TOKEN from the environment.
# Shell steps do not receive them. The workflow exports them with
# crazy-max/ghaction-github-runtime before this script runs. Cache v2 uses
# ACTIONS_RESULTS_URL when ACTIONS_CACHE_URL is unset.
if [ -n "${GITHUB_ACTIONS:-}" ]; then
  if { [ -z "${ACTIONS_CACHE_URL:-}" ] && [ -z "${ACTIONS_RESULTS_URL:-}" ]; } || [ -z "${ACTIONS_RUNTIME_TOKEN:-}" ]; then
    echo "ACTIONS_RUNTIME_TOKEN and ACTIONS_CACHE_URL or ACTIONS_RESULTS_URL are required for type=gha" >&2
    exit 1
  fi
  [ -n "${ACTIONS_CACHE_URL:-}" ] && export ACTIONS_CACHE_URL
  [ -n "${ACTIONS_RESULTS_URL:-}" ] && export ACTIONS_RESULTS_URL
  export ACTIONS_RUNTIME_TOKEN
fi

build() {
  local scope="$1"
  shift
  echo "::group::fixture ${scope}"
  docker buildx build \
    --load \
    --cache-from "type=gha,scope=${scope}" \
    --cache-to "type=gha,scope=${scope},mode=max" \
    "$@"
  echo "::endgroup::"
}

cache_base() {
  local image="$1"
  local scope
  scope="pull-$(printf '%s' "$image" | tr '/:' '-')"
  build "$scope" \
    --file .github/ci/fixtures/pull.Dockerfile \
    --build-arg "IMAGE=${image}" \
    --tag "$image" \
    .github/ci/fixtures
}

# Every shard runs job containers from the default platform image.
images=(
  node:24-bookworm-slim
  node:16-buster-slim
  alpine:latest
  debian:bookworm-slim
)

case "$index" in
  0)
    images+=(node:16-buster ubuntu:latest node:12-buster-slim alpine:3)
    ;;
  1)
    images+=(node:16-buster alpine:3)
    ;;
  2)
    images+=(node:16-buster mysql:8 nginx:latest)
    ;;
  3)
    images+=(
      node:12-buster-slim
      alpine:3
      nginx:latest
      catthehacker/ubuntu:pwsh-latest
      catthehacker/ubuntu:runner-latest
    )
    ;;
esac

for image in "${images[@]}"; do
  cache_base "$image"
done

# bookworm docker-action-host-env. Tag matches the name act computes so the
# test does not rebuild the image (the rebuild is what re-runs apt-get).
build docker-action-host-env \
  --file pkg/runner/testdata/docker-action-host-env/action/Dockerfile \
  --tag act-docker-action-host-env-action-dockeraction:latest \
  pkg/runner/testdata/docker-action-host-env/action

build bookworm-curl \
  --file .github/ci/fixtures/bookworm-curl.Dockerfile \
  --tag act-ci/bookworm-curl:latest \
  .github/ci/fixtures
