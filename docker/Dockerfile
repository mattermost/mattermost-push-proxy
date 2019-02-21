FROM ubuntu:16.04

ENV APP_VERSION=4.4.0
ENV PATH="/mattermost-push-proxy/bin:${PATH}"

# Install some needed packages
RUN apt-get update \
    && apt-get -y --no-install-recommends install \
      curl \
      wget \
      netcat \
      ca-certificates \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin

RUN mkdir -p /mattermost-push-proxy \
    && cd /mattermost-push-proxy \
    && wget https://github.com/mattermost/mattermost-push-proxy/releases/download/v`echo $APP_VERSION | cut -d. -f1,2`/mattermost-push-proxy-$APP_VERSION.tar.gz \
    && tar -xvzf mattermost-push-proxy-$APP_VERSION.tar.gz \
    && rm mattermost-push-proxy-$APP_VERSION.tar.gz \
    && mv mattermost-push-proxy/* ./ \
    && rm -rf mattermost-push-proxy

WORKDIR /mattermost-push-proxy

COPY entrypoint.sh /

EXPOSE 8066

VOLUME ["/mattermost-push-proxy", "/config", "/certs"]

ENTRYPOINT ["/entrypoint.sh"]
