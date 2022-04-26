// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"io"
)

const (
	PushNotifyApple   = "apple"
	PushNotifyAndroid = "android"

	PushTypeMessage     = "message"
	PushTypeClear       = "clear"
	PushTypeUpdateBadge = "update_badge"
	PushTypeSession     = "session"
	PushTypeTest        = "test"

	PushMessageV2 = "v2"

	PushSoundNone = "none"
)

type PushNotificationAck struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
}

type PushNotification struct {
	ID               string `json:"id"`
	AckID            string `json:"ack_id"`
	Platform         string `json:"platform"`
	ServerID         string `json:"server_id"`
	DeviceID         string `json:"device_id"`
	Category         string `json:"category"`
	Sound            string `json:"sound"`
	Message          string `json:"message"`
	TeamID           string `json:"team_id"`
	ChannelID        string `json:"channel_id"`
	PostID           string `json:"post_id"`
	RootID           string `json:"root_id"`
	ChannelName      string `json:"channel_name"`
	Type             string `json:"type"`
	SenderName       string `json:"sender_name"`
	SenderID         string `json:"sender_id"`
	OverrideUsername string `json:"override_username"`
	OverrideIconURL  string `json:"override_icon_url"`
	FromWebhook      string `json:"from_webhook"`
	Version          string `json:"version"`
	AppVersion       int    `json:"app_version,omitempty"`
	Badge            int    `json:"badge"`
	ContentAvailable int    `json:"cont_ava"`
	IsCRTEnabled     bool   `json:"is_crt_enabled"`
	IsIDLoaded       bool   `json:"is_id_loaded"`
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
	} else {
		return nil
	}
}

func (me *PushNotificationAck) ToJSON() string {
	b, err := json.Marshal(me)
	if err != nil {
		return ""
	}
	return string(b)
}

func PushNotificationAckFromJSON(data io.Reader) *PushNotificationAck {
	var me PushNotificationAck
	decoder := json.NewDecoder(data)
	err := decoder.Decode(&me)
	if err == nil {
		return &me
	}
	return nil
}
