// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"crypto/tls"
	"fmt"
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
}

func NewAppleNotificationServer(settings ApplePushSettings) NotificationServer {
	return &AppleNotificationServer{ApplePushSettings: settings}
}

func (me *AppleNotificationServer) Initialize() bool {
	LogInfo(fmt.Sprintf("Initializing apple notification server for type=%v", me.ApplePushSettings.Type))

	if len(me.ApplePushSettings.ApplePushCertPrivate) > 0 {
		appleCert, appleCertErr := certificate.FromPemFile(me.ApplePushSettings.ApplePushCertPrivate, me.ApplePushSettings.ApplePushCertPassword)
		if appleCertErr != nil {
			LogCritical(fmt.Sprintf("Failed to load the apple pem cert err=%v for type=%v", appleCertErr, me.ApplePushSettings.Type))
			return false
		}

		if me.ApplePushSettings.ApplePushUseDevelopment {
			me.AppleClient = apns.NewClient(appleCert).Development()
		} else {
			me.AppleClient = apns.NewClient(appleCert).Production()
		}

		// Override the native transport.
		if CfgPP.ProxyServer != "" {
			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{appleCert},
			}

			if len(appleCert.Certificate) > 0 {
				tlsConfig.BuildNameToCertificate()
			}

			transport := &http.Transport{
				TLSClientConfig: tlsConfig,
				Proxy: func(request *http.Request) (*url.URL, error) {
					return url.Parse(CfgPP.ProxyServer)
				},
				IdleConnTimeout: apns.HTTPClientTimeout,
			}
			err := http2.ConfigureTransport(transport)
			if err != nil {
				LogError(fmt.Sprintf("Transport Error: %v", err))
				return false
			}

			me.AppleClient.HTTPClient.Transport = transport
		}

		return true
	} else {
		LogError(fmt.Sprintf("Apple push notifications not configured.  Missing ApplePushCertPrivate. for type=%v", me.ApplePushSettings.Type))
		return false
	}
}

func (me *AppleNotificationServer) SendNotification(msg *PushNotification) PushResponse {

	data := payload.NewPayload()
	data.Badge(msg.Badge)

	notification := &apns.Notification{}
	notification.DeviceToken = msg.DeviceId
	notification.Payload = data
	notification.Topic = me.ApplePushSettings.ApplePushTopic

	var pushType = msg.Type
	if msg.IsIdLoaded {
		data.Category(msg.Category)
		data.Sound("default")
		data.Custom("version", msg.Version)
		data.Custom("id_loaded", true)
		data.MutableContent()
		data.AlertBody(msg.Message)
	} else {
		switch msg.Type {
		case PUSH_TYPE_MESSAGE:
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
		case PUSH_TYPE_CLEAR:
			data.ContentAvailable()
		case PUSH_TYPE_UPDATE_BADGE:
			// Handled by the apps, nothing else to do here
		}
	}

	incrementNotificationTotal(PUSH_NOTIFY_APPLE, pushType)
	data.Custom("type", pushType)

	if len(msg.AckId) > 0 {
		data.Custom("ack_id", msg.AckId)
	}

	if len(msg.ChannelId) > 0 {
		data.Custom("channel_id", msg.ChannelId)
		data.ThreadID(msg.ChannelId)
	}

	if len(msg.TeamId) > 0 {
		data.Custom("team_id", msg.TeamId)
	}

	if len(msg.SenderId) > 0 {
		data.Custom("sender_id", msg.SenderId)
	}

	if len(msg.SenderName) > 0 {
		data.Custom("sender_name", msg.SenderName)
	}

	if len(msg.PostId) > 0 {
		data.Custom("post_id", msg.PostId)
	}

	if len(msg.RootId) > 0 {
		data.Custom("root_id", msg.RootId)
	}

	if len(msg.OverrideUsername) > 0 {
		data.Custom("override_username", msg.OverrideUsername)
	}

	if len(msg.OverrideIconUrl) > 0 {
		data.Custom("override_icon_url", msg.OverrideIconUrl)
	}

	if len(msg.FromWebhook) > 0 {
		data.Custom("from_webhook", msg.FromWebhook)
	}

	if me.AppleClient != nil {
		LogInfo(fmt.Sprintf("Sending apple push notification for device=%v and type=%v", me.ApplePushSettings.Type, msg.Type))
		start := time.Now()
		res, err := me.AppleClient.Push(notification)
		observerNotificationResponse(PUSH_NOTIFY_APPLE, time.Since(start).Seconds())
		if err != nil {
			LogError(fmt.Sprintf("Failed to send apple push sid=%v did=%v err=%v type=%v", msg.ServerId, msg.DeviceId, err, me.ApplePushSettings.Type))
			incrementFailure(PUSH_NOTIFY_APPLE, pushType, "RequestError")
			return NewErrorPushResponse("unknown transport error")
		}

		if !res.Sent() {
			if res.Reason == apns.ReasonBadDeviceToken || res.Reason == apns.ReasonUnregistered || res.Reason == apns.ReasonMissingDeviceToken || res.Reason == apns.ReasonDeviceTokenNotForTopic {
				LogInfo(fmt.Sprintf("Failed to send apple push sending remove code res ApnsID=%v reason=%v code=%v type=%v", res.ApnsID, res.Reason, res.StatusCode, me.ApplePushSettings.Type))
				incrementRemoval(PUSH_NOTIFY_APPLE, pushType, res.Reason)
				return NewRemovePushResponse()
			}

			LogError(fmt.Sprintf("Failed to send apple push with res ApnsID=%v reason=%v code=%v type=%v", res.ApnsID, res.Reason, res.StatusCode, me.ApplePushSettings.Type))
			incrementFailure(PUSH_NOTIFY_APPLE, pushType, res.Reason)
			return NewErrorPushResponse("unknown send response error")
		}
	}

	if len(msg.AckId) > 0 {
		incrementSuccessWithAck(PUSH_NOTIFY_APPLE, pushType)
	} else {
		incrementSuccess(PUSH_NOTIFY_APPLE, pushType)
	}
	return NewOkPushResponse()
}
