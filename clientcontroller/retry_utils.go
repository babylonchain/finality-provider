package clientcontroller

import (
	"strings"
	"time"

	errorsmod "cosmossdk.io/errors"
	"github.com/avast/retry-go/v4"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
)

// Variables used for retries
var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Millisecond * 400)
	rtyErr    = retry.LastErrorOnly(true)
)

// these errors are considered unrecoverable because these indicate
// something critical in the validator program or the Babylon server
var unrecoverableErrors = []*errorsmod.Error{
	ftypes.ErrBlockNotFound,
	ftypes.ErrInvalidFinalitySig,
	ftypes.ErrHeightTooHigh,
	ftypes.ErrInvalidPubRand,
	ftypes.ErrNoPubRandYet,
	ftypes.ErrPubRandNotFound,
	ftypes.ErrTooFewPubRand,
	bstypes.ErrBTCValAlreadySlashed,
}

// IsUnrecoverable returns true when the error is in the unrecoverableErrors list
func IsUnrecoverable(err error) bool {
	for _, e := range unrecoverableErrors {
		if strings.Contains(err.Error(), e.Error()) {
			return true
		}
	}

	return false
}

var expectedErrors = []*errorsmod.Error{
	// if due to some low-level reason (e.g., network), we submit duplicated finality sig,
	// we should just ignore the error
	ftypes.ErrDuplicatedFinalitySig,
}

// IsExpected returns true when the error is in the expectedErrors list
func IsExpected(err error) bool {
	for _, e := range expectedErrors {
		if strings.Contains(err.Error(), e.Error()) {
			return true
		}
	}

	return false
}
