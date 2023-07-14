package val

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/btc-validator/valrpc"
)

type ValidatorStore struct {
	s store.Store
}

func NewValidatorStore(dbcfg *valcfg.DatabaseConfig) (*ValidatorStore, error) {
	s, err := openStore(dbcfg)
	if err != nil {
		return nil, err
	}

	return &ValidatorStore{s: s}, nil
}

func (vs *ValidatorStore) SaveValidator(val *valrpc.Validator) error {
	k := val.BabylonPk
	v, err := proto.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marshal the created validator object: %w", err)
	}

	if err := vs.s.Put(k, v); err != nil {
		return fmt.Errorf("failed to save the created validator object: %w", err)
	}

	return nil
}

func (vs *ValidatorStore) GetValidator(pk []byte) (*valrpc.Validator, error) {
	valsBytes, err := vs.s.Get(pk)
	if err != nil {
		return nil, err
	}

	val := new(valrpc.Validator)
	err = proto.Unmarshal(valsBytes, val)

	return val, nil
}

func (vs *ValidatorStore) ListValidators() ([]*valrpc.Validator, error) {
	valsBytes, err := vs.s.List(nil)
	if err != nil {
		return nil, err
	}

	valsList := make([]*valrpc.Validator, len(valsBytes))
	for i := 0; i < len(valsBytes); i++ {
		val := new(valrpc.Validator)
		err := proto.Unmarshal(valsBytes[i].Value, val)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal validator from the database: %w", err))
		}
		valsList[i] = val
	}

	return valsList, nil
}

func (vs *ValidatorStore) Close() error {
	if err := vs.s.Close(); err != nil {
		return err
	}

	return nil
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
