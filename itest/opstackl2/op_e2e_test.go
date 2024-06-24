//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"testing"

	"math/rand"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/finality-provider/types"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"
)

// tests the finality signature submission to the op-finality-gadget contract
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// register FP in Babylon Chain
	fpList := ctm.StartFinalityProvider(t, 1)

	// generate randomness data
	pubRandListInfo, msgPub, err := ctm.GenerateCommitPubRandListMsg(fpList[0].GetBtcPkBIP340(), 1, 100)
	require.NoError(t, err)

	// commit pub rand to smart contract
	commitRes, err := ctm.OpL2ConsumerCtrl.CommitPubRandList(
		msgPub.FpBtcPk.MustToBTCPK(),
		msgPub.StartHeight,
		msgPub.NumPubRand,
		msgPub.Commitment,
		msgPub.Sig.MustToBTCSig(),
	)
	require.NoError(t, err)
	t.Logf("Commit PubRandList to op finality contract %s", commitRes.TxHash)

	// query pub rand
	committedPubRandMap, err := ctm.OpL2ConsumerCtrl.QueryLastCommittedPublicRand(msgPub.FpBtcPk.MustToBTCPK(), 1)
	require.NoError(t, err)
	for k, v := range committedPubRandMap {
		require.Equal(t, uint64(1), k)
		require.Equal(t, uint64(100), v.NumPubRand)
		require.Equal(t, msgPub.Commitment, v.Commitment)
		break
	}

	// mock block
	r := rand.New(rand.NewSource(1))
	block := &types.BlockInfo{
		Height: uint64(1),
		Hash:   datagen.GenRandomByteArray(r, 32),
	}
	// fp sign
	fpSig, err := fpList[0].SignFinalitySig(block)
	require.NoError(t, err)

	// pub rand proof
	proof, err := pubRandListInfo.ProofList[0].ToProto().Marshal()
	require.NoError(t, err)

	// submit finality signature to smart contract
	submitRes, err := ctm.OpL2ConsumerCtrl.SubmitFinalitySig(
		msgPub.FpBtcPk.MustToBTCPK(),
		block,
		pubRandListInfo.PubRandList[0],
		proof,
		fpSig.ToModNScalar(),
	)
	require.NoError(t, err)
	t.Logf("Submit finality signature to op finality contract %s", submitRes.TxHash)

	// mock more blocks
	blocks := []*types.BlockInfo{}
	var fpSigs []*secp256k1.ModNScalar
	for i := 2; i <= 4; i++ {
		block := &types.BlockInfo{
			Height: uint64(i),
			Hash:   datagen.GenRandomByteArray(r, 32),
		}
		blocks = append(blocks, block)
		// fp sign
		fpSig, err := fpList[0].SignFinalitySig(block)
		require.NoError(t, err)
		fpSigs = append(fpSigs, fpSig.ToModNScalar())
	}

	// proofs
	var proofs [][]byte
	for i := 1; i <= 3; i++ {
		proof, err := pubRandListInfo.ProofList[i].ToProto().Marshal()
		require.NoError(t, err)
		proofs = append(proofs, proof)
	}

	// submit batch finality signatures to smart contract
	batchSubmitRes, err := ctm.OpL2ConsumerCtrl.SubmitBatchFinalitySigs(
		msgPub.FpBtcPk.MustToBTCPK(),
		blocks,
		pubRandListInfo.PubRandList[1:4],
		proofs,
		fpSigs,
	)
	require.NoError(t, err)
	t.Logf("Submit batch finality signatures to op finality contract %s", batchSubmitRes.TxHash)
}
