package valcfg

import (
	"time"
)

var (
	defaultCovenantKeyName = "covenant-key"
	defaultQueryInterval   = 15 * time.Second
	defaultDelegationLimit = uint64(100)
)

type CovenantConfig struct {
	CovenantKeyName string        `long:"covenantkeyname" description:"The key name of the Covenant if the program is running in Covenant mode"`
	QueryInterval   time.Duration `long:"queryinterval" description:"The interval between each query for pending BTC delegations"`
	DelegationLimit uint64        `long:"delegationlimit" description:"The maximum number of delegations that the Covenant processes each time"`
	SlashingAddress string        `long:"slashingaddress" description:"The slashing address that the slashed fund is sent to"`
}

func DefaultCovenantConfig() CovenantConfig {
	return CovenantConfig{
		CovenantKeyName: defaultCovenantKeyName,
		QueryInterval:   defaultQueryInterval,
		DelegationLimit: defaultDelegationLimit,
	}
}
