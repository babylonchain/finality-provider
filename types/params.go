package types

import (
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
)

type StakingParams struct {
	// K-deep
	ComfirmationTimeBlocks uint64
	// W-deep
	FinalizationTimeoutBlocks uint64

	// Minimum amount of tx fee (quantified in Satoshi) needed for the pre-signed slashing tx
	MinSlashingTxFeeSat int64

	// Bitcoin public keys of the covenant committee
	CovenantPks []*btcec.PublicKey

	// Address to which slashing transactions are sent
	SlashingAddress string

	// Minimum number of signatures needed for the covenant multisignature
	CovenantQuorum uint32

	// Chain-wide minimum commission rate that a validator can charge their delegators
	MinCommissionRate *big.Int

	// The staked amount to be slashed, expressed as a decimal (e.g., 0.5 for 50%).
	SlashingRate *big.Int

	// Maximum number of active BTC validators in the BTC staking protocol
	MaxActiveBtcValidators uint32
}
