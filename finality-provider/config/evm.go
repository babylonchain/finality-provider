package config

import (
	"fmt"
)

type EVMConfig struct {
	OPFinalityGadgetAddress string `long:"op-finality-gadget" description:"the contract address of the op-finality-gadget"`
}

func (cfg *EVMConfig) Validate() error {
	if cfg.OPFinalityGadgetAddress == "" {
		return fmt.Errorf("op-finality-gadget contract address not specified")
	}
	return nil
}
