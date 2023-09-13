package service

import (
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/types"
)

type FastSyncResult struct {
	Responses           []*provider.RelayerTxResponse
	SyncedHeight        uint64
	LastProcessedHeight uint64
}

// FastSync attempts to send a batch of finality signatures
// from the maximum of the last voted height and the last finalized height
// to the current height
func (v *ValidatorInstance) FastSync(startHeight, endHeight uint64) (*FastSyncResult, error) {
	if v.inSync.Swap(true) {
		return nil, fmt.Errorf("the validator has already been in fast sync")
	}
	defer v.inSync.Store(false)

	if startHeight >= endHeight {
		return nil, fmt.Errorf("the start height %v should not be higher than or equal to the end height %v",
			startHeight, endHeight)
	}

	var syncedHeight uint64
	responses := make([]*provider.RelayerTxResponse, 0)
	// we may need several rounds to catch-up as we need to limit
	// the catch-up distance for each round to avoid memory overflow
	for startHeight < endHeight {
		blocks, err := v.cc.QueryBlocks(startHeight, endHeight, v.cfg.FastSyncLimit)
		if err != nil {
			return nil, err
		}

		if len(blocks) < 1 {
			break
		}

		// Note: not all the blocks in the range will have votes cast
		// due to lack of voting power or public randomness, so we may
		// have gaps during sync
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
			continue
		}

		syncedHeight = catchUpBlocks[len(catchUpBlocks)-1].Height

		res, err := v.SubmitBatchFinalitySignatures(catchUpBlocks)
		if err != nil {
			return nil, err
		}

		startHeight = blocks[len(blocks)-1].Height + 1
		responses = append(responses, res)

		v.logger.WithFields(logrus.Fields{
			"btc_pk_hex":    v.GetBtcPkHex(),
			"start_height":  catchUpBlocks[0].Height,
			"synced_height": syncedHeight,
		}).Debug("the validator is catching up by sending finality signatures in a batch")
	}

	if err := v.SetLastProcessedHeight(endHeight); err != nil {
		return nil, err
	}

	return &FastSyncResult{
		Responses:           responses,
		SyncedHeight:        syncedHeight,
		LastProcessedHeight: endHeight,
	}, nil
}
