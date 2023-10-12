package service

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/eotsmanager/proto"
	"github.com/babylonchain/btc-validator/version"
)

// rpcServer is the main RPC server for the BTC-Validator daemon that handles
// gRPC incoming requests.
type rpcServer struct {
	started  int32
	shutdown int32

	proto.UnimplementedEOTSManagerServer

	interceptor signal.Interceptor

	em eotsmanager.EOTSManager

	logger *logrus.Logger

	cfg *config.Config

	quit chan struct{}
	wg   sync.WaitGroup
}

// newRPCServer creates a new RPC sever from the set of input dependencies.
func newRPCServer(
	interceptor signal.Interceptor,
	l *logrus.Logger,
	cfg *config.Config,
	em eotsmanager.EOTSManager,
) (*rpcServer, error) {

	return &rpcServer{
		interceptor: interceptor,
		logger:      l,
		quit:        make(chan struct{}),
		cfg:         cfg,
		em:          em,
	}, nil
}

// Start signals that the RPC server starts accepting requests.
func (r *rpcServer) Start() error {
	if atomic.AddInt32(&r.started, 1) != 1 {
		return nil
	}

	r.logger.Infof("Starting RPC Server")

	return nil
}

// Stop signals that the RPC server should attempt a graceful shutdown and
// cancel any outstanding requests.
func (r *rpcServer) Stop() error {
	if atomic.AddInt32(&r.shutdown, 1) != 1 {
		return nil
	}

	r.logger.Infof("Stopping RPC Server")

	close(r.quit)

	r.wg.Wait()

	return nil
}

// RegisterWithGrpcServer registers the rpcServer with the passed root gRPC
// server.
func (r *rpcServer) RegisterWithGrpcServer(grpcServer *grpc.Server) error {
	// Register the main RPC server.
	proto.RegisterEOTSManagerServer(grpcServer, r)
	return nil
}

// GetInfo returns general information relating to the active daemon
func (r *rpcServer) GetInfo(context.Context, *proto.GetInfoRequest) (*proto.GetInfoResponse, error) {

	return &proto.GetInfoResponse{
		Version: version.Version(),
	}, nil
}

// CreateKey generates and saves an EOTS key
func (r *rpcServer) CreateKey(ctx context.Context, req *proto.CreateKeyRequest) (
	*proto.CreateKeyResponse, error) {

	pk, err := r.em.CreateKey(req.Name, req.PassPhrase)

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

	record, err := r.em.KeyRecord(req.Uid, req.PassPhrase)
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

// SignSchnorrSig signs a Schnorr sig with teh EOTS private key
func (r *rpcServer) SignSchnorrSig(ctx context.Context, req *proto.SignSchnorrSigRequest) (
	*proto.SignSchnorrSigResponse, error) {

	sig, err := r.em.SignSchnorrSig(req.Uid, req.Msg)
	if err != nil {
		return nil, err
	}

	return &proto.SignSchnorrSigResponse{Sig: sig.Serialize()}, nil
}
