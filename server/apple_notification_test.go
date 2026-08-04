// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
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
		transport model.PushTransport
	}{
		{"VoIP transport", model.PushTransportVoIP},
		{"default transport", model.PushTransportStandard},
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

			msg := &model.PushNotification{
				Platform:  model.PushNotifyApple + "_rn",
				DeviceId:  "tok",
				Type:      model.PushTypeMessage,
				Transport: tc.transport,
			}
			resp := srv.SendNotification(1, msg)
			require.Equal(t, NewOkPushResponse(), resp)

			got := testutil.ToFloat64(m.metricNotificationsTotal.WithLabelValues(model.PushNotifyApple, model.PushTypeMessage, string(tc.transport)))
			require.Equal(t, float64(1), got, "expected notifications_total{transport=%q} to be 1", tc.transport)

			// The opposite transport label is untouched — proves the branch
			// picked the right one rather than incrementing both.
			other := model.PushTransportStandard
			if tc.transport == model.PushTransportStandard {
				other = model.PushTransportVoIP
			}
			otherCount := testutil.ToFloat64(m.metricNotificationsTotal.WithLabelValues(model.PushNotifyApple, model.PushTypeMessage, string(other)))
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
		msg := &model.PushNotification{
			DeviceId: "abcd1234",
			Type:     model.PushTypeMessage,
			SubType:  model.PushSubTypeCalls,
		}
		n := srv.buildVoIPNotification(msg)

		assert.Equal(t, "abcd1234", n.DeviceToken)
		assert.Equal(t, "com.mattermost.rnbeta.voip", n.Topic, "VoIP pushes must use the .voip subtopic")
		assert.Equal(t, apns.PriorityHigh, n.Priority)
		assert.Equal(t, apns.PushTypeVOIP, n.PushType)
	})

	t.Run("payload carries the routing fields the device needs", func(t *testing.T) {
		msg := &model.PushNotification{
			DeviceId:    "tok",
			Type:        model.PushTypeMessage,
			SubType:     model.PushSubTypeCalls,
			ChannelId:   "channel1",
			ServerId:    "server1",
			PostId:      "post1",
			RootId:      "thread1",
			SenderId:    "sender1",
			SenderName:  "Sender Name",
			ChannelName: "Channel Name",
			IsIdLoaded:  true,
			AckId:       "ack1",
			Signature:   "signed",
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))

		// Required routing fields.
		assert.Equal(t, "channel1", body["channel_id"])
		assert.Equal(t, "server1", body["server_id"])
		assert.Equal(t, "post1", body["post_id"])
		assert.Equal(t, "thread1", body["thread_id"])
		assert.Equal(t, "sender1", body["sender_id"])
		assert.Equal(t, model.PushTypeMessage, body["type"])
		assert.Equal(t, string(model.PushSubTypeCalls), body["sub_type"])
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
		msg := &model.PushNotification{
			DeviceId:   "tok",
			Type:       model.PushTypeMessage,
			SubType:    model.PushSubTypeCalls,
			ChannelId:  "channel1",
			ServerId:   "server1",
			IsIdLoaded: true,
			AckId:      "ack1",
			Signature:  "signed",
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))

		_, hasSender := body["sender_name"]
		_, hasChannel := body["channel_name"]
		assert.False(t, hasSender, "sender_name should not appear when not populated")
		assert.False(t, hasChannel, "channel_name should not appear when not populated")
	})

	t.Run("missing signature falls back to NO_SIGNATURE sentinel", func(t *testing.T) {
		msg := &model.PushNotification{
			DeviceId: "tok",
			Type:     model.PushTypeMessage,
			SubType:  model.PushSubTypeCalls,
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))
		assert.Equal(t, "NO_SIGNATURE", body["signature"])
	})

	t.Run("missing ack_id is omitted (no empty value on wire)", func(t *testing.T) {
		msg := &model.PushNotification{
			DeviceId: "tok",
			Type:     model.PushTypeMessage,
			SubType:  model.PushSubTypeCalls,
		}
		body := marshalPayload(t, srv.buildVoIPNotification(msg))
		_, hasAck := body["ack_id"]
		assert.False(t, hasAck, "ack_id should not appear when not populated")
	})

}

func TestCertExpiryReport(t *testing.T) {
	now := time.Now()

	for _, tc := range []struct {
		name        string
		hasCert     bool
		notAfter    time.Time
		wantLevel   expiryLevel
		wantMessage string
	}{
		{"token auth, no cert", false, time.Time{}, expiryNone, ""},
		{"cert present but unparseable expiry", true, time.Time{}, expiryWarn, "Could not determine Apple push certificate expiry"},
		{"plenty of validity", true, now.Add(90 * 24 * time.Hour), expiryNone, ""},
		{"just above warn threshold", true, now.Add(certExpiryWarnThreshold + time.Hour), expiryNone, ""},
		{"at warn threshold", true, now.Add(certExpiryWarnThreshold), expiryWarn, "Apple push certificate is expiring soon"},
		{"within warn window", true, now.Add(20 * 24 * time.Hour), expiryWarn, "Apple push certificate is expiring soon"},
		{"just above error threshold", true, now.Add(certExpiryErrorThreshold + time.Hour), expiryWarn, "Apple push certificate is expiring soon"},
		{"at error threshold", true, now.Add(certExpiryErrorThreshold), expiryError, "Apple push certificate is expiring soon"},
		{"within error window", true, now.Add(3 * 24 * time.Hour), expiryError, "Apple push certificate is expiring soon"},
		{"already expired", true, now.Add(-time.Hour), expiryError, "Apple push certificate has expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			level, message := certExpiryReport(tc.hasCert, tc.notAfter, now)
			assert.Equal(t, tc.wantLevel, level)
			assert.Equal(t, tc.wantMessage, message)
		})
	}
}

func TestCertNotAfter(t *testing.T) {
	notAfter := time.Now().Add(45 * 24 * time.Hour).Truncate(time.Second)
	der := newTestCertDER(t, notAfter)

	t.Run("uses populated leaf", func(t *testing.T) {
		leaf, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		got := certNotAfter(tls.Certificate{Leaf: leaf, Certificate: [][]byte{der}})
		assert.WithinDuration(t, notAfter, got, time.Second)
	})

	t.Run("parses from DER when leaf is nil", func(t *testing.T) {
		got := certNotAfter(tls.Certificate{Certificate: [][]byte{der}})
		assert.WithinDuration(t, notAfter, got, time.Second)
	})

	t.Run("zero time when nothing to parse", func(t *testing.T) {
		assert.True(t, certNotAfter(tls.Certificate{}).IsZero())
	})
}

func newTestCertDER(t *testing.T, notAfter time.Time) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test apns cert"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return der
}

func TestCheckCredentialExpiryNoCertIsNoop(t *testing.T) {
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	// Token (AuthKey) auth leaves certNotAfter at its zero value.
	srv := &AppleNotificationServer{
		ApplePushSettings: ApplePushSettings{Type: "apple"},
		logger:            logger,
	}

	assert.True(t, srv.certNotAfter.IsZero())
	assert.NotPanics(t, srv.checkCredentialExpiry)
}

func marshalPayload(t *testing.T, n *apns.Notification) map[string]any {
	t.Helper()
	raw, err := n.MarshalJSON()
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}
