// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/kyokomi/emoji"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	apnsAuthError       = "APNS_AUTH_ERROR"
	internalError       = "INTERNAL"
	thirdPartyAuthError = "THIRD_PARTY_AUTH_ERROR"
	invalidArgument     = "INVALID_ARGUMENT"
	quotaExceeded       = "QUOTA_EXCEEDED"
	unregistered        = "UNREGISTERED"
	unavailable         = "UNAVAILABLE"
	tokenSourceError    = "TOKEN_SOURCE_ERROR"
)

const (
	scope = "https://www.googleapis.com/auth/firebase.messaging"
)

type AndroidNotificationServer struct {
	metrics             *metrics
	logger              *mlog.Logger
	AndroidPushSettings AndroidPushSettings
	client              *messaging.Client
	sendTimeout         time.Duration
	retryTimeout        time.Duration
}

// serviceAccount contains a subset of the fields in service-account.json.
// It is mainly used to extract the projectID and client email for authentication.
type serviceAccount struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	ClientID    string `json:"client_id"`
	AuthURI     string `json:"auth_uri"`
	TokenURI    string `json:"token_uri"`
}

func NewAndroidNotificationServer(settings AndroidPushSettings, logger *mlog.Logger, metrics *metrics, sendTimeoutSecs int, retryTimeoutSecs int) *AndroidNotificationServer {
	return &AndroidNotificationServer{
		AndroidPushSettings: settings,
		metrics:             metrics,
		logger:              logger,
		sendTimeout:         time.Duration(sendTimeoutSecs) * time.Second,
		retryTimeout:        time.Duration(retryTimeoutSecs) * time.Second,
	}
}

func (me *AndroidNotificationServer) Initialize() error {
	me.logger.Info("Initializing Android notification server", mlog.String("type", me.AndroidPushSettings.Type))

	if me.AndroidPushSettings.AndroidAPIKey != "" {
		me.logger.Info("AndroidPushSettings.AndroidAPIKey is no longer used. Please remove this config value.")
	}

	if me.AndroidPushSettings.ServiceFileLocation == "" {
		return errors.New("Android push notifications not configured.  Missing ServiceFileLocation.")
	}

	jsonKey, err := os.ReadFile(me.AndroidPushSettings.ServiceFileLocation)
	if err != nil {
		return fmt.Errorf("error reading service file: %v", err)
	}

	cfg, err := google.JWTConfigFromJSON(jsonKey, scope)
	if err != nil {
		return fmt.Errorf("error getting JWT config: %v", err)
	}

	var serviceAcc serviceAccount
	err = json.Unmarshal(jsonKey, &serviceAcc)
	if err != nil {
		return fmt.Errorf("error parsing service account JSON: %v", err)
	}

	opt := option.WithTokenSource(cfg.TokenSource(context.Background()))
	conf := &firebase.Config{
		ProjectID:        serviceAcc.ProjectID,
		ServiceAccountID: serviceAcc.ClientEmail,
	}
	app, err := firebase.NewApp(context.Background(), conf, opt)
	if err != nil {
		return fmt.Errorf("error initializing app: %v", err)
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return fmt.Errorf("error initializing client: %v", err)
	}
	me.client = client

	return nil
}

func (me *AndroidNotificationServer) SendNotification(msg *PushNotification) PushResponse {
	pushType := msg.Type
	data := map[string]string{
		"ack_id":         msg.AckID,
		"type":           pushType,
		"sub_type":       msg.SubType,
		"version":        msg.Version,
		"channel_id":     msg.ChannelID,
		"is_crt_enabled": strconv.FormatBool(msg.IsCRTEnabled),
		"server_id":      msg.ServerID,
		"category":       msg.Category,
	}

	if msg.Badge != -1 {
		data["badge"] = strconv.Itoa(msg.Badge)
	}

	if msg.RootID != "" {
		data["root_id"] = msg.RootID
	}

	if msg.Signature == "" {
		data["signature"] = "NO_SIGNATURE"
	} else {
		data["signature"] = msg.Signature
	}

	if msg.IsIDLoaded {
		data["post_id"] = msg.PostID
		data["message"] = msg.Message
		data["id_loaded"] = "true"
		data["sender_id"] = msg.SenderID
		data["sender_name"] = "Someone"
		data["team_id"] = msg.TeamID
	} else if pushType == PushTypeMessage || pushType == PushTypeSession {
		data["team_id"] = msg.TeamID
		data["sender_id"] = msg.SenderID
		data["sender_name"] = msg.SenderName
		data["message"] = emoji.Sprint(msg.Message)
		data["channel_name"] = msg.ChannelName
		data["post_id"] = msg.PostID
		data["override_username"] = msg.OverrideUsername
		data["override_icon_url"] = msg.OverrideIconURL
		data["from_webhook"] = msg.FromWebhook
	}

	if me.metrics != nil {
		me.metrics.incrementNotificationTotal(PushNotifyAndroid, pushType)
	}
	fcmMsg := &messaging.Message{
		Token: msg.DeviceID,
		Data:  data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	me.logger.Info(
		"Sending android push notification",
		mlog.String("device", me.AndroidPushSettings.Type),
		mlog.String("type", msg.Type),
		mlog.String("ack_id", msg.AckID),
	)
	err := me.SendNotificationWithRetry(fcmMsg)

	if err != nil {
		errorCode, hasStatusCode := getErrorCode(err)
		if !hasStatusCode {
			errorCode = "NONE"
		}

		me.logger.Error(
			"Failed to send FCM push",
			mlog.String("sid", msg.ServerID),
			mlog.String("did", msg.DeviceID),
			mlog.Err(err),
			mlog.String("type", me.AndroidPushSettings.Type),
			mlog.String("errorCode", errorCode),
		)

		if messaging.IsUnregistered(err) || messaging.IsSenderIDMismatch(err) {
			me.logger.Info("Android response failure sending remove code", mlog.String("type", me.AndroidPushSettings.Type))
			if me.metrics != nil {
				me.metrics.incrementRemoval(PushNotifyAndroid, pushType, unregistered)
			}
			return NewRemovePushResponse()
		}

		var reason string
		switch {
		case messaging.IsInternal(err):
			reason = internalError
		case messaging.IsInvalidArgument(err):
			reason = invalidArgument
		case messaging.IsQuotaExceeded(err):
			reason = quotaExceeded
		case messaging.IsThirdPartyAuthError(err):
			reason = thirdPartyAuthError
		case messaging.IsUnavailable(err):
			reason = unavailable
		default:
			reason = "unknown transport error"

		}
		if me.metrics != nil {
			me.metrics.incrementFailure(PushNotifyAndroid, pushType, reason)
		}

		return NewErrorPushResponse(err.Error())
	}

	if me.metrics != nil {
		if msg.AckID != "" {
			me.metrics.incrementSuccessWithAck(PushNotifyAndroid, pushType)
		} else {
			me.metrics.incrementSuccess(PushNotifyAndroid, pushType)
		}
	}
	return NewOkPushResponse()
}

func (me *AndroidNotificationServer) SendNotificationWithRetry(fcmMsg *messaging.Message) error {
	var err error
	waitTime := time.Second

	logger := me.logger.With(mlog.String("did", fcmMsg.Token))

	// Keep a general context to make sure the whole retry
	// doesn't take longer than the timeout.
	generalContext, cancelGeneralContext := context.WithTimeout(context.Background(), me.sendTimeout)
	defer cancelGeneralContext()

	for retries := 0; retries < MAX_RETRIES; retries++ {
		start := time.Now()

		retryContext, cancelRetryContext := context.WithTimeout(generalContext, me.retryTimeout)
		defer cancelRetryContext()
		_, err = me.client.Send(retryContext, fcmMsg)
		if me.metrics != nil {
			me.metrics.observerNotificationResponse(PushNotifyAndroid, time.Since(start).Seconds())
		}

		if err == nil || !isRetryable(err) {
			break
		}

		logger.Error(
			"Failed to send android push",
			mlog.Int("retry", retries),
			mlog.Err(err),
		)

		if retries == MAX_RETRIES-1 {
			logger.Error("Max retries reached")
			break
		}

		select {
		case <-generalContext.Done():
			if generalContext.Err() != nil {
				logger.Info(
					"Not retrying because context error",
					mlog.Int("retry", retries),
					mlog.Err(generalContext.Err()),
				)
			}
			return generalContext.Err()
		case <-time.After(waitTime):
		}

		waitTime *= 2
	}

	return err
}

func isRetryable(err error) bool {
	// We retry if the context deadline is exceeded.
	// This may cause double notifications, but we expect
	// this not to happen often.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// We retry the errors based on https://firebase.google.com/docs/cloud-messaging/http-server-ref
	return messaging.IsInternal(err) ||
		messaging.IsQuotaExceeded(err)

	// messaging.IsUnavailable is retried by the default retry config in
	// firebase.google.com/go/v4@v4.14.0/internal/http_client.go
	// messaging.IsUnavailable(err)
}

func getErrorCode(err error) (string, bool) {
	if err == nil {
		return "", false
	}

	errorPointer := reflect.ValueOf(err)
	if errorPointer.Kind() != reflect.Ptr {
		return "", false
	}

	errorValue := errorPointer.Elem()
	if errorValue.Kind() != reflect.Struct {
		return "", false
	}

	codeValue := errorValue.FieldByName("ErrorCode")
	if !codeValue.IsValid() {
		return "", false
	}

	return codeValue.String(), true
}
