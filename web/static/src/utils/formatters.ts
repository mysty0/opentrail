import { FACILITIES, SEVERITIES } from './constants';
import type { SeverityInfo } from '../types';

export const formatTimestamp = (timestamp: string): string => {
  try {
    const date = new Date(timestamp);
    return date.toLocaleString('en-US', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false
    });
  } catch (error) {
    return timestamp;
  }
};

export const getFacilityName = (facility: number): string => {
  return FACILITIES[facility as keyof typeof FACILITIES] || `F${facility}`;
};

export const getSeverityInfo = (severity: number): SeverityInfo => {
  return SEVERITIES[severity as keyof typeof SEVERITIES] || { 
    name: `S${severity}`, 
    class: 'unknown' 
  };
};

export const escapeHtml = (text: string): string => {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
};

export const getCurrentTime = (): string => {
  return new Date().toLocaleString('en-US', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  });
};