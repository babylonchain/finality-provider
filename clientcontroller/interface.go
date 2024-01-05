package clientcontroller

import (
	"fmt"

	"cosmossdk.io/math"
	"github.com/btcsuite/btcd/chaincfg"
	"go.uber.org/zap"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"github.com/babylonchain/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
)

const (
	babylonConsumerChainName = "babylon"
)

type ClientController interface {

	// RegisterFinalityProvider registers a finality provider to the consumer chain
	// it returns tx hash and error
	RegisterFinalityProvider(
		chainPk []byte,
		fpPk *btcec.PublicKey,
		pop []byte,
		commission *math.LegacyDec,
		description []byte,
	) (*types.TxResponse, error)

	// CommitPubRandList commits a list of EOTS public randomness the consumer chain
	// it returns tx hash and error
	CommitPubRandList(fpPk *btcec.PublicKey, startHeight uint64, pubRandList []*btcec.FieldVal, sig *schnorr.Signature) (*types.TxResponse, error)

	// SubmitFinalitySig submits the finality signature to the consumer chain
	SubmitFinalitySig(fpPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error)

	// SubmitBatchFinalitySigs submits a batch of finality signatures to the consumer chain
	SubmitBatchFinalitySigs(fpPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error)

	// Note: the following queries are only for PoC

	// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
	QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error)

	// QueryFinalityProviderSlashed queries if the finality provider is slashed
	QueryFinalityProviderSlashed(fpPk *btcec.PublicKey) (bool, error)

	// QueryLatestFinalizedBlocks returns the latest finalized blocks
	QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error)

	// QueryLastCommittedPublicRand returns the last committed public randomness
	QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*btcec.FieldVal, error)

	// QueryBlock queries the block at the given height
	QueryBlock(height uint64) (*types.BlockInfo, error)

	// QueryBlocks returns a list of blocks from startHeight to endHeight
	QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error)

	// QueryBestBlock queries the tip block of the consumer chain
	QueryBestBlock() (*types.BlockInfo, error)

	// QueryActivatedHeight returns the activated height of the consumer chain
	// error will be returned if the consumer chain has not been activated
	QueryActivatedHeight() (uint64, error)

	Close() error
}

func NewClientController(chainName string, bbnConfig *config.BBNConfig, netParams *chaincfg.Params, logger *zap.Logger) (ClientController, error) {
	var (
		cc  ClientController
		err error
	)
	switch chainName {
	case babylonConsumerChainName:
		cc, err = NewBabylonController(bbnConfig, netParams, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Babylon rpc client: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported consumer chain")
	}

	return cc, err
}
