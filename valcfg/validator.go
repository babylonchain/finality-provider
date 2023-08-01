package valcfg

type ValidatorConfig struct {
	StaticChainScanningStartHeight uint64 `long:"staticchainscanningstartheight" description:"The static height from which we start polling the chain"`
	AutoChainScanningMode          bool   `long:"autochainscanningmode" description:"Automatically discover the height from which to start polling the chain"`
}

func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		StaticChainScanningStartHeight: 1,
		AutoChainScanningMode:          true,
	}
}
