PREFIX ?= /usr/local
VERSION ?= $(shell git describe --tags --dirty | cut -c 2-)
IS_SNAPSHOT = $(if $(findstring -, $(VERSION)),true,false)
MAJOR_VERSION = $(word 1, $(subst ., ,$(VERSION)))
MINOR_VERSION = $(word 2, $(subst ., ,$(VERSION)))
PATCH_VERSION = $(word 3, $(subst ., ,$(word 1,$(subst -, , $(VERSION)))))
NEW_VERSION ?= $(MAJOR_VERSION).$(MINOR_VERSION).$(shell echo $$(( $(PATCH_VERSION) + 1)) )

ACT ?= go run main.go
export GITHUB_TOKEN := $(shell cat ~/.config/github/token)

.PHONY: build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o dist/local/act main.go

.PHONY: format
format:
	go fmt ./...

.PHONY: test
test:
	go test ./...
	$(ACT)

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
