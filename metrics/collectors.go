package metrics

import (
	"github.com/babylonchain/finality-provider/finality-provider/proto"
	"github.com/babylonchain/finality-provider/finality-provider/store"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

type Metrics struct {
	// all finality provider metrics
	runningFpGauge prometheus.Gauge
	// poller metrics
	babylonTipHeight     prometheus.Gauge
	lastPolledHeight     prometheus.Gauge
	pollerStartingHeight prometheus.Gauge
	// single finality provider metrics
	fpStatus                     *prometheus.GaugeVec
	fpSecondsSinceLastVote       *prometheus.GaugeVec
	fpSecondsSinceLastRandomness *prometheus.GaugeVec
	fpLastVotedHeight            *prometheus.GaugeVec
	fpLastProcessedHeight        *prometheus.GaugeVec
	// time keeper
	mu                     sync.Mutex
	previousVoteByFp       map[string]*time.Time
	previousRandomnessByFp map[string]*time.Time
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
			fpSecondsSinceLastVote: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "fp_seconds_since_last_vote",
					Help: "Seconds since the last finality sig vote by a finality provider.",
				},
				[]string{"fp_btc_pk_hex"},
			),
			fpSecondsSinceLastRandomness: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "fp_seconds_since_last_randomness",
					Help: "Seconds since the last public randomness commitment by a finality provider.",
				},
				[]string{"fp_btc_pk_hex"},
			),
			mu: sync.Mutex{},
		}

		// Register the metrics with Prometheus
		prometheus.MustRegister(metricsInstance.runningFpGauge)
		prometheus.MustRegister(metricsInstance.fpStatus)
		prometheus.MustRegister(metricsInstance.babylonTipHeight)
		prometheus.MustRegister(metricsInstance.lastPolledHeight)
		prometheus.MustRegister(metricsInstance.pollerStartingHeight)
		prometheus.MustRegister(metricsInstance.fpSecondsSinceLastVote)
		prometheus.MustRegister(metricsInstance.fpSecondsSinceLastRandomness)
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

func (m *Metrics) RecordFpSecondsSinceLastVote(fpBtcPkHex string, seconds float64) {
	m.fpSecondsSinceLastVote.WithLabelValues(fpBtcPkHex).Set(seconds)
}

func (m *Metrics) RecordFpSecondsSinceLastRandomness(fpBtcPkHex string, seconds float64) {
	m.fpSecondsSinceLastRandomness.WithLabelValues(fpBtcPkHex).Set(seconds)
}

func (m *Metrics) RecordFpLastVotedHeight(fpBtcPkHex string, height uint64) {
	m.fpLastVotedHeight.WithLabelValues(fpBtcPkHex).Set(float64(height))
}

func (m *Metrics) RecordFpLastProcessedHeight(fpBtcPkHex string, height uint64) {
	m.fpLastProcessedHeight.WithLabelValues(fpBtcPkHex).Set(float64(height))
}

func (m *Metrics) RecordFpVoteTime(fpBtcPkHex string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	if m.previousVoteByFp == nil {
		m.previousVoteByFp = make(map[string]*time.Time)
	}
	m.previousVoteByFp[fpBtcPkHex] = &now
}

func (m *Metrics) RecordFpRandomnessTime(fpBtcPkHex string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	if m.previousRandomnessByFp == nil {
		m.previousRandomnessByFp = make(map[string]*time.Time)
	}
	m.previousRandomnessByFp[fpBtcPkHex] = &now
}

func (m *Metrics) UpdateFpMetrics(fps []*store.StoredFinalityProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, fp := range fps {
		m.RecordFpStatus(fp.GetBIP340BTCPK().MarshalHex(), fp.Status)

		if lastVoteTime, ok := m.previousVoteByFp[fp.GetBIP340BTCPK().MarshalHex()]; ok {
			m.RecordFpSecondsSinceLastVote(fp.GetBIP340BTCPK().MarshalHex(), time.Since(*lastVoteTime).Seconds())
		}

		if lastRandomnessTime, ok := m.previousRandomnessByFp[fp.GetBIP340BTCPK().MarshalHex()]; ok {
			m.RecordFpSecondsSinceLastRandomness(fp.GetBIP340BTCPK().MarshalHex(), time.Since(*lastRandomnessTime).Seconds())
		}
	}
}
