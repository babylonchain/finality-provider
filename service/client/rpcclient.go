package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/babylonchain/btc-validator/valrpc"
)

type ValidatorServiceGRpcClient struct {
	client valrpc.BtcValidatorsClient
}

func NewValidatorServiceGRpcClient(remoteAddr string) (*ValidatorServiceGRpcClient, func(), error) {
	conn, err := grpc.Dial(remoteAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build gRPC connection to %s: %w", remoteAddr, err)
	}

	cleanUp := func() {
		conn.Close()
	}

	return &ValidatorServiceGRpcClient{
		client: valrpc.NewBtcValidatorsClient(conn),
	}, cleanUp, nil
}

func (c *ValidatorServiceGRpcClient) GetInfo(ctx context.Context) (*valrpc.GetInfoResponse, error) {
	req := &valrpc.GetInfoRequest{}
	res, err := c.client.GetInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ValidatorServiceGRpcClient) RegisterValidator(ctx context.Context, bbnPkBytes []byte) (*valrpc.RegisterValidatorResponse, error) {
	req := &valrpc.RegisterValidatorRequest{BabylonPk: bbnPkBytes}
	res, err := c.client.RegisterValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
