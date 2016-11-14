// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/alexjlockwood/gcm"
	"github.com/braintree/manners"
	"github.com/gorilla/mux"
	"github.com/kyokomi/emoji"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"gopkg.in/throttled/throttled.v1"
	throttledStore "gopkg.in/throttled/throttled.v1/store"
)

const (
	HEADER_FORWARDED = "X-Forwarded-For"
	HEADER_REAL_IP   = "X-Real-IP"
)

var appleClient *apns.Client

func Start() {
	LogInfo("Push proxy server is initializing...")

	if len(CfgPP.ApplePushCertPrivate) > 0 {
		appleCert, appleCertErr := certificate.FromPemFile(CfgPP.ApplePushCertPrivate, CfgPP.ApplePushCertPassword)
		if appleCertErr != nil {
			LogCritical(fmt.Sprintf("Failed to load the apple pem cert err=%v", appleCertErr))
		}

		if CfgPP.ApplePushUseDevelopment {
			appleClient = apns.NewClient(appleCert).Development()
		} else {
			appleClient = apns.NewClient(appleCert).Production()
		}
	} else {
		LogError("Apple push notifications not configured.  Mssing ApplePushCertPrivate.")
	}

	if len(CfgPP.AndroidApiKey) == 0 {
		LogError("Android push notifications not configured.  Mssing AndroidApiKey.")
	}

	router := mux.NewRouter()
	var handler http.Handler = router
	vary := throttled.VaryBy{}
	vary.RemoteAddr = false
	vary.Headers = strings.Fields(CfgPP.ThrottleVaryByHeader)
	th := throttled.RateLimit(throttled.PerSec(CfgPP.ThrottlePerSec), &vary, throttledStore.NewMemStore(CfgPP.ThrottleMemoryStoreSize))

	th.DeniedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		LogError(fmt.Sprintf("%v: code=429 ip=%v", r.URL.Path, GetIpAddress(r)))
		throttled.DefaultDeniedHandler.ServeHTTP(w, r)
	})

	handler = th.Throttle(router)

	router.HandleFunc("/", root).Methods("GET")

	r := router.PathPrefix("/api/v1").Subrouter()
	r.HandleFunc("/send_push", handleSendNotification).Methods("POST")

	go func() {
		err := manners.ListenAndServe(CfgPP.ListenAddress, handler)
		if err != nil {
			LogCritical(err.Error())
		}
	}()

	LogInfo("Server is listening on " + CfgPP.ListenAddress)
}

func Stop() {
	LogInfo("Stopping Server...")
	manners.Close()
}

func root(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<html><body>Mattermost Push Proxy</body></html>"))
}

func handleSendNotification(w http.ResponseWriter, r *http.Request) {
	msg := PushNotificationFromJson(r.Body)

	if msg == nil {
		LogError("Failed to read message body")
	}

	if len(msg.ServerId) == 0 {
		LogError("Failed because of missing server Id")
	}

	if len(msg.Message) > 2047 {
		msg.Message = msg.Message[0:2046]
	}

	if msg.Platform == PUSH_NOTIFY_APPLE {
		go sendAppleNotification(msg)
	} else if msg.Platform == PUSH_NOTIFY_ANDROID {
		go sendAndroidNotification(msg)
	} else {
		LogError("Missing platform property")
	}
}

func sendAndroidNotification(msg *PushNotification) {
	var data map[string]interface{}
	if msg.Type == PUSH_TYPE_CLEAR {
		data = map[string]interface{}{"type": PUSH_TYPE_CLEAR, "channel_id": msg.ChannelId}
	} else {
		data = map[string]interface{}{"type": PUSH_TYPE_MESSAGE, "message": emoji.Sprint(msg.Message), "channel_id": msg.ChannelId, "channel_name": msg.ChannelName}
	}

	regIDs := []string{msg.DeviceId}
	gcmMsg := gcm.NewMessage(data, regIDs...)

	sender := &gcm.Sender{ApiKey: CfgPP.AndroidApiKey}

	if len(CfgPP.AndroidApiKey) > 0 {
		LogInfo("Sending android push notification")
		resp, err := sender.Send(gcmMsg, 2)

		if err != nil {
			LogError(fmt.Sprintf("Failed to send GCM push sid=%v did=%v err=%v", msg.ServerId, msg.DeviceId, err))
			return
		}

		if resp.Failure > 0 {
			LogError(fmt.Sprintf("Android response failure: %v", resp))
		}
	}
}

func sendAppleNotification(msg *PushNotification) {

	notification := &apns.Notification{}
	notification.DeviceToken = msg.DeviceId
	payload := payload.NewPayload()
	notification.Payload = payload
	notification.Topic = CfgPP.ApplePushTopic
	payload.Badge(msg.Badge)

	if msg.Type != PUSH_TYPE_CLEAR {
		payload.Alert(emoji.Sprint(msg.Message))
		payload.Category(msg.Category)
		payload.Sound("default")
	}

	if len(msg.ChannelId) > 0 {
		payload.Custom("channel_id", msg.ChannelId)
	}

	if len(msg.ChannelName) > 0 {
		payload.Custom("channel_name", msg.ChannelName)
	}

	if appleClient != nil {
		LogInfo("Sending apple push notification")
		res, err := appleClient.Push(notification)
		if err != nil {
			LogError(fmt.Sprintf("Failed to send apple push sid=%v did=%v err=%v", msg.ServerId, msg.DeviceId, err))
		}

		if !res.Sent() {
			LogError(fmt.Sprintf("Failed to send apple push with res ApnsID=%v reason=%v code=%v", res.ApnsID, res.Reason, res.StatusCode))
		}
	}

	// payload := apns.NewPayload()

	// if msg.Type != PUSH_TYPE_CLEAR {
	// 	payload.Alert = emoji.Sprint(msg.Message)
	// 	payload.Badge = msg.Badge
	// 	payload.Category = msg.Category
	// 	payload.Sound = "default"
	// }

	// payload.Badge = msg.Badge

	// pn := apns.NewPushNotification()
	// pn.DeviceToken = msg.DeviceId
	// pn.AddPayload(payload)

	// if len(msg.ChannelId) > 0 {
	// 	pn.Set("channel_id", msg.ChannelId)
	// }

	// if len(msg.ChannelName) > 0 {
	// 	pn.Set("channel_name", msg.ChannelName)
	// }

	// client := apns.NewClient(CfgPP.ApplePushServer, CfgPP.ApplePushCertPublic, CfgPP.ApplePushCertPrivate)

	// LogInfo("Sending apple push notification")
	// resp := client.Send(pn)

	// if resp.Error != nil {
	// 	LogError(fmt.Sprintf("Failed to send apple push sid=%v did=%v err=%v", msg.ServerId, msg.DeviceId, resp.Error))
	// }
}

func LogInfo(msg string) {
	Log("INFO", msg)
}

func LogError(msg string) {
	Log("ERROR", msg)
}

func LogCritical(msg string) {
	Log("CRIT", msg)
	panic(msg)
}

func Log(level string, msg string) {
	log.Printf("%v %v\n", level, msg)
}

func GetIpAddress(r *http.Request) string {
	address := r.Header.Get(HEADER_FORWARDED)

	if len(address) == 0 {
		address = r.Header.Get(HEADER_REAL_IP)
	}

	if len(address) == 0 {
		address, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	return address
}
