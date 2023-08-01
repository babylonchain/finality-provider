package valcfg

var (
	defaultAutoChainScanningStart = true
)

type ValidatorConfig struct {
	StaticChainScanningStart *uint64 `long:"staticchainscanningstart" desciption:"The static height from which we start polling the chain"`
	AutoChainScnaningStart   bool    `long:"autochainscanningstart" description:"Automatically discover the height from which to start polling the chain"`
}

func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		StaticChainScanningStart: nil,
		AutoChainScnaningStart:   defaultAutoChainScanningStart,
	}
}
