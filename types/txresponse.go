package types

import (
	"encoding/json"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

// TxResponse is the interface that handles different types of transaction responses
type TxResponse interface {
	GetTxHash() string
	GetEventsBytes() []byte
}

var _ TxResponse = &BabylonTxResponse{}

type BabylonTxResponse struct {
	TxHash string
	Events []provider.RelayerEvent
}

func (tx *BabylonTxResponse) GetTxHash() string {
	return tx.TxHash
}

func (tx *BabylonTxResponse) GetEventsBytes() []byte {
	bytes, err := json.Marshal(tx.Events)
	if err != nil {
		return nil
	}
	return bytes
}

var _ TxResponse = &CosmwasmTxResponse{}

type CosmwasmTxResponse struct {
	TxHash string `json:"txhash"`
	Events []struct {
		Type       string `json:"type"`
		Attributes []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"attributes"`
	} `json:"events"`
}

func (tx *CosmwasmTxResponse) GetTxHash() string {
	return tx.TxHash
}

func (tx *CosmwasmTxResponse) GetEventsBytes() []byte {
	bytes, err := json.Marshal(tx.Events)
	if err != nil {
		return nil
	}
	return bytes
}
