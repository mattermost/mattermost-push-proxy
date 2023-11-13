FROM alpine:3.15 AS TMP

WORKDIR /mattermost-push-proxy

COPY dist/ dist/
COPY docker/entrypoint .

ARG ARCH
COPY bin/mattermost-push-proxy-linux-$ARCH bin/mattermost-push-proxy

FROM alpine:3.15

# hadolint ignore=DL3018
RUN apk add --no-cache \
  ca-certificates \
  libc6-compat \
  libffi-dev \
  linux-headers \
  netcat-openbsd \
  tzdata \
  && rm -rf /tmp/* \
  && mkdir -p mattermost-push-proxy/bin \
  && chown -R nobody:nogroup /mattermost-push-proxy

COPY --from=TMP /mattermost-push-proxy/dist /
COPY --from=TMP /mattermost-push-proxy/bin/ /mattermost-push-proxy/bin/
COPY --from=TMP /mattermost-push-proxy/entrypoint /usr/local/bin/

USER nobody

WORKDIR /mattermost-push-proxy

ENV PUSH_PROXY=/mattermost-push-proxy/bin/mattermost-push-proxy

EXPOSE 8066

VOLUME ["/mattermost-push-proxy/config", "/mattermost-push-proxy/certs"]

ENTRYPOINT ["/usr/local/bin/entrypoint"]
