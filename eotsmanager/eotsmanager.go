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

	// RegisterChain registers a new chain to the validator where the chainID
	// is an arbitrary-length public ASCII byte string identifying the target
	// chain, e.g., "babylonchain-testnet-alpha"
	// It fails if the validator does not exist or the chain has been registered
	// by this validator
	RegisterChain(uid string, chainID string) error

	// SignEOTS signs an EOTS using the private key of the validator and the corresponding
	// secret randomness of the give chain at the given height
	// It fails if the validator does not exist or the chain has not been registered
	// by the validator
	SignEOTS(uid string, chainID string, msg []byte, height uint64) (*types.SchnorrEOTSSig, error)

	// SignSchnorrSig signs a Schnorr signature using the private key of the validator
	// It fails if the validator does not exist
	SignSchnorrSig(uid string, msg []byte, height uint64) (*schnorr.Signature, error)

	// GetPublicRandomness returns a list of public randomness from startingHeight to
	// startingHeight+(num-1)*step where step means the gap between each block height
	// that the validator wants to finalize and num means the number of public randomness
	// It fails if the validator does not exist or the chain has not been registered
	// by the validator
	GetPublicRandomness(uid, chainID string, startingHeight uint64, step int, num int) ([]*types.SchnorrPubRand, error)
}
