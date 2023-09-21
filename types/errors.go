package types

import (
	"errors"
)

var (
	ErrValidatorSlashed      = errors.New("the validator has been slashed")
	ErrTooFewPubRand         = errors.New("the request contains too few public randomness")
	ErrBlockNotFound         = errors.New("the block is not found")
	ErrHeightTooHigh         = errors.New("the chain has not reached the given height yet")
	ErrInvalidPubRand        = errors.New("the public randomness list is invalid")
	ErrNoPubRandYet          = errors.New("the BTC validator has not committed any public randomness yet")
	ErrPubRandNotFound       = errors.New("public randomness is not found")
	ErrInvalidFinalitySig    = errors.New("finality signature is not valid")
	ErrDuplicatedFinalitySig = errors.New("the finality signature has been casted before")
)
