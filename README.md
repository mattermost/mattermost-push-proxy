# Mattermost Push Notifications Service 

A server for proxying push notifications to iOS and Android devices from Mattermost, [a self-hosted team communication solution](https://about.mattermost.com/). 

See our [mobile applications deployment guide](https://docs.mattermost.com/mobile/mobile-overview.html) for details on how MPNS works with your Mattermost server and mobile applications.

For organizations who want to keep internal communications behind their firewall, this service encrypts notification messages with a private key under your control before sending them to Apple's public push notification service for delivery to your Android and iOS devices. 

### Requirements

1. A linux Ubuntu 14.04 server with at least 1GB of memory
2. Either compile the Mattermost Android and iOS apps and submit it to the App Stores, or host it in your own Enterprise App Store
3. Private and public keys obtained from the Apple Developer Program
4. An Android API key generated from Google Cloud Messaging

### Set Up Push Proxy Server

1. For the sake of making this guide simple we located the files at
   `/home/ubuntu/mattermost-push-proxy`. 
2. We have also elected to run the Push Proxy Server as the `ubuntu` account for simplicity. We recommend setting up and running the service under a `mattermost-push-proxy` user account with limited permissions.
3. Download Mattermost Notification Server v2.0 by typing:

   -   `wget https://github.com/mattermost/mattermost-push-proxy/releases/download/vX.X/mattermost-push-proxy.tar.gz`
   
4. Unzip the Push Proxy Server by typing:

   -  `tar -xvzf mattermost-push-proxy.tar.gz`

5. Configure Push Proxy Server by editing the mattermost-push-proxy.json file at
   `/home/ubuntu/mattermost-push-proxy/config`, read the [Push Notifications with Your Own Build](https://docs.mattermost.com/developer/mobile-developer-setup.html#push-notifications-with-your-own-build) documentation to learn more.

   - Change directories by typing `cd ~/mattermost-push-proxy/config`
   - Edit the file by typing `vi mattermost-push-proxy.json`
   - Replace `"ApplePushCertPrivate": ""` with a path to the public and private keys obtained from the Apple Developer Program, in two places.
   - For `"AndroidApiKey": ""`, set the key generated from Google Cloud Messaging, in two places.
   - Replace `"ApplePushTopic": "com.mattermost.Mattermost"` with the iOS bundle ID of your custom mobile app, in two places.
   - Replace `"ApplePushCertPassword": ""` if your certificate has a password, in two places. Otherwise leave it blank.
   - For example: 
   
     ``` javascript
     {
      "ListenAddress":":8066",
      "ThrottlePerSec":300,
      "ThrottleMemoryStoreSize":50000,
      "ThrottleVaryByHeader":"X-Forwarded-For",
      "ApplePushSettings":[
        {
            "Type":"apple",
            "ApplePushUseDevelopment":false,
            "ApplePushCertPrivate":"./config/aps_production_priv.pem",
            "ApplePushCertPassword":"",
            "ApplePushTopic":"com.mattermost.Mattermost"
        },
        {
            "Type":"apple_rn",
            "ApplePushUseDevelopment":false,
            "ApplePushCertPrivate":"./config/aps_production_priv.pem",
            "ApplePushCertPassword":"",
            "ApplePushTopic":"com.mattermost.react.native"
        }
      ],
      "AndroidPushSettings":[
        {
            "Type":"android",
            "AndroidApiKey":"DKJDIiwjerljd290u34jFKDSF"
        },
        {
            "Type":"android_rn",
            "AndroidApiKey":"DKJDIiwjerljd290u34jFKDSF"
        }
      ]
    }
    ```

6. Setup Push Proxy to use the Upstart daemon which handles supervision
   of the Push Proxy process.

   -  `sudo touch /etc/init/mattermost-push-proxy.conf`
   -  `sudo vi /etc/init/mattermost-push-proxy.conf`
   -  Copy the following lines into `/etc/init/mattermost-push-proxy.conf`
     
     ```
     start on runlevel [2345]
     stop on runlevel [016]
     respawn
     chdir /home/ubuntu/mattermost-push-proxy
     setuid ubuntu
     console log
     exec bin/mattermost-push-proxy | logger
     ```
     
   - You can manage the process by typing:
     -  `sudo start mattermost-push-proxy`
   - You can also stop the process by running the command `sudo stop mattermost-push-proxy`, but we will skip this step for now

   
7. Test the Push Proxy Server

   - Verify the server is functioning normally and test the push notifications using curl: 
     - `curl http://127.0.0.1:8066/api/v1/send_push -X POST -H "Content-Type: application/json" -d '{ "message":"test", "badge": 1, "platform":"PLATFORM", "server_id":"MATTERMOST_DIAG_ID", "device_id":"DEVICE_ID"}'`
     - Replace MATTERMOST_DIAG_ID with the value found by running the SQL query:
       - `SELECT * FROM Systems WHERE Name = 'DiagnosticId';`
     - Replace DEVICE_ID with your device ID, which can be found using: 
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
     - Remove the "apple:", "apple_rn:", "android:" or "android_rn:" prefix from your device ID before replacing DEVICE_ID. Use that prefix as the PLATFORM (make sure to remove the ":")
   - You can also verify push notifications are working by opening your Mattermost site and mentioning a user who has push notifications enabled in Account Settings > Notifications > Mobile Push Notifications
   - To view the log file, use: 
     
     ```
     sudo tail -n 1000 /var/log/upstart/
     mattermost-push-proxy.log
     ```

### Troubleshooting

#### High Sierra Apple Developer Keys Steps - follow these instructions if you run into an error like below:
```
2018/04/13 12:39:24 CRIT Failed to load the apple pem cert err=failed to parse PKCS1 private key for type=apple_rn
panic: Failed to load the apple pem cert err=failed to parse PKCS1 private key for type=apple_rn
```

1. Follow the directions at [developer.apple.com](https://developer.apple.com/library/content/documentation/IDEs/Conceptual/AppDistributionGuide/DistributingEnterpriseProgramApps/DistributingEnterpriseProgramApps.html#//apple_ref/doc/uid/TP40012582-CH33-SW4) to generate an Apple Push Notification service SSL Certificate, this should give you an `aps_production.cer`
2. Convert the certificate format to .pem:
  - `openssl x509 -in aps.cer -inform DER -out aps_production.pem`
3. Double click `aps_production.cer` to install it into the keychain tool
4. Right click the private cert in keychain access and export to .p12
5. Extract the private key from the certificate into an intermediate state:
  - `openssl pkcs12 -in Certificates.p12 -out intermediate.pem -nodes -clcerts`
6. Generate an intermediate RSA private key
  - `openssl rsa -in intermediate.pem -out intermediate_rsa_priv.pem`
7. Remove the private key information from intermediate.pem
  - `sed -i '/^-----BEGIN PRIVATE KEY-----$/,$d' intermediate.pem`
8. Combine intermediate.pem and intermediate_rsa_priv.pem to create a valid bundle
  - `cat intermediate.pem intermediate_rsa_priv.pem >> aps_production_priv.pem && rm intermediate.pem intermediate_rsa_priv.pem`
6. Verifying the certificate works with apple:
  - `openssl s_client -connect gateway.push.apple.com:2195 -cert aps_production.pem -key aps_production_priv.pem`

### Reporting issues 

For issues with repro steps, please report to https://github.com/mattermost/mattermost-server/issues
