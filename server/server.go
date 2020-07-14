// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"fmt"
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

// Server is the main struct which performs all activities.
type Server struct {
	cfg         *ConfigPushProxy
	httpServer  *http.Server
	pushTargets map[string]NotificationServer
	logger      *Logger
}

// New returns a new Server instance.
func New(cfg *ConfigPushProxy, logger *Logger) *Server {
	return &Server{
		cfg:         cfg,
		pushTargets: make(map[string]NotificationServer),
		logger:      logger,
	}
}

// Start starts the server.
func (s *Server) Start() {
	s.logger.Infof("Push proxy server is initializing. BuildNumber: %s, BuildDate: %s, BuildHash: %s", BuildNumber, BuildDate, BuildHash)

	proxyServer := getProxyServer()
	if proxyServer != "" {
		s.logger.Infof("Proxy server detected. Routing all requests through: %s", proxyServer)
	}

	for _, settings := range s.cfg.ApplePushSettings {
		server := NewAppleNotificationServer(settings, s.logger)
		if server.Initialize() {
			s.pushTargets[settings.Type] = server
		}
	}

	for _, settings := range s.cfg.AndroidPushSettings {
		server := NewAndroidNotificationServer(settings, s.logger)
		if server.Initialize() {
			s.pushTargets[settings.Type] = server
		}
	}

	router := mux.NewRouter()
	vary := throttled.VaryBy{}
	vary.RemoteAddr = false
	vary.Headers = strings.Fields(s.cfg.ThrottleVaryByHeader)
	th := throttled.RateLimit(throttled.PerSec(s.cfg.ThrottlePerSec), &vary, throttledStore.NewMemStore(s.cfg.ThrottleMemoryStoreSize))

	th.DeniedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Errorf("%v: code=429 ip=%v", r.URL.Path, s.getIpAddress(r))
		throttled.DefaultDeniedHandler.ServeHTTP(w, r)
	})

	handler := th.Throttle(router)

	router.HandleFunc("/", root).Methods("GET")

	metricCompatibleSendNotificationHandler := s.handleSendNotification
	metricCompatibleAckNotificationHandler := s.handleAckNotification
	if s.cfg.EnableMetrics {
		MetricsEnabled = true
		metrics := NewPrometheusHandler()
		router.Handle("/metrics", metrics).Methods("GET")
		metricCompatibleSendNotificationHandler = responseTimeMiddleware(s.handleSendNotification)
		metricCompatibleAckNotificationHandler = responseTimeMiddleware(s.handleAckNotification)
	}
	r := router.PathPrefix("/api/v1").Subrouter()
	r.HandleFunc("/send_push", metricCompatibleSendNotificationHandler).Methods("POST")
	r.HandleFunc("/ack", metricCompatibleAckNotificationHandler).Methods("POST")

	s.httpServer = &http.Server{
		Addr:         s.cfg.ListenAddress,
		Handler:      handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))(handler),
		ReadTimeout:  time.Duration(CONNECTION_TIMEOUT_SECONDS) * time.Second,
		WriteTimeout: time.Duration(CONNECTION_TIMEOUT_SECONDS) * time.Second,
	}
	go func() {
		err := s.httpServer.ListenAndServe()
		if err != http.ErrServerClosed {
			s.logger.Panic(err.Error())
		}
	}()

	s.logger.Info("Server is listening on " + s.cfg.ListenAddress)
}

// Stop stops the server.
func (s *Server) Stop() {
	s.logger.Info("Stopping Server...")
	ctx, cancel := context.WithTimeout(context.Background(), WAIT_FOR_SERVER_SHUTDOWN)
	defer cancel()
	// Close shop
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		s.logger.Error(err.Error())
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

func (s *Server) handleSendNotification(w http.ResponseWriter, r *http.Request) {
	msg := PushNotificationFromJson(r.Body)

	if msg == nil {
		rMsg := "Failed to read message body"
		s.logger.Error(rMsg)
		resp := NewErrorPushResponse(rMsg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}

	if msg.ServerID == "" {
		rMsg := "Failed because of missing server Id"
		s.logger.Error(rMsg)
		resp := NewErrorPushResponse(rMsg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}

	if msg.DeviceID == "" {
		rMsg := fmt.Sprintf("Failed because of missing device Id serverId=%v", msg.ServerID)
		s.logger.Error(rMsg)
		resp := NewErrorPushResponse(rMsg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}

	if len(msg.Message) > 2047 {
		msg.Message = msg.Message[0:2046]
	}

	if server, ok := s.pushTargets[msg.Platform]; ok {
		rMsg := server.SendNotification(msg)
		_, _ = w.Write([]byte(rMsg.ToJson()))
		return
	} else {
		rMsg := fmt.Sprintf("Did not send message because of missing platform property type=%v serverId=%v", msg.Platform, msg.ServerID)
		s.logger.Error(rMsg)
		resp := NewErrorPushResponse(rMsg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}
}

func (s *Server) handleAckNotification(w http.ResponseWriter, r *http.Request) {
	ack := PushNotificationAckFromJSON(r.Body)

	if ack == nil {
		msg := "Failed to read ack body"
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}

	if ack.ID == "" {
		msg := "Failed because of missing ack Id"
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}

	if ack.Platform == "" {
		msg := "Failed because of missing ack platform"
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}

	if ack.Type == "" {
		msg := "Failed because of missing ack type"
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		_, _ = w.Write([]byte(resp.ToJson()))
		incrementBadRequest()
		return
	}

	// Increment ACK
	s.logger.Infof("Acknowledge delivery receipt for AckId=%v", ack.ID)
	incrementDelivered(ack.Platform, ack.Type)

	rMsg := NewOkPushResponse()
	_, _ = w.Write([]byte(rMsg.ToJson()))
}

func (s *Server) getIpAddress(r *http.Request) string {
	address := r.Header.Get(HEADER_FORWARDED)
	var err error

	if address == "" {
		address = r.Header.Get(HEADER_REAL_IP)
	}

	if address == "" {
		address, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			s.logger.Errorf("error in getting IP address: %v", err)
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
