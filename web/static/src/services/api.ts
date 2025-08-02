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

      const response = await fetch(`/api/logs?${params}`);
      
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const data: ApiResponse<LogEntry[]> = await response.json();
      
      if (!data.success || !data.data) {
        throw new Error(data.error || 'Failed to fetch logs');
      }

      return data.data;
    } catch (error) {
      console.error('Error fetching logs:', error);
      throw error;
    }
  }
}