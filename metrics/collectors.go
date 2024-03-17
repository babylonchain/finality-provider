package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds our metrics
type Metrics struct {
	runningFpGauge prometheus.Gauge
	stoppedFpGauge prometheus.Gauge
	fpStatus       *prometheus.GaugeVec
}

// RegisterMetrics registers the metrics for finality providers.
func RegisterMetrics() *Metrics {
	m := &Metrics{
		runningFpGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "running_finality_providers",
				Help: "Current number of finality providers that are running",
			},
		),
		stoppedFpGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "stopped_finality_providers",
				Help: "Current number of finality providers that have been stopped",
			},
		),
		fpStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "finality_provider_status",
				Help: "Current status of a finality provider",
			},
			[]string{"fp_name"},
		),
	}

	// Register the metrics.
	prometheus.MustRegister(m.runningFpGauge)
	prometheus.MustRegister(m.stoppedFpGauge)
	prometheus.MustRegister(m.fpStatus)
	return m
}

func (m *Metrics) DecrementRunningFpGauge() {
	m.runningFpGauge.Dec()
}

func (m *Metrics) IncrementRunningFpGauge() {
	m.runningFpGauge.Inc()
}

func (m *Metrics) IncrementStoppedFpGauge() {
	m.stoppedFpGauge.Inc()
}

func (m *Metrics) RecordFpStatus(fpName string, status float64) {
	m.fpStatus.WithLabelValues(fpName).Set(status)
}
