//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/hex"
	"testing"

	"github.com/babylonchain/babylon-da-sdk/sdk"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

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
var BlockTimestamp = uint64(1231473952)

// tests the finality signature submission to the op-finality-gadget contract
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// start consumer chain FP
	fpList := ctm.StartFinalityProvider(t, false, 1)
	fpInstance := fpList[0]

	e2eutils.WaitForFpPubRandCommitted(t, fpInstance)
	// query the first committed pub rand
	committedPubRand, err := queryFirstPublicRandCommit(ctm.OpL2ConsumerCtrl, fpInstance.GetBtcPk())
	require.NoError(t, err)
	committedStartHeight := committedPubRand.StartHeight
	t.Logf("First committed pubrandList startHeight %d", committedStartHeight)
	testBlocks := ctm.WaitForNBlocksAndReturn(t, committedStartHeight, 1)
	testBlock := testBlocks[0]

	// wait for the fp sign
	ctm.WaitForFpVoteAtHeight(t, fpInstance, testBlock.Height)
	queryParams := &sdk.L2Block{
		BlockHeight:    testBlock.Height,
		BlockHash:      hex.EncodeToString(testBlock.Hash),
		BlockTimestamp: BlockTimestamp,
	}
	_, err = ctm.SdkClient.QueryIsBlockBabylonFinalized(queryParams)
	require.ErrorIs(t, err, sdk.ErrNoFpHasVotingPower)
}

// This test has two test cases:
// 1. block has both two FP signs, so it would be finalized
// 2. block has only one FP with smaller power (1/4) signs, so it would not be considered as finalized
func TestOpMultipleFinalityProviders(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// A BTC delegation has to stake to at least one Babylon finality provider
	// https://github.com/babylonchain/babylon-private/blob/base/consumer-chain-support/x/btcstaking/keeper/msg_server.go#L169-L213
	// So we have to start Babylon chain FP
	bbnFpPk := ctm.RegisterBBNFinalityProvider(t)

	// start consumer chain FP
	n := 2
	fpList := ctm.StartFinalityProvider(t, false, n)

	// check the public randomness is committed
	e2eutils.WaitForFpPubRandCommitted(t, fpList[0])
	e2eutils.WaitForFpPubRandCommitted(t, fpList[1])

	// send a BTC delegation to consumer and Babylon finality providers
	// for the first FP, we give it more power b/c it will be used later
	ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpPk, fpList[0].GetBtcPk()}, e2eutils.StakingTime, 3*e2eutils.StakingAmount)
	ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpPk, fpList[1].GetBtcPk()}, e2eutils.StakingTime, e2eutils.StakingAmount)

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

	// the first block both FP will sign
	targetBlockHeight := ctm.WaitForTargetBlockPubRand(t, fpList)

	ctm.WaitForFpVoteAtHeight(t, fpList[0], targetBlockHeight)
	// stop the first FP instance
	fpStopErr := fpList[0].Stop()
	require.NoError(t, fpStopErr)

	ctm.WaitForFpVoteAtHeight(t, fpList[1], targetBlockHeight)

	testBlock, err := ctm.OpL2ConsumerCtrl.QueryBlock(targetBlockHeight)
	require.NoError(t, err)
	queryParams := &sdk.L2Block{
		BlockHeight:    testBlock.Height,
		BlockHash:      hex.EncodeToString(testBlock.Hash),
		BlockTimestamp: uint64(1231473952),
	}
	finalized, err := ctm.SdkClient.QueryIsBlockBabylonFinalized(queryParams)
	require.NoError(t, err)
	require.Equal(t, true, finalized)
	t.Logf("Test case 1: block %d is finalized", testBlock.Height)

	// ===  another test case only for the last FP instance sign ===
	// first make sure the first FP is stopped
	require.Eventually(t, func() bool {
		return !fpList[0].IsRunning()
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf("Stopped the first FP instance")

	// select a block that the first FP has not processed yet to give to the second FP to sign
	testNextBlockHeight := fpList[0].GetLastProcessedHeight() + 1
	t.Logf("Test next block height %d", testNextBlockHeight)
	ctm.WaitForFpVoteAtHeight(t, fpList[1], testNextBlockHeight)

	testNextBlock, err := ctm.OpL2ConsumerCtrl.QueryBlock(testNextBlockHeight)
	require.NoError(t, err)
	queryNextParams := &sdk.L2Block{
		BlockHeight:    testNextBlock.Height,
		BlockHash:      hex.EncodeToString(testNextBlock.Hash),
		BlockTimestamp: uint64(1231473952),
	}
	// testNextBlock only have 1/4 total voting power
	nextFinalized, err := ctm.SdkClient.QueryIsBlockBabylonFinalized(queryNextParams)
	require.NoError(t, err)
	require.Equal(t, false, nextFinalized)
	t.Logf("Test case 2: block %d is not finalized", testNextBlock.Height)
}
