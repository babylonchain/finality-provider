package types

import "errors"

var (
	ErrValidatorAlreadyExisted         = errors.New("the validator has already existed")
	ErrSchnorrRandomnessAlreadyCreated = errors.New("the Schnorr randomness has already been created")
)
