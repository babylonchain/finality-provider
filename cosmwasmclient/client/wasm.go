package client

import (
	"context"
	"fmt"
	"time"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbnclient "github.com/babylonchain/babylon/client/client"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	txsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// This function takes custom executeMsg and call designated cosmwasm contract execute endpoint
// it returns the instantiated contract address if succeeds
// Input fund example: "1000ubbn". Empty string can be passed if this execution doesn't intend to attach any fund.
func (c *Client) ExecuteContract(contractAddress string, code uint64, executeMsg []byte, fund string) (*sdk.TxResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	amount, _ := sdk.ParseCoinsNormalized(fund)
	msg := &wasmtypes.MsgExecuteContract{
		Sender:   c.cfg.SubmitterAddress,
		Contract: contractAddress,
		Msg:      executeMsg,
		Funds:    amount,
	}

	txBuilder := c.encodingCfg.TxConfig.NewTxBuilder()
	_ = txBuilder.SetMsgs(msg)

	return c.SignAndSendTx(ctx, txBuilder)
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

func (c *Client) SignAndSendTx(ctx context.Context, txBuilder sdkclient.TxBuilder) (*sdk.TxResponse, error) {
	if err := c.SignTx(ctx, txBuilder); err != nil {
		return nil, err
	}
	return c.SendTx(txBuilder)
}

func (c *Client) SignTx(ctx context.Context, txBuilder sdkclient.TxBuilder) error {
	bbnConfig := c.cfg.ToBabylonConfig()
	if err := bbnConfig.Validate(); err != nil {
		return fmt.Errorf("invalid config for Babylon client: %w", err)
	}
	bbnClient, err := bbnclient.New(
		&bbnConfig,
		c.logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create Babylon client: %w", err)
	}

	accountNumber, sequence, err := c.getAccountNumberSequenceNumber()
	if err != nil {
		return fmt.Errorf("failed to get account and sequence number: %w", err)
	}

	signMode, err := authsigning.APISignModeToInternal(c.encodingCfg.TxConfig.SignModeHandler().DefaultMode())
	if err != nil {
		return err
	}
	// sign any data to get pubKey
	_, pubKey, err := bbnClient.GetKeyring().Sign(bbnConfig.Key, []byte(""), signMode)
	if err != nil {
		return fmt.Errorf("failed to sign with the key %s: %w", bbnConfig.Key, err)
	}
	signerData := authsigning.SignerData{
		ChainID:       c.cfg.ChainID,
		AccountNumber: accountNumber,
		Sequence:      sequence,
		PubKey:        pubKey,
		Address:       sdk.AccAddress(pubKey.Address()).String(),
	}
	signData := txsigning.SingleSignatureData{
		SignMode:  signMode,
		Signature: nil,
	}
	var signsV2 []txsigning.SignatureV2
	signV2 := txsigning.SignatureV2{
		PubKey:   pubKey,
		Data:     &signData,
		Sequence: sequence,
	}
	signsV2 = append(signsV2, signV2)
	if err := txBuilder.SetSignatures(signsV2...); err != nil {
		return err
	}
	bytesToSign, err := authsigning.GetSignBytesAdapter(ctx, c.encodingCfg.TxConfig.SignModeHandler(), signMode, signerData, txBuilder.GetTx())
	if err != nil {
		return err
	}
	// Sign those bytes
	signBytes, _, err := bbnClient.GetKeyring().Sign(bbnConfig.Key, bytesToSign, signMode)
	if err != nil {
		return fmt.Errorf("failed to sign with the key %s: %w", bbnConfig.Key, err)
	}
	// Construct the SignatureV2 struct
	signData = txsigning.SingleSignatureData{
		SignMode:  signMode,
		Signature: signBytes,
	}
	signV2 = txsigning.SignatureV2{
		PubKey:   pubKey,
		Data:     &signData,
		Sequence: sequence,
	}
	// overwrite sign
	err = txBuilder.SetSignatures(signV2)
	if err != nil {
		return fmt.Errorf("unable to set signatures on payload: %w", err)
	}

	return nil
}

func (c *Client) SendTx(txBuilder sdkclient.TxBuilder) (*sdk.TxResponse, error) {
	// establish grpc connection
	grpcConn, err := grpc.NewClient(c.cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to build gRPC connection to %s: %w", c.cfg.GRPCAddr, err)
	}
	client := txtypes.NewServiceClient(grpcConn)

	txBytes, err := c.encodingCfg.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	grpcRes, err := client.BroadcastTx(
		context.Background(),
		&txtypes.BroadcastTxRequest{
			Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes,
		},
	)
	if err != nil {
		return nil, err
	}

	return grpcRes.TxResponse, nil
}

func (c *Client) getAccountNumberSequenceNumber() (uint64, uint64, error) {
	accountRetriever := authtypes.AccountRetriever{}
	cl, err := sdkclient.NewClientFromNode(c.cfg.RPCAddr)
	if err != nil {
		return 0, 0, err
	}
	context := sdkclient.Context{}
	context = context.WithNodeURI(c.cfg.RPCAddr)
	context = context.WithClient(cl)
	context = context.WithInterfaceRegistry(c.encodingCfg.InterfaceRegistry)
	account, seq, err := accountRetriever.GetAccountNumberSequence(context, sdk.AccAddress(c.cfg.SubmitterAddress))
	if err != nil {
		time.Sleep(5 * time.Second)
		// retry once after 5 seconds
		account, seq, err = accountRetriever.GetAccountNumberSequence(context, sdk.AccAddress(c.cfg.SubmitterAddress))
		if err != nil {
			panic(err)
		}
	}
	return account, seq, nil
}
