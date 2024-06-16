package client

import (
	"context"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// This function takes custom executeMsg and call designated cosmwasm contract execute endpoint
// it returns the instantiated contract address if succeeds
// Input fund example: "1000ubbn". Empty string can be passed if this execution doesn't intend to attach any fund.
func (c *Client) ExecuteContract(contractAddress string, code uint64, executeMsg []byte, fund string) (*wasmtypes.MsgExecuteContractResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	amount, _ := sdk.ParseCoinsNormalized(fund)
	msg := &wasmtypes.MsgExecuteContract{
		Sender:   c.cfg.SubmitterAddress,
		Contract: contractAddress,
		Msg:      executeMsg,
		Funds:    amount,
	}

	return c.msgClient.ExecuteContract(ctx, msg)
}

// This function takes contract address and gets the contract meta data
func (c *Client) QueryContractInfo(contractAddress string) (*wasmtypes.QueryContractInfoResponse, error) {
	var resp *wasmtypes.QueryContractInfoResponse
	err := c.QueryClient.QueryWasm(func(ctx context.Context, queryClient wasmtypes.QueryClient) error {
		var err error
		req := &wasmtypes.QueryContractInfoRequest{
			Address: contractAddress,
		}
		resp, err = queryClient.ContractInfo(ctx, req)
		return err
	})

	return resp, err
}
