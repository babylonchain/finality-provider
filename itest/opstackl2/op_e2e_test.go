//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/hex"
	"testing"
	"time"

	sdkclient "github.com/babylonchain/babylon-finality-gadget/sdk/client"
	"github.com/babylonchain/babylon-finality-gadget/sdk/cwclient"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	"github.com/babylonchain/finality-provider/testutil/log"
	"github.com/stretchr/testify/require"
)

// tests the finality signature submission to the op-finality-gadget contract
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t, 1)
	defer ctm.Stop(t)

	consumerFpPkList := ctm.RegisterConsumerFinalityProvider(t, 1)
	// start consumer chain FP
	fpList := ctm.StartConsumerFinalityProvider(t, consumerFpPkList)
	fpInstance := fpList[0]

	e2eutils.WaitForFpPubRandCommitted(t, fpInstance)
	// query the first committed pub rand
	opcc := ctm.getFirstOpCC()
	committedPubRand, err := queryFirstPublicRandCommit(opcc, fpInstance.GetBtcPk())
	require.NoError(t, err)
	committedStartHeight := committedPubRand.StartHeight
	t.Logf(log.Prefix("First committed pubrandList startHeight %d"), committedStartHeight)
	testBlocks := ctm.WaitForNBlocksAndReturn(t, committedStartHeight, 1)
	testBlock := testBlocks[0]

	// wait for the fp sign
	ctm.WaitForFpVoteAtHeight(t, fpInstance, testBlock.Height)
	queryParams := cwclient.L2Block{
		BlockHeight:    testBlock.Height,
		BlockHash:      hex.EncodeToString(testBlock.Hash),
		BlockTimestamp: 12345, // doesn't matter b/c the BTC client is mocked
	}

	// note: QueryFinalityProviderHasPower is hardcode to return true so FPs can still submit finality sigs even if they
	// don't have voting power. But the finality sigs will not be counted at tally time.
	_, err = ctm.SdkClient.QueryIsBlockBabylonFinalized(queryParams)
	require.ErrorIs(t, err, sdkclient.ErrNoFpHasVotingPower)
}

// This test has two test cases:
// 1. block has both two FP signs, so it would be finalized
// 2. block has only one FP with smaller power (1/4) signs, so it would not be considered as finalized
func TestOpMultipleFinalityProviders(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t, 2)
	defer ctm.Stop(t)

	// register, get BTC delegations, and start FPs
	n := 2
	fpList := ctm.SetupFinalityProviders(t, n, []stakingParam{
		// for the first FP, we give it more power b/c it will be used later
		{e2eutils.StakingTime, 3 * e2eutils.StakingAmount},
		{e2eutils.StakingTime, e2eutils.StakingAmount},
	})

	// check the public randomness is committed
	for i := 0; i < n; i++ {
		e2eutils.WaitForFpPubRandCommitted(t, fpList[i])
	}

	// both FP will sign the first block
	targetBlockHeight := ctm.WaitForTargetBlockPubRand(t, fpList)

	ctm.WaitForFpVoteAtHeight(t, fpList[0], targetBlockHeight)
	// stop the first FP instance
	fpStopErr := fpList[0].Stop()
	require.NoError(t, fpStopErr)

	ctm.WaitForFpVoteAtHeight(t, fpList[1], targetBlockHeight)

	testBlock, err := ctm.getFirstOpCC().QueryBlock(targetBlockHeight)
	require.NoError(t, err)
	queryParams := cwclient.L2Block{
		BlockHeight:    testBlock.Height,
		BlockHash:      hex.EncodeToString(testBlock.Hash),
		BlockTimestamp: 12345, // doesn't matter b/c the BTC client is mocked
	}
	finalized, err := ctm.SdkClient.QueryIsBlockBabylonFinalized(queryParams)
	require.NoError(t, err)
	require.Equal(t, true, finalized)
	t.Logf(log.Prefix("Test case 1: block %d is finalized"), testBlock.Height)

	// ===  another test case only for the last FP instance sign ===
	// first make sure the first FP is stopped
	require.Eventually(t, func() bool {
		return !fpList[0].IsRunning()
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf(log.Prefix("Stopped the first FP instance"))

	// select a block that the first FP has not processed yet to give to the second FP to sign
	testNextBlockHeight := fpList[0].GetLastVotedHeight() + 1
	t.Logf(log.Prefix("Test next block height %d"), testNextBlockHeight)
	ctm.WaitForFpVoteAtHeight(t, fpList[1], testNextBlockHeight)

	testNextBlock, err := ctm.getFirstOpCC().QueryBlock(testNextBlockHeight)
	require.NoError(t, err)
	queryNextParams := cwclient.L2Block{
		BlockHeight:    testNextBlock.Height,
		BlockHash:      hex.EncodeToString(testNextBlock.Hash),
		BlockTimestamp: 12345, // doesn't matter b/c the BTC client is mocked
	}
	// testNextBlock only have 1/4 total voting power
	nextFinalized, err := ctm.SdkClient.QueryIsBlockBabylonFinalized(queryNextParams)
	require.NoError(t, err)
	require.Equal(t, false, nextFinalized)
	t.Logf(log.Prefix("Test case 2: block %d is not finalized"), testNextBlock.Height)
}

func TestFinalityStuckAndRecover(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t, 1)
	defer ctm.Stop(t)

	// register, get BTC delegations, and start FPs
	n := 1
	fpList := ctm.SetupFinalityProviders(t, n, []stakingParam{
		{e2eutils.StakingTime, e2eutils.StakingAmount},
	})
	fpInstance := fpList[0]

	// wait for the first block to be finalized
	e2eutils.WaitForFpPubRandCommitted(t, fpInstance)
	ctm.WaitForNextFinalizedBlock(t, uint64(1))

	// stop the FP instance
	fpStopErr := fpInstance.Stop()
	require.NoError(t, fpStopErr)
	// make sure the FP is stopped
	require.Eventually(t, func() bool {
		return !fpInstance.IsRunning()
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf(log.Prefix("Stopped the FP instance"))

	// get the last voted height
	lastVotedHeight := fpInstance.GetLastVotedHeight()
	t.Logf(log.Prefix("last voted height %d"), lastVotedHeight)
	// wait until the block finalized
	require.Eventually(t, func() bool {
		latestFinalizedBlock, err := ctm.getFirstOpCC().QueryLatestFinalizedBlock()
		require.NoError(t, err)
		stuckHeight := latestFinalizedBlock.Height
		return lastVotedHeight == stuckHeight
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)

	// check the finality gets stuck
	time.Sleep(5 * ctm.getL1BlockTime())
	latestFinalizedBlock, err := ctm.getFirstOpCC().QueryLatestFinalizedBlock()
	require.NoError(t, err)
	stuckHeight := latestFinalizedBlock.Height
	require.Equal(t, lastVotedHeight, stuckHeight)
	t.Logf(log.Prefix("OP chain block finalized head stuck at height %d"), stuckHeight)

	// restart the FP instance
	fpStartErr := fpInstance.Start()
	require.NoError(t, fpStartErr)
	// make sure the FP is running
	require.Eventually(t, func() bool {
		return fpInstance.IsRunning()
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf(log.Prefix("Restarted the FP instance"))

	// wait for next finalized block > stuckHeight
	nextFinalizedHeight := ctm.WaitForNextFinalizedBlock(t, stuckHeight)
	t.Logf(log.Prefix(
		"OP chain fianlity is recovered, the latest finalized block height %d",
	), nextFinalizedHeight)
}
