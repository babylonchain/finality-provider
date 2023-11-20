package service

import (
	"fmt"
	"sync/atomic"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/sirupsen/logrus"

	"github.com/babylonchain/btc-validator/covenant"
)

// CovenantServer is the main daemon construct for the covenant emulator.
type CovenantServer struct {
	started int32

	ce *covenant.CovenantEmulator

	logger *logrus.Logger

	interceptor signal.Interceptor

	quit chan struct{}
}

// NewCovenantServer creates a new server with the given config.
func NewCovenantServer(l *logrus.Logger, ce *covenant.CovenantEmulator, sig signal.Interceptor) *CovenantServer {
	return &CovenantServer{
		logger:      l,
		ce:          ce,
		interceptor: sig,
		quit:        make(chan struct{}, 1),
	}
}

// RunUntilShutdown runs the main EOTS manager server loop until a signal is
// received to shut down the process.
func (s *CovenantServer) RunUntilShutdown() error {
	if atomic.AddInt32(&s.started, 1) != 1 {
		return nil
	}

	defer func() {
		s.ce.Stop()
		s.logger.Info("Shutdown covenant emulator server complete")
	}()

	if err := s.ce.Start(); err != nil {
		return fmt.Errorf("failed to start covenant emulator: %w", err)
	}

	s.logger.Infof("Covenant Emulator Daemon is fully active!")

	// Wait for shutdown signal from either a graceful server stop or from
	// the interrupt handler.
	<-s.interceptor.ShutdownChannel()

	return nil
}
