package types

import "github.com/btcsuite/btcd/btcec/v2"

type KeyPair struct {
	PublicKey  *btcec.PublicKey
	PrivateKey *btcec.PrivateKey
}
