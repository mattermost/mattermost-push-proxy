// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	apns "github.com/sideshow/apns2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SendNotification dispatches by msg.Transport: "voip" routes through the
// VoIP send path, anything else through the standard alert path. Routing is
// observable via the transport label.
func TestSendNotificationTransportRouting(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transport string
	}{
		{"VoIP transport", PushTransportVoIP},
		{"default transport", PushTransportDefault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newMetrics()
			defer m.shutdown()

			srv := &AppleNotificationServer{
				ApplePushSettings: ApplePushSettings{
					ApplePushTopic: "com.mattermost.rnbeta",
				},
				metrics: m,
			}

			msg := &PushNotification{
				Platform:  PushNotifyApple + "_rn",
				DeviceID:  "tok",
				Type:      PushTypeMessage,
				Transport: tc.transport,
			}
			resp := srv.SendNotification(msg)
			require.Equal(t, NewOkPushResponse(), resp)

			got := testutil.ToFloat64(m.metricNotificationsTotal.WithLabelValues(PushNotifyApple, PushTypeMessage, tc.transport))
			require.Equal(t, float64(1), got, "expected notifications_total{transport=%q} to be 1", tc.transport)

			// The opposite transport label is untouched — proves the branch
			// picked the right one rather than incrementing both.
			other := PushTransportDefault
			if tc.transport == PushTransportDefault {
				other = PushTransportVoIP
			}
			otherCount := testutil.ToFloat64(m.metricNotificationsTotal.WithLabelValues(PushNotifyApple, PushTypeMessage, other))
			require.Equal(t, float64(0), otherCount, "expected notifications_total{transport=%q} to be 0", other)
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
			DeviceID:    "tok",
			Type:        PushTypeMessage,
			SubType:     PushSubTypeCalls,
			ChannelID:   "channel1",
			ServerID:    "server1",
			PostID:      "post1",
			RootID:      "thread1",
			SenderID:    "sender1",
			SenderName:  "Sender Name",
			ChannelName: "Channel Name",
			IsIDLoaded:  true,
			AckID:       "ack1",
			Signature:   "signed",
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))

		// Required routing fields.
		assert.Equal(t, "channel1", body["channel_id"])
		assert.Equal(t, "server1", body["server_id"])
		assert.Equal(t, "post1", body["post_id"])
		assert.Equal(t, "thread1", body["thread_id"])
		assert.Equal(t, "sender1", body["sender_id"])
		assert.Equal(t, PushTypeMessage, body["type"])
		assert.Equal(t, PushSubTypeCalls, body["sub_type"])
		assert.Equal(t, true, body["id_loaded"])
		assert.Equal(t, "ack1", body["ack_id"])
		assert.Equal(t, "signed", body["signature"])

		// Populated optional fields are forwarded.
		assert.Equal(t, "Sender Name", body["sender_name"])
		assert.Equal(t, "Channel Name", body["channel_name"])

		// content-available=1 so iOS wakes the app even when locked.
		aps, ok := body["aps"].(map[string]any)
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

}

func marshalPayload(t *testing.T, n *apns.Notification) map[string]any {
	t.Helper()
	raw, err := n.MarshalJSON()
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}
