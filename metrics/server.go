package metrics

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"net/http"
)

// Server represents the metrics server.
type Server struct {
	httpServer *http.Server
}

func Start(addr string) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Create the HTTP server with the custom ServeMux as the handler
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start the metrics server in a goroutine.
	go func() {
		log.Info().Str("addr", addr).Msg("Metrics server is starting")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Metrics server failed to start")
		}
	}()

	return &Server{httpServer: server}
}

func (s *Server) Stop(ctx context.Context) {
	log.Info().Msg("Stopping metrics server")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Metrics server shutdown failed")
	} else {
		log.Info().Msg("Metrics server stopped gracefully")
	}
}
