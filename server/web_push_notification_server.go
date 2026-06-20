// Copyright (c) 2026 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/hakobe/paranoidhttp"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const defaultWebPushTTLSeconds = 30

const defaultMaxErrorBodyBytes = 8 * 1024

type WebPushNotificationServer struct {
	WebPushSettings WebPushSettings
	metrics         *metrics
	logger          *mlog.Logger
	// paranoidClient blocks destinations on a private/internal IP range;
	// normalClient doesn't. SendNotification picks per-request, mirroring the
	// approach of the unified push proxy reference implementation.
	paranoidClient *http.Client
	normalClient   *http.Client
	ttlSeconds     int
}

// errDestinationBlocked marks a send rejected by paranoidClient's SSRF guard.
var errDestinationBlocked = errors.New("destination resolves to a forbidden IP range")

func denyRedirect(_ *http.Request, _ []*http.Request) error {
	return errors.New("redirects are not followed")
}

// wrapBlockedIPSentinel turns paranoidhttp's dial errors into
// errDestinationBlocked, so SendNotification can tell an SSRF block apart
// from an ordinary network failure.
func wrapBlockedIPSentinel(dial func(ctx context.Context, network, addr string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dial(ctx, network, addr)
		if err != nil && (strings.Contains(err.Error(), "bad ip is detected") || strings.Contains(err.Error(), "bad host is detected")) {
			return nil, fmt.Errorf("%w: %s", errDestinationBlocked, addr)
		}
		return conn, err
	}
}

// webPushBundle is never stored; the server keeps the durable copy. Endpoint
// and Auth together let someone spoof pushes, so keep them out of logs and
// errors. P256dh is public.
type webPushBundle struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

// buildWebPushPayload copies the FCM data map from
// AndroidNotificationServer.SendNotification field-for-field, so the client
// reuses its Android path.
func buildWebPushPayload(msg *model.PushNotification) map[string]string {
	pushType := msg.Type
	data := map[string]string{
		"ack_id":         msg.AckId,
		"type":           pushType,
		"sub_type":       string(msg.SubType),
		"version":        msg.Version,
		"channel_id":     msg.ChannelId,
		"is_crt_enabled": strconv.FormatBool(msg.IsCRTEnabled),
		"server_id":      msg.ServerId,
		"category":       msg.Category,
	}

	if msg.Badge != -1 {
		data["badge"] = strconv.Itoa(msg.Badge)
	}

	if msg.RootId != "" {
		data["root_id"] = msg.RootId
	}

	if msg.Signature == "" {
		data["signature"] = "NO_SIGNATURE"
	} else {
		data["signature"] = msg.Signature
	}

	if msg.IsIdLoaded {
		data["post_id"] = msg.PostId
		data["message"] = msg.Message
		data["id_loaded"] = "true"
		data["sender_id"] = msg.SenderId
		data["sender_name"] = "Someone"
		data["team_id"] = msg.TeamId
	} else if pushType == model.PushTypeMessage || pushType == model.PushTypeSession {
		data["team_id"] = msg.TeamId
		data["sender_id"] = msg.SenderId
		data["sender_name"] = msg.SenderName
		data["message"] = msg.Message
		data["channel_name"] = msg.ChannelName
		data["post_id"] = msg.PostId
		data["override_username"] = msg.OverrideUsername
		data["override_icon_url"] = msg.OverrideIconURL
		data["from_webhook"] = msg.FromWebhook
	}

	return data
}

func NewWebPushNotificationServer(settings WebPushSettings, logger *mlog.Logger, metrics *metrics, sendTimeoutSecs int) *WebPushNotificationServer {
	timeoutSecs := settings.RequestTimeout
	if timeoutSecs <= 0 {
		timeoutSecs = sendTimeoutSecs
	}
	timeout := time.Duration(timeoutSecs) * time.Second

	// Mirrors the unified push proxy reference implementation's use of
	// paranoidhttp. Known gap: its denylist misses CGNAT addresses — see
	// TestWebPushSendNotification_CGNATGapDocumented.
	paranoidClient, paranoidTransport, _ := paranoidhttp.NewClient()
	paranoidClient.Timeout = timeout
	paranoidClient.CheckRedirect = denyRedirect
	paranoidTransport.DialContext = wrapBlockedIPSentinel(paranoidTransport.DialContext)

	normalClient := &http.Client{Timeout: timeout, CheckRedirect: denyRedirect}

	ttlSeconds := settings.TTLSeconds
	if ttlSeconds == 0 {
		ttlSeconds = defaultWebPushTTLSeconds
	}

	return &WebPushNotificationServer{
		WebPushSettings: settings,
		metrics:         metrics,
		logger:          logger,
		paranoidClient:  paranoidClient,
		normalClient:    normalClient,
		ttlSeconds:      ttlSeconds,
	}
}

func isAllowedHost(host string, allowedHosts []string) bool {
	return slices.Contains(allowedHosts, host)
}

func (me *WebPushNotificationServer) Initialize() error {
	me.logger.Info("Initializing WebPush notification server", mlog.String("type", me.WebPushSettings.Type))

	if err := validateVAPIDKeys(me.WebPushSettings.VAPIDPublicKey, me.WebPushSettings.VAPIDPrivateKey); err != nil {
		return fmt.Errorf("failed to initialize WebPush notification service for type=%q: %w", me.WebPushSettings.Type, err)
	}

	if err := validateSubscriber(me.WebPushSettings.Subscriber); err != nil {
		return fmt.Errorf("failed to initialize WebPush notification service for type=%q: %w", me.WebPushSettings.Type, err)
	}

	if err := validateAllowedHosts(me.WebPushSettings.AllowedHosts); err != nil {
		return fmt.Errorf("failed to initialize WebPush notification service for type=%q: %w", me.WebPushSettings.Type, err)
	}

	if me.WebPushSettings.InsecureSkipDestinationIPCheck {
		me.logger.Warn("InsecureSkipDestinationIPCheck is enabled: the private-IP-range SSRF guard is disabled for this WebPush type", mlog.String("type", me.WebPushSettings.Type))
	}

	return nil
}

// validateAllowedHosts fails fast on entries that would silently never match
// at send time, rather than leaving the config value inert.
func validateAllowedHosts(hosts []string) error {
	for _, h := range hosts {
		if strings.Contains(h, "://") || strings.Contains(h, "/") || strings.Contains(h, "*") {
			return fmt.Errorf("AllowedHosts entry %q must be a bare host or host:port, not a URL or glob", h)
		}
	}
	return nil
}

// validateSubscriber checks at startup instead of per send: some relays
// accept a missing Subscriber and some reject it.
func validateSubscriber(subscriber string) error {
	if subscriber == "" {
		return fmt.Errorf("Subscriber is required and must be a mailto: or https: contact URI in WebPushSettings")
	}
	if !strings.HasPrefix(subscriber, "mailto:") && !strings.HasPrefix(subscriber, "https://") {
		return fmt.Errorf("Subscriber must be a mailto: or https: URI, got %q", subscriber)
	}
	return nil
}

// validateVAPIDKeys rejects a missing, malformed, or mismatched keypair at
// startup. There's no fallback key, so a bad one would otherwise show up as
// per-send failures, or as spoofable pushes on relays that skip VAPID checks.
// Failing at startup is better.
func validateVAPIDKeys(publicKey, privateKey string) error {
	if publicKey == "" || privateKey == "" {
		return fmt.Errorf("VAPIDPublicKey and VAPIDPrivateKey are both required and must be set in WebPushSettings")
	}

	privBytes, err := decodeVAPIDKey(privateKey)
	if err != nil {
		return fmt.Errorf("VAPIDPrivateKey is not valid base64: %w", err)
	}
	priv, err := ecdh.P256().NewPrivateKey(privBytes)
	if err != nil {
		return fmt.Errorf("VAPIDPrivateKey is not a valid P-256 private key: %w", err)
	}

	pubBytes, err := decodeVAPIDKey(publicKey)
	if err != nil {
		return fmt.Errorf("VAPIDPublicKey is not valid base64: %w", err)
	}
	pub, err := ecdh.P256().NewPublicKey(pubBytes)
	if err != nil {
		return fmt.Errorf("VAPIDPublicKey is not a valid P-256 public key: %w", err)
	}

	// Both decode but don't pair, so every signed request would fail VAPID on
	// enforcing relays.
	if !priv.PublicKey().Equal(pub) {
		return fmt.Errorf("VAPIDPublicKey does not correspond to VAPIDPrivateKey")
	}

	return nil
}

// decodeVAPIDKey accepts a key in either encoding: base64url first (what
// GenerateVAPIDKeys emits), then standard base64.
func decodeVAPIDKey(key string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(key); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(key)
}

// redactWebPushError drops the endpoint URL from transport errors. Go wraps
// them as *url.Error, whose message includes the request URL, and that endpoint
// is a bearer credential.
func redactWebPushError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
	}
	return err
}

// redactHostOnly returns just the host of a WebPush endpoint, safe to log
// since the path carries a bearer credential and the host doesn't.
func redactHostOnly(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "unknown"
	}
	return u.Host
}

func (me *WebPushNotificationServer) SendNotification(_ int, msg *model.PushNotification) PushResponse {
	pushType := msg.Type
	if me.metrics != nil {
		me.metrics.incrementNotificationTotal(PushNotifyWebPush, pushType, model.PushTransportStandard)
	}

	sub, err := decodeWebPushDeviceID(msg.DeviceId)
	if err != nil {
		me.logger.Error(
			"Failed to decode WebPush device id",
			mlog.String("sid", msg.ServerId),
			mlog.Err(err),
		)
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, "INVALID_DEVICE_ID")
		}
		return NewErrorPushResponse(fmt.Sprintf("invalid WebPush device id: %v", err))
	}

	endpointURL, err := url.Parse(sub.Endpoint)
	if err != nil {
		me.logger.Error(
			"WebPush device id bundle has an unparseable endpoint",
			mlog.String("sid", msg.ServerId),
			mlog.Err(err),
		)
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, "INVALID_DEVICE_ID")
		}
		return NewErrorPushResponse(fmt.Sprintf("invalid WebPush device id: %v", err))
	}

	skipGuard := me.WebPushSettings.InsecureSkipDestinationIPCheck || isAllowedHost(endpointURL.Host, me.WebPushSettings.AllowedHosts)

	if endpointURL.Scheme != "https" && !skipGuard {
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, "SCHEME_NOT_ALLOWED")
		}
		return NewErrorPushResponse("endpoint must use https, or be in AllowedHosts to use http")
	}

	thisClient := me.paranoidClient
	if skipGuard {
		thisClient = me.normalClient
	}

	payload, err := json.Marshal(buildWebPushPayload(msg))
	if err != nil {
		me.logger.Error("Failed to marshal WebPush payload", mlog.String("sid", msg.ServerId), mlog.Err(err))
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, "MARSHAL_ERROR")
		}
		return NewErrorPushResponse(fmt.Sprintf("failed to marshal payload: %v", err))
	}

	me.logger.Info(
		"Sending web push notification",
		mlog.String("device", me.WebPushSettings.Type),
		mlog.String("type", msg.Type),
		mlog.String("ack_id", msg.AckId),
	)

	start := time.Now()
	resp, err := webpush.SendNotification(payload, sub, &webpush.Options{
		HTTPClient:      thisClient,
		Subscriber:      me.WebPushSettings.Subscriber,
		TTL:             me.ttlSeconds,
		VAPIDPublicKey:  me.WebPushSettings.VAPIDPublicKey,
		VAPIDPrivateKey: me.WebPushSettings.VAPIDPrivateKey,
	})
	if me.metrics != nil {
		me.metrics.observerNotificationResponse(PushNotifyWebPush, time.Since(start).Seconds())
	}
	if err != nil {
		err = redactWebPushError(err)
		if errors.Is(err, errDestinationBlocked) {
			if me.metrics != nil {
				me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, "DESTINATION_BLOCKED")
			}
			me.logger.Warn(
				"blocked WebPush send: destination resolves to a private address",
				mlog.String("sid", msg.ServerId),
				mlog.String("host", redactHostOnly(sub.Endpoint)),
				mlog.String("type", me.WebPushSettings.Type),
			)
			return NewErrorPushResponse("destination not permitted")
		}
		me.logger.Error(
			"Failed to send web push",
			mlog.String("sid", msg.ServerId),
			mlog.String("did", redactToken(msg.DeviceId)),
			mlog.Err(err),
			mlog.String("type", me.WebPushSettings.Type),
		)
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, "RequestError")
		}
		return NewErrorPushResponse(err.Error())
	}
	defer resp.Body.Close()

	return me.handleResponse(resp, msg, pushType)
}

func (me *WebPushNotificationServer) handleResponse(resp *http.Response, msg *model.PushNotification, pushType string) PushResponse {
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		if me.metrics != nil {
			if msg.AckId != "" {
				me.metrics.incrementSuccessWithAck(PushNotifyWebPush, pushType, model.PushTransportStandard)
			} else {
				me.metrics.incrementSuccess(PushNotifyWebPush, pushType, model.PushTransportStandard)
			}
		}
		return NewOkPushResponse()

	case http.StatusGone, http.StatusNotFound:
		me.logger.Info(
			"WebPush endpoint gone, sending remove code",
			mlog.Int("code", resp.StatusCode),
			mlog.String("type", me.WebPushSettings.Type),
		)
		if me.metrics != nil {
			me.metrics.incrementRemoval(PushNotifyWebPush, pushType, model.PushTransportStandard, strconv.Itoa(resp.StatusCode))
		}
		return NewRemovePushResponse()

	case http.StatusTooManyRequests:
		me.logger.Info("WebPush endpoint rate limited", mlog.String("type", me.WebPushSettings.Type))
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, "RATE_LIMITED")
		}
		return NewErrorPushResponse("webpush relay rate limited the request")

	default:
		maxBody := me.WebPushSettings.MaxErrorBodyBytes
		if maxBody == 0 {
			maxBody = defaultMaxErrorBodyBytes
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(maxBody)))
		logFields := []mlog.Field{
			mlog.Int("code", resp.StatusCode),
			mlog.String("type", me.WebPushSettings.Type),
		}
		me.logger.Error("WebPush request failed", logFields...)
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyWebPush, pushType, model.PushTransportStandard, strconv.Itoa(resp.StatusCode))
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return NewErrorPushResponse(string(body))
		}
		return NewErrorPushResponse(fmt.Sprintf("webpush relay returned status %d", resp.StatusCode))
	}
}

// decodeWebPushDeviceID base64url+JSON-decodes deviceID into a subscription.
// The server already stripped the "unified_push-v2:" prefix, so what arrives
// here is just the raw encoded bundle.
func decodeWebPushDeviceID(deviceID string) (*webpush.Subscription, error) {
	raw, err := base64.RawURLEncoding.DecodeString(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to base64url-decode device id: %w", err)
	}

	var bundle webPushBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, fmt.Errorf("failed to JSON-decode device id bundle: %w", err)
	}

	if bundle.Endpoint == "" || bundle.P256dh == "" || bundle.Auth == "" {
		return nil, fmt.Errorf("device id bundle missing required fields")
	}

	return &webpush.Subscription{
		Endpoint: bundle.Endpoint,
		Keys: webpush.Keys{
			P256dh: bundle.P256dh,
			Auth:   bundle.Auth,
		},
	}, nil
}
