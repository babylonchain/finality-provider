//go:build e2e
// +build e2e

package e2etest

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/types"
)

var (
	stakingTime   = uint16(100)
	stakingAmount = int64(20000)
)

// TestFinalityProviderLifeCycle tests the whole life cycle of a finality-provider
// creation -> registration -> randomness commitment ->
// activation with BTC delegation and Covenant sig ->
// vote submission -> block finalization
func TestFinalityProviderLifeCycle(t *testing.T) {
	tm, fpInsList := StartManagerWithFinalityProvider(t, 1)
	defer tm.Stop(t)

	fpIns := fpInsList[0]

	params := tm.GetParams(t)

	// check the public randomness is committed
	tm.WaitForFpPubRandCommitted(t, fpIns)

	// send a BTC delegation
	_ = tm.InsertBTCDelegation(t, []*btcec.PublicKey{fpIns.MustGetBtcPk()}, stakingTime, stakingAmount, params)

	// check the BTC delegation is pending
	_ = tm.WaitForNPendingDels(t, 1)

	// check the BTC delegation is active
	_ = tm.WaitForFpNActiveDels(t, fpIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForFpVoteCast(t, fpIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)
}

// TestDoubleSigning tests the attack scenario where the finality-provider
// sends a finality vote over a conflicting block
// in this case, the BTC private key should be extracted by Babylon
func TestDoubleSigning(t *testing.T) {
	tm, fpInsList := StartManagerWithFinalityProvider(t, 1)
	defer tm.Stop(t)

	fpIns := fpInsList[0]

	params := tm.GetParams(t)

	// check the public randomness is committed
	tm.WaitForFpPubRandCommitted(t, fpIns)

	// send a BTC delegation
	_ = tm.InsertBTCDelegation(t, []*btcec.PublicKey{fpIns.MustGetBtcPk()}, stakingTime, stakingAmount, params)

	// check the BTC delegation is pending
	_ = tm.WaitForNPendingDels(t, 1)

	// check the BTC delegation is active
	_ = tm.WaitForFpNActiveDels(t, fpIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForFpVoteCast(t, fpIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)
	t.Logf("the block at height %v is finalized", lastVotedHeight)

	finalizedBlocks := tm.WaitForNFinalizedBlocks(t, 1)

	// attack: manually submit a finality vote over a conflicting block
	// to trigger the extraction of finality-provider's private key
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := &types.BlockInfo{
		Height: finalizedBlocks[0].Height,
		Hash:   datagen.GenRandomByteArray(r, 32),
	}
	_, extractedKey, err := fpIns.TestSubmitFinalitySignatureAndExtractPrivKey(b)
	require.NoError(t, err)
	require.NotNil(t, extractedKey)
	localKey := tm.GetFpPrivKey(t, fpIns.GetBtcPkBIP340().MustMarshal())
	require.True(t, localKey.Key.Equals(&extractedKey.Key) || localKey.Key.Negate().Equals(&extractedKey.Key))

	t.Logf("the equivocation attack is successful")
}

// TestMultipleFinalityProviders tests starting with multiple finality providers
func TestMultipleFinalityProviders(t *testing.T) {
	n := 3
	tm, fpInstances := StartManagerWithFinalityProvider(t, n)
	defer tm.Stop(t)

	params := tm.GetParams(t)

	// submit BTC delegations for each finality-provider
	for _, fpIns := range fpInstances {
		tm.Wg.Add(1)
		go func(fpi *service.FinalityProviderInstance) {
			defer tm.Wg.Done()
			// check the public randomness is committed
			tm.WaitForFpPubRandCommitted(t, fpi)

			// send a BTC delegation
			_ = tm.InsertBTCDelegation(t, []*btcec.PublicKey{fpi.MustGetBtcPk()}, stakingTime, stakingAmount, params)
		}(fpIns)
	}
	tm.Wg.Wait()

	for _, fpIns := range fpInstances {
		tm.Wg.Add(1)
		go func(fpi *service.FinalityProviderInstance) {
			defer tm.Wg.Done()
			_ = tm.WaitForFpNActiveDels(t, fpi.GetBtcPkBIP340(), 1)
		}(fpIns)
	}
	tm.Wg.Wait()

	// check there's a block finalized
	_ = tm.WaitForNFinalizedBlocks(t, 1)
}

// TestFastSync tests the fast sync process where the finality-provider is terminated and restarted with fast sync
func TestFastSync(t *testing.T) {
	tm, fpInsList := StartManagerWithFinalityProvider(t, 1)
	defer tm.Stop(t)

	fpIns := fpInsList[0]

	params := tm.GetParams(t)

	// check the public randomness is committed
	tm.WaitForFpPubRandCommitted(t, fpIns)

	// send a BTC delegation
	_ = tm.InsertBTCDelegation(t, []*btcec.PublicKey{fpIns.MustGetBtcPk()}, stakingTime, stakingAmount, params)

	// check the BTC delegation is pending
	_ = tm.WaitForNPendingDels(t, 1)

	_ = tm.WaitForFpNActiveDels(t, fpIns.GetBtcPkBIP340(), 1)

	// check the last voted block is finalized
	lastVotedHeight := tm.WaitForFpVoteCast(t, fpIns)
	tm.CheckBlockFinalization(t, lastVotedHeight, 1)

	t.Logf("the block at height %v is finalized", lastVotedHeight)

	var finalizedBlocks []*types.BlockInfo
	finalizedBlocks = tm.WaitForNFinalizedBlocks(t, 1)

	n := 3
	// stop the finality-provider for a few blocks then restart to trigger the fast sync
	tm.FpConfig.FastSyncGap = uint64(n)
	tm.StopAndRestartFpAfterNBlocks(t, n, fpIns)

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
