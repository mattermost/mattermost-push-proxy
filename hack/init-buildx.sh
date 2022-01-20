#!/usr/bin/env bash

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

export DOCKER_CLI_EXPERIMENTAL=enabled

# Expected builder output
#
# Name:   k-release-multiarch
# Driver: docker-container
#
# Nodes:
# Name:      k-release-multiarch0
# Endpoint:  unix:///var/run/docker.sock
# Status:    running
# Platforms: linux/amd64, linux/arm64, linux/riscv64, linux/ppc64le, linux/s390x, linux/386, linux/arm/v7, linux/arm/v6
current_builder="$(docker buildx inspect)"

# We can skip setup if the current builder already has multi-arch
# AND if it isn't the "docker" driver, which doesn't work
#
# From https://docs.docker.com/buildx/working-with-buildx/#build-with-buildx:
# "You can run Buildx in different configurations that are exposed through a
# driver concept. Currently, Docker supports a “docker” driver that uses the
# BuildKit library bundled into the docker daemon binary, and a
# “docker-container” driver that automatically launches BuildKit inside a
# Docker container.
#
# The user experience of using Buildx is very similar across drivers.
# However, there are some features that are not currently supported by the
# “docker” driver, because the BuildKit library which is bundled into docker
# daemon uses a different storage component. In contrast, all images built with
# the “docker” driver are automatically added to the “docker images” view by
# default, whereas when using other drivers, the method for outputting an image
# needs to be selected with --output."
if ! grep -q "^Driver: docker$"  <<<"${current_builder}" \
  && grep -q "linux/amd64" <<<"${current_builder}" \
  && grep -q "linux/arm64" <<<"${current_builder}"; then
  exit 0
fi

# Ensure qemu is in binfmt_misc
# NOTE: Please always pin this to a digest for predictability/auditability
# Last updated: 2021-10-11
if [ "$(uname)" == 'Linux' ]; then
  docker run --rm --privileged tonistiigi/binfmt:qemu-v6.0.0@sha256:ce4d5a2a6ac4a189047fca2d71cbd901cc7beebacf538be95fccb3aca87cb2ec --install all
fi

# Ensure we use a builder that can leverage it (the default on linux will not)
docker context rm release-multiarch || true
docker buildx rm release-multiarch || true
docker context create release-multiarch
docker buildx create release-multiarch
docker buildx inspect --bootstrap
