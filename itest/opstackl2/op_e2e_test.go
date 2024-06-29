//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

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

	// expect 2 rounds of submissions
	e2eutils.WaitForFpPubRandCommitted(t, fpInstance, 2)

	// query pub rand
	committedPubRand, err := ctm.OpL2ConsumerCtrl.QueryLastPublicRandCommit(fpInstance.GetBtcPk())
	require.NoError(t, err)
	lastCommittedStartHeight := committedPubRand.StartHeight
	t.Logf("Last committed pubrandList startHeight %d", lastCommittedStartHeight)
	pubRandList, err := fpInstance.GetPubRandList(lastCommittedStartHeight, ctm.FpConfig.NumPubRand)
	require.NoError(t, err)
	// generate commitment and proof for each public randomness
	_, proofList := types.GetPubRandCommitAndProofs(pubRandList)

	// create a mock block
	r := rand.New(rand.NewSource(time.Now().Unix()))
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

// This test has two test cases:
// 1. block has both two FP signs, so it would be finalized
// 2. block has only one FP with smaller power (1/4) signs, so it would not be considered as finalized
func TestBlockBabylonFinalized(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Test panicked: %v", r)
		}
	}()

	// A BTC delegation has to stake to at least one Babylon finality provider
	// https://github.com/babylonchain/babylon-private/blob/base/consumer-chain-support/x/btcstaking/keeper/msg_server.go#L169-L213
	// So we have to start Babylon chain FP
	bbnFpList := ctm.StartFinalityProvider(t, true, 1)

	// start consumer chain FP
	n := 2
	fpList := ctm.StartFinalityProvider(t, false, n)

	// check the public randomness is committed
	e2eutils.WaitForFpPubRandCommitted(t, fpList[0], 2)
	e2eutils.WaitForFpPubRandCommitted(t, fpList[1], 2)

	// send a BTC delegation to consumer and Babylon finality providers
	// for the first FP, we give it more power b/c it will be used later
	ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpList[0].GetBtcPk(), fpList[0].GetBtcPk()}, e2eutils.StakingTime, 3*e2eutils.StakingAmount)
	ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpList[0].GetBtcPk(), fpList[1].GetBtcPk()}, e2eutils.StakingTime, e2eutils.StakingAmount)

	// check the BTC delegations are pending
	delsResp := ctm.WaitForNPendingDels(t, n)
	require.Equal(t, n, len(delsResp))

	// send covenant sigs to each of the delegations
	for _, delResp := range delsResp {
		d, err := e2eutils.ParseRespBTCDelToBTCDel(delResp)
		require.NoError(t, err)
		// send covenant sigs
		ctm.InsertCovenantSigForDelegation(t, d)
	}

	// check the BTC delegations are active
	ctm.WaitForNActiveDels(t, n)

	// find all fps' first committed pubrand start height
	// Note: now we only need two blocks, one for each test cases, so we pass in 2
	fpStartHeightList := ctm.WaitForTargetBlockPubRand(t, fpList, 2)

	// the first block both FP will sign
	targetBlockHeight := *fpStartHeightList[0]
	if targetBlockHeight < *fpStartHeightList[1] {
		targetBlockHeight = *fpStartHeightList[1]
	}

	// test block data
	r := rand.New(rand.NewSource(time.Now().Unix()))
	testBlock := &types.BlockInfo{
		// set the test block height with the last committed StartHeight by the last FP instance
		Height: targetBlockHeight,
		Hash:   datagen.GenRandomByteArray(r, 32),
	}
	ctm.fpSubmitFinalitySignature(t, fpList[0], fpStartHeightList[0], testBlock)
	ctm.fpSubmitFinalitySignature(t, fpList[1], fpStartHeightList[1], testBlock)

	queryParams := &sdk.L2Block{
		BlockHeight: testBlock.Height,
		BlockHash:   hex.EncodeToString(testBlock.Hash),
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
	t.Logf("Test case 1: block %d is finalized", testBlock.Height)

	// ======  another test case only for the last FP instance ======
	nextBlockHeight := targetBlockHeight + 1
	r2 := rand.New(rand.NewSource(time.Now().Unix()))
	testNextBlock := &types.BlockInfo{
		Height: nextBlockHeight,
		Hash:   datagen.GenRandomByteArray(r2, 32),
	}
	ctm.fpSubmitFinalitySignature(t, fpList[1], fpStartHeightList[1], testNextBlock)

	queryNextParams := &sdk.L2Block{
		BlockHeight:    testNextBlock.Height,
		BlockHash:      hex.EncodeToString(testNextBlock.Hash),
		BlockTimestamp: uint64(1231473952),
	}
	// testNextBlock only have 1/4 total voting power
	finalized, err = ctm.SdkClient.QueryIsBlockBabylonFinalized(queryNextParams)
	require.NoError(t, err)
	require.Equal(t, false, finalized)
	t.Logf("Test case 2: block %d is not finalized", testNextBlock.Height)
}
