package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var MetricsEnabled bool

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

var metricNotificationsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricNotificationsTotalName,
		Help: "Number of notifications sent",
	},
	[]string{"platform", "type"},
)

var metricSuccess = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricSuccessName,
		Help: "Number of push success.",
	},
	[]string{"platform", "type"},
)

var metricSuccessWithAck = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricSuccessWithAckName,
		Help: "Number of push success that contains ackId.",
	},
	[]string{"platform", "type"},
)

var metricDelivered = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricDeliveredName,
		Help: "Number of push delivered.",
	},
	[]string{"platform", "type"},
)

var metricFailure = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricFailureName,
		Help: "Number of push errors.",
	},
	[]string{"platform", "type"},
)

var metricFailureWithReason = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricFailureWithReasonName,
		Help: "Number of push errors with reasons.",
	},
	[]string{"platform", "type", "reason"},
)

var metricRemoval = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricRemovalName,
		Help: "Number of device token errors.",
	},
	[]string{"platform", "reason"},
)

var metricBadRequest = prometheus.NewCounter(prometheus.CounterOpts{
	Name: metricBadRequestName,
	Help: "Request to push proxy was a bad request",
})

var metricAPNSResponse = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: metricAPNSResponseName,
	Help: "Request latency distribution",
})

var metricFCMResponse = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: metricFCMResponseName,
	Help: "Request latency distribution",
})

var metricNotificationResponse = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: metricNotificationResponseName,
	Help: "Notifiction request latency distribution",
}, []string{"platform"})

var metricServiceResponse = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: metricServiceResponseName,
	Help: "Request latency distribution",
})

func init() {
	prometheus.MustRegister(
		metricNotificationsTotal,
		metricSuccess,
		metricSuccessWithAck,
		metricFailure,
		metricFailureWithReason,
		metricRemoval,
		metricBadRequest,
		metricAPNSResponse,
		metricFCMResponse,
		metricServiceResponse,
		metricNotificationResponse,
	)
}

func NewPrometheusHandler() http.Handler {
	return promhttp.Handler()
}

func incrementNotificationTotal(platform, pushType string) {
	if MetricsEnabled {
		metricNotificationsTotal.WithLabelValues(platform, pushType).Inc()
	}
}

func incrementSuccess(platform, pushType string) {
	if MetricsEnabled {
		metricSuccess.WithLabelValues(platform, pushType).Inc()
	}
}

func incrementSuccessWithAck(platform, pushType string) {
	incrementSuccess(platform, pushType)
	if MetricsEnabled {
		metricSuccessWithAck.WithLabelValues(platform, pushType).Inc()
	}
}

func incrementDelivered(platform, pushType string) {
	if MetricsEnabled {
		metricDelivered.WithLabelValues(platform, pushType).Inc()
	}
}

func incrementFailure(platform, pushType, reason string) {
	if MetricsEnabled {
		metricFailure.WithLabelValues(platform, pushType).Inc()
		if reason != "" {
			metricFailureWithReason.WithLabelValues(platform, pushType, reason).Inc()
		}
	}
}

func incrementRemoval(platform, pushType, reason string) {
	if MetricsEnabled {
		metricRemoval.WithLabelValues(platform, reason).Inc()
		incrementFailure(platform, pushType, reason)
	}
}

func incrementBadRequest() {
	if MetricsEnabled {
		metricBadRequest.Inc()
	}
}

func observeAPNSResponse(dur float64) {
	if MetricsEnabled {
		metricAPNSResponse.Observe(dur)
	}
}

func observeFCMResponse(dur float64) {
	if MetricsEnabled {
		metricFCMResponse.Observe(dur)
	}
}

func observeServiceResponse(dur float64) {
	if MetricsEnabled {
		metricServiceResponse.Observe(dur)
	}
}

func observerNotificationResponse(platform string, dur float64) {
	if MetricsEnabled {
		metricNotificationResponse.WithLabelValues(platform).Observe(dur)

		switch platform {
		case PushNotifyApple:
			observeAPNSResponse(dur)
		case PushNotifyAndroid:
			observeFCMResponse(dur)
		}
	}
}
