package types

import (
	"encoding/base64"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/crypto/eots"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	fptypes "github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/stretchr/testify/require"

	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
)

func genRandomFinalityProvider() fptypes.NewFinalityProvider {
	return fptypes.NewFinalityProvider{
		Description: &fptypes.FinalityProviderDescription{
			Moniker:         "fp1",
			Identity:        "Finality Provider 1",
			Website:         "https://fp1.com",
			SecurityContact: "security_contact",
			Details:         "details",
		},
		Commission: "0.05",
		BabylonPK: &fptypes.PubKey{
			Key: base64.StdEncoding.EncodeToString([]byte("mock_pub_rand")),
		},
		BTCPKHex: "1",
		Pop: &fptypes.ProofOfPossession{
			BTCSigType: 0,
			BabylonSig: base64.StdEncoding.EncodeToString([]byte("mock_babylon_sig")),
			BTCSig:     base64.StdEncoding.EncodeToString([]byte("mock_btc_sig")),
		},
		ConsumerID: "osmosis-1",
	}
}

func genRandomBtcDelegation() (*bstypes.Params, fptypes.ActiveBtcDelegation) {
	var net = &chaincfg.RegressionNetParams
	r := rand.New(rand.NewSource(time.Now().Unix()))
	t := &testing.T{}

	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	// restaked to a random number of finality providers
	numRestakedFPs := int(datagen.RandomInt(r, 10) + 1)
	_, fpPKs, err := datagen.GenRandomBTCKeyPairs(r, numRestakedFPs)
	require.NoError(t, err)
	fpBTCPKs := bbn.NewBIP340PKsFromBTCPKs(fpPKs)

	// (3, 5) covenant committee
	covenantSKs, covenantPKs, err := datagen.GenRandomBTCKeyPairs(r, 5)
	require.NoError(t, err)
	covenantQuorum := uint32(3)

	stakingTimeBlocks := uint16(50000)
	stakingValue := int64(2 * 10e8)
	slashingAddress, err := datagen.GenRandomBTCAddress(r, net)
	require.NoError(t, err)

	slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)
	unbondingTime := uint16(100) + 1
	slashingChangeLockTime := unbondingTime

	bsParams := &bstypes.Params{
		CovenantPks:     bbn.NewBIP340PKsFromBTCPKs(covenantPKs),
		CovenantQuorum:  covenantQuorum,
		SlashingAddress: slashingAddress.EncodeAddress(),
	}

	// only the quorum of signers provided the signatures
	covenantSigners := covenantSKs[:covenantQuorum]

	// construct the BTC delegation with everything
	btcDel, err := datagen.GenRandomBTCDelegation(
		r,
		t,
		net,
		fpBTCPKs,
		delSK,
		covenantSigners,
		covenantPKs,
		covenantQuorum,
		slashingAddress.EncodeAddress(),
		1,
		uint64(1000+stakingTimeBlocks),
		uint64(stakingValue),
		slashingRate,
		slashingChangeLockTime,
	)
	require.NoError(t, err)

	activeDel := convertBTCDelegationToActiveBtcDelegation(btcDel)
	return bsParams, activeDel
}

func convertBTCDelegationToActiveBtcDelegation(mockDel *bstypes.BTCDelegation) fptypes.ActiveBtcDelegation {
	var fpBtcPkList []string
	for _, pk := range mockDel.FpBtcPkList {
		fpBtcPkList = append(fpBtcPkList, pk.MarshalHex())
	}

	var covenantSigs []fptypes.CovenantAdaptorSignatures
	for _, cs := range mockDel.CovenantSigs {
		var adaptorSigs []string
		for _, sig := range cs.AdaptorSigs {
			adaptorSigs = append(adaptorSigs, base64.StdEncoding.EncodeToString(sig))
		}
		covenantSigs = append(covenantSigs, fptypes.CovenantAdaptorSignatures{
			CovPK:       cs.CovPk.MarshalHex(),
			AdaptorSigs: adaptorSigs,
		})
	}

	var covenantUnbondingSigs []fptypes.SignatureInfo
	for _, sigInfo := range mockDel.BtcUndelegation.CovenantUnbondingSigList {
		covenantUnbondingSigs = append(covenantUnbondingSigs, fptypes.SignatureInfo{
			PK:  sigInfo.Pk.MarshalHex(),
			Sig: base64.StdEncoding.EncodeToString(sigInfo.Sig.MustMarshal()),
		})
	}

	var covenantSlashingSigs []fptypes.CovenantAdaptorSignatures
	for _, cs := range mockDel.BtcUndelegation.CovenantSlashingSigs {
		var adaptorSigs []string
		for _, sig := range cs.AdaptorSigs {
			adaptorSigs = append(adaptorSigs, base64.StdEncoding.EncodeToString(sig))
		}
		covenantSlashingSigs = append(covenantSlashingSigs, fptypes.CovenantAdaptorSignatures{
			CovPK:       cs.CovPk.MarshalHex(),
			AdaptorSigs: adaptorSigs,
		})
	}

	undelegationInfo := fptypes.BtcUndelegationInfo{
		UnbondingTx:           base64.StdEncoding.EncodeToString(mockDel.BtcUndelegation.UnbondingTx),
		SlashingTx:            base64.StdEncoding.EncodeToString(mockDel.BtcUndelegation.SlashingTx.MustMarshal()),
		DelegatorSlashingSig:  base64.StdEncoding.EncodeToString(mockDel.BtcUndelegation.DelegatorSlashingSig.MustMarshal()),
		CovenantUnbondingSigs: covenantUnbondingSigs,
		CovenantSlashingSigs:  covenantSlashingSigs,
	}

	return fptypes.ActiveBtcDelegation{
		BTCPkHex:             mockDel.BtcPk.MarshalHex(),
		FpBtcPkList:          fpBtcPkList,
		StartHeight:          mockDel.StartHeight,
		EndHeight:            mockDel.EndHeight,
		TotalSat:             mockDel.TotalSat,
		StakingTx:            base64.StdEncoding.EncodeToString(mockDel.StakingTx),
		SlashingTx:           base64.StdEncoding.EncodeToString(mockDel.SlashingTx.MustMarshal()),
		DelegatorSlashingSig: base64.StdEncoding.EncodeToString(mockDel.DelegatorSig.MustMarshal()),
		CovenantSigs:         covenantSigs,
		StakingOutputIdx:     mockDel.StakingOutputIdx,
		UnbondingTime:        mockDel.UnbondingTime,
		UndelegationInfo:     undelegationInfo,
		ParamsVersion:        mockDel.ParamsVersion,
	}
}

func GenCommitPubRandListMsg(r *rand.Rand, fpSk *btcec.PrivateKey, startHeight uint64, numPubRand uint64) (*datagen.RandListInfo, *ftypes.MsgCommitPubRandList, error) {
	randListInfo, err := genRandomPubRandList(r, numPubRand)
	if err != nil {
		return nil, nil, err
	}
	msg := &ftypes.MsgCommitPubRandList{
		Signer:      datagen.GenRandomAccount().Address,
		FpBtcPk:     bbn.NewBIP340PubKeyFromBTCPK(fpSk.PubKey()),
		StartHeight: startHeight,
		NumPubRand:  numPubRand,
		Commitment:  randListInfo.Commitment,
	}
	hash, err := msg.HashToSign()
	if err != nil {
		return nil, nil, err
	}
	schnorrSig, err := schnorr.Sign(fpSk, hash)
	if err != nil {
		panic(err)
	}
	msg.Sig = bbn.NewBIP340SignatureFromBTCSig(schnorrSig)

	return randListInfo, msg, nil
}

func genRandomPubRandList(r *rand.Rand, numPubRand uint64) (*datagen.RandListInfo, error) {
	// generate a list of secret/public randomness
	var srList []*eots.PrivateRand
	var prList []bbn.SchnorrPubRand
	for i := uint64(0); i < numPubRand; i++ {
		eotsSR, eotsPR, err := eots.RandGen(r)
		if err != nil {
			return nil, err
		}
		pr := bbn.NewSchnorrPubRandFromFieldVal(eotsPR)
		srList = append(srList, eotsSR)
		prList = append(prList, *pr)
	}

	var prByteList [][]byte
	for i := range prList {
		prByteList = append(prByteList, prList[i])
	}

	// generate the commitment to these public randomness
	commitment, proofList := merkle.ProofsFromByteSlices(prByteList)

	return &datagen.RandListInfo{SRList: srList, PRList: prList, Commitment: commitment, ProofList: proofList}, nil
}

func genAddFinalitySig(startHeight uint64, blockHeight uint64, randListInfo *datagen.RandListInfo, sk *btcec.PrivateKey) *ftypes.MsgAddFinalitySig {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	blockHash := datagen.GenRandomByteArray(r, 32)

	signer := datagen.GenRandomAccount().Address
	msg, err := newMsgAddFinalitySig(signer, sk, startHeight, blockHeight, randListInfo, blockHash)
	if err != nil {
		panic(err)
	}

	return msg
}

func newMsgAddFinalitySig(
	signer string,
	sk *btcec.PrivateKey,
	startHeight uint64,
	blockHeight uint64,
	randListInfo *datagen.RandListInfo,
	blockAppHash []byte,
) (*ftypes.MsgAddFinalitySig, error) {
	idx := blockHeight - startHeight

	msg := &ftypes.MsgAddFinalitySig{
		Signer:       signer,
		FpBtcPk:      bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
		PubRand:      &randListInfo.PRList[idx],
		Proof:        randListInfo.ProofList[idx].ToProto(),
		BlockHeight:  blockHeight,
		BlockAppHash: blockAppHash,
		FinalitySig:  nil,
	}
	msgToSign := msg.MsgToSign()
	sig, err := eots.Sign(sk, randListInfo.SRList[idx], msgToSign)
	if err != nil {
		return nil, err
	}
	msg.FinalitySig = bbn.NewSchnorrEOTSSigFromModNScalar(sig)

	return msg, nil
}
