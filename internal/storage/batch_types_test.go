package storage

import (
	"context"
	"testing"
	"time"

	"opentrail/internal/types"
)

func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	if config.BatchSize != 100 {
		t.Errorf("Expected BatchSize 100, got %d", config.BatchSize)
	}

	if config.BatchTimeout != 100*time.Millisecond {
		t.Errorf("Expected BatchTimeout 100ms, got %v", config.BatchTimeout)
	}

	if config.QueueSize != 10000 {
		t.Errorf("Expected QueueSize 10000, got %d", config.QueueSize)
	}

	if config.WALEnabled == nil || !*config.WALEnabled {
		t.Error("Expected WALEnabled to be true")
	}
}

func TestBatchConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  BatchConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultBatchConfig(),
			wantErr: false,
		},
		{
			name: "zero batch size",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    0,
					BatchTimeout: 100 * time.Millisecond,
					QueueSize:    1000,
					WALEnabled:   &walEnabled,
				}
			}(),
			wantErr: true,
		},
		{
			name: "negative batch size",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    -1,
					BatchTimeout: 100 * time.Millisecond,
					QueueSize:    1000,
					WALEnabled:   &walEnabled,
				}
			}(),
			wantErr: true,
		},
		{
			name: "batch size too large",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    20000,
					BatchTimeout: 100 * time.Millisecond,
					QueueSize:    1000,
					WALEnabled:   &walEnabled,
				}
			}(),
			wantErr: true,
		},
		{
			name: "zero timeout",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    100,
					BatchTimeout: 0,
					QueueSize:    1000,
					WALEnabled:   &walEnabled,
				}
			}(),
			wantErr: true,
		},
		{
			name: "timeout too large",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    100,
					BatchTimeout: 15 * time.Second,
					QueueSize:    1000,
					WALEnabled:   &walEnabled,
				}
			}(),
			wantErr: true,
		},
		{
			name: "zero queue size",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    100,
					BatchTimeout: 100 * time.Millisecond,
					QueueSize:    0,
					WALEnabled:   &walEnabled,
				}
			}(),
			wantErr: true,
		},
		{
			name: "queue size too large",
			config: func() BatchConfig {
				walEnabled := true
				return BatchConfig{
					BatchSize:    100,
					BatchTimeout: 100 * time.Millisecond,
					QueueSize:    200000,
					WALEnabled:   &walEnabled,
				}
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBatchConfigApplyDefaults(t *testing.T) {
	config := BatchConfig{}
	config.ApplyDefaults()

	defaults := DefaultBatchConfig()

	if config.BatchSize != defaults.BatchSize {
		t.Errorf("Expected BatchSize %d, got %d", defaults.BatchSize, config.BatchSize)
	}

	if config.BatchTimeout != defaults.BatchTimeout {
		t.Errorf("Expected BatchTimeout %v, got %v", defaults.BatchTimeout, config.BatchTimeout)
	}

	if config.QueueSize != defaults.QueueSize {
		t.Errorf("Expected QueueSize %d, got %d", defaults.QueueSize, config.QueueSize)
	}

	if config.WALEnabled == nil || *config.WALEnabled != *defaults.WALEnabled {
		t.Errorf("Expected WALEnabled %v, got %v", *defaults.WALEnabled, config.WALEnabled)
	}
}

func TestBatchConfigApplyDefaultsPartial(t *testing.T) {
	config := BatchConfig{
		BatchSize: 50, // Keep this value
		// Other fields should get defaults
	}
	config.ApplyDefaults()

	if config.BatchSize != 50 {
		t.Errorf("Expected BatchSize 50 (preserved), got %d", config.BatchSize)
	}

	defaults := DefaultBatchConfig()
	if config.BatchTimeout != defaults.BatchTimeout {
		t.Errorf("Expected BatchTimeout %v (default), got %v", defaults.BatchTimeout, config.BatchTimeout)
	}
}

func TestWriteRequest(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{
		Message: "test log",
	}

	req := newWriteRequest(entry, ctx)

	if req.entry != entry {
		t.Error("Expected entry to be preserved")
	}

	if req.ctx != ctx {
		t.Error("Expected context to be preserved")
	}

	if req.resultChan == nil {
		t.Error("Expected result channel to be created")
	}
}

func TestWriteRequestSendResult(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Send result
	go func() {
		time.Sleep(10 * time.Millisecond)
		req.sendResult(123, nil)
	}()

	// Wait for result
	id, err := req.waitForResult(100 * time.Millisecond)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if id != 123 {
		t.Errorf("Expected ID 123, got %d", id)
	}
}

func TestWriteRequestTimeout(t *testing.T) {
	ctx := context.Background()
	entry := &types.LogEntry{Message: "test"}
	req := newWriteRequest(entry, ctx)

	// Don't send result, should timeout
	id, err := req.waitForResult(10 * time.Millisecond)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if id != 0 {
		t.Errorf("Expected ID 0 on timeout, got %d", id)
	}
}

func TestWriteRequestCancellation(t *testing.T) {
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
		t.Error("Expected cancellation error")
	}

	if id != 0 {
		t.Errorf("Expected ID 0 on cancellation, got %d", id)
	}
}

func TestBatchBuffer(t *testing.T) {
	buffer := newBatchBuffer(3)

	if !buffer.isEmpty() {
		t.Error("Expected new buffer to be empty")
	}

	if buffer.size() != 0 {
		t.Errorf("Expected size 0, got %d", buffer.size())
	}

	// Add requests
	ctx := context.Background()
	req1 := newWriteRequest(&types.LogEntry{Message: "test1"}, ctx)
	req2 := newWriteRequest(&types.LogEntry{Message: "test2"}, ctx)
	req3 := newWriteRequest(&types.LogEntry{Message: "test3"}, ctx)

	// Add first request
	full := buffer.add(req1)
	if full {
		t.Error("Expected buffer not to be full after first add")
	}
	if buffer.size() != 1 {
		t.Errorf("Expected size 1, got %d", buffer.size())
	}

	// Add second request
	full = buffer.add(req2)
	if full {
		t.Error("Expected buffer not to be full after second add")
	}
	if buffer.size() != 2 {
		t.Errorf("Expected size 2, got %d", buffer.size())
	}

	// Add third request (should be full now)
	full = buffer.add(req3)
	if !full {
		t.Error("Expected buffer to be full after third add")
	}
	if buffer.size() != 3 {
		t.Errorf("Expected size 3, got %d", buffer.size())
	}

	// Flush buffer
	requests := buffer.flush()
	if len(requests) != 3 {
		t.Errorf("Expected 3 requests from flush, got %d", len(requests))
	}

	if !buffer.isEmpty() {
		t.Error("Expected buffer to be empty after flush")
	}

	if buffer.size() != 0 {
		t.Errorf("Expected size 0 after flush, got %d", buffer.size())
	}

	// Verify request order
	if requests[0].entry.Message != "test1" {
		t.Errorf("Expected first request message 'test1', got '%s'", requests[0].entry.Message)
	}
	if requests[1].entry.Message != "test2" {
		t.Errorf("Expected second request message 'test2', got '%s'", requests[1].entry.Message)
	}
	if requests[2].entry.Message != "test3" {
		t.Errorf("Expected third request message 'test3', got '%s'", requests[2].entry.Message)
	}
}

func TestBatchBufferFlushEmpty(t *testing.T) {
	buffer := newBatchBuffer(10)

	requests := buffer.flush()
	if requests != nil {
		t.Error("Expected nil from flushing empty buffer")
	}
}
