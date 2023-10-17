package eotsmanager

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"github.com/babylonchain/btc-validator/eotsmanager/types"
)

type EOTSManager interface {
	// Ping checks whether the EOTS manager is available
	Ping() error

	// CreateKey generates a key pair at the given name and persists it in storage.
	// The key pair is formatted by BIP-340 (Schnorr Signatures)
	// It fails if there is an existing key Info with the same name or public key.
	CreateKey(name, passPhrase string) ([]byte, error)

	// CreateRandomnessPairList generates and persists a list of Schnorr randomness pairs from
	// startHeight to startHeight+(num-1)*step where step means the gap between each block height
	// that the validator wants to finalize and num means the number of public randomness
	// It fails if the validator does not exist or a randomness pair has been created before
	// NOTE: it will overwrite the randomness regardless of whether there's one existed
	CreateRandomnessPairList(uid []byte, chainID []byte, startHeight uint64, num uint32) ([]*btcec.FieldVal, error)

	// CreateRandomnessPairListWithExistenceCheck checks the existence of randomness by given height
	// before calling CreateRandomnessPairList.
	// It fails if there's randomness existed at a given height
	CreateRandomnessPairListWithExistenceCheck(uid []byte, chainID []byte, startHeight uint64, num uint32) ([]*btcec.FieldVal, error)

	// KeyRecord returns the validator record
	// It fails if the validator does not exist or passPhrase is incorrect
	KeyRecord(uid []byte, passPhrase string) (*types.KeyRecord, error)

	// SignEOTS signs an EOTS using the private key of the validator and the corresponding
	// secret randomness of the give chain at the given height
	// It fails if the validator does not exist or there's no randomness committed to the given height
	SignEOTS(uid []byte, chainID []byte, msg []byte, height uint64) (*btcec.ModNScalar, error)

	// SignSchnorrSig signs a Schnorr signature using the private key of the validator
	// It fails if the validator does not exist or the message size is not 32 bytes
	SignSchnorrSig(uid []byte, msg []byte) (*schnorr.Signature, error)

	Close() error
}
