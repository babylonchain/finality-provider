package babylonclient

import (
	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

type BabylonClient interface {
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
	// QueryShouldSubmitJurySigs queries BTC delegations that need a Jury signature
	// it is only used when the program is running in Jury mode
	QueryPendingBTCDelegations() ([]*btcstakingtypes.BTCDelegation, error)
	// QueryShouldValidatorVote asks Babylon if the validator should submit a finality sig for the given block height
	QueryShouldValidatorVote(btcPubKey *types.BIP340PubKey, blockHeight uint64) (bool, error)

	// QueryNodeStatus returns current node status, with info about latest block
	QueryNodeStatus() (*ctypes.ResultStatus, error)

	// QueryHeader queries the header at the given height, if header is not found
	// it returns result with nil header
	QueryHeader(height int64) (*ctypes.ResultHeader, error)

	// QueryBestHeader queries the tip header of the Babylon chain, if header is not found
	// it returns result with nil header
	QueryBestHeader() (*ctypes.ResultHeader, error)
}
