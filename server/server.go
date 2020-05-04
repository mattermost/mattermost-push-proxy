// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"gopkg.in/throttled/throttled.v1"
	throttledStore "gopkg.in/throttled/throttled.v1/store"
)

const (
	HEADER_FORWARDED           = "X-Forwarded-For"
	HEADER_REAL_IP             = "X-Real-IP"
	WAIT_FOR_SERVER_SHUTDOWN   = time.Second * 5
	CONNECTION_TIMEOUT_SECONDS = 60
)

var (
	BuildNumber string
	BuildDate   string
	BuildHash   string
)

type NotificationServer interface {
	SendNotification(msg *PushNotification) PushResponse
	Initialize() bool
}

var servers map[string]NotificationServer = make(map[string]NotificationServer)

var server *http.Server

func Start() {
	LogInfo(fmt.Sprintf("Push proxy server is initializing. BuildNumber: %s, BuildDate: %s, BuildHash: %s", BuildNumber, BuildDate, BuildHash))

	proxyServer := getProxyServer()
	if proxyServer != "" {
		LogInfo(fmt.Sprintf("Proxy server detected. Routing all requests through: %s", proxyServer))
	}

	for _, settings := range CfgPP.ApplePushSettings {
		server := NewAppleNotificationServer(settings)
		if server.Initialize() {
			servers[settings.Type] = server
		}
	}

	for _, settings := range CfgPP.AndroidPushSettings {
		server := NewAndroideNotificationServer(settings)
		if server.Initialize() {
			servers[settings.Type] = server
		}
	}

	router := mux.NewRouter()
	vary := throttled.VaryBy{}
	vary.RemoteAddr = false
	vary.Headers = strings.Fields(CfgPP.ThrottleVaryByHeader)
	th := throttled.RateLimit(throttled.PerSec(CfgPP.ThrottlePerSec), &vary, throttledStore.NewMemStore(CfgPP.ThrottleMemoryStoreSize))

	th.DeniedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		LogError(fmt.Sprintf("%v: code=429 ip=%v", r.URL.Path, GetIpAddress(r)))
		throttled.DefaultDeniedHandler.ServeHTTP(w, r)
	})

	handler := th.Throttle(router)

	router.HandleFunc("/", root).Methods("GET")

	metricCompatibleSendNotificationHandler := handleSendNotification
	metricCompatibleAckNotificationHandler := handleAckNotification
	if CfgPP.EnableMetrics {
		MetricsEnabled = true
		metrics := NewPrometheusHandler()
		router.Handle("/metrics", metrics).Methods("GET")
		metricCompatibleSendNotificationHandler = responseTimeMiddleware(handleSendNotification)
		metricCompatibleAckNotificationHandler = responseTimeMiddleware(handleAckNotification)
	}
	r := router.PathPrefix("/api/v1").Subrouter()
	r.HandleFunc("/send_push", metricCompatibleSendNotificationHandler).Methods("POST")
	r.HandleFunc("/ack", metricCompatibleAckNotificationHandler).Methods("POST")

	server = &http.Server{
		Addr:         CfgPP.ListenAddress,
		Handler:      handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))(handler),
		ReadTimeout:  time.Duration(CONNECTION_TIMEOUT_SECONDS) * time.Second,
		WriteTimeout: time.Duration(CONNECTION_TIMEOUT_SECONDS) * time.Second,
	}
	go func() {
		err := server.ListenAndServe()
		if err != http.ErrServerClosed {
			LogCritical(err.Error())
		}
	}()

	LogInfo("Server is listening on " + CfgPP.ListenAddress)
}

func Stop() {
	LogInfo("Stopping Server...")
	ctx, cancel := context.WithTimeout(context.Background(), WAIT_FOR_SERVER_SHUTDOWN)
	defer cancel()
	// Close shop
	err := server.Shutdown(ctx)
	if err != nil {
		LogError(err.Error())
	}
}

func root(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("<html><body>Mattermost Push Proxy</body></html>"))
}

func responseTimeMiddleware(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		f(w, r)
		observeServiceResponse(time.Since(start).Seconds())
	}
}

func handleSendNotification(w http.ResponseWriter, r *http.Request) {
	msg := PushNotificationFromJson(r.Body)

	if msg == nil {
		rMsg := LogError("Failed to read message body")
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}

	if len(msg.ServerID) == 0 {
		rMsg := LogError("Failed because of missing server Id")
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}

	if len(msg.DeviceID) == 0 {
		rMsg := LogError(fmt.Sprintf("Failed because of missing device Id serverId=%v", msg.ServerID))
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}

	if len(msg.Message) > 2047 {
		msg.Message = msg.Message[0:2046]
	}

	if server, ok := servers[msg.Platform]; ok {
		rMsg := server.SendNotification(msg)
		_, _ = w.Write([]byte(rMsg.ToJson()))
		return
	} else {
		rMsg := LogError(fmt.Sprintf("Did not send message because of missing platform property type=%v serverId=%v", msg.Platform, msg.ServerID))
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}
}

func handleAckNotification(w http.ResponseWriter, r *http.Request) {
	ack := PushNotificationAckFromJSON(r.Body)

	if ack == nil {
		rMsg := LogError("Failed to read ack body")
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}

	if len(ack.ID) == 0 {
		rMsg := LogError("Failed because of missing ack Id")
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}

	if len(ack.Platform) == 0 {
		rMsg := LogError("Failed because of missing ack platform")
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}

	if len(ack.Type) == 0 {
		rMsg := LogError("Failed because of missing ack type")
		_, _ = w.Write([]byte(rMsg.ToJson()))
		incrementBadRequest()
		return
	}

	// Increment ACK
	LogInfo(fmt.Sprintf("Acknowledge delivery receipt for AckId=%v", ack.ID))
	incrementDelivered(ack.Platform, ack.Type)

	rMsg := NewOkPushResponse()
	_, _ = w.Write([]byte(rMsg.ToJson()))
}

func LogInfo(msg string) {
	Log("INFO", msg)
}

func LogError(msg string) PushResponse {
	Log("ERROR", msg)
	return NewErrorPushResponse(msg)
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
	var err error

	if len(address) == 0 {
		address = r.Header.Get(HEADER_REAL_IP)
	}

	if len(address) == 0 {
		address, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			LogError(fmt.Sprintf("error in getting IP address: %v", err))
		}
	}

	return address
}

func getProxyServer() string {
	// HTTPS_PROXY gets the higher priority.
	proxyServer := os.Getenv("HTTPS_PROXY")
	if proxyServer == "" {
		proxyServer = os.Getenv("HTTP_PROXY")
	}
	return proxyServer
}
