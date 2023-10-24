package eotsmanager

import (
	"fmt"

	"github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/eotsmanager/types"
	"github.com/babylonchain/btc-validator/store"
)

const (
	validatorKeyNamePrefix = "val-key"
)

type EOTSStore struct {
	s store.Store
}

func NewEOTSStore(cfg *config.DatabaseConfig) (*EOTSStore, error) {
	s, err := openStore(cfg)
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

func (es *EOTSStore) saveValidatorKey(pk []byte, keyName string) error {
	k := getValidatorKeyNameKey(pk)

	exists, err := es.s.Exists(k)
	if err != nil {
		return nil
	}
	if exists {
		return types.ErrValidatorAlreadyExisted
	}

	return es.s.Put(k, []byte(keyName))
}

func (es *EOTSStore) getValidatorKeyName(pk []byte) (string, error) {
	k := getValidatorKeyNameKey(pk)
	v, err := es.s.Get(k)
	if err != nil {
		return "", err
	}

	return string(v), nil
}

func getValidatorKeyNameKey(pk []byte) []byte {
	return append([]byte(validatorKeyNamePrefix), pk...)
}

func openStore(dbcfg *config.DatabaseConfig) (store.Store, error) {
	switch dbcfg.Backend {
	case "bbolt":
		return store.NewBboltStore(dbcfg.Path, dbcfg.Name)
	default:
		return nil, fmt.Errorf("unsupported database type")
	}
}
