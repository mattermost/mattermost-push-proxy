// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"time"

	"github.com/appleboy/go-fcm"
	"github.com/kyokomi/emoji"
)

type AndroidNotificationServer struct {
	AndroidPushSettings AndroidPushSettings
}

func NewAndroideNotificationServer(settings AndroidPushSettings) NotificationServer {
	return &AndroidNotificationServer{AndroidPushSettings: settings}
}

func (me *AndroidNotificationServer) Initialize() bool {
	LogInfo(fmt.Sprintf("Initializing Android notification server for type=%v", me.AndroidPushSettings.Type))

	if len(me.AndroidPushSettings.AndroidApiKey) == 0 {
		LogError("Android push notifications not configured.  Missing AndroidApiKey.")
		return false
	}

	return true
}

func (me *AndroidNotificationServer) SendNotification(msg *PushNotification) PushResponse {
	pushType := msg.Type
	data := map[string]interface{}{
		"ack_id":  msg.AckId,
		"type":    pushType,
		"badge":   msg.Badge,
		"version": msg.Version,
	}

	if pushType == PUSH_TYPE_ID_LOADED {
		data["post_id"] = msg.PostId
	} else if pushType == PUSH_TYPE_CLEAR {
		data["channel_id"] = msg.ChannelId
	} else if pushType == PUSH_TYPE_MESSAGE {
		data["channel_id"] = msg.ChannelId
		data["team_id"] = msg.TeamId
		data["sender_id"] = msg.SenderId
		data["sender_name"] = msg.SenderName
		data["message"] = emoji.Sprint(msg.Message)
		data["channel_name"] = msg.ChannelName
		data["post_id"] = msg.PostId
		data["root_id"] = msg.RootId
		data["override_username"] = msg.OverrideUsername
		data["override_icon_url"] = msg.OverrideIconUrl
		data["from_webhook"] = msg.FromWebhook
	}

	incrementNotificationTotal(PUSH_NOTIFY_ANDROID, pushType)

	fcmMsg := &fcm.Message{
		To:       msg.DeviceId,
		Data:     data,
		Priority: "high",
	}

	if len(me.AndroidPushSettings.AndroidApiKey) > 0 {
		sender, err := fcm.NewClient(me.AndroidPushSettings.AndroidApiKey)
		if err != nil {
			incrementFailure(PUSH_NOTIFY_ANDROID, pushType, "invalid ApiKey")
			return NewErrorPushResponse(err.Error())
		}

		LogInfo(fmt.Sprintf("Sending android push notification for device=%v and type=%v", me.AndroidPushSettings.Type, msg.Type))

		start := time.Now()
		resp, err := sender.SendWithRetry(fcmMsg, 2)
		observerNotificationResponse(PUSH_NOTIFY_ANDROID, time.Since(start).Seconds())

		if err != nil {
			LogError(fmt.Sprintf("Failed to send FCM push sid=%v did=%v err=%v type=%v", msg.ServerId, msg.DeviceId, err, me.AndroidPushSettings.Type))
			incrementFailure(PUSH_NOTIFY_ANDROID, pushType, "unknown transport error")
			return NewErrorPushResponse("unknown transport error")
		}

		if resp.Failure > 0 {
			fcmError := resp.Results[0].Error

			if fcmError == fcm.ErrInvalidRegistration || fcmError == fcm.ErrNotRegistered || fcmError == fcm.ErrMissingRegistration {
				LogInfo(fmt.Sprintf("Android response failure sending remove code: %v type=%v", resp, me.AndroidPushSettings.Type))
				incrementRemoval(PUSH_NOTIFY_ANDROID, pushType, fcmError.Error())
				return NewRemovePushResponse()
			}

			LogError(fmt.Sprintf("Android response failure: %v type=%v", resp, me.AndroidPushSettings.Type))
			incrementFailure(PUSH_NOTIFY_ANDROID, pushType, fcmError.Error())
			return NewErrorPushResponse(fcmError.Error())
		}
	}

	if len(msg.AckId) > 0 {
		incrementSuccessWithAck(PUSH_NOTIFY_ANDROID, pushType)
	} else {
		incrementSuccess(PUSH_NOTIFY_ANDROID, pushType)
	}
	return NewOkPushResponse()
}
