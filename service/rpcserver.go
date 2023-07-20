package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/lightningnetwork/lnd/signal"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/babylonchain/btc-validator/proto"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/btc-validator/version"
)

// rpcServer is the main RPC server for the BTC-Validator daemon that handles
// gRPC incoming requests.
type rpcServer struct {
	started  int32
	shutdown int32

	proto.UnimplementedBtcValidatorsServer

	interceptor signal.Interceptor

	app *ValidatorApp

	logger *logrus.Logger

	cfg *valcfg.Config

	quit chan struct{}
	wg   sync.WaitGroup
}

// newRPCServer creates a new RPC sever from the set of input dependencies.
func newRPCServer(
	interceptor signal.Interceptor,
	l *logrus.Logger,
	cfg *valcfg.Config,
	v *ValidatorApp,
) (*rpcServer, error) {

	return &rpcServer{
		interceptor: interceptor,
		logger:      l,
		quit:        make(chan struct{}),
		cfg:         cfg,
		app:         v,
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
	proto.RegisterBtcValidatorsServer(grpcServer, r)
	return nil
}

// GetInfo returns general information relating to the active daemon
func (r *rpcServer) GetInfo(context.Context, *proto.GetInfoRequest) (*proto.GetInfoResponse, error) {

	return &proto.GetInfoResponse{
		Version: version.Version(),
	}, nil
}

// CreateValidator generates a validator object and saves it in the database
func (r *rpcServer) CreateValidator(ctx context.Context, req *proto.CreateValidatorRequest) (
	*proto.CreateValidatorResponse, error) {
	result, err := r.app.CreateValidator(req.KeyName)

	if err != nil {
		return nil, err
	}

	btcPk := schnorr.SerializePubKey(&result.BtcValidatorPk)

	return &proto.CreateValidatorResponse{
		BtcPk:     hex.EncodeToString(btcPk),
		BabylonPk: hex.EncodeToString(result.BabylonValidatorPk.Key),
	}, nil

}

// RegisterValidator sends a transactions to Babylon to register a BTC validator
func (r *rpcServer) RegisterValidator(ctx context.Context, req *proto.RegisterValidatorRequest) (
	*proto.RegisterValidatorResponse, error) {

	txHash, err := r.app.RegisterValidator(req.BabylonPk)
	if err != nil {
		return nil, err
	}

	return &proto.RegisterValidatorResponse{TxHash: txHash}, nil
}

// CommitPubRandForValidator commits a list of Schnorr public randomness for a specific BTC validator
func (r *rpcServer) CommitPubRandForValidator(ctx context.Context, req *proto.CommitPubRandForValidatorRequest) (
	*proto.CommitPubRandForValidatorResponse, error) {

	if req.Num > r.cfg.RandomNumMax {
		return nil, fmt.Errorf("the request public rand num %v should not be larger than %v",
			req.Num, r.cfg.RandomNumMax)
	}

	if req.BabylonPk == nil {
		return nil, fmt.Errorf("the Babylon public key should not be nil")
	}

	txHash, err := r.app.CommitPubRandForValidator(req.BabylonPk, req.Num)
	if err != nil {
		return nil, err
	}

	return &proto.CommitPubRandForValidatorResponse{TxHash: txHash}, nil
}

// CommitPubRandForAll commits a list of Schnorr public randomness for each managed BTC validator
func (r *rpcServer) CommitPubRandForAll(ctx context.Context, req *proto.CommitPubRandForAllRequest) (
	*proto.CommitPubRandForAllResponse, error) {

	if req.Num > r.cfg.RandomNumMax {
		return nil, fmt.Errorf("the request public rand num %v should not be larger than %v",
			req.Num, r.cfg.RandomNumMax)
	}

	txHashes, err := r.app.CommitPubRandForAll(req.Num)
	if err != nil {
		return nil, err
	}

	return &proto.CommitPubRandForAllResponse{TxHashes: txHashes}, nil
}

// QueryValidator queries the information of the validator
func (r *rpcServer) QueryValidator(ctx context.Context, req *proto.QueryValidatorRequest) (
	*proto.QueryValidatorResponse, error) {
	panic("implement me")
}

// QueryValidatorList queries the information of a list of validators
func (r *rpcServer) QueryValidatorList(ctx context.Context, req *proto.QueryValidatorListRequest) (
	*proto.QueryValidatorListResponse, error) {

	vals, err := r.app.ListValidators()
	if err != nil {
		return nil, err
	}

	return &proto.QueryValidatorListResponse{Validators: vals}, nil
}
