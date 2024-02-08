package types

import "errors"

var (
	ErrFinalityProviderAlreadyExisted = errors.New("the finality provider has already existed")
)
