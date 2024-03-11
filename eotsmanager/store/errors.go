package store

import "errors"

var (
	// ErrCorruptedEOTSDb For some reason, db on disk representation have changed
	ErrCorruptedEOTSDb = errors.New("EOTS manager db is corrupted")

	// ErrDuplicateEOTSKeyName The EOTS key name we try to add already exists in db
	ErrDuplicateEOTSKeyName = errors.New("EOTS key name already exists")

	// ErrEOTSKeyNameNotFound The EOTS key name we try to fetch is not found in db
	ErrEOTSKeyNameNotFound = errors.New("EOTS key name not found")
)
