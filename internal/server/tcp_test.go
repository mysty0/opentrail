package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"opentrail/internal/interfaces"
	"opentrail/internal/types"
)

// MockLogService implements the LogService interface for testing
type MockLogService struct {
	processedLogs []string
	mutex         sync.RWMutex
	processError  error
	stats         mockServiceStats
}

type mockServiceStats struct {
	ProcessedLogs     int64
	FailedLogs        int64
	ActiveSubscribers int
	QueueSize         int
	IsRunning         bool
}

func (m *MockLogService) ProcessLog(rawMessage string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.processError != nil {
		return m.processError
	}
	
	m.processedLogs = append(m.processedLogs, rawMessage)
	m.stats.ProcessedLogs++
	return nil
}

func (m *MockLogService) ProcessLogBatch(rawMessages []string) error {
	for _, msg := range rawMessages {
		if err := m.ProcessLog(msg); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockLogService) Search(query types.SearchQuery) ([]*types.LogEntry, error) {
	return nil, nil
}

func (m *MockLogService) GetRecent(limit int) ([]*types.LogEntry, error) {
	return nil, nil
}

func (m *MockLogService) Subscribe() <-chan *types.LogEntry {
	return make(<-chan *types.LogEntry)
}

func (m *MockLogService) Unsubscribe(subscription <-chan *types.LogEntry) {}

func (m *MockLogService) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stats.IsRunning = true
	return nil
}

func (m *MockLogService) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stats.IsRunning = false
	return nil
}

func (m *MockLogService) GetStats() interfaces.ServiceStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return interfaces.ServiceStats{
		ProcessedLogs:     m.stats.ProcessedLogs,
		FailedLogs:        m.stats.FailedLogs,
		ActiveSubscribers: m.stats.ActiveSubscribers,
		QueueSize:         m.stats.QueueSize,
		IsRunning:         m.stats.IsRunning,
	}
}

func (m *MockLogService) GetProcessedLogs() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	logs := make([]string, len(m.processedLogs))
	copy(logs, m.processedLogs)
	return logs
}

func (m *MockLogService) SetProcessError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.processError = err
}

func (m *MockLogService) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.processedLogs = nil
	m.processError = nil
	m.stats = mockServiceStats{}
}

func TestTCPServer_StartStop(t *testing.T) {
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Test starting the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	
	// Verify server is running
	stats := server.GetStats()
	if !stats.IsRunning {
		t.Error("Server should be running")
	}
	
	// Test starting already running server
	err = server.Start()
	if err == nil {
		t.Error("Expected error when starting already running server")
	}
	
	// Test stopping the server
	err = server.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
	
	// Verify server is stopped
	stats = server.GetStats()
	if stats.IsRunning {
		t.Error("Server should be stopped")
	}
	
	// Test stopping already stopped server
	err = server.Stop()
	if err != nil {
		t.Error("Stopping already stopped server should not return error")
	}
}

func TestTCPServer_SingleConnection(t *testing.T) {
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	
	// Get the actual port the server is listening on
	addr := server.listener.Addr().String()
	
	// Connect to the server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	
	// Send test messages
	testMessages := []string{
		"2023-01-01T10:00:00Z|INFO|user123|Test message 1",
		"2023-01-01T10:00:01Z|ERROR|user456|Test message 2",
		"2023-01-01T10:00:02Z|DEBUG|user789|Test message 3",
	}
	
	writer := bufio.NewWriter(conn)
	for _, msg := range testMessages {
		_, err := writer.WriteString(msg + "\n")
		if err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}
	}
	writer.Flush()
	
	// Give the server time to process messages
	time.Sleep(100 * time.Millisecond)
	
	// Verify messages were processed
	processedLogs := mockService.GetProcessedLogs()
	if len(processedLogs) != len(testMessages) {
		t.Errorf("Expected %d processed logs, got %d", len(testMessages), len(processedLogs))
	}
	
	for i, expected := range testMessages {
		if i < len(processedLogs) && processedLogs[i] != expected {
			t.Errorf("Expected message %d to be %q, got %q", i, expected, processedLogs[i])
		}
	}
	
	// Verify server stats
	stats := server.GetStats()
	if stats.MessagesReceived != int64(len(testMessages)) {
		t.Errorf("Expected %d messages received, got %d", len(testMessages), stats.MessagesReceived)
	}
}

func TestTCPServer_MultipleConnections(t *testing.T) {
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	
	// Get the actual port the server is listening on
	addr := server.listener.Addr().String()
	
	// Test with multiple concurrent connections
	numConnections := 5
	messagesPerConnection := 10
	
	var wg sync.WaitGroup
	wg.Add(numConnections)
	
	for i := 0; i < numConnections; i++ {
		go func(connID int) {
			defer wg.Done()
			
			// Connect to the server
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("Connection %d failed to connect: %v", connID, err)
				return
			}
			defer conn.Close()
			
			// Send messages
			writer := bufio.NewWriter(conn)
			for j := 0; j < messagesPerConnection; j++ {
				msg := fmt.Sprintf("2023-01-01T10:00:%02dZ|INFO|conn%d|Message %d from connection %d", 
					j, connID, j, connID)
				_, err := writer.WriteString(msg + "\n")
				if err != nil {
					t.Errorf("Connection %d failed to write message %d: %v", connID, j, err)
					return
				}
			}
			writer.Flush()
			
			// Keep connection open for a bit
			time.Sleep(50 * time.Millisecond)
		}(i)
	}
	
	// Wait for all connections to complete
	wg.Wait()
	
	// Give the server time to process all messages
	time.Sleep(200 * time.Millisecond)
	
	// Verify all messages were processed
	expectedMessages := numConnections * messagesPerConnection
	processedLogs := mockService.GetProcessedLogs()
	
	if len(processedLogs) != expectedMessages {
		t.Errorf("Expected %d processed logs, got %d", expectedMessages, len(processedLogs))
	}
	
	// Verify server stats
	stats := server.GetStats()
	if stats.MessagesReceived != int64(expectedMessages) {
		t.Errorf("Expected %d messages received, got %d", expectedMessages, stats.MessagesReceived)
	}
	
	if stats.TotalConnections != int64(numConnections) {
		t.Errorf("Expected %d total connections, got %d", numConnections, stats.TotalConnections)
	}
}

func TestTCPServer_ConnectionLimit(t *testing.T) {
	maxConnections := 2
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: maxConnections,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	
	// Get the actual port the server is listening on
	addr := server.listener.Addr().String()
	
	// Create connections up to the limit
	var connections []net.Conn
	for i := 0; i < maxConnections; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to create connection %d: %v", i, err)
		}
		connections = append(connections, conn)
		
		// Send a message to establish the connection
		conn.Write([]byte("test message\n"))
	}
	
	// Give server time to process connections
	time.Sleep(100 * time.Millisecond)
	
	// Try to create one more connection (should be rejected)
	extraConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to create extra connection: %v", err)
	}
	
	// The connection should be closed by the server
	extraConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buffer := make([]byte, 1)
	_, err = extraConn.Read(buffer)
	extraConn.Close()
	
	// Should get EOF or connection reset error
	if err == nil {
		t.Error("Expected connection to be closed by server due to connection limit")
	}
	
	// Clean up connections
	for _, conn := range connections {
		conn.Close()
	}
	
	// Give server time to clean up
	time.Sleep(100 * time.Millisecond)
	
	// Verify stats show connection errors
	stats := server.GetStats()
	if stats.ConnectionErrors == 0 {
		t.Error("Expected connection errors due to connection limit")
	}
}

func TestTCPServer_EmptyLines(t *testing.T) {
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	
	// Get the actual port the server is listening on
	addr := server.listener.Addr().String()
	
	// Connect to the server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	
	// Send messages with empty lines
	messages := []string{
		"",
		"2023-01-01T10:00:00Z|INFO|user123|Test message 1",
		"",
		"",
		"2023-01-01T10:00:01Z|ERROR|user456|Test message 2",
		"",
	}
	
	writer := bufio.NewWriter(conn)
	for _, msg := range messages {
		_, err := writer.WriteString(msg + "\n")
		if err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}
	}
	writer.Flush()
	
	// Give the server time to process messages
	time.Sleep(100 * time.Millisecond)
	
	// Verify only non-empty messages were processed
	processedLogs := mockService.GetProcessedLogs()
	expectedNonEmpty := []string{
		"2023-01-01T10:00:00Z|INFO|user123|Test message 1",
		"2023-01-01T10:00:01Z|ERROR|user456|Test message 2",
	}
	
	if len(processedLogs) != len(expectedNonEmpty) {
		t.Errorf("Expected %d processed logs, got %d", len(expectedNonEmpty), len(processedLogs))
	}
	
	for i, expected := range expectedNonEmpty {
		if i < len(processedLogs) && processedLogs[i] != expected {
			t.Errorf("Expected message %d to be %q, got %q", i, expected, processedLogs[i])
		}
	}
}

func TestTCPServer_ProcessingErrors(t *testing.T) {
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	
	// Get the actual port the server is listening on
	addr := server.listener.Addr().String()
	
	// Connect to the server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	
	// Configure mock service to return errors
	mockService.SetProcessError(fmt.Errorf("processing error"))
	
	// Send a test message
	_, err = conn.Write([]byte("test message\n"))
	if err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}
	
	// Give the server time to process
	time.Sleep(100 * time.Millisecond)
	
	// Connection should still be alive despite processing error
	_, err = conn.Write([]byte("another message\n"))
	if err != nil {
		t.Error("Connection should remain alive after processing error")
	}
	
	// Reset error and send another message
	mockService.SetProcessError(nil)
	_, err = conn.Write([]byte("final message\n"))
	if err != nil {
		t.Fatalf("Failed to write final message: %v", err)
	}
	
	// Give the server time to process
	time.Sleep(100 * time.Millisecond)
	
	// Verify the messages that were processed (should include the ones after error was cleared)
	processedLogs := mockService.GetProcessedLogs()
	if len(processedLogs) < 1 {
		t.Error("Expected at least one message to be processed after error was cleared")
	}
	
	// Check that the final message is in the processed logs
	found := false
	for _, log := range processedLogs {
		if log == "final message" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'final message' to be processed, got: %v", processedLogs)
	}
}

func TestTCPServer_GracefulShutdown(t *testing.T) {
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	
	// Get the actual port the server is listening on
	addr := server.listener.Addr().String()
	
	// Create multiple connections
	var connections []net.Conn
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to create connection %d: %v", i, err)
		}
		connections = append(connections, conn)
	}
	
	// Give server time to establish connections
	time.Sleep(50 * time.Millisecond)
	
	// Verify connections are active
	stats := server.GetStats()
	if stats.ActiveConnections != 3 {
		t.Errorf("Expected 3 active connections, got %d", stats.ActiveConnections)
	}
	
	// Stop the server
	err = server.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
	
	// Verify all connections are closed
	for i, conn := range connections {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		buffer := make([]byte, 1)
		_, err := conn.Read(buffer)
		if err == nil {
			t.Errorf("Connection %d should be closed after server shutdown", i)
		}
		conn.Close()
	}
	
	// Verify server stats
	stats = server.GetStats()
	if stats.IsRunning {
		t.Error("Server should not be running after shutdown")
	}
	if stats.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections after shutdown, got %d", stats.ActiveConnections)
	}
}

func TestTCPServer_LongMessages(t *testing.T) {
	config := &types.Config{
		TCPPort:        0, // Use random port
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewTCPServer(config, mockService)
	
	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	
	// Get the actual port the server is listening on
	addr := server.listener.Addr().String()
	
	// Connect to the server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	
	// Create a long message (larger than buffer size)
	longMessage := "2023-01-01T10:00:00Z|INFO|user123|" + strings.Repeat("A", 8192)
	
	// Send the long message
	_, err = conn.Write([]byte(longMessage + "\n"))
	if err != nil {
		t.Fatalf("Failed to write long message: %v", err)
	}
	
	// Give the server time to process
	time.Sleep(100 * time.Millisecond)
	
	// Verify the long message was processed
	processedLogs := mockService.GetProcessedLogs()
	if len(processedLogs) != 1 {
		t.Errorf("Expected 1 processed log, got %d", len(processedLogs))
	}
	
	if len(processedLogs) > 0 && processedLogs[0] != longMessage {
		t.Error("Long message was not processed correctly")
	}
}