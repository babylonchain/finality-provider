package types

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
