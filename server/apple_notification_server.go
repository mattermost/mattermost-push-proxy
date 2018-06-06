// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"time"

	"github.com/kyokomi/emoji"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
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

		return true
	} else {
		LogError(fmt.Sprintf("Apple push notifications not configured.  Missing ApplePushCertPrivate. for type=%v", me.ApplePushSettings.Type))
		return false
	}
}

func (me *AppleNotificationServer) SendNotification(msg *PushNotification) PushResponse {
	notification := &apns.Notification{}
	notification.DeviceToken = msg.DeviceId
	payload := payload.NewPayload()
	notification.Payload = payload
	notification.Topic = me.ApplePushSettings.ApplePushTopic

	payload.Badge(msg.Badge)

	if msg.Type != PUSH_TYPE_CLEAR {
		payload.Category(msg.Category)
		payload.Sound("default")
		payload.Custom("version", msg.Version)

		if len(msg.ChannelName) > 0 && msg.Version == "v2" {
			payload.AlertTitle(msg.ChannelName)
			payload.AlertBody(emoji.Sprint(msg.Message))
			payload.Custom("channel_name", msg.ChannelName)
		} else {
			payload.Alert(emoji.Sprint(msg.Message))

			if len(msg.ChannelName) > 0 {
				payload.Custom("channel_name", msg.ChannelName)
			}
		}
	} else {
		payload.Alert("")
	}

	payload.Custom("type", msg.Type)

	if len(msg.ChannelId) > 0 {
		payload.Custom("channel_id", msg.ChannelId)
	}

	if len(msg.TeamId) > 0 {
		payload.Custom("team_id", msg.TeamId)
	}

	if len(msg.SenderId) > 0 {
		payload.Custom("sender_id", msg.SenderId)
	}

	if len(msg.PostId) > 0 {
		payload.Custom("post_id", msg.PostId)
	}

	if len(msg.RootId) > 0 {
		payload.Custom("root_id", msg.RootId)
	}

	if len(msg.OverrideUsername) > 0 {
		payload.Custom("override_username", msg.OverrideUsername)
	}

	if len(msg.OverrideIconUrl) > 0 {
		payload.Custom("override_icon_url", msg.OverrideIconUrl)
	}

	if len(msg.FromWebhook) > 0 {
		payload.Custom("from_webhook", msg.FromWebhook)
	}

	if me.AppleClient != nil {
		LogInfo(fmt.Sprintf("Sending apple push notification type=%v", me.ApplePushSettings.Type))
		start := time.Now()
		res, err := me.AppleClient.Push(notification)
		observeAPNSResponse(time.Since(start).Seconds())
		if err != nil {
			LogError(fmt.Sprintf("Failed to send apple push sid=%v did=%v err=%v type=%v", msg.ServerId, msg.DeviceId, err, me.ApplePushSettings.Type))
			incrementFailure(me.ApplePushSettings.Type)
			return NewErrorPushResponse("unknown transport error")
		}

		if !res.Sent() {
			if res.Reason == "BadDeviceToken" || res.Reason == "Unregistered" || res.Reason == "MissingDeviceToken" || res.Reason == "DeviceTokenNotForTopic" {
				LogInfo(fmt.Sprintf("Failed to send apple push sending remove code res ApnsID=%v reason=%v code=%v type=%v", res.ApnsID, res.Reason, res.StatusCode, me.ApplePushSettings.Type))
				incrementRemoval(me.ApplePushSettings.Type)
				return NewRemovePushResponse()
			}

			LogError(fmt.Sprintf("Failed to send apple push with res ApnsID=%v reason=%v code=%v type=%v", res.ApnsID, res.Reason, res.StatusCode, me.ApplePushSettings.Type))
			incrementFailure(me.ApplePushSettings.Type)
			return NewErrorPushResponse("unknown send response error")
		}
	}

	incrementSuccess(me.ApplePushSettings.Type)
	return NewOkPushResponse()
}
