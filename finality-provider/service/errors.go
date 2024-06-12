package service

import "errors"

var (
	ErrFinalityProviderShutDown = errors.New("the finality provider instance is shutting down")
)
