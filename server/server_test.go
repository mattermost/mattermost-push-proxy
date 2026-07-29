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

	"github.com/mattermost/mattermost/server/public/model"
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

	msg := model.PushNotification{}
	msg.Message = "test"
	msg.Badge = 1
	msg.DeviceId = "test"

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
	msg.ServerId = "test"
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

	msg := model.PushNotification{}
	msg.Message = "test"
	msg.Badge = 1
	msg.Platform = model.PushNotifyAndroid
	msg.ServerId = "test"
	msg.DeviceId = "test"

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

func TestHandleAckNotificationIncrementsStats(t *testing.T) {
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	srv := New(&ConfigPushProxy{}, logger)

	ack := model.PushNotificationAck{
		Id:               "ack1",
		ClientPlatform:   model.PushNotifyAndroid,
		NotificationType: model.PushTypeMessage,
	}
	buf, err := json.Marshal(ack)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ack", bytes.NewReader(buf))
	res := httptest.NewRecorder()
	srv.handleAckNotification(res, req)
	require.Equal(t, http.StatusOK, res.Code)

	_, _, acks := srv.stats.swap()
	assert.Equal(t, int64(1), acks)
}

func TestLogThroughput(t *testing.T) {
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	srv := New(&ConfigPushProxy{}, logger)

	srv.stats.incrementAndroidSend()
	srv.stats.incrementAppleSend()
	srv.stats.incrementAck()

	// Emitting the throughput report must consume (reset) the counters.
	assert.NotPanics(t, srv.logThroughput)

	android, apple, acks := srv.stats.swap()
	assert.Equal(t, int64(0), android)
	assert.Equal(t, int64(0), apple)
	assert.Equal(t, int64(0), acks)

	// A report over an empty window is a safe no-op.
	assert.NotPanics(t, srv.logThroughput)
}

type fakeExpiryTarget struct {
	calls int
}

func (f *fakeExpiryTarget) SendNotification(_ int, _ *model.PushNotification) PushResponse {
	return NewOkPushResponse()
}
func (f *fakeExpiryTarget) Initialize() error      { return nil }
func (f *fakeExpiryTarget) checkCredentialExpiry() { f.calls++ }

// plainTarget implements NotificationServer but not the credential-expiry
// interface, so it must be skipped by the periodic check.
type plainTarget struct{}

func (plainTarget) SendNotification(_ int, _ *model.PushNotification) PushResponse {
	return NewOkPushResponse()
}
func (plainTarget) Initialize() error { return nil }

func TestServerCheckCredentialExpiry(t *testing.T) {
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	srv := New(&ConfigPushProxy{}, logger)
	ft := &fakeExpiryTarget{}
	srv.pushTargets["apple"] = ft
	srv.pushTargets["android"] = plainTarget{}

	// Targets that implement checkCredentialExpiry are called; those that
	// don't are skipped without panicking.
	assert.NotPanics(t, srv.checkCredentialExpiry)
	assert.Equal(t, 1, ft.calls)

	srv.checkCredentialExpiry()
	assert.Equal(t, 2, ft.calls)
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
