package bbolt

import (
	"bytes"
	"errors"

	bolt "go.etcd.io/bbolt"

	"github.com/babylonchain/btc-validator/store"
)

// BboltStore implements the Store interface
type BboltStore struct {
	db         *bolt.DB
	bucketName string
}

// Put stores the given value for the given key.
// Values are automatically marshalled to JSON or gob.
// The key must not be "" and the value must not be nil.
func (s BboltStore) Put(k []byte, v []byte) error {
	if err := checkKeyAndValue(k, v); err != nil {
		return err
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		return b.Put(k, v)
	})
	if err != nil {
		return err
	}
	return nil
}

// Get retrieves the stored value for the given key.
func (s BboltStore) Get(k []byte) ([]byte, error) {
	if err := checkKey(k); err != nil {
		return nil, err
	}

	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		data = b.Get(k)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := checkValue(data); err != nil {
		return nil, err
	}

	return data, nil
}

// Exists checks whether the given key exists in the store.
func (s BboltStore) Exists(k []byte) (bool, error) {
	if err := checkKey(k); err != nil {
		return false, err
	}

	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		data = b.Get(k)
		return checkValue(data)
	})

	if err != nil {
		return false, nil
	}

	return true, nil
}

func (s BboltStore) List(keyPrefix []byte) ([]*store.KVPair, error) {
	if len(keyPrefix) == 0 {
		return s.listFromStart()
	}

	var kvList []*store.KVPair

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		cursor := b.Cursor()
		prefix := keyPrefix

		for key, v := cursor.Seek(prefix); bytes.HasPrefix(key, prefix); key, v = cursor.Next() {
			if err := checkValue(v); err != nil {
				return err
			}
			kvList = append(kvList, &store.KVPair{
				Key:   key,
				Value: v,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return kvList, nil
}

func (s BboltStore) listFromStart() ([]*store.KVPair, error) {
	var kvList []*store.KVPair

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		cursor := b.Cursor()

		for key, v := cursor.First(); ; key, v = cursor.Next() {
			if key == nil {
				break
			}
			if err := checkValue(v); err != nil {
				return err
			}
			kvList = append(kvList, &store.KVPair{
				Key:   key,
				Value: v,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return kvList, nil
}

// Delete deletes the stored value for the given key.
// Deleting a non-existing key-value pair does NOT lead to an error.
func (s BboltStore) Delete(k []byte) error {
	if err := checkKey(k); err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		return b.Delete(k)
	})
}

// Close closes the store.
// It must be called to make sure that all open transactions finish and to release all DB resources.
func (s BboltStore) Close() error {
	return s.db.Close()
}

// Options are the options for the bbolt store.
type Options struct {
	// Bucket name for storing the key-value pairs.
	// Optional ("default" by default).
	BucketName string
	// Path of the DB file.
	// Optional ("bbolt.db" by default).
	Path string
}

// DefaultOptions is an Options object with default values.
// BucketName: "default", Path: "bbolt.db", Codec: encoding.JSON
var DefaultOptions = Options{
	BucketName: "default",
	Path:       "bbolt.db",
}

// NewBboltStore creates a new bbolt store.
// Note: bbolt uses an exclusive write lock on the database file so it cannot be shared by multiple processes.
// So when creating multiple clients you should always use a new database file (by setting a different Path in the options).
//
// You must call the Close() method on the store when you're done working with it.
func NewBboltStore(options Options) (BboltStore, error) {
	result := BboltStore{}

	// Set default values
	if options.BucketName == "" {
		options.BucketName = DefaultOptions.BucketName
	}
	if options.Path == "" {
		options.Path = DefaultOptions.Path
	}

	// Open DB
	db, err := bolt.Open(options.Path, 0600, nil)
	if err != nil {
		return result, err
	}

	// Create a bucket if it doesn't exist yet.
	// In bbolt key/value pairs are stored to and read from buckets.
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(options.BucketName))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	result.db = db
	result.bucketName = options.BucketName

	return result, nil
}

// checkKey returns an error if k == ""
func checkKey(k []byte) error {
	if len(k) == 0 {
		return errors.New("the key should not be empty")
	}
	return nil
}

func checkValue(v []byte) error {
	if v == nil {
		return errors.New("the value should not be nil")
	}

	return nil
}

// checkKeyAndValue returns an error if k == "" or if v == nil
func checkKeyAndValue(k []byte, v []byte) error {
	if err := checkKey(k); err != nil {
		return err
	}
	return checkValue(v)
}
