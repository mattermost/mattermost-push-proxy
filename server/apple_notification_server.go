// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"net/http"
	"net/url"
	"time"

	"github.com/kyokomi/emoji"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"golang.org/x/net/http2"
)

type AppleNotificationServer struct {
	AppleClient       *apns.Client
	metrics           *metrics
	logger            *Logger
	ApplePushSettings ApplePushSettings
	sendTimeout       time.Duration
	retryTimeout      time.Duration
}

func NewAppleNotificationServer(settings ApplePushSettings, logger *Logger, metrics *metrics, sendTimeoutSecs int, retryTimeoutSecs int) *AppleNotificationServer {
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
			return fmt.Errorf("Transport Error: %v", err)
		}

		me.AppleClient.HTTPClient.Transport = transport
	}

	if appleCert != nil {
		me.logger.Info("Initializing apple notification server with PEM certificate", mlog.String("type", me.ApplePushSettings.Type))
	} else {
		me.logger.Info("Initializing apple notification server for type=%v with AuthKey", mlog.String("type", me.ApplePushSettings.Type))
	}

	return nil
}

func (me *AppleNotificationServer) Initialize() error {
	if me.ApplePushSettings.AppleAuthKeyFile != "" && me.ApplePushSettings.AppleAuthKeyID != "" && me.ApplePushSettings.AppleTeamID != "" {
		authKey, err := token.AuthKeyFromFile(me.ApplePushSettings.AppleAuthKeyFile)
		if err != nil {
			return fmt.Errorf("Failed to initialize apple notification service with AuthKey file err=%v ", err)
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
			return fmt.Errorf("Failed to initialize apple notification service with pem cert err=%v for type=%v", appleCertErr, me.ApplePushSettings.Type)
		}

		if me.ApplePushSettings.ApplePushUseDevelopment {
			me.AppleClient = apns.NewClient(appleCert).Development()
		} else {
			me.AppleClient = apns.NewClient(appleCert).Production()
		}

		// Override the native transport.
		return me.setupProxySettings(&appleCert)
	}

	return fmt.Errorf("Apple push notifications not configured.  Missing ApplePushCertPrivate. for type=%v", me.ApplePushSettings.Type)
}

func (me *AppleNotificationServer) SendNotification(msg *PushNotification) PushResponse {

	data := payload.NewPayload()
	if msg.Badge == 0 && msg.Type == PushTypeClear && msg.AppVersion > 1 {
		data.Badge(1)
	} else if msg.Badge != -1 {
		data.Badge(msg.Badge)
	}

	notification := &apns.Notification{}
	notification.DeviceToken = msg.DeviceID
	notification.Payload = data
	notification.Topic = me.ApplePushSettings.ApplePushTopic
	notification.Priority = apns.PriorityHigh

	var pushType = msg.Type
	if msg.IsIDLoaded {
		data.Category(msg.Category)
		data.Sound("default")
		data.Custom("version", msg.Version)
		data.Custom("id_loaded", true)
		data.MutableContent()
		data.AlertBody(msg.Message)
		data.ContentAvailable()
	} else {
		switch msg.Type {
		case PushTypeMessage, PushTypeSession:
			data.Category(msg.Category)
			data.Sound("default")
			data.Custom("version", msg.Version)
			data.MutableContent()
			if msg.Type == PushTypeMessage {
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
		case PushTypeClear, PushTypeTest:
			data.ContentAvailable()
		case PushTypeUpdateBadge:
			// Handled by the apps, nothing else to do here
		}
	}
	if me.metrics != nil {
		me.metrics.incrementNotificationTotal(PushNotifyApple, pushType)
	}
	data.Custom("type", pushType)
	data.Custom("sub_type", msg.SubType)
	data.Custom("server_id", msg.ServerID)

	if msg.AckID != "" {
		data.Custom("ack_id", msg.AckID)
	}

	data.Custom("is_crt_enabled", msg.IsCRTEnabled)

	if msg.ChannelID != "" {
		data.Custom("channel_id", msg.ChannelID)

		if msg.IsCRTEnabled && msg.RootID != "" {
			data.ThreadID(msg.RootID)
		} else {
			data.ThreadID(msg.ChannelID)
		}
	}

	if msg.Signature == "" {
		data.Custom("signature", "NO_SIGNATURE")
	} else {
		data.Custom("signature", msg.Signature)
	}

	if msg.TeamID != "" {
		data.Custom("team_id", msg.TeamID)
	}

	if msg.SenderID != "" {
		data.Custom("sender_id", msg.SenderID)
	}

	if msg.SenderName != "" {
		data.Custom("sender_name", msg.SenderName)
	}

	if msg.PostID != "" {
		data.Custom("post_id", msg.PostID)
	}

	if msg.RootID != "" {
		data.Custom("root_id", msg.RootID)
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

	if me.AppleClient != nil {
		me.logger.Info(
			"Sending apple push notification for ackId=%v",
			mlog.String("device", me.ApplePushSettings.Type),
			mlog.String("type", msg.Type),
			mlog.String("AckId", msg.AckID),
		)

		res, err := me.SendNotificationWithRetry(notification)
		if err != nil {
			me.logger.Error(
				"Failed to send apple push",
				mlog.String("sid", msg.ServerID),
				mlog.String("did", msg.DeviceID),
				mlog.Err(err),
				mlog.String("type", me.ApplePushSettings.Type),
			)
			if me.metrics != nil {
				me.metrics.incrementFailure(PushNotifyApple, pushType, "RequestError")
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
					me.metrics.incrementRemoval(PushNotifyApple, pushType, res.Reason)
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
				me.metrics.incrementFailure(PushNotifyApple, pushType, res.Reason)
			}
			return NewErrorPushResponse("unknown send response error")
		}
	}
	if me.metrics != nil {
		if msg.AckID != "" {
			me.metrics.incrementSuccessWithAck(PushNotifyApple, pushType)
		} else {
			me.metrics.incrementSuccess(PushNotifyApple, pushType)
		}
	}
	return NewOkPushResponse()
}

func (me *AppleNotificationServer) SendNotificationWithRetry(notification *apns.Notification) (*apns.Response, error) {
	var res *apns.Response
	var err error
	waitTime := time.Second

	// Keep a general context to make sure the whole retry
	// doesn't take longer than the timeout.
	generalContext, cancelGeneralContext := context.WithTimeout(context.Background(), me.sendTimeout)
	defer cancelGeneralContext()

	for retries := 0; retries < MAX_RETRIES; retries++ {
		start := time.Now()

		retryContext, cancelRetryContext := context.WithTimeout(generalContext, me.retryTimeout)
		defer cancelRetryContext()
		res, err = me.AppleClient.PushWithContext(retryContext, notification)
		if me.metrics != nil {
			me.metrics.observerNotificationResponse(PushNotifyApple, time.Since(start).Seconds())
		}

		if err == nil {
			break
		}

		me.logger.Error(
			"Failed to send apple push",
			mlog.String("did", notification.DeviceToken),
			mlog.Int("retry", retries),
			mlog.Err(err),
		)

		if retries == MAX_RETRIES-1 {
			me.logger.Error("Max retries reached", mlog.String("did", notification.DeviceToken))
			break
		}

		select {
		case <-generalContext.Done():
		case <-time.After(waitTime):
		}

		if generalContext.Err() != nil {
			me.logger.Info(
				"Not retrying because context error",
				mlog.String("did", notification.DeviceToken),
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
