package service

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"

	bbncli "github.com/babylonchain/btc-validator/bbnclient"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

type ValidatorManager struct {
	mu sync.Mutex
	// validator instances map keyed by the hex string of the Babylon public key
	vals map[string]*ValidatorInstance
}

func NewValidatorManagerFromStore(
	valStore *val.ValidatorStore,
	config *valcfg.Config,
	kr keyring.Keyring,
	bc bbncli.BabylonClient,
	logger *logrus.Logger,
) (*ValidatorManager, error) {
	storedVals, err := valStore.ListRegisteredValidators()
	if err != nil {
		return nil, fmt.Errorf("failed to list registered validators: %w", err)
	}
	vals := make(map[string]*ValidatorInstance)
	for _, sv := range storedVals {
		validator, err := NewValidatorInstance(sv.GetBabylonPK(), config, valStore, kr, bc, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create validator %s instance: %w", sv.GetBabylonPkHexString(), err)
		}
		vals[sv.GetBabylonPkHexString()] = validator
	}

	return &ValidatorManager{vals: vals}, nil
}

func (vm *ValidatorManager) start() error {
	var startErr error
	for _, v := range vm.vals {
		if err := v.Start(); err != nil {
			startErr = err
			break
		}
	}

	return startErr
}

func (vm *ValidatorManager) stop() error {
	var stopErr error
	for _, v := range vm.vals {
		if err := v.Stop(); err != nil {
			stopErr = err
			break
		}
	}

	return stopErr
}

func (vm *ValidatorManager) receiveBlock(b *BlockInfo) {
	for _, v := range vm.vals {
		v.receiveBlock(b)
	}
}

func (vm *ValidatorManager) listValidatorInstances() []*ValidatorInstance {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	valsList := make([]*ValidatorInstance, 0, len(vm.vals))
	for _, v := range vm.vals {
		valsList = append(valsList, v)
	}

	return valsList
}

func (vm *ValidatorManager) getValidatorInstance(babylonPk *secp256k1.PubKey) (*ValidatorInstance, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	keyHex := hex.EncodeToString(babylonPk.Key)
	v, exists := vm.vals[keyHex]
	if !exists {
		return nil, fmt.Errorf("cannot find the validator instance with PK: %s", keyHex)
	}

	return v, nil
}

func (vm *ValidatorManager) addValidatorInstance(valIns *ValidatorInstance) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	k := valIns.GetBabylonPkHex()
	if _, exists := vm.vals[k]; exists {
		return fmt.Errorf("validator instance already exists")
	}
	vm.vals[k] = valIns

	return nil
}
