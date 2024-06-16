package client

import (
	"context"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// This function takes custom executeMsg and call designated cosmwasm contract execute endpoint
// it returns the instantiated contract address if succeeds
// Input fund example: "1000ubbn". Empty string can be passed if this execution doesn't intend to attach any fund.
func (c *Client) ExecuteContract(contractAddr string, code uint64, executeMsg string, fund string) (*wasmtypes.MsgExecuteContractResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	amount, _ := sdk.ParseCoinsNormalized(fund)
	msg := &wasmtypes.MsgExecuteContract{
		Sender:   c.cfg.SubmitterAddress,
		Contract: contractAddr,
		Msg:      wasmtypes.RawContractMessage(executeMsg),
		Funds:    amount,
	}

	return c.msgClient.ExecuteContract(ctx, msg)
}
