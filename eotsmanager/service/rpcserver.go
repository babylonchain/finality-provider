package service

import (
	"context"

	"google.golang.org/grpc"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/proto"
)

// rpcServer is the main RPC server for the BTC-Validator daemon that handles
// gRPC incoming requests.
type rpcServer struct {
	proto.UnimplementedEOTSManagerServer

	em eotsmanager.EOTSManager
}

// newRPCServer creates a new RPC sever from the set of input dependencies.
func newRPCServer(
	em eotsmanager.EOTSManager,
) *rpcServer {

	return &rpcServer{
		em: em,
	}
}

// RegisterWithGrpcServer registers the rpcServer with the passed root gRPC
// server.
func (r *rpcServer) RegisterWithGrpcServer(grpcServer *grpc.Server) error {
	// Register the main RPC server.
	proto.RegisterEOTSManagerServer(grpcServer, r)
	return nil
}

func (r *rpcServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{}, nil
}

// CreateKey generates and saves an EOTS key
func (r *rpcServer) CreateKey(ctx context.Context, req *proto.CreateKeyRequest) (
	*proto.CreateKeyResponse, error) {

	pk, err := r.em.CreateKey(req.Name, req.PassPhrase, req.HdPath)

	if err != nil {
		return nil, err
	}

	return &proto.CreateKeyResponse{Pk: pk}, nil
}

// CreateRandomnessPairList returns a list of Schnorr randomness pairs
func (r *rpcServer) CreateRandomnessPairList(ctx context.Context, req *proto.CreateRandomnessPairListRequest) (
	*proto.CreateRandomnessPairListResponse, error) {

	pubRandList, err := r.em.CreateRandomnessPairList(req.Uid, req.ChainId, req.StartHeight, req.Num)

	if err != nil {
		return nil, err
	}

	pubRandBytesList := make([][]byte, 0, len(pubRandList))
	for _, p := range pubRandList {
		pubRandBytesList = append(pubRandBytesList, p.Bytes()[:])
	}

	return &proto.CreateRandomnessPairListResponse{
		PubRandList: pubRandBytesList,
	}, nil
}

// KeyRecord returns the key record
func (r *rpcServer) KeyRecord(ctx context.Context, req *proto.KeyRecordRequest) (
	*proto.KeyRecordResponse, error) {

	record, err := r.em.KeyRecord(req.Uid)
	if err != nil {
		return nil, err
	}

	res := &proto.KeyRecordResponse{
		Name:       record.Name,
		PrivateKey: record.PrivKey.Serialize(),
	}

	return res, nil
}

// SignEOTS signs an EOTS with the EOTS private key and the relevant randomness
func (r *rpcServer) SignEOTS(ctx context.Context, req *proto.SignEOTSRequest) (
	*proto.SignEOTSResponse, error) {

	sig, err := r.em.SignEOTS(req.Uid, req.ChainId, req.Msg, req.Height)
	if err != nil {
		return nil, err
	}

	sigBytes := sig.Bytes()

	return &proto.SignEOTSResponse{Sig: sigBytes[:]}, nil
}

// SignSchnorrSig signs a Schnorr sig with the EOTS private key
func (r *rpcServer) SignSchnorrSig(ctx context.Context, req *proto.SignSchnorrSigRequest) (
	*proto.SignSchnorrSigResponse, error) {

	sig, err := r.em.SignSchnorrSig(req.Uid, req.Msg)
	if err != nil {
		return nil, err
	}

	return &proto.SignSchnorrSigResponse{Sig: sig.Serialize()}, nil
}
