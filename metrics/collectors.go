package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds our metrics
type Metrics struct {
	runningFpCounter    prometheus.Counter
	stoppedFpCounter    prometheus.Counter
	createdFpCounter    prometheus.Counter
	registeredFpCounter prometheus.Counter
	activeFpCounter     prometheus.Counter
	inactiveFpCounter   prometheus.Counter
	slashedFpCounter    prometheus.Counter
}

// RegisterMetrics registers the metrics for finality providers.
func RegisterMetrics() *Metrics {
	m := &Metrics{
		runningFpCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "running_finality_providers_counter",
				Help: "Total number of finality providers that are currently running",
			},
		),
		stoppedFpCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "stopped_finality_providers_counter",
				Help: "Total number of finality providers that have been stopped",
			},
		),
		createdFpCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "created_finality_providers_counter",
				Help: "Total number of finality providers that have been created",
			},
		),
		registeredFpCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "registered_finality_providers_counter",
				Help: "Total number of finality providers that have been registered",
			},
		),
		activeFpCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "active_finality_providers_counter",
				Help: "Total number of active finality providers",
			},
		),
		inactiveFpCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "inactive_finality_providers_counter",
				Help: "Total number of inactive finality providers",
			},
		),
		slashedFpCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
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

func (m *Metrics) IncrementRunningFPCounter() {
	m.runningFpCounter.Inc()
}

func (m *Metrics) IncrementStoppedFPCounter() {
	m.stoppedFpCounter.Inc()
}

func (m *Metrics) IncrementCreatedFPCounter() {
	m.createdFpCounter.Inc()
}

func (m *Metrics) IncrementRegisteredFPCounter() {
	m.registeredFpCounter.Inc()
}
