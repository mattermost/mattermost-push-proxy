.PHONY: all dist build build-release build-local package test clean run update-dependencies golangci-lint

GOFLAGS ?= $(GOFLAGS:)
LDFLAGS ?= $(LDFLAGS:)

export GOBIN = $(PWD)/bin
GO=go

# Set version variables for LDFLAGS
GIT_VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_HASH = $(shell git rev-parse --short HEAD)
BUILD_TAG_LATEST = $(shell git describe --tags --match 'v*' --abbrev=0)
BUILD_TAG_CURRENT = $(shell git tag --points-at HEAD)
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
LDFLAGS="-X $(PP_PKG).gitVersion=$(GIT_VERSION) -X $(PP_PKG).buildHash=$(BUILD_HASH) -X $(PP_PKG).buildTagLatest=$(BUILD_TAG_LATEST) -X $(PP_PKG).buildTagCurrent=$(BUILD_TAG_CURRENT) -X $(PP_PKG).gitTreeState=$(GIT_TREESTATE) -X $(PP_PKG).buildDate=$(BUILD_DATE)"

DIST_ROOT=dist
DIST_PATH=$(DIST_ROOT)/mattermost-push-proxy

all: dist

dist: | build-release test package

update-dependencies:
	$(GO) get -u ./...
	$(GO) mod tidy

check-deps:
	$(GO) mod tidy -v
	@if [ -n "$$(command git --no-pager diff --exit-code go.mod go.sum)" ]; then \
		echo "There are unused dependencies that should be removed. Please execute `go mod tidy` to fix it."; \
		exit 1; \
	fi

build-release:
	@echo Building proxy push server
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(GOBIN)/mattermost-push-proxy-linux-amd64 -trimpath -ldflags $(LDFLAGS) $(GOFLAGS)
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -o $(GOBIN)/mattermost-push-proxy-linux-arm64 -trimpath -ldflags $(LDFLAGS) $(GOFLAGS)

build-local: # build push proxy for the current arch
	@echo Building proxy push server
	env CGO_ENABLED=0 $(GO) build -o $(GOBIN) -trimpath -ldflags $(LDFLAGS) $(GOFLAGS)

golangci-lint: ## Run golangci-lint on codebase
# https://stackoverflow.com/a/677212/1027058 (check if a command exists or not)
	@if ! [ -x "$$(command -v golangci-lint)" ]; then \
		echo "golangci-lint is not installed. Please see https://github.com/golangci/golangci-lint#install for installation instructions."; \
		exit 1; \
	fi; \

	@echo Running golangci-lint
	golangci-lint run ./...

package-linux-amd64:
	@ echo Packaging push proxy for linux amd64

	mkdir -p $(DIST_PATH)/bin
	cp $(GOBIN)/mattermost-push-proxy-linux-amd64 $(DIST_PATH)/bin/mattermost-push-proxy

	cp -RL config $(DIST_PATH)/config
	touch $(DIST_PATH)/config/build.txt
	echo $(BUILD_NUMBER) | tee -a $(DIST_PATH)/config/build.txt

	mkdir -p $(DIST_PATH)/logs

	cp LICENSE.txt $(DIST_PATH)
	cp NOTICE.txt $(DIST_PATH)
	cp README.md $(DIST_PATH)

	tar -C dist -czf $(DIST_PATH)-linux-amd64.tar.gz mattermost-push-proxy
	rm -rf $(DIST_PATH)

package-linux-arm64:
	@ echo Packaging push proxy for linux arm64

	mkdir -p $(DIST_PATH)/bin
	cp $(GOBIN)/mattermost-push-proxy-linux-arm64 $(DIST_PATH)/bin/mattermost-push-proxy

	cp -RL config $(DIST_PATH)/config
	touch $(DIST_PATH)/config/build.txt
	echo $(BUILD_NUMBER) | tee -a $(DIST_PATH)/config/build.txt

	mkdir -p $(DIST_PATH)/logs

	cp LICENSE.txt $(DIST_PATH)
	cp NOTICE.txt $(DIST_PATH)
	cp README.md $(DIST_PATH)

	tar -C dist -czf $(DIST_PATH)-linux-arm64.tar.gz mattermost-push-proxy

package: build-release package-linux-arm64 package-linux-amd64
	cd dist \
	sha256sum mattermost-push-proxy-linux-amd64.tar.gz >> checksums.txt \
	&& sha256sum mattermost-push-proxy-linux-arm64.tar.gz >> checksums.txt

package-image: build-release
	mkdir -p $(DIST_PATH)/bin

	cp -RL config $(DIST_PATH)/config
	touch $(DIST_PATH)/config/build.txt
	echo $(BUILD_NUMBER) | tee -a $(DIST_PATH)/config/build.txt

	mkdir -p $(DIST_PATH)/logs

	cp LICENSE.txt $(DIST_PATH)
	cp NOTICE.txt $(DIST_PATH)
	cp README.md $(DIST_PATH)

PLATFORMS ?= linux/amd64 linux/arm64
ARCHS = $(patsubst linux/%,%,$(PLATFORMS))
IMAGE ?= mattermost/mattermost-push-proxy
TAG ?= $(shell git describe --tags --always --dirty)

# build with buildx
.PHONY: container
container: package-image
	@for platform in $(PLATFORMS); do \
		echo "Starting build for $${platform} platform"; \
		docker buildx build \
			--load \
			--progress plain \
			--platform $${platform} \
			--build-arg=ARCH=$${platform##*/} \
			--tag $(IMAGE)-$${platform##*/}:$(TAG) \
			--file docker/Dockerfile \
			.; \
	done

.PHONY: push
push: container
	echo "Pushing $(IMGNAME) tags"
	@for platform in $(PLATFORMS); do \
		echo "Pushing tags for $${platform} platform"; \
		docker push $(IMAGE)-$${platform##*/}:$(TAG); \
	done

.PHONY: manifest
manifest: push
	docker manifest create --amend $(IMAGE):$(TAG) $(shell echo $(ARCHS) | sed -e "s~[^ ]*~$(IMAGE)\-&:$(TAG)~g")
	@for platform in $(ARCHS); do docker manifest annotate --arch "$${platform}" ${IMAGE}:${TAG} ${IMAGE}-$${platform}:${TAG}; done
	docker manifest push --purge $(IMAGE):$(TAG)

test:
	$(GO) test $(GOFLAGS) -v -timeout=180s ./...

clean:
	@echo Cleaning
	rm -Rf $(DIST_ROOT)
	go clean $(GOFLAGS) -i ./...

run:
	@echo Starting go web server
	$(GO) run $(GOFLAGS) -ldflags $(LDFLAGS) main.go

build-swagger:
	npm run validate
	npm run build

serve-swagger:
	npm run validate
	npm run serve
