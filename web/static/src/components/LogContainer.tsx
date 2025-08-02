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
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 5;
    const isAtTop = scrollTop < 50;

    // Update auto-scroll state
    if (isAtBottom && !autoScroll) {
      onAutoScrollChange(true);
    } else if (!isAtBottom && autoScroll) {
      onAutoScrollChange(false);
    }

    // Load more logs when scrolled near the top
    if (isAtTop && !isLoadingMore && hasMoreLogs && logs.length > 0) {
      onLoadMore();
    }
  }, [autoScroll, onAutoScrollChange, onLoadMore, isLoadingMore, hasMoreLogs, logs.length]);

  // Auto-scroll when new logs arrive
  useEffect(() => {
    if (autoScroll && logs.length > lastLogCountRef.current) {
      scrollToBottom();
    }
    lastLogCountRef.current = logs.length;
  }, [logs.length, autoScroll, scrollToBottom]);

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
                isNew={index === logs.length - 1}
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