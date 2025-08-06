import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { ConnectionStatus } from './components/ConnectionStatus';
import { FilterPanel } from './components/FilterPanel';
import { DisplayPanel } from './components/DisplayPanel';
import { LogContainer } from './components/LogContainer';
import { useWebSocket } from './hooks/useWebSocket';
import { useLocalStorage } from './hooks/useLocalStorage';
import { ApiService } from './services/api';
import { DEFAULT_DISPLAY_OPTIONS, STORAGE_KEYS } from './utils/constants';
import type { LogEntry, LogFilters, DisplayOptions } from './types';

const MAX_RENDERED_LOGS = 500;
const LOAD_BATCH_SIZE = 50;

const App: React.FC = () => {
  // State management - using a circular buffer approach
  const [displayedLogs, setDisplayedLogs] = useState<LogEntry[]>([]);
  const [filters, setFilters] = useState<LogFilters>({});
  const [autoScroll, setAutoScroll] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [hasMoreLogs, setHasMoreLogs] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  // Track the oldest log timestamp for pagination
  const oldestLogTimestamp = useRef<string | null>(null);
  const newestLogTimestamp = useRef<string | null>(null);

  
  // Persistent display options
  const [displayOptions, setDisplayOptions] = useLocalStorage<DisplayOptions>(
    STORAGE_KEYS.DISPLAY_OPTIONS,
    DEFAULT_DISPLAY_OPTIONS
  );

  // API service
  const apiService = useMemo(() => ApiService.getInstance(), []);

  // WebSocket connection - optimized for performance
  const handleNewLogEntry = useCallback((logEntry: LogEntry) => {
    setDisplayedLogs(prev => {
      // Simply append new logs to the end (they should be newer)
      const updated = [...prev, logEntry];
      
      // Update newest timestamp
      newestLogTimestamp.current = logEntry.timestamp;
      
      // Trim to max size if needed (remove from beginning)
      if (updated.length > MAX_RENDERED_LOGS) {
        const trimmed = updated.slice(-MAX_RENDERED_LOGS);
        // Update oldest timestamp after trimming
        if (trimmed.length > 0) {
          oldestLogTimestamp.current = trimmed[0].timestamp;
        }
        return trimmed;
      }
      
      return updated;
    });
  }, []);

  const { connectionStatus } = useWebSocket({
    onMessage: handleNewLogEntry
  });

  // Filter logs based on current filters - optimized
  const filteredLogs = useMemo(() => {
    if (!filters || Object.keys(filters).length === 0) {
      return displayedLogs;
    }

    return displayedLogs.filter(log => {
      // Facility filter
      if (filters.facility !== null && filters.facility !== undefined && 
          log.facility !== filters.facility) {
        return false;
      }

      // Severity filter (exact match)
      if (filters.severity !== null && filters.severity !== undefined && 
          log.severity !== filters.severity) {
        return false;
      }

      // Minimum severity filter (severity <= minSeverity, lower numbers are more severe)
      if (filters.minSeverity !== null && filters.minSeverity !== undefined && 
          log.severity > filters.minSeverity) {
        return false;
      }

      // Hostname filter
      if (filters.hostname && 
          !log.hostname.toLowerCase().includes(filters.hostname.toLowerCase())) {
        return false;
      }

      // App name filter
      if (filters.appName && 
          !log.app_name.toLowerCase().includes(filters.appName.toLowerCase())) {
        return false;
      }

      // Process ID filter
      if (filters.procId && 
          !log.proc_id.toLowerCase().includes(filters.procId.toLowerCase())) {
        return false;
      }

      // Message ID filter
      if (filters.msgId && 
          !log.msg_id.toLowerCase().includes(filters.msgId.toLowerCase())) {
        return false;
      }

      // Text filter (search in message)
      if (filters.text && 
          !log.message.toLowerCase().includes(filters.text.toLowerCase())) {
        return false;
      }

      return true;
    });
  }, [displayedLogs, filters]);

  // Load initial logs
  useEffect(() => {
    const loadInitialLogs = async () => {
      try {
        setError(null);
        const logs = await apiService.fetchLogs(LOAD_BATCH_SIZE, 0);
        
        if (logs.length > 0) {
          // Logs should already be sorted by the API (newest first)
          setDisplayedLogs(logs);
          
          // Track timestamps for pagination
          oldestLogTimestamp.current = logs[logs.length - 1].timestamp;
          newestLogTimestamp.current = logs[0].timestamp;
          
          setHasMoreLogs(logs.length === LOAD_BATCH_SIZE);
        }
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Failed to load initial logs';
        console.error('Failed to load initial logs:', errorMessage);
        setError(errorMessage);
      }
    };

    loadInitialLogs();
  }, [apiService]);

  // Load more logs (older logs when scrolling up)
  const handleLoadMore = useCallback(async () => {
    if (isLoadingMore || !hasMoreLogs || !oldestLogTimestamp.current) return;

    setIsLoadingMore(true);
    try {
      // Fetch older logs using timestamp-based pagination
      const moreLogs = await apiService.fetchLogsBefore(oldestLogTimestamp.current, LOAD_BATCH_SIZE);
      
      if (moreLogs.length > 0) {
        setDisplayedLogs(prev => {
          // Prepend older logs to the beginning
          const combined = [...moreLogs, ...prev];
          
          // Trim from the end if we exceed max size
          const trimmed = combined.length > MAX_RENDERED_LOGS 
            ? combined.slice(0, MAX_RENDERED_LOGS)
            : combined;
          
          // Update timestamps
          if (trimmed.length > 0) {
            oldestLogTimestamp.current = trimmed[0].timestamp;
            if (trimmed.length < combined.length) {
              // We trimmed some logs from the end, update newest timestamp
              newestLogTimestamp.current = trimmed[trimmed.length - 1].timestamp;
            }
          }
          
          return trimmed;
        });
        
        setHasMoreLogs(moreLogs.length === LOAD_BATCH_SIZE);
      } else {
        setHasMoreLogs(false);
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to load more logs';
      console.error('Failed to load more logs:', errorMessage);
      setError(errorMessage);
      // Don't disable hasMoreLogs on error, allow retry
    } finally {
      setIsLoadingMore(false);
    }
  }, [apiService, isLoadingMore, hasMoreLogs]);

  // Filter handlers
  const handleApplyFilters = useCallback(() => {
    // Filters are applied automatically via useMemo
    console.log('Filters applied:', filters);
  }, [filters]);

  const handleClearFilters = useCallback(() => {
    setFilters({});
  }, []);

  // Display option handlers
  const handleResetDisplayOptions = useCallback(() => {
    setDisplayOptions(DEFAULT_DISPLAY_OPTIONS);
  }, [setDisplayOptions]);

  return (
    <div className="container">
      <header className="header">
        <h1 className="title">OpenTrail</h1>
        <ConnectionStatus connectionStatus={connectionStatus} />
      </header>
      
      <main className="main">
        <FilterPanel
          filters={filters}
          onFiltersChange={setFilters}
          onApplyFilters={handleApplyFilters}
          onClearFilters={handleClearFilters}
        />
        
        <DisplayPanel
          displayOptions={displayOptions}
          onDisplayOptionsChange={setDisplayOptions}
          onResetDisplayOptions={handleResetDisplayOptions}
        />
        
        {error && (
          <div className="error-banner">
            <span className="error-message">{error}</span>
            <button 
              className="error-dismiss" 
              onClick={() => setError(null)}
              aria-label="Dismiss error"
            >
              ×
            </button>
          </div>
        )}
        
        <LogContainer
          logs={filteredLogs}
          displayOptions={displayOptions}
          autoScroll={autoScroll}
          onAutoScrollChange={setAutoScroll}
          onLoadMore={handleLoadMore}
          isLoadingMore={isLoadingMore}
          hasMoreLogs={hasMoreLogs}
        />
      </main>
    </div>
  );
};

export default App;