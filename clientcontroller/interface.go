package clientcontroller

import (
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	babylonConsumerChainName = "babylon"
)

type ClientController interface {
	ValidatorAPIs

	CovenantAPIs

	QueryStakingParams() (*types.StakingParams, error)

	Close() error
}

// ValidatorAPIs contains interfaces needed when the program is running in the validator mode
type ValidatorAPIs interface {
	// RegisterValidator registers a BTC validator to the consumer chain
	// it returns tx hash and error
	RegisterValidator(
		chainPk []byte,
		valPk *btcec.PublicKey,
		pop []byte,
		commission *big.Int,
		description []byte,
	) (*types.TxResponse, error)

	// CommitPubRandList commits a list of EOTS public randomness the consumer chain
	// it returns tx hash and error
	CommitPubRandList(valPk *btcec.PublicKey, startHeight uint64, pubRandList []*btcec.FieldVal, sig *schnorr.Signature) (*types.TxResponse, error)

	// SubmitFinalitySig submits the finality signature to the consumer chain
	SubmitFinalitySig(valPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error)

	// SubmitBatchFinalitySigs submits a batch of finality signatures to the consumer chain
	SubmitBatchFinalitySigs(valPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error)

	// Note: the following queries are only for PoC

	// QueryValidatorVotingPower queries the voting power of the validator at a given height
	QueryValidatorVotingPower(valPk *btcec.PublicKey, blockHeight uint64) (uint64, error)

	// QueryValidatorSlashed queries if the validator is slashed
	QueryValidatorSlashed(valPk *btcec.PublicKey) (bool, error)

	// QueryLatestFinalizedBlocks returns the latest finalized blocks
	QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error)

	// QueryBlock queries the block at the given height
	QueryBlock(height uint64) (*types.BlockInfo, error)

	// QueryBlocks returns a list of blocks from startHeight to endHeight
	QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error)

	// QueryBestBlock queries the tip block of the consumer chain
	QueryBestBlock() (*types.BlockInfo, error)

	// QueryActivatedHeight returns the activated height of the consumer chain
	// error will be returned if the consumer chain has not been activated
	QueryActivatedHeight() (uint64, error)
}

// CovenantAPIs contains interfaces needed when the program is running in the covenant mode
type CovenantAPIs interface {
	// SubmitCovenantSigs submits Covenant signatures to the consumer chain, each corresponding to
	// a validator that the delegation is (re-)staked to
	// it returns tx hash and error
	SubmitCovenantSigs(covPk *btcec.PublicKey, stakingTxHash string, sigs [][]byte) (*types.TxResponse, error)

	// SubmitCovenantUnbondingSigs submits the Covenant signatures for undelegation to the consumer chain
	// it returns tx hash and error
	SubmitCovenantUnbondingSigs(
		covPk *btcec.PublicKey,
		stakingTxHash string,
		unbondingSig *schnorr.Signature,
		slashUnbondingSigs [][]byte,
	) (*types.TxResponse, error)

	// QueryPendingDelegations queries BTC delegations that are in status of pending
	QueryPendingDelegations(limit uint64) ([]*types.Delegation, error)

	// QueryUnbondingDelegations queries BTC delegations that are in status of unbonding
	QueryUnbondingDelegations(limit uint64) ([]*types.Delegation, error)
}

func NewClientController(cfg *valcfg.Config, logger *logrus.Logger) (ClientController, error) {
	var (
		cc  ClientController
		err error
	)
	switch cfg.ChainName {
	case babylonConsumerChainName:
		cc, err = NewBabylonController(cfg.BabylonConfig, &cfg.ActiveNetParams, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Babylon rpc client: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported consumer chain")
	}

	return cc, err
}
