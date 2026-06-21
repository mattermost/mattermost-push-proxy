// Copyright (c) 2026 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"golang.org/x/crypto/hkdf"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

var testVAPIDPrivateKey, testVAPIDPublicKey = generateTestVAPIDKeys()

func generateTestVAPIDKeys() (string, string) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		panic(err)
	}
	return priv, pub
}

// testWebPushSettings provides the base VAPID/subscriber config every test
// needs. The private-IP SSRF guard and the https requirement are always on
// (there's no global bypass), so tests that need to reach a local httptest
// server use testWebPushSettingsAllowing and trustTestServerCert instead.
func testWebPushSettings() WebPushSettings {
	return WebPushSettings{
		Type:            PushNotifyWebPush,
		VAPIDPublicKey:  testVAPIDPublicKey,
		VAPIDPrivateKey: testVAPIDPrivateKey,
		Subscriber:      "mailto:test@example.com",
	}
}

// testWebPushSettingsAllowing exempts host from the private-IP SSRF guard
// (it must still be https), for tests that prove the allowlist mechanism
// itself or just need to reach a local httptest.NewTLSServer.
func testWebPushSettingsAllowing(host string) WebPushSettings {
	s := testWebPushSettings()
	s.AllowedHosts = []string{host}
	return s
}

// trustTestServerCert makes webPushSrv's normalClient trust the given
// httptest.NewTLSServer(s)' self-signed certificate(s), so an
// AllowedHosts-exempted send in a test can reach them.
func trustTestServerCert(webPushSrv *WebPushNotificationServer, srvs ...*httptest.Server) {
	pool := x509.NewCertPool()
	for _, srv := range srvs {
		pool.AddCert(srv.Certificate())
	}
	webPushSrv.normalClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool},
	}
}

// unusedLoopbackPort returns a loopback port that was free at the time of
// the call, for tests that need a closed port to force connection-refused
// rather than depending on a hardcoded port being unused on the host.
func unusedLoopbackPort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// recipientKeys is a recipient's WebPush subscription keypair, generated
// in-test to stand in for a real browser/UnifiedPush subscriber.
type recipientKeys struct {
	privateKey *ecdh.PrivateKey
	authSecret []byte // 16-byte auth secret
}

func newRecipientKeys(t *testing.T) recipientKeys {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)

	auth := make([]byte, 16)
	_, err = rand.Read(auth)
	require.NoError(t, err)

	return recipientKeys{
		privateKey: priv,
		authSecret: auth,
	}
}

func (k recipientKeys) publicKey() []byte {
	return k.privateKey.PublicKey().Bytes()
}

func (k recipientKeys) p256dh() string {
	return base64.RawURLEncoding.EncodeToString(k.publicKey())
}

func (k recipientKeys) auth() string {
	return base64.RawURLEncoding.EncodeToString(k.authSecret)
}

// decrypt undoes the RFC8291 aes128gcm encryption webpush-go applied to the
// body, repeating its key derivation so the test can read the plaintext back.
func (k recipientKeys) decrypt(t *testing.T, body []byte) []byte {
	require.Greater(t, len(body), 21, "record too short to contain a header")

	salt := body[:16]
	idLen := int(body[20])
	senderPublicKeyBytes := body[21 : 21+idLen]
	ciphertext := body[21+idLen:]

	senderPublicKey, err := ecdh.P256().NewPublicKey(senderPublicKeyBytes)
	require.NoError(t, err, "sender public key is not a valid curve point")

	sharedSecret, err := k.privateKey.ECDH(senderPublicKey)
	require.NoError(t, err)

	infoBuf := bytes.NewBuffer([]byte("WebPush: info\x00"))
	infoBuf.Write(k.publicKey())
	infoBuf.Write(senderPublicKeyBytes)

	ikm := make([]byte, 32)
	_, err = io.ReadFull(hkdf.New(sha256.New, sharedSecret, k.authSecret, infoBuf.Bytes()), ikm)
	require.NoError(t, err)

	cek := make([]byte, 16)
	_, err = io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: aes128gcm\x00")), cek)
	require.NoError(t, err)

	nonce := make([]byte, 12)
	_, err = io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: nonce\x00")), nonce)
	require.NoError(t, err)

	block, err := aes.NewCipher(cek)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)

	padded, err := gcm.Open(nil, nonce, ciphertext, nil)
	require.NoError(t, err)

	// Padding per RFC8188: the message is followed by a 0x02 delimiter,
	// then zero padding out to the record length.
	padded = bytes.TrimRight(padded, "\x00")
	require.NotEmpty(t, padded)
	require.Equal(t, byte(0x02), padded[len(padded)-1], "missing padding delimiter")
	return padded[:len(padded)-1]
}

// webPushDeviceID builds the encoded bundle as the proxy receives it, with no
// "unified_push-v2:" prefix, since the server strips that before forwarding.
func webPushDeviceID(t *testing.T, endpoint, p256dh, auth string) string {
	b, err := json.Marshal(webPushBundle{Endpoint: endpoint, P256dh: p256dh, Auth: auth})
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestWebPushSendNotification_EncryptRoundTrip(t *testing.T) {
	keys := newRecipientKeys(t)

	var capturedBody []byte
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		capturedBody = body
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	host := strings.TrimPrefix(srv.URL, "https://")
	webPushSrv := NewWebPushNotificationServer(testWebPushSettingsAllowing(host), logger, nil, 5)
	trustTestServerCert(webPushSrv, srv)

	msg := &model.PushNotification{
		ServerId:     "server1",
		AckId:        "ack1",
		Type:         model.PushTypeMessage,
		Message:      "hello from the test",
		ChannelName:  "town-square",
		ChannelId:    "channel1",
		PostId:       "post1",
		Signature:    "sig1",
		SenderId:     "sender1",
		SenderName:   "Some User",
		TeamId:       "team1",
		Badge:        -1,
		IsCRTEnabled: true,
		DeviceId:     webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
	}

	resp := webPushSrv.SendNotification(1, msg)
	require.Equal(t, NewOkPushResponse(), resp)
	require.NotNil(t, capturedBody)

	plaintext := keys.decrypt(t, capturedBody)

	var got map[string]string
	require.NoError(t, json.Unmarshal(plaintext, &got))
	require.Equal(t, buildWebPushPayload(msg), got)
}

// A transport failure must not leak the endpoint: Go's *url.Error embeds the
// request URL, and that endpoint is a bearer credential.
func TestWebPushSendNotification_TransportErrorRedactsEndpoint(t *testing.T) {
	keys := newRecipientKeys(t)

	// Reserve a port and immediately free it, so SendNotification gets a
	// connection-refused url.Error without depending on what else is
	// running on the host.
	closedPort := unusedLoopbackPort(t)
	const secretTopic = "supersecret-bearer-topic"
	host := fmt.Sprintf("127.0.0.1:%d", closedPort)
	endpoint := "https://" + host + "/" + secretTopic

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	webPushSrv := NewWebPushNotificationServer(testWebPushSettingsAllowing(host), logger, nil, 5)
	msg := &model.PushNotification{
		ServerId: "server1",
		Type:     model.PushTypeMessage,
		DeviceId: webPushDeviceID(t, endpoint, keys.p256dh(), keys.auth()),
	}

	resp := webPushSrv.SendNotification(1, msg)
	require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
	require.NotContains(t, resp[PUSH_STATUS_ERROR_MSG], secretTopic)
	require.NotContains(t, resp[PUSH_STATUS_ERROR_MSG], endpoint)
}

func TestWebPushSendNotification_StatusMapping(t *testing.T) {
	keys := newRecipientKeys(t)

	for _, tc := range []struct {
		name       string
		statusCode int
		want       PushResponse
	}{
		{"200 OK maps to ok", http.StatusOK, NewOkPushResponse()},
		{"201 Created maps to ok", http.StatusCreated, NewOkPushResponse()},
		{"202 Accepted maps to ok", http.StatusAccepted, NewOkPushResponse()},
		{"410 Gone maps to remove", http.StatusGone, NewRemovePushResponse()},
		{"404 Not Found maps to remove", http.StatusNotFound, NewRemovePushResponse()},
		{"429 Too Many Requests maps to error", http.StatusTooManyRequests, nil},
		{"400 Bad Request maps to error", http.StatusBadRequest, nil},
		{"500 Internal Server Error maps to error", http.StatusInternalServerError, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			logger, err := mlog.NewLogger()
			require.NoError(t, err)

			host := strings.TrimPrefix(srv.URL, "https://")
			webPushSrv := NewWebPushNotificationServer(testWebPushSettingsAllowing(host), logger, nil, 5)
			trustTestServerCert(webPushSrv, srv)

			msg := &model.PushNotification{
				ServerId: "server1",
				Type:     model.PushTypeMessage,
				DeviceId: webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
			}

			resp := webPushSrv.SendNotification(1, msg)
			if tc.want == nil {
				require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
				return
			}
			require.Equal(t, tc.want, resp)
		})
	}
}

func TestWebPushSendNotification_CapsErrorBody(t *testing.T) {
	keys := newRecipientKeys(t)

	const oversizedBodyLen = 100 * 1024
	oversizedBody := strings.Repeat("x", oversizedBodyLen)

	for _, tc := range []struct {
		name              string
		maxErrorBodyBytes int
		wantLen           int
	}{
		{"default cap applies when unset", 0, defaultMaxErrorBodyBytes},
		{"configured cap is honored", 100, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(oversizedBody))
			}))
			defer srv.Close()

			logger, err := mlog.NewLogger()
			require.NoError(t, err)

			host := strings.TrimPrefix(srv.URL, "https://")
			settings := testWebPushSettingsAllowing(host)
			settings.MaxErrorBodyBytes = tc.maxErrorBodyBytes
			webPushSrv := NewWebPushNotificationServer(settings, logger, nil, 5)
			trustTestServerCert(webPushSrv, srv)

			msg := &model.PushNotification{
				ServerId: "server1",
				Type:     model.PushTypeMessage,
				DeviceId: webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
			}

			resp := webPushSrv.SendNotification(1, msg)
			require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
			require.Len(t, resp[PUSH_STATUS_ERROR_MSG], tc.wantLen)
		})
	}
}

func TestWebPushSendNotification_TTL(t *testing.T) {
	keys := newRecipientKeys(t)

	for _, tc := range []struct {
		name       string
		ttlSeconds int
		wantTTL    string
	}{
		{"default TTL applies when unset", 0, "30"},
		{"configured TTL is honored", 120, "120"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotTTL string
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotTTL = r.Header.Get("TTL")
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
			}))
			defer srv.Close()

			logger, err := mlog.NewLogger()
			require.NoError(t, err)

			host := strings.TrimPrefix(srv.URL, "https://")
			settings := testWebPushSettingsAllowing(host)
			settings.TTLSeconds = tc.ttlSeconds
			webPushSrv := NewWebPushNotificationServer(settings, logger, nil, 5)
			trustTestServerCert(webPushSrv, srv)

			msg := &model.PushNotification{
				ServerId: "server1",
				Type:     model.PushTypeMessage,
				DeviceId: webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
			}

			resp := webPushSrv.SendNotification(1, msg)
			require.Equal(t, NewOkPushResponse(), resp)
			require.Equal(t, tc.wantTTL, gotTTL)
		})
	}
}

func TestDecodeWebPushDeviceID(t *testing.T) {
	keys := newRecipientKeys(t)

	t.Run("valid bundle decodes", func(t *testing.T) {
		deviceID := webPushDeviceID(t, "https://ntfy.sh/sometopic", keys.p256dh(), keys.auth())
		sub, err := decodeWebPushDeviceID(deviceID)
		require.NoError(t, err)
		require.Equal(t, "https://ntfy.sh/sometopic", sub.Endpoint)
		require.Equal(t, keys.p256dh(), sub.Keys.P256dh)
		require.Equal(t, keys.auth(), sub.Keys.Auth)
	})

	t.Run("invalid base64 errors", func(t *testing.T) {
		_, err := decodeWebPushDeviceID("not-valid-base64!!!")
		require.Error(t, err)
	})

	t.Run("missing fields errors", func(t *testing.T) {
		b, err := json.Marshal(webPushBundle{Endpoint: "https://ntfy.sh/x"})
		require.NoError(t, err)
		deviceID := base64.RawURLEncoding.EncodeToString(b)
		_, err = decodeWebPushDeviceID(deviceID)
		require.Error(t, err)
	})
}

func TestWebPushInitialize(t *testing.T) {
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	newServer := func(settings WebPushSettings) *WebPushNotificationServer {
		return NewWebPushNotificationServer(settings, logger, nil, 5)
	}

	t.Run("valid keypair initializes", func(t *testing.T) {
		require.NoError(t, newServer(testWebPushSettings()).Initialize())
	})

	t.Run("missing keys fail fast", func(t *testing.T) {
		require.Error(t, newServer(WebPushSettings{Type: PushNotifyWebPush}).Initialize())
	})

	t.Run("malformed private key fails fast", func(t *testing.T) {
		s := testWebPushSettings()
		s.VAPIDPrivateKey = "not-base64!!!"
		require.Error(t, newServer(s).Initialize())
	})

	t.Run("mismatched keypair fails fast", func(t *testing.T) {
		// Public key from a different keypair than the private key.
		_, otherPub, err := webpush.GenerateVAPIDKeys()
		require.NoError(t, err)
		s := testWebPushSettings()
		s.VAPIDPublicKey = otherPub
		require.Error(t, newServer(s).Initialize())
	})

	t.Run("missing subscriber fails fast", func(t *testing.T) {
		s := testWebPushSettings()
		s.Subscriber = ""
		require.Error(t, newServer(s).Initialize())
	})

	t.Run("non-uri subscriber fails fast", func(t *testing.T) {
		s := testWebPushSettings()
		s.Subscriber = "admin@example.com" // missing mailto: scheme
		require.Error(t, newServer(s).Initialize())
	})

	t.Run("bad AllowedHosts entry fails fast", func(t *testing.T) {
		s := testWebPushSettings()
		s.AllowedHosts = []string{"https://ntfy.mydomain.com"}
		require.Error(t, newServer(s).Initialize())
	})

	t.Run("valid AllowedHosts entries initialize", func(t *testing.T) {
		s := testWebPushSettings()
		s.AllowedHosts = []string{"ntfy.mydomain.com", "192.168.1.123:8123"}
		require.NoError(t, newServer(s).Initialize())
	})

	t.Run("bad AdditionalBlockedCIDRs entry fails fast", func(t *testing.T) {
		s := testWebPushSettings()
		s.AdditionalBlockedCIDRs = []string{"not-a-cidr"}
		require.Error(t, newServer(s).Initialize())
	})

	t.Run("valid AdditionalBlockedCIDRs entries initialize", func(t *testing.T) {
		s := testWebPushSettings()
		s.AdditionalBlockedCIDRs = []string{"203.0.113.0/24"}
		require.NoError(t, newServer(s).Initialize())
	})

	t.Run("negative TTLSeconds fails fast", func(t *testing.T) {
		s := testWebPushSettings()
		s.TTLSeconds = -1
		require.Error(t, newServer(s).Initialize())
	})
}

// Exercises paranoidClient directly, to prove the dialer wiring itself works
// before relying on it through SendNotification.
func TestNewWebPushNotificationServer_ParanoidClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	webPushSrv := NewWebPushNotificationServer(testWebPushSettings(), logger, nil, 5)

	_, err = webPushSrv.paranoidClient.Get(srv.URL)
	require.Error(t, err)
	require.True(t, errors.Is(err, errDestinationBlocked), "got: %v", err)
}

func TestWebPushSendNotification_RequiresHTTPSAlways(t *testing.T) {
	keys := newRecipientKeys(t)

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	t.Run("http to non-allowlisted host fails before any dial", func(t *testing.T) {
		webPushSrv := NewWebPushNotificationServer(testWebPushSettingsAllowing("not-this-host:9999"), logger, nil, 5)
		msg := &model.PushNotification{
			ServerId: "server1",
			Type:     model.PushTypeMessage,
			DeviceId: webPushDeviceID(t, "http://127.0.0.1:1/topic", keys.p256dh(), keys.auth()),
		}

		resp := webPushSrv.SendNotification(1, msg)
		require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
		require.Contains(t, resp[PUSH_STATUS_ERROR_MSG], "must use https")
	})

	t.Run("http to allowlisted host still fails: AllowedHosts only exempts the IP check, not the scheme", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		host := strings.TrimPrefix(srv.URL, "http://")
		webPushSrv := NewWebPushNotificationServer(testWebPushSettingsAllowing(host), logger, nil, 5)
		msg := &model.PushNotification{
			ServerId: "server1",
			Type:     model.PushTypeMessage,
			DeviceId: webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
		}

		resp := webPushSrv.SendNotification(1, msg)
		require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
		require.Contains(t, resp[PUSH_STATUS_ERROR_MSG], "must use https")
	})

	t.Run("https to non-allowlisted host passes the scheme check and is blocked by the IP guard instead", func(t *testing.T) {
		webPushSrv := NewWebPushNotificationServer(testWebPushSettings(), logger, nil, 5)
		msg := &model.PushNotification{
			ServerId: "server1",
			Type:     model.PushTypeMessage,
			DeviceId: webPushDeviceID(t, "https://127.0.0.1:1/topic", keys.p256dh(), keys.auth()),
		}

		resp := webPushSrv.SendNotification(1, msg)
		require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
		require.NotContains(t, resp[PUSH_STATUS_ERROR_MSG], "must use https")
	})
}

func TestWebPushSendNotification_AllowedHostBypassesIPCheck(t *testing.T) {
	keys := newRecipientKeys(t)

	var hit bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	host := strings.TrimPrefix(srv.URL, "https://")
	webPushSrv := NewWebPushNotificationServer(testWebPushSettingsAllowing(host), logger, nil, 5)
	trustTestServerCert(webPushSrv, srv)

	msg := &model.PushNotification{
		ServerId: "server1",
		Type:     model.PushTypeMessage,
		DeviceId: webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
	}

	resp := webPushSrv.SendNotification(1, msg)
	require.Equal(t, NewOkPushResponse(), resp)
	require.True(t, hit, "request never reached the relay")
}

func TestWebPushSendNotification_BlocksPrivateIP(t *testing.T) {
	keys := newRecipientKeys(t)
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	t.Run("127.0.0.1 (loopback): blocked before any connection", func(t *testing.T) {
		var hit bool
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hit = true
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		webPushSrv := NewWebPushNotificationServer(testWebPushSettings(), logger, nil, 5)
		msg := &model.PushNotification{
			ServerId: "server1",
			Type:     model.PushTypeMessage,
			DeviceId: webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
		}

		resp := webPushSrv.SendNotification(1, msg)
		require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
		require.Equal(t, "destination not permitted", resp[PUSH_STATUS_ERROR_MSG])
		require.False(t, hit, "request reached the listener despite being blocked")
	})

	for _, host := range []string{
		"169.254.169.254",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"::1",
		"fe80::1",
		"fc00::1",
	} {
		t.Run(host, func(t *testing.T) {
			endpointHost := host
			if strings.Contains(host, ":") {
				endpointHost = "[" + host + "]"
			}
			webPushSrv := NewWebPushNotificationServer(testWebPushSettings(), logger, nil, 5)
			msg := &model.PushNotification{
				ServerId: "server1",
				Type:     model.PushTypeMessage,
				DeviceId: webPushDeviceID(t, "https://"+endpointHost+"/topic", keys.p256dh(), keys.auth()),
			}

			resp := webPushSrv.SendNotification(1, msg)
			require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
			require.Equal(t, "destination not permitted", resp[PUSH_STATUS_ERROR_MSG])
		})
	}
}

// RFC 6598 CGNAT addresses (100.64.0.0/10) aren't in paranoidhttp's own
// denylist; baseForbiddenCIDRs adds them so paranoidhttp's dialer blocks them
// the same way it blocks everything else. This literal-IP endpoint is
// rejected pre-dial, so the test doesn't depend on network reachability.
func TestWebPushSendNotification_BlocksCGNAT(t *testing.T) {
	keys := newRecipientKeys(t)
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	webPushSrv := NewWebPushNotificationServer(testWebPushSettings(), logger, nil, 2)
	msg := &model.PushNotification{
		ServerId: "server1",
		Type:     model.PushTypeMessage,
		DeviceId: webPushDeviceID(t, "https://100.64.0.1/topic", keys.p256dh(), keys.auth()),
	}

	resp := webPushSrv.SendNotification(1, msg)
	require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
	require.Equal(t, "destination not permitted", resp[PUSH_STATUS_ERROR_MSG])
}

// 203.0.113.0/24 (TEST-NET-3) is otherwise-routable space, not in
// baseForbiddenCIDRs, so this only blocks because AdditionalBlockedCIDRs is
// merged into the SSRF guard's denylist at construction time.
func TestWebPushSendNotification_BlocksAdditionalConfiguredCIDR(t *testing.T) {
	keys := newRecipientKeys(t)
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	s := testWebPushSettings()
	s.AdditionalBlockedCIDRs = []string{"203.0.113.0/24"}
	webPushSrv := NewWebPushNotificationServer(s, logger, nil, 2)
	msg := &model.PushNotification{
		ServerId: "server1",
		Type:     model.PushTypeMessage,
		DeviceId: webPushDeviceID(t, "https://203.0.113.1/topic", keys.p256dh(), keys.auth()),
	}

	resp := webPushSrv.SendNotification(1, msg)
	require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
	require.Equal(t, "destination not permitted", resp[PUSH_STATUS_ERROR_MSG])
}

// paranoidhttp blocks the literal hostname "localhost" via a separate code
// path from the IP-range check above; this confirms wrapBlockedIPSentinel
// catches both.
func TestWebPushSendNotification_BlocksLocalhostHostname(t *testing.T) {
	keys := newRecipientKeys(t)
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	webPushSrv := NewWebPushNotificationServer(testWebPushSettings(), logger, nil, 5)
	msg := &model.PushNotification{
		ServerId: "server1",
		Type:     model.PushTypeMessage,
		DeviceId: webPushDeviceID(t, "https://localhost/topic", keys.p256dh(), keys.auth()),
	}

	resp := webPushSrv.SendNotification(1, msg)
	require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
	require.Equal(t, "destination not permitted", resp[PUSH_STATUS_ERROR_MSG])
}

func TestWebPushSendNotification_DeniesRedirect(t *testing.T) {
	keys := newRecipientKeys(t)
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	var targetCalled atomic.Bool
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalled.Store(true)
		w.WriteHeader(http.StatusCreated)
	}))
	defer target.Close()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	// The original host is allowlisted (normalClient, no IP-range check) —
	// proving redirect denial applies unconditionally, not just to
	// paranoidClient.
	host := strings.TrimPrefix(srv.URL, "https://")
	webPushSrv := NewWebPushNotificationServer(testWebPushSettingsAllowing(host), logger, nil, 5)
	// Trust target's cert too, not just srv's: otherwise a regression that
	// follows the redirect would still fail (and the test would still
	// pass) on an unrelated TLS verification error against target.
	trustTestServerCert(webPushSrv, srv, target)
	msg := &model.PushNotification{
		ServerId: "server1",
		Type:     model.PushTypeMessage,
		DeviceId: webPushDeviceID(t, srv.URL, keys.p256dh(), keys.auth()),
	}

	resp := webPushSrv.SendNotification(1, msg)
	require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
	require.False(t, targetCalled.Load(), "redirect target was reached; CheckRedirect should have blocked the follow-up request")
}

// IPv4-mapped IPv6 literals are a common SSRF-filter bypass; paranoidhttp
// handles them for free since net.IP.To4()/net.IPNet.Contains() unwrap a
// ::ffff:0:0/96-mapped address before comparing.
func TestWebPushSendNotification_BlocksIPv4MappedIPv6(t *testing.T) {
	keys := newRecipientKeys(t)
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	for _, host := range []string{
		"[::ffff:127.0.0.1]",
		"[::ffff:169.254.169.254]",
	} {
		t.Run(host, func(t *testing.T) {
			webPushSrv := NewWebPushNotificationServer(testWebPushSettings(), logger, nil, 5)
			msg := &model.PushNotification{
				ServerId: "server1",
				Type:     model.PushTypeMessage,
				DeviceId: webPushDeviceID(t, "https://"+host+"/topic", keys.p256dh(), keys.auth()),
			}

			resp := webPushSrv.SendNotification(1, msg)
			require.Equal(t, PUSH_STATUS_FAIL, resp[PUSH_STATUS])
			require.Equal(t, "destination not permitted", resp[PUSH_STATUS_ERROR_MSG])
		})
	}
}

func TestValidateAllowedHosts(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		wantErr bool
	}{
		{"empty", nil, false},
		{"bare hostname", []string{"ntfy.mydomain.com"}, false},
		{"hostname with port", []string{"192.168.1.123:8123"}, false},
		{"scheme present", []string{"https://ntfy.mydomain.com"}, true},
		{"path present", []string{"ntfy.mydomain.com/topic"}, true},
		{"glob", []string{"ntfy.mydomain.com*"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAllowedHosts(tt.hosts)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
