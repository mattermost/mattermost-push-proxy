# Mattermost Push Proxy ![CircleCI branch](https://img.shields.io/circleci/project/github/mattermost/mattermost-push-proxy/master.svg)

See https://developers.mattermost.com/contribute/mobile/push-notifications/service/

## VoIP push notifications (iOS Calls)

The proxy delivers PushKit / VoIP pushes for iOS calls. Dispatch is driven by the `transport` field on the incoming notification: when `transport=voip`, the proxy emits a VoIP-shaped APNs request (`apns-push-type: voip`, topic `<ApplePushTopic>.voip`, minimal payload) using the existing `apple_rn` / `apple_rnbeta` `ApplePushSettings` entry indicated by the message's `platform`. No extra configuration block is required; the same APNs key is reused for both standard and VoIP pushes.

Operator prerequisites:

- The iOS app bundle must declare the `voip` background mode and ship with an entitlement granting the `<bundle>.voip` APNs topic.

No changes to the standard `apple_rn` / `apple_rnbeta` entries are required.

## WebPush (UnifiedPush) notifications

The proxy can deliver Web Push notifications (RFC8291) to a UnifiedPush v2 relay
(e.g. `ntfy.sh`) for clients that register a `unified_push` device. This is
configured with a `WebPushSettings` block:

```json
"WebPushSettings": [
    {
        "Type": "unified_push",
        "RequestTimeout": 30,
        "VAPIDPublicKey": "<base64url public key>",
        "VAPIDPrivateKey": "<base64url private key>",
        "Subscriber": "mailto:push-admin@example.com",
        "AllowedHosts": [],
        "MaxErrorBodyBytes": 8192,
        "TTLSeconds": 30,
        "AdditionalBlockedCIDRs": []
    }
]
```

| Field | Required | Description |
| --- | --- | --- |
| `Type` | yes | Routing key the message's `platform` matches; must be `unified_push`. |
| `RequestTimeout` | no | Per-send HTTP timeout in seconds. Defaults to `SendTimeoutSec`. |
| `VAPIDPublicKey` | yes | P-256 VAPID public key, base64url. Must be the same key the client uses as its `applicationServerKey` at subscribe time. |
| `VAPIDPrivateKey` | yes | P-256 VAPID private key, base64url. **Secret** — provision per deployment, never commit. |
| `Subscriber` | yes | VAPID `sub` contact (RFC8292): a `mailto:` or `https:` URI a relay operator can reach. |
| `AllowedHosts` | no | Hosts (`hostname` or `hostname:port`, exact match, no scheme/path/wildcard) exempt from the private-IP check below and allowed to use plain `http://`. Empty by default. |
| `MaxErrorBodyBytes` | no | Cap, in bytes, on the relay's response body before it's echoed back in the push error. Applies to status codes other than 200/201/202 (success), 404/410 (gone), and 429 (rate limited), which don't read the body. The relay is client-chosen and could return an arbitrarily large body otherwise. Defaults to `8192`. |
| `TTLSeconds` | no | Relay hold time for an undelivered push (RFC8030 `TTL` header). Defaults to `30`. |
| `AdditionalBlockedCIDRs` | no | Extra CIDRs (e.g. `"203.0.113.0/24"`) to block on top of the built-in denylist below — for a range found to be abused after launch, without waiting on a new release. A malformed entry fails startup for that `Type`, same as the other fields here. Empty by default. |

A bad value in any of these fields fails startup for that `Type` only —
logged with the precise reason — rather than failing silently per
notification later.

### SSRF protection

The WebPush `endpoint` comes from the client, so the proxy rejects any
destination that resolves to a private or internal IP (loopback, RFC1918,
link-local, cloud metadata, etc.) and requires `https://`, both by default.
Redirects are never followed, even to an allowlisted host.

To use a relay that's only reachable on a private network — e.g. NAT
hairpin or split-horizon DNS pointing `ntfy.mydomain.com` at
`192.168.1.11` — add its `host` or `host:port` to `AllowedHosts` for that
`Type`; that's the supported way to do it. There's also an
`InsecureSkipDestinationIPCheck` field that disables the check entirely
for a `Type`, but it exists for tests against a local server, not
production — leave it unset.

Going the other direction, `AdditionalBlockedCIDRs` extends the denylist
for a `Type` — e.g. to block a range discovered to be abused for SSRF
after launch, without waiting on a new release.

### Generating a VAPID keypair

The keys must be a matched P-256 pair, base64url-encoded (the format
`webpush-go`'s `GenerateVAPIDKeys` emits). Generate one with either:

**Go** (uses the dependency already vendored here):

```go
package main

import (
	"fmt"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		panic(err)
	}
	fmt.Println("VAPIDPrivateKey:", priv)
	fmt.Println("VAPIDPublicKey: ", pub)
}
```

**Node** (the `web-push` CLI emits the same base64url format):

```sh
npx web-push generate-vapid-keys
```

Put the public key in `VAPIDPublicKey`, the private key in `VAPIDPrivateKey`, and
keep the private key secret. The same public key must be distributed to clients
as their `applicationServerKey`.


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
