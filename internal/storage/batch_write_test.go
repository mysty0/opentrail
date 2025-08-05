package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"opentrail/internal/types"

	_ "modernc.org/sqlite"
)

func TestBatchWriteOperations(t *testing.T) {
	batchedStorage := createTestStorage(t, "test_batch_write.db")
	defer batchedStorage.Close()

	t.Run("ExecuteBatchWrite_Success", func(t *testing.T) {
		storage := createTestStorage(t, "test_batch_success.db")

		// Create test requests
		requests := createTestWriteRequests(3)

		// Execute batch write
		err := storage.executeBatchWrite(requests)
		if err != nil {
			t.Fatalf("Batch write failed: %v", err)
		}

		// Verify all requests got results
		for i, req := range requests {
			select {
			case result := <-req.resultChan:
				if result.err != nil {
					t.Errorf("Request %d failed: %v", i, result.err)
				}
				if result.id <= 0 {
					t.Errorf("Request %d got invalid ID: %d", i, result.id)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("Request %d timed out waiting for result", i)
			}
		}

		// Verify data was actually written to database
		verifyDataInDatabase(t, storage.db, requests)
	})

	t.Run("ExecuteBatchWrite_WithStructuredData", func(t *testing.T) {
		storage := createTestStorage(t, "test_batch_structured.db")

		// Create test requests with structured data
		requests := createTestWriteRequestsWithStructuredData(2)

		// Execute batch write
		err := storage.executeBatchWrite(requests)
		if err != nil {
			t.Fatalf("Batch write failed: %v", err)
		}

		// Verify all requests got results
		for i, req := range requests {
			select {
			case result := <-req.resultChan:
				if result.err != nil {
					t.Errorf("Request %d failed: %v", i, result.err)
				}
				if result.id <= 0 {
					t.Errorf("Request %d got invalid ID: %d", i, result.id)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("Request %d timed out waiting for result", i)
			}
		}

		// Verify structured data was stored correctly
		verifyStructuredDataInDatabase(t, storage.db, requests)
	})

	t.Run("ExecuteIndividualWrite_Success", func(t *testing.T) {
		storage := createTestStorage(t, "test_individual.db")

		// Create test request
		req := createTestWriteRequests(1)[0]

		// Execute individual write
		err := storage.executeIndividualWrite(req)
		if err != nil {
			t.Fatalf("Individual write failed: %v", err)
		}

		// Verify request got result
		select {
		case result := <-req.resultChan:
			if result.err != nil {
				t.Errorf("Request failed: %v", result.err)
			}
			if result.id <= 0 {
				t.Errorf("Request got invalid ID: %d", result.id)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Request timed out waiting for result")
		}
	})

	t.Run("RetryIndividualWrites_AfterBatchFailure", func(t *testing.T) {
		storage := createTestStorage(t, "test_retry.db")

		// Create test requests
		requests := createTestWriteRequests(3)

		// Simulate batch failure by calling retry directly
		batchErr := fmt.Errorf("simulated batch failure")
		storage.retryIndividualWrites(requests, batchErr)

		// Verify all requests got results
		for i, req := range requests {
			select {
			case result := <-req.resultChan:
				if result.err != nil {
					t.Errorf("Request %d failed: %v", i, result.err)
				}
				if result.id <= 0 {
					t.Errorf("Request %d got invalid ID: %d", i, result.id)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("Request %d timed out waiting for result", i)
			}
		}
	})

	t.Run("BatchWrite_WithCancelledContext", func(t *testing.T) {
		storage := createTestStorage(t, "test_cancelled.db")

		// Create test requests with cancelled context
		requests := make([]*writeRequest, 2)
		for i := 0; i < 2; i++ {
			entry := createTestLogEntry(i)
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately
			requests[i] = newWriteRequest(entry, ctx)
		}

		// Execute batch write
		storage.executeBatchWrite(requests)

		// Verify cancelled requests got cancellation errors
		for i, req := range requests {
			select {
			case result := <-req.resultChan:
				if result.err == nil {
					t.Errorf("Request %d should have failed with cancellation error", i)
				}
				if result.id != 0 {
					t.Errorf("Request %d should have ID 0 for error, got %d", i, result.id)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("Request %d timed out waiting for result", i)
			}
		}
	})

	t.Run("ConvertStructuredDataToJSON", func(t *testing.T) {
		tests := []struct {
			name     string
			input    map[string]interface{}
			expected string
			hasError bool
		}{
			{
				name:     "nil data",
				input:    nil,
				expected: "",
				hasError: false,
			},
			{
				name:     "empty data",
				input:    map[string]interface{}{},
				expected: "",
				hasError: false,
			},
			{
				name: "simple data",
				input: map[string]interface{}{
					"key1": "value1",
					"key2": 42,
				},
				expected: `{"key1":"value1","key2":42}`,
				hasError: false,
			},
			{
				name: "nested data",
				input: map[string]interface{}{
					"outer": map[string]interface{}{
						"inner": "value",
					},
				},
				expected: `{"outer":{"inner":"value"}}`,
				hasError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := batchedStorage.convertStructuredDataToJSON(tt.input)

				if tt.hasError && err == nil {
					t.Errorf("Expected error but got none")
				}
				if !tt.hasError && err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tt.expected != "" {
					// Parse both to compare JSON content
					var expectedJSON, resultJSON interface{}
					json.Unmarshal([]byte(tt.expected), &expectedJSON)
					json.Unmarshal([]byte(result), &resultJSON)

					expectedBytes, _ := json.Marshal(expectedJSON)
					resultBytes, _ := json.Marshal(resultJSON)

					if string(expectedBytes) != string(resultBytes) {
						t.Errorf("Expected %s, got %s", tt.expected, result)
					}
				} else if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			})
		}
	})
}

func TestBatchWriteErrorHandling(t *testing.T) {
	t.Run("BatchWrite_DatabaseClosed", func(t *testing.T) {
		storage := createTestStorage(t, "test_error.db")

		// Close the database to simulate connection failure
		storage.db.Close()

		// Create test requests
		requests := createTestWriteRequests(2)

		// Execute batch write (should fail)
		err := storage.executeBatchWrite(requests)
		if err == nil {
			t.Fatalf("Expected batch write to fail with closed database")
		}

		// Verify all requests got errors through individual retry
		for i, req := range requests {
			select {
			case result := <-req.resultChan:
				if result.err == nil {
					t.Errorf("Request %d should have failed", i)
				}
				if result.id != 0 {
					t.Errorf("Request %d should have ID 0 for error, got %d", i, result.id)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("Request %d timed out waiting for result", i)
			}
		}
	})

	t.Run("ConvertStructuredDataToJSON", func(t *testing.T) {
		storage := createTestStorage(t, "test_json.db")

		tests := []struct {
			name     string
			input    map[string]interface{}
			expected string
			hasError bool
		}{
			{
				name:     "nil data",
				input:    nil,
				expected: "",
				hasError: false,
			},
			{
				name:     "empty data",
				input:    map[string]interface{}{},
				expected: "",
				hasError: false,
			},
			{
				name: "simple data",
				input: map[string]interface{}{
					"key1": "value1",
					"key2": 42,
				},
				expected: `{"key1":"value1","key2":42}`,
				hasError: false,
			},
			{
				name: "nested data",
				input: map[string]interface{}{
					"outer": map[string]interface{}{
						"inner": "value",
					},
				},
				expected: `{"outer":{"inner":"value"}}`,
				hasError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := storage.convertStructuredDataToJSON(tt.input)

				if tt.hasError && err == nil {
					t.Errorf("Expected error but got none")
				}
				if !tt.hasError && err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tt.expected != "" {
					// Parse both to compare JSON content
					var expectedJSON, resultJSON interface{}
					json.Unmarshal([]byte(tt.expected), &expectedJSON)
					json.Unmarshal([]byte(result), &resultJSON)

					expectedBytes, _ := json.Marshal(expectedJSON)
					resultBytes, _ := json.Marshal(resultJSON)

					if string(expectedBytes) != string(resultBytes) {
						t.Errorf("Expected %s, got %s", tt.expected, result)
					}
				} else if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			})
		}
	})
}

// Helper functions

func createTestStorage(t *testing.T, dbFile string) *BatchedSQLiteStorage {
	// Remove any existing database file
	os.Remove(dbFile)

	// Create storage with small batch size for testing
	config := BatchConfig{
		BatchSize:    3,
		BatchTimeout: 100 * time.Millisecond,
		QueueSize:    100,
		WALEnabled:   &[]bool{true}[0],
	}

	storage, err := NewBatchedSQLiteStorage(dbFile, config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Clean up database file when test completes
	t.Cleanup(func() {
		storage.Close()
		os.Remove(dbFile)
	})

	return storage.(*BatchedSQLiteStorage)
}

func createTestLogEntry(index int) *types.LogEntry {
	return &types.LogEntry{
		Priority:       16 + index, // facility 2, severity varies
		Facility:       2,
		Severity:       index,
		Version:        1,
		Timestamp:      time.Now().Add(time.Duration(index) * time.Second),
		Hostname:       fmt.Sprintf("host%d", index),
		AppName:        fmt.Sprintf("app%d", index),
		ProcID:         fmt.Sprintf("proc%d", index),
		MsgID:          fmt.Sprintf("msg%d", index),
		StructuredData: nil,
		Message:        fmt.Sprintf("Test message %d", index),
	}
}

func createTestWriteRequests(count int) []*writeRequest {
	requests := make([]*writeRequest, count)
	for i := 0; i < count; i++ {
		entry := createTestLogEntry(i)
		ctx := context.Background()
		requests[i] = newWriteRequest(entry, ctx)
	}
	return requests
}

func createTestWriteRequestsWithStructuredData(count int) []*writeRequest {
	requests := make([]*writeRequest, count)
	for i := 0; i < count; i++ {
		entry := createTestLogEntry(i)
		entry.StructuredData = map[string]interface{}{
			"test_key":   fmt.Sprintf("test_value_%d", i),
			"test_num":   i,
			"test_bool":  i%2 == 0,
			"test_array": []string{fmt.Sprintf("item_%d", i)},
		}
		ctx := context.Background()
		requests[i] = newWriteRequest(entry, ctx)
	}
	return requests
}

func verifyDataInDatabase(t *testing.T, db *sql.DB, requests []*writeRequest) {
	for i, req := range requests {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM logs WHERE message = ?", req.entry.Message).Scan(&count)
		if err != nil {
			t.Errorf("Failed to query database for request %d: %v", i, err)
			continue
		}
		if count != 1 {
			t.Errorf("Expected 1 record for request %d, got %d", i, count)
		}
	}
}

func verifyStructuredDataInDatabase(t *testing.T, db *sql.DB, requests []*writeRequest) {
	for i, req := range requests {
		var structuredDataJSON string
		err := db.QueryRow("SELECT structured_data FROM logs WHERE message = ?", req.entry.Message).Scan(&structuredDataJSON)
		if err != nil {
			t.Errorf("Failed to query structured data for request %d: %v", i, err)
			continue
		}

		// If stored data is empty, check if original was also empty
		if structuredDataJSON == "" {
			if req.entry.StructuredData != nil && len(req.entry.StructuredData) > 0 {
				t.Errorf("Request %d: expected non-empty structured data but got empty string", i)
			}
			continue
		}

		// Parse stored JSON
		var storedData map[string]interface{}
		if err := json.Unmarshal([]byte(structuredDataJSON), &storedData); err != nil {
			t.Errorf("Failed to parse stored JSON for request %d: %v", i, err)
			continue
		}

		// Compare with original data
		originalJSON, _ := json.Marshal(req.entry.StructuredData)
		storedJSON, _ := json.Marshal(storedData)

		if string(originalJSON) != string(storedJSON) {
			t.Errorf("Structured data mismatch for request %d: expected %s, got %s",
				i, string(originalJSON), string(storedJSON))
		}
	}
}
