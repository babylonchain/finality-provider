//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon-da-sdk/sdk"
	"github.com/babylonchain/babylon/testutil/datagen"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"
)

// tests the finality signature submission to the op-finality-gadget contract
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// start consumer chain FP
	fpList := ctm.StartFinalityProvider(t, false, 1)
	fpInstance := fpList[0]

	e2eutils.WaitForFpPubRandCommitted(t, fpInstance)

	// query pub rand
	committedPubRand, err := ctm.OpL2ConsumerCtrl.QueryLastCommittedPublicRand(fpInstance.GetBtcPk())
	require.NoError(t, err)
	lastCommittedStartHeight := committedPubRand.StartHeight
	t.Logf("Last committed pubrandList startHeight %d", lastCommittedStartHeight)
	pubRandList, err := fpInstance.GetPubRandList(lastCommittedStartHeight, ctm.FpConfig.NumPubRand)
	require.NoError(t, err)
	// generate commitment and proof for each public randomness
	_, proofList := types.GetPubRandCommitAndProofs(pubRandList)

	// create a mock block
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
	_, err = ctm.OpL2ConsumerCtrl.SubmitFinalitySig(
		fpInstance.GetBtcPk(),
		block,
		pubRandList[0],
		proof,
		fpSig.ToModNScalar(),
	)
	require.NoError(t, err)
	t.Logf("Submit finality signature to op finality contract")

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
	_, err = ctm.OpL2ConsumerCtrl.SubmitBatchFinalitySigs(
		fpInstance.GetBtcPk(),
		blocks,
		pubRandList[1:4],
		proofs,
		fpSigs,
	)
	require.NoError(t, err)
	t.Logf("Submit batch finality signatures to op finality contract")
}

// tests the query whether the block is Babylon finalized
func TestBlockBabylonFinalized(t *testing.T) {

	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// A BTC delegation has to stake to at least one Babylon finality provider
	// https://github.com/babylonchain/babylon-private/blob/base/consumer-chain-support/x/btcstaking/keeper/msg_server.go#L169-L213
	// So we have to start Babylon chain FP
	bbnFpList := ctm.StartFinalityProvider(t, true, 1)

	// start consumer chain FP
	n := 1
	fpList := ctm.StartFinalityProvider(t, false, n)

	var mockHash []byte
	// submit BTC delegations for each finality-provider
	for _, fp := range fpList {
		// check the public randomness is committed
		e2eutils.WaitForFpPubRandCommitted(t, fp)
		// send a BTC delegation to consumer and Babylon finality providers
		ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpList[0].GetBtcPk(), fp.GetBtcPk()}, e2eutils.StakingTime, e2eutils.StakingAmount)
	}

	// check the BTC delegations are pending
	delsResp := ctm.WaitForNPendingDels(t, 1)
	require.Equal(t, 1, len(delsResp))

	// send covenant sigs to each of the delegations
	for _, delResp := range delsResp {
		d, err := e2eutils.ParseRespBTCDelToBTCDel(delResp)
		require.NoError(t, err)
		// send covenant sigs
		ctm.InsertCovenantSigForDelegation(t, d)
	}

	// check the BTC delegations are active
	_ = ctm.WaitForNActiveDels(t, 1)

	var lastCommittedStartHeight uint64
	for _, fp := range fpList {
		// query pub rand
		committedPubRand, err := ctm.OpL2ConsumerCtrl.QueryLastCommittedPublicRand(fp.GetBtcPk())
		require.NoError(t, err)
		lastCommittedStartHeight = committedPubRand.StartHeight
		t.Logf("Last committed pubrandList startHeight %d", lastCommittedStartHeight)

		pubRandList, err := fp.GetPubRandList(lastCommittedStartHeight, ctm.FpConfig.NumPubRand)
		require.NoError(t, err)
		// generate commitment and proof for each public randomness
		_, proofList := types.GetPubRandCommitAndProofs(pubRandList)

		// mock block hash
		r := rand.New(rand.NewSource(1))
		mockHash := datagen.GenRandomByteArray(r, 32)
		block := &types.BlockInfo{
			Height: lastCommittedStartHeight,
			Hash:   mockHash,
		}
		// fp sign
		fpSig, err := fp.SignFinalitySig(block)
		require.NoError(t, err)

		// pub rand proof
		proof, err := proofList[0].ToProto().Marshal()
		require.NoError(t, err)

		// submit finality signature to smart contract
		submitRes, err := ctm.OpL2ConsumerCtrl.SubmitFinalitySig(
			fp.GetBtcPk(),
			block,
			pubRandList[0],
			proof,
			fpSig.ToModNScalar(),
		)
		require.NoError(t, err)
		t.Logf("Submit finality signature to op finality contract %s", submitRes.TxHash)
	}

	queryParams := &sdk.L2Block{
		BlockHeight: lastCommittedStartHeight,
		BlockHash:   hex.EncodeToString(mockHash),
		/*
			this is BTC height 10's timestamp: https://mempool.space/block/10

			we use it b/c in InsertBTCDelegation(), it inserts at BTC block 0 because
			- `tm.BBNClient.QueryBtcLightClientTip()` returns block 0
			- `params.ComfirmationTimeBlocks` defaults to be 6, meaning the delegation
			becomes active around block 6

			Note: the staking time is default defined in utils.go to be 100 blocks:
			StakingTime           = uint16(100)

			since we only mock the first few BTC blocks and inject into the local
			babylon chain, the delegation will be active forever. So even if we choose
			a very large real BTC mainnet's block timestamp, the test will still pass.

			But to be safe, we decided to choose the timestamp of block 10.
		*/
		BlockTimestamp: uint64(1231473952),
	}
	finalized, err := ctm.SdkClient.QueryIsBlockBabylonFinalized(queryParams)
	require.NoError(t, err)
	require.Equal(t, true, finalized)
}
