export interface LogEntry {
  id: string;
  timestamp: string;
  priority: number;
  facility: number;
  severity: number;
  hostname: string;
  app_name: string;
  proc_id: string;
  msg_id: string;
  message: string;
  structured_data?: Record<string, any>;
}

export interface LogFilters {
  facility?: number | null;
  severity?: number | null;
  minSeverity?: number | null;
  hostname?: string;
  appName?: string;
  procId?: string;
  msgId?: string;
  text?: string;
}

export interface DisplayOptions {
  showTimestamp: boolean;
  showPriority: boolean;
  showFacility: boolean;
  showSeverity: boolean;
  showHostname: boolean;
  showAppName: boolean;
  showProcId: boolean;
  showMsgId: boolean;
  showSeparators: boolean;
  compactMode: boolean;
}

export interface ConnectionStatus {
  status: 'connected' | 'connecting' | 'disconnected' | 'error';
  text: string;
}

export interface SeverityInfo {
  name: string;
  class: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}