package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func TestIncrementNotificationByAppVersion(t *testing.T) {
	m := newMetrics()
	defer m.shutdown()

	platform := model.PushNotifyApple

	// Versions 1 and 2 are tracked discretely; everything else collapses
	// into the "other" sentinel to keep cardinality bounded.
	m.incrementNotificationByAppVersion(platform, 1)
	m.incrementNotificationByAppVersion(platform, 2)
	m.incrementNotificationByAppVersion(platform, 3)
	m.incrementNotificationByAppVersion(platform, 99)

	assert.Equal(t, float64(1), testutil.ToFloat64(m.metricNotificationByAppVersion.WithLabelValues(platform, "1")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.metricNotificationByAppVersion.WithLabelValues(platform, "2")))
	assert.Equal(t, float64(2), testutil.ToFloat64(m.metricNotificationByAppVersion.WithLabelValues(platform, appVersionOther)))
}

func TestMetricDisabled(t *testing.T) {
	t.Log("Testing Metrics Enabled")
	platform := "junk"
	pushType := model.PushTypeMessage

	fileName := FindConfigFile("mattermost-push-proxy.sample.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)
	cfg.AndroidPushSettings[0].AndroidAPIKey = platform
	cfg.EnableMetrics = false

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	srv := New(cfg, logger)
	srv.Start()

	time.Sleep(time.Second * 2)
	defer func() {
		srv.Stop()
		time.Sleep(time.Second * 2)
	}()

	m := newMetrics()
	defer m.shutdown()

	m.incrementBadRequest()
	m.incrementNotificationTotal(platform, pushType, "")
	m.incrementNotificationByAppVersion(platform, 1)
	m.incrementSuccess(platform, pushType, "")
	m.incrementRemoval(platform, pushType, "", "not registered")
	m.incrementFailure(platform, pushType, "", "error")
	m.observerNotificationResponse(model.PushNotifyApple, 1)
	m.observerNotificationResponse(model.PushNotifyAndroid, 1)
	m.observeServiceResponse(1)

	resp, err := http.Get("http://localhost:8066/metrics")
	if err != nil {
		t.Fatalf("service should not return an http error")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("service should return a parsable response")
	}
	if !strings.Contains(string(data), "404 page not found") {
		t.Fatalf("service should return a 404")
	}
}

func TestMetricEnabled(t *testing.T) {
	t.Log("Testing Metrics Enabled")
	platform := "junk"
	pushType := model.PushTypeMessage

	fileName := FindConfigFile("mattermost-push-proxy.sample.json")
	cfg, err := LoadConfig(fileName)
	require.NoError(t, err)
	cfg.AndroidPushSettings[0].AndroidAPIKey = platform
	cfg.EnableMetrics = true

	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	srv := New(cfg, logger)
	srv.Start()

	time.Sleep(time.Second * 2)
	defer func() {
		srv.Stop()
		time.Sleep(time.Second * 2)
	}()

	srv.metrics.incrementBadRequest()
	srv.metrics.incrementNotificationTotal(platform, pushType, "")
	srv.metrics.incrementNotificationByAppVersion(platform, 1)
	srv.metrics.incrementSuccess(platform, pushType, "")
	srv.metrics.incrementRemoval(platform, pushType, "", "not registered")
	srv.metrics.incrementFailure(platform, pushType, "", "error")
	srv.metrics.observerNotificationResponse(model.PushNotifyApple, 1)
	srv.metrics.observerNotificationResponse(model.PushNotifyAndroid, 1)
	srv.metrics.observeServiceResponse(1)

	resp, err := http.Get("http://localhost:8066/metrics")
	if err != nil {
		t.Fatalf("failed to get metrics endpoint - %s", err.Error())
	}
	defer resp.Body.Close()

	parser := &expfmt.TextParser{}
	metrics, _ := parser.TextToMetricFamilies(resp.Body)

	counters := []string{metricSuccessName, metricFailureName, metricFailureWithReasonName, metricRemovalName, metricBadRequestName, metricNotificationsTotalName, metricNotificationByAppVersionName}
	for _, cn := range counters {
		if m, ok := metrics[cn]; !ok {
			t.Fatalf("metric not found. name: %s", cn)
		} else {
			val := m.Metric[0].Counter.Value
			result := float64(1)

			if cn == metricFailureName {
				result = float64(2)
			}

			if val == nil {
				t.Fatalf("no metric value. name: %s", cn)
			}
			if *val != result {
				t.Fatalf("metric value does not match. mame: %s, got: %v, expected: %v",
					cn, *val, result)
			}
		}
	}

	histograms := []string{metricAPNSResponseName, metricFCMResponseName, metricServiceResponseName, metricNotificationResponseName}
	for _, hn := range histograms {
		if m, ok := metrics[hn]; !ok {
			t.Fatalf("metric not found. name: %s", hn)
		} else {
			val := m.Metric[0].Histogram.SampleCount
			if val == nil {
				t.Fatalf("no metric value. name: %s", hn)
			}
			if *val != 1 {
				t.Fatalf("metric value does not match. mame: %s, got: %v, expected: %v",
					hn, *val, 1)
			}
		}
	}
}
