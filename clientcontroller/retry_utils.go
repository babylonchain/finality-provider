package clientcontroller

import (
	"errors"
	"time"

	sdkErr "cosmossdk.io/errors"
	"github.com/avast/retry-go/v4"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
)

// Variables used for retries
var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Millisecond * 400)
	rtyErr    = retry.LastErrorOnly(true)
)

// these errors are considered unrecoverable because these indicate
// something critical in the validator program or the consumer chain
var unrecoverableErrors = []*sdkErr.Error{
	finalitytypes.ErrBlockNotFound,
	finalitytypes.ErrInvalidFinalitySig,
	finalitytypes.ErrHeightTooHigh,
	finalitytypes.ErrInvalidPubRand,
	finalitytypes.ErrNoPubRandYet,
	finalitytypes.ErrPubRandNotFound,
	finalitytypes.ErrTooFewPubRand,
	btcstakingtypes.ErrBTCValAlreadySlashed,
}

// IsUnrecoverable returns true when the error is in the unrecoverableErrors list
func IsUnrecoverable(err error) bool {
	for _, e := range unrecoverableErrors {
		if errors.Is(err, e) {
			return true
		}
	}

	return false
}

var expectedErrors = []*sdkErr.Error{
	// if due to some low-level reason (e.g., network), we submit duplicated finality sig,
	// we should just ignore the error
	finalitytypes.ErrDuplicatedFinalitySig,
}

// IsExpected returns true when the error is in the expectedErrors list
func IsExpected(err error) bool {
	for _, e := range expectedErrors {
		if errors.Is(err, e) {
			return true
		}
	}

	return false
}
