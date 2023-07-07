package service

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/btc-validator/valrpc"
	"github.com/babylonchain/btc-validator/version"
)

// rpcServer is the main RPC server for the BTC-Validator daemon that handles
// gRPC incoming requests.
type rpcServer struct {
	started  int32
	shutdown int32

	valrpc.UnimplementedBtcValidatorsServer

	interceptor signal.Interceptor

	logger *logrus.Logger

	cfg *valcfg.Config

	quit chan struct{}
	wg   sync.WaitGroup
}

// newRPCServer creates a new RPC sever from the set of input dependencies.
func newRPCServer(interceptor signal.Interceptor,
	cfg *valcfg.Config) (*rpcServer, error) {

	return &rpcServer{
		interceptor: interceptor,
		quit:        make(chan struct{}),
		cfg:         cfg,
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
	valrpc.RegisterBtcValidatorsServer(grpcServer, r)
	return nil
}

// GetInfo returns general information relating to the active daemon
func (r *rpcServer) GetInfo(context.Context, *valrpc.GetInfoRequest) (*valrpc.GetInfoResponse, error) {

	return &valrpc.GetInfoResponse{
		Version: version.Version(),
	}, nil
}

// CreateValidator generates a validator object and saves it in the database
func (r *rpcServer) CreateValidator(ctx context.Context, req *valrpc.CreateValidatorRequest) (
	*valrpc.CreateValidatorResponse, error) {
	panic("implement me")
}

// RegisterValidator sends a transactions to Babylon to register a BTC validator
func (r *rpcServer) RegisterValidator(ctx context.Context, req *valrpc.RegisterValidatorRequest) (
	*valrpc.RegisterValidatorResponse, error) {
	panic("implement me")
}

// QueryValidator queries the information of the validator
func (r *rpcServer) QueryValidator(ctx context.Context, req *valrpc.QueryValidatorRequest) (
	*valrpc.QueryValidatorResponse, error) {
	panic("implement me")
}

// QueryValidatorList queries the information of a list of validators
func (r *rpcServer) QueryValidatorList(ctx context.Context, req *valrpc.QueryValidatorListRequest) (
	*valrpc.QueryValidatorListResponse, error) {
	panic("implement me")
}
