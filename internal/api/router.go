package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/hue"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/loxone"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// Server represents the HTTP/WebSocket server
type Server struct {
	router         *mux.Router
	httpServer     *http.Server
	wsHub          *WebSocketHub
	handlers       *Handlers
	hueClient      *hue.Client
	mappingManager *loxone.MappingManager
	udpSender      *loxone.UDPSender
	httpSender     *loxone.HTTPSender
}

// NewServer creates a new API server
func NewServer(hueClient *hue.Client, mappingManager *loxone.MappingManager, udpSender *loxone.UDPSender, httpSender *loxone.HTTPSender) *Server {
	s := &Server{
		router:         mux.NewRouter(),
		hueClient:      hueClient,
		mappingManager: mappingManager,
		udpSender:      udpSender,
		httpSender:     httpSender,
	}

	s.wsHub = NewWebSocketHub(hueClient, mappingManager, udpSender, httpSender)
	s.handlers = NewHandlers(hueClient, mappingManager, udpSender, httpSender)
	s.handlers.feedbackDropped = s.wsHub.DroppedFeedback

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Middleware. Auth runs after CORS so preflights still get their headers,
	// and before everything else so no handler is reachable unauthenticated.
	s.router.Use(corsMiddleware)
	s.router.Use(authMiddleware)
	s.router.Use(bodyLimitMiddleware)
	s.router.Use(loggingMiddleware)

	// API routes
	api := s.router.PathPrefix("/api").Subrouter()

	// Bridge endpoints
	api.HandleFunc("/bridge", s.handlers.GetBridge).Methods("GET")
	api.HandleFunc("/bridge/discover", s.handlers.DiscoverBridges).Methods("GET")
	api.HandleFunc("/bridge/pair", s.handlers.PairBridge).Methods("POST")
	api.HandleFunc("/bridge/test", s.handlers.TestBridgeConnection).Methods("POST")

	// Device endpoints
	api.HandleFunc("/devices", s.handlers.GetDevices).Methods("GET")
	api.HandleFunc("/devices/{id}", s.handlers.GetDevice).Methods("GET")
	api.HandleFunc("/devices/{id}", s.handlers.SetDevice).Methods("PUT", "POST")

	// Group endpoints
	api.HandleFunc("/groups", s.handlers.GetGroups).Methods("GET")
	api.HandleFunc("/groups/{id}", s.handlers.GetGroup).Methods("GET")
	api.HandleFunc("/groups/{id}", s.handlers.SetGroup).Methods("PUT", "POST")

	// Scene endpoints
	api.HandleFunc("/scenes", s.handlers.GetScenes).Methods("GET")
	api.HandleFunc("/scenes/{id}/activate", s.handlers.ActivateScene).Methods("POST")

	// Sensor endpoints
	api.HandleFunc("/sensors", s.handlers.GetSensors).Methods("GET")

	// Mapping endpoints
	api.HandleFunc("/mappings", s.handlers.GetMappings).Methods("GET")
	api.HandleFunc("/mappings", s.handlers.CreateMapping).Methods("POST")
	api.HandleFunc("/mappings/{id}", s.handlers.UpdateMapping).Methods("PUT")
	api.HandleFunc("/mappings/{id}", s.handlers.DeleteMapping).Methods("DELETE")
	api.HandleFunc("/mappings/export", s.handlers.ExportMappings).Methods("GET")
	api.HandleFunc("/mappings/import", s.handlers.ImportMappings).Methods("POST")

	// Config endpoints
	api.HandleFunc("/config", s.handlers.GetConfig).Methods("GET")
	api.HandleFunc("/config", s.handlers.UpdateConfig).Methods("PUT")

	// Login: status and login are reachable without credentials (see isAuthExempt)
	api.HandleFunc("/auth/status", s.handlers.AuthStatus).Methods("GET")
	api.HandleFunc("/auth/login", s.handlers.Login).Methods("POST")
	api.HandleFunc("/auth/logout", s.handlers.Logout).Methods("POST")
	api.HandleFunc("/auth", s.handlers.UpdateAuth).Methods("PUT")

	// Backup endpoints
	api.HandleFunc("/backups", s.handlers.ListBackups).Methods("GET")
	api.HandleFunc("/backups", s.handlers.CreateBackup).Methods("POST")
	api.HandleFunc("/backups/upload", s.handlers.UploadBackup).Methods("POST")
	api.HandleFunc("/backups/{id}", s.handlers.DeleteBackup).Methods("DELETE")
	api.HandleFunc("/backups/{id}/download", s.handlers.DownloadBackup).Methods("GET")
	api.HandleFunc("/backups/{id}/restore", s.handlers.RestoreBackup).Methods("POST")

	// UDP test endpoint
	api.HandleFunc("/udp/test", s.handlers.TestUDP).Methods("POST")

	// Loxone XML export endpoints
	api.HandleFunc("/export/inputs", s.handlers.ExportVirtualInputs).Methods("GET")
	api.HandleFunc("/export/outputs", s.handlers.ExportVirtualOutputs).Methods("GET")

	// Logs endpoint
	api.HandleFunc("/logs", s.handlers.GetLogs).Methods("GET")

	// Health check
	api.HandleFunc("/health", s.handlers.Health).Methods("GET")

	// Swagger documentation
	api.HandleFunc("/swagger.json", s.SwaggerSpec).Methods("GET")
	api.HandleFunc("/swagger", s.SwaggerUI).Methods("GET")
	api.HandleFunc("/swagger/", s.SwaggerUI).Methods("GET")

	// WebSocket endpoint
	s.router.HandleFunc("/ws", s.wsHub.HandleWebSocket)

	// Serve static files (frontend)
	s.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/dist")))
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start WebSocket hub
	go s.wsHub.Run(ctx)

	log.Info().Str("addr", addr).Str("auth", authStatus()).Msg("Starting HTTP server")

	if !config.GetAuth().Enabled() && !isLoopback(host) {
		log.Warn().
			Str("host", host).
			Msg("No password configured: the API, the device control and the backup download " +
				"(which contains the HUE application key) are reachable by anyone on this network. " +
				"Set a password under Einstellungen > Login-Schutz (or auth.password in the configuration) to require a login.")
	}

	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown()
	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Info().Msg("Shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// isSameOriginRequest reports whether a request's Origin header is absent
// (non-browser client) or matches the request's own Host. This blocks
// cross-site / DNS-rebinding requests from arbitrary web pages while still
// allowing the bundled same-origin UI and non-browser API clients.
func isSameOriginRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser client (curl, Loxone, etc.)
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

// corsMiddleware adds CORS headers, reflecting the Origin only for same-origin
// requests. It never emits a wildcard, so foreign sites cannot read responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && isSameOriginRequest(r) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// maxRequestBody caps the size of any request body. The largest legitimate
// payload is a configuration backup with a few hundred mappings, well under
// 1 MiB; anything bigger is a mistake or an attempt to exhaust memory.
const maxRequestBody = 2 << 20

// bodyLimitMiddleware wraps every request body in a MaxBytesReader. Handlers
// decode through decodeJSON, which turns the resulting error into a 413.
func bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Header.Get("Upgrade") != "websocket" {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging wrapper for WebSocket upgrades to preserve Hijacker interface
		if r.Header.Get("Upgrade") == "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.statusCode).
			Dur("duration", time.Since(start)).
			Msg("HTTP request")
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
