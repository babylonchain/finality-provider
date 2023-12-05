package e2etest

import (
	"os"
	"testing"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonchain/btc-validator/eotsmanager"
	"github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/eotsmanager/service"
)

type EOTSServerHandler struct {
	t           *testing.T
	interceptor *signal.Interceptor
	eotsServer  *service.Server
	baseDir     string
}

func NewEOTSServerHandler(t *testing.T, cfg *config.Config) *EOTSServerHandler {
	shutdownInterceptor, err := signal.Intercept()
	require.NoError(t, err)

	logger := zap.NewNop()
	eotsManager, err := eotsmanager.NewLocalEOTSManager(cfg, logger)
	require.NoError(t, err)

	eotsServer := service.NewEOTSManagerServer(cfg, logger, eotsManager, shutdownInterceptor)

	return &EOTSServerHandler{
		t:           t,
		eotsServer:  eotsServer,
		interceptor: &shutdownInterceptor,
	}
}

func (eh *EOTSServerHandler) Start() {
	go eh.startServer()
}

func (eh *EOTSServerHandler) startServer() {
	err := eh.eotsServer.RunUntilShutdown()
	require.NoError(eh.t, err)
}

func (eh *EOTSServerHandler) Stop() {
	eh.interceptor.RequestShutdown()
	err := os.RemoveAll(eh.baseDir)
	require.NoError(eh.t, err)
}
