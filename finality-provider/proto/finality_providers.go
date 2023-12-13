package proto

import (
	"encoding/hex"
	"fmt"

	bbn "github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

func (v *StoreFinalityProvider) GetBabylonPK() *secp256k1.PubKey {
	return &secp256k1.PubKey{
		Key: v.BabylonPk,
	}
}

func (v *StoreFinalityProvider) GetBabylonPkHexString() string {
	return hex.EncodeToString(v.BabylonPk)
}

func (v *StoreFinalityProvider) MustGetBTCPK() *btcec.PublicKey {
	btcPubKey, err := schnorr.ParsePubKey(v.BtcPk)
	if err != nil {
		panic(fmt.Errorf("failed to parse BTC PK: %w", err))
	}
	return btcPubKey
}

func (v *StoreFinalityProvider) MustGetBIP340BTCPK() *bbn.BIP340PubKey {
	btcPK := v.MustGetBTCPK()
	return bbn.NewBIP340PubKeyFromBTCPK(btcPK)
}

func NewFinalityProviderInfo(v *StoreFinalityProvider) *FinalityProviderInfo {
	return &FinalityProviderInfo{
		BabylonPkHex:        v.GetBabylonPkHexString(),
		BtcPkHex:            v.MustGetBIP340BTCPK().MarshalHex(),
		Description:         v.Description,
		LastVotedHeight:     v.LastVotedHeight,
		LastCommittedHeight: v.LastCommittedHeight,
		Status:              v.Status,
	}
}
