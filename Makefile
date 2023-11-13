# ====================================================================================
# Variables

## General Variables

# Branch Variables
PROTECTED_BRANCH := master
CURRENT_BRANCH   := $(shell git rev-parse --abbrev-ref HEAD)
# Use repository name as application name
APP_NAME    := $(shell basename -s .git `git config --get remote.origin.url`)
# Get current commit
APP_COMMIT  := $(shell git log --pretty=format:'%h' -n 1)
# Check if we are in protected branch, if yes use `protected_branch_name-sha` as app version.
# Else check if we are in a release tag, if yes use the tag as app version, else use `dev-sha` as app version.
APP_VERSION ?= $(shell if [ $(PROTECTED_BRANCH) = $(CURRENT_BRANCH) ]; then echo $(PROTECTED_BRANCH); else (git describe --abbrev=0 --exact-match --tags 2>/dev/null || echo dev-$(APP_COMMIT)) ; fi)
GIT_VERSION ?= $(shell git describe --tags --always --dirty)
GIT_TREESTATE = clean
DIFF = $(shell git diff --quiet >/dev/null 2>&1; if [ $$? -eq 1 ]; then echo "1"; fi)
ifeq ($(DIFF), 1)
    GIT_TREESTATE = dirty
endif

# Get current date and format like: 2022-04-27 11:32
BUILD_DATE  := $(shell date +%Y-%m-%d\ %H:%M)

## General Configuration Variables
# We don't need make's built-in rules.
MAKEFLAGS     += --no-builtin-rules
# Be pedantic about undefined variables.
MAKEFLAGS     += --warn-undefined-variables
# Set help as default target
.DEFAULT_GOAL := help

# App Code location
CONFIG_APP_CODE         += ./

## Docker Variables
# Docker executable
DOCKER                  := $(shell which docker)
# Dockerfile's location
DOCKER_FILE             ?= ./docker/Dockerfile
# Docker options to inherit for all docker run commands
DOCKER_OPTS             += --rm --platform "linux/amd64"
# Registry to upload images
DOCKER_REGISTRY         ?= docker.io
DOCKER_REGISTRY_REPO    ?= mattermost/${APP_NAME}-daily
# Registry credentials
DOCKER_USER             ?= user
DOCKER_PASSWORD         ?= password
## Docker Images
DOCKER_IMAGE_GO         ?= "golang:${GO_VERSION}@sha256:dd9ad81920b63c7f9f18823d888d5fdcc7e7516086fd16654d07bc437f0e2427"
DOCKER_IMAGE_GOLINT     ?= "golangci/golangci-lint:v1.52.2@sha256:5fa6a92ab28ca3421c88d2b6cd794c9759d05a999aceca73053d014aad41b9d3"
DOCKER_IMAGE_DOCKERLINT ?= "hadolint/hadolint:v2.9.2@sha256:d355bd7df747a0f124f3b5e7b21e9dafd0cb19732a276f901f0fdee243ec1f3b"
DOCKER_IMAGE_COSIGN     ?= "bitnami/cosign:1.8.0@sha256:8c2c61c546258fffff18b47bb82a65af6142007306b737129a7bd5429d53629a"
DOCKER_IMAGE_GH_CLI     ?= "ghcr.io/supportpal/github-gh-cli:2.31.0@sha256:71371e36e62bd24ddd42d9e4c720a7e9954cb599475e24d1407af7190e2a5685"

## Cosign Variables
# The public key
COSIGN_PUBLIC_KEY       ?= akey
# The private key
COSIGN_KEY              ?= akey
# The passphrase used to decrypt the private key
COSIGN_PASSWORD         ?= password

## Go Variables
# Go executable
GO                           := $(shell which go)
# Extract GO version from go.mod file
GO_VERSION                   ?= $(shell grep -E '^go' go.mod | awk {'print $$2'})
# LDFLAGS
GO_LDFLAGS                   += -X "github.com/mattermost/${APP_NAME}/internal/version.gitVersion=$(GIT_VERSION)"
GO_LDFLAGS                   += -X "github.com/mattermost/${APP_NAME}/internal/version.gitCommit=$(APP_COMMIT)"
GO_LDFLAGS                   += -X "github.com/mattermost/${APP_NAME}/internal/version.gitTreeState=$(GIT_TREESTATE)"
GO_LDFLAGS                   += -X "github.com/mattermost/${APP_NAME}/internal/version.buildDate=$(BUILD_DATE)"


# Architectures to build for
GO_BUILD_PLATFORMS           ?= linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 freebsd-amd64
GO_BUILD_PLATFORMS_ARTIFACTS = $(foreach cmd,$(addprefix go-build/,${APP_NAME}),$(addprefix $(cmd)-,$(GO_BUILD_PLATFORMS)))

# Build options
GO_BUILD_OPTS                += -trimpath
GO_TEST_OPTS                 += -v -timeout=180s
# Temporary folder to output compiled binaries artifacts
GO_OUT_BIN_DIR               := ./dist

## Github Variables
# A github access token that provides access to upload artifacts under releases
GITHUB_TOKEN                 ?= a_token
# Github organization
GITHUB_ORG                   := mattermost
# Most probably the name of the repo
GITHUB_REPO                  := ${APP_NAME}

# ====================================================================================
# Colors

BLUE   := $(shell printf "\033[34m")
YELLOW := $(shell printf "\033[33m")
RED    := $(shell printf "\033[31m")
GREEN  := $(shell printf "\033[32m")
CYAN   := $(shell printf "\033[36m")
CNone  := $(shell printf "\033[0m")

# ====================================================================================
# Logger

TIME_LONG	= `date +%Y-%m-%d' '%H:%M:%S`
TIME_SHORT	= `date +%H:%M:%S`
TIME		= $(TIME_SHORT)

INFO = echo ${TIME} ${BLUE}[ .. ]${CNone}
WARN = echo ${TIME} ${YELLOW}[WARN]${CNone}
ERR  = echo ${TIME} ${RED}[FAIL]${CNone}
OK   = echo ${TIME} ${GREEN}[ OK ]${CNone}
FAIL = (echo ${TIME} ${RED}[FAIL]${CNone} && false)

# ====================================================================================
# Verbosity control hack

VERBOSE ?= 0
AT_0 := @
AT_1 :=
AT = $(AT_$(VERBOSE))

# ====================================================================================
# Targets

help: ## to get help
	@echo "Usage:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) |\
	awk 'BEGIN {FS = ":.*?## "}; {printf "make ${CYAN}%-30s${CNone} %s\n", $$1, $$2}'

.PHONY: build
build: go-build-docker ## to build

.PHONY: release
release: build github-release ## to build and release artifacts

.PHONY: package
package: go-build package-software ## to build, package

.PHONY: sign
sign: docker-sign docker-verify ## to sign the artifact and perform verification

.PHONY: lint
lint: go-lint docker-lint ## to lint

.PHONY: test
test: go-test ## to test

package-software:  ## to package the binary
	@$(INFO) Packaging
	$(AT) for file in $(GO_OUT_BIN_DIR)/mattermost-push-proxy-*; do \
		target=$$(basename $$file); \
		mkdir -p $(GO_OUT_BIN_DIR)/$${target}_temp/bin; \
		cp -RL config $(GO_OUT_BIN_DIR)/$${target}_temp/config; \
		echo $(APP_VERSION) > $(GO_OUT_BIN_DIR)/$${target}_temp/config/build.txt; \
		cp LICENSE.txt NOTICE.txt README.md $(GO_OUT_BIN_DIR)/$${target}_temp; \
		mv $$file $(GO_OUT_BIN_DIR)/$${target}_temp/bin/mattermost-push-proxy; \
		mv $(GO_OUT_BIN_DIR)/$${target}_temp $(GO_OUT_BIN_DIR)/$${target}; \
		tar -czf $(GO_OUT_BIN_DIR)/$${target}.tar.gz -C $(GO_OUT_BIN_DIR) $${target}; \
		rm -r $(GO_OUT_BIN_DIR)/$${target}; \
	done
	@$(OK) Packaging

.PHONY: docker-build
docker-build: ## to build the docker image
	@$(INFO) Performing Docker build ${APP_NAME}:${APP_VERSION}
	$(AT)$(DOCKER) build \
	--build-arg GO_IMAGE=${DOCKER_IMAGE_GO} \
	--build-arg=ARCH=amd64 \
	-f ${DOCKER_FILE} . \
	-t ${APP_NAME}:${APP_VERSION} || ${FAIL}
	@$(OK) Performing Docker build ${APP_NAME}:${APP_VERSION}

.PHONY: docker-push
docker-push: ## to push the docker image
	@$(INFO) Pushing to registry...
	$(AT)$(DOCKER) tag ${APP_NAME}:${APP_VERSION} $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:${APP_VERSION} || ${FAIL}
	$(AT)$(DOCKER) push $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:${APP_VERSION} || ${FAIL}
# if we are on a latest semver APP_VERSION tag, also push latest
ifneq ($(shell echo $(APP_VERSION) | egrep '^v([0-9]+\.){0,2}(\*|[0-9]+)'),)
  ifeq ($(shell git tag -l --sort=v:refname | tail -n1),$(APP_VERSION))
	$(AT)$(DOCKER) tag ${APP_NAME}:${APP_VERSION} $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:latest || ${FAIL}
	$(AT)$(DOCKER) push $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:latest || ${FAIL}
  endif
endif
	@$(OK) Pushing to registry $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:${APP_VERSION}

.PHONY: docker-sign
docker-sign: ## to sign the docker image
	@$(INFO) Signing the docker image...
	$(AT)echo "$${COSIGN_KEY}" > cosign.key && \
	$(DOCKER) run ${DOCKER_OPTS} \
	--entrypoint '/bin/sh' \
        -v $(PWD):/app -w /app \
	-e COSIGN_PASSWORD=${COSIGN_PASSWORD} \
	-e HOME="/tmp" \
    ${DOCKER_IMAGE_COSIGN} \
	-c \
	"echo Signing... && \
	cosign login $(DOCKER_REGISTRY) -u ${DOCKER_USER} -p ${DOCKER_PASSWORD} && \
	cosign sign --key cosign.key $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:${APP_VERSION}" || ${FAIL}
# if we are on a latest semver APP_VERSION tag, also sign latest tag
ifneq ($(shell echo $(APP_VERSION) | egrep '^v([0-9]+\.){0,2}(\*|[0-9]+)'),)
  ifeq ($(shell git tag -l --sort=v:refname | tail -n1),$(APP_VERSION))
	$(DOCKER) run ${DOCKER_OPTS} \
	--entrypoint '/bin/sh' \
        -v $(PWD):/app -w /app \
	-e COSIGN_PASSWORD=${COSIGN_PASSWORD} \
	-e HOME="/tmp" \
	${DOCKER_IMAGE_COSIGN} \
	-c \
	"echo Signing... && \
	cosign login $(DOCKER_REGISTRY) -u ${DOCKER_USER} -p ${DOCKER_PASSWORD} && \
	cosign sign --key cosign.key $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:latest" || ${FAIL}
  endif
endif
	$(AT)rm -f cosign.key || ${FAIL}
	@$(OK) Signing the docker image: $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:${APP_VERSION}

.PHONY: docker-verify
docker-verify: ## to verify the docker image
	@$(INFO) Verifying the published docker image...
	$(AT)echo "$${COSIGN_PUBLIC_KEY}" > cosign_public.key && \
	$(DOCKER) run ${DOCKER_OPTS} \
	--entrypoint '/bin/sh' \
	-v $(PWD):/app -w /app \
	${DOCKER_IMAGE_COSIGN} \
	-c \
	"echo Verifying... && \
	cosign verify --key cosign_public.key $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:${APP_VERSION}" || ${FAIL}
# if we are on a latest semver APP_VERSION tag, also verify latest tag
ifneq ($(shell echo $(APP_VERSION) | egrep '^v([0-9]+\.){0,2}(\*|[0-9]+)'),)
  ifeq ($(shell git tag -l --sort=v:refname | tail -n1),$(APP_VERSION))
	$(DOCKER) run ${DOCKER_OPTS} \
	--entrypoint '/bin/sh' \
	-v $(PWD):/app -w /app \
	${DOCKER_IMAGE_COSIGN} \
	-c \
	"echo Verifying... && \
	cosign verify --key cosign_public.key $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:latest" || ${FAIL}
  endif
endif
	$(AT)rm -f cosign_public.key || ${FAIL}
	@$(OK) Verifying the published docker image: $(DOCKER_REGISTRY)/${DOCKER_REGISTRY_REPO}:${APP_VERSION}

.PHONY: docker-sbom
docker-sbom: ## to print a sbom report
	@$(INFO) Performing Docker sbom report...
	$(AT)$(DOCKER) sbom ${APP_NAME}:${APP_VERSION} || ${FAIL}
	@$(OK) Performing Docker sbom report

.PHONY: docker-scan
docker-scan: ## to print a vulnerability report
	@$(INFO) Performing Docker scan report...
	$(AT)$(DOCKER) scan ${APP_NAME}:${APP_VERSION} || ${FAIL}
	@$(OK) Performing Docker scan report

.PHONY: docker-lint
docker-lint: ## to lint the Dockerfile
	@$(INFO) Dockerfile linting...
	$(AT)$(DOCKER) run -i ${DOCKER_OPTS} \
	${DOCKER_IMAGE_DOCKERLINT} \
	< ${DOCKER_FILE} || ${FAIL}
	@$(OK) Dockerfile linting

.PHONY: docker-login
docker-login: ## to login to a container registry
	@$(INFO) Dockerd login to container registry ${DOCKER_REGISTRY}...
	$(AT) echo "${DOCKER_PASSWORD}" | $(DOCKER) login --password-stdin -u ${DOCKER_USER} $(DOCKER_REGISTRY) || ${FAIL}
	@$(OK) Dockerd login to container registry ${DOCKER_REGISTRY}...

go-build: $(GO_BUILD_PLATFORMS_ARTIFACTS) ## to build binaries

.PHONY: go-build
go-build/%:
	@$(INFO) go build $*...
	$(AT)target="$*"; \
	command="${APP_NAME}"; \
	platform_ext="$${target#$$command-*}"; \
	platform="$${platform_ext%.*}"; \
	export GOOS="$${platform%%-*}"; \
	export GOARCH="$${platform#*-}"; \
	echo export GOOS=$${GOOS}; \
	echo export GOARCH=$${GOARCH}; \
	CGO_ENABLED=0 \
	$(GO) build ${GO_BUILD_OPTS} \
	-ldflags '${GO_LDFLAGS}' \
	-o ${GO_OUT_BIN_DIR}/$* \
	${CONFIG_APP_CODE} || ${FAIL}
	@$(OK) go build $*

.PHONY: go-build-docker
go-build-docker: # to build binaries under a controlled docker dedicated go container using DOCKER_IMAGE_GO
	@$(INFO) go build docker
	$(AT)$(DOCKER) run  \
	-v $(PWD):/app -w /app \
	$(DOCKER_IMAGE_GO) \
	/bin/sh -c \
	"cd /app && \
	make go-build"  || ${FAIL}
	@$(OK) go build docker

.PHONY: go-run
go-run: ## to run locally for development
	@$(INFO) running locally...
	$(AT)$(GO) run ${GO_BUILD_OPTS} ${CONFIG_APP_CODE} || ${FAIL}
	@$(OK) running locally

.PHONY: go-test
go-test: ## to run tests
	@$(INFO) testing...
	$(AT)$(DOCKER) run ${DOCKER_OPTS} \
	-v $(PWD):/app -w /app \
	$(DOCKER_IMAGE_GO) \
	/bin/sh -c \
	"cd /app && \
	go test ${GO_TEST_OPTS} ./... " || ${FAIL}
	@$(OK) testing

.PHONY: go-mod-check
go-mod-check: ## to check go mod files consistency
	@$(INFO) Checking go mod files consistency...
	$(AT)$(GO) mod tidy
	$(AT)git --no-pager diff --exit-code go.mod go.sum || \
	(${WARN} Please run "go mod tidy" and commit the changes in go.mod and go.sum. && ${FAIL} ; exit 128 )
	@$(OK) Checking go mod files consistency

.PHONY: go-lint
go-lint: ## to lint go code
	@$(INFO) App linting...
	$(AT)$(DOCKER) run ${DOCKER_OPTS} \
	-v $(PWD):/app -w /app \
	${DOCKER_IMAGE_GOLINT} \
	golangci-lint run ./... || ${FAIL}
	@$(OK) App linting

.PHONY: go-doc
go-doc: ## to generate documentation
	@$(INFO) Generating Documentation...
	$(AT)$(GO) run ./scripts/env_config.go ./docs/env_config.md || ${FAIL}
	@$(OK) Generating Documentation

.PHONY: github-release
github-release: ## to publish a release and relevant artifacts to GitHub
	@$(INFO) Generating github-release http://github.com/$(GITHUB_ORG)/$(GITHUB_REPO)/releases/tag/$(APP_VERSION) ...
ifeq ($(shell echo $(APP_VERSION) | egrep '^v([0-9]+\.){0,2}(\*|[0-9]+)'),)
	$(error "We only support releases from semver tags")
else
	$(AT)$(DOCKER) run \
	-v $(PWD):/app -w /app \
	-e GITHUB_TOKEN=${GITHUB_TOKEN} \
	$(DOCKER_IMAGE_GH_CLI) \
	/bin/sh -c \
	"git config --global --add safe.directory /app && cd /app && \
	gh release create $(APP_VERSION) --generate-notes $(GO_OUT_BIN_DIR)/*" || ${FAIL}
endif
	@$(OK) Generating github-release http://github.com/$(GITHUB_ORG)/$(GITHUB_REPO)/releases/tag/$(APP_VERSION) ...

.PHONY: clean
clean: ## to clean-up
	@$(INFO) cleaning /${GO_OUT_BIN_DIR} folder...
	$(AT)rm -rf ${GO_OUT_BIN_DIR} || ${FAIL}
	@$(OK) cleaning /${GO_OUT_BIN_DIR} folder
