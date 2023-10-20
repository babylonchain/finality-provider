package randgenerator

import (
	"crypto/hmac"
	"crypto/sha256"

	"github.com/btcsuite/btcd/btcec/v2"
)

// GenerateRandomness generates a random scalar with the given key and src
// the result is deterministic with each given input
func GenerateRandomness(key []byte, src []byte) *btcec.ModNScalar {
	digest := hmac.New(sha256.New, key)
	digest.Write(src)
	randPre := digest.Sum(nil)
	var randScalar btcec.ModNScalar
	randScalar.SetByteSlice(randPre)

	return &randScalar
}
