package clientcontroller

import (
	"fmt"

	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	"github.com/babylonchain/finality-provider/clientcontroller/api"
	"github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/clientcontroller/cosmwasm"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	cosmwasmcfg "github.com/babylonchain/finality-provider/cosmwasmclient/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"go.uber.org/zap"
)

const (
	BabylonConsumerChainName   = "babylon"
	OPStackL2ConsumerChainName = "OPStackL2"
	WasmdConsumerChainName     = "wasmd"
	BcdConsumerChainName       = "bcd"
)

// NewClientController TODO: this is always going to be babylon so rename accordingly
func NewClientController(config *fpcfg.Config, logger *zap.Logger) (api.ClientController, error) {
	cc, err := babylon.NewBabylonController(config.BabylonConfig, &config.BTCNetParams, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon rpc client: %w", err)
	}

	return cc, err
}

func NewConsumerController(config *fpcfg.Config, logger *zap.Logger) (api.ConsumerController, error) {
	var (
		ccc api.ConsumerController
		err error
	)

	switch config.ChainName {
	case BabylonConsumerChainName:
		ccc, err = babylon.NewBabylonConsumerController(config.BabylonConfig, &config.BTCNetParams, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Babylon consumer client: %w", err)
		}
	case OPStackL2ConsumerChainName:
		ccc, err = opstackl2.NewOPStackL2ConsumerController(config.OPStackL2Config, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create OPStack L2 consumer client: %w", err)
		}
	case WasmdConsumerChainName:
		wasmdEncodingCfg := cosmwasmcfg.GetWasmdEncodingConfig()
		ccc, err = cosmwasm.NewCosmwasmConsumerController(config.CosmwasmConfig, wasmdEncodingCfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Wasmd consumer client: %w", err)
		}
	case BcdConsumerChainName:
		bcdEncodingCfg := cosmwasmcfg.GetBcdEncodingConfig()
		wasmdEncodingCfg := wasmdparams.EncodingConfig{
			InterfaceRegistry: bcdEncodingCfg.InterfaceRegistry,
			Codec:             bcdEncodingCfg.Codec,
			TxConfig:          bcdEncodingCfg.TxConfig,
			Amino:             bcdEncodingCfg.Amino,
		}
		ccc, err = cosmwasm.NewCosmwasmConsumerController(config.CosmwasmConfig, wasmdEncodingCfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Bcd consumer client: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported consumer chain")
	}

	return ccc, err
}
