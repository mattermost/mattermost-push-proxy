FROM ubuntu:noble-20240127.1@sha256:36fa0c7153804946e17ee951fdeffa6a1c67e5088438e5b90de077de5c600d4c AS TMP

# Setting bash as our shell, and enabling pipefail option
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Copying binaries
WORKDIR /mattermost-push-proxy
COPY dist/ dist/
COPY docker/entrypoint .
ARG ARCH
COPY bin/mattermost-push-proxy-linux-$ARCH bin/mattermost-push-proxy

#Using multi stage build
FROM ubuntu:noble-20240127.1@sha256:36fa0c7153804946e17ee951fdeffa6a1c67e5088438e5b90de077de5c600d4c

# Install needed packages and indirect dependencies
# hadolint ignore=DL3008
RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
  ca-certificates \
  libffi-dev \
  netcat-openbsd \
  tzdata \
  && rm -rf /var/lib/apt/lists/* \
  && mkdir -p mattermost-push-proxy/bin \
  && chown -R nobody:nogroup /mattermost-push-proxy

# Coyping needed files from previous stage
COPY --from=TMP /mattermost-push-proxy/dist /
COPY --from=TMP /mattermost-push-proxy/bin/ /mattermost-push-proxy/bin/
COPY --from=TMP /mattermost-push-proxy/entrypoint /usr/local/bin/

USER nobody
WORKDIR /mattermost-push-proxy
ENV PUSH_PROXY=/mattermost-push-proxy/bin/mattermost-push-proxy
EXPOSE 8066
VOLUME ["/mattermost-push-proxy/config", "/mattermost-push-proxy/certs"]
ENTRYPOINT ["/usr/local/bin/entrypoint"]
