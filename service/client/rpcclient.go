package client

import (
	"context"
	"fmt"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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

func (c *ValidatorServiceGRpcClient) CreateValidator(ctx context.Context, keyName string, description *stakingtypes.Description) (*proto.CreateValidatorResponse, error) {
	req := &proto.CreateValidatorRequest{KeyName: keyName, Description: description}
	res, err := c.client.CreateValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ValidatorServiceGRpcClient) AddFinalitySignature(ctx context.Context, bbnPk []byte, height uint64, lch []byte) (*proto.AddFinalitySignatureResponse, error) {
	req := &proto.AddFinalitySignatureRequest{
		BabylonPk:      bbnPk,
		Height:         height,
		LastCommitHash: lch,
	}

	res, err := c.client.AddFinalitySignature(ctx, req)
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

func (c *ValidatorServiceGRpcClient) QueryValidatorInfo(ctx context.Context, bbnPk []byte) (*proto.QueryValidatorResponse, error) {
	req := &proto.QueryValidatorRequest{BabylonPk: bbnPk}
	res, err := c.client.QueryValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
