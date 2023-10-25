package types

import (
	bbntypes "github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

type DelegationStatus int32

const (
	// PENDING defines a delegation that is waiting for a jury signature to become active.
	DelegationStatus_PENDING DelegationStatus = 0
	// ACTIVE defines a delegation that has voting power
	DelegationStatus_ACTIVE DelegationStatus = 1
	// UNBONDING defines a delegation that is being unbonded i.e it received an unbonding tx
	// from staker, but not yet received signatures from validator and jury.
	// Delegation in this state already lost its voting power.
	DelegationStatus_UNBONDING DelegationStatus = 2
	// UNBONDED defines a delegation no longer has voting power:
	// - either reaching the end of staking transaction timelock
	// - or receiving unbonding tx and then receiving signatures from validator and jury for this
	// unbonding tx.
	DelegationStatus_UNBONDED DelegationStatus = 3
	// ANY is any of the above status
	DelegationStatus_ANY DelegationStatus = 4
)

type Delegation struct {
	// babylon_pk is the Babylon secp256k1 PK of this BTC delegation
	BabylonPk *secp256k1.PubKey
	// btc_pk is the Bitcoin secp256k1 PK of this BTC delegation
	// the PK follows encoding in BIP-340 spec
	BtcPk *bbntypes.BIP340PubKey
	// pop is the proof of possession of babylon_pk and btc_pk
	Pop *btcstakingtypes.ProofOfPossession
	// val_btc_pk is the Bitcoin secp256k1 PK of the BTC validator that
	// this BTC delegation delegates to
	// the PK follows encoding in BIP-340 spec
	ValBtcPk *bbntypes.BIP340PubKey
	// start_height is the start BTC height of the BTC delegation
	// it is the start BTC height of the timelock
	StartHeight uint64
	// end_height is the end height of the BTC delegation
	// it is the end BTC height of the timelock - w
	EndHeight uint64
	// total_sat is the total amount of BTC stakes in this delegation
	// quantified in satoshi
	TotalSat uint64
	// staking_tx is the staking tx
	StakingTx *btcstakingtypes.BabylonBTCTaprootTx
	// slashing_tx is the slashing tx
	// It is partially signed by SK corresponding to btc_pk, but not signed by
	// validator or jury yet.
	SlashingTx *btcstakingtypes.BTCSlashingTx
	// delegator_sig is the signature on the slashing tx
	// by the delegator (i.e., SK corresponding to btc_pk).
	// It will be a part of the witness for the staking tx output.
	DelegatorSig *bbntypes.BIP340Signature
	// jury_sig is the signature signature on the slashing tx
	// by the jury (i.e., SK corresponding to jury_pk in params)
	// It will be a part of the witness for the staking tx output.
	JurySig *bbntypes.BIP340Signature
	// if this object is present it menans that staker requested undelegation, and whole
	// delegation is being undelegated.
	// directly in delegation object
	BtcUndelegation *btcstakingtypes.BTCUndelegation
}

func (d *Delegation) GetStatus(btcHeight uint64, w uint64) DelegationStatus {
	if d.BtcUndelegation != nil {
		if d.BtcUndelegation.HasAllSignatures() {
			return DelegationStatus_UNBONDED
		}
		// If we received an undelegation but is still does not have all required signature,
		// delegation receives UNBONING status.
		// Voting power from this delegation is removed from the total voting power and now we
		// are waiting for signatures from validator and jury for delegation to become expired.
		// For now we do not have any unbonding time on Babylon chain, only time lock on BTC chain
		// we may consider adding unbonding time on Babylon chain later to avoid situation where
		// we can lose to much voting power in to short time.
		return DelegationStatus_UNBONDING
	}

	if d.StartHeight <= btcHeight && btcHeight+w <= d.EndHeight {
		if d.JurySig != nil {
			return DelegationStatus_ACTIVE
		} else {
			return DelegationStatus_PENDING
		}
	}
	return DelegationStatus_UNBONDED
}
