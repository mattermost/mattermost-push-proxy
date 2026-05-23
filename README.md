# Mattermost Push Proxy ![CircleCI branch](https://img.shields.io/circleci/project/github/mattermost/mattermost-push-proxy/master.svg)

See https://developers.mattermost.com/contribute/mobile/push-notifications/service/

## VoIP push notifications (iOS Calls)

The proxy delivers PushKit / VoIP pushes for iOS calls under the platform identifiers `apple_voip_rn` and `apple_voip_rnbeta`. These are aliased internally to the existing `apple_rn` / `apple_rnbeta` `ApplePushSettings` entries — no extra configuration block is required; the same APNs key is reused. The proxy emits a VoIP-shaped APNs request (`apns-push-type: voip`, topic `<ApplePushTopic>.voip`, minimal payload) when an incoming notification's platform carries the `apple_voip_` prefix.

Operator prerequisites:

- The iOS app bundle must declare the `voip` background mode and ship with an entitlement granting the `<bundle>.voip` APNs topic.

No changes to the standard `apple_rn` / `apple_rnbeta` entries are required.


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
