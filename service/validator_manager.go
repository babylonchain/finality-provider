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

func NewValidatorManager() *ValidatorManager {
	return &ValidatorManager{
		vals: make(map[string]*ValidatorInstance),
	}
}

func (vm *ValidatorManager) stop() error {
	var stopErr error
	for _, v := range vm.vals {
		if !v.started.Load() {
			continue
		}
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

// addValidatorInstance creates a validator instance, starts it and adds it into the validator manager
func (vm *ValidatorManager) addValidatorInstance(
	pk *secp256k1.PubKey,
	config *valcfg.Config,
	valStore *val.ValidatorStore,
	kr keyring.Keyring,
	bc bbncli.BabylonClient,
	logger *logrus.Logger,
) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	pkHex := hex.EncodeToString(pk.Key)
	if _, exists := vm.vals[pkHex]; exists {
		return fmt.Errorf("validator instance already exists")
	}

	valIns, err := NewValidatorInstance(pk, config, valStore, kr, bc, logger)
	if err != nil {
		return fmt.Errorf("failed to create validator %s instance: %w", pkHex, err)
	}

	if err := valIns.Start(); err != nil {
		return fmt.Errorf("failed to start validator %s instance: %w", pkHex, err)
	}

	vm.vals[pkHex] = valIns

	return nil
}
