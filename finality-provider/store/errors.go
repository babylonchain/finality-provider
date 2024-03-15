package store

import "errors"

var (
	// ErrCorruptedFinalityProviderDb For some reason, db on disk representation have changed
	ErrCorruptedFinalityProviderDb = errors.New("finality provider db is corrupted")

	// ErrFinalityProviderNotFound The finality provider we try update is not found in db
	ErrFinalityProviderNotFound = errors.New("finality provider not found")

	// ErrDuplicateFinalityProvider The finality provider we try to add already exists in db
	ErrDuplicateFinalityProvider = errors.New("finality provider already exists")
)
