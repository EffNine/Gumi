// Package dashboard serves the local Gumi control surface and proxies its
// API calls to the authenticated runtime gateway.
package dashboard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
)

type Server struct {
	server *http.Server
	log    *logger.Logger
}

func New(cfg *config.Config, log *logger.Logger) *Server {
	mux := http.NewServeMux()
	target, _ := url.Parse(fmt.Sprintf("http://%s:%d", cfg.Runtime.Host, cfg.Runtime.Port))
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")
		req.Header.Set("Authorization", "Bearer "+cfg.Auth.LocalKey)
	}
	mux.Handle("/api/", proxy)

	// Serve the documentation site from docs-site/ directory.
	// The docs site is a set of static HTML pages that explain Gumi's
	// architecture, quickstart, integrations, and benchmarks.
	docsDir := findDocsSiteDir()
	if docsDir != "" {
		docsFiles := http.FileServer(http.Dir(docsDir))
		mux.Handle("/docs-site/", http.StripPrefix("/docs-site/", docsFiles))
	}

	dist := findDistDir()
	if dist == "" {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("<!doctype html><title>Gumi Dashboard</title><style>body{font:16px system-ui;padding:48px;max-width:720px}code{background:#eee;padding:2px 6px}</style><h1>Dashboard build not found</h1><p>Run <code>npm install && npm run build</code> in the dashboard directory, then restart Gumi.</p>"))
		})
	} else {
		files := http.FileServer(http.Dir(dist))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(dist, filepath.Clean(r.URL.Path))
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
			http.ServeFile(w, r, filepath.Join(dist, "index.html"))
		})
	}

	return &Server{server: &http.Server{
		Addr:              net.JoinHostPort(cfg.Dashboard.Host, fmt.Sprintf("%d", cfg.Dashboard.Port)),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}, log: log}
}

func (s *Server) Start() <-chan error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("dashboard starting", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	return errCh
}

func (s *Server) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

func findDistDir() string {
	starts := []string{}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		current := start
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(current, "dashboard", "dist")
			if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
				return candidate
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return ""
}

// findDocsSiteDir locates the docs-site/ directory by walking up from the
// current working directory and the executable directory. Returns "" if not found.
func findDocsSiteDir() string {
	starts := []string{}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		current := start
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(current, "docs-site")
			if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
				return candidate
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return ""
}
