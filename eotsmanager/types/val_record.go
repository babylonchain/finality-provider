package types

import "github.com/btcsuite/btcd/btcec/v2"

type ValidatorRecord struct {
	ValName string
	ValSk   *btcec.PrivateKey
}
