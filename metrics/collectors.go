package metrics

import (
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

// Metrics holds our metrics
type Metrics struct {
	runningFpGauge prometheus.Gauge
	fpStatus       *prometheus.GaugeVec
}

// Declare a package-level variable for sync.Once to ensure metrics are registered only once
var registerOnce sync.Once

// Declare a variable to hold the instance of Metrics
var metricsInstance *Metrics

// RegisterMetrics initializes and registers the metrics, using sync.Once to ensure it's done only once
func RegisterMetrics() *Metrics {
	registerOnce.Do(func() {
		metricsInstance = &Metrics{
			runningFpGauge: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "running_finality_providers",
				Help: "Current number of finality providers that are running",
			}),
			fpStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "finality_provider_status",
				Help: "Current status of a finality provider",
			}, []string{"fp_btc_pk_hex"}),
		}

		// Register the metrics with Prometheus
		prometheus.MustRegister(metricsInstance.runningFpGauge)
		prometheus.MustRegister(metricsInstance.fpStatus)
	})
	return metricsInstance
}

func (m *Metrics) DecrementRunningFpGauge() {
	m.runningFpGauge.Dec()
}

func (m *Metrics) IncrementRunningFpGauge() {
	m.runningFpGauge.Inc()
}

func (m *Metrics) RecordFpStatus(fpBtcPkHex string, status proto.FinalityProviderStatus) {
	m.fpStatus.WithLabelValues(fpBtcPkHex).Set(float64(status))
}
