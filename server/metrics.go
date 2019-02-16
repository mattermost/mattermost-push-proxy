package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var MetricsEnabled bool

const (
	metricSuccessName         = "service_success_total"
	metricFailureName         = "service_failure_total"
	metricRemovalName         = "service_removal_total"
	metricBadRequestName      = "service_bad_request_total"
	metricFCMResponseName     = "service_fcm_request_duration_seconds"
	metricAPNSResponseName    = "service_apns_request_duration_seconds"
	metricServiceResponseName = "service_request_duration_seconds"
)

var metricSuccess = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricSuccessName,
		Help: "Number of push success.",
	},
	[]string{"type"},
)

var metricFailure = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricFailureName,
		Help: "Number of push errors.",
	},
	[]string{"type"},
)

var metricRemoval = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: metricRemovalName,
		Help: "Number of token errors.",
	},
	[]string{"type"},
)

var metricBadRequest = prometheus.NewCounter(prometheus.CounterOpts{
	Name: metricBadRequestName,
	Help: "Request to pushproxy was a bad request",
})

var metricAPNSResponse = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: metricAPNSResponseName,
	Help: "Request latency distribution",
})

var metricFCMResponse = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: metricFCMResponseName,
	Help: "Request latency distribution",
})

var metricServiceResponse = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: metricServiceResponseName,
	Help: "Request latency distribution",
})

func init() {
	prometheus.MustRegister(metricSuccess, metricFailure, metricRemoval)
	prometheus.MustRegister(metricBadRequest)
	prometheus.MustRegister(metricAPNSResponse, metricFCMResponse, metricServiceResponse)
}

func NewPrometheusHandler() http.Handler {
	return prometheus.Handler()
}

func incrementSuccess(pushType string) {
	if MetricsEnabled {
		metricSuccess.WithLabelValues(pushType).Inc()
	}
}

func incrementFailure(pushType string) {
	if MetricsEnabled {
		metricFailure.WithLabelValues(pushType).Inc()
	}
}

func incrementRemoval(pushType string) {
	if MetricsEnabled {
		metricRemoval.WithLabelValues(pushType).Inc()
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
