package client

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/proto"
	"github.com/babylonchain/btc-validator/eotsmanager/types"
)

var _ eotsmanager.EOTSManager = &EOTSManagerGRpcClient{}

type EOTSManagerGRpcClient struct {
	client proto.EOTSManagerClient
	conn   *grpc.ClientConn
}

func NewEOTSManagerGRpcClient(remoteAddr string) (*EOTSManagerGRpcClient, error) {
	conn, err := grpc.Dial(remoteAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to build gRPC connection to %s: %w", remoteAddr, err)
	}

	return &EOTSManagerGRpcClient{
		client: proto.NewEOTSManagerClient(conn),
		conn:   conn,
	}, nil
}

func (c *EOTSManagerGRpcClient) CreateKey(name, passPhrase string) ([]byte, error) {
	req := &proto.CreateKeyRequest{Name: name, PassPhrase: passPhrase}
	res, err := c.client.CreateKey(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return res.Pk, nil
}

func (c *EOTSManagerGRpcClient) CreateRandomnessPairList(uid, chainID []byte, startHeight uint64, num uint32) ([]*btcec.FieldVal, error) {
	req := &proto.CreateRandomnessPairListRequest{
		Uid:         uid,
		ChainId:     chainID,
		StartHeight: startHeight,
		Num:         num,
	}
	res, err := c.client.CreateRandomnessPairList(context.Background(), req)
	if err != nil {
		return nil, err
	}

	pubRandFieldValList := make([]*btcec.FieldVal, 0, len(res.PubRandList))
	for _, r := range res.PubRandList {
		var fieldVal *btcec.FieldVal
		fieldVal.SetByteSlice(r)
		pubRandFieldValList = append(pubRandFieldValList, fieldVal)
	}

	return pubRandFieldValList, nil
}

func (c *EOTSManagerGRpcClient) CreateRandomnessPairListWithExistenceCheck(uid, chainID []byte, startHeight uint64, num uint32) ([]*btcec.FieldVal, error) {
	// TODO consider remove this API when we no longer store randomness
	return c.CreateRandomnessPairList(uid, chainID, startHeight, num)
}

func (c *EOTSManagerGRpcClient) KeyRecord(uid []byte, passPhrase string) (*types.KeyRecord, error) {
	req := &proto.KeyRecordRequest{Uid: uid, PassPhrase: passPhrase}

	res, err := c.client.KeyRecord(context.Background(), req)
	if err != nil {
		return nil, err
	}

	privKey, _ := btcec.PrivKeyFromBytes(res.PrivateKey)

	return &types.KeyRecord{
		Name:    res.Name,
		PrivKey: privKey,
	}, nil
}

func (c *EOTSManagerGRpcClient) SignEOTS(uid, chaiID, msg []byte, height uint64) (*btcec.ModNScalar, error) {
	req := &proto.SignEOTSRequest{
		Uid:     uid,
		ChainId: chaiID,
		Msg:     msg,
		Height:  height,
	}
	res, err := c.client.SignEOTS(context.Background(), req)
	if err != nil {
		return nil, err
	}

	var s *btcec.ModNScalar
	s.SetByteSlice(res.Sig)

	return s, nil
}

func (c *EOTSManagerGRpcClient) SignSchnorrSig(uid, msg []byte) (*schnorr.Signature, error) {
	req := &proto.SignSchnorrSigRequest{Uid: uid, Msg: msg}
	res, err := c.client.SignSchnorrSig(context.Background(), req)
	if err != nil {
		return nil, err
	}

	sig, err := schnorr.ParseSignature(res.Sig)
	if err != nil {
		return nil, err
	}

	return sig, nil
}

func (c *EOTSManagerGRpcClient) Close() error {
	return c.conn.Close()
}
