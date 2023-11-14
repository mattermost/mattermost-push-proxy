// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

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
	SubType          string `json:"sub_type,omitempty"`
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
