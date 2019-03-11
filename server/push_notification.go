// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"io"
)

const (
	PUSH_NOTIFY_APPLE   = "apple"
	PUSH_NOTIFY_ANDROID = "android"
	PUSH_TYPE_MESSAGE   = "message"
	PUSH_TYPE_CLEAR     = "clear"
)

type PushNotificationAck struct {
	Id         string `json:"id"`
	ReceivedAt int64  `json:"received_at"`
	AckAt      int64  `json:"ack_at"`
}

type PushNotification struct {
	Id               string `json:"id"`
	AckId            string `json:"ack_id"`
	Platform         string `json:"platform"`
	ServerId         string `json:"server_id"`
	DeviceId         string `json:"device_id"`
	Category         string `json:"category"`
	Sound            string `json:"sound"`
	Message          string `json:"message"`
	Badge            int    `json:"badge"`
	ContentAvailable int    `json:"cont_ava"`
	TeamId           string `json:"team_id"`
	ChannelId        string `json:"channel_id"`
	PostId           string `json:"post_id"`
	RootId           string `json:"root_id"`
	ChannelName      string `json:"channel_name"`
	Type             string `json:"type"`
	SenderId         string `json:"sender_id"`
	OverrideUsername string `json:"override_username"`
	OverrideIconUrl  string `json:"override_icon_url"`
	FromWebhook      string `json:"from_webhook"`
	Version          string `json:"version"`
}

func (me *PushNotification) ToJson() string {
	b, err := json.Marshal(me)
	if err != nil {
		return ""
	}
	return string(b)
}

func PushNotificationFromJson(data io.Reader) *PushNotification {
	decoder := json.NewDecoder(data)
	var me PushNotification
	err := decoder.Decode(&me)
	if err == nil {
		return &me
	}
	return nil
}

func (me *PushNotificationAck) ToJson() string {
	b, err := json.Marshal(me)
	if err != nil {
		return ""
	}
	return string(b)
}

func PushNotificationAckFromJson(data io.Reader) *PushNotificationAck {
	decoder := json.NewDecoder(data)
	var me PushNotificationAck
	err := decoder.Decode(&me)
	if err == nil {
		return &me
	}
	return nil
}
