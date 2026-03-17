// Package api provides the REST API server for the lnget consumer dashboard.
package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
)

// Server is the REST API server for the lnget dashboard.
type Server struct {
	eventStore *events.Store
	tokenStore l402.Store
	backend    ln.Backend
	cfg        *config.Config
	httpServer *http.Server
	serveErr   chan error
}

// ServerConfig contains configuration for the API server.
type ServerConfig struct {
	// EventStore is the SQLite event store.
	EventStore *events.Store

	// TokenStore is the per-domain token store.
	TokenStore l402.Store

	// Backend is the Lightning backend.
	Backend ln.Backend

	// Config is the lnget configuration.
	Config *config.Config

	// DashboardDir is the optional path to static dashboard files.
	DashboardDir string
}

// NewServer creates a new API server.
func NewServer(cfg *ServerConfig) *Server {
	s := &Server{
		eventStore: cfg.EventStore,
		tokenStore: cfg.TokenStore,
		backend:    cfg.Backend,
		cfg:        cfg.Config,
		serveErr:   make(chan error, 1),
	}

	mux := http.NewServeMux()

	// Event endpoints.
	mux.HandleFunc("GET /api/events", s.handleListEvents)
	mux.HandleFunc("GET /api/events/stats", s.handleEventStats)
	mux.HandleFunc("GET /api/events/domains", s.handleDomainSpending)

	// Token endpoints.
	mux.HandleFunc("GET /api/tokens", s.handleListTokens)
	mux.HandleFunc("GET /api/tokens/{domain}", s.handleShowToken)
	mux.HandleFunc("DELETE /api/tokens/{domain}", s.handleRemoveToken)

	// Status endpoints.
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/config", s.handleConfig)

	// Serve static dashboard files. An explicit directory takes
	// priority over embedded assets. Next.js static export places
	// pages at <route>.html (e.g. payments.html) and creates
	// <route>/ directories for RSC payloads. A plain FileServer
	// would show directory listings for /payments/ instead of
	// serving payments.html, so we use a custom handler that
	// resolves clean URLs to their .html counterparts.
	switch {
	case cfg.DashboardDir != "":
		mux.Handle("/", nextStaticHandler(
			http.Dir(cfg.DashboardDir),
		))

	case dashboardEmbedded:
		sub, err := fs.Sub(embeddedDashboard, "dashboard_dist")
		if err != nil {
			log.Printf("warning: failed to load embedded "+
				"dashboard: %v", err)
		} else {
			mux.Handle("/", nextStaticHandler(
				http.FS(sub),
			))
		}
	}

	s.httpServer = &http.Server{
		Handler:        corsMiddleware(mux),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	return s
}

// Start starts the API server on the given address.
func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		err := s.httpServer.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			s.serveErr <- err
		}
		close(s.serveErr)
	}()

	return nil
}

// Err returns a channel that receives the first non-ErrServerClosed
// error from the underlying Serve goroutine. The channel is closed
// when Serve exits.
func (s *Server) Err() <-chan error {
	return s.serveErr
}

// Stop gracefully shuts down the API server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// ListenAndServe starts the server and blocks until shutdown.
func (s *Server) ListenAndServe(addr string) error {
	s.httpServer.Addr = addr

	return s.httpServer.ListenAndServe()
}

// corsMiddleware adds CORS headers for localhost origins.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow localhost origins on any port.
		if isLocalhostOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods",
				"GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isLocalhostOrigin checks whether the origin is a localhost URL by parsing
// the URL and comparing the hostname exactly. This prevents bypass via
// domains like "http://localhost.evil.com".
func isLocalhostOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := u.Hostname()

	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// nextStaticHandler returns an http.Handler that serves files from the
// given filesystem with Next.js static export routing. For clean URLs
// like /payments, it tries the path as-is first, then <path>.html,
// then falls back to index.html. This prevents directory listings for
// routes that have both a .html file and an RSC payload directory.
func nextStaticHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Serve static assets (_next/*, *.css, *.js, etc.)
		// and the root index directly.
		if path == "/" || strings.Contains(path, ".") ||
			strings.HasPrefix(path, "/_next/") {

			fileServer.ServeHTTP(w, r)

			return
		}

		// For clean URLs (e.g. /payments), try <path>.html.
		// Strip trailing slash first.
		cleanPath := strings.TrimSuffix(path, "/")
		htmlPath := cleanPath + ".html"

		f, err := fsys.Open(htmlPath)
		if err == nil {
			_ = f.Close()

			r.URL.Path = htmlPath
			fileServer.ServeHTTP(w, r)

			return
		}

		// Fall back to the default file server.
		fileServer.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		log.Printf("warning: failed to encode JSON response: %v",
			err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
