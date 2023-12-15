package types

import "github.com/btcsuite/btcd/btcec/v2"

type KeyInfo struct {
	Name       string
	Mnemonic   string
	Address    string
	PublicKey  *btcec.PublicKey
	PrivateKey *btcec.PrivateKey
}
