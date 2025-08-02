package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"

	"github.com/gorilla/websocket"
)

const (
	// DefaultWebSocketReadTimeout is the default timeout for reading WebSocket messages
	DefaultWebSocketReadTimeout = 30 * time.Second
	// DefaultWebSocketWriteTimeout is the default timeout for writing WebSocket messages
	DefaultWebSocketWriteTimeout = 10 * time.Second
	// WebSocketBufferSize is the buffer size for WebSocket messages
	WebSocketBufferSize = 4096
)

// WebSocketServer implements a WebSocket server for log ingestion
type WebSocketServer struct {
	config     *types.Config
	logService interfaces.LogService
	upgrader   websocket.Upgrader
	server     *http.Server

	// Connection management
	connections    map[*websocket.Conn]bool
	connectionsMux sync.RWMutex
	activeConns    int64

	// Server lifecycle
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isRunning  bool
	runningMux sync.RWMutex

	// Statistics
	stats      WebSocketServerStats
	statsMutex sync.RWMutex
}

// WebSocketServerStats represents statistics about the WebSocket server
type WebSocketServerStats struct {
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	MessagesReceived  int64 `json:"messages_received"`
	ConnectionErrors  int64 `json:"connection_errors"`
	IsRunning         bool  `json:"is_running"`
}

// NewWebSocketServer creates a new WebSocket server instance
func NewWebSocketServer(config *types.Config, logService interfaces.LogService) *WebSocketServer {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketServer{
		config:      config,
		logService:  logService,
		connections: make(map[*websocket.Conn]bool),
		ctx:         ctx,
		cancel:      cancel,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  WebSocketBufferSize,
			WriteBufferSize: WebSocketBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for simplicity
				return true
			},
		},
		stats: WebSocketServerStats{
			IsRunning: false,
		},
	}
}

// Start starts the WebSocket server
func (s *WebSocketServer) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if s.isRunning {
		return fmt.Errorf("WebSocket server is already running")
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/logs", s.handleWebSocket)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.WebSocketPort),
		Handler: mux,
	}

	s.isRunning = true

	// Update stats
	s.updateStats(func(stats *WebSocketServerStats) {
		stats.IsRunning = true
	})

	// Start server in goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Printf("WebSocket server starting on port %d", s.config.WebSocketPort)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
			s.updateStats(func(stats *WebSocketServerStats) {
				stats.ConnectionErrors++
			})
		}
		log.Printf("WebSocket server stopped")
	}()

	return nil
}

// Stop gracefully stops the WebSocket server
func (s *WebSocketServer) Stop() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if !s.isRunning {
		return nil
	}

	// Cancel context to signal shutdown
	s.cancel()

	// Shutdown HTTP server
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(ctx); err != nil {
			log.Printf("WebSocket server shutdown error: %v", err)
		}
	}

	// Close all active connections
	s.connectionsMux.Lock()
	for conn := range s.connections {
		conn.Close()
	}
	s.connectionsMux.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.isRunning = false

	// Update stats
	s.updateStats(func(stats *WebSocketServerStats) {
		stats.IsRunning = false
		stats.ActiveConnections = 0
	})

	log.Printf("WebSocket server stopped")
	return nil
}

// GetStats returns server statistics
func (s *WebSocketServer) GetStats() WebSocketServerStats {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()

	stats := s.stats
	stats.ActiveConnections = atomic.LoadInt64(&s.activeConns)

	return stats
}

// handleWebSocket handles WebSocket connections
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check connection limit
	currentConns := atomic.LoadInt64(&s.activeConns)
	if currentConns >= int64(s.config.MaxConnections) {
		log.Printf("Connection limit reached (%d), rejecting WebSocket connection from %s",
			s.config.MaxConnections, r.RemoteAddr)
		http.Error(w, "Connection limit reached", http.StatusServiceUnavailable)
		return
	}

	// Upgrade connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error from %s: %v", r.RemoteAddr, err)
		s.updateStats(func(stats *WebSocketServerStats) {
			stats.ConnectionErrors++
		})
		return
	}

	// Handle the connection
	s.wg.Add(1)
	go s.handleConnection(conn)
}

// handleConnection handles a single WebSocket connection
func (s *WebSocketServer) handleConnection(conn *websocket.Conn) {
	defer s.wg.Done()
	defer s.removeConnection(conn)

	// Add connection to tracking
	s.addConnection(conn)

	// Set connection deadlines
	conn.SetReadDeadline(time.Now().Add(DefaultWebSocketReadTimeout))

	log.Printf("New WebSocket connection from %s", conn.RemoteAddr())

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Read a message
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error from %s: %v", conn.RemoteAddr(), err)
				}
				return
			}

			// Only handle text messages
			if messageType != websocket.TextMessage {
				continue
			}

			// Convert message to string
			logMessage := string(message)

			// Skip empty messages
			if len(logMessage) == 0 {
				continue
			}

			// Update read deadline
			conn.SetReadDeadline(time.Now().Add(DefaultWebSocketReadTimeout))

			// Process the log message
			if err := s.logService.ProcessLog(logMessage); err != nil {
				log.Printf("Error processing WebSocket log from %s: %v", conn.RemoteAddr(), err)
				// Don't close connection on processing errors, just log and continue
			} else {
				s.updateStats(func(stats *WebSocketServerStats) {
					stats.MessagesReceived++
				})
			}
		}
	}
}

// addConnection adds a connection to the tracking map
func (s *WebSocketServer) addConnection(conn *websocket.Conn) {
	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()

	s.connections[conn] = true
	atomic.AddInt64(&s.activeConns, 1)

	s.updateStats(func(stats *WebSocketServerStats) {
		stats.TotalConnections++
	})
}

// removeConnection removes a connection from the tracking map
func (s *WebSocketServer) removeConnection(conn *websocket.Conn) {
	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()

	if _, exists := s.connections[conn]; exists {
		delete(s.connections, conn)
		atomic.AddInt64(&s.activeConns, -1)
		conn.Close()

		log.Printf("WebSocket connection from %s closed", conn.RemoteAddr())
	}
}

// updateStats safely updates the server statistics
func (s *WebSocketServer) updateStats(updateFunc func(*WebSocketServerStats)) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()
	updateFunc(&s.stats)
}