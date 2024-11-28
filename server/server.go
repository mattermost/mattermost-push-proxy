// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"gopkg.in/throttled/throttled.v1"
	throttledStore "gopkg.in/throttled/throttled.v1/store"

	"github.com/mattermost/mattermost-push-proxy/internal/version"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	HEADER_FORWARDED           = "X-Forwarded-For"
	HEADER_REAL_IP             = "X-Real-IP"
	WAIT_FOR_SERVER_SHUTDOWN   = time.Second * 5
	CONNECTION_TIMEOUT_SECONDS = 60
	MAX_RETRIES                = 3
)

type NotificationServer interface {
	SendNotification(msg *PushNotification) PushResponse
	Initialize() error
}

// Server is the main struct which performs all activities.
type Server struct {
	cfg         *ConfigPushProxy
	httpServer  *http.Server
	pushTargets map[string]NotificationServer
	metrics     *metrics
	logger      *Logger
}

// New returns a new Server instance.
func New(cfg *ConfigPushProxy, logger *mlog.Logger) *Server {
	return &Server{
		cfg:         cfg,
		pushTargets: make(map[string]NotificationServer),
		logger:      logger,
	}
}

// Start starts the server.
func (s *Server) Start() {
	v := version.VersionInfo()
	s.logger.Info("Push proxy server is initializing...", mlog.String("version", v.String()))

	proxyServer := getProxyServer()
	if proxyServer != "" {
		s.logger.Info("Proxy server detected.", mlog.String("proxyServer", proxyServer))
	}

	var m *metrics
	if s.cfg.EnableMetrics {
		m = newMetrics()
		s.metrics = m
	}

	for _, settings := range s.cfg.ApplePushSettings {
		server := NewAppleNotificationServer(settings, s.logger, m, s.cfg.SendTimeoutSec, s.cfg.RetryTimeoutSec)
		err := server.Initialize()
		if err != nil {
			s.logger.Error("Failed to initialize client", mlog.Err(err))
			continue
		}
		s.pushTargets[settings.Type] = server
	}

	for _, settings := range s.cfg.AndroidPushSettings {
		server := NewAndroidNotificationServer(settings, s.logger, m, s.cfg.SendTimeoutSec)
		err := server.Initialize()
		if err != nil {
			s.logger.Error("Failed to initialize client", mlog.Err(err))
			continue
		}
		s.pushTargets[settings.Type] = server
	}

	router := mux.NewRouter()
	vary := throttled.VaryBy{}
	vary.RemoteAddr = false
	vary.Headers = strings.Fields(s.cfg.ThrottleVaryByHeader)
	th := throttled.RateLimit(throttled.PerSec(s.cfg.ThrottlePerSec), &vary, throttledStore.NewMemStore(s.cfg.ThrottleMemoryStoreSize))

	th.DeniedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Error("Error: code=429", mlog.String("path", r.URL.Path), mlog.String("ip", s.getIpAddress(r)))
		throttled.DefaultDeniedHandler.ServeHTTP(w, r)
	})

	handler := th.Throttle(router)

	router.HandleFunc("/", root).Methods("GET")
	router.HandleFunc("/version", s.version).Methods("GET")

	metricCompatibleSendNotificationHandler := s.handleSendNotification
	metricCompatibleAckNotificationHandler := s.handleAckNotification
	if s.cfg.EnableMetrics {
		metrics := NewPrometheusHandler()
		router.Handle("/metrics", metrics).Methods("GET")
		metricCompatibleSendNotificationHandler = s.responseTimeMiddleware(s.handleSendNotification)
		metricCompatibleAckNotificationHandler = s.responseTimeMiddleware(s.handleAckNotification)
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
			s.logger.Fatal(err.Error())
		}
	}()

	s.logger.Info("Server is listening on " + s.cfg.ListenAddress)
}

// Stop stops the server.
func (s *Server) Stop() {
	s.logger.Info("Stopping Server...")
	ctx, cancel := context.WithTimeout(context.Background(), WAIT_FOR_SERVER_SHUTDOWN)
	defer cancel()
	if s.metrics != nil {
		s.metrics.shutdown()
	}
	// Close shop
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		s.logger.Error(err.Error())
	}
}

func root(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("<html><body>Mattermost Push Proxy</body></html>"))
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	info := version.VersionInfo()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		s.logger.Error("Failed to write response", mlog.Err(err))
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
	}
}

func (s *Server) responseTimeMiddleware(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		f(w, r)
		if s.metrics != nil {
			s.metrics.observeServiceResponse(time.Since(start).Seconds())
		}
	}
}

func (s *Server) handleSendNotification(w http.ResponseWriter, r *http.Request) {
	var msg PushNotification
	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		rMsg := fmt.Sprintf("Failed to read message body: %v", err)
		s.logger.Error(rMsg)
		resp := NewErrorPushResponse(rMsg)
		if err2 := json.NewEncoder(w).Encode(resp); err2 != nil {
			s.logger.Error("Failed to write response", mlog.Err(err2))
		}
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
		return
	}

	if msg.ServerID == "" {
		rMsg := "Failed because of missing server Id"
		s.logger.Error(rMsg)
		resp := NewErrorPushResponse(rMsg)
		if err2 := json.NewEncoder(w).Encode(resp); err2 != nil {
			s.logger.Error("Failed to write response", mlog.Err(err2))
		}
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
		return
	}

	if msg.DeviceID == "" {
		rMsg := fmt.Sprintf("Failed because of missing device Id serverId=%v", msg.ServerID)
		s.logger.Error(rMsg)
		resp := NewErrorPushResponse(rMsg)
		if err2 := json.NewEncoder(w).Encode(resp); err2 != nil {
			s.logger.Error("Failed to write response", mlog.Err(err2))
		}
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
		return
	}

	if len(msg.Message) > 2047 {
		msg.Message = msg.Message[0:2046]
	}

	if len(msg.ChannelName) > 64 {
		msg.ChannelName = msg.ChannelName[0:64]
	}

	// Parse the app version if available
	index := strings.Index(msg.Platform, "-v")
	platform := msg.Platform
	msg.AppVersion = 1
	if index > -1 {
		msg.Platform = platform[:index]
		appVersionString := platform[index+2:]
		version, e := strconv.Atoi(appVersionString)
		if e == nil {
			msg.AppVersion = version
		} else {
			rMsg := fmt.Sprintf("Could not determine the app version in %v appVersion=%v", msg.Platform, appVersionString)
			s.logger.Error(rMsg)
		}
	}

	if server, ok := s.pushTargets[msg.Platform]; ok {
		rMsg := server.SendNotification(&msg)
		if err2 := json.NewEncoder(w).Encode(rMsg); err2 != nil {
			s.logger.Error("Failed to write message", mlog.Err(err2))
		}
		return
	}
	rMsg := fmt.Sprintf("Did not send message because of missing platform property type=%v serverId=%v", msg.Platform, msg.ServerID)
	s.logger.Error(rMsg)
	resp := NewErrorPushResponse(rMsg)
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		s.logger.Error("Failed to write response", mlog.Err(err))
	}
	if s.metrics != nil {
		s.metrics.incrementBadRequest()
	}
}

func (s *Server) handleAckNotification(w http.ResponseWriter, r *http.Request) {
	var ack PushNotificationAck
	err := json.NewDecoder(r.Body).Decode(&ack)
	if err != nil {
		msg := fmt.Sprintf("Failed to read ack body: %v", err)
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		if err2 := json.NewEncoder(w).Encode(resp); err2 != nil {
			s.logger.Error("Failed to write response", mlog.Err(err2))
		}
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
		return
	}

	if ack.ID == "" {
		msg := "Failed because of missing ack Id"
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			s.logger.Error("Failed to write response", mlog.Err(err))
		}
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
		return
	}

	if ack.Platform == "" {
		msg := "Failed because of missing ack platform"
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			s.logger.Error("Failed to write response", mlog.Err(err))
		}
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
		return
	}

	if ack.Type == "" {
		msg := "Failed because of missing ack type"
		s.logger.Error(msg)
		resp := NewErrorPushResponse(msg)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			s.logger.Error("Failed to write response", mlog.Err(err))
		}
		if s.metrics != nil {
			s.metrics.incrementBadRequest()
		}
		return
	}

	// Increment ACK
	s.logger.Info("Acknowledge delivery receipt for AckId", mlog.String("AckId", ack.ID))
	if s.metrics != nil {
		s.metrics.incrementDelivered(ack.Platform, ack.Type)
	}

	rMsg := NewOkPushResponse()
	if err := json.NewEncoder(w).Encode(rMsg); err != nil {
		s.logger.Error("Failed to write message", mlog.Err(err))
	}
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
			s.logger.Error("error in getting IP address", mlog.Err(err))
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
