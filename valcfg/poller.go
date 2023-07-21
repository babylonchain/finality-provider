package valcfg

type ChainPollerConfig struct {
	StartingHeight uint64 `long:"startingheight" description:"The Babylon block height where the poller starts poll"`
	BufferSize     uint32 `long:"buffersize" desciption:"The maximum number of Babylon blocks can be stored in the buffer"`
}

func DefaultChainPollerConfig() ChainPollerConfig {
	return ChainPollerConfig{
		StartingHeight: 1,
		BufferSize:     1000,
	}
}
