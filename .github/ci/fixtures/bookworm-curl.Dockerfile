# Service-container fixtures need curl and netstat. Build them once and reuse
# the layer cache so the workflow steps do not apt-get on every run.
# Debian bookworm matches the docker-action-host-env fixture (nektos/act#6214).
FROM debian:bookworm-slim@sha256:99c0b25c46d0214d5a2091092e7908907dcaecd6a962cde0ac700acee222b6f3

RUN apt-get update \
    && apt-get install -y --no-install-recommends bash ca-certificates curl net-tools \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
