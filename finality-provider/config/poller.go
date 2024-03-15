package config

import "time"

var (
	defaultBufferSize        = uint32(1000)
	defaultPollingInterval   = 20 * time.Second
	defaultStaticStartHeight = uint64(1)
)

type ChainPollerConfig struct {
	BufferSize                     uint32        `long:"buffersize" description:"The maximum number of Babylon blocks that can be stored in the buffer"`
	PollInterval                   time.Duration `long:"pollinterval" description:"The interval between each polling of Babylon blocks"`
	StaticChainScanningStartHeight uint64        `long:"staticchainscanningstartheight" description:"The static height from which we start polling the chain"`
	AutoChainScanningMode          bool          `long:"autochainscanningmode" description:"Automatically discover the height from which to start polling the chain"`
}

func DefaultChainPollerConfig() ChainPollerConfig {
	return ChainPollerConfig{
		BufferSize:                     defaultBufferSize,
		PollInterval:                   defaultPollingInterval,
		StaticChainScanningStartHeight: defaultStaticStartHeight,
		AutoChainScanningMode:          true,
	}
}
