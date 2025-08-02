import React from 'react';
import type { ConnectionStatus as ConnectionStatusType } from '../types';

interface ConnectionStatusProps {
  connectionStatus: ConnectionStatusType;
}

export const ConnectionStatus: React.FC<ConnectionStatusProps> = ({ connectionStatus }) => {
  return (
    <div className="connection-status">
      <span className={`status-indicator ${connectionStatus.status}`}></span>
      <span className="status-text">{connectionStatus.text}</span>
    </div>
  );
};