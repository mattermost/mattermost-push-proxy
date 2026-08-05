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

	"github.com/mattermost/mattermost/server/public/model"
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
	stats               *stats
	logger              *mlog.Logger
	AndroidPushSettings AndroidPushSettings
	client              *messaging.Client
	sendTimeout         time.Duration
	retryTimeout        time.Duration
	// validateToken mints an OAuth2 token from the service account to verify
	// the credentials are still accepted by Google. It is nil until a service
	// account has been loaded in Initialize.
	validateToken func(context.Context) error
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

func NewAndroidNotificationServer(settings AndroidPushSettings, logger *mlog.Logger, metrics *metrics, stats *stats, sendTimeoutSecs int, retryTimeoutSecs int) *AndroidNotificationServer {
	return &AndroidNotificationServer{
		AndroidPushSettings: settings,
		metrics:             metrics,
		stats:               stats,
		logger:              logger,
		sendTimeout:         time.Duration(sendTimeoutSecs) * time.Second,
		retryTimeout:        time.Duration(retryTimeoutSecs) * time.Second,
	}
}

func (me *AndroidNotificationServer) Initialize() error {
	me.logger.Info("Initializing Android notification server", mlog.String("target_type", me.AndroidPushSettings.Type))

	if me.AndroidPushSettings.AndroidAPIKey != "" {
		me.logger.Warn("AndroidPushSettings.AndroidAPIKey is no longer used. Please remove this config value.")
	}

	if me.AndroidPushSettings.ServiceFileLocation == "" {
		return errors.New("android push notifications not configured: missing ServiceFileLocation")
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

	me.validateToken = func(ctx context.Context) error {
		_, tErr := cfg.TokenSource(ctx).Token()
		return tErr
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

// checkCredentialExpiry validates the FCM service account credentials by
// minting an OAuth2 token. The service account key carries no readable expiry,
// so this actively probes whether the key is still accepted (it fails once the
// key is revoked, disabled, or deleted) and logs an Error to alert on.
func (me *AndroidNotificationServer) checkCredentialExpiry() {
	if me.validateToken == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), me.sendTimeout)
	defer cancel()

	err := me.validateToken(ctx)
	if err == nil {
		return
	}

	// A timeout is a transient/network problem, not evidence the key itself is
	// bad, so keep it out of Error to avoid false credential alerts.
	if errors.Is(err, context.DeadlineExceeded) {
		me.logger.Warn(
			"FCM credential check timed out; could not verify the service account key",
			mlog.String("target_type", me.AndroidPushSettings.Type),
			mlog.Err(err),
		)
		return
	}

	me.logger.Error(
		"FCM credentials rejected; the service account key may be revoked, disabled, or expired",
		mlog.String("target_type", me.AndroidPushSettings.Type),
		mlog.Err(err),
	)
}

func (me *AndroidNotificationServer) SendNotification(_ int, msg *model.PushNotification) PushResponse {
	pushType := msg.Type
	data := map[string]string{
		"ack_id":         msg.AckId,
		"type":           pushType,
		"sub_type":       string(msg.SubType),
		"version":        msg.Version,
		"channel_id":     msg.ChannelId,
		"is_crt_enabled": strconv.FormatBool(msg.IsCRTEnabled),
		"server_id":      msg.ServerId,
		"category":       msg.Category,
	}

	if msg.Badge != -1 {
		data["badge"] = strconv.Itoa(msg.Badge)
	}

	if msg.RootId != "" {
		data["root_id"] = msg.RootId
	}

	if msg.Signature == "" {
		data["signature"] = "NO_SIGNATURE"
	} else {
		data["signature"] = msg.Signature
	}

	if msg.IsIdLoaded {
		data["post_id"] = msg.PostId
		data["message"] = msg.Message
		data["id_loaded"] = "true"
		data["sender_id"] = msg.SenderId
		data["sender_name"] = "Someone"
		data["team_id"] = msg.TeamId
	} else if pushType == model.PushTypeMessage || pushType == model.PushTypeSession {
		data["team_id"] = msg.TeamId
		data["sender_id"] = msg.SenderId
		data["sender_name"] = msg.SenderName
		data["message"] = emoji.Sprint(msg.Message)
		data["channel_name"] = msg.ChannelName
		data["post_id"] = msg.PostId
		data["override_username"] = msg.OverrideUsername
		data["override_icon_url"] = msg.OverrideIconURL
		data["from_webhook"] = msg.FromWebhook
	}

	if me.metrics != nil {
		me.metrics.incrementNotificationTotal(model.PushNotifyAndroid, pushType, model.PushTransportStandard)
	}
	fcmMsg := &messaging.Message{
		Token: msg.DeviceId,
		Data:  data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	me.stats.incrementAndroidSend()
	me.logger.Debug(
		"Sending android push notification",
		mlog.String("target_type", me.AndroidPushSettings.Type),
		mlog.String("push_type", msg.Type),
		mlog.String("ack_id", msg.AckId),
	)
	err := me.SendNotificationWithRetry(fcmMsg)
	if err != nil {
		// An unregistered or mismatched token is a client-driven condition, not
		// a server fault: log it at Warn and route through the removal branch.
		if messaging.IsUnregistered(err) || messaging.IsSenderIDMismatch(err) {
			me.logger.Warn(
				"Android response failure sending remove code",
				mlog.String("server_id", msg.ServerId),
				mlog.String("device_id", redactToken(msg.DeviceId)),
				mlog.String("target_type", me.AndroidPushSettings.Type),
				mlog.Err(err),
			)
			if me.metrics != nil {
				me.metrics.incrementRemoval(model.PushNotifyAndroid, pushType, model.PushTransportStandard, unregistered)
			}
			return NewRemovePushResponse()
		}

		errorCode, hasStatusCode := getErrorCode(err)
		if !hasStatusCode {
			errorCode = "NONE"
		}

		me.logger.Error(
			"Failed to send FCM push",
			mlog.String("server_id", msg.ServerId),
			mlog.String("device_id", redactToken(msg.DeviceId)),
			mlog.Err(err),
			mlog.String("target_type", me.AndroidPushSettings.Type),
			mlog.String("error_code", errorCode),
		)

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
			me.metrics.incrementFailure(model.PushNotifyAndroid, pushType, model.PushTransportStandard, reason)
		}

		return NewErrorPushResponse(err.Error())
	}

	if me.metrics != nil {
		if msg.AckId != "" {
			me.metrics.incrementSuccessWithAck(model.PushNotifyAndroid, pushType, model.PushTransportStandard)
		} else {
			me.metrics.incrementSuccess(model.PushNotifyAndroid, pushType, model.PushTransportStandard)
		}
	}
	return NewOkPushResponse()
}

func (me *AndroidNotificationServer) SendNotificationWithRetry(fcmMsg *messaging.Message) error {
	var err error
	waitTime := time.Second

	logger := me.logger.With(mlog.String("device_id", redactToken(fcmMsg.Token)))

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
			me.metrics.observerNotificationResponse(model.PushNotifyAndroid, time.Since(start).Seconds())
		}

		if err == nil || !isRetryable(err) {
			break
		}

		logger.Warn(
			"Failed to send android push",
			mlog.Int("retry", retries),
			mlog.Err(err),
		)

		if retries == MAX_RETRIES-1 {
			logger.Warn("Max retries reached")
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
	if errorPointer.Kind() != reflect.Pointer {
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
