package e2e_utils

import (
	"testing"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/eotsmanager/service"
)

type EOTSServerHandler struct {
	t           *testing.T
	interceptor *signal.Interceptor
	eotsServers []*service.Server
}

func NewEOTSServerHandlerMultiFP(
	t *testing.T, configs []*config.Config, eotsHomeDirs []string, logger *zap.Logger,
) *EOTSServerHandler {
	shutdownInterceptor, err := signal.Intercept()
	require.NoError(t, err)

	eotsServers := make([]*service.Server, 0, len(configs))
	for i, cfg := range configs {
		dbBackend, err := cfg.DatabaseConfig.GetDbBackend()
		require.NoError(t, err)

		eotsManager, err := eotsmanager.NewLocalEOTSManager(eotsHomeDirs[i], cfg.KeyringBackend, dbBackend, logger)
		require.NoError(t, err)

		eotsServer := service.NewEOTSManagerServer(cfg, logger, eotsManager, dbBackend, shutdownInterceptor)
		eotsServers = append(eotsServers, eotsServer)
	}

	return &EOTSServerHandler{
		t:           t,
		eotsServers: eotsServers,
		interceptor: &shutdownInterceptor,
	}
}

func NewEOTSServerHandler(t *testing.T, cfg *config.Config, eotsHomeDir string) *EOTSServerHandler {
	// TODO: no-op logger makes it hard to debug. replace w real logger.
	// this need refactor of NewEOTSServerHandler
	return NewEOTSServerHandlerMultiFP(t, []*config.Config{cfg}, []string{eotsHomeDir}, zap.NewNop())
}

func (eh *EOTSServerHandler) Start() {
	go eh.startServer()
}

func (eh *EOTSServerHandler) startServer() {
	for _, eotsServer := range eh.eotsServers {
		go func(eotsServer *service.Server) {
			err := eotsServer.RunUntilShutdown()
			require.NoError(eh.t, err)
		}(eotsServer)
	}
}

func (eh *EOTSServerHandler) Stop() {
	eh.interceptor.RequestShutdown()
}
