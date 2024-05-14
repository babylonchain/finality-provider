package clientcontroller

import (
	"fmt"

	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/btcsuite/btcd/btcec/v2"
	"go.uber.org/zap"
)

var _ ConsumerController = &EVMConsumerController{}

type EVMConsumerController struct {
	evmClient *rpc.Client
	cfg       *fpcfg.EVMConfig
	logger    *zap.Logger
}

func NewEVMConsumerController(
	evmCfg *fpcfg.EVMConfig,
	logger *zap.Logger,
) (*EVMConsumerController, error) {
	if err := evmCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for EVM RPC client: %w", err)
	}
	ec, err := rpc.Dial(evmCfg.RPCAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the EVM RPC server %s: %w", evmCfg.RPCAddr, err)
	}
	return &EVMConsumerController{
		ec,
		evmCfg,
		logger,
	}, nil
}

// SubmitFinalitySig submits the finality signature
func (ec *EVMConsumerController) SubmitFinalitySig(fpPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error) {

	return &types.TxResponse{TxHash: "", Events: nil}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (ec *EVMConsumerController) SubmitBatchFinalitySigs(fpPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}

	return &types.TxResponse{TxHash: "", Events: nil}, nil
}

func (ec *EVMConsumerController) QueryFinalityProviderSlashed(fpPk *btcec.PublicKey) (bool, error) {

	return false, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (ec *EVMConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {

	return 0, nil
}

func (ec *EVMConsumerController) QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error) {
	return ec.queryLatestBlocks(nil, count, finalitytypes.QueriedBlockStatus_FINALIZED, true)
}

func (ec *EVMConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {

	return ec.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), 0, finalitytypes.QueriedBlockStatus_ANY, false)
}

func (ec *EVMConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	var blocks []*types.BlockInfo

	return blocks, nil
}

func (ec *EVMConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {

	return &types.BlockInfo{
		Height:    height,
		Hash:      nil,
		Finalized: false,
	}, nil
}

func (ec *EVMConsumerController) QueryActivatedHeight() (uint64, error) {

	return 0, nil
}

func (ec *EVMConsumerController) QueryBestBlock() (*types.BlockInfo, error) {

	return &types.BlockInfo{
		Height: uint64(0),
		Hash:   nil,
	}, nil
}

func (ec *EVMConsumerController) Close() error {
	ec.evmClient.Close()
	return nil
}
