package clientcontroller

import (
	"fmt"

	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	babylonConsumerChainName = "babylon"
)

type ClientController interface {
	// RegisterValidator registers a BTC validator to the consumer chain
	// it returns tx hash and error
	RegisterValidator(
		chainPk []byte,
		valPk []byte,
		pop []byte,
		commission string,
		description string,
	) (*types.TxResponse, error)

	// CommitPubRandList commits a list of EOTS public randomness the consumer chain
	// it returns tx hash and error
	CommitPubRandList(valPk []byte, startHeight uint64, pubRandList [][]byte, sig []byte) (*types.TxResponse, error)

	// SubmitJurySig submits the Jury signature to the consumer chain if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJurySig(valPk []byte, delPk []byte, stakingTxHash string, sig []byte) (*types.TxResponse, error)

	// SubmitJuryUnbondingSigs submits the Jury signatures to the consumer chain if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJuryUnbondingSigs(
		valPk []byte,
		delPk []byte,
		stakingTxHash string,
		unbondingSig []byte,
		slashUnbondingSig []byte,
	) (*types.TxResponse, error)

	// SubmitFinalitySig submits the finality signature to the consumer chain
	SubmitFinalitySig(valPk []byte, blockHeight uint64, blockHash []byte, sig []byte) (*types.TxResponse, error)

	// SubmitBatchFinalitySigs submits a batch of finality signatures to the consumer chain
	SubmitBatchFinalitySigs(valPk []byte, blocks []*types.BlockInfo, sigs [][]byte) (*types.TxResponse, error)

	// SubmitValidatorUnbondingSig submits the validator signature for unbonding transaction to the consumer chain
	SubmitValidatorUnbondingSig(
		valPk []byte,
		delPk []byte,
		stakingTxHash string,
		sig []byte,
	) (*types.TxResponse, error)

	// Note: the following queries are only for PoC

	// QueryBTCDelegations queries BTC delegations with the given status
	// it is only used when the program is running in Jury mode
	QueryBTCDelegations(status types.DelegationStatus, limit uint64) ([]*btcstakingtypes.BTCDelegation, error)

	// QueryValidatorVotingPower queries the voting power of the validator at a given height
	QueryValidatorVotingPower(valPk []byte, blockHeight uint64) (uint64, error)

	// QueryValidatorSlashed queries if the validator is slashed
	QueryValidatorSlashed(valPk []byte) (bool, error)

	// QueryLatestFinalizedBlocks returns the latest finalized blocks
	QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error)

	// QueryBlock queries the block at the given height
	QueryBlock(height uint64) (*types.BlockInfo, error)

	// QueryBlocks returns a list of blocks from startHeight to endHeight
	QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error)

	// QueryBestBlock queries the tip block of the consumer chain
	QueryBestBlock() (*types.BlockInfo, error)

	// QueryBlockFinalization queries whether the block has been finalized
	QueryBlockFinalization(height uint64) (bool, error)

	// QueryBTCValidatorUnbondingDelegations queries the unbonding delegations.UnbondingDelegations:
	// - already received unbodning transaction on babylon chain
	// - not received validator signature yet
	QueryBTCValidatorUnbondingDelegations(valPk []byte, max uint64) ([]*btcstakingtypes.BTCDelegation, error)

	Close() error
}

func NewClientController(cfg *valcfg.Config, logger *logrus.Logger) (ClientController, error) {
	var (
		cc  ClientController
		err error
	)
	switch cfg.ChainName {
	case babylonConsumerChainName:
		cc, err = NewBabylonController(cfg.DataDir, cfg.BabylonConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Babylon rpc client: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported consumer chain")
	}

	return cc, err
}
