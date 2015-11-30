.PHONY: all dist dist-local dist-travis start-docker build-server package build-client test travis-init build-container stop-docker clean-docker clean nuke run stop setup-mac cleandb docker-build docker-run

GOPATH ?= $(GOPATH:)
GOFLAGS ?= $(GOFLAGS:)
BUILD_NUMBER ?= $(BUILD_NUMBER:)
BUILD_DATE = $(shell date -u)
BUILD_HASH = $(shell git rev-parse HEAD)

GO=$(GOPATH)/bin/godep go

ifeq ($(BUILD_NUMBER),)
	BUILD_NUMBER := dev
endif

ifeq ($(TRAVIS_BUILD_NUMBER),)
	BUILD_NUMBER := dev
else
	BUILD_NUMBER := $(TRAVIS_BUILD_NUMBER)
endif

DIST_ROOT=dist
DIST_PATH=$(DIST_ROOT)/matter-push-proxy

TESTS=.

all: dist

dist: | build-server test package

build-server: | .prepare
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
	cp $(GOPATH)/bin/push-proxy $(DIST_PATH)/bin

	cp -RL config $(DIST_PATH)/config
	touch $(DIST_PATH)/config/build.txt
	echo $(BUILD_NUMBER) | tee -a $(DIST_PATH)/config/build.txt

	mkdir -p $(DIST_PATH)/logs

	cp build/MIT-COMPILED-LICENSE.md $(DIST_PATH)
	cp NOTICE.txt $(DIST_PATH)
	cp README.md $(DIST_PATH)

	tar -C dist -czf $(DIST_PATH).tar.gz matter-proxy-push

test:
	$(GO) test $(GOFLAGS) -run=$(TESTS) -test.v -test.timeout=180s ./server || exit 1

clean: stop-docker
	rm -Rf $(DIST_ROOT)
	go clean $(GOFLAGS) -i ./...

	rm -rf Godeps/_workspace/pkg/

	rm -f .prepare

.prepare:
	@echo Preparation for run step

	go get $(GOFLAGS) github.com/tools/godep

	touch $@

run: .prepare
	@echo Starting go web server
	$(GO) run $(GOFLAGS) main.go -config=config.json &
