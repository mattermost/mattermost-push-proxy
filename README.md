# Mattermost Push Notifications Service 

A server for proxying push notifications to iOS devices from Mattermost, [a self-hosted team communication solution](http://www.mattermost.org/). 

For organizations who want to keep internal communications behind their firewall, this service encrypts notification messages with a private key under your control before sending them to Apple's public push notification service for delivery to your iOS devices. 

### Requirements

1. A linux Ubuntu 14.04 server with at least 1GB of memory.  
2. Having either compiled the Mattermost iOS app and submitted it to the Apple App Store, or hosted in your own Enterprise App Store. 
3. Private and public keys obtained from the Apple Developer Program


### Installation 

1. Install the [latest release](https://github.com/mattermost/push-proxy/releases) of the Mattermost Notification Server.
  1. Create a directory, for example `/home/ubuntu/push-proxy`.
  2. Download Mattermost Notification Server v2.0 with `wget https://github.com/mattermost/push-proxy/releases/download/v2.0/matter-push-proxy.tar.gz`.
  3. Uncompress the file with `tar -xvzf matter-push-proxy.tar.gz`.
2. Update `config.json` with your private and public keys.
  1. Edit using `vi /home/ubuntu/push-proxy/config/config.json` and set `ApplePushCertPublic` and `ApplePushCertPrivate`, this should be a path to the public and private keys previously generated.  For example 
  ```
"ApplePushCertPublic": "./config/publickey.pem",
"ApplePushCertPrivate": "./config/privatekey.pem",
  ```
  2. Edit using `vi /home/ubuntu/push-proxy/config/config.json` and set `AndroidApiKey`, this should be a key generated from Google Cloud Messaging.  For example 
  ```
"AndroidApiKey": "DKJDIiwjerljd290u34jFKDSF",
  ```
3. Verify push notifications are working by mentioning a user who is offline, which should trigger a push notification.
4. You can verify that the server operates normally by using curl:
```
curl http://127.0.0.1:8066/api/v1/send_push -X POST -H "Content-Type: application/json" -d '{ "message":"test", "badge": 1, "platform":"apple", "server_id":"MATTERMOST_DIAG_ID", "device_id":"IPHONE_DEVICE_ID"}'
```
Replace MATTERMOST_DIAG_ID and IPHONE_DEVICE_ID with the relevant values.


### Troubleshooting 

The push-poxy logs to the console. If you're using Ubuntu 14.04 and `upstart` to start the process you can use something similar to the  following to send console output to a log file: 

- `/etc/init/matter-push-proxy.conf`

```
start on runlevel [2345]
stop on runlevel [016]
respawn
chdir /home/ubuntu/matter-push-proxy
setuid ubuntu
console log
exec bin/push-proxy | logger
```

To show the log file: 

```
sudo tail -n 1000 /var/log/upstart/
matter-push-proxy.log
```

