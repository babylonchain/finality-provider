package types

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

type Delegation struct {
	// btc_pk is the Bitcoin secp256k1 PK of this BTC delegation
	BtcPk *btcec.PublicKey
	// val_btc_pk is the Bitcoin secp256k1 PK of the BTC validator that
	// this BTC delegation delegates to
	ValBtcPk *btcec.PublicKey
	// start_height is the start BTC height of the BTC delegation
	// it is the start BTC height of the timelock
	StartHeight uint64
	// end_height is the end height of the BTC delegation
	// it is the end BTC height of the timelock - w
	EndHeight uint64
	// staking_tx_hex is the hex string of the staking tx
	StakingTxHex string
	// slashing_tx_hex is the hex string of the slashing tx
	// It is partially signed by SK corresponding to btc_pk, but not signed by
	// validator or covenant yet.
	SlashingTxHex string
	// covenant_sig is the signature on the slashing tx
	// by the covenant (i.e., SK corresponding to covenant_pk in params)
	// It will be a part of the witness for the staking tx output.
	CovenantSig *schnorr.Signature
	// if this object is present it menans that staker requested undelegation, and whole
	// delegation is being undelegated.
	// directly in delegation object
	BtcUndelegation *Undelegation
}

// Undelegation signalizes that the delegation is being undelegated
type Undelegation struct {
	// unbonding_tx_hex is the hex string of the transaction which will transfer the funds from staking
	// output to unbonding output. Unbonding output will usually have lower timelock
	// than staking output.
	UnbondingTxHex string
	// slashing_tx is the hex string of the slashing tx for unbodning transactions
	// It is partially signed by SK corresponding to btc_pk, but not signed by
	// validator or covenant yet.
	SlashingTxHex string
	// covenant_slashing_sig is the signature on the slashing tx
	// by the covenant (i.e., SK corresponding to covenant_pk in params)
	// It must be provided after processing undelagate message by the consumer chain
	CovenantSlashingSig *schnorr.Signature
	// covenant_unbonding_sig is the signature on the unbonding tx
	// by the covenant (i.e., SK corresponding to covenant_pk in params)
	// It must be provided after processing undelagate message by the consumer chain and after
	// validator sig will be provided by validator
	CovenantUnbondingSig *schnorr.Signature
	// validator_unbonding_sig is the signature on the unbonding tx
	// by the validator (i.e., SK corresponding to covenant_pk in params)
	// It must be provided after processing undelagate message by the consumer chain
	ValidatorUnbondingSig *schnorr.Signature
}
