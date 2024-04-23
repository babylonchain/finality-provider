package eotsmanager

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"github.com/babylonchain/finality-provider/eotsmanager/types"
)

type EOTSManager interface {
	// CreateKey generates a key pair at the given name and persists it in storage.
	// The key pair is formatted by BIP-340 (Schnorr Signatures)
	// It fails if there is an existing key Info with the same name or public key.
	CreateKey(name, passphrase, hdPath string) ([]byte, error)

	// CreateKeyFromMnemonic generates a key pair from the mnemonic at the given name
	// and persists it in storage. The key pair is formatted by BIP-340 (Schnorr Signatures)
	// It fails if there is an existing key Info with the same name or public key.
	CreateKeyFromMnemonic(name, passphrase, hdPath, mnemonic string) ([]byte, error)

	// CreateMasterRandPair generates a pair of master secret/public randomness
	// It fails if the finality provider does not exist or passPhrase is incorrect
	// NOTE: the master randomness pair is deterministically generated based on the EOTS key and chainID
	CreateMasterRandPair(uid []byte, chainID []byte, passphrase string) (string, error)

	// KeyRecord returns the finality provider record
	// It fails if the finality provider does not exist or passPhrase is incorrect
	KeyRecord(uid []byte, passphrase string) (*types.KeyRecord, error)

	// SignEOTS signs an EOTS using the private key of the finality provider and the corresponding
	// secret randomness of the give chain at the given height
	// It fails if the finality provider does not exist or there's no randomness committed to the given height
	// or passPhrase is incorrect
	SignEOTS(uid []byte, chainID []byte, msg []byte, height uint64, passphrase string) (*btcec.ModNScalar, error)

	// SignSchnorrSig signs a Schnorr signature using the private key of the finality provider
	// It fails if the finality provider does not exist or the message size is not 32 bytes
	// or passPhrase is incorrect
	SignSchnorrSig(uid []byte, msg []byte, passphrase string) (*schnorr.Signature, error)

	Close() error
}
