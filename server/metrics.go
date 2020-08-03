package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricNotificationsTotalName   = "service_notifications_total"
	metricSuccessName              = "service_success_total"
	metricSuccessWithAckName       = "service_success_with_ack_total"
	metricDeliveredName            = "service_delivered_total"
	metricFailureName              = "service_failure_total"
	metricFailureWithReasonName    = "service_failure_with_reason_total"
	metricRemovalName              = "service_removal_total"
	metricBadRequestName           = "service_bad_request_total"
	metricFCMResponseName          = "service_fcm_request_duration_seconds"
	metricAPNSResponseName         = "service_apns_request_duration_seconds"
	metricServiceResponseName      = "service_request_duration_seconds"
	metricNotificationResponseName = "service_notification_duration_seconds"
)

// NewPrometheusHandler returns the http.Handler to expose Prometheus metrics
func NewPrometheusHandler() http.Handler {
	return promhttp.Handler()
}

type metrics struct {
	metricNotificationsTotal   *prometheus.CounterVec
	metricSuccess              *prometheus.CounterVec
	metricSuccessWithAck       *prometheus.CounterVec
	metricDelivered            *prometheus.CounterVec
	metricFailure              *prometheus.CounterVec
	metricFailureWithReason    *prometheus.CounterVec
	metricRemoval              *prometheus.CounterVec
	metricBadRequest           prometheus.Counter
	metricAPNSResponse         prometheus.Histogram
	metricFCMResponse          prometheus.Histogram
	metricNotificationResponse *prometheus.HistogramVec
	metricServiceResponse      prometheus.Histogram
}

// newMetrics initializes the metrics and registers them
func newMetrics() *metrics {
	m := &metrics{
		metricNotificationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricNotificationsTotalName,
			Help: "Number of notifications sent"},
			[]string{"platform", "type"}),
		metricSuccess: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricSuccessName,
			Help: "Number of push success."},
			[]string{"platform", "type"}),
		metricSuccessWithAck: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricSuccessWithAckName,
			Help: "Number of push success that contains ackId."},
			[]string{"platform", "type"},
		),
		metricDelivered: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricDeliveredName,
			Help: "Number of push delivered."},
			[]string{"platform", "type"},
		),
		metricFailure: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricFailureName,
			Help: "Number of push errors."},
			[]string{"platform", "type"}),
		metricFailureWithReason: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricFailureWithReasonName,
			Help: "Number of push errors with reasons."},
			[]string{"platform", "type", "reason"}),
		metricRemoval: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricRemovalName,
			Help: "Number of device token errors."},
			[]string{"platform", "reason"}),
		metricBadRequest: prometheus.NewCounter(prometheus.CounterOpts{
			Name: metricBadRequestName,
			Help: "Request to push proxy was a bad request",
		}),
		metricAPNSResponse: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: metricAPNSResponseName,
			Help: "Request latency distribution",
		}),
		metricFCMResponse: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: metricFCMResponseName,
			Help: "Request latency distribution",
		}),
		metricNotificationResponse: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: metricNotificationResponseName,
			Help: "Notifiction request latency distribution"},
			[]string{"platform"}),
		metricServiceResponse: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: metricServiceResponseName,
			Help: "Request latency distribution",
		}),
	}

	prometheus.MustRegister(
		m.metricNotificationsTotal,
		m.metricSuccess,
		m.metricSuccessWithAck,
		m.metricFailure,
		m.metricFailureWithReason,
		m.metricRemoval,
		m.metricBadRequest,
		m.metricAPNSResponse,
		m.metricFCMResponse,
		m.metricServiceResponse,
		m.metricNotificationResponse,
	)

	return m
}

func (m *metrics) shutdown() {
	func(cs ...prometheus.Collector) {
		for _, c := range cs {
			prometheus.Unregister(c)
		}
	}(
		m.metricNotificationsTotal,
		m.metricSuccess,
		m.metricSuccessWithAck,
		m.metricFailure,
		m.metricFailureWithReason,
		m.metricRemoval,
		m.metricBadRequest,
		m.metricAPNSResponse,
		m.metricFCMResponse,
		m.metricServiceResponse,
		m.metricNotificationResponse,
	)
}

func (m *metrics) incrementNotificationTotal(platform, pushType string) {
	m.metricNotificationsTotal.WithLabelValues(platform, pushType).Inc()
}

func (m *metrics) incrementSuccess(platform, pushType string) {
	m.metricSuccess.WithLabelValues(platform, pushType).Inc()
}

func (m *metrics) incrementSuccessWithAck(platform, pushType string) {
	m.incrementSuccess(platform, pushType)
	m.metricSuccessWithAck.WithLabelValues(platform, pushType).Inc()
}

func (m *metrics) incrementDelivered(platform, pushType string) {
	m.metricDelivered.WithLabelValues(platform, pushType).Inc()
}

func (m *metrics) incrementFailure(platform, pushType, reason string) {
	m.metricFailure.WithLabelValues(platform, pushType).Inc()
	if len(reason) > 0 {
		m.metricFailureWithReason.WithLabelValues(platform, pushType, reason).Inc()
	}
}

func (m *metrics) incrementRemoval(platform, pushType, reason string) {
	m.metricRemoval.WithLabelValues(platform, reason).Inc()
	m.incrementFailure(platform, pushType, reason)
}

func (m *metrics) incrementBadRequest() {
	m.metricBadRequest.Inc()
}

func (m *metrics) observeAPNSResponse(dur float64) {
	m.metricAPNSResponse.Observe(dur)
}

func (m *metrics) observeFCMResponse(dur float64) {
	m.metricFCMResponse.Observe(dur)
}

func (m *metrics) observeServiceResponse(dur float64) {
	m.metricServiceResponse.Observe(dur)
}

func (m *metrics) observerNotificationResponse(platform string, dur float64) {
	m.metricNotificationResponse.WithLabelValues(platform).Observe(dur)
	switch platform {
	case PushNotifyApple:
		m.observeAPNSResponse(dur)
	case PushNotifyAndroid:
		m.observeFCMResponse(dur)
	}
}
