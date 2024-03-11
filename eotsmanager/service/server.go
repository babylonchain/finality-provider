package service

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/lightningnetwork/lnd/kvdb"
	"github.com/lightningnetwork/lnd/signal"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/eotsmanager/config"
)

// Server is the main daemon construct for the EOTS manager server. It handles
// spinning up the RPC sever, the database, and any other components that the
// EOTS manager server needs to function.
type Server struct {
	started int32

	cfg    *config.Config
	logger *zap.Logger

	rpcServer   *rpcServer
	db          kvdb.Backend
	interceptor signal.Interceptor

	quit chan struct{}
}

// NewEOTSManagerServer creates a new server with the given config.
func NewEOTSManagerServer(cfg *config.Config, l *zap.Logger, em eotsmanager.EOTSManager, db kvdb.Backend, sig signal.Interceptor) *Server {
	return &Server{
		cfg:         cfg,
		logger:      l,
		rpcServer:   newRPCServer(em),
		db:          db,
		interceptor: sig,
		quit:        make(chan struct{}, 1),
	}
}

// RunUntilShutdown runs the main EOTS manager server loop until a signal is
// received to shut down the process.
func (s *Server) RunUntilShutdown() error {
	if atomic.AddInt32(&s.started, 1) != 1 {
		return nil
	}

	defer func() {
		s.logger.Info("Shutdown complete")
	}()

	defer func() {
		s.logger.Info("Closing database...")
		s.db.Close()
		s.logger.Info("Database closed")
	}()

	listenAddr := s.cfg.RpcListener
	// we create listeners from the RPCListeners defined
	// in the config.
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	if err := s.rpcServer.RegisterWithGrpcServer(grpcServer); err != nil {
		return fmt.Errorf("failed to register gRPC server: %w", err)
	}

	// All the necessary components have been registered, so we can
	// actually start listening for requests.
	if err := s.startGrpcListen(grpcServer, []net.Listener{lis}); err != nil {
		return fmt.Errorf("failed to start gRPC listener: %v", err)
	}

	s.logger.Info("EOTS Manager Daemon is fully active!")

	// Wait for shutdown signal from either a graceful server stop or from
	// the interrupt handler.
	<-s.interceptor.ShutdownChannel()

	return nil
}

// startGrpcListen starts the GRPC server on the passed listeners.
func (s *Server) startGrpcListen(grpcServer *grpc.Server, listeners []net.Listener) error {

	// Use a WaitGroup so we can be sure the instructions on how to input the
	// password is the last thing to be printed to the console.
	var wg sync.WaitGroup

	for _, lis := range listeners {
		wg.Add(1)
		go func(lis net.Listener) {
			s.logger.Info("RPC server listening", zap.String("address", lis.Addr().String()))

			// Close the ready chan to indicate we are listening.
			defer lis.Close()

			wg.Done()
			_ = grpcServer.Serve(lis)
		}(lis)
	}

	// Wait for gRPC servers to be up running.
	wg.Wait()

	return nil
}
