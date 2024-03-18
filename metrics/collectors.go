package metrics

import (
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

type Metrics struct {
	runningFpGauge       prometheus.Gauge
	fpStatus             *prometheus.GaugeVec
	babylonTipHeight     prometheus.Gauge
	lastPolledHeight     prometheus.Gauge
	pollerStartingHeight prometheus.Gauge
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
			babylonTipHeight: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "babylon_tip_height",
				Help: "The current tip height of the Babylon network",
			}),
			lastPolledHeight: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "last_polled_height",
				Help: "The most recent block height checked by the poller",
			}),
			pollerStartingHeight: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "poller_starting_height",
				Help: "The initial block height when the poller started operation",
			}),
		}

		// Register the metrics with Prometheus
		prometheus.MustRegister(metricsInstance.runningFpGauge)
		prometheus.MustRegister(metricsInstance.fpStatus)
		prometheus.MustRegister(metricsInstance.babylonTipHeight)
		prometheus.MustRegister(metricsInstance.lastPolledHeight)
		prometheus.MustRegister(metricsInstance.pollerStartingHeight)
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

func (m *Metrics) RecordBabylonTipHeight(height uint64) {
	m.babylonTipHeight.Set(float64(height))
}

func (m *Metrics) RecordLastPolledHeight(height uint64) {
	m.lastPolledHeight.Set(float64(height))
}

func (m *Metrics) RecordPollerStartingHeight(height uint64) {
	m.pollerStartingHeight.Set(float64(height))
}
