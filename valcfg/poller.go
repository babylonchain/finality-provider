package valcfg

import "time"

var (
	defaultStartingHeight  = uint64(1)
	defaultBufferSize      = uint32(1000)
	defaultPollingInterval = 5 * time.Second
)

type ChainPollerConfig struct {
	StartingHeight uint64        `long:"startingheight" description:"The Babylon block height where the poller starts poll"`
	BufferSize     uint32        `long:"buffersize" desciption:"The maximum number of Babylon blocks can be stored in the buffer"`
	PollInterval   time.Duration `long:"pollinterval" description:"The interval between each polling Babylon blocks"`
}

func DefaultChainPollerConfig() ChainPollerConfig {
	return ChainPollerConfig{
		StartingHeight: defaultStartingHeight,
		BufferSize:     defaultBufferSize,
		PollInterval:   defaultPollingInterval,
	}
}
