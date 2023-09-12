package service

import (
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/types"
)

// FastSync attempts to send a batch of finality signatures
// from the maximum of the last voted height and the last finalized height
// to the current height
func (v *ValidatorInstance) FastSync(startHeight, endHeight uint64) (*provider.RelayerTxResponse, error) {
	if v.inSync.Swap(true) {
		return nil, fmt.Errorf("the validator has already been in fast sync")
	}
	defer v.inSync.Store(false)

	if startHeight > endHeight {
		return nil, fmt.Errorf("the start height %v should not be higher than the current block height %v",
			startHeight, endHeight)
	}

	blocks, err := v.cc.QueryBlocks(startHeight, endHeight, v.cfg.FastSyncLimit)
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
			"end_height":   endHeight,
		}).Debug("no blocks should be submitted finality signature for while catching up")
		return nil, nil
	}

	syncedHeight := catchUpBlocks[len(catchUpBlocks)-1].Height
	v.logger.WithFields(logrus.Fields{
		"btc_pk_hex":   v.GetBtcPkHex(),
		"start_height": catchUpBlocks[0].Height,
		"end_height":   syncedHeight,
	}).Debug("the validator is catching up by sending finality signatures in a batch")

	res, err := v.SubmitBatchFinalitySignatures(catchUpBlocks)
	if err != nil {
		return nil, err
	}

	lastProcessedHeight := blocks[len(blocks)-1].Height
	if lastProcessedHeight > syncedHeight {
		if err := v.SetLastProcessedHeight(lastProcessedHeight); err != nil {
			return nil, err
		}
	}

	return res, nil
}
