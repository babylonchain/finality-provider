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

// TODO: rename the file name, class name and etc
// This is not a simple EVM chain. It's a OP Stack L2 chain, which has many
// implications. So we should rename to sth like e.g. OPStackL2Consumer
// This helps distinguish from pure EVM sidechains e.g. Binance Chain
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

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (ec *EVMConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	/* TODO: implement

	latest_committed_l2_height = read `latestBlockNumber()` from the L1 L2OutputOracle contract and return the result

	if blockHeight > latest_committed_l2_height:

		query the VP from the L1 oracle contract using "latest" as the block tag

	else:

		1. query the L1 event `emit OutputProposed(_outputRoot, nextOutputIndex(), _l2BlockNumber, block.timestamp, block.number);`
		  to find the first event where the `_l2BlockNumber` >= blockHeight
		2. get the block.number from the event
		3. query the VP from the L1 oracle contract using `block.number` as the block tag

	*/

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
	/* TODO: implement

		oracle_event = query the event in the L1 oracle contract where the FP's voting power is firstly set

		l1_activated_height = get the L1 block number from the `oracle_event`

	  output_event = query the L1 event `emit OutputProposed(_outputRoot, nextOutputIndex(), _l2BlockNumber, block.timestamp, block.number);`
				to find the first event where the `block.number` >= l1_activated_height

		if output_event == nil:

				read `nextBlockNumber()` from the L1 L2OutputOracle contract and return the result

		else:

				return output_event._l2BlockNumber

	*/

	return 0, nil
}

func (ec *EVMConsumerController) QueryBestBlock() (*types.BlockInfo, error) {
	/* TODO: implement
	get the latest L2 block number from a RPC call
	*/

	return &types.BlockInfo{
		Height: uint64(0),
		Hash:   nil,
	}, nil
}

func (ec *EVMConsumerController) Close() error {
	ec.evmClient.Close()
	return nil
}
