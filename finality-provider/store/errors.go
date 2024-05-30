package store

import "errors"

var (
	// ErrCorruptedFinalityProviderDb For some reason, db on disk representation have changed
	ErrCorruptedFinalityProviderDb = errors.New("finality provider db is corrupted")

	// ErrFinalityProviderNotFound The finality provider we try update is not found in db
	ErrFinalityProviderNotFound = errors.New("finality provider not found")

	// ErrDuplicateFinalityProvider The finality provider we try to add already exists in db
	ErrDuplicateFinalityProvider = errors.New("finality provider already exists")

	// ErrCorruptedPubRandProofDb For some reason, db on disk representation have changed
	ErrCorruptedPubRandProofDb = errors.New("public randomness proof db is corrupted")

	// ErrPubRandProofNotFound The finality provider we try update is not found in db
	ErrPubRandProofNotFound = errors.New("public randomness proof not found")

	// ErrDuplicatePubRand The public randomness proof we try to add already exists in db
	ErrDuplicatePubRand = errors.New("public randomness proof already exists")
)
