package val

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/types"
	gproto "google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	validatorPrefix = "validator"
	randPairPrefix  = "rand-pair"
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

func (vs *ValidatorStore) getValidatorKey(pk []byte) []byte {
	return append([]byte(validatorPrefix), pk...)
}

func (vs *ValidatorStore) getValidatorListKey() []byte {
	return []byte(validatorPrefix)
}

func (vs *ValidatorStore) getRandPairKey(pk []byte, height uint64) []byte {
	return append(vs.getRandPairListKey(pk), types.Uint64ToBigEndian(height)...)
}

func (vs *ValidatorStore) getRandPairListKey(pk []byte) []byte {
	return append([]byte(randPairPrefix), pk...)
}

func (vs *ValidatorStore) SaveValidator(val *proto.Validator) error {
	k := vs.getValidatorKey(val.BabylonPk)
	v, err := gproto.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marshal the created validator object: %w", err)
	}

	if err := vs.s.Put(k, v); err != nil {
		return fmt.Errorf("failed to save the created validator object: %w", err)
	}

	return nil
}

func (vs *ValidatorStore) SetValidatorLastVotedHeight(val *proto.Validator, height uint64) error {
	val.LastVotedHeight = height
	return vs.SaveValidator(val)
}

func (vs *ValidatorStore) SetValidatorStatus(val *proto.Validator, status proto.ValidatorStatus) error {
	val.Status = status
	return vs.SaveValidator(val)
}

func (vs *ValidatorStore) SaveRandPair(pk []byte, height uint64, randPair *proto.SchnorrRandPair) error {
	k := vs.getRandPairKey(pk, height)
	v, err := gproto.Marshal(randPair)
	if err != nil {
		return fmt.Errorf("failed to marshal the Schnorr random pair: %w", err)
	}

	if err := vs.s.Put(k, v); err != nil {
		return fmt.Errorf("failed to save the Schnorr random pair: %w", err)
	}

	return nil
}

func (vs *ValidatorStore) GetRandPairList(pk []byte) ([]*proto.SchnorrRandPair, error) {
	k := vs.getRandPairListKey(pk)
	pairsBytes, err := vs.s.List(k)
	if err != nil {
		return nil, err
	}

	pairList := make([]*proto.SchnorrRandPair, len(pairsBytes))
	for i := 0; i < len(pairsBytes); i++ {
		pair := new(proto.SchnorrRandPair)
		err := gproto.Unmarshal(pairsBytes[i].Value, pair)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal Schnorr randomness pair from the database: %w", err))
		}
		pairList[i] = pair
	}

	return pairList, nil
}

func (vs *ValidatorStore) GetRandPair(pk []byte, height uint64) (*proto.SchnorrRandPair, error) {
	k := vs.getRandPairKey(pk, height)
	v, err := vs.s.Get(k)
	if err != nil {
		return nil, fmt.Errorf("failed to get the randomness pair from DB: %w", err)
	}
	pair := new(proto.SchnorrRandPair)
	err = gproto.Unmarshal(v, pair)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal Schnorr randomness pair: %w", err))
	}

	return pair, nil
}

func (vs *ValidatorStore) GetValidator(pk []byte) (*proto.Validator, error) {
	k := vs.getValidatorKey(pk)
	valsBytes, err := vs.s.Get(k)
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
	k := vs.getValidatorListKey()
	valsBytes, err := vs.s.List(k)
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

func (vs *ValidatorStore) ListRegisteredValidators() ([]*proto.Validator, error) {
	k := vs.getValidatorListKey()
	valsBytes, err := vs.s.List(k)
	if err != nil {
		return nil, err
	}

	valsList := make([]*proto.Validator, 0)
	for i := 0; i < len(valsBytes); i++ {
		val := new(proto.Validator)
		err := gproto.Unmarshal(valsBytes[i].Value, val)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal validator from the database: %w", err))
		}
		if val.Status != proto.ValidatorStatus_CREATED {
			valsList = append(valsList, val)
		}
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
