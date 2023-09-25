package local

import (
	"fmt"

	gproto "google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
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
