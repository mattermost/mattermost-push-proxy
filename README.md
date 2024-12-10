# Mattermost Push Proxy ![CircleCI branch](https://img.shields.io/circleci/project/github/mattermost/mattermost-push-proxy/master.svg)

See https://developers.mattermost.com/contribute/mobile/push-notifications/service/


# How to Release

To trigger a release of Mattermost Push-Proxy, follow these steps:

1. **For Patch Release:** Run the following command:
    ```
    make patch
    ```
   This will release a patch change.

2. **For Minor Release:** Run the following command:
    ```
    make minor
    ```
   This will release a minor change.

3. **For Major Release:** Run the following command:
    ```
    make major
    ```
   This will release a major change.
