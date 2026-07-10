// Package gateway exposes the Novexa HTTP API.
//
// The gateway stays intentionally thin: it handles routing, middleware, and
// request serialization, then forwards inference requests to Pipeline Engine.
package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
	"github.com/novexa/novexa/runtime/internal/pipeline"
	"github.com/novexa/novexa/runtime/internal/profiles"
	"github.com/novexa/novexa/runtime/internal/provider"
	"github.com/novexa/novexa/runtime/internal/telemetry"
)

// Server wraps the Novexa HTTP gateway.
type Server struct {
	cfg       *config.Config
	log       *logger.Logger
	manager   *provider.Manager
	pipeline  *pipeline.Engine
	telemetry *telemetry.Writer
	profiles  []*profiles.Profile
	server    *http.Server
	addr      string
}

// New creates a gateway server from configuration and logger.
func New(cfg *config.Config, log *logger.Logger) *Server {
	mux := http.NewServeMux()

	mgr, err := provider.DefaultRegistry().Build(cfg, log)
	if err != nil {
		log.Error("failed to build provider manager", err)
		mgr = provider.NewManager(make(map[string]provider.ProviderAdapter), log)
	}

	var tw *telemetry.Writer
	if cfg.Telemetry.Local {
		tw, err = telemetry.Open(cfg, log)
		if err != nil {
			log.Error("telemetry storage unavailable; continuing in degraded mode", err)
			tw = telemetry.NewNoop(cfg, log)
		}
	} else {
		tw = telemetry.NewNoop(cfg, log)
	}

	mgr.Telemetry = tw

	loadedProfiles, _ := profiles.NewDefaultLoader().Load()
	pipe := pipeline.New(cfg, mgr, log)
	pipe.SetTelemetry(tw)

	s := &Server{
		cfg:       cfg,
		log:       log,
		manager:   mgr,
		pipeline:  pipe,
		telemetry: tw,
		profiles:  loadedProfiles.Profiles,
		addr:      net.JoinHostPort(cfg.Runtime.Host, fmt.Sprintf("%d", cfg.Runtime.Port)),
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

// Shutdown gracefully stops the HTTP server and closes telemetry storage.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("gateway shutting down")
	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}
	if s.telemetry != nil {
		_ = s.telemetry.Close()
	}
	return nil
}
