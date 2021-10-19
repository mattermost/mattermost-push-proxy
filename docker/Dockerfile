FROM alpine:3.14

ARG MM_PROXY_PACKAGE=https://github.com/mattermost/mattermost-push-proxy/releases/download/v5.22.5

ARG TARGETPLATFORM

RUN targetArch=$(echo $TARGETPLATFORM | cut -f2 -d '/') \
  && if [ ${targetArch} = "amd64" ]; then \
  PUSH_PACKAGE="mattermost-push-proxy-linux-amd64.tar.gz"; \
elif [ ${targetArch} = "arm64" ]; then \
  PUSH_PACKAGE="mattermost-push-proxy-linux-arm64.tar.gz"; \
fi \
    && apk add --no-cache \
    ca-certificates \
    libc6-compat \
    libffi-dev \
    linux-headers \
    netcat-openbsd \
    tzdata \
    && rm -rf /tmp/* \
    && mkdir -p /mattermost-push-proxy \
    && wget $MM_PROXY_PACKAGE/"${PUSH_PACKAGE}" \
    && tar -xzvf "${PUSH_PACKAGE}" \
    && rm "${PUSH_PACKAGE}" \
    && chown -R nobody:nogroup /mattermost-push-proxy

USER nobody

WORKDIR /mattermost-push-proxy

ENV PUSH_PROXY=/mattermost-push-proxy/bin/mattermost-push-proxy

COPY docker/entrypoint /usr/local/bin/

EXPOSE 8066

VOLUME ["/mattermost-push-proxy/config", "/mattermost-push-proxy/certs"]

ENTRYPOINT ["/usr/local/bin/entrypoint"]
