package e2etest

import (
	"testing"

	"github.com/lightningnetwork/lnd/signal"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/btc-validator/eotsmanager/config"
	"github.com/babylonchain/btc-validator/eotsmanager/local"
	"github.com/babylonchain/btc-validator/eotsmanager/service"
)

type EOTSServerHandler struct {
	t           *testing.T
	interceptor *signal.Interceptor
	eotsServer  *service.Server
}

func (eh *EOTSServerHandler) NewEOTSServerHandler(t *testing.T, cfg *config.Config) *EOTSServerHandler {
	shutdownInterceptor, err := signal.Intercept()
	require.NoError(t, err)

	logger := logrus.New()
	eotsManager, err := local.NewLocalEOTSManager(cfg, logger)
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
