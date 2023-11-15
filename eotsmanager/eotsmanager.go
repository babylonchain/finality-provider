package eotsmanager

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"github.com/babylonchain/btc-validator/eotsmanager/types"
)

type EOTSManager interface {
	// CreateKey generates a key pair at the given name and persists it in storage.
	// The key pair is formatted by BIP-340 (Schnorr Signatures)
	// It fails if there is an existing key Info with the same name or public key.
	CreateKey(name, passphrase, hdPath string) ([]byte, error)

	// CreateRandomnessPairList generates a list of Schnorr randomness pairs from
	// startHeight to startHeight+(num-1) where num means the number of public randomness
	// It fails if the validator does not exist or a randomness pair has been created before
	// NOTE: the randomness is deterministically generated based on the EOTS key, chainID and
	// block height
	CreateRandomnessPairList(uid []byte, chainID []byte, startHeight uint64, num uint32, passphrase string) ([]*btcec.FieldVal, error)

	// KeyRecord returns the validator record
	// It fails if the validator does not exist
	KeyRecord(uid []byte, passphrase string) (*types.KeyRecord, error)

	// SignEOTS signs an EOTS using the private key of the validator and the corresponding
	// secret randomness of the give chain at the given height
	// It fails if the validator does not exist or there's no randomness committed to the given height
	SignEOTS(uid []byte, chainID []byte, msg []byte, height uint64, passphrase string) (*btcec.ModNScalar, error)

	// SignSchnorrSig signs a Schnorr signature using the private key of the validator
	// It fails if the validator does not exist or the message size is not 32 bytes
	SignSchnorrSig(uid []byte, msg []byte, passphrase string) (*schnorr.Signature, error)

	Close() error
}
