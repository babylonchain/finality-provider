package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	ProcessFuncDuration  *prometheus.HistogramVec
	DocumentCount        *prometheus.GaugeVec
	ClientRequestLatency *prometheus.HistogramVec
}

func RegisterMetrics() *Metrics {
	m := &Metrics{
		ProcessFuncDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rpc_poller_polling_job_duration_seconds",
				Help:    "Duration of rpc poller polling job execution time by outcome",
				Buckets: prometheus.LinearBuckets(1, 5, 8), // You need to adjust the number of buckets and the size accordingly.
			},
			[]string{"poller_method", "outcome"},
		),
		DocumentCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rpc_poller_document_count",
				Help: "Count of documents per collection",
			},
			[]string{"collection"},
		),
		ClientRequestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rpc_poller_client_request_latency_ms",
				Help:    "Latency of client requests to dependent services",
				Buckets: prometheus.LinearBuckets(0.5, 0.5, 9), // Adjust as needed for your latency measurements.
			},
			[]string{"client", "method", "outcome"},
		),
	}

	// You must handle potential errors from Register, this is simplified for brevity.
	prometheus.MustRegister(m.ProcessFuncDuration)
	prometheus.MustRegister(m.DocumentCount)
	prometheus.MustRegister(m.ClientRequestLatency)

	return m
}
