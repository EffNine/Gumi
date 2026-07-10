// Package gateway exposes the Novexa HTTP API.
//
// It is intentionally thin in Sprint 2: it handles routing, middleware, request
// serialization, and placeholder responses. Real provider calls will be added
// in Sprint 3 via the Pipeline Engine.
package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
	"github.com/novexa/novexa/runtime/internal/provider"
)

// Server wraps the Novexa HTTP gateway.
type Server struct {
	cfg     *config.Config
	log     *logger.Logger
	manager *provider.Manager
	server  *http.Server
	addr    string
}

// New creates a gateway server from configuration and logger.
func New(cfg *config.Config, log *logger.Logger) *Server {
	mux := http.NewServeMux()

	mgr, err := provider.DefaultRegistry().Build(cfg, log)
	if err != nil {
		log.Error("failed to build provider manager", err)
		mgr = provider.NewManager(make(map[string]provider.ProviderAdapter), log)
	}

	s := &Server{
		cfg:     cfg,
		log:     log,
		manager: mgr,
		addr:    net.JoinHostPort(cfg.Runtime.Host, fmt.Sprintf("%d", cfg.Runtime.Port)),
		server: &http.Server{
			Addr:              net.JoinHostPort(cfg.Runtime.Host, fmt.Sprintf("%d", cfg.Runtime.Port)),
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       120 * time.Second,
			WriteTimeout:      120 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}

	s.registerRoutes(mux)
	return s
}

// Start begins listening for HTTP requests in a goroutine.
// The returned channel receives the listen error once.
func (s *Server) Start() <-chan error {
	errCh := make(chan error, 1)

	s.log.Info("gateway starting", "addr", s.server.Addr)
	go func() {
		ln, err := net.Listen("tcp", s.server.Addr)
		if err != nil {
			errCh <- err
			return
		}
		// Update Addr to the actual bound address so callers can discover
		// dynamically assigned ports in tests.
		s.server.Addr = ln.Addr().String()
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	return errCh
}

// Addr returns the bound address once the server has started.
func (s *Server) Addr() string {
	return s.server.Addr
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("gateway shutting down")
	return s.server.Shutdown(ctx)
}
