import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { ConnectionStatus } from './components/ConnectionStatus';
import { FilterPanel } from './components/FilterPanel';
import { DisplayPanel } from './components/DisplayPanel';
import { LogContainer } from './components/LogContainer';
import { useWebSocket } from './hooks/useWebSocket';
import { useLocalStorage } from './hooks/useLocalStorage';
import { ApiService } from './services/api';
import { DEFAULT_DISPLAY_OPTIONS, STORAGE_KEYS } from './utils/constants';
import type { LogEntry, LogFilters, DisplayOptions } from './types';

const App: React.FC = () => {
  // State management
  const [allLogs, setAllLogs] = useState<LogEntry[]>([]);
  const [filters, setFilters] = useState<LogFilters>({});
  const [autoScroll, setAutoScroll] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [hasMoreLogs, setHasMoreLogs] = useState(true);
  
  // Persistent display options
  const [displayOptions, setDisplayOptions] = useLocalStorage<DisplayOptions>(
    STORAGE_KEYS.DISPLAY_OPTIONS,
    DEFAULT_DISPLAY_OPTIONS
  );

  // API service
  const apiService = useMemo(() => ApiService.getInstance(), []);

  // WebSocket connection
  const handleNewLogEntry = useCallback((logEntry: LogEntry) => {
    setAllLogs(prev => [...prev, logEntry]);
  }, []);

  const { connectionStatus } = useWebSocket({
    onMessage: handleNewLogEntry
  });

  // Filter logs based on current filters
  const filteredLogs = useMemo(() => {
    if (!filters || Object.keys(filters).length === 0) {
      return allLogs;
    }

    return allLogs.filter(log => {
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
  }, [allLogs, filters]);

  // Load initial logs
  useEffect(() => {
    const loadInitialLogs = async () => {
      try {
        const logs = await apiService.fetchLogs(50, 0);
        setAllLogs(logs);
        setHasMoreLogs(logs.length === 50);
      } catch (error) {
        console.error('Failed to load initial logs:', error);
      }
    };

    loadInitialLogs();
  }, [apiService]);

  // Load more logs
  const handleLoadMore = useCallback(async () => {
    if (isLoadingMore || !hasMoreLogs) return;

    setIsLoadingMore(true);
    try {
      const moreLogs = await apiService.fetchLogs(50, allLogs.length);
      
      if (moreLogs.length > 0) {
        setAllLogs(prev => [...moreLogs, ...prev]);
        setHasMoreLogs(moreLogs.length === 50);
      } else {
        setHasMoreLogs(false);
      }
    } catch (error) {
      console.error('Failed to load more logs:', error);
    } finally {
      setIsLoadingMore(false);
    }
  }, [apiService, allLogs.length, isLoadingMore, hasMoreLogs]);

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