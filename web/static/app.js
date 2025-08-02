/**
 * LogViewer class for displaying RFC5424 logs with filtering and display customization
 * Features:
 * - Real-time log streaming via WebSocket
 * - RFC5424 field filtering (facility, severity, hostname, etc.)
 * - Immediate display options to show/hide specific header fields
 * - Compact mode for denser log display
 * - Structured data expansion
 * - Auto-scroll control with smart scroll detection
 * - Load-more functionality when scrolling to top
 * - Persistent display preferences
 * - Hidden filters and display options by default
 */
class LogViewer {
    constructor() {
        this.ws = null;
        this.isConnected = false;
        this.autoScroll = true;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
        this.currentFilters = {};
        this.allLogs = [];
        this.filteredLogs = [];
        this.isLoadingMore = false;
        this.hasMoreLogs = true;
        this.oldestLogId = null;

        // DOM elements
        this.logContainer = document.getElementById('logContainer');
        this.statusIndicator = document.getElementById('statusIndicator');
        this.statusText = document.getElementById('statusText');
        this.scrollIndicator = document.getElementById('scrollIndicator');
        this.scrollToBottomBtn = document.getElementById('scrollToBottom');

        // Filter elements
        this.filterToggle = document.getElementById('filterToggle');
        this.filterContent = document.getElementById('filterContent');
        this.facilityFilter = document.getElementById('facilityFilter');
        this.severityFilter = document.getElementById('severityFilter');
        this.minSeverityFilter = document.getElementById('minSeverityFilter');
        this.hostnameFilter = document.getElementById('hostnameFilter');
        this.appNameFilter = document.getElementById('appNameFilter');
        this.procIdFilter = document.getElementById('procIdFilter');
        this.msgIdFilter = document.getElementById('msgIdFilter');
        this.textFilter = document.getElementById('textFilter');
        this.applyFiltersBtn = document.getElementById('applyFilters');
        this.clearFiltersBtn = document.getElementById('clearFilters');

        // Display elements
        this.displayToggle = document.getElementById('displayToggle');
        this.displayContent = document.getElementById('displayContent');
        this.showTimestamp = document.getElementById('showTimestamp');
        this.showPriority = document.getElementById('showPriority');
        this.showFacility = document.getElementById('showFacility');
        this.showSeverity = document.getElementById('showSeverity');
        this.showHostname = document.getElementById('showHostname');
        this.showAppName = document.getElementById('showAppName');
        this.showProcId = document.getElementById('showProcId');
        this.showMsgId = document.getElementById('showMsgId');
        this.showSeparators = document.getElementById('showSeparators');
        this.compactMode = document.getElementById('compactMode');
        this.resetDisplayBtn = document.getElementById('resetDisplayOptions');

        // Display options state
        this.displayOptions = {
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

        this.init();
    }

    init() {
        this.setupEventListeners();
        this.loadDisplayOptions();
        this.connect();
        this.loadRecentLogs();
    }

    setupEventListeners() {
        // Scroll event listener for auto-scroll control and load-more
        this.logContainer.addEventListener('scroll', () => {
            this.handleScroll();
        });

        // Scroll to bottom button
        this.scrollToBottomBtn.addEventListener('click', () => {
            this.scrollToBottom();
            this.autoScroll = true;
            this.updateScrollIndicator();
        });

        // Filter toggle
        this.filterToggle.addEventListener('click', () => {
            this.toggleFilters();
        });

        // Filter controls
        this.applyFiltersBtn.addEventListener('click', () => {
            this.applyFilters();
        });

        this.clearFiltersBtn.addEventListener('click', () => {
            this.clearFilters();
        });

        // Real-time text filtering
        this.textFilter.addEventListener('input', () => {
            this.debounceFilter();
        });

        // Display toggle
        this.displayToggle.addEventListener('click', () => {
            this.toggleDisplayOptions();
        });

        // Display controls - immediate application
        this.showTimestamp.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showPriority.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showFacility.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showSeverity.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showHostname.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showAppName.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showProcId.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showMsgId.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.showSeparators.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.compactMode.addEventListener('change', () => {
            this.applyDisplayOptions();
        });

        this.resetDisplayBtn.addEventListener('click', () => {
            this.resetDisplayOptions();
        });

        // Window resize handler
        window.addEventListener('resize', () => {
            if (this.autoScroll) {
                // Delay scroll to bottom to allow layout to settle
                setTimeout(() => {
                    this.scrollToBottom();
                }, 100);
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
            this.allLogs.push(logEntry);

            // Apply current filters to new log entry
            if (this.passesFilters(logEntry)) {
                this.filteredLogs.push(logEntry);
                this.addLogEntry(logEntry, true);
            }
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
                // Store all logs
                this.allLogs = data.data;
                this.filteredLogs = [...this.allLogs];

                // Set oldest log ID for pagination
                if (this.allLogs.length > 0) {
                    this.oldestLogId = this.allLogs[0].id;
                }

                // Check if we have more logs
                this.hasMoreLogs = this.allLogs.length === 50;

                // Clear welcome message and display logs
                this.refreshLogDisplay();

                // Ensure we start at the bottom for initial load
                this.autoScroll = true;
                this.updateScrollIndicator();
                setTimeout(() => {
                    this.scrollToBottom();
                }, 100);
            }
        } catch (error) {
            console.error('Error loading recent logs:', error);
            this.showError('Failed to load recent logs: ' + error.message);
        }
    }

    async loadMoreLogs() {
        if (this.isLoadingMore || !this.hasMoreLogs) {
            return;
        }

        this.isLoadingMore = true;

        try {
            // Build query parameters
            const params = new URLSearchParams({
                limit: '50',
                offset: this.allLogs.length.toString()
            });

            const response = await fetch(`/api/logs?${params}`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();
            if (data.success && data.data && data.data.length > 0) {
                // Remember current scroll position and the element at the top
                const container = this.logContainer;
                const oldScrollHeight = container.scrollHeight;
                const oldScrollTop = container.scrollTop;

                // Add older logs to the beginning
                this.allLogs = [...data.data, ...this.allLogs];

                // Update filtered logs
                const newFilteredLogs = data.data.filter(log => this.passesFilters(log));
                this.filteredLogs = [...newFilteredLogs, ...this.filteredLogs];

                // Update oldest log ID
                if (data.data.length > 0) {
                    this.oldestLogId = data.data[0].id;
                }

                // Check if we have more logs
                this.hasMoreLogs = data.data.length === 50;

                // Refresh display
                this.refreshLogDisplay();

                // Restore scroll position to maintain user's view
                requestAnimationFrame(() => {
                    const newScrollHeight = container.scrollHeight;
                    const heightDifference = newScrollHeight - oldScrollHeight;
                    container.scrollTop = oldScrollTop + heightDifference;
                });

                console.log(`Loaded ${data.data.length} more logs`);
            } else {
                this.hasMoreLogs = false;
                console.log('No more logs to load');
            }
        } catch (error) {
            console.error('Error loading more logs:', error);
            this.showError('Failed to load more logs: ' + error.message);
        } finally {
            this.isLoadingMore = false;
        }
    }

    addLogEntry(logEntry, isNew = true) {
        const logElement = this.createLogElement(logEntry, isNew);
        this.logContainer.appendChild(logElement);

        // Auto-scroll if enabled and this is a new entry
        if (this.autoScroll && isNew) {
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

        // Format RFC5424 fields
        const priority = logEntry.priority || 0;
        const facility = this.getFacilityName(logEntry.facility || 0);
        const severity = this.getSeverityInfo(logEntry.severity || 0);
        const hostname = logEntry.hostname || '-';
        const appName = logEntry.app_name || '-';
        const procId = logEntry.proc_id || '-';
        const msgId = logEntry.msg_id || '-';
        const message = logEntry.message || '';

        // Create header with RFC5424 fields
        const headerDiv = document.createElement('div');
        headerDiv.className = 'log-entry-header';
        headerDiv.innerHTML = `
            <span class="timestamp">${this.escapeHtml(timestamp)}</span>
            <span class="field-separator">|</span>
            <span class="priority">P:${priority}</span>
            <span class="field-separator">|</span>
            <span class="facility">${this.escapeHtml(facility)}</span>
            <span class="field-separator">|</span>
            <span class="severity ${severity.class}">${this.escapeHtml(severity.name)}</span>
            <span class="field-separator">|</span>
            <span class="hostname">${this.escapeHtml(hostname)}</span>
            <span class="field-separator">|</span>
            <span class="app-name">${this.escapeHtml(appName)}</span>
            <span class="field-separator">|</span>
            <span class="proc-id">${this.escapeHtml(procId)}</span>
            <span class="field-separator">|</span>
            <span class="msg-id">${this.escapeHtml(msgId)}</span>
        `;

        // Create message div
        const messageDiv = document.createElement('div');
        messageDiv.className = 'log-entry-message';
        messageDiv.textContent = message;

        logDiv.appendChild(headerDiv);
        logDiv.appendChild(messageDiv);

        // Add structured data if present
        if (logEntry.structured_data && Object.keys(logEntry.structured_data).length > 0) {
            const structuredDataDiv = this.createStructuredDataElement(logEntry.structured_data);
            logDiv.appendChild(structuredDataDiv);
        }

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

    getFacilityName(facility) {
        const facilities = {
            0: 'Kernel', 1: 'User', 2: 'Mail', 3: 'Daemon', 4: 'Auth', 5: 'Syslog',
            6: 'LPR', 7: 'News', 8: 'UUCP', 9: 'Cron', 10: 'Authpriv', 11: 'FTP',
            16: 'Local0', 17: 'Local1', 18: 'Local2', 19: 'Local3',
            20: 'Local4', 21: 'Local5', 22: 'Local6', 23: 'Local7'
        };
        return facilities[facility] || `F${facility}`;
    }

    getSeverityInfo(severity) {
        const severities = {
            0: { name: 'EMERG', class: 'emergency' },
            1: { name: 'ALERT', class: 'alert' },
            2: { name: 'CRIT', class: 'critical' },
            3: { name: 'ERR', class: 'error' },
            4: { name: 'WARN', class: 'warning' },
            5: { name: 'NOTICE', class: 'notice' },
            6: { name: 'INFO', class: 'info' },
            7: { name: 'DEBUG', class: 'debug' }
        };
        return severities[severity] || { name: `S${severity}`, class: 'unknown' };
    }

    createStructuredDataElement(structuredData) {
        const container = document.createElement('div');
        container.className = 'log-entry-structured-data';

        const toggle = document.createElement('button');
        toggle.className = 'structured-data-toggle';
        toggle.textContent = '▶ Show Structured Data';

        const content = document.createElement('div');
        content.className = 'structured-data-content hidden';
        content.textContent = JSON.stringify(structuredData, null, 2);

        toggle.addEventListener('click', () => {
            const isHidden = content.classList.contains('hidden');
            content.classList.toggle('hidden');
            toggle.textContent = isHidden ? '▼ Hide Structured Data' : '▶ Show Structured Data';
        });

        container.appendChild(toggle);
        container.appendChild(content);

        return container;
    }

    toggleFilters() {
        const isHidden = this.filterContent.classList.contains('hidden');
        this.filterContent.classList.toggle('hidden');
        this.filterToggle.textContent = isHidden ? 'Hide Filters' : 'Show Filters';
    }

    toggleDisplayOptions() {
        const isHidden = this.displayContent.classList.contains('hidden');
        this.displayContent.classList.toggle('hidden');
        this.displayToggle.textContent = isHidden ? 'Hide Options' : 'Show Options';
    }

    applyDisplayOptions() {
        // Update display options from checkboxes
        this.displayOptions = {
            showTimestamp: this.showTimestamp.checked,
            showPriority: this.showPriority.checked,
            showFacility: this.showFacility.checked,
            showSeverity: this.showSeverity.checked,
            showHostname: this.showHostname.checked,
            showAppName: this.showAppName.checked,
            showProcId: this.showProcId.checked,
            showMsgId: this.showMsgId.checked,
            showSeparators: this.showSeparators.checked,
            compactMode: this.compactMode.checked
        };

        // Apply display options to the log container
        this.updateDisplayStyles();

        // Refresh the log display to apply changes
        this.refreshLogDisplay();

        // Save to localStorage
        localStorage.setItem('opentrail-display-options', JSON.stringify(this.displayOptions));
    }

    resetDisplayOptions() {
        // Reset to default values
        this.displayOptions = {
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

        // Update checkboxes
        this.showTimestamp.checked = true;
        this.showPriority.checked = true;
        this.showFacility.checked = true;
        this.showSeverity.checked = true;
        this.showHostname.checked = true;
        this.showAppName.checked = true;
        this.showProcId.checked = true;
        this.showMsgId.checked = true;
        this.showSeparators.checked = true;
        this.compactMode.checked = false;

        // Apply changes
        this.applyDisplayOptions();
    }

    loadDisplayOptions() {
        // Load from localStorage
        const saved = localStorage.getItem('opentrail-display-options');
        if (saved) {
            try {
                this.displayOptions = { ...this.displayOptions, ...JSON.parse(saved) };
            } catch (error) {
                console.warn('Failed to load display options from localStorage:', error);
            }
        }

        // Update checkboxes to match loaded options
        this.showTimestamp.checked = this.displayOptions.showTimestamp;
        this.showPriority.checked = this.displayOptions.showPriority;
        this.showFacility.checked = this.displayOptions.showFacility;
        this.showSeverity.checked = this.displayOptions.showSeverity;
        this.showHostname.checked = this.displayOptions.showHostname;
        this.showAppName.checked = this.displayOptions.showAppName;
        this.showProcId.checked = this.displayOptions.showProcId;
        this.showMsgId.checked = this.displayOptions.showMsgId;
        this.showSeparators.checked = this.displayOptions.showSeparators;
        this.compactMode.checked = this.displayOptions.compactMode;

        // Apply the loaded options
        this.updateDisplayStyles();
    }

    updateDisplayStyles() {
        // Apply compact mode
        if (this.displayOptions.compactMode) {
            this.logContainer.classList.add('compact-mode');
        } else {
            this.logContainer.classList.remove('compact-mode');
        }

        // Create CSS rules for field visibility
        let styleId = 'display-options-style';
        let existingStyle = document.getElementById(styleId);
        if (existingStyle) {
            existingStyle.remove();
        }

        const style = document.createElement('style');
        style.id = styleId;

        let css = '';

        // Field visibility rules
        const fields = [
            { option: 'showTimestamp', class: 'timestamp' },
            { option: 'showPriority', class: 'priority' },
            { option: 'showFacility', class: 'facility' },
            { option: 'showSeverity', class: 'severity' },
            { option: 'showHostname', class: 'hostname' },
            { option: 'showAppName', class: 'app-name' },
            { option: 'showProcId', class: 'proc-id' },
            { option: 'showMsgId', class: 'msg-id' }
        ];

        fields.forEach(field => {
            if (!this.displayOptions[field.option]) {
                css += `.log-entry-header .${field.class} { display: none !important; }\n`;
            }
        });

        // Separator visibility
        if (!this.displayOptions.showSeparators) {
            css += '.log-entry-header .field-separator { display: none !important; }\n';
        }

        style.textContent = css;
        document.head.appendChild(style);
    }

    applyFilters() {
        this.currentFilters = {
            facility: this.facilityFilter.value ? parseInt(this.facilityFilter.value) : null,
            severity: this.severityFilter.value ? parseInt(this.severityFilter.value) : null,
            minSeverity: this.minSeverityFilter.value ? parseInt(this.minSeverityFilter.value) : null,
            hostname: this.hostnameFilter.value.trim(),
            appName: this.appNameFilter.value.trim(),
            procId: this.procIdFilter.value.trim(),
            msgId: this.msgIdFilter.value.trim(),
            text: this.textFilter.value.trim()
        };

        this.filterLogs();
    }

    clearFilters() {
        this.facilityFilter.value = '';
        this.severityFilter.value = '';
        this.minSeverityFilter.value = '';
        this.hostnameFilter.value = '';
        this.appNameFilter.value = '';
        this.procIdFilter.value = '';
        this.msgIdFilter.value = '';
        this.textFilter.value = '';

        this.currentFilters = {};
        this.filterLogs();
    }

    debounceFilter() {
        clearTimeout(this.filterTimeout);
        this.filterTimeout = setTimeout(() => {
            this.applyFilters();
        }, 300);
    }

    passesFilters(logEntry) {
        if (!this.currentFilters || Object.keys(this.currentFilters).length === 0) {
            return true;
        }

        // Facility filter
        if (this.currentFilters.facility !== null && logEntry.facility !== this.currentFilters.facility) {
            return false;
        }

        // Severity filter (exact match)
        if (this.currentFilters.severity !== null && logEntry.severity !== this.currentFilters.severity) {
            return false;
        }

        // Minimum severity filter (severity <= minSeverity, lower numbers are more severe)
        if (this.currentFilters.minSeverity !== null && logEntry.severity > this.currentFilters.minSeverity) {
            return false;
        }

        // Hostname filter
        if (this.currentFilters.hostname && !logEntry.hostname.toLowerCase().includes(this.currentFilters.hostname.toLowerCase())) {
            return false;
        }

        // App name filter
        if (this.currentFilters.appName && !logEntry.app_name.toLowerCase().includes(this.currentFilters.appName.toLowerCase())) {
            return false;
        }

        // Process ID filter
        if (this.currentFilters.procId && !logEntry.proc_id.toLowerCase().includes(this.currentFilters.procId.toLowerCase())) {
            return false;
        }

        // Message ID filter
        if (this.currentFilters.msgId && !logEntry.msg_id.toLowerCase().includes(this.currentFilters.msgId.toLowerCase())) {
            return false;
        }

        // Text filter (search in message)
        if (this.currentFilters.text && !logEntry.message.toLowerCase().includes(this.currentFilters.text.toLowerCase())) {
            return false;
        }

        return true;
    }

    filterLogs() {
        this.filteredLogs = this.allLogs.filter(log => this.passesFilters(log));
        this.refreshLogDisplay();
    }

    refreshLogDisplay() {
        // Remember if we should maintain scroll position
        const wasAutoScrolling = this.autoScroll;

        this.logContainer.innerHTML = '';

        // Add loading indicator if loading more logs
        if (this.isLoadingMore) {
            const loadingDiv = document.createElement('div');
            loadingDiv.className = 'loading-more';
            loadingDiv.innerHTML = `
                <div class="log-entry">
                    <div class="log-entry-header">
                        <span class="timestamp">${this.formatTimestamp(new Date())}</span>
                        <span class="field-separator">|</span>
                        <span class="severity info">INFO</span>
                        <span class="field-separator">|</span>
                        <span class="app-name">system</span>
                    </div>
                    <div class="log-entry-message">Loading more logs...</div>
                </div>
            `;
            this.logContainer.appendChild(loadingDiv);
        }

        if (this.filteredLogs.length === 0) {
            const emptyMessage = document.createElement('div');
            emptyMessage.className = 'welcome-message';
            emptyMessage.innerHTML = `
                <div class="log-entry">
                    <div class="log-entry-header">
                        <span class="timestamp">${this.formatTimestamp(new Date())}</span>
                        <span class="field-separator">|</span>
                        <span class="severity info">INFO</span>
                        <span class="field-separator">|</span>
                        <span class="app-name">system</span>
                    </div>
                    <div class="log-entry-message">
                        ${this.allLogs.length === 0 ?
                    'OpenTrail RFC5424 log viewer initialized. Waiting for log entries...' :
                    'No logs match the current filters. Try adjusting your filter criteria.'}
                    </div>
                </div>
            `;
            this.logContainer.appendChild(emptyMessage);
        } else {
            this.filteredLogs.forEach(logEntry => {
                this.addLogEntry(logEntry, false);
            });
        }

        // Only auto-scroll if we were auto-scrolling before refresh
        if (wasAutoScrolling) {
            this.scrollToBottom();
        }
    }

    handleScroll() {
        const container = this.logContainer;
        const scrollTop = container.scrollTop;
        const scrollHeight = container.scrollHeight;
        const clientHeight = container.clientHeight;

        // Check if we're at the bottom (with small tolerance for rounding)
        const isAtBottom = scrollHeight - scrollTop - clientHeight < 5;
        const isAtTop = scrollTop < 50;

        // Update auto-scroll state based on user interaction
        if (isAtBottom && !this.autoScroll) {
            this.autoScroll = true;
            this.updateScrollIndicator();
        } else if (!isAtBottom && this.autoScroll) {
            this.autoScroll = false;
            this.updateScrollIndicator();
        }

        // Load more logs when scrolled near the top
        if (isAtTop && !this.isLoadingMore && this.hasMoreLogs && this.allLogs.length > 0) {
            this.loadMoreLogs();
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
        // Use requestAnimationFrame to ensure DOM is updated
        requestAnimationFrame(() => {
            this.logContainer.scrollTop = this.logContainer.scrollHeight;
        });
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