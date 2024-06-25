//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/finality-provider/types"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"
)

// tests the finality signature submission to the op-finality-gadget contract
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// A BTC delegation has to stake to at least one Babylon finality provider
	// https://github.com/babylonchain/babylon-private/blob/base/consumer-chain-support/x/btcstaking/keeper/msg_server.go#L169-L213
	// So we have to start Babylon chain FP
	ctm.StartFinalityProvider(t, true, 1)

	// start consumer chain FP
	fpList := ctm.StartFinalityProvider(t, false, 1)
	fpInstance := fpList[0]

	ctm.WaitForFpPubRandCommitted(t, fpInstance)

	// query pub rand
	committedPubRandMap, err := ctm.OpL2ConsumerCtrl.QueryLastCommittedPublicRand(fpInstance.GetBtcPk(), 1)
	require.NoError(t, err)
	var lastCommittedStartHeight uint64
	for key := range committedPubRandMap {
		lastCommittedStartHeight = key
		break
	}
	t.Logf("Last committed pubrandList startHeight %d", lastCommittedStartHeight)
	pubRandList, err := fpInstance.GetPubRandList(lastCommittedStartHeight, ctm.FpConfig.NumPubRand)
	require.NoError(t, err)
	// generate commitment and proof for each public randomness
	_, proofList := types.GetPubRandCommitAndProofs(pubRandList)

	r := rand.New(rand.NewSource(1))
	block := &types.BlockInfo{
		Height: lastCommittedStartHeight,
		// mock block hash
		Hash: datagen.GenRandomByteArray(r, 32),
	}

	// fp sign
	fpSig, err := fpInstance.SignFinalitySig(block)
	require.NoError(t, err)

	// pub rand proof
	proof, err := proofList[0].ToProto().Marshal()
	require.NoError(t, err)

	// submit finality signature to smart contract
	submitRes, err := ctm.OpL2ConsumerCtrl.SubmitFinalitySig(
		fpInstance.GetBtcPk(),
		block,
		pubRandList[0],
		proof,
		fpSig.ToModNScalar(),
	)
	require.NoError(t, err)
	t.Logf("Submit finality signature to op finality contract %s", submitRes.TxHash)

	// mock more blocks
	blocks := []*types.BlockInfo{}
	var fpSigs []*secp256k1.ModNScalar
	for i := 1; i <= 3; i++ {
		block := &types.BlockInfo{
			Height: lastCommittedStartHeight + uint64(i),
			Hash:   datagen.GenRandomByteArray(r, 32),
		}
		blocks = append(blocks, block)
		// fp sign
		fpSig, err := fpInstance.SignFinalitySig(block)
		require.NoError(t, err)
		fpSigs = append(fpSigs, fpSig.ToModNScalar())
	}

	// proofs
	var proofs [][]byte
	for i := 1; i <= 3; i++ {
		proof, err := proofList[i].ToProto().Marshal()
		require.NoError(t, err)
		proofs = append(proofs, proof)
	}

	// submit batch finality signatures to smart contract
	batchSubmitRes, err := ctm.OpL2ConsumerCtrl.SubmitBatchFinalitySigs(
		fpInstance.GetBtcPk(),
		blocks,
		pubRandList[1:4],
		proofs,
		fpSigs,
	)
	require.NoError(t, err)
	t.Logf("Submit batch finality signatures to op finality contract %s", batchSubmitRes.TxHash)
}
