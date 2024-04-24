FROM --platform=${TARGETPLATFORM} ubuntu:jammy-20240405 AS builder
ARG TARGETARCH

# Setting bash as our shell, and enabling pipefail option
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Copying tarball
WORKDIR /mattermost-push-proxy
COPY dist/mattermost-push-proxy-linux-${TARGETARCH}.tar.gz /mattermost-push-proxy-linux.tar.gz
RUN tar -xf /mattermost-push-proxy-linux.tar.gz --strip-components=1  -C /mattermost-push-proxy

FROM --platform=${TARGETPLATFORM} ubuntu:jammy-20240405

# Install needed packages and indirect dependencies
# hadolint ignore=DL3008
RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
  ca-certificates \
  libffi-dev \
  netcat-openbsd \
  tzdata \
  && rm -rf /var/lib/apt/lists/*

# Coyping needed files from previous stage
COPY --from=builder /mattermost-push-proxy/ /mattermost-push-proxy/
COPY docker/entrypoint /usr/local/bin/

RUN chown -R nobody:nogroup /mattermost-push-proxy

USER nobody
WORKDIR /mattermost-push-proxy
ENV PUSH_PROXY=/mattermost-push-proxy/bin/mattermost-push-proxy

EXPOSE 8066
VOLUME ["/mattermost-push-proxy/config", "/mattermost-push-proxy/certs"]

ENTRYPOINT ["/usr/local/bin/entrypoint"]
