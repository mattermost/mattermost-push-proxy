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
  2. Download Mattermost Notification Server v0.1.1 with `wget https://github.com/mattermost/push-proxy/releases/download/v0.1.1/matter-push-proxy.tar.gz`.
  3. Uncompress the file with `tar -xvzf matter-push-proxy.tar.gz`.
2. Update `config.json` with your private and public keys.
  1. Edit using `vi /home/ubuntu/push-proxy/config/config.json` and set `ApplePushCertPublic` and `ApplePushCertPrivate`, this should be a path to the public and private keys previously generated. 
3. Verify push notifications are working by mentioning a user who is offline, which should trigger a push notification.
