package babylonclient

import (
	bbncfg "github.com/babylonchain/rpc-client/config"
	"github.com/babylonchain/rpc-client/query"
	"github.com/sirupsen/logrus"
	lensclient "github.com/strangelove-ventures/lens/client"
)

type BabylonController struct {
	*lensclient.ChainClient
	*query.QueryClient
	cfg    *bbncfg.BabylonConfig
	logger *logrus.Logger
}

func NewBabylonController(
	cfg *bbncfg.BabylonConfig,
	logger *logrus.Logger,
) (*BabylonController, error) {
	// TODO should be validated earlier
	if err := babylonConfig.Validate(); err != nil {
		return nil, err
	}

	// create a Tendermint/Cosmos client for Babylon
	cc, err := newLensClient(babylonConfig.Unwrap())
	if err != nil {
		return nil, err
	}

	// create a queryClient so that the Client inherits all query functions
	queryClient, err := query.NewWithClient(cc.RPCClient, cfg.Timeout)
	if err != nil {
		return nil, err
	}

	// wrap to our type
	client := &BabylonController{
		cc,
		queryClient,
		cfg,
		btcParams,
		logger,
	}

	return client, nil
}
