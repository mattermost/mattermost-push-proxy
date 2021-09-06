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
	metrics             *metrics
	logger              *Logger
}

func NewAndroidNotificationServer(settings AndroidPushSettings, logger *Logger, metrics *metrics) NotificationServer {
	return &AndroidNotificationServer{
		AndroidPushSettings: settings,
		metrics:             metrics,
		logger:              logger,
	}
}

func (me *AndroidNotificationServer) Initialize() bool {
	me.logger.Infof("Initializing Android notification server for type=%v", me.AndroidPushSettings.Type)

	if me.AndroidPushSettings.AndroidAPIKey == "" {
		me.logger.Error("Android push notifications not configured.  Missing AndroidAPIKey.")
		return false
	}

	return true
}

func (me *AndroidNotificationServer) SendNotification(msg *PushNotification) PushResponse {
	pushType := msg.Type
	data := map[string]interface{}{
		"ack_id":         msg.AckID,
		"type":           pushType,
		"badge":          msg.Badge,
		"version":        msg.Version,
		"channel_id":     msg.ChannelID,
		"is_crt_enabled": msg.IsCRTEnabled,
	}

	if msg.IsIDLoaded {
		data["post_id"] = msg.PostID
		data["message"] = msg.Message
		data["id_loaded"] = true
		data["sender_id"] = msg.SenderID
		data["sender_name"] = "Someone"
		data["root_id"] = msg.RootID
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

	if me.metrics != nil {
		me.metrics.incrementNotificationTotal(PushNotifyAndroid, pushType)
	}
	fcmMsg := &fcm.Message{
		To:       msg.DeviceID,
		Data:     data,
		Priority: "high",
	}

	if me.AndroidPushSettings.AndroidAPIKey != "" {
		sender, err := fcm.NewClient(me.AndroidPushSettings.AndroidAPIKey)
		if err != nil {
			if me.metrics != nil {
				me.metrics.incrementFailure(PushNotifyAndroid, pushType, "invalid ApiKey")
			}
			return NewErrorPushResponse(err.Error())
		}

		me.logger.Infof("Sending android push notification for device=%v type=%v ackId=%v", me.AndroidPushSettings.Type, msg.Type, msg.AckID)

		start := time.Now()
		resp, err := sender.SendWithRetry(fcmMsg, 2)
		if me.metrics != nil {
			me.metrics.observerNotificationResponse(PushNotifyAndroid, time.Since(start).Seconds())
		}

		if err != nil {
			me.logger.Errorf("Failed to send FCM push sid=%v did=%v err=%v type=%v", msg.ServerID, msg.DeviceID, err, me.AndroidPushSettings.Type)
			if me.metrics != nil {
				me.metrics.incrementFailure(PushNotifyAndroid, pushType, "unknown transport error")
			}
			return NewErrorPushResponse("unknown transport error")
		}

		if resp.Failure > 0 {
			fcmError := resp.Results[0].Error

			if fcmError == fcm.ErrInvalidRegistration || fcmError == fcm.ErrNotRegistered || fcmError == fcm.ErrMissingRegistration {
				me.logger.Infof("Android response failure sending remove code: %v type=%v", resp, me.AndroidPushSettings.Type)
				if me.metrics != nil {
					me.metrics.incrementRemoval(PushNotifyAndroid, pushType, fcmError.Error())
				}
				return NewRemovePushResponse()
			}

			me.logger.Errorf("Android response failure: %v type=%v", resp, me.AndroidPushSettings.Type)
			if me.metrics != nil {
				me.metrics.incrementFailure(PushNotifyAndroid, pushType, fcmError.Error())
			}
			return NewErrorPushResponse(fcmError.Error())
		}
	}

	if me.metrics != nil {
		if msg.AckID != "" {
			me.metrics.incrementSuccessWithAck(PushNotifyAndroid, pushType)
		} else {
			me.metrics.incrementSuccess(PushNotifyAndroid, pushType)
		}
	}
	return NewOkPushResponse()
}
