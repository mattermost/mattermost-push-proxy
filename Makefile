.PHONY: all dist build-server package test clean run update-dependencies gofmt govet check-style

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

all: dist

check-style: gofmt govet

dist: | gofmt govet build-server test package

update-dependencies:
	$(GO) get -u ./...
	$(GO) mod tidy

build-server: gofmt govet
	@echo Building proxy push server

	$(GO) build -o $(GOBIN) -ldflags '$(LDFLAGS)' $(GOFLAGS)

gofmt:
	@echo GOFMT
	$(eval GOFMT_OUTPUT := $(shell gofmt -d -s server/ main.go 2>&1))
	@echo "$(GOFMT_OUTPUT)"
	@if [ ! "$(GOFMT_OUTPUT)" ]; then \
		echo "gofmt sucess"; \
	else \
		echo "gofmt failure"; \
		exit 1; \
	fi

govet:
	@echo Running govet
	env GO111MODULE=off $(GO) get golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow
	$(GO) vet $$(go list ./...) .
	$(GO) vet -vettool=$(GOBIN)/shadow $$(go list ./...)
	@echo Govet success

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
