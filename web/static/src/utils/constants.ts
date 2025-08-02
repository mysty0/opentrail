export const FACILITIES = {
  0: 'Kernel',
  1: 'User',
  2: 'Mail',
  3: 'Daemon',
  4: 'Auth',
  5: 'Syslog',
  6: 'LPR',
  7: 'News',
  8: 'UUCP',
  9: 'Cron',
  10: 'Authpriv',
  11: 'FTP',
  16: 'Local0',
  17: 'Local1',
  18: 'Local2',
  19: 'Local3',
  20: 'Local4',
  21: 'Local5',
  22: 'Local6',
  23: 'Local7'
} as const;

export const SEVERITIES = {
  0: { name: 'EMERG', class: 'emergency' },
  1: { name: 'ALERT', class: 'alert' },
  2: { name: 'CRIT', class: 'critical' },
  3: { name: 'ERR', class: 'error' },
  4: { name: 'WARN', class: 'warning' },
  5: { name: 'NOTICE', class: 'notice' },
  6: { name: 'INFO', class: 'info' },
  7: { name: 'DEBUG', class: 'debug' }
} as const;

export const DEFAULT_DISPLAY_OPTIONS = {
  showTimestamp: true,
  showPriority: true,
  showFacility: true,
  showSeverity: true,
  showHostname: true,
  showAppName: true,
  showProcId: true,
  showMsgId: true,
  showSeparators: true,
  compactMode: false
};

export const STORAGE_KEYS = {
  DISPLAY_OPTIONS: 'opentrail-display-options'
} as const;