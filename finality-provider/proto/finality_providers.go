package proto

import (
	"encoding/hex"
	"fmt"

	bbn "github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (sfp *FinalityProvider) GetChainPK() *secp256k1.PubKey {
	return &secp256k1.PubKey{
		Key: sfp.ChainPk,
	}
}

func (sfp *FinalityProvider) GetChainPkHexString() string {
	return hex.EncodeToString(sfp.ChainPk)
}

func (sfp *FinalityProvider) MustGetBTCPK() *btcec.PublicKey {
	btcPubKey, err := schnorr.ParsePubKey(sfp.BtcPk)
	if err != nil {
		panic(fmt.Errorf("failed to parse BTC PK: %w", err))
	}
	return btcPubKey
}

func (sfp *FinalityProvider) MustGetBIP340BTCPK() *bbn.BIP340PubKey {
	btcPK := sfp.MustGetBTCPK()
	return bbn.NewBIP340PubKeyFromBTCPK(btcPK)
}

func NewFinalityProviderInfo(sfp *FinalityProvider) (*FinalityProviderInfo, error) {
	var des types.Description
	if err := des.Unmarshal(sfp.Description); err != nil {
		return nil, err
	}
	return &FinalityProviderInfo{
		ChainPkHex: sfp.GetChainPkHexString(),
		BtcPkHex:   sfp.MustGetBIP340BTCPK().MarshalHex(),
		Description: &Description{
			Moniker:         des.Moniker,
			Identity:        des.Identity,
			Website:         des.Website,
			SecurityContact: des.SecurityContact,
			Details:         des.Details,
		},
		LastVotedHeight: sfp.LastVotedHeight,
		Status:          sfp.Status.String(),
	}, nil
}
