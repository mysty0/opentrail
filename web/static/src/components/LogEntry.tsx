import React, { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { formatTimestamp, getFacilityName, getSeverityInfo } from '../utils/formatters';
import type { LogEntry as LogEntryType, DisplayOptions } from '../types';

interface LogEntryProps {
  logEntry: LogEntryType;
  displayOptions: DisplayOptions;
  isNew?: boolean;
}

export const LogEntry: React.FC<LogEntryProps> = ({ 
  logEntry, 
  displayOptions, 
  isNew = false 
}) => {
  const [showStructuredData, setShowStructuredData] = useState(false);

  const timestamp = formatTimestamp(logEntry.timestamp);
  const priority = logEntry.priority || 0;
  const facility = getFacilityName(logEntry.facility || 0);
  const severity = getSeverityInfo(logEntry.severity || 0);
  const hostname = logEntry.hostname || '-';
  const appName = logEntry.app_name || '-';
  const procId = logEntry.proc_id || '-';
  const msgId = logEntry.msg_id || '-';
  const message = logEntry.message || '';

  const hasStructuredData = logEntry.structured_data && 
    Object.keys(logEntry.structured_data).length > 0;

  return (
    <div className={`log-entry${isNew ? ' new' : ''}`}>
      <div className="log-entry-header">
        {displayOptions.showTimestamp && (
          <>
            <span className="timestamp">{timestamp}</span>
            {displayOptions.showSeparators && <span className="field-separator">|</span>}
          </>
        )}
        
        {displayOptions.showPriority && (
          <>
            <span className="priority">P:{priority}</span>
            {displayOptions.showSeparators && <span className="field-separator">|</span>}
          </>
        )}
        
        {displayOptions.showFacility && (
          <>
            <span className="facility">{facility}</span>
            {displayOptions.showSeparators && <span className="field-separator">|</span>}
          </>
        )}
        
        {displayOptions.showSeverity && (
          <>
            <span className={`severity ${severity.class}`}>{severity.name}</span>
            {displayOptions.showSeparators && <span className="field-separator">|</span>}
          </>
        )}
        
        {displayOptions.showHostname && (
          <>
            <span className="hostname">{hostname}</span>
            {displayOptions.showSeparators && <span className="field-separator">|</span>}
          </>
        )}
        
        {displayOptions.showAppName && (
          <>
            <span className="app-name">{appName}</span>
            {displayOptions.showSeparators && <span className="field-separator">|</span>}
          </>
        )}
        
        {displayOptions.showProcId && (
          <>
            <span className="proc-id">{procId}</span>
            {displayOptions.showSeparators && <span className="field-separator">|</span>}
          </>
        )}
        
        {displayOptions.showMsgId && (
          <span className="msg-id">{msgId}</span>
        )}
      </div>
      
      <div className="log-entry-message">{message}</div>
      
      {hasStructuredData && (
        <div className="log-entry-structured-data">
          <button 
            className="structured-data-toggle"
            onClick={() => setShowStructuredData(!showStructuredData)}
          >
            {showStructuredData ? (
              <>
                <ChevronDown size={12} />
                Hide Structured Data
              </>
            ) : (
              <>
                <ChevronRight size={12} />
                Show Structured Data
              </>
            )}
          </button>
          
          {showStructuredData && (
            <div className="structured-data-content">
              <pre>{JSON.stringify(logEntry.structured_data, null, 2)}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
};