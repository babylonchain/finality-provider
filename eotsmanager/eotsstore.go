package eotsmanager

import (
	"fmt"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	gproto "google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/eotsmanager/types"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/store"
)

const (
	randPairPrefix         = "rand-pair"
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

func (es *EOTSStore) saveRandPair(pk []byte, chainID []byte, height uint64, randPair *proto.SchnorrRandPair) error {
	k := getRandPairKey(pk, chainID, height)
	v, err := gproto.Marshal(randPair)
	if err != nil {
		return fmt.Errorf("failed to marshal the Schnorr random pair: %w", err)
	}

	if err := es.s.Put(k, v); err != nil {
		return fmt.Errorf("failed to save the Schnorr random pair: %w", err)
	}

	return nil
}

func (es *EOTSStore) getRandPair(pk []byte, chainID []byte, height uint64) (*proto.SchnorrRandPair, error) {
	k := getRandPairKey(pk, chainID, height)
	v, err := es.s.Get(k)
	if err != nil {
		return nil, err
	}

	pair := new(proto.SchnorrRandPair)
	if err := gproto.Unmarshal(v, pair); err != nil {
		return nil, err
	}

	return pair, nil
}

func (es *EOTSStore) randPairExists(pk []byte, chainID []byte, height uint64) (bool, error) {
	k := getRandPairKey(pk, chainID, height)
	return es.s.Exists(k)
}

func getRandPairKey(pk []byte, chainID []byte, height uint64) []byte {
	return append(getRandPairListKey(pk, chainID), sdktypes.Uint64ToBigEndian(height)...)
}

func getRandPairListKey(pk []byte, chainID []byte) []byte {
	return append(append([]byte(randPairPrefix), pk...), chainID...)
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
