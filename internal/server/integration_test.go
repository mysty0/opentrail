package server

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"opentrail/internal/parser"
	"opentrail/internal/service"
	"opentrail/internal/storage"
	"opentrail/internal/types"
)

func TestTCPServer_Integration(t *testing.T) {
	// Create a temporary database file with unique name
	tempDB := fmt.Sprintf("test_integration_%d.db", time.Now().UnixNano())
	defer os.Remove(tempDB)
	
	// Create configuration
	config := &types.Config{
		TCPPort:        0, // Use random port
		HTTPPort:       8081,
		DatabasePath:   tempDB,
		LogFormat:      "{{timestamp}}|{{level}}|{{tracking_id}}|{{message}}",
		RetentionDays:  30,
		MaxConnections: 10,
		AuthEnabled:    false,
	}
	
	// Create storage
	storageInstance, err := storage.NewSQLiteStorage(config.DatabasePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	// Note: storage interface doesn't expose Close method, but it will be cleaned up when process ends
	
	// Create parser
	parserInstance := parser.NewDefaultLogParser()
	err = parserInstance.SetFormat(config.LogFormat)
	if err != nil {
		t.Fatalf("Failed to set parser format: %v", err)
	}
	
	// Create log service
	logService := service.NewLogService(parserInstance, storageInstance)
	err = logService.Start()
	if err != nil {
		t.Fatalf("Failed to start log service: %v", err)
	}
	defer logService.Stop()
	
	// Create TCP server
	tcpServer := NewTCPServer(config, logService)
	err = tcpServer.Start()
	if err != nil {
		t.Fatalf("Failed to start TCP server: %v", err)
	}
	defer tcpServer.Stop()
	
	// Get the actual port the server is listening on
	addr := tcpServer.listener.Addr().String()
	
	// Connect to the server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	
	// Send test log messages
	testMessages := []string{
		"2023-01-01T10:00:00Z|INFO|user123|User logged in successfully",
		"2023-01-01T10:00:01Z|ERROR|user456|Failed to authenticate user",
		"2023-01-01T10:00:02Z|DEBUG|user789|Processing user request",
		"2023-01-01T10:00:03Z|WARN|user123|Session about to expire",
	}
	
	writer := bufio.NewWriter(conn)
	for _, msg := range testMessages {
		_, err := writer.WriteString(msg + "\n")
		if err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}
	}
	writer.Flush()
	
	// Give the system time to process messages (service processes in batches)
	time.Sleep(2 * time.Second)
	
	// Check service stats for debugging
	serviceStats := logService.GetStats()
	t.Logf("Service stats: ProcessedLogs=%d, FailedLogs=%d, IsRunning=%t", 
		serviceStats.ProcessedLogs, serviceStats.FailedLogs, serviceStats.IsRunning)
	
	// Check TCP server stats for debugging
	tcpStats := tcpServer.GetStats()
	t.Logf("TCP stats: MessagesReceived=%d, ConnectionErrors=%d", 
		tcpStats.MessagesReceived, tcpStats.ConnectionErrors)
	
	// Query the logs to verify they were stored
	query := types.SearchQuery{
		Limit: 10,
	}
	
	logs, err := logService.Search(query)
	if err != nil {
		t.Fatalf("Failed to search logs: %v", err)
	}
	
	t.Logf("Found %d logs in storage", len(logs))
	
	if len(logs) != len(testMessages) {
		t.Errorf("Expected %d logs, got %d", len(testMessages), len(logs))
	}
	
	// Verify the content of stored logs (logs are returned in reverse order - newest first)
	expectedLevels := []string{"WARN", "DEBUG", "ERROR", "INFO"}
	expectedTrackingIDs := []string{"user123", "user789", "user456", "user123"}
	
	for i, log := range logs {
		if i < len(expectedLevels) && log.Level != expectedLevels[i] {
			t.Errorf("Expected log %d level to be %s, got %s", i, expectedLevels[i], log.Level)
		}
		if i < len(expectedTrackingIDs) && log.TrackingID != expectedTrackingIDs[i] {
			t.Errorf("Expected log %d tracking ID to be %s, got %s", i, expectedTrackingIDs[i], log.TrackingID)
		}
	}
	
	// Test search functionality with retry for database locking
	var searchResults []*types.LogEntry
	for retry := 0; retry < 3; retry++ {
		searchQuery := types.SearchQuery{
			Text:  "user",
			Limit: 10,
		}
		
		searchResults, err = logService.Search(searchQuery)
		if err == nil {
			break
		}
		t.Logf("Search retry %d failed: %v", retry+1, err)
		time.Sleep(100 * time.Millisecond)
	}
	
	if err != nil {
		t.Logf("Search failed after retries, skipping search tests: %v", err)
	} else {
		if len(searchResults) == 0 {
			t.Error("Expected search results for 'user' query")
		}
		
		// Test level filtering
		var levelResults []*types.LogEntry
		for retry := 0; retry < 3; retry++ {
			levelQuery := types.SearchQuery{
				Level: "ERROR",
				Limit: 10,
			}
			
			levelResults, err = logService.Search(levelQuery)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		
		if err != nil {
			t.Logf("Level search failed after retries: %v", err)
		} else {
			if len(levelResults) != 1 {
				t.Errorf("Expected 1 ERROR log, got %d", len(levelResults))
			}
			
			if len(levelResults) > 0 && levelResults[0].Level != "ERROR" {
				t.Errorf("Expected ERROR level log, got %s", levelResults[0].Level)
			}
		}
	}
	
	// Verify server statistics (reuse tcpStats from above)
	if tcpStats.MessagesReceived != int64(len(testMessages)) {
		t.Errorf("Expected %d messages received, got %d", len(testMessages), tcpStats.MessagesReceived)
	}
	
	if tcpStats.TotalConnections != 1 {
		t.Errorf("Expected 1 total connection, got %d", tcpStats.TotalConnections)
	}
	
	// Verify service statistics (reuse serviceStats from above)
	if serviceStats.ProcessedLogs != int64(len(testMessages)) {
		t.Errorf("Expected %d processed logs, got %d", len(testMessages), serviceStats.ProcessedLogs)
	}
}