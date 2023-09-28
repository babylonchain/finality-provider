package eotsmanager

import "errors"

var (
	ErrValidatorAlreadyExisted         = errors.New("the validator has already existed")
	ErrSchnorrRandomnessALreadyCreated = errors.New("the Schnorr randomness has already been created")
)
