package types

import (
	"encoding/base64"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbn "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/babylon/x/btcstaking/types"
	zctypes "github.com/babylonchain/babylon/x/zoneconcierge/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"

	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
)

func GenExecMessage() ExecuteMessage {
	_, newDel := genBTCDelegation()

	newFp := NewFinalityProvider{
		Description: &FinalityProviderDescription{
			Moniker:         "fp1",
			Identity:        "Finality Provider 1",
			Website:         "https://fp1.com",
			SecurityContact: "security_contact",
			Details:         "details",
		},
		Commission: "0.05",
		BabylonPK: &PubKey{
			Key: base64.StdEncoding.EncodeToString([]byte("mock_pub_rand")),
		},
		BTCPKHex: newDel.FpBtcPkList[0],
		Pop: &ProofOfPossession{
			BTCSigType: 0,
			BabylonSig: base64.StdEncoding.EncodeToString([]byte("mock_pub_rand")),
			BTCSig:     base64.StdEncoding.EncodeToString([]byte("mock_pub_rand")),
		},
		ConsumerID: "osmosis-1",
	}

	// Create the ExecuteMessage instance
	executeMessage := ExecuteMessage{
		BtcStaking: BtcStaking{
			NewFP:       []NewFinalityProvider{newFp},
			ActiveDel:   []ActiveBtcDelegation{newDel},
			SlashedDel:  []SlashedBtcDelegation{},
			UnbondedDel: []UnbondedBtcDelegation{},
		},
	}

	return executeMessage
}

func genBTCDelegation() (*types.Params, ActiveBtcDelegation) {
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

	bsParams := &types.Params{
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

func convertBTCDelegationToActiveBtcDelegation(mockDel *bstypes.BTCDelegation) ActiveBtcDelegation {
	var fpBtcPkList []string
	for _, pk := range mockDel.FpBtcPkList {
		fpBtcPkList = append(fpBtcPkList, pk.MarshalHex())
	}

	var covenantSigs []CovenantAdaptorSignatures
	for _, cs := range mockDel.CovenantSigs {
		var adaptorSigs []string
		for _, sig := range cs.AdaptorSigs {
			adaptorSigs = append(adaptorSigs, base64.StdEncoding.EncodeToString(sig))
		}
		covenantSigs = append(covenantSigs, CovenantAdaptorSignatures{
			CovPK:       cs.CovPk.MarshalHex(),
			AdaptorSigs: adaptorSigs,
		})
	}

	var covenantUnbondingSigs []SignatureInfo
	for _, sigInfo := range mockDel.BtcUndelegation.CovenantUnbondingSigList {
		covenantUnbondingSigs = append(covenantUnbondingSigs, SignatureInfo{
			PK:  sigInfo.Pk.MarshalHex(),
			Sig: base64.StdEncoding.EncodeToString(sigInfo.Sig.MustMarshal()),
		})
	}

	var covenantSlashingSigs []CovenantAdaptorSignatures
	for _, cs := range mockDel.BtcUndelegation.CovenantSlashingSigs {
		var adaptorSigs []string
		for _, sig := range cs.AdaptorSigs {
			adaptorSigs = append(adaptorSigs, base64.StdEncoding.EncodeToString(sig))
		}
		covenantSlashingSigs = append(covenantSlashingSigs, CovenantAdaptorSignatures{
			CovPK:       cs.CovPk.MarshalHex(),
			AdaptorSigs: adaptorSigs,
		})
	}

	undelegationInfo := BtcUndelegationInfo{
		UnbondingTx:           base64.StdEncoding.EncodeToString(mockDel.BtcUndelegation.UnbondingTx),
		SlashingTx:            base64.StdEncoding.EncodeToString(mockDel.BtcUndelegation.SlashingTx.MustMarshal()),
		DelegatorSlashingSig:  base64.StdEncoding.EncodeToString(mockDel.BtcUndelegation.DelegatorSlashingSig.MustMarshal()),
		CovenantUnbondingSigs: covenantUnbondingSigs,
		CovenantSlashingSigs:  covenantSlashingSigs,
	}

	return ActiveBtcDelegation{
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

type NewFinalityProvider struct {
	Description *FinalityProviderDescription `json:"description,omitempty"`
	Commission  string                       `json:"commission"`
	BabylonPK   *PubKey                      `json:"babylon_pk,omitempty"`
	BTCPKHex    string                       `json:"btc_pk_hex"`
	Pop         *ProofOfPossession           `json:"pop,omitempty"`
	ConsumerID  string                       `json:"consumer_id"`
}

type FinalityProviderDescription struct {
	Moniker         string `json:"moniker"`
	Identity        string `json:"identity"`
	Website         string `json:"website"`
	SecurityContact string `json:"security_contact"`
	Details         string `json:"details"`
}

type PubKey struct {
	Key string `json:"key"`
}

type ProofOfPossession struct {
	BTCSigType int32  `json:"btc_sig_type"`
	BabylonSig string `json:"babylon_sig"`
	BTCSig     string `json:"btc_sig"`
}

type CovenantAdaptorSignatures struct {
	CovPK       string   `json:"cov_pk"`
	AdaptorSigs []string `json:"adaptor_sigs"`
}

type SignatureInfo struct {
	PK  string `json:"pk"`
	Sig string `json:"sig"`
}

type BtcUndelegationInfo struct {
	UnbondingTx           string                      `json:"unbonding_tx"`
	DelegatorUnbondingSig string                      `json:"delegator_unbonding_sig"`
	CovenantUnbondingSigs []SignatureInfo             `json:"covenant_unbonding_sig_list"`
	SlashingTx            string                      `json:"slashing_tx"`
	DelegatorSlashingSig  string                      `json:"delegator_slashing_sig"`
	CovenantSlashingSigs  []CovenantAdaptorSignatures `json:"covenant_slashing_sigs"`
}

type ActiveBtcDelegation struct {
	BTCPkHex             string                      `json:"btc_pk_hex"`
	FpBtcPkList          []string                    `json:"fp_btc_pk_list"`
	StartHeight          uint64                      `json:"start_height"`
	EndHeight            uint64                      `json:"end_height"`
	TotalSat             uint64                      `json:"total_sat"`
	StakingTx            string                      `json:"staking_tx"`
	SlashingTx           string                      `json:"slashing_tx"`
	DelegatorSlashingSig string                      `json:"delegator_slashing_sig"`
	CovenantSigs         []CovenantAdaptorSignatures `json:"covenant_sigs"`
	StakingOutputIdx     uint32                      `json:"staking_output_idx"`
	UnbondingTime        uint32                      `json:"unbonding_time"`
	UndelegationInfo     BtcUndelegationInfo         `json:"undelegation_info"`
	ParamsVersion        uint32                      `json:"params_version"`
}

type SlashedBtcDelegation struct {
	// Define fields as needed
}

type UnbondedBtcDelegation struct {
	// Define fields as needed
}

type ExecuteMessage struct {
	BtcStaking BtcStaking `json:"btc_staking"`
}

type BtcStaking struct {
	NewFP       []NewFinalityProvider   `json:"new_fp"`
	ActiveDel   []ActiveBtcDelegation   `json:"active_del"`
	SlashedDel  []SlashedBtcDelegation  `json:"slashed_del"`
	UnbondedDel []UnbondedBtcDelegation `json:"unbonded_del"`
}

func GenIBCPacket(t *testing.T, r *rand.Rand) *zctypes.ZoneconciergePacketData {
	// generate a finality provider
	fpBTCSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fpBabylonSK, _, err := datagen.GenRandomSecp256k1KeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenRandomCustomFinalityProvider(r, fpBTCSK, fpBabylonSK, "consumer-id")
	require.NoError(t, err)

	packet := &bstypes.BTCStakingIBCPacket{
		NewFp: []*bstypes.NewFinalityProvider{
			// TODO: fill empty data
			{
				// Description: fp.Description,
				Commission: fp.Commission.String(),
				// BabylonPk:  fp.BabylonPk,
				BtcPkHex: fp.BtcPk.MarshalHex(),
				// Pop:        fp.Pop,
				ConsumerId: fp.ConsumerId,
			},
		},
		ActiveDel:   []*bstypes.ActiveBTCDelegation{},
		SlashedDel:  []*bstypes.SlashedBTCDelegation{},
		UnbondedDel: []*bstypes.UnbondedBTCDelegation{},
	}
	return NewBTCStakingPacketData(packet)
}

func NewBTCStakingPacketData(packet *bstypes.BTCStakingIBCPacket) *zctypes.ZoneconciergePacketData {
	return &zctypes.ZoneconciergePacketData{
		Packet: &zctypes.ZoneconciergePacketData_BtcStaking{
			BtcStaking: packet,
		},
	}
}

type ConsumerFpsResponse struct {
	ConsumerFps []SingleConsumerFpResponse `json:"fps"`
}

// SingleConsumerFpResponse represents the finality provider data returned by the contract query.
// For more details, refer to the following links:
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/packages/apis/src/btc_staking_api.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/src/msg.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/schema/btc-staking.json
type SingleConsumerFpResponse struct {
	BtcPkHex             string `json:"btc_pk_hex"`
	SlashedBabylonHeight uint64 `json:"slashed_babylon_height"`
	SlashedBtcHeight     uint64 `json:"slashed_btc_height"`
	ConsumerId           string `json:"consumer_id"`
}

type SubmitFinalitySignature struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	Height      uint64 `json:"height"`
	PubRand     string `json:"pub_rand"`   // base64 encoded
	Proof       Proof  `json:"proof"`      // nested struct
	BlockHash   string `json:"block_hash"` // base64 encoded
	Signature   string `json:"signature"`  // base64 encoded
}

type ExecuteMsg struct {
	SubmitFinalitySignature *SubmitFinalitySignature `json:"submit_finality_signature"`
}

type Proof struct {
	Total    uint64   `json:"total"`
	Index    uint64   `json:"index"`
	LeafHash string   `json:"leaf_hash"`
	Aunts    []string `json:"aunts"`
}

// Generate a finality signature message with mock data
func GenFinalitySignatureMessage(fpBtcPkHex string, height uint64) *ExecuteMsg {
	pubRand := base64.StdEncoding.EncodeToString([]byte("mock_pub_rand"))
	leafHash := base64.StdEncoding.EncodeToString([]byte("mock_leaf_hash"))
	blockHash := base64.StdEncoding.EncodeToString([]byte("mock_block_hash"))
	signature := base64.StdEncoding.EncodeToString([]byte("mock_signature"))

	msg := ExecuteMsg{
		SubmitFinalitySignature: &SubmitFinalitySignature{
			FpPubkeyHex: fpBtcPkHex,
			Height:      height,
			PubRand:     pubRand,
			Proof: Proof{
				Total:    0,
				Index:    0,
				LeafHash: leafHash,
				Aunts:    []string{},
			},
			BlockHash: blockHash,
			Signature: signature,
		},
	}

	return &msg
}

type ConsumerDelegationsResponse struct {
	ConsumerDelegations []SingleConsumerDelegationResponse `json:"delegations"`
}

type SingleConsumerDelegationResponse struct {
	BtcPkHex             string                      `json:"btc_pk_hex"`
	FpBtcPkList          []string                    `json:"fp_btc_pk_list"`
	StartHeight          uint64                      `json:"start_height"`
	EndHeight            uint64                      `json:"end_height"`
	TotalSat             uint64                      `json:"total_sat"`
	StakingTx            []byte                      `json:"staking_tx"`
	SlashingTx           []byte                      `json:"slashing_tx"`
	DelegatorSlashingSig []byte                      `json:"delegator_slashing_sig"`
	CovenantSigs         []CovenantAdaptorSignatures `json:"covenant_sigs"`
	StakingOutputIdx     uint32                      `json:"staking_output_idx"`
	UnbondingTime        uint32                      `json:"unbonding_time"`
	UndelegationInfo     *BtcUndelegationInfo        `json:"undelegation_info"`
	ParamsVersion        uint32                      `json:"params_version"`
}

type ConsumerFpInfo struct {
	BtcPkHex string `json:"btc_pk_hex"`
	Power    uint64 `json:"power"`
}

type ConsumerFpsByPowerResponse struct {
	Fps []ConsumerFpInfo `json:"fps"`
}
