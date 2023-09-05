package clientcontroller

import (
	"strings"
	"time"

	errorsmod "cosmossdk.io/errors"
	"github.com/avast/retry-go/v4"
	"github.com/babylonchain/babylon/x/finality/types"
	"github.com/cosmos/cosmos-sdk/types/errors"
)

// Variables used for retries
var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Millisecond * 400)
	rtyErr    = retry.LastErrorOnly(true)
)

var retriableErrors = []*errorsmod.Error{
	errors.ErrInsufficientFunds,
	errors.ErrMempoolIsFull,
}

// IsRetriable returns true when the error is in the retriableErrors list
func IsRetriable(err error) bool {
	for _, e := range retriableErrors {
		if strings.Contains(err.Error(), e.Error()) {
			return true
		}
	}

	return false
}

var expectedErrors = []*errorsmod.Error{
	types.ErrDuplicatedFinalitySig,
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
