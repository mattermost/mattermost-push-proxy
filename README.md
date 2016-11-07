# Mattermost Push Notifications Service 

A server for proxying push notifications to iOS and Android devices from Mattermost, [a self-hosted team communication solution](http://www.mattermost.org/). 

For organizations who want to keep internal communications behind their firewall, this service encrypts notification messages with a private key under your control before sending them to Apple's public push notification service for delivery to your iOS devices. 

### Requirements

1. A linux Ubuntu 14.04 server with at least 1GB of memory
2. Either compile the Mattermost iOS app and submit it to the Apple App Store, or host it in your own Enterprise App Store
3. Private and public keys obtained from the Apple Developer Program
4. An Android API key generated from Google Cloud Messaging

### Obtaining Apple Developer Keys

1. Follow the directions at [developer.apple.com](https://developer.apple.com/library/content/documentation/IDEs/Conceptual/AppDistributionGuide/DistributingEnterpriseProgramApps/DistributingEnterpriseProgramApps.html#//apple_ref/doc/uid/TP40012582-CH33-SW4) to generate an Apple Push Notification service SSL Certificate, this should give you an `aps_production.cer`
2. `openssl x509 -in aps.cer -inform DER -out aps_production.pem`
3. Double click `aps_production.cer` to install it into the keychain tool
4. Right click the private cert in keychain access and export to .p12
5. `openssl pkcs12 -in Certificates.p12 -out aps_production_priv.pem -nodes -clcerts`
6. `openssl s_client -connect gateway.push.apple.com:2195 -cert aps_production.pem -key aps_production_priv.pem`

### Set Up Push Proxy Server

1. For the sake of making this guide simple we located the files at
   `/home/ubuntu/push-proxy`. 
2. We have also elected to run the Push Proxy Server as the `ubuntu` account for simplicity. We recommend setting up and running the service under a `matter-push-proxy` user account with limited permissions.
3. Download Mattermost Notification Server v2.0 by typing:

   -   `wget https://github.com/mattermost/push-proxy/releases/download/v2.0/matter-push-proxy.tar.gz`
   
4. Unzip the Push Proxy Server by typing:

   -  `tar -xvzf matter-push-proxy.tar.gz`

5. Configure Push Proxy Server by editing the config-push-proxy.json file at
   `/home/ubuntu/push-proxy/config`

   - Change directories by typing `cd ~/push-proxy/config`
   - Edit the file by typing `vi config-push-proxy.json`
   - Replace `"ApplePushCertPublic": ""` and `"ApplePushCertPrivate": ""` with a path to the public and private keys obtained from the Apple Developer Program
   - For `"AndroidApiKey": ""`, set the key generated from Google Cloud Messaging
   - For example: 
   
     ```
     "ApplePushCertPublic": "./config/aps_production.pem",
     "ApplePushCertPrivate": "./config/aps_production_priv.pem",
     "AndroidApiKey": "DKJDIiwjerljd290u34jFKDSF",
     ```

6. Setup Push Proxy to use the Upstart daemon which handles supervision
   of the Push Proxy process.

   -  `sudo touch /etc/init/matter-push-proxy.conf`
   -  `sudo vi /etc/init/matter-push-proxy.conf`
   -  Copy the following lines into `/etc/init/matter-push-proxy.conf`
     
     ```
     start on runlevel [2345]
     stop on runlevel [016]
     respawn
     chdir /home/ubuntu/matter-push-proxy
     setuid ubuntu
     console log
     exec bin/push-proxy | logger
     ```
     
   - You can manage the process by typing:
     -  `sudo start matter-push-proxy`
   - You can also stop the process by running the command `sudo stop matter-push-proxy`, but we will skip this step for now

   
7. Test the Push Proxy Server

   - Verify the server is functioning normally and test the push notifications using curl: 
     - `curl http://127.0.0.1:8066/api/v1/send_push -X POST -H "Content-Type: application/json" -d '{ "message":"test", "badge": 1, "platform":"apple", "server_id":"MATTERMOST_DIAG_ID", "device_id":"IPHONE_DEVICE_ID"}'`
     - Replace MATTERMOST_DIAG_ID with the value found by running the SQL query:
       - `SELECT * FROM Systems WHERE Name = 'DiagnosticId';`
     - Replace IPHONE_DEVICE_ID with your device ID, which can be found using: 
      ```
      SELECT
         Email, DeviceId
      FROM
         Sessions,
         Users
      WHERE
         Sessions.UserId = Users.Id
            AND DeviceId != ''
            AND Email = 'test@example.com'`
     ```
   - You can also verify push notifications are working by opening your Mattermost site and mentioning a user who has push notifications enabled in Account Settings > Notifications > Mobile Push Notifications
   - To view the log file, use: 
     
     ```
     sudo tail -n 1000 /var/log/upstart/
     matter-push-proxy.log
     ```

         
