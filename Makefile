.PHONY: all dist build-server package test clean run update-dependencies gofmt govet golangci-lint

GOFLAGS ?= $(GOFLAGS:)
BUILD_NUMBER ?= $(BUILD_NUMBER:)
BUILD_DATE = $(shell date -u)
BUILD_HASH = $(shell git rev-parse HEAD)
LDFLAGS ?= $(LDFLAGS:)

export GOBIN = $(PWD)/bin
GO=go

ifeq ($(BUILD_NUMBER),)
	BUILD_NUMBER := dev
endif

LDFLAGS += -X "github.com/mattermost/mattermost-push-proxy/server.BuildNumber=$(BUILD_NUMBER)"
LDFLAGS += -X "github.com/mattermost/mattermost-push-proxy/server.BuildDate=$(BUILD_DATE)"
LDFLAGS += -X "github.com/mattermost/mattermost-push-proxy/server.BuildHash=$(BUILD_HASH)"

DIST_ROOT=dist
DIST_PATH=$(DIST_ROOT)/mattermost-push-proxy

include build/*.mk

all: dist

dist: | gofmt govet build-server test package

update-dependencies:
	$(GO) get -u ./...
	$(GO) mod tidy

build-server: gofmt golangci-lint
	@echo Building proxy push server

	$(GO) build -o $(GOBIN) -ldflags '$(LDFLAGS)' $(GOFLAGS)

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
