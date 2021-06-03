PREFIX ?= /usr/local
VERSION ?= $(shell git describe --tags --dirty --always | sed -e 's/^v//')
IS_SNAPSHOT = $(if $(findstring -, $(VERSION)),true,false)
MAJOR_VERSION = $(word 1, $(subst ., ,$(VERSION)))
MINOR_VERSION = $(word 2, $(subst ., ,$(VERSION)))
PATCH_VERSION = $(word 3, $(subst ., ,$(word 1,$(subst -, , $(VERSION)))))
NEW_VERSION ?= $(MAJOR_VERSION).$(MINOR_VERSION).$(shell echo $$(( $(PATCH_VERSION) + 1)) )

fix = false
ifeq (true,$(fix))
	FIX = --fix
endif

ACT ?= go run main.go

HAS_TOKEN = $(if $(test -e ~/.config/github/token),true,false)
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
		-e 'RUN_LOCAL=true' \
		-e 'FILTER_REGEX_EXCLUDE=.*testdata/*' \
		-e 'VALIDATE_BASH=false' \
		-e 'VALIDATE_DOCKERFILE=false' \
		-e 'VALIDATE_DOCKERFILE_HADOLINT=false' \
		-e 'VALIDATE_GO=false' \
		-e 'VALIDATE_JSCPD=false' \
		-e 'VALIDATE_SHELL_SHFMT=false' \
		-v $(PWD):/tmp/lint \
		github/super-linter

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
installer:
	@GO111MODULE=off go get github.com/goreleaser/godownloader
	godownloader -r nektos/act -o install.sh

.PHONY: promote
promote:
	@git fetch --tags
	@echo "VERSION:$(VERSION) IS_SNAPSHOT:$(IS_SNAPSHOT) NEW_VERSION:$(NEW_VERSION)"
ifeq (false,$(IS_SNAPSHOT))
	@echo "Unable to promote a non-snapshot"
	@exit 1
endif
ifneq ($(shell git status -s),)
	@echo "Unable to promote a dirty workspace"
	@exit 1
endif
	git tag -a -m "releasing v$(NEW_VERSION)" v$(NEW_VERSION)
	git push origin v$(NEW_VERSION)
