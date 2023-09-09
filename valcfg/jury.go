package valcfg

import (
	"time"
)

var (
	defaultJuryKeyName   = "jury-key"
	defaultQueryInterval = 15 * time.Second
)

type JuryConfig struct {
	JuryKeyName   string        `long:"jurykeyname" description:"The key name of the Jury if the program is running in Jury mode"`
	QueryInterval time.Duration `long:"queryinterval" description:"The interval between each query for pending BTC delegations"`
}

func DefaultJuryConfig() JuryConfig {
	return JuryConfig{
		JuryKeyName:   defaultJuryKeyName,
		QueryInterval: defaultQueryInterval,
	}
}
