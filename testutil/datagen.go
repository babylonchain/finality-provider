package testutil

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/client"

	"github.com/babylonchain/btc-validator/codec"
	"github.com/babylonchain/btc-validator/valrpc"
)

func GenRandomByteArray(r *rand.Rand, length uint64) []byte {
	newHeaderBytes := make([]byte, length)
	r.Read(newHeaderBytes)
	return newHeaderBytes
}

func GenRandomHexStr(r *rand.Rand, length uint64) string {
	randBytes := GenRandomByteArray(r, length)
	return hex.EncodeToString(randBytes)
}

func AddRandomSeedsToFuzzer(f *testing.F, num uint) {
	// Seed based on the current time
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var idx uint
	for idx = 0; idx < num; idx++ {
		f.Add(r.Int63())
	}
}

func GenRandomValidator(r *rand.Rand) *valrpc.Validator {
	return &valrpc.Validator{
		BabylonPk: GenRandomByteArray(r, btcec.PubKeyBytesLenCompressed),
		BtcPk:     GenRandomByteArray(r, btcec.PubKeyBytesLenCompressed),
	}
}

func GenSdkContext(r *rand.Rand, t *testing.T) client.Context {
	chainID := "testchain-" + GenRandomHexStr(r, 4)
	dir := t.TempDir()
	return client.Context{}.
		WithChainID(chainID).
		WithCodec(codec.MakeCodec()).
		WithKeyringDir(dir)
}
