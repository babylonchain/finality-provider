package service

import (
	"fmt"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/val"
)

func (app *ValidatorApp) getValidatorStore() (*val.ValidatorStore, error) {
	valStore, err := val.NewValidatorStore(app.config.DatabaseConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open the store for validators: %w", err)
	}

	return valStore, nil
}

func (app *ValidatorApp) accessValidatorStore(accessFunc func(valStore *val.ValidatorStore) error) error {
	valStore, err := app.getValidatorStore()
	if err != nil {
		return err
	}
	if err := accessFunc(valStore); err != nil {
		return err
	}
	if err := valStore.Close(); err != nil {
		return err
	}
	return nil
}

func (app *ValidatorApp) SaveValidator(validator *proto.Validator) error {
	err := app.accessValidatorStore(func(valStore *val.ValidatorStore) error {
		return valStore.SaveValidator(validator)
	})
	return err
}

func (app *ValidatorApp) GetValidator(pkBytes []byte) (*proto.Validator, error) {
	var validator *proto.Validator
	err := app.accessValidatorStore(func(valStore *val.ValidatorStore) error {
		val, err := valStore.GetValidator(pkBytes)
		if err != nil {
			return err
		}
		validator = val
		return nil
	})
	if err != nil {
		return nil, err
	}
	return validator, nil
}

func (app *ValidatorApp) ListValidators() ([]*proto.Validator, error) {
	var validators []*proto.Validator
	err := app.accessValidatorStore(func(valStore *val.ValidatorStore) error {
		vals, err := valStore.ListValidators()
		if err != nil {
			return err
		}
		validators = vals
		return nil
	})
	if err != nil {
		return nil, err
	}
	return validators, nil
}

func (app *ValidatorApp) SaveRandPair(pkBytes []byte, height uint64, randPair *proto.SchnorrRandPair) error {
	return app.accessValidatorStore(func(valStore *val.ValidatorStore) error {
		return valStore.SaveRandPair(pkBytes, height, randPair)
	})
}

func (app *ValidatorApp) GetRandPair(pkBytes []byte, height uint64) (*proto.SchnorrRandPair, error) {
	var randPair *proto.SchnorrRandPair
	err := app.accessValidatorStore(func(valStore *val.ValidatorStore) error {
		rp, err := valStore.GetRandPair(pkBytes, height)
		if err != nil {
			return err
		}
		randPair = rp
		return nil
	})
	if err != nil {
		return nil, err
	}
	return randPair, nil
}
