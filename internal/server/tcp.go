package server

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

const (
	// DefaultReadTimeout is the default timeout for reading from connections
	DefaultReadTimeout = 30 * time.Second
	// DefaultWriteTimeout is the default timeout for writing to connections
	DefaultWriteTimeout = 10 * time.Second
	// ConnectionBufferSize is the buffer size for reading from connections
	ConnectionBufferSize = 4096
)

// TCPServer implements a TCP server for log ingestion
type TCPServer struct {
	config      *types.Config
	logService  interfaces.LogService
	listener    net.Listener
	
	// Connection management
	connections    map[net.Conn]bool
	connectionsMux sync.RWMutex
	activeConns    int64
	
	// Server lifecycle
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isRunning  bool
	runningMux sync.RWMutex
	
	// Statistics
	stats      TCPServerStats
	statsMutex sync.RWMutex
}

// TCPServerStats represents statistics about the TCP server
type TCPServerStats struct {
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	MessagesReceived  int64 `json:"messages_received"`
	ConnectionErrors  int64 `json:"connection_errors"`
	IsRunning         bool  `json:"is_running"`
}

// NewTCPServer creates a new TCP server instance
func NewTCPServer(config *types.Config, logService interfaces.LogService) *TCPServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &TCPServer{
		config:      config,
		logService:  logService,
		connections: make(map[net.Conn]bool),
		ctx:         ctx,
		cancel:      cancel,
		stats: TCPServerStats{
			IsRunning: false,
		},
	}
}

// Start starts the TCP server
func (s *TCPServer) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()
	
	if s.isRunning {
		return fmt.Errorf("TCP server is already running")
	}
	
	// Create listener
	addr := fmt.Sprintf(":%d", s.config.TCPPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	
	s.listener = listener
	s.isRunning = true
	
	// Update stats
	s.updateStats(func(stats *TCPServerStats) {
		stats.IsRunning = true
	})
	
	// Start accepting connections
	s.wg.Add(1)
	go s.acceptConnections()
	
	log.Printf("TCP server started on port %d", s.config.TCPPort)
	return nil
}

// Stop gracefully stops the TCP server
func (s *TCPServer) Stop() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()
	
	if !s.isRunning {
		return nil
	}
	
	// Cancel context to signal shutdown
	s.cancel()
	
	// Close the listener to stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
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
	s.updateStats(func(stats *TCPServerStats) {
		stats.IsRunning = false
		stats.ActiveConnections = 0
	})
	
	log.Printf("TCP server stopped")
	return nil
}

// GetStats returns server statistics
func (s *TCPServer) GetStats() TCPServerStats {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()
	
	stats := s.stats
	stats.ActiveConnections = atomic.LoadInt64(&s.activeConns)
	
	return stats
}

// acceptConnections runs in a goroutine to accept incoming connections
func (s *TCPServer) acceptConnections() {
	defer s.wg.Done()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Set a deadline for Accept to make it interruptible
			if tcpListener, ok := s.listener.(*net.TCPListener); ok {
				tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
			}
			
			conn, err := s.listener.Accept()
			if err != nil {
				// Check if this is due to context cancellation
				select {
				case <-s.ctx.Done():
					return
				default:
					// Check if this is a timeout (expected during shutdown)
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					
					log.Printf("Error accepting connection: %v", err)
					s.updateStats(func(stats *TCPServerStats) {
						stats.ConnectionErrors++
					})
					continue
				}
			}
			
			// Check connection limit
			currentConns := atomic.LoadInt64(&s.activeConns)
			if currentConns >= int64(s.config.MaxConnections) {
				log.Printf("Connection limit reached (%d), rejecting connection from %s", 
					s.config.MaxConnections, conn.RemoteAddr())
				conn.Close()
				s.updateStats(func(stats *TCPServerStats) {
					stats.ConnectionErrors++
				})
				continue
			}
			
			// Handle the connection
			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// handleConnection handles a single TCP connection
func (s *TCPServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer s.removeConnection(conn)
	
	// Add connection to tracking
	s.addConnection(conn)
	
	// Set up connection timeouts
	conn.SetReadDeadline(time.Now().Add(DefaultReadTimeout))
	
	// Create a buffered reader for efficient line reading
	reader := bufio.NewReaderSize(conn, ConnectionBufferSize)
	
	log.Printf("New connection from %s", conn.RemoteAddr())
	
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Read a line (newline-delimited message)
			line, err := reader.ReadString('\n')
			if err != nil {
				if err.Error() != "EOF" {
					log.Printf("Error reading from connection %s: %v", conn.RemoteAddr(), err)
				}
				return
			}
			
			// Remove the trailing newline
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			
			// Skip empty lines
			if len(line) == 0 {
				continue
			}
			
			// Update read deadline
			conn.SetReadDeadline(time.Now().Add(DefaultReadTimeout))
			
			// Process the log message
			if err := s.logService.ProcessLog(line); err != nil {
				log.Printf("Error processing log from %s: %v", conn.RemoteAddr(), err)
				// Don't close connection on processing errors, just log and continue
			} else {
				s.updateStats(func(stats *TCPServerStats) {
					stats.MessagesReceived++
				})
			}
		}
	}
}

// addConnection adds a connection to the tracking map
func (s *TCPServer) addConnection(conn net.Conn) {
	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()
	
	s.connections[conn] = true
	atomic.AddInt64(&s.activeConns, 1)
	
	s.updateStats(func(stats *TCPServerStats) {
		stats.TotalConnections++
	})
}

// removeConnection removes a connection from the tracking map
func (s *TCPServer) removeConnection(conn net.Conn) {
	s.connectionsMux.Lock()
	defer s.connectionsMux.Unlock()
	
	if _, exists := s.connections[conn]; exists {
		delete(s.connections, conn)
		atomic.AddInt64(&s.activeConns, -1)
		conn.Close()
		
		log.Printf("Connection from %s closed", conn.RemoteAddr())
	}
}

// updateStats safely updates the server statistics
func (s *TCPServer) updateStats(updateFunc func(*TCPServerStats)) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()
	updateFunc(&s.stats)
}