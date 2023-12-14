package proto

import (
	"encoding/hex"
	"fmt"

	bbn "github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

func (sfp *StoreFinalityProvider) GetBabylonPK() *secp256k1.PubKey {
	return &secp256k1.PubKey{
		Key: sfp.BabylonPk,
	}
}

func (sfp *StoreFinalityProvider) GetBabylonPkHexString() string {
	return hex.EncodeToString(sfp.BabylonPk)
}

func (sfp *StoreFinalityProvider) MustGetBTCPK() *btcec.PublicKey {
	btcPubKey, err := schnorr.ParsePubKey(sfp.BtcPk)
	if err != nil {
		panic(fmt.Errorf("failed to parse BTC PK: %w", err))
	}
	return btcPubKey
}

func (sfp *StoreFinalityProvider) MustGetBIP340BTCPK() *bbn.BIP340PubKey {
	btcPK := sfp.MustGetBTCPK()
	return bbn.NewBIP340PubKeyFromBTCPK(btcPK)
}

func NewFinalityProviderInfo(sfp *StoreFinalityProvider) *FinalityProviderInfo {
	return &FinalityProviderInfo{
		BabylonPkHex:        sfp.GetBabylonPkHexString(),
		BtcPkHex:            sfp.MustGetBIP340BTCPK().MarshalHex(),
		Description:         sfp.Description,
		LastVotedHeight:     sfp.LastVotedHeight,
		LastCommittedHeight: sfp.LastCommittedHeight,
		Status:              sfp.Status,
	}
}
