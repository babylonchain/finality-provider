package clientcontroller

import (
	"fmt"

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
	// RegisterValidator registers a BTC validator to the consumer chain
	// it returns tx hash and error
	RegisterValidator(
		chainPk []byte,
		valPk *btcec.PublicKey,
		pop []byte,
		commission string,
		description string,
	) (*types.TxResponse, error)

	// CommitPubRandList commits a list of EOTS public randomness the consumer chain
	// it returns tx hash and error
	CommitPubRandList(valPk *btcec.PublicKey, startHeight uint64, pubRandList [][]byte, sig *schnorr.Signature) (*types.TxResponse, error)

	// SubmitJurySig submits the Jury signature to the consumer chain if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJurySig(valPk *btcec.PublicKey, delPk *btcec.PublicKey, stakingTxHash string, sig *schnorr.Signature) (*types.TxResponse, error)

	// SubmitJuryUnbondingSigs submits the Jury signatures to the consumer chain if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJuryUnbondingSigs(
		valPk *btcec.PublicKey,
		delPk *btcec.PublicKey,
		stakingTxHash string,
		unbondingSig *schnorr.Signature,
		slashUnbondingSig *schnorr.Signature,
	) (*types.TxResponse, error)

	// SubmitFinalitySig submits the finality signature to the consumer chain
	SubmitFinalitySig(valPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error)

	// SubmitBatchFinalitySigs submits a batch of finality signatures to the consumer chain
	SubmitBatchFinalitySigs(valPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error)

	// SubmitValidatorUnbondingSig submits the validator signature for unbonding transaction to the consumer chain
	SubmitValidatorUnbondingSig(
		valPk *btcec.PublicKey,
		delPk *btcec.PublicKey,
		stakingTxHash string,
		sig *schnorr.Signature,
	) (*types.TxResponse, error)

	// Note: the following queries are only for PoC

	// QueryBTCDelegations queries BTC delegations with the given status
	// it is only used when the program is running in Jury mode
	QueryBTCDelegations(status types.DelegationStatus, limit uint64) ([]*types.Delegation, error)

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

	// QueryBTCValidatorUnbondingDelegations queries the unbonding delegations.UnbondingDelegations:
	// - already received unbodning transaction on babylon chain
	// - not received validator signature yet
	QueryBTCValidatorUnbondingDelegations(valPk *btcec.PublicKey, max uint64) ([]*types.Delegation, error)

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
