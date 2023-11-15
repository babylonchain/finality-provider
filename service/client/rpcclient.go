package client

import (
	"context"
	"fmt"

	bbntypes "github.com/babylonchain/babylon/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
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

func (c *ValidatorServiceGRpcClient) RegisterValidator(ctx context.Context, valPk *bbntypes.BIP340PubKey) (*proto.RegisterValidatorResponse, error) {
	req := &proto.RegisterValidatorRequest{BtcPk: valPk.MarshalHex()}
	res, err := c.client.RegisterValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ValidatorServiceGRpcClient) CreateValidator(
	ctx context.Context,
	keyName, chainID, passPhrase, hdPath string,
	description *stakingtypes.Description,
	commission *sdktypes.Dec,
) (*proto.CreateValidatorResponse, error) {

	req := &proto.CreateValidatorRequest{
		KeyName:     keyName,
		ChainId:     chainID,
		PassPhrase:  passPhrase,
		HdPath:      hdPath,
		Description: description,
		Commission:  commission.String(),
	}

	res, err := c.client.CreateValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ValidatorServiceGRpcClient) AddFinalitySignature(ctx context.Context, valPk string, height uint64, lch []byte) (*proto.AddFinalitySignatureResponse, error) {
	req := &proto.AddFinalitySignatureRequest{
		BtcPk:          valPk,
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

func (c *ValidatorServiceGRpcClient) QueryValidatorInfo(ctx context.Context, valPk *bbntypes.BIP340PubKey) (*proto.QueryValidatorResponse, error) {
	req := &proto.QueryValidatorRequest{BtcPk: valPk.MarshalHex()}
	res, err := c.client.QueryValidator(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
