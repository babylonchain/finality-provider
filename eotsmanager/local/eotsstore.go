package local

import (
	"fmt"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	gproto "google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	randPairPrefix = "rand-pair"
)

type EOTSStore struct {
	s store.Store
}

func NewEOTSStore(dbcfg *valcfg.DatabaseConfig) (*EOTSStore, error) {
	s, err := openStore(dbcfg)
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

func (es *EOTSStore) saveValidator(storeVal *proto.StoreValidator) error {
	k := val.GetValidatorKey(storeVal.BtcPk)

	exists, err := es.s.Exists(k)
	if err != nil {
		return nil
	}
	if exists {
		return eotsmanager.ErrValidatorAlreadyExisted
	}

	v, err := gproto.Marshal(storeVal)
	if err != nil {
		return err
	}

	return es.s.Put(k, v)
}

func (es *EOTSStore) saveRandPair(pk []byte, chainID string, height uint64, randPair *proto.SchnorrRandPair) error {
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

func getRandPairKey(pk []byte, chainID string, height uint64) []byte {
	return append(getRandPairListKey(pk, chainID), sdktypes.Uint64ToBigEndian(height)...)
}

func getRandPairListKey(pk []byte, chainID string) []byte {
	return append(append([]byte(randPairPrefix), pk...), []byte(chainID)...)
}

func (es *EOTSStore) getValidatorKeyName(pk []byte) (string, error) {
	k := val.GetValidatorKey(pk)
	v, err := es.s.Get(k)
	if err != nil {
		return "", err
	}

	validator := new(proto.StoreValidator)
	if err := gproto.Unmarshal(v, validator); err != nil {
		return "", err
	}

	return validator.KeyName, nil
}

// openStore returns a Store instance with the given db type, path and name
// currently, we only support bbolt
func openStore(dbcfg *valcfg.DatabaseConfig) (store.Store, error) {
	switch dbcfg.Backend {
	case "bbolt":
		return store.NewBboltStore(dbcfg.Path, dbcfg.Name)
	default:
		return nil, fmt.Errorf("unsupported database type")
	}
}
