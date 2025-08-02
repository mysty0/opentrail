package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"opentrail/internal/types"
)

func TestNewWebSocketServer(t *testing.T) {
	config := &types.Config{
		WebSocketPort:  8081,
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	
	server := NewWebSocketServer(config, mockService)
	if server == nil {
		t.Fatal("NewWebSocketServer should not return nil")
	}
	
	if server.config != config {
		t.Error("Config should be set correctly")
	}
	
	if server.logService != mockService {
		t.Error("Log service should be set correctly")
	}
	
	if server.upgrader.ReadBufferSize != WebSocketBufferSize {
		t.Error("Read buffer size should be set correctly")
	}
	
	if server.upgrader.WriteBufferSize != WebSocketBufferSize {
		t.Error("Write buffer size should be set correctly")
	}
}

func TestWebSocketServerStartStop(t *testing.T) {
	config := &types.Config{
		WebSocketPort:  8082,
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewWebSocketServer(config, mockService)
	
	// Test start
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start WebSocket server: %v", err)
	}
	
	// Verify server is running
	stats := server.GetStats()
	if !stats.IsRunning {
		t.Error("Server should be running after start")
	}
	
	// Test double start
	err = server.Start()
	if err == nil {
		t.Error("Double start should return error")
	}
	
	// Test stop
	err = server.Stop()
	if err != nil {
		t.Fatalf("Failed to stop WebSocket server: %v", err)
	}
	
	// Verify server is stopped
	stats = server.GetStats()
	if stats.IsRunning {
		t.Error("Server should not be running after stop")
	}
	
	// Test stop when already stopped
	err = server.Stop()
	if err != nil {
		t.Error("Stop on already stopped server should not return error")
	}
}

func TestWebSocketServerConnectionHandling(t *testing.T) {
	config := &types.Config{
		WebSocketPort:  8083,
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewWebSocketServer(config, mockService)
	
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start WebSocket server: %v", err)
	}
	defer server.Stop()
	
	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer ts.Close()
	
	// Convert http URL to ws URL
	wsURL := "ws" + ts.URL[4:] + "/ws/logs"
	
	// Create WebSocket client
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer conn.Close()
	
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("Expected status 101, got %d", resp.StatusCode)
	}
	
	// Send a test message
	testMessage := "test log message"
	err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}
	
	// Wait for message to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Verify message was processed
	logs := mockService.GetProcessedLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}
	
	if logs[0] != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, logs[0])
	}
	
	// Verify connection statistics
	stats := server.GetStats()
	if stats.TotalConnections != 1 {
		t.Errorf("Expected 1 total connection, got %d", stats.TotalConnections)
	}
	
	if stats.MessagesReceived != 1 {
		t.Errorf("Expected 1 message received, got %d", stats.MessagesReceived)
	}
}

func TestWebSocketServerConnectionLimit(t *testing.T) {
	config := &types.Config{
		WebSocketPort:  8084,
		MaxConnections: 1,
	}
	
	mockService := &MockLogService{}
	server := NewWebSocketServer(config, mockService)
	
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start WebSocket server: %v", err)
	}
	defer server.Stop()
	
	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer ts.Close()
	
	// Convert http URL to ws URL
	wsURL := "ws" + ts.URL[4:] + "/ws/logs"
	
	// Create first WebSocket client (should succeed)
	dialer := websocket.DefaultDialer
	conn1, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect first client: %v", err)
	}
	defer conn1.Close()
	
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("Expected status 101 for first client, got %d", resp.StatusCode)
	}
	
	// Create second WebSocket client (should fail)
	conn2, resp, err := dialer.Dial(wsURL, nil)
	if err == nil {
		conn2.Close()
		t.Error("Second client should have been rejected")
	}
	
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 for rejected client, got %d", resp.StatusCode)
	}
	
	// Verify connection statistics
	stats := server.GetStats()
	if stats.TotalConnections != 1 {
		t.Errorf("Expected 1 total connection, got %d", stats.TotalConnections)
	}
	
	// Note: Connection limit rejections happen before connection establishment
	// so they don't count as ConnectionErrors in the current implementation
}

func TestWebSocketServerGracefulShutdown(t *testing.T) {
	config := &types.Config{
		WebSocketPort:  8085,
		MaxConnections: 10,
	}
	
	mockService := &MockLogService{}
	server := NewWebSocketServer(config, mockService)
	
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start WebSocket server: %v", err)
	}
	
	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer ts.Close()
	
	// Convert http URL to ws URL
	wsURL := "ws" + ts.URL[4:] + "/ws/logs"
	
	// Create WebSocket client
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer conn.Close()
	
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("Expected status 101, got %d", resp.StatusCode)
	}
	
	// Verify connection is active
	stats := server.GetStats()
	if stats.ActiveConnections != 1 {
		t.Errorf("Expected 1 active connection, got %d", stats.ActiveConnections)
	}
	
	// Shutdown server
	err = server.Stop()
	if err != nil {
		t.Fatalf("Failed to stop WebSocket server: %v", err)
	}
	
	// Verify all connections are closed
	stats = server.GetStats()
	if stats.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections after shutdown, got %d", stats.ActiveConnections)
	}
}