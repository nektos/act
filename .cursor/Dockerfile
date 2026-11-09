FROM ubuntu:24.04@sha256:24737cef17f4e22b1977df7d9a05e77c4e28b7da5b8911f8d8ccda136bf3573c

# Match go.mod and .github/workflows/checks.yml (golangci-lint-action version: v2.11.4).
ARG GO_VERSION=1.26.0
ARG GOLANGCI_LINT_VERSION=2.11.4

# nodejs and npm provide npx for `make format-all` and `make pr`.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git make gcc libc6-dev nodejs npm \
    && rm -rf /var/lib/apt/lists/* \
    && arch="$(dpkg --print-architecture)" \
    && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" -o /tmp/go.tgz \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz \
    && ln -sf /usr/local/go/bin/go /usr/local/bin/go \
    && ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt \
    && curl -fsSL "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${arch}.tar.gz" -o /tmp/golangci.tgz \
    && tar -C /usr/local/bin -xzf /tmp/golangci.tgz --strip-components=1 "golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${arch}/golangci-lint" \
    && rm /tmp/golangci.tgz

ENV PATH="/usr/local/go/bin:${PATH}"

USER ubuntu
