package types

import (
	bbn "github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cometbft/cometbft/crypto/merkle"
)

type PubRandCommit struct {
	NumPubRand uint64 `json:"num_pub_rand"`
	Commitment []byte `json:"commitment"`
}

// GetPubRandCommitAndProofs commits a list of public randomness and returns
// the commitment (i.e., Merkle root) and all Merkle proofs
func GetPubRandCommitAndProofs(pubRandList []*btcec.FieldVal) ([]byte, []*merkle.Proof) {
	prBytesList := make([][]byte, 0, len(pubRandList))
	for _, pr := range pubRandList {
		prBytesList = append(prBytesList, bbn.NewSchnorrPubRandFromFieldVal(pr).MustMarshal())
	}
	return merkle.ProofsFromByteSlices(prBytesList)
}
