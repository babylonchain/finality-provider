package val

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

func CreateValidator(pubkey *secp256k1.PubKey, key *btcec.PublicKey) *
