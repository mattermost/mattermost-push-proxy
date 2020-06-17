// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"time"

	fcm "github.com/appleboy/go-fcm"
	"github.com/kyokomi/emoji"
)

type AndroidNotificationServer struct {
	AndroidPushSettings AndroidPushSettings
	logger              *Logger
}

func NewAndroidNotificationServer(settings AndroidPushSettings, logger *Logger) NotificationServer {
	return &AndroidNotificationServer{
		AndroidPushSettings: settings,
		logger:              logger,
	}
}

func (me *AndroidNotificationServer) Initialize() bool {
	me.logger.Infof("Initializing Android notification server for type=%v", me.AndroidPushSettings.Type)

	if len(me.AndroidPushSettings.AndroidAPIKey) == 0 {
		me.logger.Error("Android push notifications not configured.  Missing AndroidAPIKey.")
		return false
	}

	return true
}

func (me *AndroidNotificationServer) SendNotification(msg *PushNotification) PushResponse {
	pushType := msg.Type
	data := map[string]interface{}{
		"ack_id":     msg.AckID,
		"type":       pushType,
		"badge":      msg.Badge,
		"version":    msg.Version,
		"channel_id": msg.ChannelID,
	}

	if msg.IsIDLoaded {
		data["post_id"] = msg.PostID
		data["message"] = msg.Message
		data["id_loaded"] = true
		data["sender_id"] = msg.SenderID
		data["sender_name"] = "Someone"
	} else if pushType == PushTypeMessage || pushType == PushTypeSession {
		data["team_id"] = msg.TeamID
		data["sender_id"] = msg.SenderID
		data["sender_name"] = msg.SenderName
		data["message"] = emoji.Sprint(msg.Message)
		data["channel_name"] = msg.ChannelName
		data["post_id"] = msg.PostID
		data["root_id"] = msg.RootID
		data["override_username"] = msg.OverrideUsername
		data["override_icon_url"] = msg.OverrideIconURL
		data["from_webhook"] = msg.FromWebhook
	}

	incrementNotificationTotal(PushNotifyAndroid, pushType)

	fcmMsg := &fcm.Message{
		To:       msg.DeviceID,
		Data:     data,
		Priority: "high",
	}

	if len(me.AndroidPushSettings.AndroidAPIKey) > 0 {
		sender, err := fcm.NewClient(me.AndroidPushSettings.AndroidAPIKey)
		if err != nil {
			incrementFailure(PushNotifyAndroid, pushType, "invalid ApiKey")
			return NewErrorPushResponse(err.Error())
		}

		me.logger.Infof("Sending android push notification for device=%v and type=%v", me.AndroidPushSettings.Type, msg.Type)

		start := time.Now()
		resp, err := sender.SendWithRetry(fcmMsg, 2)
		observerNotificationResponse(PushNotifyAndroid, time.Since(start).Seconds())

		if err != nil {
			me.logger.Errorf("Failed to send FCM push sid=%v did=%v err=%v type=%v", msg.ServerID, msg.DeviceID, err, me.AndroidPushSettings.Type)
			incrementFailure(PushNotifyAndroid, pushType, "unknown transport error")
			return NewErrorPushResponse("unknown transport error")
		}

		if resp.Failure > 0 {
			fcmError := resp.Results[0].Error

			if fcmError == fcm.ErrInvalidRegistration || fcmError == fcm.ErrNotRegistered || fcmError == fcm.ErrMissingRegistration {
				me.logger.Infof("Android response failure sending remove code: %v type=%v", resp, me.AndroidPushSettings.Type)
				incrementRemoval(PushNotifyAndroid, pushType, fcmError.Error())
				return NewRemovePushResponse()
			}

			me.logger.Errorf("Android response failure: %v type=%v", resp, me.AndroidPushSettings.Type)
			incrementFailure(PushNotifyAndroid, pushType, fcmError.Error())
			return NewErrorPushResponse(fcmError.Error())
		}
	}

	if len(msg.AckID) > 0 {
		incrementSuccessWithAck(PushNotifyAndroid, pushType)
	} else {
		incrementSuccess(PushNotifyAndroid, pushType)
	}
	return NewOkPushResponse()
}
