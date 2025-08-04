import React, { useEffect, useRef, useCallback } from 'react';
import { ArrowDown } from 'lucide-react';
import { LogEntry } from './LogEntry';
import { getCurrentTime } from '../utils/formatters';
import type { LogEntry as LogEntryType, DisplayOptions } from '../types';

interface LogContainerProps {
  logs: LogEntryType[];
  displayOptions: DisplayOptions;
  autoScroll: boolean;
  onAutoScrollChange: (autoScroll: boolean) => void;
  onLoadMore: () => void;
  isLoadingMore: boolean;
  hasMoreLogs: boolean;
}

export const LogContainer: React.FC<LogContainerProps> = ({
  logs,
  displayOptions,
  autoScroll,
  onAutoScrollChange,
  onLoadMore,
  isLoadingMore,
  hasMoreLogs
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const lastLogCountRef = useRef(logs.length);
  const loadMoreTimeoutRef = useRef<number | null>(null);
  const previousScrollHeightRef = useRef(0);
  const wasLoadingMoreRef = useRef(false);
  const autoScrollTimeoutRef = useRef<number | null>(null);

  const scrollToBottom = useCallback(() => {
    if (containerRef.current) {
      requestAnimationFrame(() => {
        if (containerRef.current) {
          containerRef.current.scrollTop = containerRef.current.scrollHeight;
        }
      });
    }
  }, []);

  const handleScroll = useCallback(() => {
    if (!containerRef.current) return;

    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 10; // Increased tolerance
    const isAtTop = scrollTop < 100;

    // Update auto-scroll state with some hysteresis to prevent flickering
    if (isAtBottom && !autoScroll) {
      onAutoScrollChange(true);
      console.log('Enabling auto-scroll due to near bottom');
    } else if (!isAtBottom && autoScroll && scrollHeight - scrollTop - clientHeight > 50) {
      // Only disable auto-scroll if user scrolled significantly up
      console.log('Disabling auto-scroll due to significant scroll up');
      onAutoScrollChange(false);
    }

    // Load more logs when scrolled near the top (with debouncing)
    if (isAtTop && !isLoadingMore && hasMoreLogs && logs.length > 0) {
      // Debounce the load more request
      if (loadMoreTimeoutRef.current) {
        clearTimeout(loadMoreTimeoutRef.current);
      }
      loadMoreTimeoutRef.current = setTimeout(() => {
        onLoadMore();
      }, 300);
    }
  }, [autoScroll, onAutoScrollChange, onLoadMore, isLoadingMore, hasMoreLogs, logs.length]);

  // Handle scroll position when logs change
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const logCountChanged = logs.length !== lastLogCountRef.current;
    const logCountIncreased = logs.length > lastLogCountRef.current;
    const wasLoadingMore = wasLoadingMoreRef.current;

    if (logCountChanged) {
      if (wasLoadingMore && !isLoadingMore) {
        // Just finished loading more logs - preserve scroll position
        const heightDifference = container.scrollHeight - previousScrollHeightRef.current;
        container.scrollTop = container.scrollTop + heightDifference;
        wasLoadingMoreRef.current = false;
      } else if (autoScroll && logCountIncreased && !wasLoadingMore) {
        // New logs arrived (not from load-more) and auto-scroll is enabled
        if (autoScrollTimeoutRef.current) {
          clearTimeout(autoScrollTimeoutRef.current);
        }
        autoScrollTimeoutRef.current = setTimeout(() => {
          scrollToBottom();
        }, 10);
      }
    }

    lastLogCountRef.current = logs.length;
    previousScrollHeightRef.current = container.scrollHeight;
  }, [logs.length, autoScroll, scrollToBottom, isLoadingMore]);

  // Track loading state changes
  useEffect(() => {
    if (isLoadingMore && !wasLoadingMoreRef.current) {
      // Started loading more logs - remember current scroll height
      if (containerRef.current) {
        previousScrollHeightRef.current = containerRef.current.scrollHeight;
      }
      wasLoadingMoreRef.current = true;
    }
  }, [isLoadingMore]);

  // Handle window resize
  useEffect(() => {
    const handleResize = () => {
      if (autoScroll) {
        setTimeout(scrollToBottom, 100);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [autoScroll, scrollToBottom]);

  // Cleanup timeouts on unmount
  useEffect(() => {
    return () => {
      if (loadMoreTimeoutRef.current) {
        clearTimeout(loadMoreTimeoutRef.current);
      }
      if (autoScrollTimeoutRef.current) {
        clearTimeout(autoScrollTimeoutRef.current);
      }
    };
  }, []);

  const handleScrollToBottom = () => {
    scrollToBottom();
    onAutoScrollChange(true);
  };

  return (
    <div className="terminal-container">
      <div className="terminal-header">
        <div className="terminal-controls">
          <span className="control-dot red"></span>
          <span className="control-dot yellow"></span>
          <span className="control-dot green"></span>
        </div>
        <div className="terminal-title">RFC5424 Log Stream</div>
      </div>
      
      <div className="terminal-content">
        <div 
          className={`log-container${displayOptions.compactMode ? ' compact-mode' : ''}`}
          ref={containerRef}
          onScroll={handleScroll}
        >
          {isLoadingMore && (
            <div className="loading-more">
              <div className="log-entry">
                <div className="log-entry-header">
                  <span className="timestamp">{getCurrentTime()}</span>
                  <span className="field-separator">|</span>
                  <span className="severity info">INFO</span>
                  <span className="field-separator">|</span>
                  <span className="app-name">system</span>
                </div>
                <div className="log-entry-message">Loading more logs...</div>
              </div>
            </div>
          )}
          
          {logs.length === 0 ? (
            <div className="welcome-message">
              <div className="log-entry">
                <div className="log-entry-header">
                  <span className="timestamp">{getCurrentTime()}</span>
                  <span className="field-separator">|</span>
                  <span className="priority">P:134</span>
                  <span className="field-separator">|</span>
                  <span className="facility">Local0</span>
                  <span className="field-separator">|</span>
                  <span className="severity info">INFO</span>
                  <span className="field-separator">|</span>
                  <span className="hostname">localhost</span>
                  <span className="field-separator">|</span>
                  <span className="app-name">opentrail</span>
                  <span className="field-separator">|</span>
                  <span className="proc-id">-</span>
                  <span className="field-separator">|</span>
                  <span className="msg-id">startup</span>
                </div>
                <div className="log-entry-message">
                  OpenTrail RFC5424 log viewer initialized. Waiting for log entries...
                </div>
              </div>
            </div>
          ) : (
            logs.map((log, index) => (
              <LogEntry
                key={`${log.id}-${index}`}
                logEntry={log}
                displayOptions={displayOptions}
                isNew={false} // Remove the new animation for now to avoid confusion
              />
            ))
          )}
        </div>
        
        {!autoScroll && (
          <div className="scroll-indicator visible">
            <button className="scroll-to-bottom" onClick={handleScrollToBottom}>
              <ArrowDown size={16} />
              Scroll to bottom to resume auto-scroll
            </button>
          </div>
        )}
      </div>
    </div>
  );
};