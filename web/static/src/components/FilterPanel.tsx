import React, { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import type { LogFilters } from '../types';

interface FilterPanelProps {
  filters: LogFilters;
  onFiltersChange: (filters: LogFilters) => void;
  onApplyFilters: () => void;
  onClearFilters: () => void;
}

export const FilterPanel: React.FC<FilterPanelProps> = ({
  filters,
  onFiltersChange,
  onApplyFilters,
  onClearFilters
}) => {
  const [isExpanded, setIsExpanded] = useState(false);

  const handleInputChange = (field: keyof LogFilters, value: string | number | null) => {
    onFiltersChange({
      ...filters,
      [field]: value
    });
  };

  return (
    <div className="filter-panel">
      <div className="filter-header">
        <h3>RFC5424 Filters</h3>
        <button 
          className="filter-toggle" 
          onClick={() => setIsExpanded(!isExpanded)}
        >
          {isExpanded ? (
            <>
              <ChevronDown size={16} />
              Hide Filters
            </>
          ) : (
            <>
              <ChevronRight size={16} />
              Show Filters
            </>
          )}
        </button>
      </div>
      
      {isExpanded && (
        <div className="filter-content">
          <div className="filter-row">
            <div className="filter-group">
              <label htmlFor="facilityFilter">Facility:</label>
              <select 
                id="facilityFilter"
                value={filters.facility ?? ''}
                onChange={(e) => handleInputChange('facility', e.target.value ? parseInt(e.target.value) : null)}
              >
                <option value="">All</option>
                <option value="0">Kernel (0)</option>
                <option value="1">User (1)</option>
                <option value="2">Mail (2)</option>
                <option value="3">Daemon (3)</option>
                <option value="4">Auth (4)</option>
                <option value="5">Syslog (5)</option>
                <option value="6">LPR (6)</option>
                <option value="7">News (7)</option>
                <option value="8">UUCP (8)</option>
                <option value="9">Cron (9)</option>
                <option value="10">Authpriv (10)</option>
                <option value="11">FTP (11)</option>
                <option value="16">Local0 (16)</option>
                <option value="17">Local1 (17)</option>
                <option value="18">Local2 (18)</option>
                <option value="19">Local3 (19)</option>
                <option value="20">Local4 (20)</option>
                <option value="21">Local5 (21)</option>
                <option value="22">Local6 (22)</option>
                <option value="23">Local7 (23)</option>
              </select>
            </div>
            
            <div className="filter-group">
              <label htmlFor="severityFilter">Severity:</label>
              <select 
                id="severityFilter"
                value={filters.severity ?? ''}
                onChange={(e) => handleInputChange('severity', e.target.value ? parseInt(e.target.value) : null)}
              >
                <option value="">All</option>
                <option value="0">Emergency (0)</option>
                <option value="1">Alert (1)</option>
                <option value="2">Critical (2)</option>
                <option value="3">Error (3)</option>
                <option value="4">Warning (4)</option>
                <option value="5">Notice (5)</option>
                <option value="6">Info (6)</option>
                <option value="7">Debug (7)</option>
              </select>
            </div>
            
            <div className="filter-group">
              <label htmlFor="minSeverityFilter">Min Severity:</label>
              <select 
                id="minSeverityFilter"
                value={filters.minSeverity ?? ''}
                onChange={(e) => handleInputChange('minSeverity', e.target.value ? parseInt(e.target.value) : null)}
              >
                <option value="">None</option>
                <option value="0">Emergency+</option>
                <option value="1">Alert+</option>
                <option value="2">Critical+</option>
                <option value="3">Error+</option>
                <option value="4">Warning+</option>
                <option value="5">Notice+</option>
                <option value="6">Info+</option>
                <option value="7">Debug+</option>
              </select>
            </div>
          </div>
          
          <div className="filter-row">
            <div className="filter-group">
              <label htmlFor="hostnameFilter">Hostname:</label>
              <input 
                type="text" 
                id="hostnameFilter"
                placeholder="Filter by hostname"
                value={filters.hostname || ''}
                onChange={(e) => handleInputChange('hostname', e.target.value)}
              />
            </div>
            
            <div className="filter-group">
              <label htmlFor="appNameFilter">App Name:</label>
              <input 
                type="text" 
                id="appNameFilter"
                placeholder="Filter by application"
                value={filters.appName || ''}
                onChange={(e) => handleInputChange('appName', e.target.value)}
              />
            </div>
            
            <div className="filter-group">
              <label htmlFor="procIdFilter">Process ID:</label>
              <input 
                type="text" 
                id="procIdFilter"
                placeholder="Filter by process ID"
                value={filters.procId || ''}
                onChange={(e) => handleInputChange('procId', e.target.value)}
              />
            </div>
          </div>
          
          <div className="filter-row">
            <div className="filter-group">
              <label htmlFor="msgIdFilter">Message ID:</label>
              <input 
                type="text" 
                id="msgIdFilter"
                placeholder="Filter by message ID"
                value={filters.msgId || ''}
                onChange={(e) => handleInputChange('msgId', e.target.value)}
              />
            </div>
            
            <div className="filter-group">
              <label htmlFor="textFilter">Message Text:</label>
              <input 
                type="text" 
                id="textFilter"
                placeholder="Search in message content"
                value={filters.text || ''}
                onChange={(e) => handleInputChange('text', e.target.value)}
              />
            </div>
            
            <div className="filter-group filter-actions">
              <button onClick={onApplyFilters} className="btn-primary">
                Apply Filters
              </button>
              <button onClick={onClearFilters} className="btn-secondary">
                Clear All
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};