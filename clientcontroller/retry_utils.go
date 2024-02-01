package clientcontroller

import (
	"errors"

	sdkErr "cosmossdk.io/errors"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
)

// these errors are considered unrecoverable because these indicate
// something critical in the finality provider program or the consumer chain
var unrecoverableErrors = []*sdkErr.Error{
	finalitytypes.ErrBlockNotFound,
	finalitytypes.ErrInvalidFinalitySig,
	finalitytypes.ErrNoPubRandYet,
	finalitytypes.ErrPubRandNotFound,
	finalitytypes.ErrTooFewPubRand,
	btcstakingtypes.ErrFpAlreadySlashed,
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

type ExpectedError struct {
	error
}

// Expected wraps an error in ExpectedError struct
func Expected(err error) error {
	return ExpectedError{err}
}

// IsExpected checks if error is an instance of ExpectedError
func IsExpected(err error) bool {
	return errors.Is(err, ExpectedError{})
}
