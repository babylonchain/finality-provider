package valcfg

import "time"

var (
	defaultBufferSize      = uint32(1000)
	defaultPollingInterval = 5 * time.Second
)

type ChainPollerConfig struct {
	BufferSize   uint32        `long:"buffersize" description:"The maximum number of Babylon blocks that can be stored in the buffer"`
	PollInterval time.Duration `long:"pollinterval" description:"The interval between each polling of Babylon blocks"`
}

func DefaultChainPollerConfig() ChainPollerConfig {
	return ChainPollerConfig{
		BufferSize:   defaultBufferSize,
		PollInterval: defaultPollingInterval,
	}
}
