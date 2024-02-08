package eotsmanager

import (
	"fmt"

	"github.com/babylonchain/finality-provider/eotsmanager/types"
	"github.com/babylonchain/finality-provider/store"
)

const (
	finalityProviderKeyNamePrefix = "fp-key"
)

type EOTSStore struct {
	s store.Store
}

func NewEOTSStore(dbPath string, dbName string, dbBackend string) (*EOTSStore, error) {
	s, err := openStore(dbPath, dbName, dbBackend)
	if err != nil {
		return nil, err
	}

	return &EOTSStore{s: s}, nil
}

func (es *EOTSStore) Close() error {
	if err := es.s.Close(); err != nil {
		return err
	}

	return nil
}

func (es *EOTSStore) saveFinalityProviderKey(pk []byte, keyName string) error {
	k := getFinalityProviderKeyNameKey(pk)

	exists, err := es.s.Exists(k)
	if err != nil {
		return nil
	}
	if exists {
		return types.ErrFinalityProviderAlreadyExisted
	}

	return es.s.Put(k, []byte(keyName))
}

func (es *EOTSStore) getFinalityProviderKeyName(pk []byte) (string, error) {
	k := getFinalityProviderKeyNameKey(pk)
	v, err := es.s.Get(k)
	if err != nil {
		return "", err
	}

	return string(v), nil
}

func getFinalityProviderKeyNameKey(pk []byte) []byte {
	return append([]byte(finalityProviderKeyNamePrefix), pk...)
}

func openStore(dbPath string, dbName string, dbBackend string) (store.Store, error) {
	switch dbBackend {
	case "bbolt":
		return store.NewBboltStore(dbPath, dbName)
	default:
		return nil, fmt.Errorf("unsupported database type")
	}
}
