package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type EotsMetrics struct {
	EotsCreatedKeysCounter        prometheus.Counter
	EotsFpTotalEotsSignCounter    *prometheus.CounterVec
	EotsFpLastEotsSignHeight      *prometheus.GaugeVec
	EotsFpTotalSchnorrSignCounter *prometheus.CounterVec
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
			EotsFpTotalEotsSignCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "eots_fp_total_eots_sign_counter",
					Help: "Total number of EOTS signatures made",
				},
				[]string{"fp_btc_pk_hex"},
			),
			EotsFpLastEotsSignHeight: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "eots_fp_last_eots_sign_height",
					Help: "Height of the last EOTS signature made",
				},
				[]string{"fp_btc_pk_hex"},
			),
			EotsFpTotalSchnorrSignCounter: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "eots_fp_total_schnorr_sign_counter",
					Help: "Total number of Schnorr signatures made by EOTS",
				},
				[]string{"fp_btc_pk_hex"},
			),
		}

		// Register the EOTS metrics with Prometheus
		prometheus.MustRegister(eotsMetricsInstance.EotsCreatedKeysCounter)
		prometheus.MustRegister(eotsMetricsInstance.EotsFpTotalEotsSignCounter)
		prometheus.MustRegister(eotsMetricsInstance.EotsFpLastEotsSignHeight)
		prometheus.MustRegister(eotsMetricsInstance.EotsFpTotalSchnorrSignCounter)
	})

	return eotsMetricsInstance
}

// IncrementEotsCreatedKeysCounter increments the EOTS created keys counter
func (em *EotsMetrics) IncrementEotsCreatedKeysCounter() {
	em.EotsCreatedKeysCounter.Inc()
}

// IncrementEotsFpTotalEotsSignCounter increments the EOTS signature counter
func (em *EotsMetrics) IncrementEotsFpTotalEotsSignCounter(fpBtcPkHex string) {
	em.EotsFpTotalEotsSignCounter.WithLabelValues(fpBtcPkHex).Inc()
}

// SetEotsFpLastEotsSignHeight sets the height of the last EOTS signature made
func (em *EotsMetrics) SetEotsFpLastEotsSignHeight(fpBtcPkHex string, height float64) {
	em.EotsFpLastEotsSignHeight.WithLabelValues(fpBtcPkHex).Set(height)
}

// IncrementEotsFpTotalSchnorrSignCounter increments the EOTS Schnorr signature counter
func (em *EotsMetrics) IncrementEotsFpTotalSchnorrSignCounter(fpBtcPkHex string) {
	em.EotsFpTotalSchnorrSignCounter.WithLabelValues(fpBtcPkHex).Inc()
}
