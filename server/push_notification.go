// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	PUSH_NOTIFY_APPLE   = "apple"
	PUSH_NOTIFY_ANDROID = "android"
	PUSH_TYPE_MESSAGE   = "message"
	PUSH_TYPE_CLEAR     = "clear"

	// TLSDialTimeout is the maximum amount of time a dial will wait for a connect
	// to complete.
	TLSDialTimeout = 20 * time.Second

	// HTTPClientTimeout specifies a time limit for requests made by the
	// HTTPClient. The timeout includes connection time, any redirects,
	// and reading the response body.
	HTTPClientTimeout = 30 * time.Second
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   TLSDialTimeout,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   TLSDialTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: enableInsecureConnections,
		},
	},
	Timeout: HTTPClientTimeout,
}

type PushNotification struct {
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
	ChannelName      string `json:"channel_name"`
	Type             string `json:"type"`
	SenderId         string `json:"sender_id"`
	OverrideUsername string `json:"override_username"`
	OverrideIconUrl  string `json:"override_icon_url"`
	FromWebhook      string `json:"from_webhook"`
}

func (me *PushNotification) ToJson() string {
	b, err := json.Marshal(me)
	if err != nil {
		return ""
	} else {
		return string(b)
	}
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
