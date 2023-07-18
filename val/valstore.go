package val

import (
	"fmt"

	gproto "google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/valcfg"
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

func (vs *ValidatorStore) SaveValidator(val *proto.Validator) error {
	k := val.BabylonPk
	v, err := gproto.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marshal the created validator object: %w", err)
	}

	if err := vs.s.Put(k, v); err != nil {
		return fmt.Errorf("failed to save the created validator object: %w", err)
	}

	return nil
}

func (vs *ValidatorStore) GetValidator(pk []byte) (*proto.Validator, error) {
	valsBytes, err := vs.s.Get(pk)
	if err != nil {
		return nil, err
	}

	val := new(proto.Validator)
	err = gproto.Unmarshal(valsBytes, val)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal validator object: %w", err))
	}

	return val, nil
}

func (vs *ValidatorStore) ListValidators() ([]*proto.Validator, error) {
	valsBytes, err := vs.s.List(nil)
	if err != nil {
		return nil, err
	}

	valsList := make([]*proto.Validator, len(valsBytes))
	for i := 0; i < len(valsBytes); i++ {
		val := new(proto.Validator)
		err := gproto.Unmarshal(valsBytes[i].Value, val)
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
