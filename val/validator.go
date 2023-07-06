package val

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/babylonchain/btc-validator/valrpc"
)

func NewValidator(babylonPk *btcec.PublicKey, btcPk *btcec.PublicKey) *valrpc.Validator {
	return &valrpc.Validator{
		BabylonPk: babylonPk.SerializeCompressed(),
		BtcPk:     btcPk.SerializeCompressed(),
		Status:    valrpc.ValidatorStatus_VALIDATOR_STATUS_CREATED,
	}
}

func GenerateValPrivKeys() (bbnPrivKey *btcec.PrivateKey, btcPrivKey *btcec.PrivateKey, err error) {
	// generate Babylon private key
	bbnPrivKey, err = btcec.NewPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate Babylon private key: %w", err)
	}

	// generate BTC private key
	btcPrivKey, err = btcec.NewPrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate BTC private key: %w", err)
	}

	return bbnPrivKey, btcPrivKey, nil
}
