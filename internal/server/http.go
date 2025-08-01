package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

// HTTPServer implements an HTTP server for the web UI and REST API
type HTTPServer struct {
	config     *types.Config
	logService interfaces.LogService
	server     *http.Server
	
	// WebSocket upgrader
	upgrader   websocket.Upgrader
	
	// Static files (embedded or filesystem)
	staticFS    fs.FS
	useEmbedded bool
	
	// Server lifecycle
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isRunning  bool
	runningMux sync.RWMutex
	
	// Statistics
	stats      HTTPServerStats
	statsMutex sync.RWMutex
}

// HTTPServerStats represents statistics about the HTTP server
type HTTPServerStats struct {
	RequestsHandled     int64 `json:"requests_handled"`
	RequestErrors       int64 `json:"request_errors"`
	WebSocketConnections int64 `json:"websocket_connections"`
	ActiveWebSockets    int64 `json:"active_websockets"`
	IsRunning           bool  `json:"is_running"`
}

// APIResponse represents a standard API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Services  map[string]interface{} `json:"services"`
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(config *types.Config, logService interfaces.LogService) *HTTPServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Configure WebSocket upgrader
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins for now - in production, this should be more restrictive
			return true
		},
	}
	
	return &HTTPServer{
		config:      config,
		logService:  logService,
		upgrader:    upgrader,
		useEmbedded: false,
		ctx:         ctx,
		cancel:      cancel,
		stats: HTTPServerStats{
			IsRunning: false,
		},
	}
}

// NewHTTPServerWithStaticFiles creates a new HTTP server instance with embedded static files
func NewHTTPServerWithStaticFiles(config *types.Config, logService interfaces.LogService, staticFS fs.FS) *HTTPServer {
	server := NewHTTPServer(config, logService)
	server.staticFS = staticFS
	server.useEmbedded = true
	return server
}

// Start starts the HTTP server
func (s *HTTPServer) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()
	
	if s.isRunning {
		return fmt.Errorf("HTTP server is already running")
	}
	
	// Create HTTP server with routes
	mux := http.NewServeMux()
	s.setupRoutes(mux)
	
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.HTTPPort),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	s.isRunning = true
	
	// Update stats
	s.updateStats(func(stats *HTTPServerStats) {
		stats.IsRunning = true
	})
	
	// Start server in goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		
		log.Printf("HTTP server starting on port %d", s.config.HTTPPort)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	
	return nil
}

// Stop gracefully stops the HTTP server
func (s *HTTPServer) Stop() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()
	
	if !s.isRunning {
		return nil
	}
	
	// Cancel context to signal shutdown
	s.cancel()
	
	// Shutdown server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		return err
	}
	
	// Wait for all goroutines to finish
	s.wg.Wait()
	
	s.isRunning = false
	
	// Update stats
	s.updateStats(func(stats *HTTPServerStats) {
		stats.IsRunning = false
	})
	
	log.Printf("HTTP server stopped")
	return nil
}

// GetStats returns server statistics
func (s *HTTPServer) GetStats() HTTPServerStats {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()
	return s.stats
}

// setupRoutes configures all HTTP routes
func (s *HTTPServer) setupRoutes(mux *http.ServeMux) {
	// API routes
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/logs", s.authMiddleware(s.handleLogs))
	mux.HandleFunc("/api/logs/stream", s.authMiddleware(s.handleLogsStream))
	
	// Static file serving
	mux.HandleFunc("/", s.authMiddleware(s.handleIndex))
	mux.HandleFunc("/static/", s.handleStatic)
}

// authMiddleware provides HTTP Basic Authentication when enabled
func (s *HTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if not enabled
		if !s.config.AuthEnabled {
			next(w, r)
			return
		}
		
		// Get credentials from request
		username, password, ok := r.BasicAuth()
		if !ok {
			s.sendUnauthorized(w)
			return
		}
		
		// Validate credentials with timing attack protection
		validUsername := s.config.AuthUsername
		validPassword := s.config.AuthPassword
		
		// Use constant-time comparison to prevent timing attacks
		// Always compare the same amount of data regardless of input length
		usernameMatch := s.constantTimeCompare(username, validUsername)
		passwordMatch := s.constantTimeCompare(password, validPassword)
		
		if !usernameMatch || !passwordMatch {
			s.sendUnauthorized(w)
			return
		}
		
		// Authentication successful, proceed to handler
		next(w, r)
	}
}

// constantTimeCompare performs constant-time string comparison to prevent timing attacks
func (s *HTTPServer) constantTimeCompare(a, b string) bool {
	// Convert strings to byte slices for comparison
	aBytes := []byte(a)
	bBytes := []byte(b)
	
	// Determine the maximum length to compare
	maxLen := len(aBytes)
	if len(bBytes) > maxLen {
		maxLen = len(bBytes)
	}
	
	// Pad shorter slice with zeros to ensure constant-time comparison
	if len(aBytes) < maxLen {
		padded := make([]byte, maxLen)
		copy(padded, aBytes)
		aBytes = padded
	}
	if len(bBytes) < maxLen {
		padded := make([]byte, maxLen)
		copy(padded, bBytes)
		bBytes = padded
	}
	
	// Perform constant-time comparison
	result := byte(0)
	for i := 0; i < maxLen; i++ {
		result |= aBytes[i] ^ bBytes[i]
	}
	
	// Also check that the original lengths match
	lengthMatch := len(a) == len(b)
	
	return result == 0 && lengthMatch
}

// sendUnauthorized sends a 401 Unauthorized response
func (s *HTTPServer) sendUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="OpenTrail"`)
	s.sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
}

// handleHealth handles the health check endpoint
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.updateStats(func(stats *HTTPServerStats) {
		stats.RequestsHandled++
	})
	
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Get service stats
	serviceStats := s.logService.GetStats()
	
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   getVersion(),
		Services: map[string]interface{}{
			"log_service": serviceStats,
			"http_server": s.GetStats(),
		},
	}
	
	s.sendJSONResponse(w, http.StatusOK, response)
}

// handleLogs handles the logs query endpoint
func (s *HTTPServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	s.updateStats(func(stats *HTTPServerStats) {
		stats.RequestsHandled++
	})
	
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Parse query parameters
	query, err := s.parseSearchQuery(r)
	if err != nil {
		s.sendErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid query parameters: %v", err))
		return
	}
	
	// Execute search
	logs, err := s.logService.Search(query)
	if err != nil {
		log.Printf("Error searching logs: %v", err)
		s.sendErrorResponse(w, http.StatusInternalServerError, "Failed to search logs")
		return
	}
	
	// Return results
	s.sendJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    logs,
	})
}

// parseSearchQuery parses HTTP query parameters into a SearchQuery
func (s *HTTPServer) parseSearchQuery(r *http.Request) (types.SearchQuery, error) {
	query := types.SearchQuery{
		Limit: 100, // Default limit
	}
	
	// Parse text search
	if text := r.URL.Query().Get("text"); text != "" {
		query.Text = text
	}
	
	// Parse level filter
	if level := r.URL.Query().Get("level"); level != "" {
		query.Level = level
	}
	
	// Parse tracking ID filter
	if trackingID := r.URL.Query().Get("tracking_id"); trackingID != "" {
		query.TrackingID = trackingID
	}
	
	// Parse start time
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return query, fmt.Errorf("invalid start_time format, expected RFC3339: %w", err)
		}
		query.StartTime = &startTime
	}
	
	// Parse end time
	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return query, fmt.Errorf("invalid end_time format, expected RFC3339: %w", err)
		}
		query.EndTime = &endTime
	}
	
	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 1000 {
			return query, fmt.Errorf("invalid limit, must be between 1 and 1000")
		}
		query.Limit = limit
	}
	
	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return query, fmt.Errorf("invalid offset, must be >= 0")
		}
		query.Offset = offset
	}
	
	return query, nil
}

// sendJSONResponse sends a JSON response
func (s *HTTPServer) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		s.updateStats(func(stats *HTTPServerStats) {
			stats.RequestErrors++
		})
	}
}

// sendErrorResponse sends an error response
func (s *HTTPServer) sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	s.updateStats(func(stats *HTTPServerStats) {
		stats.RequestErrors++
	})
	
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	
	s.sendJSONResponse(w, statusCode, response)
}

// handleLogsStream handles WebSocket connections for real-time log streaming
func (s *HTTPServer) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	s.updateStats(func(stats *HTTPServerStats) {
		stats.RequestsHandled++
	})
	
	// Only allow GET requests for WebSocket upgrade
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		s.updateStats(func(stats *HTTPServerStats) {
			stats.RequestErrors++
		})
		return
	}
	
	// Update connection statistics
	s.updateStats(func(stats *HTTPServerStats) {
		stats.WebSocketConnections++
		stats.ActiveWebSockets++
	})
	
	// Handle the WebSocket connection
	s.handleWebSocketConnection(conn)
}

// handleWebSocketConnection manages a single WebSocket connection
func (s *HTTPServer) handleWebSocketConnection(conn *websocket.Conn) {
	defer func() {
		conn.Close()
		s.updateStats(func(stats *HTTPServerStats) {
			stats.ActiveWebSockets--
		})
	}()
	
	// Set connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	// Subscribe to log updates
	subscription := s.logService.Subscribe()
	defer s.logService.Unsubscribe(subscription)
	
	// Create context for this connection
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	
	// Start ping ticker to keep connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	
	// Handle connection in separate goroutines
	s.wg.Add(2)
	
	// Goroutine to handle incoming messages (for connection keep-alive)
	go func() {
		defer s.wg.Done()
		defer cancel()
		
		for {
			// Read message to detect client disconnection
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				return
			}
		}
	}()
	
	// Goroutine to handle outgoing messages and pings
	go func() {
		defer s.wg.Done()
		defer cancel()
		
		for {
			select {
			case logEntry, ok := <-subscription:
				if !ok {
					// Subscription channel closed
					return
				}
				
				// Send log entry to client
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(logEntry); err != nil {
					log.Printf("WebSocket write error: %v", err)
					return
				}
				
			case <-pingTicker.C:
				// Send ping to keep connection alive
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("WebSocket ping error: %v", err)
					return
				}
				
			case <-ctx.Done():
				// Connection context cancelled or server shutting down
				return
			}
		}
	}()
	
	// Wait for either goroutine to finish
	<-ctx.Done()
}

// updateStats safely updates the server statistics
func (s *HTTPServer) updateStats(updateFunc func(*HTTPServerStats)) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()
	updateFunc(&s.stats)
}

// getVersion returns the application version (placeholder for build-time injection)
func getVersion() string {
	// This will be replaced by build-time version injection
	return "1.0.0"
}

// handleIndex serves the main UI page
func (s *HTTPServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.updateStats(func(stats *HTTPServerStats) {
		stats.RequestsHandled++
	})
	
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Only serve index for root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	
	// Serve the index.html file
	if s.useEmbedded {
		s.serveEmbeddedFile(w, r, "index.html")
	} else {
		indexPath := s.getStaticFilePath("index.html")
		http.ServeFile(w, r, indexPath)
	}
}

// handleStatic serves static files (CSS, JS, etc.)
func (s *HTTPServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	s.updateStats(func(stats *HTTPServerStats) {
		stats.RequestsHandled++
	})
	
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	// Remove /static/ prefix and serve from web/static/
	path := r.URL.Path[len("/static/"):]
	if path == "" {
		http.NotFound(w, r)
		return
	}
	
	// Set appropriate content type
	switch {
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(path, ".html"):
		w.Header().Set("Content-Type", "text/html")
	}
	
	// Serve the file
	if s.useEmbedded {
		s.serveEmbeddedFile(w, r, path)
	} else {
		filePath := s.getStaticFilePath(path)
		http.ServeFile(w, r, filePath)
	}
}

// serveEmbeddedFile serves a file from the embedded filesystem
func (s *HTTPServer) serveEmbeddedFile(w http.ResponseWriter, r *http.Request, filename string) {
	// Read the file from the filesystem
	data, err := fs.ReadFile(s.staticFS, filename)
	if err != nil {
		log.Printf("Error reading embedded file %s: %v", filename, err)
		http.NotFound(w, r)
		return
	}
	
	// Set cache headers for static assets
	w.Header().Set("Cache-Control", "public, max-age=3600")
	
	// Write the file content
	w.Write(data)
}

// getStaticFilePath resolves the path to static files, handling different working directories
func (s *HTTPServer) getStaticFilePath(filename string) string {
	// Try different possible paths for static files
	possiblePaths := []string{
		"web/static/" + filename,           // From project root
		"../../web/static/" + filename,     // From internal/server during tests
		"../../../web/static/" + filename, // From deeper nested directories
	}
	
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	// Default to the standard path if none found
	return "web/static/" + filename
}