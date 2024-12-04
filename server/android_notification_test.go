// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"errors"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.Error(t, NewAndroidNotificationServer(cfg.AndroidPushSettings[0], logger, nil, cfg.SendTimeoutSec).Initialize())

	f, err := os.CreateTemp("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name()) // clean up

	cfg.AndroidPushSettings[0].ServiceFileLocation = f.Name()

	// Verify error for bad JSON
	_, err = f.Write([]byte("badJSON"))
	require.NoError(t, err)
	require.Error(t, NewAndroidNotificationServer(cfg.AndroidPushSettings[0], logger, nil, cfg.SendTimeoutSec).Initialize())

	require.NoError(t, f.Truncate(0))
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	// Verify no error for dummy JSON
	require.NoError(t, json.NewEncoder(f).Encode(serviceAccount{
		Type:      "service_account",
		ProjectID: "sample",
	}))
	require.NoError(t, f.Sync())
	require.NoError(t, NewAndroidNotificationServer(cfg.AndroidPushSettings[0], logger, nil, cfg.SendTimeoutSec).Initialize())

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
