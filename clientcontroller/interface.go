package clientcontroller

import (
	"fmt"

	bbntypes "github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdkTypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/types"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	babylonConsumerChainName = "babylon"
)

type StakingParams struct {
	// K-deep
	ComfirmationTimeBlocks uint64
	// W-deep
	FinalizationTimeoutBlocks uint64

	// Minimum amount of satoshis required for slashing transaction
	MinSlashingTxFeeSat btcutil.Amount

	// Bitcoin public key of the current jury
	JuryPk *btcec.PublicKey

	// Address to which slashing transactions are sent
	SlashingAddress string

	// Minimum commission required by babylon
	MinCommissionRate sdkTypes.Dec
}

// TODO replace babylon types with general ones
type ClientController interface {
	GetStakingParams() (*StakingParams, error)
	// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
	// it returns tx hash and error
	RegisterValidator(
		bbnPubKey *secp256k1.PubKey,
		btcPubKey *bbntypes.BIP340PubKey,
		pop *btcstakingtypes.ProofOfPossession,
		commission *sdkTypes.Dec,
		description *stakingtypes.Description,
	) (*provider.RelayerTxResponse, error)
	// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
	// it returns tx hash and error
	CommitPubRandList(btcPubKey *bbntypes.BIP340PubKey, startHeight uint64, pubRandList []bbntypes.SchnorrPubRand, sig *bbntypes.BIP340Signature) (*provider.RelayerTxResponse, error)
	// SubmitJurySig submits the Jury signature via a MsgAddJurySig to Babylon if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJurySig(btcPubKey *bbntypes.BIP340PubKey, delPubKey *bbntypes.BIP340PubKey, stakingTxHash string, sig *bbntypes.BIP340Signature) (*provider.RelayerTxResponse, error)

	// SubmitJuryUnbondingSigs submits the Jury signatures via a MsgAddJuryUnbondingSigs to Babylon if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJuryUnbondingSigs(
		btcPubKey *bbntypes.BIP340PubKey,
		delPubKey *bbntypes.BIP340PubKey,
		stakingTxHash string,
		unbondingSig *bbntypes.BIP340Signature,
		slashUnbondingSig *bbntypes.BIP340Signature,
	) (*provider.RelayerTxResponse, error)

	// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
	SubmitFinalitySig(btcPubKey *bbntypes.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *bbntypes.SchnorrEOTSSig) (*provider.RelayerTxResponse, error)
	// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
	SubmitBatchFinalitySigs(btcPubKey *bbntypes.BIP340PubKey, blocks []*types.BlockInfo, sigs []*bbntypes.SchnorrEOTSSig) (*provider.RelayerTxResponse, error)

	// SubmitValidatorUnbondingSig submits the validator signature for unbonding transaction
	SubmitValidatorUnbondingSig(
		valPubKey *bbntypes.BIP340PubKey,
		delPubKey *bbntypes.BIP340PubKey,
		stakingTxHash string,
		sig *bbntypes.BIP340Signature) (*provider.RelayerTxResponse, error)

	// Note: the following queries are only for PoC

	// QueryHeightWithLastPubRand queries the height of the last block with public randomness
	QueryHeightWithLastPubRand(btcPubKey *bbntypes.BIP340PubKey) (uint64, error)
	// QueryPendingBTCDelegations queries BTC delegations that need a Jury signature
	// it is only used when the program is running in Jury mode
	QueryPendingBTCDelegations() ([]*btcstakingtypes.BTCDelegation, error)

	// QueryUnbondindBTCDelegations queries BTC delegations that need a Jury sig for unbodning
	// it is only used when the program is running in Jury mode
	QueryUnbondindBTCDelegations() ([]*btcstakingtypes.BTCDelegation, error)

	// QueryValidatorVotingPower queries the voting power of the validator at a given height
	QueryValidatorVotingPower(btcPubKey *bbntypes.BIP340PubKey, blockHeight uint64) (uint64, error)
	// QueryLatestFinalizedBlocks returns the latest finalized blocks
	QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error)
	// QueryBlocks returns a list of blocks from startHeight to endHeight
	QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error)
	// QueryBlockFinalization queries whether the block has been finalized
	QueryBlockFinalization(height uint64) (bool, error)

	// QueryBestHeader queries the tip header of the Babylon chain, if header is not found
	// it returns result with nil header
	QueryBestHeader() (*ctypes.ResultHeader, error)
	// QueryNodeStatus returns current node status, with info about latest block
	QueryNodeStatus() (*ctypes.ResultStatus, error)
	// QueryHeader queries the header at the given height, if header is not found
	// it returns result with nil header
	QueryHeader(height int64) (*ctypes.ResultHeader, error)

	// QueryBTCValidatorUnbondingDelegations queries the unbonding delegations.UnbondingDelegations:
	// - already received unbodning transaction on babylon chain
	// - not received validator signature yet
	QueryBTCValidatorUnbondingDelegations(valBtcPk *bbntypes.BIP340PubKey, max uint64) ([]*btcstakingtypes.BTCDelegation, error)

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
