package config

import "fmt"

// VerifierConfig is used to configure the EOTS verifier
type VerifierConfig struct {
	RollupRPC  string `long:"rolluprpc" description:"The Rollup RPC URL."`
	EotsAggRPC string `long:"eotsaggrpc" description:"The EOTS aggregator RPC URL."`
	BabylonRPC string `long:"babylonrpc" description:"The Babylon RPC URL."`
}

func DefaultVerifierConfig() *VerifierConfig {
	return &VerifierConfig{
		RollupRPC:  "http://127.0.0.1:8545",
		EotsAggRPC: "http://127.0.0.1:9527",
		BabylonRPC: "http://127.0.0.1:26657",
	}
}

func (cfg *VerifierConfig) Validate() error {
	if cfg.RollupRPC == "" || cfg.EotsAggRPC == "" || cfg.BabylonRPC == "" {
		return fmt.Errorf("missing needed RPC URL for the verifier")
	}

	return nil
}
