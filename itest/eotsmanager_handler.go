package e2etest

import (
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
}

func NewEOTSServerHandler(t *testing.T, cfg *config.Config, eotsHomeDir string) *EOTSServerHandler {
	shutdownInterceptor, err := signal.Intercept()
	require.NoError(t, err)

	logger := zap.NewNop()
	_, store, err := eotsmanager.LoadHome(eotsHomeDir, cfg)
	eotsManager, err := eotsmanager.NewLocalEOTSManager(cfg, store, logger, eotsHomeDir)
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
}
