// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"strings"
	"testing"

	apns "github.com/sideshow/apns2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVoIPPlatformDispatchAlias(t *testing.T) {
	for _, tc := range []struct {
		name          string
		inputPlatform string
		expected      string
	}{
		{"apple_voip_rn aliases to apple_rn", "apple_voip_rn", "apple_rn"},
		{"apple_voip_rnbeta aliases to apple_rnbeta", "apple_voip_rnbeta", "apple_rnbeta"},
		{"apple_rn is unchanged", "apple_rn", "apple_rn"},
		{"apple_rnbeta is unchanged", "apple_rnbeta", "apple_rnbeta"},
		{"android_rn is unchanged", "android_rn", "android_rn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.inputPlatform
			if strings.HasPrefix(target, applePlatformVoIPPrefix) {
				target = "apple_" + strings.TrimPrefix(target, applePlatformVoIPPrefix)
			}
			assert.Equal(t, tc.expected, target)
		})
	}
}

func TestBuildVoIPNotification(t *testing.T) {
	srv := &AppleNotificationServer{
		ApplePushSettings: ApplePushSettings{
			ApplePushTopic: "com.mattermost.rnbeta",
		},
	}

	t.Run("APNs envelope shape (VoIP-specific)", func(t *testing.T) {
		msg := &PushNotification{
			DeviceID: "abcd1234",
			Type:     PushTypeMessage,
			SubType:  PushSubTypeCalls,
		}
		n := srv.buildVoIPNotification(msg)

		assert.Equal(t, "abcd1234", n.DeviceToken)
		assert.Equal(t, "com.mattermost.rnbeta.voip", n.Topic, "VoIP pushes must use the .voip subtopic")
		assert.Equal(t, apns.PriorityHigh, n.Priority)
		assert.Equal(t, apns.PushTypeVOIP, n.PushType)
	})

	t.Run("payload carries the routing fields the device needs", func(t *testing.T) {
		msg := &PushNotification{
			DeviceID:   "tok",
			Type:       PushTypeMessage,
			SubType:    PushSubTypeCalls,
			ChannelID:  "channel1",
			ServerID:   "server1",
			PostID:     "post1",
			RootID:     "thread1",
			SenderID:   "sender1",
			SenderName: "Sender Name",
			ChannelName: "Channel Name",
			IsIDLoaded: true,
			AckID:      "ack1",
			Signature:  "signed",
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))

		// Required routing fields.
		assert.Equal(t, "channel1", body["channel_id"])
		assert.Equal(t, "server1", body["server_id"])
		assert.Equal(t, "post1", body["post_id"])
		assert.Equal(t, "thread1", body["thread_id"])
		assert.Equal(t, "sender1", body["sender_id"])
		assert.Equal(t, string(PushTypeMessage), body["type"])
		assert.Equal(t, string(PushSubTypeCalls), body["sub_type"])
		assert.Equal(t, true, body["id_loaded"])
		assert.Equal(t, "ack1", body["ack_id"])
		assert.Equal(t, "signed", body["signature"])

		// Populated optional fields are forwarded.
		assert.Equal(t, "Sender Name", body["sender_name"])
		assert.Equal(t, "Channel Name", body["channel_name"])

		// content-available=1 so iOS wakes the app even when locked.
		aps, ok := body["aps"].(map[string]interface{})
		require.True(t, ok, "aps section missing")
		assert.EqualValues(t, 1, aps["content-available"])
	})

	t.Run("sender_name and channel_name are omitted when empty (IdLoaded mode)", func(t *testing.T) {
		msg := &PushNotification{
			DeviceID:   "tok",
			Type:       PushTypeMessage,
			SubType:    PushSubTypeCalls,
			ChannelID:  "channel1",
			ServerID:   "server1",
			IsIDLoaded: true,
			AckID:      "ack1",
			Signature:  "signed",
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))

		_, hasSender := body["sender_name"]
		_, hasChannel := body["channel_name"]
		assert.False(t, hasSender, "sender_name should not appear when not populated")
		assert.False(t, hasChannel, "channel_name should not appear when not populated")
	})

	t.Run("missing signature falls back to NO_SIGNATURE sentinel", func(t *testing.T) {
		msg := &PushNotification{
			DeviceID: "tok",
			Type:     PushTypeMessage,
			SubType:  PushSubTypeCalls,
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))
		assert.Equal(t, "NO_SIGNATURE", body["signature"])
	})

	t.Run("missing ack_id is omitted (no empty value on wire)", func(t *testing.T) {
		msg := &PushNotification{
			DeviceID: "tok",
			Type:     PushTypeMessage,
			SubType:  PushSubTypeCalls,
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))
		_, hasAck := body["ack_id"]
		assert.False(t, hasAck, "ack_id should not appear when not populated")
	})

	t.Run("category is forwarded when set (used by mobile to pick CallKit end reason)", func(t *testing.T) {
		msg := &PushNotification{
			DeviceID: "tok",
			Type:     PushTypeClear,
			SubType:  PushSubTypeCalls,
			Category: "answered_elsewhere",
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))
		assert.Equal(t, "answered_elsewhere", body["category"])
	})

	t.Run("category is omitted when empty", func(t *testing.T) {
		msg := &PushNotification{
			DeviceID: "tok",
			Type:     PushTypeMessage,
			SubType:  PushSubTypeCalls,
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))
		_, hasCategory := body["category"]
		assert.False(t, hasCategory, "category should not appear when not populated")
	})
}

func marshalPayload(t *testing.T, n *apns.Notification) map[string]interface{} {
	t.Helper()
	raw, err := n.MarshalJSON()
	require.NoError(t, err)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}
