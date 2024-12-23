// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mattermost/mattermost-push-proxy/internal/version"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicServer(t *testing.T) {
	fileName := FindConfigFile("mattermost-push-proxy.sample.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	srv := New(cfg, logger)
	srv.Start()

	time.Sleep(time.Second * 2)

	msg := PushNotification{}
	msg.Message = "test"
	msg.Badge = 1
	msg.DeviceID = "test"

	// Test for missing server Id
	client := http.Client{}
	buf, err := json.Marshal(msg)
	require.NoError(t, err)
	rq, _ := http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", bytes.NewReader(buf))
	if resp, err2 := client.Do(rq); err2 != nil {
		t.Fatal(err2)
	} else {
		pr := PushResponseFromJson(resp.Body)
		if pr == nil || pr[PUSH_STATUS] != PUSH_STATUS_FAIL {
			t.Fatal("invalid response")
		}
	}

	// Test for missing platform type
	msg.ServerID = "test"
	client = http.Client{}
	buf, err = json.Marshal(msg)
	require.NoError(t, err)
	rq, _ = http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", bytes.NewReader(buf))
	if resp, err2 := client.Do(rq); err2 != nil {
		t.Fatal(err2)
	} else {
		pr := PushResponseFromJson(resp.Body)
		if pr == nil || pr[PUSH_STATUS] != PUSH_STATUS_FAIL {
			t.Fatal("invalid response")
		}
	}

	// Test for junk platform type
	msg.Platform = "junk"
	buf, err = json.Marshal(msg)
	require.NoError(t, err)
	rq, _ = http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", bytes.NewReader(buf))
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

func TestAndroidSend(t *testing.T) {
	fileName := FindConfigFile("mattermost-push-proxy.sample.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)

	cfg.AndroidPushSettings[0].AndroidAPIKey = "junk"
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

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
	buf, err := json.Marshal(msg)
	require.NoError(t, err)
	rq, _ := http.NewRequest("POST", "http://localhost:8066/api/v1/send_push", bytes.NewReader(buf))
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

func TestServer_version(t *testing.T) {
	fileName := FindConfigFile("mattermost-push-proxy.sample.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	srv := New(cfg, logger)

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	res := httptest.NewRecorder()
	srv.version(res, req)
	assert.Equal(t, res.Code, http.StatusOK)

	info := version.VersionInfo()
	ret := struct {
		Version string
		Hash    string
	}{}
	err = json.NewDecoder(res.Body).Decode(&ret)
	assert.NoError(t, err)
	assert.Equal(t, info.BuildVersion, ret.Version)
	assert.Equal(t, info.BuildHash, ret.Hash)
}
