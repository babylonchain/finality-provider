package types

import (
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/wire"
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
	// total_sat is the total amount of BTC stakes in this delegation
	// quantified in satoshi
	TotalSat uint64
	// staking_tx is the staking tx
	StakingTx *btcstakingtypes.BabylonBTCTaprootTx
	// slashing_tx is the slashing tx
	// It is partially signed by SK corresponding to btc_pk, but not signed by
	// validator or jury yet.
	SlashingTx *btcstakingtypes.BTCSlashingTx
	// jury_sig is the signature on the slashing tx
	// by the jury (i.e., SK corresponding to jury_pk in params)
	// It will be a part of the witness for the staking tx output.
	JurySig *schnorr.Signature
	// if this object is present it menans that staker requested undelegation, and whole
	// delegation is being undelegated.
	// directly in delegation object
	BtcUndelegation *btcstakingtypes.BTCUndelegation
}

// BTCUndelegation signalizes that the delegation is being undelegated
type BTCUndelegation struct {
	// unbonding_tx is the transaction which will transfer the funds from staking
	// output to unbonding output. Unbonding output will usually have lower timelock
	// than staking output.
	UnbondingTx *wire.MsgTx
	// slashing_tx is the slashing tx for unbodning transactions
	// It is partially signed by SK corresponding to btc_pk, but not signed by
	// validator or jury yet.
	SlashingTx *wire.MsgTx
	// jury_slashing_sig is the signature on the slashing tx
	// by the jury (i.e., SK corresponding to jury_pk in params)
	// It must be provided after processing undelagate message by the consumer chain
	JurySlashingSig *schnorr.Signature
	// jury_unbonding_sig is the signature on the unbonding tx
	// by the jury (i.e., SK corresponding to jury_pk in params)
	// It must be provided after processing undelagate message by the consumer chain and after
	// validator sig will be provided by validator
	JuryUnbondingSig *schnorr.Signature
	// validator_unbonding_sig is the signature on the unbonding tx
	// by the validator (i.e., SK corresponding to jury_pk in params)
	// It must be provided after processing undelagate message by the consumer chain
	ValidatorUnbondingSig *schnorr.Signature
}
