import React, { useEffect, useRef, useCallback, useState } from 'react';
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
  const [isUserScrolling, setIsUserScrolling] = useState(false);
  const scrollTimeoutRef = useRef<number | null>(null);
  const lastScrollTop = useRef(0);
  const previousScrollHeight = useRef(0);



  // Optimized scroll handler with debouncing
  const handleScroll = useCallback(() => {
    if (!containerRef.current) return;

    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 5;
    const isAtTop = scrollTop < 50;

    // Clear existing timeout
    if (scrollTimeoutRef.current) {
      clearTimeout(scrollTimeoutRef.current);
    }

    // Mark as user scrolling
    setIsUserScrolling(true);

    // Debounced scroll handling
    scrollTimeoutRef.current = setTimeout(() => {
      setIsUserScrolling(false);
      
      // Update auto-scroll state
      if (isAtBottom && !autoScroll) {
        onAutoScrollChange(true);
      } else if (!isAtBottom && autoScroll) {
        onAutoScrollChange(false);
      }

      // Load more when at top
      if (isAtTop && !isLoadingMore && hasMoreLogs && logs.length > 0) {
        onLoadMore();
      }
    }, 150);

    lastScrollTop.current = scrollTop;
  }, [autoScroll, onAutoScrollChange, onLoadMore, isLoadingMore, hasMoreLogs, logs.length]);

  // Handle new logs - only scroll if auto-scroll is enabled and user isn't actively scrolling
  useEffect(() => {
    if (!containerRef.current) return;

    const container = containerRef.current;
    const currentScrollHeight = container.scrollHeight;

    // If we're loading more logs (prepending), maintain scroll position
    if (isLoadingMore && previousScrollHeight.current > 0) {
      const heightDiff = currentScrollHeight - previousScrollHeight.current;
      if (heightDiff > 0) {
        container.scrollTop = lastScrollTop.current + heightDiff;
      }
    }
    // If auto-scroll is enabled and user isn't scrolling, scroll to bottom
    else if (autoScroll && !isUserScrolling) {
      container.scrollTop = currentScrollHeight;
    }

    previousScrollHeight.current = currentScrollHeight;
  }, [logs, autoScroll, isUserScrolling, isLoadingMore]);

  // Cleanup
  useEffect(() => {
    return () => {
      if (scrollTimeoutRef.current) {
        clearTimeout(scrollTimeoutRef.current);
      }
    };
  }, []);

  const handleScrollToBottom = useCallback(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
      onAutoScrollChange(true);
    }
  }, [onAutoScrollChange]);

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
            logs.map((log) => (
              <LogEntry
                key={log.id}
                logEntry={log}
                displayOptions={displayOptions}
                isNew={false}
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