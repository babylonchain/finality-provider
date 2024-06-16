package client

import (
	"context"
	"fmt"
	"time"

	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/babylonchain/finality-provider/cosmwasmclient/config"
	"github.com/babylonchain/finality-provider/cosmwasmclient/query"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	*query.QueryClient
	// MsgClient is the client API for Msg service
	msgClient wasmtypes.MsgClient
	provider  *cosmos.CosmosProvider
	timeout   time.Duration
	logger    *zap.Logger
	cfg       *config.CosmwasmConfig
}

func New(cfg *config.CosmwasmConfig, chainName string, encodingCfg wasmdparams.EncodingConfig, logger *zap.Logger) (*Client, error) {
	var (
		zapLogger *zap.Logger
		err       error
	)

	// ensure cfg is valid
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// use the existing logger or create a new one if not given
	zapLogger = logger
	if zapLogger == nil {
		zapLogger, err = newRootLogger("console", true)
		if err != nil {
			return nil, err
		}
	}

	// establish grpc connection
	grpcConn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to build gRPC connection to %s: %w", cfg.GRPCAddr, err)
	}
	msgClient := wasmtypes.NewMsgClient(grpcConn)

	provider, err := cfg.ToCosmosProviderConfig().NewProvider(
		zapLogger,
		"", // TODO: set home path
		true,
		chainName,
	)
	if err != nil {
		return nil, err
	}

	cp := provider.(*cosmos.CosmosProvider)
	cp.PCfg.KeyDirectory = cfg.KeyDirectory
	cp.Cdc = cosmos.Codec{
		InterfaceRegistry: encodingCfg.InterfaceRegistry,
		Marshaler:         encodingCfg.Codec,
		TxConfig:          encodingCfg.TxConfig,
		Amino:             encodingCfg.Amino,
	}

	// initialise Cosmos provider
	// NOTE: this will create a RPC client. The RPC client will be used for
	// submitting txs and making ad hoc queries. It won't create WebSocket
	// connection with wasmd node
	err = cp.Init(context.Background())
	if err != nil {
		return nil, err
	}

	// create a queryClient so that the Client inherits all query functions
	// TODO: merge this RPC client with the one in `cp` after Cosmos side
	// finishes the migration to new RPC client
	// see https://github.com/strangelove-ventures/cometbft-client
	c, err := rpchttp.NewWithTimeout(cp.PCfg.RPCAddr, "/websocket", uint(cfg.Timeout.Seconds()))
	if err != nil {
		return nil, err
	}
	queryClient, err := query.NewWithClient(c, cfg.Timeout)
	if err != nil {
		return nil, err
	}

	return &Client{
		queryClient,
		msgClient,
		cp,
		cfg.Timeout,
		zapLogger,
		cfg,
	}, nil
}

func (c *Client) GetConfig() *config.CosmwasmConfig {
	return c.cfg
}
