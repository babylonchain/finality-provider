package types

import (
	"math"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

type Delegation struct {
	// The Bitcoin secp256k1 PK of this BTC delegation
	BtcPk *btcec.PublicKey
	// The Bitcoin secp256k1 PK of the BTC validator that
	// this BTC delegation delegates to
	ValBtcPks []*btcec.PublicKey
	// The start BTC height of the BTC delegation
	// it is the start BTC height of the timelock
	StartHeight uint64
	// The end height of the BTC delegation
	// it is the end BTC height of the timelock - w
	EndHeight uint64
	// The total amount of BTC stakes in this delegation
	// quantified in satoshi
	TotalSat uint64
	// The hex string of the staking tx
	StakingTxHex string
	// The index of the staking output in the staking tx
	StakingOutputIdx uint32
	// The hex string of the slashing tx
	SlashingTxHex string
	// The signature on the slashing tx
	// by the covenant (i.e., SK corresponding to covenant_pk in params)
	// It will be a part of the witness for the staking tx output.
	CovenantSigs []*CovenantSignatureInfo
	// if this object is present it menans that staker requested undelegation, and whole
	// delegation is being undelegated directly in delegation object
	BtcUndelegation *Undelegation
}

// HasCovenantQuorum returns whether a delegation has sufficient sigs
// from Covenant members to make a quorum
func (d *Delegation) HasCovenantQuorum(quorum uint32) bool {
	return uint32(len(d.CovenantSigs)) >= quorum
}

func (d *Delegation) GetStakingTime() uint16 {
	diff := d.EndHeight - d.StartHeight

	if diff > math.MaxUint16 {
		// In valid delegation, EndHeight is always greater than StartHeight and it is always uint16 value
		panic("invalid delegation in database")
	}

	return uint16(diff)
}

// Undelegation signalizes that the delegation is being undelegated
type Undelegation struct {
	// How long the funds will be locked in the unbonding output
	UnbondingTime uint32
	// The hex string of the transaction which will transfer the funds from staking
	// output to unbonding output. Unbonding output will usually have lower timelock
	// than staking output.
	UnbondingTxHex string
	// The hex string of the slashing tx for unbodning transactions
	// It is partially signed by SK corresponding to btc_pk, but not signed by
	// validator or covenant yet.
	SlashingTxHex string
	// The signatures on the slashing tx by the covenant
	// (i.e., SK corresponding to covenant_pk in params)
	// It must be provided after processing undelagate message by the consumer chain
	CovenantSlashingSigs []*CovenantSignatureInfo
	// The signatures on the unbonding tx by the covenant
	// (i.e., SK corresponding to covenant_pk in params)
	// It must be provided after processing undelagate message by the consumer chain
	CovenantUnbondingSigs []*SignatureInfo
}

func (ud *Undelegation) HasCovenantQuorumOnSlashing(quorum uint32) bool {
	return len(ud.CovenantUnbondingSigs) >= int(quorum)
}

func (ud *Undelegation) HasCovenantQuorumOnUnbonding(quorum uint32) bool {
	return len(ud.CovenantUnbondingSigs) >= int(quorum)
}

func (ud *Undelegation) HasAllSignatures(covenantQuorum uint32) bool {
	return ud.HasCovenantQuorumOnUnbonding(covenantQuorum) && ud.HasCovenantQuorumOnSlashing(covenantQuorum)
}

type CovenantSignatureInfo struct {
	Pk   *btcec.PublicKey
	Sigs [][]byte
}

type SignatureInfo struct {
	Pk  *btcec.PublicKey
	Sig *schnorr.Signature
}
