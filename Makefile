.PHONY: all dist build-server package test clean run update-dependencies-after-release

GOPATH ?= $(GOPATH:)
GOFLAGS ?= $(GOFLAGS:)
BUILD_NUMBER ?= $(BUILD_NUMBER:)
BUILD_DATE = $(shell date -u)
BUILD_HASH = $(shell git rev-parse HEAD)

GO=go

ifeq ($(BUILD_NUMBER),)
	BUILD_NUMBER := dev
endif

DIST_ROOT=dist
DIST_PATH=$(DIST_ROOT)/mattermost-push-proxy

TESTS=.

all: dist

dist: | build-server test package

update-dependencies-after-release:
	@echo Run this to updated the go lang dependencies after a major release
	dep ensure -update

build-server: | .prebuild
	@echo Building proxy push server

	rm -Rf $(DIST_ROOT)
	$(GO) clean $(GOFLAGS) -i ./...

	@echo GOFMT
	$(eval GOFMT_OUTPUT := $(shell gofmt -d -s server/ main.go 2>&1))
	@echo "$(GOFMT_OUTPUT)"
	@if [ ! "$(GOFMT_OUTPUT)" ]; then \
		echo "gofmt sucess"; \
	else \
		echo "gofmt failure"; \
		exit 1; \
	fi

	$(GO) build $(GOFLAGS) ./...
	$(GO) install $(GOFLAGS) ./...

package:
	@ echo Packaging push proxy

	mkdir -p $(DIST_PATH)/bin
	cp $(GOPATH)/bin/mattermost-push-proxy $(DIST_PATH)/bin

	cp -RL config $(DIST_PATH)/config
	touch $(DIST_PATH)/config/build.txt
	echo $(BUILD_NUMBER) | tee -a $(DIST_PATH)/config/build.txt

	mkdir -p $(DIST_PATH)/logs

	cp LICENSE.txt $(DIST_PATH)
	cp NOTICE.txt $(DIST_PATH)
	cp README.md $(DIST_PATH)

	tar -C dist -czf $(DIST_PATH).tar.gz mattermost-push-proxy

test:
	$(GO) test $(GOFLAGS) -run=$(TESTS) -test.v -test.timeout=180s ./server || exit 1

clean:
	@echo Cleaning
	rm -Rf $(DIST_ROOT)
	go clean $(GOFLAGS) -i ./...

	rm -f .prebuild

.prebuild:
	@echo Preparation for running go code
	go get $(GOFLAGS) github.com/Masterminds/glide

	touch $@

run: .prebuild
	@echo Starting go web server
	$(GO) run $(GOFLAGS) main.go
