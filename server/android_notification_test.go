// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func TestAndroidInitialize(t *testing.T) {
	fileName := FindConfigFile("mattermost-push-proxy.sample.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	// Verify error for no service file
	pushSettings := AndroidPushSettings{}
	cfg.AndroidPushSettings[0] = pushSettings
	require.Error(t, NewAndroidNotificationServer(cfg.AndroidPushSettings[0], logger, nil, nil, cfg.SendTimeoutSec, cfg.RetryTimeoutSec).Initialize())

	f, err := os.CreateTemp("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name()) // clean up

	cfg.AndroidPushSettings[0].ServiceFileLocation = f.Name()

	// Verify error for bad JSON
	_, err = f.Write([]byte("badJSON"))
	require.NoError(t, err)
	require.Error(t, NewAndroidNotificationServer(cfg.AndroidPushSettings[0], logger, nil, nil, cfg.SendTimeoutSec, cfg.RetryTimeoutSec).Initialize())

	require.NoError(t, f.Truncate(0))
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	// Verify no error for dummy JSON
	require.NoError(t, json.NewEncoder(f).Encode(serviceAccount{
		Type:      "service_account",
		ProjectID: "sample",
	}))
	require.NoError(t, f.Sync())
	require.NoError(t, NewAndroidNotificationServer(cfg.AndroidPushSettings[0], logger, nil, nil, cfg.SendTimeoutSec, cfg.RetryTimeoutSec).Initialize())

	require.NoError(t, f.Close())
}

// Copied from firebase.google.com/go/v4@v4.14.0/internal/errors.go
type ErrorCode string
type FirebaseError struct {
	ErrorCode ErrorCode
	String    string
	Response  *http.Response
	Ext       map[string]interface{}
}

func (fe *FirebaseError) Error() string {
	return fe.String
}

func TestAndroidCheckCredentialExpiry(t *testing.T) {
	t.Run("logs Error when credentials are rejected", func(t *testing.T) {
		logger, buf := newCapturingLogger(t)
		me := &AndroidNotificationServer{
			AndroidPushSettings: AndroidPushSettings{Type: "android"},
			logger:              logger,
			sendTimeout:         time.Second,
			validateToken: func(_ context.Context) error {
				return errors.New("service account key revoked")
			},
		}

		me.checkCredentialExpiry()

		records := flushLogs(t, logger, buf)
		require.Len(t, records, 1)
		assert.Equal(t, "error", records[0]["level"])
		assert.Contains(t, records[0]["msg"], "FCM credentials rejected")
		assert.Equal(t, "android", records[0]["target_type"])
	})

	t.Run("logs Warn on timeout", func(t *testing.T) {
		logger, buf := newCapturingLogger(t)
		me := &AndroidNotificationServer{
			AndroidPushSettings: AndroidPushSettings{Type: "android"},
			logger:              logger,
			sendTimeout:         time.Second,
			validateToken: func(_ context.Context) error {
				return context.DeadlineExceeded
			},
		}

		me.checkCredentialExpiry()

		records := flushLogs(t, logger, buf)
		require.Len(t, records, 1)
		assert.Equal(t, "warn", records[0]["level"])
		assert.Contains(t, records[0]["msg"], "timed out")
	})

	t.Run("validates with a bounded context and stays silent on success", func(t *testing.T) {
		logger, buf := newCapturingLogger(t)
		var hasDeadline bool
		me := &AndroidNotificationServer{
			AndroidPushSettings: AndroidPushSettings{Type: "android"},
			logger:              logger,
			sendTimeout:         time.Second,
			validateToken: func(ctx context.Context) error {
				_, hasDeadline = ctx.Deadline()
				return nil
			},
		}

		me.checkCredentialExpiry()

		assert.True(t, hasDeadline, "validation should be bounded by sendTimeout")
		assert.Empty(t, flushLogs(t, logger, buf), "successful validation should not log")
	})

	t.Run("no validator is a no-op", func(t *testing.T) {
		logger, buf := newCapturingLogger(t)
		me := &AndroidNotificationServer{
			AndroidPushSettings: AndroidPushSettings{Type: "android"},
			logger:              logger,
		}

		me.checkCredentialExpiry()

		assert.Nil(t, me.validateToken)
		assert.Empty(t, flushLogs(t, logger, buf))
	})
}

func TestGetErrorCode(t *testing.T) {
	var errorCode ErrorCode = "some error code"
	err := &FirebaseError{
		ErrorCode: errorCode,
	}

	extractedCode, found := getErrorCode(err)
	require.True(t, found)
	require.Equal(t, string(errorCode), extractedCode)

	extractedCode, found = getErrorCode(errors.New("non firebase error"))
	require.Equal(t, "", extractedCode)
	require.False(t, found)
}
