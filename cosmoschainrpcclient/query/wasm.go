package query

import (
	"context"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
)

func (c *QueryClient) QueryWasm(f func(ctx context.Context, queryClient wasmtypes.QueryClient) error) error {
	ctx, cancel := c.getQueryContext()
	defer cancel()

	clientCtx := client.Context{Client: c.RPCClient}
	queryClient := wasmtypes.NewQueryClient(clientCtx)

	return f(ctx, queryClient)
}

func (c *QueryClient) ListCodes(pagination *sdkquerytypes.PageRequest) (*wasmtypes.QueryCodesResponse, error) {
	var resp *wasmtypes.QueryCodesResponse
	err := c.QueryWasm(func(ctx context.Context, queryClient wasmtypes.QueryClient) error {
		var err error
		req := &wasmtypes.QueryCodesRequest{
			Pagination: pagination,
		}
		resp, err = queryClient.Codes(ctx, req)
		return err
	})

	return resp, err
}
