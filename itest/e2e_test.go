package e2etest

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/types"
)

var (
	stakingTime   = uint16(100)
	stakingAmount = int64(20000)
)

// TestValidatorLifeCycle tests the whole life cycle of a validator
// creation -> registration -> randomness commitment ->
// activation with BTC delegation and Covenant sig ->
// vote submission -> block finalization
func TestValidatorLifeCycle(t *testing.T) {
	tm, valIns := StartManagerWithValidator(t)
	defer tm.Stop(t)

	params := tm.getParams(t)

	// check the public randomness is committed
	tm.WaitForValPubRandCommitted(t, valIns)

	// send a BTC delegation
	_ = tm.InsertBTCDelegation(t, []*btcec.PublicKey{valIns.MustGetBtcPk()}, stakingTime, stakingAmount, params)

	// check the BTC delegation is pending
	_ = tm.WaitForNPendingDels(t, 1)

	// check the BTC delegation is active
	_ = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForValVoteCast(t, valIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)
}

// TestDoubleSigning tests the attack scenario where the validator
// sends a finality vote over a conflicting block
// in this case, the BTC private key should be extracted by Babylon
func TestDoubleSigning(t *testing.T) {
	tm, valIns := StartManagerWithValidator(t)
	defer tm.Stop(t)

	params := tm.getParams(t)

	// check the public randomness is committed
	tm.WaitForValPubRandCommitted(t, valIns)

	// send a BTC delegation
	_ = tm.InsertBTCDelegation(t, []*btcec.PublicKey{valIns.MustGetBtcPk()}, stakingTime, stakingAmount, params)

	// check the BTC delegation is pending
	_ = tm.WaitForNPendingDels(t, 1)

	// check the BTC delegation is active
	_ = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForValVoteCast(t, valIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)

	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)

	// attack: manually submit a finality vote over a conflicting block
	// to trigger the extraction of validator's private key
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := &types.BlockInfo{
		Height: finalizedBlocks[0].Height,
		Hash:   datagen.GenRandomAppHash(r),
	}
	_, extractedKey, err := valIns.TestSubmitFinalitySignatureAndExtractPrivKey(b)
	require.NoError(t, err)
	require.NotNil(t, extractedKey)
	localKey := tm.GetValPrivKey(t, valIns.GetBtcPkBIP340().MustMarshal())
	require.True(t, localKey.Key.Equals(&extractedKey.Key) || localKey.Key.Negate().Equals(&extractedKey.Key))

	t.Logf("the equivocation attack is successful")
}

// TestFastSync tests the fast sync process where the validator is terminated and restarted with fast sync
func TestFastSync(t *testing.T) {
	tm, valIns := StartManagerWithValidator(t)
	defer tm.Stop(t)

	params := tm.getParams(t)

	// check the public randomness is committed
	tm.WaitForValPubRandCommitted(t, valIns)

	// send a BTC delegation
	_ = tm.InsertBTCDelegation(t, []*btcec.PublicKey{valIns.MustGetBtcPk()}, stakingTime, stakingAmount, params)

	// check the BTC delegation is pending
	_ = tm.WaitForNPendingDels(t, 1)

	_ = tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForValVoteCast(t, valIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)

	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)

	n := 3
	// stop the validator for a few blocks then restart to trigger the fast sync
	tm.ValConfig.FastSyncGap = uint64(n)
	tm.StopAndRestartValidatorAfterNBlocks(t, n, valIns)

	// check there are n+1 blocks finalized
	finalizedBlocks = tm.WaitForNFinalizedBlocks(t, n+1)
	finalizedHeight := finalizedBlocks[0].Height
	t.Logf("the latest finalized block is at %v", finalizedHeight)

	// check if the fast sync works by checking if the gap is not more than 1
	currentHeaderRes, err := tm.BabylonClient.QueryBestBlock()
	currentHeight := currentHeaderRes.Height
	t.Logf("the current block is at %v", currentHeight)
	require.NoError(t, err)
	require.True(t, currentHeight < finalizedHeight+uint64(n))
}

func TestCovenantLifeCycle(t *testing.T) {
	tm, valIns := StartManagerWithValidator(t)
	defer tm.Stop(t)

	params := tm.getParams(t)

	valPk := valIns.MustGetBtcPk()
	valBtcPk := valIns.GetBtcPkBIP340()
	// send BTC delegation and make sure it's deep enough in btclightclient module
	delData := tm.InsertBTCDelegation(t, []*btcec.PublicKey{valPk}, stakingTime, stakingAmount, params)
	dels := tm.WaitForValNActiveDels(t, valIns.GetBtcPkBIP340(), 1)
	del := dels[0]
	err := tm.Va.Stop()
	require.NoError(t, err)

	tm.InsertBTCUnbonding(
		t,
		del,
		delData.DelegatorPrivKey,
		delData.ChangeAddr,
		params,
	)

	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valBtcPk, 1000)
		if err != nil {
			return false
		}
		return len(dels) == 1 && dels[0].BtcUndelegation != nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Log("undelegation is found, waiting for covenant sigs")

	// after providing validator unbodning signature, we should wait for covenant to provide both valid signatures
	require.Eventually(t, func() bool {
		dels, err = tm.BabylonClient.QueryBTCValidatorDelegations(valBtcPk, 1000)
		if err != nil {
			return false
		}

		if len(dels) != 1 {
			return false
		}

		del := dels[0]

		if del.BtcUndelegation == nil {
			return false
		}

		return len(del.BtcUndelegation.CovenantSlashingSigs) != 0 && len(del.BtcUndelegation.CovenantUnbondingSigs) != 0
	}, 1*time.Minute, eventuallyPollTime)
}
