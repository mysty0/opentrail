import type { LogEntry, ApiResponse } from '../types';

export class ApiService {
  private static instance: ApiService;

  static getInstance(): ApiService {
    if (!ApiService.instance) {
      ApiService.instance = new ApiService();
    }
    return ApiService.instance;
  }

  async fetchLogs(limit = 50, offset = 0): Promise<LogEntry[]> {
    try {
      const params = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString()
      });

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 10000); // 10 second timeout

      const response = await fetch(`/api/logs?${params}`, {
        signal: controller.signal,
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json'
        }
      });
      
      clearTimeout(timeoutId);
      
      if (!response.ok) {
        const errorText = await response.text().catch(() => 'Unknown error');
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }

      const data: ApiResponse<LogEntry[]> = await response.json();
      
      if (!data.success) {
        throw new Error(data.error || 'API returned unsuccessful response');
      }

      return data.data || [];
    } catch (error) {
      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new Error('Request timeout - please try again');
        }
        console.error('Error fetching logs:', error.message);
        throw new Error(`Failed to fetch logs: ${error.message}`);
      }
      console.error('Unknown error fetching logs:', error);
      throw new Error('Failed to fetch logs: Unknown error');
    }
  }

  async fetchLogsBefore(beforeTimestamp: string, limit = 50): Promise<LogEntry[]> {
    try {
      const params = new URLSearchParams({
        limit: limit.toString(),
        end_time: beforeTimestamp
      });

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 10000); // 10 second timeout

      const response = await fetch(`/api/logs?${params}`, {
        signal: controller.signal,
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json'
        }
      });
      
      clearTimeout(timeoutId);
      
      if (!response.ok) {
        const errorText = await response.text().catch(() => 'Unknown error');
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }

      const data: ApiResponse<LogEntry[]> = await response.json();
      
      if (!data.success) {
        throw new Error(data.error || 'API returned unsuccessful response');
      }

      return data.data || [];
    } catch (error) {
      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new Error('Request timeout - please try again');
        }
        console.error('Error fetching logs:', error.message);
        throw new Error(`Failed to fetch logs: ${error.message}`);
      }
      console.error('Unknown error fetching logs:', error);
      throw new Error('Failed to fetch logs: Unknown error');
    }
  }
}