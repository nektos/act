PREFIX ?= /usr/local
VERSION ?= $(shell git describe --tags --dirty --always | sed -e 's/^v//')
IS_SNAPSHOT = $(if $(findstring -, $(VERSION)),true,false)
MAJOR_VERSION = $(word 1, $(subst ., ,$(VERSION)))
MINOR_VERSION = $(word 2, $(subst ., ,$(VERSION)))
PATCH_VERSION = $(word 3, $(subst ., ,$(word 1,$(subst -, , $(VERSION)))))
NEW_VERSION ?= $(MAJOR_VERSION).$(MINOR_VERSION).$(shell echo $$(( $(PATCH_VERSION) + 1)) )
GOVULNCHECK_PACKAGE ?= golang.org/x/vuln/cmd/govulncheck@v1

fix = false
ifeq (true,$(fix))
	FIX = --fix
endif

ACT ?= go run main.go

# $(test ...) is not a Make function, so the previous line always expanded to false.
HAS_TOKEN = $(shell if test -e ~/.config/github/token; then echo true; else echo false; fi)
ifeq (true,$(HAS_TOKEN))
	export GITHUB_TOKEN := $(shell cat ~/.config/github/token)
endif

.PHONY: pr
pr: tidy format-all lint test

.PHONY: build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o dist/local/act main.go

.PHONY: format
format:
	go fmt ./...

.PHONY: format-all
format-all:
	go fmt ./...
	npx prettier --write .

.PHONY: test
test:
	go test ./...
	$(ACT)

# Unit tests only. -short skips Docker integration tests (see testing.Short() in
# pkg/runner and pkg/container). Does not need a Docker daemon.
.PHONY: test-fast
test-fast:
	go test -short ./...

.PHONY: lint-go
lint-go:
	golangci-lint run $(FIX)

.PHONY: lint-js
lint-js:
	npx standard $(FIX)

.PHONY: lint-md
lint-md:
	npx markdownlint . $(FIX)

.PHONY: lint-rest
lint-rest:
	docker run --rm -it \
		-v $(PWD):/tmp/lint \
		-e GITHUB_STATUS_REPORTER=false \
		-e GITHUB_COMMENT_REPORTER=false \
		oxsecurity/megalinter-go:v8.4.2

.PHONY: lint
lint: lint-go lint-rest

.PHONY: lint-fix
lint-fix: lint-md lint-go

.PHONY: fix
fix:
	make lint-fix fix=true

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: install
install: build
	@cp dist/local/act $(PREFIX)/bin/act
	@chmod 755 $(PREFIX)/bin/act
	@act --version

.PHONY: installer
# godownloader's go.mod has a replace directive, so `go install` of the module path is rejected.
# Installing from a checkout makes it the main module, where that directive is allowed.
installer:
	@set -e; \
	tmpdir=$$(mktemp -d); \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	git clone --depth 1 --branch v0.1.0 https://github.com/goreleaser/godownloader.git "$$tmpdir/src"; \
	mkdir -p "$$tmpdir/bin"; \
	( cd "$$tmpdir/src" && GOBIN="$$tmpdir/bin" go install . ); \
	"$$tmpdir/bin/godownloader" -r nektos/act -o install.sh

.PHONY: promote
# promote tags HEAD of PROMOTE_BRANCH and pushes only the tag; it never pushes
# a commit to the branch. Builds take their version from the tag through
# ldflags, so the VERSION file is not bumped. promote.yml sets PROMOTE_BRANCH
# to the repository default branch. A local run uses the current branch.
# Only master and main are accepted.
promote:
	@git fetch origin --tags
	@if [ -n "$$(git status -s)" ]; then \
		echo "Unable to promote a dirty workspace"; \
		exit 1; \
	fi
	@set -eu; \
	branch="$${PROMOTE_BRANCH:-$$(git rev-parse --abbrev-ref HEAD)}"; \
	case "$$branch" in \
		master|main) ;; \
		*) echo "Unable to promote branch '$$branch' (expected master or main)"; exit 1 ;; \
	esac; \
	head_sha=$$(git rev-parse HEAD); \
	remote_head=$$(git ls-remote origin "refs/heads/$$branch" | awk '{print $$1}'); \
	if [ "$$remote_head" != "$$head_sha" ]; then \
		echo "Unable to promote: HEAD $$head_sha is not origin/$$branch ($${remote_head:-missing})"; \
		exit 1; \
	fi; \
	if git describe --tags --exact-match >/dev/null 2>&1; then \
		tag=$$(git describe --tags --exact-match); \
		version=$${tag#v}; \
		local_sha=$$(git rev-parse "$$tag"); \
		remote_sha=$$(git ls-remote origin "refs/tags/$$tag" | awk '{print $$1}'); \
		if [ -n "$$remote_sha" ] && [ "$$remote_sha" = "$$local_sha" ]; then \
			echo "Nothing to promote: $$version is already a release (no new commits since the last tag)"; \
			exit 0; \
		fi; \
		echo "Release tag $$tag is not on origin at this commit; pushing"; \
		git push origin "refs/tags/$$tag"; \
		exit 0; \
	fi; \
	if ! git describe --tags >/dev/null 2>&1; then \
		echo "Unable to promote: no release tag is reachable from HEAD"; \
		exit 1; \
	fi; \
	version=$$(git describe --tags --always | sed -e 's/^v//'); \
	major=$$(printf '%s\n' "$$version" | cut -d. -f1); \
	minor=$$(printf '%s\n' "$$version" | cut -d. -f2); \
	patch=$$(printf '%s\n' "$$version" | cut -d. -f3 | cut -d- -f1); \
	new_version="$$major.$$minor.$$((patch + 1))"; \
	echo "VERSION:$$version IS_SNAPSHOT:true NEW_VERSION:$$new_version"; \
	git tag -a -m "releasing v$$new_version" "v$$new_version"; \
	git push origin "refs/tags/v$$new_version"

.PHONY: snapshot
snapshot:
	goreleaser build \
		--clean \
		--single-target \
		--snapshot

.PHONY: clean all

.PHONY: upgrade
upgrade:
	go get -u
	go mod tidy

# "$(shell go env GOROOT)/bin/go" allows us to use an outdated global go tool and use the build toolchain defined by the project
# go build auto upgrades to the same toolchain version as defined in the go.mod file
.PHONY: deps-tools
deps-tools: ## install tool dependencies
	"$(shell go env GOROOT)/bin/go" install $(GOVULNCHECK_PACKAGE)

.PHONY: security-check
security-check: deps-tools
	GOEXPERIMENT= "$(shell go env GOROOT)/bin/go" run $(GOVULNCHECK_PACKAGE) -show color ./...
