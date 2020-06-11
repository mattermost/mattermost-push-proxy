FROM alpine:3.11 AS downloader

ARG MM_PROXY_PACKAGE=https://github.com/mattermost/mattermost-push-proxy/releases/download/v5.21/mattermost-push-proxy.tar.gz

RUN apk add --no-cache \
    ca-certificates \
    libc6-compat \
    libffi-dev \
    linux-headers \
    netcat-openbsd \
    tzdata \
    && rm -rf /tmp/* \
    && mkdir -p /mattermost-push-proxy \
    && wget $MM_PROXY_PACKAGE \
    && tar -xzvf mattermost-push-proxy.tar.gz \
    && rm mattermost-push-proxy.tar.gz \
    && chown -R nobody:nogroup /mattermost-push-proxy

USER nobody

WORKDIR /mattermost-push-proxy

ENV PUSH_PROXY=/mattermost-push-proxy/bin/mattermost-push-proxy

COPY entrypoint /usr/local/bin/

EXPOSE 8066

VOLUME ["/mattermost-push-proxy/config", "/mattermost-push-proxy/certs"]

ENTRYPOINT ["/usr/local/bin/entrypoint"]
