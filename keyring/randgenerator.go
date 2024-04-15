package keyring

import (
	"crypto/hmac"
	"crypto/sha256"

	"github.com/babylonchain/babylon/crypto/eots"
)

// GenerateRandomness generates a random scalar with the given key and src
// the result is deterministic with each given input
func GenerateMasterRandPair(key []byte, chainID []byte) (*eots.MasterSecretRand, *eots.MasterPublicRand, error) {
	// calculate the random hash of the key concatenated with chainID and height
	hasher := hmac.New(sha256.New, key)
	hasher.Write(chainID)
	seedSlice := hasher.Sum(nil)

	// convert to 32-byte seed
	var seed [32]byte
	copy(seed[:], seedSlice[:32])

	// convert the hash into private random
	return eots.NewMasterRandPairFromSeed(seed)
}
