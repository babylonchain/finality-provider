package service

import (
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/types"
)

// TryFastSync attempts to send a batch of finality signatures
// from the maximum of the last voted height and the last finalized height
// to the current height
func (v *ValidatorInstance) TryFastSync(currentBlock *types.BlockInfo) (*provider.RelayerTxResponse, error) {
	if v.inSync.Swap(true) {
		return nil, fmt.Errorf("the validator has already been in fast sync")
	}
	defer v.inSync.Store(false)

	// get the last finalized height
	lastFinalizedBlocks, err := v.cc.QueryLatestFinalizedBlocks(1)
	if err != nil {
		return nil, err
	}
	if lastFinalizedBlocks == nil {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":   v.GetBtcPkHex(),
			"block_height": currentBlock.Height,
		}).Debug("no finalized blocks yet, no need to catch up")
		return nil, nil
	}

	lastFinalizedHeight := lastFinalizedBlocks[0].Height
	lastVotedHeight := v.GetLastVotedHeight()

	// get the startHeight from the maximum of the lastVotedHeight and
	// the lastFinalizedHeight plus 1
	var startHeight uint64
	if lastFinalizedHeight < lastVotedHeight {
		startHeight = lastVotedHeight + 1
	} else {
		startHeight = lastFinalizedHeight + 1
	}

	if startHeight == currentBlock.Height {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":     v.GetBtcPkHex(),
			"start_height":   startHeight,
			"current_height": currentBlock.Height,
		}).Debug("the start height is equal to the current block height, no need to catch up")
		return nil, nil
	}

	if startHeight > currentBlock.Height {
		return nil, fmt.Errorf("the start height %v should not be higher than the current block height %v",
			startHeight, currentBlock.Height)
	}

	blocks, err := v.cc.QueryBlocks(startHeight, currentBlock.Height)
	if err != nil {
		return nil, err
	}

	catchUpBlocks := make([]*types.BlockInfo, 0, len(blocks))
	for _, b := range blocks {
		should, err := v.shouldSubmitFinalitySignature(b)
		if err != nil {
			// stop catch-up when critical errors occur
			return nil, err
		}
		if !should {
			// two cases could lead to here:
			// 1. insufficient committed randomness
			// 2. no voting power
			// thus we should continue here in case the two conditions
			// will not happen in the rest of the blocks
			continue
		}
		catchUpBlocks = append(catchUpBlocks, b)
	}

	if len(catchUpBlocks) < 1 {
		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":   v.GetBtcPkHex(),
			"start_height": startHeight,
			"end_height":   currentBlock.Height,
		}).Debug("no blocks should be submitted finality signature for while catching up")
		return nil, nil
	}

	// TODO: we should add an upper bound of blocks to catch-up

	v.logger.WithFields(logrus.Fields{
		"btc_pk_hex":   v.GetBtcPkHex(),
		"start_height": catchUpBlocks[0].Height,
		"end_height":   catchUpBlocks[len(catchUpBlocks)-1].Height,
	}).Debug("the validator is catching up by sending finality signatures in a batch")

	return v.SubmitBatchFinalitySignatures(catchUpBlocks)
}
