package eotsmanager

import (
	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

type EOTSManager interface {
	// CreateValidator generates a key pair to create a unique validator object
	// and persists it in storage. The key pair is formatted by BIP-340 (Schnorr Signatures)
	// It fails if there is an existing key Info with the same name or public key.
	CreateValidator(name, passPhrase string) (*types.BIP340PubKey, error)

	// CreateRandomnessPairList generates and persists a list of Schnorr randomness pairs from
	// startHeight to startHeight+(num-1)*step where step means the gap between each block height
	// that the validator wants to finalize and num means the number of public randomness
	// It fails if the validator does not exist
	// NOTE: the same Schnorr randomness pair should not be used twice in a global view
	CreateRandomnessPairList(valPk *types.BIP340PubKey, chainID string, startingHeight uint64, step int, num int) ([]*types.SchnorrPubRand, error)

	// SignEOTS signs an EOTS using the private key of the validator and the corresponding
	// secret randomness of the give chain at the given height
	// It fails if the validator does not exist or the chain has not been registered
	// by the validator
	SignEOTS(valPk *types.BIP340PubKey, chainID string, msg []byte, height uint64) (*types.SchnorrEOTSSig, error)

	// SignSchnorrSig signs a Schnorr signature using the private key of the validator
	// It fails if the validator does not exist
	SignSchnorrSig(valPk *types.BIP340PubKey, msg []byte) (*schnorr.Signature, error)
}
