package babylonclient

import (
	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

type BabylonClient interface {
	// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
	// it returns tx hash and error
	RegisterValidator(bbnPubKey *secp256k1.PubKey, btcPubKey *types.BIP340PubKey, pop *btcstakingtypes.ProofOfPossession) ([]byte, error)
	// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
	// it returns tx hash and error
	CommitPubRandList(btcPubKey *types.BIP340PubKey, startHeight uint64, pubRandList []*types.SchnorrPubRand, sig *types.BIP340Signature) ([]byte, error)
	// SubmitJurySig submits the Jury signature via a MsgAddJurySig to Babylon if the daemon runs in Jury mode
	// it returns tx hash and error
	SubmitJurySig(btcPubKey *types.BIP340PubKey, delPubKey *types.BIP340PubKey, sig *types.BIP340Signature) ([]byte, error)
	// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
	SubmitFinalitySig(btcPubKey *types.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *types.SchnorrEOTSSig) ([]byte, error)

	// Note: the following queries are only for PoC
	// TODO is Babylon ready for such quereis?
	// ShouldValidatorCommitPubRand asks Babylon if the validator's public randomness used out
	// it also returns the start height
	ShouldValidatorCommitPubRand(btcPubKey *types.BIP340PubKey) (bool, uint64, error)
	// ShouldSubmitJurySig asks Babylon if the Jury should submit a Jury sig
	// it is only used when the program is running in Jury mode
	// it also returns the public key used in the delegation
	ShouldSubmitJurySig(btcPubKey *types.BIP340PubKey) (bool, *types.BIP340PubKey, error)
	// ShouldValidatorVote asks Babylon if the validator should submit a finality sig for the given block height
	ShouldValidatorVote(btcPubKey *types.BIP340PubKey, blockHeight uint64) (bool, error)
}
