package store

// Store is an abstraction for different key-value store implementations
type Store interface {
	// Put stores the given value for the given key.
	Put(k string, v []byte) error
	// Get retrieves the value for the given key.
	Get(k string) ([]byte, error)
	// Exists checks if a key exists in the store
	Exists(k string) (bool, error)
	// List returns the range of keys starting with the passed in prefix
	List(keyPrevix string) ([]*KVPair, error)
	// Delete deletes the stored value for the given key.
	Delete(k string) error
	// Close must be called when the work with the key-value store is done.
	Close() error
}

// KVPair represents {Key, Value} pair
type KVPair struct {
	Key   string
	Value []byte
}
