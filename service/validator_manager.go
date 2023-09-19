package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/clientcontroller"
	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/val"
	"github.com/babylonchain/btc-validator/valcfg"
)

const instanceTerminatingMsg = "terminating the validator instance due to critical error"

type CriticalError struct {
	err error
	// TODO use validator BTC key as the unique id of
	//  the validator; currently, the storage is keyed
	//  the babylon public key
	valBtcPk *types.BIP340PubKey
	bbnPk    *secp256k1.PubKey
}

type ValidatorManager struct {
	mu sync.Mutex
	// validator instances map keyed by the hex string of the BTC public key
	vals map[string]*ValidatorInstance

	criticalErrChan chan *CriticalError

	quit chan struct{}
}

func NewValidatorManager() *ValidatorManager {
	vm := &ValidatorManager{
		vals:            make(map[string]*ValidatorInstance),
		criticalErrChan: make(chan *CriticalError),
		quit:            make(chan struct{}),
	}

	go vm.monitorCriticalErr()

	return vm
}

// monitorCriticalErr takes actions when it receives critical errors from a validator instance
// if the validator is slashed, it will be terminated and the program keeps running in case
// new validators join
// otherwise, the program will panic
func (vm *ValidatorManager) monitorCriticalErr() {
	var criticalErr *CriticalError
	for {
		select {
		case criticalErr = <-vm.criticalErrChan:
			vi, err := vm.getValidatorInstance(criticalErr.bbnPk)
			if err != nil {
				panic(fmt.Errorf("failed to get the validator instance: %w", err))
			}
			if strings.Contains(criticalErr.err.Error(), bstypes.ErrBTCValAlreadySlashed.Error()) {
				vi.MustSetStatus(proto.ValidatorStatus_SLASHED)
				if err := vm.removeValidatorInstance(vi.GetBabylonPk()); err != nil {
					panic(fmt.Errorf("failed to terminate a slashed validator %s: %w", vi.GetBtcPkHex(), err))
				}
				continue
			}
			vi.logger.WithFields(logrus.Fields{
				"err":        criticalErr.err,
				"btc_pk_hex": vi.GetBtcPkHex(),
			}).Fatal(instanceTerminatingMsg)
		case <-vm.quit:
			return
		}
	}
}

func (vm *ValidatorManager) stop() error {
	var stopErr error

	close(vm.quit)

	for _, v := range vm.vals {
		if err := v.Stop(); err != nil {
			stopErr = err
			break
		}
	}

	return stopErr
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

func (vm *ValidatorManager) removeValidatorInstance(babylonPk *secp256k1.PubKey) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	keyHex := hex.EncodeToString(babylonPk.Key)
	v, exists := vm.vals[keyHex]
	if !exists {
		return fmt.Errorf("cannot find the validator instance with PK: %s", keyHex)
	}
	if v.IsRunning() {
		if err := v.Stop(); err != nil {
			return fmt.Errorf("failed to stop the validator instance %s", keyHex)
		}
	}

	delete(vm.vals, keyHex)
	return nil
}

// addValidatorInstance creates a validator instance, starts it and adds it into the validator manager
func (vm *ValidatorManager) addValidatorInstance(
	pk *secp256k1.PubKey,
	config *valcfg.Config,
	valStore *val.ValidatorStore,
	kr keyring.Keyring,
	cc clientcontroller.ClientController,
	logger *logrus.Logger,
) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	pkHex := hex.EncodeToString(pk.Key)
	if _, exists := vm.vals[pkHex]; exists {
		return fmt.Errorf("validator instance already exists")
	}

	valIns, err := NewValidatorInstance(pk, config, valStore, kr, cc, vm.criticalErrChan, logger)
	if err != nil {
		return fmt.Errorf("failed to create validator %s instance: %w", pkHex, err)
	}

	if err := valIns.Start(); err != nil {
		return fmt.Errorf("failed to start validator %s instance: %w", pkHex, err)
	}

	vm.vals[pkHex] = valIns

	return nil
}
