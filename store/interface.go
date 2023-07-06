package store

// Store is an abstraction for different key-value store implementations
type Store interface {
	// Put stores the given value for the given key.
	Put(k []byte, v []byte) error
	// Get retrieves the value for the given key.
	Get(k []byte) ([]byte, error)
	// Exists checks if a key exists in the store
	Exists(k []byte) (bool, error)
	// List returns the range of keys starting with the passed in prefix
	List(keyPrefix []byte) ([]*KVPair, error)
	// Delete deletes the stored value for the given key.
	Delete(k []byte) error
	// Close must be called when the work with the key-value store is done.
	Close() error
}

// KVPair represents {Key, Value} pair
type KVPair struct {
	Key   []byte
	Value []byte
}
