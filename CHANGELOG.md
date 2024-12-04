# Mattermost Push Proxy Changelog


## 5.9.0 Release
- Release Date: <to define>
- Compatible with all versions of Mattermost server and mobile

### Compatibility
- As of April 10, 2018, Google has deprecated the Google Cloud Messaging (GCM) service. The GCM server and client APIs are deprecated and will be removed as of April 11, 2019. You must [migrate GCM apps to Firebase Cloud Messaging (FCM)](https://developers.mattermost.com/contribute/mobile/push-notifications/migrate-gcm-fcm/), which inherits the reliable and scalable GCM infrastructure, plus many new features.

### Highlights
- Replaced old Logger system with [mlog.Logger](https://github.com/mattermost/mattermost/blob/master/server/public/shared/mlog/mlog.go).
- Deprecated type EnableConsoleLog, EnableFileLog, and LogFileLocation from the config  and replaced it with [LoggingCfgFile and LoggingCfgJSON](https://docs.mattermost.com/manage/logging.html).
- Switched from go 1.21 to go 1.22.
  

## 5.8.1 Release
- Release Date: March 28, 2019
- Compatible with all versions of Mattermost server and mobile

### Compatibility
 - As of April 10, 2018, Google has deprecated the Google Cloud Messaging (GCM) service. The GCM server and client APIs are deprecated and will be removed as of April 11, 2019. You must [migrate GCM apps to Firebase Cloud Messaging (FCM)](https://developers.mattermost.com/contribute/mobile/push-notifications/migrate-gcm-fcm/), which inherits the reliable and scalable GCM infrastructure, plus many new features.
 
### Bug Fixes
 - FCM messages are sent as high priority to wake Android devices in doze mode

________________

## 5.8 Release
- Release Date: March 20, 2019
- Compatible with all versions of Mattermost server and mobile

### Combatibility
 - As of April 10, 2018, Google has deprecated the Google Cloud Messaging (GCM) service. The GCM server and client APIs are deprecated and will be removed as of April 11, 2019. You must [migrate GCM apps to Firebase Cloud Messaging (FCM)](https://developers.mattermost.com/contribute/mobile/push-notifications/migrate-gcm-fcm/), which inherits the reliable and scalable GCM infrastructure, plus many new features.
 
### Highlights
 - Adds support for Firebase Cloud Messaging
