import React, { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import type { DisplayOptions } from '../types';

interface DisplayPanelProps {
  displayOptions: DisplayOptions;
  onDisplayOptionsChange: (options: DisplayOptions) => void;
  onResetDisplayOptions: () => void;
}

export const DisplayPanel: React.FC<DisplayPanelProps> = ({
  displayOptions,
  onDisplayOptionsChange,
  onResetDisplayOptions
}) => {
  const [isExpanded, setIsExpanded] = useState(false);

  const handleCheckboxChange = (field: keyof DisplayOptions, checked: boolean) => {
    onDisplayOptionsChange({
      ...displayOptions,
      [field]: checked
    });
  };

  return (
    <div className="display-panel">
      <div className="display-header">
        <h3>Display Options</h3>
        <button 
          className="display-toggle" 
          onClick={() => setIsExpanded(!isExpanded)}
        >
          {isExpanded ? (
            <>
              <ChevronDown size={16} />
              Hide Options
            </>
          ) : (
            <>
              <ChevronRight size={16} />
              Show Options
            </>
          )}
        </button>
      </div>
      
      {isExpanded && (
        <div className="display-content">
          <div className="display-section">
            <h4>Show/Hide Fields</h4>
            <div className="checkbox-grid">
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showTimestamp}
                  onChange={(e) => handleCheckboxChange('showTimestamp', e.target.checked)}
                />
                <span>Timestamp</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showPriority}
                  onChange={(e) => handleCheckboxChange('showPriority', e.target.checked)}
                />
                <span>Priority</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showFacility}
                  onChange={(e) => handleCheckboxChange('showFacility', e.target.checked)}
                />
                <span>Facility</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showSeverity}
                  onChange={(e) => handleCheckboxChange('showSeverity', e.target.checked)}
                />
                <span>Severity</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showHostname}
                  onChange={(e) => handleCheckboxChange('showHostname', e.target.checked)}
                />
                <span>Hostname</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showAppName}
                  onChange={(e) => handleCheckboxChange('showAppName', e.target.checked)}
                />
                <span>App Name</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showProcId}
                  onChange={(e) => handleCheckboxChange('showProcId', e.target.checked)}
                />
                <span>Process ID</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showMsgId}
                  onChange={(e) => handleCheckboxChange('showMsgId', e.target.checked)}
                />
                <span>Message ID</span>
              </label>
            </div>
          </div>
          
          <div className="display-section">
            <h4>Layout Options</h4>
            <div className="layout-options">
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.showSeparators}
                  onChange={(e) => handleCheckboxChange('showSeparators', e.target.checked)}
                />
                <span>Field Separators</span>
              </label>
              
              <label className="checkbox-item">
                <input 
                  type="checkbox" 
                  checked={displayOptions.compactMode}
                  onChange={(e) => handleCheckboxChange('compactMode', e.target.checked)}
                />
                <span>Compact Mode</span>
              </label>
            </div>
          </div>
          
          <div className="display-actions">
            <button onClick={onResetDisplayOptions} className="btn-secondary">
              Reset to Default
            </button>
          </div>
        </div>
      )}
    </div>
  );
};