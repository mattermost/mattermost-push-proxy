// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/kyokomi/emoji"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"golang.org/x/net/http2"
)

type AppleNotificationServer struct {
	ApplePushSettings ApplePushSettings
	AppleClient       *apns.Client
	logger            *Logger
}

func NewAppleNotificationServer(settings ApplePushSettings, logger *Logger) NotificationServer {
	return &AppleNotificationServer{
		ApplePushSettings: settings,
		logger:            logger,
	}
}

func (me *AppleNotificationServer) Initialize() bool {
	me.logger.Infof("Initializing apple notification server for type=%v", me.ApplePushSettings.Type)

	if len(me.ApplePushSettings.ApplePushCertPrivate) > 0 {
		appleCert, appleCertErr := certificate.FromPemFile(me.ApplePushSettings.ApplePushCertPrivate, me.ApplePushSettings.ApplePushCertPassword)
		if appleCertErr != nil {
			me.logger.Panicf("Failed to load the apple pem cert err=%v for type=%v", appleCertErr, me.ApplePushSettings.Type)
			return false
		}

		if me.ApplePushSettings.ApplePushUseDevelopment {
			me.AppleClient = apns.NewClient(appleCert).Development()
		} else {
			me.AppleClient = apns.NewClient(appleCert).Production()
		}

		// Override the native transport.
		proxyServer := getProxyServer()
		if proxyServer != "" {
			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{appleCert},
			}

			transport := &http.Transport{
				TLSClientConfig: tlsConfig,
				Proxy: func(request *http.Request) (*url.URL, error) {
					return url.Parse(proxyServer)
				},
				IdleConnTimeout: apns.HTTPClientTimeout,
			}
			err := http2.ConfigureTransport(transport)
			if err != nil {
				me.logger.Errorf("Transport Error: %v", err)
				return false
			}

			me.AppleClient.HTTPClient.Transport = transport
		}

		return true
	} else {
		me.logger.Errorf("Apple push notifications not configured.  Missing ApplePushCertPrivate. for type=%v", me.ApplePushSettings.Type)
		return false
	}
}

func (me *AppleNotificationServer) SendNotification(msg *PushNotification) PushResponse {

	data := payload.NewPayload()
	data.Badge(msg.Badge)

	notification := &apns.Notification{}
	notification.DeviceToken = msg.DeviceID
	notification.Payload = data
	notification.Topic = me.ApplePushSettings.ApplePushTopic

	var pushType = msg.Type
	if msg.IsIDLoaded {
		data.Category(msg.Category)
		data.Sound("default")
		data.Custom("version", msg.Version)
		data.Custom("id_loaded", true)
		data.MutableContent()
		data.AlertBody(msg.Message)
	} else {
		switch msg.Type {
		case PushTypeMessage:
			data.Category(msg.Category)
			data.Sound("default")
			data.Custom("version", msg.Version)
			data.MutableContent()

			if len(msg.ChannelName) > 0 && msg.Version == "v2" {
				data.AlertTitle(msg.ChannelName)
				data.AlertBody(emoji.Sprint(msg.Message))
				data.Custom("channel_name", msg.ChannelName)
			} else {
				data.Alert(emoji.Sprint(msg.Message))

				if len(msg.ChannelName) > 0 {
					data.Custom("channel_name", msg.ChannelName)
				}
			}
		case PushTypeClear:
			data.ContentAvailable()
		case PushTypeUpdateBadge:
			// Handled by the apps, nothing else to do here
		}
	}

	incrementNotificationTotal(PushNotifyApple, pushType)
	data.Custom("type", pushType)

	if len(msg.AckID) > 0 {
		data.Custom("ack_id", msg.AckID)
	}

	if len(msg.ChannelID) > 0 {
		data.Custom("channel_id", msg.ChannelID)
		data.ThreadID(msg.ChannelID)
	}

	if len(msg.TeamID) > 0 {
		data.Custom("team_id", msg.TeamID)
	}

	if len(msg.SenderID) > 0 {
		data.Custom("sender_id", msg.SenderID)
	}

	if len(msg.SenderName) > 0 {
		data.Custom("sender_name", msg.SenderName)
	}

	if len(msg.PostID) > 0 {
		data.Custom("post_id", msg.PostID)
	}

	if len(msg.RootID) > 0 {
		data.Custom("root_id", msg.RootID)
	}

	if len(msg.OverrideUsername) > 0 {
		data.Custom("override_username", msg.OverrideUsername)
	}

	if len(msg.OverrideIconURL) > 0 {
		data.Custom("override_icon_url", msg.OverrideIconURL)
	}

	if len(msg.FromWebhook) > 0 {
		data.Custom("from_webhook", msg.FromWebhook)
	}

	if me.AppleClient != nil {
		me.logger.Infof("Sending apple push notification for device=%v and type=%v", me.ApplePushSettings.Type, msg.Type)
		start := time.Now()
		res, err := me.AppleClient.Push(notification)
		observerNotificationResponse(PushNotifyApple, time.Since(start).Seconds())
		if err != nil {
			me.logger.Errorf("Failed to send apple push sid=%v did=%v err=%v type=%v", msg.ServerID, msg.DeviceID, err, me.ApplePushSettings.Type)
			incrementFailure(PushNotifyApple, pushType, "RequestError")
			return NewErrorPushResponse("unknown transport error")
		}

		if !res.Sent() {
			if res.Reason == apns.ReasonBadDeviceToken || res.Reason == apns.ReasonUnregistered || res.Reason == apns.ReasonMissingDeviceToken || res.Reason == apns.ReasonDeviceTokenNotForTopic {
				me.logger.Infof("Failed to send apple push sending remove code res ApnsID=%v reason=%v code=%v type=%v", res.ApnsID, res.Reason, res.StatusCode, me.ApplePushSettings.Type)
				incrementRemoval(PushNotifyApple, pushType, res.Reason)
				return NewRemovePushResponse()
			}

			me.logger.Errorf("Failed to send apple push with res ApnsID=%v reason=%v code=%v type=%v", res.ApnsID, res.Reason, res.StatusCode, me.ApplePushSettings.Type)
			incrementFailure(PushNotifyApple, pushType, res.Reason)
			return NewErrorPushResponse("unknown send response error")
		}
	}

	if len(msg.AckID) > 0 {
		incrementSuccessWithAck(PushNotifyApple, pushType)
	} else {
		incrementSuccess(PushNotifyApple, pushType)
	}
	return NewOkPushResponse()
}
