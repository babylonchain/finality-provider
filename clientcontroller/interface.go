package clientcontroller

import (
	"fmt"

	"cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"go.uber.org/zap"

	cosmwasmcfg "github.com/babylonchain/finality-provider/cosmwasmclient/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
)

const (
	BabylonConsumerChainName = "babylon"
	EVMConsumerChainName     = "evm"
	WasmdConsumerChainName   = "wasmd"
)

type ClientController interface {

	// RegisterFinalityProvider registers a finality provider to the consumer chain
	// it returns tx hash and error
	RegisterFinalityProvider(
		chainID string,
		chainPk []byte,
		fpPk *btcec.PublicKey,
		pop []byte,
		commission *math.LegacyDec,
		description []byte,
	) (*types.TxResponse, error)

	// Note: the following queries are only for PoC

	// QueryFinalityProviderSlashed queries if the finality provider is slashed
	// Note: if the FP wants to get the information from the consumer chain directly, they should add this interface
	// function in ConsumerController. (https://github.com/babylonchain/finality-provider/pull/335#discussion_r1606175344)
	QueryFinalityProviderSlashed(fpPk *btcec.PublicKey) (bool, error)

	Close() error
}

// NewClientController TODO: this is always going to be babylon so rename accordingly
func NewClientController(config *fpcfg.Config, logger *zap.Logger) (ClientController, error) {
	cc, err := NewBabylonController(config.BabylonConfig, &config.BTCNetParams, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon rpc client: %w", err)
	}

	return cc, err
}

type ConsumerController interface {
	// CommitPubRandList commits a list of EOTS public randomness the consumer chain
	// it returns tx hash and error
	CommitPubRandList(fpPk *btcec.PublicKey, startHeight uint64, numPubRand uint64, commitment []byte, sig *schnorr.Signature) (*types.TxResponse, error)

	// SubmitFinalitySig submits the finality signature to the consumer chain
	SubmitFinalitySig(fpPk *btcec.PublicKey, block *types.BlockInfo, pubRand *btcec.FieldVal, proof []byte, sig *btcec.ModNScalar) (*types.TxResponse, error)

	// SubmitBatchFinalitySigs submits a batch of finality signatures to the consumer chain
	SubmitBatchFinalitySigs(fpPk *btcec.PublicKey, blocks []*types.BlockInfo, pubRandList []*btcec.FieldVal, proofList [][]byte, sigs []*btcec.ModNScalar) (*types.TxResponse, error)

	// Note: the following queries are only for PoC

	// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
	QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error)

	// QueryLatestFinalizedBlock returns the latest finalized block
	// Note: nil will be returned if the finalized block does not exist
	QueryLatestFinalizedBlock() (*types.BlockInfo, error)

	// QueryLastCommittedPublicRand returns the last committed public randomness
	QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*types.PubRandCommit, error)

	// QueryBlock queries the block at the given height
	QueryBlock(height uint64) (*types.BlockInfo, error)

	// QueryIsBlockFinalized queries if the block at the given height is finalized
	QueryIsBlockFinalized(height uint64) (bool, error)

	// QueryBlocks returns a list of blocks from startHeight to endHeight
	QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error)

	// QueryLatestBlockHeight queries the tip block height of the consumer chain
	QueryLatestBlockHeight() (uint64, error)

	// QueryActivatedHeight returns the activated height of the consumer chain
	// error will be returned if the consumer chain has not been activated
	QueryActivatedHeight() (uint64, error)

	Close() error
}

func NewConsumerController(config *fpcfg.Config, logger *zap.Logger) (ConsumerController, error) {
	var (
		ccc ConsumerController
		err error
	)

	switch config.ChainName {
	case BabylonConsumerChainName:
		ccc, err = NewBabylonConsumerController(config.BabylonConfig, &config.BTCNetParams, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Babylon rpc client: %w", err)
		}
	case EVMConsumerChainName:
		ccc, err = NewEVMConsumerController(config.EVMConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create EVM rpc client: %w", err)
		}
	case WasmdConsumerChainName:
		wasmdEncodingCfg := cosmwasmcfg.GetWasmdEncodingConfig()
		ccc, err = NewCosmwasmConsumerController(config.CosmwasmConfig, wasmdEncodingCfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Wasmd rpc client: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported consumer chain")
	}

	return ccc, err
}
