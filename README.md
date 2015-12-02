# Mattermost Notifications Server 

A server for proxying push notifications to iOS devices from Mattermost, [a self-hosted team communication solution](http://www.mattermost.org/). 

For organizations who want to keep internal communications behind their firewall, this server encrypts notification messages with a private key under your control before sending them to Apple's public push notification server for delivery to your iOS devices. 

### Requirements

1. A linux Ubuntu 14.04 server with at least 1GB of memory.  
2. Private and public keys obtained from the Apple Developer Program

### Installation 

1. Install the [latest release](https://github.com/mattermost/push-proxy/releases) of the Mattermost Notification Server 
  1. Create a directory, for example `/home/ubuntu/push-proxy`
  2. Download Mattermost Notification Server v0.1.1 with `wget https://github.com/mattermost/push-proxy/releases/download/v0.1.1/matter-push-proxy.tar.gz`
  3. Uncompress the file with `tar -xvzf matter-push-proxy.tar.gz`
2. Update `config.json` with your private and public keys
  3. Edit `vi /home/ubuntu/push-proxy/config/config.json` and set `ApplePushCertPublic` and `ApplePushCertPrivate` based on the public and private keys previously generated 
3. Start the Mattermost Notification Server
  1. `sudo start push-proxy`
4. Verify push notifications are working by mentioning a user who is offline
