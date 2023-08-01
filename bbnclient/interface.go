package babylonclient

import (
	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

type StakingParams struct {
	// K-deep
	ComfirmationTimeBlocks uint32
	// W-deep
	FinalizationTimeoutBlocks uint32

	// Minimum amount of satoshis required for slashing transaction
	MinSlashingTxFeeSat btcutil.Amount

	// Bitcoin public key of the current jury
	JuryPk *btcec.PublicKey

	// Address to which slashing transactions are sent
	SlashingAddress string
}

type BabylonClient interface {
	GetStakingParams() (*StakingParams, error)
	// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
	// it returns tx hash and error
	RegisterValidator(bbnPubKey *secp256k1.PubKey, btcPubKey *types.BIP340PubKey, pop *btcstakingtypes.ProofOfPossession) ([]byte, error)
	// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
	// it returns tx hash and error
	CommitPubRandList(btcPubKey *types.BIP340PubKey, startHeight uint64, pubRandList []types.SchnorrPubRand, sig *types.BIP340Signature) ([]byte, error)
	// SubmitJurySig submits the Jury signature via a MsgAddJurySig to Babylon if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJurySig(btcPubKey *types.BIP340PubKey, delPubKey *types.BIP340PubKey, sig *types.BIP340Signature) ([]byte, error)
	// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
	SubmitFinalitySig(btcPubKey *types.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *types.SchnorrEOTSSig) ([]byte, error)

	// Note: the following queries are only for PoC

	// QueryHeightWithLastPubRand queries the height of the last block with public randomness
	QueryHeightWithLastPubRand(btcPubKey *types.BIP340PubKey) (uint64, error)
	// QueryPendingBTCDelegations queries BTC delegations that need a Jury signature
	// it is only used when the program is running in Jury mode
	QueryPendingBTCDelegations() ([]*btcstakingtypes.BTCDelegation, error)
	// QueryValidatorVotingPower queries the voting power of the validator at a given height
	QueryValidatorVotingPower(btcPubKey *types.BIP340PubKey, blockHeight uint64) (uint64, error)
	// QueryLatestFinalisedBlocks returns the latest `count` finalised blocks
	QueryLatestFinalisedBlocks(count uint64) ([]*finalitytypes.IndexedBlock, error)

	// QueryNodeStatus returns current node status, with info about latest block
	QueryNodeStatus() (*ctypes.ResultStatus, error)

	// QueryHeader queries the header at the given height, if header is not found
	// it returns result with nil header
	QueryHeader(height int64) (*ctypes.ResultHeader, error)

	// QueryBestHeader queries the tip header of the Babylon chain, if header is not found
	// it returns result with nil header
	QueryBestHeader() (*ctypes.ResultHeader, error)

	Close() error
}
