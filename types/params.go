package types

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
)

type StakingParams struct {
	// K-deep
	ComfirmationTimeBlocks uint64
	// W-deep
	FinalizationTimeoutBlocks uint64

	// Minimum amount of satoshis required for slashing transaction
	MinSlashingTxFeeSat btcutil.Amount

	// Bitcoin public key of the current covenant
	CovenantPk *btcec.PublicKey

	// Address to which slashing transactions are sent
	SlashingAddress string

	// Minimum commission required by the consumer chain
	MinCommissionRate string
}
