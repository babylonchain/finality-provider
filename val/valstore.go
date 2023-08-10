package val

import (
	"fmt"
	"math"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	gproto "google.golang.org/protobuf/proto"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/store"
	"github.com/babylonchain/btc-validator/valcfg"
)

const (
	validatorPrefix = "validator"
	randPairPrefix  = "rand-pair"
)

func NewValidatorStored(babylonPk *secp256k1.PubKey, btcPk *types.BIP340PubKey, keyName string, pop *bstypes.ProofOfPossession) *proto.ValidatorStored {
	return &proto.ValidatorStored{
		KeyName:   keyName,
		BabylonPk: babylonPk.Bytes(),
		BtcPk:     btcPk.MustMarshal(),
		Pop: &proto.ProofOfPossession{
			BabylonSig: pop.BabylonSig,
			BtcSig:     pop.BtcSig.MustMarshal(),
		},
		Status: proto.ValidatorStatus_CREATED,
	}
}

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
	return append(vs.getRandPairListKey(pk), sdktypes.Uint64ToBigEndian(height)...)
}

func (vs *ValidatorStore) getRandPairListKey(pk []byte) []byte {
	return append([]byte(randPairPrefix), pk...)
}

func (vs *ValidatorStore) SaveValidator(val *proto.ValidatorStored) error {
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

func (vs *ValidatorStore) UpdateValidator(val *proto.ValidatorStored) error {
	k := vs.getValidatorKey(val.BabylonPk)
	exists, err := vs.s.Exists(k)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("the validator does not exist")
	}

	v, err := gproto.Marshal(val)
	if err != nil {
		return err
	}

	if err := vs.s.Put(k, v); err != nil {
		return err
	}

	return nil
}

func (vs *ValidatorStore) SetValidatorStatus(val *proto.ValidatorStored, status proto.ValidatorStatus) error {
	val.Status = status
	return vs.UpdateValidator(val)
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

func (vs *ValidatorStore) GetValidatorStored(pk []byte) (*proto.ValidatorStored, error) {
	k := vs.getValidatorKey(pk)
	valsBytes, err := vs.s.Get(k)
	if err != nil {
		return nil, err
	}

	val := new(proto.ValidatorStored)
	err = gproto.Unmarshal(valsBytes, val)
	if err != nil {
		panic(fmt.Errorf("unable to unmarshal validator object: %w", err))
	}

	return val, nil
}

func (vs *ValidatorStore) ListValidators() ([]*proto.ValidatorStored, error) {
	k := vs.getValidatorListKey()
	valsBytes, err := vs.s.List(k)
	if err != nil {
		return nil, err
	}

	valsList := make([]*proto.ValidatorStored, len(valsBytes))
	for i := 0; i < len(valsBytes); i++ {
		val := new(proto.ValidatorStored)
		err := gproto.Unmarshal(valsBytes[i].Value, val)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal validator from the database: %w", err))
		}
		valsList[i] = val
	}

	return valsList, nil
}

// ListRegisteredValidators returns a list of validators whose status is more than CREATED
func (vs *ValidatorStore) ListRegisteredValidators() ([]*proto.ValidatorStored, error) {
	k := vs.getValidatorListKey()
	valsBytes, err := vs.s.List(k)
	if err != nil {
		return nil, err
	}

	valsList := make([]*proto.ValidatorStored, 0)
	for i := 0; i < len(valsBytes); i++ {
		val := new(proto.ValidatorStored)
		err := gproto.Unmarshal(valsBytes[i].Value, val)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal validator from the database: %w", err))
		}
		if val.Status >= proto.ValidatorStatus_CREATED {
			valsList = append(valsList, val)
		}
	}

	return valsList, nil
}

func (vs *ValidatorStore) GetEarliestActiveValidatorVotedHeight() (uint64, error) {
	registeredValidators, err := vs.ListRegisteredValidators()
	if err != nil {
		return 0, err
	}

	if len(registeredValidators) == 0 {
		return 0, nil
	}

	earliestHeight := uint64(math.MaxUint64)
	activeValsCnt := 0
	for _, val := range registeredValidators {
		// Note there might be a delay between the validator being active on Babylon
		// and this program capturing that. However, given that we only care
		// about the `LastVotedHeight` of the validator, other parts of the program
		// ensure that when this value is set, the validator is stored as ACTIVE.
		// TODO: Another option would be to query here for the
		// active status of each validator although this might prove inefficient.
		if val.Status != proto.ValidatorStatus_ACTIVE {
			continue
		}
		activeValsCnt += 1
		if earliestHeight > val.LastVotedHeight {
			earliestHeight = val.LastVotedHeight
		}
	}
	// If there are no active validators, return 0
	if activeValsCnt == 0 {
		return 0, nil
	}
	return earliestHeight, nil
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
