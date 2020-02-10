LATEST_VERSION := $(shell git tag -l --sort=creatordate | grep "^v[0-9]*.[0-9]*.[0-9]*$$" | tail -1 | cut -c 2-)
ifeq "$(shell git tag -l v$(LATEST_VERSION) --points-at HEAD)" "v$(LATEST_VERSION)"
### latest tag points to current commit, this is a release build
VERSION ?= $(LATEST_VERSION)
else
### latest tag points to prior commit, this is a snapshot build
MAJOR_VERSION := $(word 1, $(subst ., ,$(LATEST_VERSION)))
MINOR_VERSION := $(word 2, $(subst ., ,$(LATEST_VERSION)))
PATCH_VERSION := $(word 3, $(subst ., ,$(LATEST_VERSION)))
VERSION ?= $(MAJOR_VERSION).$(MINOR_VERSION).$(shell echo $$(( $(PATCH_VERSION) + 1)) )-develop
endif
IS_SNAPSHOT = $(if $(findstring -, $(VERSION)),true,false)
TAG_VERSION = v$(VERSION)

ACT ?= go run -mod=vendor main.go
export GITHUB_TOKEN = $(shell cat ~/.config/github/token)

default: check

test:
	go test -cover -short ./...

check:
	$(ACT) -rj ci

build: check
	$(eval export SNAPSHOT_VERSION=$(VERSION))
	$(ACT) -ra build

release:
	$(ACT) -ra local-release

install: build
	@cp dist/$(shell go env GOOS)_$(shell go env GOARCH)/act /usr/local/bin/act
	@chmod 755 /usr/local/bin/act
	@act --version

installer:
	@GO111MODULE=off go get github.com/goreleaser/godownloader
	godownloader -r nektos/act -o install.sh

promote: vendor
	@echo "VERSION:$(VERSION) IS_SNAPSHOT:$(IS_SNAPSHOT) LATEST_VERSION:$(LATEST_VERSION)"
ifeq (false,$(IS_SNAPSHOT))
	@echo "Unable to promote a non-snapshot"
	@exit 1
endif
ifneq ($(shell git status -s),)
	@echo "Unable to promote a dirty workspace"
	@exit 1
endif
	$(eval NEW_VERSION := $(word 1,$(subst -, , $(TAG_VERSION))))
	git tag -a -m "releasing $(NEW_VERSION)" $(NEW_VERSION)
	git push origin $(NEW_VERSION)

vendor:
	go mod vendor

.PHONY: vendor
