package service

import (
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"

	"github.com/babylonchain/btc-validator/types"
)

// CatchUp allows the validator to send a batch of finality signatures
// from the maximum of the lasted voted height and the last finalized height
// to the current height
func (v *ValidatorInstance) CatchUp(currentBlock *types.BlockInfo) (*provider.RelayerTxResponse, error) {
	// get the last finalized height
	lastFinalizedBlocks, err := v.cc.QueryLatestFinalizedBlocks(1)
	if err != nil {
		return nil, err
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

	if startHeight >= currentBlock.Height {
		return nil, fmt.Errorf("no need to catch up")
	}

	blocks, err := v.cc.QueryBlocks(startHeight, currentBlock.Height)
	if err != nil {
		return nil, err
	}

	catchUpBlocks := make([]*types.BlockInfo, 0, len(blocks))
	for _, b := range blocks {
		should, err := v.shouldSubmitFinalitySignature(b)
		if err != nil {
			return nil, err
		}
		if !should {
			// if false to the current block, so will be the rest
			break
		}
		catchUpBlocks = append(catchUpBlocks, b)
	}

	return v.SubmitBatchFinalitySignatures(catchUpBlocks)
}
