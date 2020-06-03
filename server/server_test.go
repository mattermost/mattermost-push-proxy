// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBasicServer(t *testing.T) {
	fileName := FindConfigFile("mattermost-push-proxy.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)

	logger := NewLogger(cfg)
	srv := New(cfg, logger)
	srv.Start()

	time.Sleep(time.Second * 2)

	msg := PushNotification{}
	msg.Message = "test"
	msg.Badge = 1
	msg.DeviceID = "test"

	// Test for missing server Id
	client := http.Client{}
	rq, _ := http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", strings.NewReader(msg.ToJson()))
	if resp, err := client.Do(rq); err != nil {
		t.Fatal(err)
	} else {
		pr := PushResponseFromJson(resp.Body)
		if pr == nil || pr[PUSH_STATUS] != PUSH_STATUS_FAIL {
			t.Fatal("invalid response")
		}
	}

	// Test for missing platform type
	msg.ServerID = "test"
	client = http.Client{}
	rq, _ = http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", strings.NewReader(msg.ToJson()))
	if resp, err := client.Do(rq); err != nil {
		t.Fatal(err)
	} else {
		pr := PushResponseFromJson(resp.Body)
		if pr == nil || pr[PUSH_STATUS] != PUSH_STATUS_FAIL {
			t.Fatal("invalid response")
		}
	}

	// Test for junk platform type
	msg.Platform = "junk"
	rq, _ = http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", strings.NewReader(msg.ToJson()))
	if resp, err := client.Do(rq); err != nil {
		t.Fatal(err)
	} else {
		pr := PushResponseFromJson(resp.Body)
		println(pr.ToJson())
		if pr == nil || pr[PUSH_STATUS] != PUSH_STATUS_FAIL {
			t.Fatal("invalid response")
		}
	}

	srv.Stop()
	time.Sleep(time.Second * 2)
}

func TestAndroidSend(t *testing.T) {
	fileName := FindConfigFile("mattermost-push-proxy.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)

	cfg.AndroidPushSettings[0].AndroidAPIKey = "junk"
	logger := NewLogger(cfg)
	srv := New(cfg, logger)
	srv.Start()

	time.Sleep(time.Second * 2)

	msg := PushNotification{}
	msg.Message = "test"
	msg.Badge = 1
	msg.Platform = PushNotifyAndroid
	msg.ServerID = "test"
	msg.DeviceID = "test"

	client := http.Client{}
	rq, _ := http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", strings.NewReader(msg.ToJson()))
	if resp, err := client.Do(rq); err != nil {
		t.Fatal(err)
	} else {
		pr := PushResponseFromJson(resp.Body)
		if pr == nil || pr[PUSH_STATUS] != PUSH_STATUS_FAIL {
			t.Fatal("invalid response")
		}
	}

	srv.Stop()
	time.Sleep(time.Second * 2)
}
