package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds our metrics
type Metrics struct {
	runningFpCounter    prometheus.Gauge
	stoppedFpCounter    prometheus.Gauge
	createdFpCounter    prometheus.Gauge
	registeredFpCounter prometheus.Gauge
	activeFpCounter     prometheus.Gauge
	inactiveFpCounter   prometheus.Gauge
	slashedFpCounter    prometheus.Gauge
}

// RegisterMetrics registers the metrics for finality providers.
func RegisterMetrics() *Metrics {
	m := &Metrics{
		runningFpCounter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "running_finality_providers_counter",
				Help: "Total number of finality providers that are currently running",
			},
		),
		stoppedFpCounter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "stopped_finality_providers_counter",
				Help: "Total number of finality providers that have been stopped",
			},
		),
		createdFpCounter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "created_finality_providers_counter",
				Help: "Total number of finality providers that have been created",
			},
		),
		registeredFpCounter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "registered_finality_providers_counter",
				Help: "Total number of finality providers that have been registered",
			},
		),
		activeFpCounter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_finality_providers_counter",
				Help: "Total number of active finality providers",
			},
		),
		inactiveFpCounter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "inactive_finality_providers_counter",
				Help: "Total number of inactive finality providers",
			},
		),
		slashedFpCounter: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "slashed_finality_providers_counter",
				Help: "Total number of finality providers that have been slashed",
			},
		),
	}

	// Register the metrics.
	prometheus.MustRegister(m.runningFpCounter)
	prometheus.MustRegister(m.stoppedFpCounter)
	prometheus.MustRegister(m.createdFpCounter)
	prometheus.MustRegister(m.registeredFpCounter)
	prometheus.MustRegister(m.activeFpCounter)
	prometheus.MustRegister(m.inactiveFpCounter)
	prometheus.MustRegister(m.slashedFpCounter)

	return m
}

func (m *Metrics) DecrementRunningFpCounter() {
	m.runningFpCounter.Dec()
}

func (m *Metrics) IncrementRunningFpCounter() {
	m.runningFpCounter.Inc()
}

func (m *Metrics) IncrementStoppedFpCounter() {
	m.stoppedFpCounter.Inc()
}

func (m *Metrics) SetCreatedFpCounter(value float64) {
	m.createdFpCounter.Set(value)
}

func (m *Metrics) SetRegisteredFpCounter(value float64) {
	m.registeredFpCounter.Set(value)
}

func (m *Metrics) SetActiveFpCounter(value float64) {
	m.activeFpCounter.Set(value)
}

func (m *Metrics) SetInactiveFpCounter(value float64) {
	m.inactiveFpCounter.Set(value)
}
