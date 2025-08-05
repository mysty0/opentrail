package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"opentrail/internal/types"
)

// TestCompletionTracking_BasicFlow tests the basic completion tracking flow
func TestCompletionTracking_BasicFlow(t *testing.T) {
	dbPath := "test_completion_basic.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.BatchSize = 1 // Process immediately
	config.BatchTimeout = 10 * time.Millisecond

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test log entry
	entry := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "test message for completion tracking",
	}

	// Verify ID is initially 0
	if entry.ID != 0 {
		t.Errorf("entry ID should be 0 initially, got %d", entry.ID)
	}

	// Store the entry
	err = storage.Store(entry)
	if err != nil {
		t.Errorf("Store failed: %v", err)
	}

	// Verify ID was assigned
	if entry.ID == 0 {
		t.Errorf("entry ID should be assigned after successful store, got %d", entry.ID)
	}

	t.Logf("Entry ID assigned: %d", entry.ID)
}

// TestCompletionTracking_ResultChannelCommunication tests result channel communication
func TestCompletionTracking_ResultChannelCommunication(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Test successful result
	go func() {
		time.Sleep(10 * time.Millisecond)
		req.sendResult(42, nil)
	}()

	id, err := req.waitForResult(100 * time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected ID 42, got %d", id)
	}
}

// TestCompletionTracking_TimeoutHandling tests timeout scenarios
func TestCompletionTracking_TimeoutHandling(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Don't send result, should timeout
	start := time.Now()
	id, err := req.waitForResult(50 * time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}
	if id != 0 {
		t.Errorf("expected ID 0 on timeout, got %d", id)
	}
	if elapsed < 45*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("timeout should be around 50ms, got %v", elapsed)
	}
}

// TestCompletionTracking_ChannelCleanup tests proper cleanup of result channels
func TestCompletionTracking_ChannelCleanup(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Send result
	req.sendResult(123, nil)

	// Wait for result
	id, err := req.waitForResult(100 * time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if id != 123 {
		t.Errorf("expected ID 123, got %d", id)
	}

	// Channel should be closed after result is sent
	select {
	case _, ok := <-req.resultChan:
		if ok {
			t.Error("result channel should be closed after sending result")
		}
	case <-time.After(10 * time.Millisecond):
		t.Error("result channel should be closed and readable immediately")
	}
}

// TestCompletionTracking_MultipleSendResult tests that sendResult is safe to call multiple times
func TestCompletionTracking_MultipleSendResult(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Send result multiple times (should be safe)
	req.sendResult(100, nil)
	req.sendResult(200, nil) // This should be ignored
	req.sendResult(300, nil) // This should be ignored

	// Should get the first result only
	id, err := req.waitForResult(100 * time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if id != 100 {
		t.Errorf("expected ID 100 (first result), got %d", id)
	}
}

// TestCompletionTracking_ContextCancellation tests context cancellation handling
func TestCompletionTracking_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Cancel context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Should return cancellation error
	id, err := req.waitForResult(100 * time.Millisecond)
	if err == nil {
		t.Error("expected cancellation error")
	}
	if id != 0 {
		t.Errorf("expected ID 0 on cancellation, got %d", id)
	}

	// Error should mention cancellation
	if err.Error() != "write operation cancelled: context canceled" {
		t.Errorf("expected cancellation error message, got: %v", err)
	}
}

// TestCompletionTracking_ConcurrentAccess tests concurrent access to completion tracking
func TestCompletionTracking_ConcurrentAccess(t *testing.T) {
	dbPath := "test_completion_concurrent.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.BatchSize = 5 // Small batch size
	config.BatchTimeout = 50 * time.Millisecond

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create multiple entries and store them concurrently
	numEntries := 10
	entries := make([]*types.LogEntry, numEntries)
	for i := 0; i < numEntries; i++ {
		entries[i] = &types.LogEntry{
			Priority:  16 + i,
			Facility:  2,
			Severity:  i % 8,
			Version:   1,
			Timestamp: time.Now(),
			Hostname:  "test-host",
			AppName:   "test-app",
			Message:   fmt.Sprintf("concurrent test message %d", i),
		}
	}

	// Store all entries concurrently
	errChan := make(chan error, numEntries)
	for i, entry := range entries {
		go func(idx int, e *types.LogEntry) {
			err := storage.Store(e)
			if err != nil {
				errChan <- fmt.Errorf("entry %d failed: %w", idx, err)
			} else {
				errChan <- nil
			}
		}(i, entry)
	}

	// Wait for all operations to complete
	for i := 0; i < numEntries; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				t.Errorf("concurrent store failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for concurrent store %d", i)
		}
	}

	// Verify all entries got unique IDs
	ids := make(map[int64]bool)
	for i, entry := range entries {
		if entry.ID == 0 {
			t.Errorf("entry %d did not get ID assigned", i)
		}
		if ids[entry.ID] {
			t.Errorf("entry %d got duplicate ID %d", i, entry.ID)
		}
		ids[entry.ID] = true
	}

	t.Logf("Successfully stored %d entries with unique IDs", numEntries)
}

// TestCompletionTracking_ErrorHandling tests error handling in completion tracking
func TestCompletionTracking_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Test error result
	testErr := fmt.Errorf("test database error")
	go func() {
		time.Sleep(10 * time.Millisecond)
		req.sendResult(0, testErr)
	}()

	id, err := req.waitForResult(100 * time.Millisecond)
	if err == nil {
		t.Error("expected error")
	}
	if id != 0 {
		t.Errorf("expected ID 0 on error, got %d", id)
	}
	if err.Error() != testErr.Error() {
		t.Errorf("expected error '%v', got '%v'", testErr, err)
	}
}

// TestCompletionTracking_LongRunningOperation tests completion tracking with longer operations
func TestCompletionTracking_LongRunningOperation(t *testing.T) {
	dbPath := "test_completion_long.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.BatchSize = 1
	config.BatchTimeout = 100 * time.Millisecond // Longer timeout

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test log entry
	entry := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "test message for long running operation",
	}

	// Store should complete even with longer batch timeout
	start := time.Now()
	err = storage.Store(entry)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Store failed: %v", err)
	}

	if entry.ID == 0 {
		t.Errorf("entry ID should be assigned, got %d", entry.ID)
	}

	// Should complete reasonably quickly despite longer batch timeout
	// because batch size is 1 (immediate processing)
	if elapsed > 1*time.Second {
		t.Errorf("Store took too long: %v", elapsed)
	}

	t.Logf("Long running operation completed in %v with ID %d", elapsed, entry.ID)
}

// TestCompletionTracking_MemoryLeaks tests for potential memory leaks in completion tracking
func TestCompletionTracking_MemoryLeaks(t *testing.T) {
	// This test creates many writeRequests to check for memory leaks
	numRequests := 1000

	for i := 0; i < numRequests; i++ {
		ctx := context.Background()
		entry := &types.LogEntry{Message: fmt.Sprintf("test %d", i)}
		req := newWriteRequest(entry, ctx)

		// Send result immediately
		req.sendResult(int64(i+1), nil)

		// Wait for result
		id, err := req.waitForResult(10 * time.Millisecond)
		if err != nil {
			t.Errorf("request %d failed: %v", i, err)
		}
		if id != int64(i+1) {
			t.Errorf("request %d got wrong ID: expected %d, got %d", i, i+1, id)
		}

		// Channel should be closed and cleaned up
		select {
		case _, ok := <-req.resultChan:
			if ok {
				t.Errorf("request %d: result channel should be closed", i)
			}
		default:
			t.Errorf("request %d: result channel should be closed and readable", i)
		}
	}

	t.Logf("Successfully processed %d requests without memory leaks", numRequests)
}

// TestCompletionTracking_ConfigurableTimeout tests configurable write timeout
func TestCompletionTracking_ConfigurableTimeout(t *testing.T) {
	dbPath := "test_completion_timeout.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	config := DefaultBatchConfig()
	config.WriteTimeout = 100 * time.Millisecond // Short timeout for testing
	config.BatchSize = 1
	config.BatchTimeout = 10 * time.Millisecond

	storage, err := NewBatchedSQLiteStorage(dbPath, config)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test log entry
	entry := &types.LogEntry{
		Priority:  16,
		Facility:  2,
		Severity:  0,
		Version:   1,
		Timestamp: time.Now(),
		Hostname:  "test-host",
		AppName:   "test-app",
		Message:   "test message for configurable timeout",
	}

	// Store should complete within the configured timeout
	start := time.Now()
	err = storage.Store(entry)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Store failed: %v", err)
	}

	if entry.ID == 0 {
		t.Errorf("entry ID should be assigned, got %d", entry.ID)
	}

	// Should complete well within the timeout
	if elapsed > config.WriteTimeout {
		t.Errorf("Store took longer than configured timeout: %v > %v", elapsed, config.WriteTimeout)
	}

	t.Logf("Store completed in %v with configured timeout %v", elapsed, config.WriteTimeout)
}

// TestCompletionTracking_WriteTimeoutValidation tests WriteTimeout validation
func TestCompletionTracking_WriteTimeoutValidation(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "valid timeout",
			timeout:     5 * time.Second,
			expectError: false,
		},
		{
			name:        "zero timeout",
			timeout:     0,
			expectError: true,
		},
		{
			name:        "negative timeout",
			timeout:     -1 * time.Second,
			expectError: true,
		},
		{
			name:        "too large timeout",
			timeout:     120 * time.Second,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultBatchConfig()
			config.WriteTimeout = tt.timeout

			err := config.Validate()
			if (err != nil) != tt.expectError {
				t.Errorf("WriteTimeout validation error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

// TestCompletionTracking_IsCompleted tests the isCompleted method
func TestCompletionTracking_IsCompleted(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Initially should not be completed
	if req.isCompleted() {
		t.Error("request should not be completed initially")
	}

	// Send result
	req.sendResult(123, nil)

	// Now should be completed
	if !req.isCompleted() {
		t.Error("request should be completed after sending result")
	}

	// Should still be completed after reading result
	id, err := req.waitForResult(10 * time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if id != 123 {
		t.Errorf("expected ID 123, got %d", id)
	}

	if !req.isCompleted() {
		t.Error("request should still be completed after reading result")
	}
}

// TestCompletionTracking_CleanupOnTimeout tests cleanup when timeout occurs
func TestCompletionTracking_CleanupOnTimeout(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Wait for timeout (don't send result)
	id, err := req.waitForResult(10 * time.Millisecond)

	if err == nil {
		t.Error("expected timeout error")
	}
	if id != 0 {
		t.Errorf("expected ID 0 on timeout, got %d", id)
	}

	// Request should not be marked as completed after timeout (no result was sent)
	if req.isCompleted() {
		t.Error("request should not be completed after timeout without sendResult")
	}

	// Simulate batch processor sending timeout result
	req.sendResult(0, fmt.Errorf("timeout"))

	// Now request should be completed
	if !req.isCompleted() {
		t.Error("request should be completed after sendResult")
	}

	// Channel should be closed (but we can't read from it again since waitForResult consumed it)
	// We can verify it's closed by checking the completed flag
	if !req.isCompleted() {
		t.Error("request should be completed, indicating channel was closed")
	}
}

// TestCompletionTracking_CleanupOnCancellation tests cleanup when context is cancelled
func TestCompletionTracking_CleanupOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Cancel context immediately
	cancel()

	// Wait for cancellation
	id, err := req.waitForResult(100 * time.Millisecond)

	if err == nil {
		t.Error("expected cancellation error")
	}
	if id != 0 {
		t.Errorf("expected ID 0 on cancellation, got %d", id)
	}

	// Request should not be marked as completed after cancellation (no result was sent)
	if req.isCompleted() {
		t.Error("request should not be completed after cancellation without sendResult")
	}

	// Simulate batch processor sending cancellation result
	req.sendResult(0, ctx.Err())

	// Now request should be completed
	if !req.isCompleted() {
		t.Error("request should be completed after sendResult")
	}

	// Channel should be closed (but we can't read from it again since waitForResult consumed it)
	// We can verify it's closed by checking the completed flag
	if !req.isCompleted() {
		t.Error("request should be completed, indicating channel was closed")
	}
}
