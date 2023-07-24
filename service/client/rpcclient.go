package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/babylonchain/btc-validator/proto"
)

type ValidatorServiceGRpcClient struct {
	client proto.BtcValidatorsClient
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
		client: proto.NewBtcValidatorsClient(conn),
	}, cleanUp, nil
}

func (c *ValidatorServiceGRpcClient) GetInfo(ctx context.Context) (*proto.GetInfoResponse, error) {
	req := &proto.GetInfoRequest{}
	res, err := c.client.GetInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ValidatorServiceGRpcClient) RegisterValidator(ctx context.Context, keyName string) (*proto.RegisterValidatorResponse, error) {
	req := &proto.RegisterValidatorRequest{KeyName: keyName}
	res, err := c.client.RegisterValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ValidatorServiceGRpcClient) CreateValidator(ctx context.Context, keyName string) (*proto.CreateValidatorResponse, error) {
	req := &proto.CreateValidatorRequest{KeyName: keyName}
	res, err := c.client.CreateValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ValidatorServiceGRpcClient) QueryValidatorList(ctx context.Context) (*proto.QueryValidatorListResponse, error) {
	req := &proto.QueryValidatorListRequest{}
	res, err := c.client.QueryValidatorList(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
