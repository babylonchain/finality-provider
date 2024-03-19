package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

type EotsMetrics struct {
	EotsCreatedKeysCounter                prometheus.Counter
	EotsFpTotalGeneratedRandomnessCounter *prometheus.CounterVec
	EotsFpLastGeneratedRandomnessHeight   prometheus.Gauge
	EotsFpTotalEotsSignCounter            *prometheus.CounterVec
	EotsFpLastEotsSignHeight              prometheus.Gauge
	EotsFpTotalSchnorrSignCounter         *prometheus.CounterVec
}

var eotsMetricsRegisterOnce sync.Once

var eotsMetricsInstance *EotsMetrics

func NewEotsMetrics() *EotsMetrics {
	eotsMetricsRegisterOnce.Do(func() {
		eotsMetricsInstance = &EotsMetrics{
			EotsCreatedKeysCounter: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "eots_created_keys_counter",
				Help: "Total number of EOTS keys created",
			}),
			EotsFpTotalGeneratedRandomnessCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "eots_fp_total_generated_randomness_counter",
					Help: "Total number of generated randomness pairs by EOTS",
				},
				[]string{"fp_btc_pk_hex"},
			),
			EotsFpLastGeneratedRandomnessHeight: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "eots_fp_last_generated_randomness_height",
				Help: "Height of the last generated randomness by EOTS",
			}),
			EotsFpTotalEotsSignCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "eots_fp_total_eots_sign_counter",
					Help: "Total number of EOTS signatures made",
				},
				[]string{"fp_btc_pk_hex"},
			),
			EotsFpLastEotsSignHeight: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "eots_fp_last_eots_sign_height",
				Help: "Height of the last EOTS signature made",
			}),
			EotsFpTotalSchnorrSignCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "eots_fp_total_schnorr_sign_counter",
					Help: "Total number of Schnorr signatures made by EOTS",
				},
				[]string{"fp_btc_pk_hex"},
			),
		}

		// Register the EOTS metrics with Prometheus
		prometheus.MustRegister(eotsMetricsInstance.EotsFpTotalGeneratedRandomnessCounter)
		prometheus.MustRegister(eotsMetricsInstance.EotsFpLastGeneratedRandomnessHeight)
		prometheus.MustRegister(eotsMetricsInstance.EotsFpTotalEotsSignCounter)
		prometheus.MustRegister(eotsMetricsInstance.EotsFpLastEotsSignHeight)
		prometheus.MustRegister(eotsMetricsInstance.EotsFpTotalSchnorrSignCounter)
	})

	return eotsMetricsInstance
}
