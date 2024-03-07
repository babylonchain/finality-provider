package store

import (
	"encoding/hex"
	"fmt"

	sdkmath "cosmossdk.io/math"
	bbn "github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonchain/finality-provider/finality-provider/proto"
)

type StoredFinalityProvider struct {
	ChainPk             *secp256k1.PubKey
	BtcPk               *btcec.PublicKey
	Description         *stakingtypes.Description
	Commission          *sdkmath.LegacyDec
	Pop                 *proto.ProofOfPossession
	KeyName             string
	ChainID             string
	LastVotedHeight     uint64
	LastProcessedHeight uint64
	Status              proto.FinalityProviderStatus
}

func protoFpToStoredFinalityProvider(fp *proto.FinalityProvider) (*StoredFinalityProvider, error) {
	chainPk := &secp256k1.PubKey{Key: fp.ChainPk}
	btcPk, err := schnorr.ParsePubKey(fp.BtcPk)
	if err != nil {
		return nil, fmt.Errorf("invalid BTC public key: %w", err)
	}

	var des stakingtypes.Description
	if err := des.Unmarshal(fp.Description); err != nil {
		return nil, fmt.Errorf("invalid description: %w", err)
	}

	commission, err := sdkmath.LegacyNewDecFromStr(fp.Commission)
	if err != nil {
		return nil, fmt.Errorf("invalid commission: %w", err)
	}

	return &StoredFinalityProvider{
		ChainPk:     chainPk,
		BtcPk:       btcPk,
		Description: &des,
		Commission:  &commission,
		Pop: &proto.ProofOfPossession{
			ChainSig: fp.Pop.ChainSig,
			BtcSig:   fp.Pop.BtcSig,
		},
		KeyName:             fp.KeyName,
		ChainID:             fp.ChainId,
		LastVotedHeight:     fp.LastVotedHeight,
		LastProcessedHeight: fp.LastProcessedHeight,
		Status:              fp.Status,
	}, nil
}

func (sfp *StoredFinalityProvider) GetChainPkHexString() string {
	return hex.EncodeToString(sfp.ChainPk.Key)
}

func (sfp *StoredFinalityProvider) GetBIP340BTCPK() *bbn.BIP340PubKey {
	return bbn.NewBIP340PubKeyFromBTCPK(sfp.BtcPk)
}

func (sfp *StoredFinalityProvider) ToFinalityProviderInfo() *proto.FinalityProviderInfo {
	return &proto.FinalityProviderInfo{
		ChainPkHex: sfp.GetChainPkHexString(),
		BtcPkHex:   sfp.GetBIP340BTCPK().MarshalHex(),
		Description: &proto.Description{
			Moniker:         sfp.Description.Moniker,
			Identity:        sfp.Description.Identity,
			Website:         sfp.Description.Website,
			SecurityContact: sfp.Description.SecurityContact,
			Details:         sfp.Description.Details,
		},
		Commission:      sfp.Commission.String(),
		LastVotedHeight: sfp.LastVotedHeight,
		Status:          sfp.Status.String(),
	}
}
