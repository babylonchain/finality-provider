package types

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

// TxResponse handles the transaction response in the interface ConsumerController
// Not every consumer has Events thing in their response,
// so consumer client implementations need to care about Events field.
type TxResponse struct {
	TxHash string
	// JSON-encoded data, now it is for testing purposes only
	Events []byte
}

func GetEventsBytes(events []provider.RelayerEvent) []byte {
	bytes, err := json.Marshal(events)
	if err != nil {
		return nil
	}
	return bytes
}

func (res *TxResponse) ParseEvents() ([]provider.RelayerEvent, error) {
	var events []provider.RelayerEvent
	if err := json.Unmarshal(res.Events, &events); err != nil {
		return nil, fmt.Errorf("failed to decode bytes to RelayerEvent: %s", err.Error())
	}
	return events, nil
}
