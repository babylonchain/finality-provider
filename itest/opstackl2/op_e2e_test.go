//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"encoding/hex"
	"testing"

	"github.com/babylonchain/babylon-finality-gadget/sdk"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	"github.com/babylonchain/finality-provider/testutil/log"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

// tests the finality signature submission to the op-finality-gadget contract
func TestOpSubmitFinalitySignature(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	consumerFpPkList := ctm.RegisterConsumerFinalityProvider(t, 1)
	// start consumer chain FP
	fpList := ctm.StartConsumerFinalityProvider(t, consumerFpPkList)
	fpInstance := fpList[0]

	e2eutils.WaitForFpPubRandCommitted(t, fpInstance)
	// query the first committed pub rand
	committedPubRand, err := queryFirstPublicRandCommit(ctm.OpL2ConsumerCtrl, fpInstance.GetBtcPk())
	require.NoError(t, err)
	committedStartHeight := committedPubRand.StartHeight
	log.Logf(t, "First committed pubrandList startHeight %d", committedStartHeight)
	testBlocks := ctm.WaitForNBlocksAndReturn(t, committedStartHeight, 1)
	testBlock := testBlocks[0]

	// wait for the fp sign
	ctm.WaitForFpVoteAtHeight(t, fpInstance, testBlock.Height)
	queryParams := &sdk.L2Block{
		BlockHeight:    testBlock.Height,
		BlockHash:      hex.EncodeToString(testBlock.Hash),
		BlockTimestamp: 12345, // doesn't matter b/c the BTC client is mocked
	}

	// note: QueryFinalityProviderVotingPower is hardcode to return 1 so FPs can still submit finality sigs even if they
	// don't have voting power. But the finality sigs will not be counted at tally time.
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
	bbnFpPk := ctm.RegisterBabylonFinalityProvider(t, 1)

	// start consumer chain FP
	n := 2
	consumerFpPkList := ctm.RegisterConsumerFinalityProvider(t, n)
	fpList := ctm.StartConsumerFinalityProvider(t, consumerFpPkList)

	// check the public randomness is committed
	e2eutils.WaitForFpPubRandCommitted(t, fpList[0])
	e2eutils.WaitForFpPubRandCommitted(t, fpList[1])

	// send a BTC delegation to consumer and Babylon finality providers
	// for the first FP, we give it more power b/c it will be used later
	ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpPk[0].MustToBTCPK(), consumerFpPkList[0].MustToBTCPK()}, e2eutils.StakingTime, 3*e2eutils.StakingAmount)
	ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpPk[0].MustToBTCPK(), consumerFpPkList[1].MustToBTCPK()}, e2eutils.StakingTime, e2eutils.StakingAmount)

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
		BlockTimestamp: 12345, // doesn't matter b/c the BTC client is mocked
	}
	finalized, err := ctm.SdkClient.QueryIsBlockBabylonFinalized(queryParams)
	require.NoError(t, err)
	require.Equal(t, true, finalized)
	log.Logf(t, "Test case 1: block %d is finalized", testBlock.Height)

	// ===  another test case only for the last FP instance sign ===
	// first make sure the first FP is stopped
	require.Eventually(t, func() bool {
		return !fpList[0].IsRunning()
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	log.Logf(t, "Stopped the first FP instance")

	// select a block that the first FP has not processed yet to give to the second FP to sign
	testNextBlockHeight := fpList[0].GetLastProcessedHeight() + 1
	log.Logf(t, "Test next block height %d", testNextBlockHeight)
	ctm.WaitForFpVoteAtHeight(t, fpList[1], testNextBlockHeight)

	testNextBlock, err := ctm.OpL2ConsumerCtrl.QueryBlock(testNextBlockHeight)
	require.NoError(t, err)
	queryNextParams := &sdk.L2Block{
		BlockHeight:    testNextBlock.Height,
		BlockHash:      hex.EncodeToString(testNextBlock.Hash),
		BlockTimestamp: 12345, // doesn't matter b/c the BTC client is mocked
	}
	// testNextBlock only have 1/4 total voting power
	nextFinalized, err := ctm.SdkClient.QueryIsBlockBabylonFinalized(queryNextParams)
	require.NoError(t, err)
	require.Equal(t, false, nextFinalized)
	log.Logf(t, "Test case 2: block %d is not finalized", testNextBlock.Height)
}

func TestFinalityStuckAndRecover(t *testing.T) {
	ctm := StartOpL2ConsumerManager(t)
	defer ctm.Stop(t)

	// register Babylon finality provider
	bbnFpPk := ctm.RegisterBabylonFinalityProvider(t, 1)

	// register consumer finality providers
	n := 1
	consumerFpPkList := ctm.RegisterConsumerFinalityProvider(t, n)

	// send a BTC delegation to consumer and Babylon finality providers
	ctm.InsertBTCDelegation(t, []*btcec.PublicKey{bbnFpPk[0].MustToBTCPK(), consumerFpPkList[0].MustToBTCPK()}, e2eutils.StakingTime, e2eutils.StakingAmount)

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

	// start consumer chain FP
	fpList := ctm.StartConsumerFinalityProvider(t, consumerFpPkList)
	fpInstance := fpList[0]
	e2eutils.WaitForFpPubRandCommitted(t, fpInstance)

	// wait for the first block to be finalized
	ctm.WaitForFinalizedBlock(t, uint64(1))

	// wait until non fast sync
	ctm.WaitForNonFastSync(t, fpInstance)

	// stop the FP instance
	fpStopErr := fpInstance.Stop()
	require.NoError(t, fpStopErr)
	// make sure the FP is stopped
	require.Eventually(t, func() bool {
		return !fpInstance.IsRunning()
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf("Stopped the FP instance")

	// wait until the finality is stuck
	stuckHeight := ctm.WaitForFinalityStuck(t)
	t.Logf("Test case 1: OP chain block finalized stuck at height %d", stuckHeight)

	// restart the FP instance
	fpStartErr := fpInstance.Start()
	require.NoError(t, fpStartErr)
	// make sure the FP is running
	require.Eventually(t, func() bool {
		return fpInstance.IsRunning()
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf("Restarted the FP instance")

	// wait for the stuck block is finalized
	latestFinalizedHeight := ctm.WaitForFinalizedBlock(t, stuckHeight)
	require.Less(t, stuckHeight, latestFinalizedHeight)
	t.Logf("Test case 2: OP chain fianlity is recover, the latest finalized block height %d", latestFinalizedHeight)
}
