# Design Document

## Overview

This design implements a high-performance, non-blocking SQLite storage layer that uses batched writes, WAL mode, and asynchronous processing to optimize write throughput while maintaining real-time responsiveness. The solution introduces a new `BatchedSQLiteStorage` that wraps the existing SQLite operations with batching capabilities while maintaining interface compatibility.

## Architecture

### Core Components

1. **BatchedSQLiteStorage**: Main storage implementation with batching capabilities
2. **BatchProcessor**: Background goroutine that handles batch write operations
3. **WriteQueue**: Channel-based queue for pending write operations
4. **WAL Configuration**: SQLite WAL mode setup and management
5. **Completion Tracking**: Mechanism to track write completion for ID assignment

### Data Flow

```
LogService -> BatchedSQLiteStorage.Store() -> WriteQueue -> BatchProcessor -> SQLite (WAL mode)
                                    |
                                    v
                              Real-time notification (immediate)
```

## Components and Interfaces

### BatchedSQLiteStorage Structure

```go
type BatchedSQLiteStorage struct {
    db *sql.DB
    
    // Batching configuration
    batchSize    int
    batchTimeout time.Duration
    queueSize    int
    
    // Processing components
    writeQueue   chan *writeRequest
    batchBuffer  []*writeRequest
    batchMutex   sync.Mutex
    batchTimer   *time.Timer
    
    // Lifecycle management
    ctx          context.Context
    cancel       context.CancelFunc
    wg           sync.WaitGroup
    isRunning    bool
    runningMux   sync.RWMutex
    
    // Prepared statements for batch operations
    insertStmt   *sql.Stmt
    batchStmt    *sql.Stmt
}

type writeRequest struct {
    entry      *types.LogEntry
    resultChan chan writeResult
}

type writeResult struct {
    id  int64
    err error
}
```

### Configuration Options

```go
type BatchConfig struct {
    BatchSize    int           // Number of entries per batch (default: 100)
    BatchTimeout time.Duration // Max time to wait for batch (default: 100ms)
    QueueSize    int           // Size of write queue (default: 10000)
    WALEnabled   bool          // Enable WAL mode (default: true)
}
```

### Interface Compatibility

The `BatchedSQLiteStorage` implements the existing `interfaces.LogStorage` interface:

```go
type LogStorage interface {
    Store(entry *types.LogEntry) error
    Search(query types.SearchQuery) ([]*types.LogEntry, error)
    GetRecent(limit int) ([]*types.LogEntry, error)
    Cleanup(retentionDays int) error
    Close() error
}
```

## Data Models

### Write Request Flow

1. **Immediate Response**: `Store()` method returns immediately after queuing
2. **Async Processing**: Background processor handles actual database writes
3. **ID Assignment**: LogEntry.ID is set when database write completes
4. **Error Handling**: Failed writes are retried individually

### Batch Processing Logic

```go
type batchProcessor struct {
    storage     *BatchedSQLiteStorage
    buffer      []*writeRequest
    timer       *time.Timer
    mutex       sync.Mutex
}
```

### WAL Mode Configuration

SQLite will be configured with:
- `PRAGMA journal_mode = WAL`
- `PRAGMA synchronous = NORMAL` 
- `PRAGMA wal_autocheckpoint = 1000`
- `PRAGMA busy_timeout = 5000`

## Error Handling

### Batch Write Failures

1. **Batch Retry**: If entire batch fails, retry with smaller batches
2. **Individual Retry**: If batch continues to fail, process entries individually
3. **Error Propagation**: Critical errors are logged but don't block processing
4. **Graceful Degradation**: System falls back to individual writes if batching fails

### Queue Management

1. **Backpressure**: When queue is full, `Store()` returns error immediately
2. **Graceful Shutdown**: During shutdown, process all queued writes
3. **Context Cancellation**: Respect context cancellation for clean shutdown

### Database Connection Issues

1. **Connection Retry**: Implement exponential backoff for connection failures
2. **WAL Recovery**: Handle WAL file corruption and recovery scenarios
3. **Disk Space**: Handle disk full scenarios gracefully

## Testing Strategy

### Unit Tests

1. **Batch Processing Logic**
   - Test batch size triggers
   - Test timeout triggers
   - Test mixed batch scenarios

2. **WAL Mode Functionality**
   - Verify WAL mode is enabled
   - Test concurrent read/write operations
   - Test checkpoint behavior

3. **Error Handling**
   - Test queue full scenarios
   - Test database connection failures
   - Test batch write failures

4. **Interface Compatibility**
   - Verify all existing tests pass
   - Test ID assignment behavior
   - Test concurrent access patterns

### Integration Tests

1. **Performance Testing**
   - Measure write throughput improvement
   - Test under high load conditions
   - Verify real-time responsiveness

2. **Concurrency Testing**
   - Test multiple concurrent writers
   - Test reader/writer concurrency
   - Test subscriber notification timing

3. **Reliability Testing**
   - Test graceful shutdown scenarios
   - Test crash recovery with WAL
   - Test long-running stability

### Benchmarks

1. **Write Performance**
   - Individual vs batched writes
   - Various batch sizes
   - Memory usage patterns

2. **Read Performance**
   - Verify no read performance regression
   - Test concurrent read/write scenarios

## Implementation Phases

### Phase 1: WAL Mode Setup
- Configure SQLite with WAL mode
- Update database initialization
- Add WAL-specific pragmas

### Phase 2: Batching Infrastructure
- Implement write queue and batch processor
- Add configuration options
- Create batch processing logic

### Phase 3: Interface Integration
- Implement LogStorage interface
- Add completion tracking for ID assignment
- Ensure backward compatibility

### Phase 4: Error Handling & Resilience
- Add comprehensive error handling
- Implement retry mechanisms
- Add graceful shutdown logic

### Phase 5: Testing & Optimization
- Add comprehensive test suite
- Performance benchmarking
- Memory usage optimization

## Performance Expectations

### Write Throughput
- **Target**: 5-10x improvement over individual writes
- **Baseline**: Current ~1000 writes/second
- **Goal**: 5000-10000 writes/second

### Memory Usage
- **Queue Memory**: ~10MB for 10k entry queue
- **Batch Buffer**: ~1MB for 100 entry batches
- **Total Overhead**: <20MB additional memory

### Latency
- **Write Latency**: <1ms for Store() call (non-blocking)
- **Batch Latency**: <100ms from queue to database
- **Real-time Updates**: No additional latency for subscribers