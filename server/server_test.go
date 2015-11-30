// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	LoadConfig("config-push-proxy.json")
	Start()
	time.Sleep(time.Second * 2)

	msg := PushNotification{}
	msg.Platform = PUSH_NOTIFY_APPLE
	msg.Message = "test"
	msg.Badge = 1
	msg.DeviceId = "test"
	msg.ServerId = "test"

	client := http.Client{}
	rq, _ := http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", strings.NewReader(msg.ToJson()))

	if _, err := client.Do(rq); err != nil {
		t.Fatal(err)
	}

	Stop()
}
