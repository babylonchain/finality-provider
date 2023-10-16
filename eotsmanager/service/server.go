package service

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/config"
)

// Server is the main daemon construct for the EOTS manager server. It handles
// spinning up the RPC sever, the database, and any other components that the
// EOTS manager server needs to function.
type Server struct {
	started int32

	cfg    *config.Config
	logger *logrus.Logger

	rpcServer   *rpcServer
	interceptor signal.Interceptor

	quit chan struct{}
}

// NewEOTSManagerServer creates a new server with the given config.
func NewEOTSManagerServer(cfg *config.Config, l *logrus.Logger, em eotsmanager.EOTSManager, sig signal.Interceptor) *Server {
	return &Server{
		cfg:         cfg,
		logger:      l,
		rpcServer:   newRPCServer(em),
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

	mkErr := func(format string, args ...interface{}) error {
		logFormat := strings.ReplaceAll(format, "%w", "%v")
		s.logger.Errorf("Shutting down because error in main "+
			"method: "+logFormat, args...)
		return fmt.Errorf(format, args...)
	}

	// we create listeners from the RPCListeners defined
	// in the config.
	grpcListeners := make([]net.Listener, 0)
	for _, grpcEndpoint := range s.cfg.RpcListeners {
		// Start a gRPC server listening for HTTP/2
		// connections.
		lis, err := net.Listen(parseNetwork(grpcEndpoint), grpcEndpoint.String())
		if err != nil {
			return mkErr("unable to listen on %s: %v",
				grpcEndpoint, err)
		}
		defer lis.Close()

		grpcListeners = append(grpcListeners, lis)
	}

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	err := s.rpcServer.RegisterWithGrpcServer(grpcServer)
	if err != nil {
		return mkErr("error registering gRPC server: %v", err)
	}

	// All the necessary components have been registered, so we can
	// actually start listening for requests.
	err = s.startGrpcListen(grpcServer, grpcListeners)
	if err != nil {
		return mkErr("error starting gRPC listener: %v", err)
	}

	s.logger.Infof("EOTS Manager Daemon is fully active!")

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
			s.logger.Infof("RPC server listening on %s", lis.Addr())

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

// parseNetwork parses the network type of the given address.
func parseNetwork(addr net.Addr) string {
	switch addr := addr.(type) {
	// TCP addresses resolved through net.ResolveTCPAddr give a default
	// network of "tcp", so we'll map back the correct network for the given
	// address. This ensures that we can listen on the correct interface
	// (IPv4 vs IPv6).
	case *net.TCPAddr:
		if addr.IP.To4() != nil {
			return "tcp4"
		}
		return "tcp6"

	default:
		return addr.Network()
	}
}
