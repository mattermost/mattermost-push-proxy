#!/bin/bash

#Read environment variables
APPLE_PUSH_CERT_PASSWORD=${APPLE_PUSH_CERT_PASSWORD:-}
APPLE_PUSH_TOPIC=${APPLE_PUSH_TOPIC:-com.mattermost.Mattermost}
ANDROID_API_KEY=${ANDROID_API_KEY:-}

if [ -f /config/config.json ]; then
  echo "Using existing config file"
else
  echo "Configuring using environment variables"
  cp /mattermost-push-proxy/config/mattermost-push-proxy.json /config/config.json
  cat /config/config.json
  if [ -f /certs/apple-push-cert.pem ]; then
    sed -i -e "s/ApplePushCertPrivate.*/ApplePushCertPrivate\":\"/certs\/apple-push-cert.pem\",/" /config/config.json
    sed -i -e "s/ApplePushCertPassword.*/ApplePushCertPassword\":\"\/certs\/apple-push-cert.pem\",/" /config/config.json
  fi
  sed -i -e "s/ApplePushTopic.*/ApplePushTopic\":\"$APPLE_PUSH_TOPIC\"/" /config/config.json
  sed -i -e "s/AndroidApiKey.*/AndroidApiKey\":\"$ANDROID_API_KEY\"/" /config/config.json
  cat /config/config.json
fi

mattermost-push-proxy --config /config/config.json
