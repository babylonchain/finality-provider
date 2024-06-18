package config

import (
	"fmt"
)

type OPStackL2Config struct {
	GRPCAddress             string `long:"grpc-address" description:"address of the grpc server(e.g.Babylon) to connect to"`
	OPFinalityGadgetAddress string `long:"op-finality-gadget" description:"the contract address of the op-finality-gadget"`
}

func (cfg *OPStackL2Config) Validate() error {
	if cfg.OPFinalityGadgetAddress == "" {
		return fmt.Errorf("op-finality-gadget contract address not specified")
	}
	return nil
}
