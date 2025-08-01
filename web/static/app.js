class LogViewer {
    constructor() {
        this.ws = null;
        this.isConnected = false;
        this.autoScroll = true;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
        
        // DOM elements
        this.logContainer = document.getElementById('logContainer');
        this.statusIndicator = document.getElementById('statusIndicator');
        this.statusText = document.getElementById('statusText');
        this.scrollIndicator = document.getElementById('scrollIndicator');
        this.scrollToBottomBtn = document.getElementById('scrollToBottom');
        
        this.init();
    }
    
    init() {
        this.setupEventListeners();
        this.connect();
        this.loadRecentLogs();
    }
    
    setupEventListeners() {
        // Scroll event listener for auto-scroll control
        this.logContainer.addEventListener('scroll', () => {
            this.handleScroll();
        });
        
        // Scroll to bottom button
        this.scrollToBottomBtn.addEventListener('click', () => {
            this.scrollToBottom();
            this.autoScroll = true;
            this.updateScrollIndicator();
        });
        
        // Window resize handler
        window.addEventListener('resize', () => {
            if (this.autoScroll) {
                this.scrollToBottom();
            }
        });
        
        // Page visibility change handler
        document.addEventListener('visibilitychange', () => {
            if (!document.hidden && !this.isConnected) {
                this.connect();
            }
        });
    }
    
    connect() {
        if (this.ws && this.ws.readyState === WebSocket.CONNECTING) {
            return;
        }
        
        this.updateConnectionStatus('connecting', 'Connecting...');
        
        // Determine WebSocket URL
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/api/logs/stream`;
        
        try {
            this.ws = new WebSocket(wsUrl);
            
            this.ws.onopen = () => {
                this.onConnect();
            };
            
            this.ws.onmessage = (event) => {
                this.onMessage(event);
            };
            
            this.ws.onclose = (event) => {
                this.onDisconnect(event);
            };
            
            this.ws.onerror = (error) => {
                this.onError(error);
            };
            
        } catch (error) {
            console.error('WebSocket connection error:', error);
            this.onError(error);
        }
    }
    
    onConnect() {
        console.log('WebSocket connected');
        this.isConnected = true;
        this.reconnectAttempts = 0;
        this.updateConnectionStatus('connected', 'Connected');
        
        // Remove welcome message if it exists
        const welcomeMessage = this.logContainer.querySelector('.welcome-message');
        if (welcomeMessage) {
            welcomeMessage.remove();
        }
    }
    
    onMessage(event) {
        try {
            const logEntry = JSON.parse(event.data);
            this.addLogEntry(logEntry);
        } catch (error) {
            console.error('Error parsing log message:', error);
        }
    }
    
    onDisconnect(event) {
        console.log('WebSocket disconnected:', event.code, event.reason);
        this.isConnected = false;
        this.updateConnectionStatus('disconnected', 'Disconnected');
        
        // Attempt to reconnect if not a normal closure
        if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
            this.scheduleReconnect();
        }
    }
    
    onError(error) {
        console.error('WebSocket error:', error);
        this.isConnected = false;
        this.updateConnectionStatus('error', 'Connection error');
    }
    
    scheduleReconnect() {
        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
        
        this.updateConnectionStatus('connecting', `Reconnecting in ${Math.ceil(delay / 1000)}s...`);
        
        setTimeout(() => {
            if (!this.isConnected) {
                this.connect();
            }
        }, delay);
    }
    
    updateConnectionStatus(status, text) {
        this.statusIndicator.className = `status-indicator ${status}`;
        this.statusText.textContent = text;
    }
    
    async loadRecentLogs() {
        try {
            const response = await fetch('/api/logs?limit=50');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const data = await response.json();
            if (data.success && data.data) {
                // Clear welcome message
                this.logContainer.innerHTML = '';
                
                // Add recent logs
                data.data.forEach(logEntry => {
                    this.addLogEntry(logEntry, false);
                });
                
                // Scroll to bottom
                this.scrollToBottom();
            }
        } catch (error) {
            console.error('Error loading recent logs:', error);
            this.showError('Failed to load recent logs: ' + error.message);
        }
    }
    
    addLogEntry(logEntry, isNew = true) {
        const logElement = this.createLogElement(logEntry, isNew);
        this.logContainer.appendChild(logElement);
        
        // Auto-scroll if enabled
        if (this.autoScroll) {
            this.scrollToBottom();
        }
        
        // Limit the number of log entries to prevent memory issues
        this.limitLogEntries();
    }
    
    createLogElement(logEntry, isNew = false) {
        const logDiv = document.createElement('div');
        logDiv.className = `log-entry${isNew ? ' new' : ''}`;
        
        // Format timestamp
        const timestamp = this.formatTimestamp(logEntry.timestamp);
        
        // Format level
        const level = (logEntry.level || 'unknown').toLowerCase();
        
        // Format tracking ID
        const trackingId = logEntry.tracking_id || '-';
        
        // Format message
        const message = logEntry.message || '';
        
        logDiv.innerHTML = `
            <span class="timestamp">${this.escapeHtml(timestamp)}</span>
            <span class="level ${level}">${this.escapeHtml(level.toUpperCase())}</span>
            <span class="tracking-id">${this.escapeHtml(trackingId)}</span>
            <span class="message">${this.escapeHtml(message)}</span>
        `;
        
        return logDiv;
    }
    
    formatTimestamp(timestamp) {
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
    }
    
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    
    handleScroll() {
        const container = this.logContainer;
        const isAtBottom = container.scrollHeight - container.scrollTop <= container.clientHeight + 10;
        
        if (isAtBottom && !this.autoScroll) {
            this.autoScroll = true;
            this.updateScrollIndicator();
        } else if (!isAtBottom && this.autoScroll) {
            this.autoScroll = false;
            this.updateScrollIndicator();
        }
    }
    
    updateScrollIndicator() {
        if (this.autoScroll) {
            this.scrollIndicator.classList.remove('visible');
        } else {
            this.scrollIndicator.classList.add('visible');
        }
    }
    
    scrollToBottom() {
        this.logContainer.scrollTop = this.logContainer.scrollHeight;
    }
    
    limitLogEntries(maxEntries = 1000) {
        const entries = this.logContainer.querySelectorAll('.log-entry');
        if (entries.length > maxEntries) {
            const entriesToRemove = entries.length - maxEntries;
            for (let i = 0; i < entriesToRemove; i++) {
                entries[i].remove();
            }
        }
    }
    
    showError(message) {
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-message';
        errorDiv.textContent = message;
        
        // Remove existing error messages
        const existingErrors = this.logContainer.querySelectorAll('.error-message');
        existingErrors.forEach(error => error.remove());
        
        this.logContainer.appendChild(errorDiv);
        
        if (this.autoScroll) {
            this.scrollToBottom();
        }
    }
    
    // Public methods for external control
    reconnect() {
        if (this.ws) {
            this.ws.close();
        }
        this.reconnectAttempts = 0;
        this.connect();
    }
    
    disconnect() {
        if (this.ws) {
            this.ws.close(1000, 'Manual disconnect');
        }
    }
    
    clearLogs() {
        this.logContainer.innerHTML = '';
    }
    
    toggleAutoScroll() {
        this.autoScroll = !this.autoScroll;
        this.updateScrollIndicator();
        
        if (this.autoScroll) {
            this.scrollToBottom();
        }
    }
}

// Utility function to get current time (used in HTML template)
function getCurrentTime() {
    return new Date().toLocaleString('en-US', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
}

// Initialize the log viewer when the page loads
document.addEventListener('DOMContentLoaded', () => {
    // Set welcome timestamp
    const welcomeTimestamp = document.getElementById('welcomeTimestamp');
    if (welcomeTimestamp) {
        welcomeTimestamp.textContent = getCurrentTime();
    }
    
    window.logViewer = new LogViewer();
});

// Handle page unload
window.addEventListener('beforeunload', () => {
    if (window.logViewer) {
        window.logViewer.disconnect();
    }
});