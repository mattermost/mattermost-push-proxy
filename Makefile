.PHONY: all dist build-server package test clean run update-dependencies gofmt govet golangci-lint

GOFLAGS ?= $(GOFLAGS:)
LDFLAGS ?= $(LDFLAGS:)

export GOBIN = $(PWD)/bin
GO=go

# Set version variables for LDFLAGS
GIT_VERSION ?= $(shell git describe --tags --always --dirty)
GIT_HASH ?= $(shell git rev-parse HEAD)
DATE_FMT = +'%Y-%m-%dT%H:%M:%SZ'
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct)
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE ?= $(shell date "$(DATE_FMT)")
endif
GIT_TREESTATE = "clean"
DIFF = $(shell git diff --quiet >/dev/null 2>&1; if [ $$? -eq 1 ]; then echo "1"; fi)
ifeq ($(DIFF), 1)
    GIT_TREESTATE = "dirty"
endif

PP_PKG=github.com/mattermost/mattermost-push-proxy/internal/version
LDFLAGS="-X $(PP_PKG).gitVersion=$(GIT_VERSION) -X $(PP_PKG).gitCommit=$(GIT_HASH) -X $(PP_PKG).gitTreeState=$(GIT_TREESTATE) -X $(PP_PKG).buildDate=$(BUILD_DATE)"


DIST_ROOT=dist
DIST_PATH=$(DIST_ROOT)/mattermost-push-proxy

include build/*.mk

all: dist

dist: | gofmt govet build-server test package

update-dependencies:
	$(GO) get -u ./...
	$(GO) mod tidy

build-server: gofmt
	@echo Building proxy push server

	$(GO) build -o $(GOBIN) -ldflags '$(LDFLAGS)' $(GOFLAGS)

build:
	@echo Building proxy push server

	$(GO) build -o $(GOBIN) -trimpath -ldflags $(LDFLAGS) $(GOFLAGS)

golangci-lint: ## Run golangci-lint on codebase
# https://stackoverflow.com/a/677212/1027058 (check if a command exists or not)
	@if ! [ -x "$$(command -v golangci-lint)" ]; then \
		echo "golangci-lint is not installed. Please see https://github.com/golangci/golangci-lint#install for installation instructions."; \
		exit 1; \
	fi; \

	@echo Running golangci-lint
	golangci-lint run ./...

package:
	@ echo Packaging push proxy

	mkdir -p $(DIST_PATH)/bin
	cp $(GOBIN)/mattermost-push-proxy $(DIST_PATH)/bin

	cp -RL config $(DIST_PATH)/config
	touch $(DIST_PATH)/config/build.txt
	echo $(BUILD_NUMBER) | tee -a $(DIST_PATH)/config/build.txt

	mkdir -p $(DIST_PATH)/logs

	cp LICENSE.txt $(DIST_PATH)
	cp NOTICE.txt $(DIST_PATH)
	cp README.md $(DIST_PATH)

	tar -C dist -czf $(DIST_PATH).tar.gz mattermost-push-proxy

test:
	$(GO) test $(GOFLAGS) -v -timeout=180s ./...

clean:
	@echo Cleaning
	rm -Rf $(DIST_ROOT)
	go clean $(GOFLAGS) -i ./...

run:
	@echo Starting go web server
	$(GO) run $(GOFLAGS) -ldflags '$(LDFLAGS)' main.go

build-swagger:
	npm run validate
	npm run build

serve-swagger:
	npm run validate
	npm run serve