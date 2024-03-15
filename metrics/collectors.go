package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds our metrics
type Metrics struct {
	RunningFPCounter    prometheus.Counter
	StoppedFPCounter    prometheus.Counter
	CreatedFPCounter    prometheus.Counter
	RegisteredFPCounter prometheus.Counter
	ActiveFPCounter     prometheus.Counter
	InactiveFPCounter   prometheus.Counter
	SlashedFPCounter    prometheus.Counter
}

// RegisterMetrics registers the metrics for finality providers.
func RegisterMetrics() *Metrics {
	m := &Metrics{
		RunningFPCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "running_finality_providers_counter",
				Help: "Total number of finality providers that are currently running",
			},
		),
		StoppedFPCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "stopped_finality_providers_counter",
				Help: "Total number of finality providers that have been stopped",
			},
		),
		CreatedFPCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "created_finality_providers_counter",
				Help: "Total number of finality providers that have been created",
			},
		),
		RegisteredFPCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "registered_finality_providers_counter",
				Help: "Total number of finality providers that have been registered",
			},
		),
		ActiveFPCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "active_finality_providers_counter",
				Help: "Total number of active finality providers",
			},
		),
		InactiveFPCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "inactive_finality_providers_counter",
				Help: "Total number of inactive finality providers",
			},
		),
		SlashedFPCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "slashed_finality_providers_counter",
				Help: "Total number of finality providers that have been slashed",
			},
		),
	}

	// Register the metrics.
	prometheus.MustRegister(m.RunningFPCounter)
	prometheus.MustRegister(m.StoppedFPCounter)
	prometheus.MustRegister(m.CreatedFPCounter)
	prometheus.MustRegister(m.RegisteredFPCounter)
	prometheus.MustRegister(m.ActiveFPCounter)
	prometheus.MustRegister(m.InactiveFPCounter)
	prometheus.MustRegister(m.SlashedFPCounter)

	return m
}
