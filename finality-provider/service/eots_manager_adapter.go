package service

import (
	"fmt"

	bbntypes "github.com/babylonchain/babylon/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func (fp *FinalityProviderInstance) createPubRandList(startHeight uint64, numPubRand uint64) ([]*btcec.FieldVal, error) {
	pubRandList, err := fp.em.CreateRandomnessPairList(
		fp.btcPk.MustMarshal(),
		fp.GetChainID(),
		startHeight,
		uint32(numPubRand),
		fp.passphrase,
	)
	if err != nil {
		return nil, err
	}

	return pubRandList, nil
}

func (fp *FinalityProviderInstance) signPubRandCommit(startHeight uint64, numPubRand uint64, commitment []byte) (*schnorr.Signature, error) {
	msg := &ftypes.MsgCommitPubRandList{
		StartHeight: startHeight,
		NumPubRand:  numPubRand,
		Commitment:  commitment,
	}
	hash, err := msg.HashToSign()
	if err != nil {
		return nil, fmt.Errorf("failed to sign the commit public randomness message: %w", err)
	}

	// sign the message hash using the finality-provider's BTC private key
	return fp.em.SignSchnorrSig(fp.btcPk.MustMarshal(), hash, fp.passphrase)
}

func (fp *FinalityProviderInstance) signFinalitySig(b *types.BlockInfo) (*bbntypes.SchnorrEOTSSig, error) {
	// build proper finality signature request
	msg := &ftypes.MsgAddFinalitySig{
		FpBtcPk:      fp.btcPk,
		BlockHeight:  b.Height,
		BlockAppHash: b.Hash,
	}
	msgToSign := msg.MsgToSign()
	sig, err := fp.em.SignEOTS(fp.btcPk.MustMarshal(), fp.GetChainID(), msgToSign, b.Height, fp.passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to sign EOTS: %w", err)
	}

	return bbntypes.NewSchnorrEOTSSigFromModNScalar(sig), nil
}

/*
	below is only used for testing purposes
*/

func (fp *FinalityProviderInstance) getEOTSPrivKey() (*btcec.PrivateKey, error) {
	record, err := fp.em.KeyRecord(fp.btcPk.MustMarshal(), fp.passphrase)
	if err != nil {
		return nil, err
	}

	return record.PrivKey, nil
}

func (fp *FinalityProviderInstance) BtcPrivKey() (*btcec.PrivateKey, error) {
	return fp.getEOTSPrivKey()
}
