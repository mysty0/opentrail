import { useState, useEffect, useRef, useCallback } from 'react';
import type { LogEntry, ConnectionStatus } from '../types';

interface UseWebSocketOptions {
  onMessage: (logEntry: LogEntry) => void;
  maxReconnectAttempts?: number;
  reconnectDelay?: number;
}

export const useWebSocket = ({ 
  onMessage, 
  maxReconnectAttempts = 5, 
  reconnectDelay = 1000 
}: UseWebSocketOptions) => {
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>({
    status: 'connecting',
    text: 'Connecting...'
  });
  
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<number | null>(null);

  const updateConnectionStatus = useCallback((status: ConnectionStatus['status'], text: string) => {
    setConnectionStatus({ status, text });
  }, []);

  const connect = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.CONNECTING) {
      return;
    }

    updateConnectionStatus('connecting', 'Connecting...');

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/logs/stream`;

    try {
      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        console.log('WebSocket connected');
        reconnectAttemptsRef.current = 0;
        updateConnectionStatus('connected', 'Connected');
      };

      wsRef.current.onmessage = (event) => {
        try {
          const logEntry: LogEntry = JSON.parse(event.data);
          onMessage(logEntry);
        } catch (error) {
          console.error('Error parsing log message:', error);
        }
      };

      wsRef.current.onclose = (event) => {
        console.log('WebSocket disconnected:', event.code, event.reason);
        updateConnectionStatus('disconnected', 'Disconnected');

        // Attempt to reconnect if not a normal closure
        if (event.code !== 1000 && reconnectAttemptsRef.current < maxReconnectAttempts) {
          scheduleReconnect();
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('WebSocket error:', error);
        updateConnectionStatus('error', 'Connection error');
      };

    } catch (error) {
      console.error('WebSocket connection error:', error);
      updateConnectionStatus('error', 'Connection error');
    }
  }, [onMessage, maxReconnectAttempts, updateConnectionStatus]);

  const scheduleReconnect = useCallback(() => {
    reconnectAttemptsRef.current++;
    const delay = reconnectDelay * Math.pow(2, reconnectAttemptsRef.current - 1);

    updateConnectionStatus('connecting', `Reconnecting in ${Math.ceil(delay / 1000)}s...`);

    reconnectTimeoutRef.current = setTimeout(() => {
      if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
        connect();
      }
    }, delay);
  }, [connect, reconnectDelay, updateConnectionStatus]);

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    
    if (wsRef.current) {
      wsRef.current.close(1000, 'Manual disconnect');
      wsRef.current = null;
    }
  }, []);

  const reconnect = useCallback(() => {
    disconnect();
    reconnectAttemptsRef.current = 0;
    connect();
  }, [connect, disconnect]);

  useEffect(() => {
    connect();

    const handleVisibilityChange = () => {
      if (!document.hidden && (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN)) {
        connect();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    connectionStatus,
    reconnect,
    disconnect
  };
};