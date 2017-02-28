// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"

	"github.com/alexjlockwood/gcm"
	"github.com/kyokomi/emoji"
)

type AndroidNotificationServer struct {
	AndroidPushSettings AndroidPushSettings
}

func NewAndroideNotificationServer(settings AndroidPushSettings) NotificationServer {
	return &AndroidNotificationServer{AndroidPushSettings: settings}
}

func (me *AndroidNotificationServer) Initialize() bool {
	LogInfo(fmt.Sprintf("Initializing Android notificaiton server for type=%v", me.AndroidPushSettings.Type))

	if len(me.AndroidPushSettings.AndroidApiKey) == 0 {
		LogError("Android push notifications not configured.  Mssing AndroidApiKey.")
		return false
	}

	return true
}

func (me *AndroidNotificationServer) SendNotification(msg *PushNotification) PushResponse {
	var data map[string]interface{}
	if msg.Type == PUSH_TYPE_CLEAR {
		data = map[string]interface{}{"type": PUSH_TYPE_CLEAR, "channel_id": msg.ChannelId, "team_id": msg.TeamId}
	} else {
		data = map[string]interface{}{"type": PUSH_TYPE_MESSAGE, "message": emoji.Sprint(msg.Message), "channel_id": msg.ChannelId, "channel_name": msg.ChannelName, "team_id": msg.TeamId}
	}

	regIDs := []string{msg.DeviceId}
	gcmMsg := gcm.NewMessage(data, regIDs...)

	sender := &gcm.Sender{ApiKey: me.AndroidPushSettings.AndroidApiKey}

	if len(me.AndroidPushSettings.AndroidApiKey) > 0 {
		LogInfo("Sending android push notification")
		resp, err := sender.Send(gcmMsg, 2)

		if err != nil {
			LogError(fmt.Sprintf("Failed to send GCM push sid=%v did=%v err=%v", msg.ServerId, msg.DeviceId, err))
			return NewErrorPushResponse("unknown transport error")
		}

		if resp.Failure > 0 {

			LogError(fmt.Sprintf("Android response failure: %v", resp))

			if len(resp.Results) > 0 && resp.Results[0].Error == "InvalidRegistration" {
				return NewRemovePushResponse()

			} else {
				return NewErrorPushResponse("unknown send response error")
			}
		}
	}

	return NewOkPushResponse()
}
