// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/kyokomi/emoji"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"golang.org/x/net/http2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

type AppleNotificationServer struct {
	AppleClient       *apns.Client
	metrics           *metrics
	logger            *mlog.Logger
	ApplePushSettings ApplePushSettings
	sendTimeout       time.Duration
	retryTimeout      time.Duration
}

func NewAppleNotificationServer(settings ApplePushSettings, logger *mlog.Logger, metrics *metrics, sendTimeoutSecs int, retryTimeoutSecs int) *AppleNotificationServer {
	return &AppleNotificationServer{
		ApplePushSettings: settings,
		metrics:           metrics,
		logger:            logger,
		sendTimeout:       time.Duration(sendTimeoutSecs) * time.Second,
		retryTimeout:      time.Duration(retryTimeoutSecs) * time.Second,
	}
}

func (me *AppleNotificationServer) setupProxySettings(appleCert *tls.Certificate) error {
	// Override the native transport.
	proxyServer := getProxyServer()
	if proxyServer != "" {
		transport := &http.Transport{
			Proxy: func(request *http.Request) (*url.URL, error) {
				return url.Parse(proxyServer)
			},
			IdleConnTimeout: apns.HTTPClientTimeout,
		}

		if appleCert != nil {
			transport.TLSClientConfig = &tls.Config{
				Certificates: []tls.Certificate{*appleCert},
			}
		}

		err := http2.ConfigureTransport(transport)
		if err != nil {
			return fmt.Errorf("transport error: %v", err)
		}

		me.AppleClient.HTTPClient.Transport = transport
	}

	if appleCert != nil {
		me.logger.Info("Initializing apple notification server with PEM certificate", mlog.String("type", me.ApplePushSettings.Type))
	} else {
		me.logger.Info("Initializing apple notification server with AuthKey", mlog.String("type", me.ApplePushSettings.Type))
	}

	return nil
}

func (me *AppleNotificationServer) Initialize() error {
	if me.ApplePushSettings.AppleAuthKeyFile != "" && me.ApplePushSettings.AppleAuthKeyID != "" && me.ApplePushSettings.AppleTeamID != "" {
		authKey, err := token.AuthKeyFromFile(me.ApplePushSettings.AppleAuthKeyFile)
		if err != nil {
			return fmt.Errorf("failed to initialize apple notification service with AuthKey file err=%v ", err)
		}

		appleToken := &token.Token{
			AuthKey: authKey,
			KeyID:   me.ApplePushSettings.AppleAuthKeyID,
			TeamID:  me.ApplePushSettings.AppleTeamID,
		}

		if me.ApplePushSettings.ApplePushUseDevelopment {
			me.AppleClient = apns.NewTokenClient(appleToken).Development()
		} else {
			me.AppleClient = apns.NewTokenClient(appleToken).Production()
		}

		// Override the native transport.
		return me.setupProxySettings(nil)
	}

	if me.ApplePushSettings.ApplePushCertPrivate != "" {
		appleCert, appleCertErr := certificate.FromPemFile(me.ApplePushSettings.ApplePushCertPrivate, me.ApplePushSettings.ApplePushCertPassword)
		if appleCertErr != nil {
			return fmt.Errorf("failed to initialize apple notification service with pem cert err=%v for type=%v", appleCertErr, me.ApplePushSettings.Type)
		}

		if me.ApplePushSettings.ApplePushUseDevelopment {
			me.AppleClient = apns.NewClient(appleCert).Development()
		} else {
			me.AppleClient = apns.NewClient(appleCert).Production()
		}

		// Override the native transport.
		return me.setupProxySettings(&appleCert)
	}

	return fmt.Errorf("apple push notifications not configured: missing ApplePushCertPrivate for type=%v", me.ApplePushSettings.Type)
}

func (me *AppleNotificationServer) SendNotification(appVersion int, msg *model.PushNotification) PushResponse {
	if msg.Transport == model.PushTransportVoIP {
		return me.sendVoIPNotification(msg)
	}

	data := payload.NewPayload()
	if msg.Badge == 0 && msg.Type == model.PushTypeClear && appVersion > 1 {
		data.Badge(1)
	} else if msg.Badge != -1 {
		data.Badge(msg.Badge)
	}

	notification := &apns.Notification{}
	notification.DeviceToken = msg.DeviceId
	notification.Payload = data
	notification.Topic = me.ApplePushSettings.ApplePushTopic
	notification.Priority = apns.PriorityHigh

	pushType := msg.Type
	if msg.IsIdLoaded {
		data.Category(msg.Category)
		data.Sound("default")
		data.Custom("version", msg.Version)
		data.Custom("id_loaded", true)
		data.MutableContent()
		data.AlertBody(msg.Message)
		data.ContentAvailable()
	} else {
		switch msg.Type {
		case model.PushTypeMessage, model.PushTypeSession:
			data.Category(msg.Category)
			data.Sound("default")
			data.Custom("version", msg.Version)
			data.MutableContent()
			if msg.Type == model.PushTypeMessage {
				data.ContentAvailable()
			}

			if msg.ChannelName != "" && msg.Version == "v2" {
				data.AlertTitle(msg.ChannelName)
				data.AlertBody(emoji.Sprint(msg.Message))
				data.Custom("channel_name", msg.ChannelName)
			} else {
				data.Alert(emoji.Sprint(msg.Message))

				if msg.ChannelName != "" {
					data.Custom("channel_name", msg.ChannelName)
				}
			}
		case model.PushTypeClear, model.PushTypeTest:
			data.ContentAvailable()
		case model.PushTypeUpdateBadge:
			// Handled by the apps, nothing else to do here
		}
	}
	if me.metrics != nil {
		me.metrics.incrementNotificationTotal(model.PushNotifyApple, pushType, model.PushTransportStandard)
	}
	data.Custom("type", pushType)
	data.Custom("sub_type", msg.SubType)
	data.Custom("server_id", msg.ServerId)

	if msg.AckId != "" {
		data.Custom("ack_id", msg.AckId)
	}

	data.Custom("is_crt_enabled", msg.IsCRTEnabled)

	if msg.ChannelId != "" {
		data.Custom("channel_id", msg.ChannelId)

		if msg.IsCRTEnabled && msg.RootId != "" {
			data.ThreadID(msg.RootId)
		} else {
			data.ThreadID(msg.ChannelId)
		}
	}

	if msg.Signature == "" {
		data.Custom("signature", "NO_SIGNATURE")
	} else {
		data.Custom("signature", msg.Signature)
	}

	if msg.TeamId != "" {
		data.Custom("team_id", msg.TeamId)
	}

	if msg.SenderId != "" {
		data.Custom("sender_id", msg.SenderId)
	}

	if msg.SenderName != "" {
		data.Custom("sender_name", msg.SenderName)
	}

	if msg.PostId != "" {
		data.Custom("post_id", msg.PostId)
	}

	if msg.RootId != "" {
		data.Custom("root_id", msg.RootId)
	}

	if msg.OverrideUsername != "" {
		data.Custom("override_username", msg.OverrideUsername)
	}

	if msg.OverrideIconURL != "" {
		data.Custom("override_icon_url", msg.OverrideIconURL)
	}

	if msg.FromWebhook != "" {
		data.Custom("from_webhook", msg.FromWebhook)
	}

	return me.dispatchAndHandleResponse(notification, msg, pushType, model.PushTransportStandard)
}

func (me *AppleNotificationServer) dispatchAndHandleResponse(notification *apns.Notification, msg *model.PushNotification, pushType string, transport model.PushTransport) PushResponse {
	if me.AppleClient == nil {
		return NewOkPushResponse()
	}

	logFields := []mlog.Field{
		mlog.String("device", me.ApplePushSettings.Type),
		mlog.String("type", msg.Type),
		mlog.String("ack_id", msg.AckId),
	}
	if transport != model.PushTransportStandard {
		logFields = append(logFields, mlog.String("transport", string(transport)))
	}
	me.logger.Info("Sending apple push notification", logFields...)

	res, err := me.SendNotificationWithRetry(notification)
	if err != nil {
		errFields := []mlog.Field{
			mlog.String("sid", msg.ServerId),
			mlog.String("did", redactToken(msg.DeviceId)),
			mlog.Err(err),
			mlog.String("type", me.ApplePushSettings.Type),
		}
		if transport != model.PushTransportStandard {
			errFields = append(errFields, mlog.String("transport", string(transport)))
		}
		me.logger.Error("Failed to send apple push", errFields...)
		if me.metrics != nil {
			me.metrics.incrementFailure(model.PushNotifyApple, pushType, transport, "RequestError")
		}
		return NewErrorPushResponse("unknown transport error")
	}

	if !res.Sent() {
		if res.Reason == apns.ReasonBadDeviceToken || res.Reason == apns.ReasonUnregistered || res.Reason == apns.ReasonMissingDeviceToken || res.Reason == apns.ReasonDeviceTokenNotForTopic {
			me.logger.Info(
				"Failed to send apple push sending remove code res",
				mlog.String("ApnsID", res.ApnsID),
				mlog.String("reason", res.Reason),
				mlog.Int("code", res.StatusCode),
				mlog.String("type", me.ApplePushSettings.Type),
			)
			if me.metrics != nil {
				me.metrics.incrementRemoval(model.PushNotifyApple, pushType, transport, res.Reason)
			}
			return NewRemovePushResponse()
		}

		me.logger.Error(
			"Failed to send apple push with res",
			mlog.String("ApnsID", res.ApnsID),
			mlog.String("reason", res.Reason),
			mlog.Int("code", res.StatusCode),
			mlog.String("type", me.ApplePushSettings.Type),
		)
		if me.metrics != nil {
			me.metrics.incrementFailure(model.PushNotifyApple, pushType, transport, res.Reason)
		}
		return NewErrorPushResponse("unknown send response error")
	}

	if me.metrics != nil {
		if msg.AckId != "" {
			me.metrics.incrementSuccessWithAck(model.PushNotifyApple, pushType, transport)
		} else {
			me.metrics.incrementSuccess(model.PushNotifyApple, pushType, transport)
		}
	}
	return NewOkPushResponse()
}

// sendVoIPNotification dispatches a PushKit VoIP push using the same APNs key
// configured for the standard target. The payload carries the routing fields
// the mobile client needs to wake the call UI; the canonical Call state
// (callID, hostID, participants, etc.) is fetched via the existing
// GET /calls REST roundtrip once the app foregrounds and reconnects its
// WebSocket.
func (me *AppleNotificationServer) sendVoIPNotification(msg *model.PushNotification) PushResponse {
	notification := me.buildVoIPNotification(msg)

	if me.metrics != nil {
		me.metrics.incrementNotificationTotal(model.PushNotifyApple, msg.Type, model.PushTransportVoIP)
	}

	return me.dispatchAndHandleResponse(notification, msg, msg.Type, model.PushTransportVoIP)
}

func (me *AppleNotificationServer) buildVoIPNotification(msg *model.PushNotification) *apns.Notification {
	data := payload.NewPayload().
		ContentAvailable().
		Custom("type", msg.Type).
		Custom("sub_type", msg.SubType).
		Custom("channel_id", msg.ChannelId).
		Custom("server_id", msg.ServerId).
		Custom("post_id", msg.PostId).
		Custom("thread_id", msg.RootId).
		Custom("sender_id", msg.SenderId).
		Custom("id_loaded", msg.IsIdLoaded)

	// sender_name and channel_name are only populated by the server when
	// PushNotificationContents is FullNotification or GenericNotification —
	// for IdLoadedNotification they're omitted on purpose and the device
	// fetches them via the ack-receipt round-trip.
	if msg.SenderName != "" {
		data.Custom("sender_name", msg.SenderName)
	}
	if msg.ChannelName != "" {
		data.Custom("channel_name", msg.ChannelName)
	}

	if msg.AckId != "" {
		data.Custom("ack_id", msg.AckId)
	}

	if msg.Signature == "" {
		data.Custom("signature", "NO_SIGNATURE")
	} else {
		data.Custom("signature", msg.Signature)
	}

	return &apns.Notification{
		DeviceToken: msg.DeviceId,
		Payload:     data,
		Topic:       me.ApplePushSettings.ApplePushTopic + ".voip",
		Priority:    apns.PriorityHigh,
		PushType:    apns.PushTypeVOIP,
	}
}

func (me *AppleNotificationServer) SendNotificationWithRetry(notification *apns.Notification) (*apns.Response, error) {
	var res *apns.Response
	var err error
	waitTime := time.Second

	// Keep a general context to make sure the whole retry
	// doesn't take longer than the timeout.
	generalContext, cancelGeneralContext := context.WithTimeout(context.Background(), me.sendTimeout)
	defer cancelGeneralContext()

	for retries := range MAX_RETRIES {
		start := time.Now()

		retryContext, cancelRetryContext := context.WithTimeout(generalContext, me.retryTimeout)
		defer cancelRetryContext()
		res, err = me.AppleClient.PushWithContext(retryContext, notification)
		if me.metrics != nil {
			me.metrics.observerNotificationResponse(model.PushNotifyApple, time.Since(start).Seconds())
		}

		if err == nil {
			break
		}

		me.logger.Error(
			"Failed to send apple push",
			mlog.String("did", redactToken(notification.DeviceToken)),
			mlog.Int("retry", retries),
			mlog.Err(err),
		)

		if retries == MAX_RETRIES-1 {
			me.logger.Error("Max retries reached", mlog.String("did", redactToken(notification.DeviceToken)))
			break
		}

		select {
		case <-generalContext.Done():
		case <-time.After(waitTime):
		}

		if generalContext.Err() != nil {
			me.logger.Info(
				"Not retrying because context error",
				mlog.String("did", redactToken(notification.DeviceToken)),
				mlog.Int("retry", retries),
				mlog.Err(generalContext.Err()),
			)
			err = generalContext.Err()
			break
		}

		waitTime *= 2
	}

	return res, err
}
