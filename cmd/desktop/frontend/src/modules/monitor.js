import { t } from '../i18n/index.js';

// State
let activeRequests = new Map(); // requestId -> request data
let endpointMetrics = new Map(); // endpointName -> metrics
let recentRequests = [];           // Recent completed requests
let endpointHealth = new Map();    // endpointName -> health status
let healthCheckLatencies = {};     // endpointName -> latency in ms (from health checks)
let throughputStats = {            // Throughput statistics
    requestsPerMin: 0,
    tokensPerMin: 0,
    recentCompletions: [],          // Rolling window for last 1 minute
    globalAvgLatencyMs: 0           // Global average latency in milliseconds
};
let durationUpdateInterval = null;
let throughputUpdateInterval = null;
let isMonitorVisible = true;

// Phase icons and labels
const phaseConfig = {
    waiting: { icon: '⏳', labelKey: 'monitor.phase.waiting' },
    connecting: { icon: '🔗', labelKey: 'monitor.phase.connecting' },
    sending: { icon: '📤', labelKey: 'monitor.phase.sending' },
    streaming: { icon: '📥', labelKey: 'monitor.phase.streaming' },
    completed: { icon: '✓', labelKey: 'monitor.phase.completed' },
    failed: { icon: '✗', labelKey: 'monitor.phase.failed' }
};

// Initialize monitor module
export function initMonitor() {
    // Listen for monitor events from backend
    if (window.runtime) {
        window.runtime.EventsOn('monitor:event', handleMonitorEvent);
    }

    // Start duration update timer
    startDurationUpdates();

    // Start throughput update timer
    startThroughputUpdates();

    // Load initial data
    loadMonitorSnapshot();
    loadRecentRequests();
    loadEndpointHealth();
}

// Handle monitor events from backend
function handleMonitorEvent(event) {
    if (!event) return;

    switch (event.type) {
        case 'request_started':
            if (event.request) {
                activeRequests.set(event.request.requestId, event.request);
                renderActiveRequests();
            }
            break;

        case 'request_updated':
            if (event.request) {
                activeRequests.set(event.request.requestId, event.request);
                renderActiveRequests();
            }
            break;

        case 'request_completed':
            if (event.request) {
                activeRequests.delete(event.request.requestId);
                renderActiveRequests();

                // Add to recent requests history
                addToRecentRequests(event.request);

                // Update throughput statistics
                updateThroughputOnCompletion(event.request);
            }
            break;

        case 'metrics_updated':
            if (event.metrics) {
                endpointMetrics.set(event.metrics.endpointName, event.metrics);
                renderEndpointMetrics();

                // Update endpoint health status
                updateEndpointHealthFromMetrics(event.metrics);
            }
            break;
    }
}

// Load initial monitor snapshot
async function loadMonitorSnapshot() {
    try {
        const snapshotStr = await window.go.main.App.GetMonitorSnapshot();
        const snapshot = JSON.parse(snapshotStr);

        // Update active requests
        activeRequests.clear();
        if (snapshot.activeRequests) {
            for (const req of snapshot.activeRequests) {
                activeRequests.set(req.requestId, req);
            }
        }

        // Update endpoint metrics
        endpointMetrics.clear();
        if (snapshot.endpointMetrics) {
            for (const metric of snapshot.endpointMetrics) {
                endpointMetrics.set(metric.endpointName, metric);
            }
        }

        // Update global average latency (prefer health check latency)
        if (snapshot.healthCheckAvgLatencyMs !== undefined && snapshot.healthCheckAvgLatencyMs > 0) {
            throughputStats.globalAvgLatencyMs = snapshot.healthCheckAvgLatencyMs;
            updateAvgLatencyDisplay();
        } else if (snapshot.globalAvgLatencyMs !== undefined) {
            throughputStats.globalAvgLatencyMs = snapshot.globalAvgLatencyMs;
            updateAvgLatencyDisplay();
        }

        // Update health check latencies per endpoint
        if (snapshot.healthCheckLatencies) {
            healthCheckLatencies = snapshot.healthCheckLatencies;
        }

        renderActiveRequests();
        renderEndpointMetrics();
    } catch (error) {
        console.error('Failed to load monitor snapshot:', error);
    }
}

// Start periodic duration updates
function startDurationUpdates() {
    if (durationUpdateInterval) {
        clearInterval(durationUpdateInterval);
    }

    durationUpdateInterval = setInterval(() => {
        if (activeRequests.size > 0 && isMonitorVisible) {
            updateDurations();
        }
    }, 100); // Update every 100ms for smooth display
}

// Update duration displays without full re-render
function updateDurations() {
    const now = Date.now();
    activeRequests.forEach((req, requestId) => {
        const durationEl = document.querySelector(`[data-request-id="${requestId}"] .request-duration`);
        if (durationEl) {
            const startTime = new Date(req.startTime).getTime();
            const duration = (now - startTime) / 1000;
            durationEl.textContent = formatDuration(duration);
            durationEl.className = 'request-duration ' + getDurationClass(duration);
        }
    });
}

// Render active requests section
function renderActiveRequests() {
    const container = document.getElementById('activeRequestsList');
    if (!container) return;

    if (activeRequests.size === 0) {
        container.innerHTML = `
            <div class="monitor-empty">
                <span class="monitor-empty-icon">✓</span>
                <span class="monitor-empty-text">${t('monitor.idle')}</span>
            </div>
        `;
        return;
    }

    const now = Date.now();
    let html = '';

    activeRequests.forEach((req, requestId) => {
        const startTime = new Date(req.startTime).getTime();
        const duration = (now - startTime) / 1000;
        const phase = phaseConfig[req.phase] || phaseConfig.connecting;
        const durationClass = getDurationClass(duration);

        html += `
            <div class="active-request-item" data-request-id="${requestId}">
                <div class="request-header">
                    <span class="request-endpoint">${escapeHtml(req.endpointName)}</span>
                    <span class="request-duration ${durationClass}">${formatDuration(duration)}</span>
                </div>
                ${req.messagePreview ? `<div class="request-message">${escapeHtml(req.messagePreview)}</div>` : ''}
                <div class="request-details">
                    <span class="request-phase">${phase.icon} ${t(phase.labelKey)}</span>
                    <span class="request-model">${escapeHtml(req.model || '-')}</span>
                    <span class="request-client">${escapeHtml(req.clientType)}</span>
                    ${req.bytesReceived > 0 ? `<span class="request-bytes">${formatBytes(req.bytesReceived)}</span>` : ''}
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
}

// Render endpoint metrics section (as stat cards in the statistics area)
function renderEndpointMetrics() {
    const container = document.getElementById('endpointMetricsGrid');
    if (!container) return;

    if (endpointMetrics.size === 0) {
        container.innerHTML = `
            <div class="stat-box-compact stat-box-condensed" style="opacity: 0.5;">
                <div class="stat-info">
                    <div class="stat-label">${t('monitor.noMetrics')}</div>
                </div>
            </div>
        `;
        return;
    }

    let html = '';

    endpointMetrics.forEach((metric, endpointName) => {
        // Prefer health check latency if available, otherwise use request avgResponseTime
        let avgTime = '-';
        if (healthCheckLatencies[endpointName] !== undefined && healthCheckLatencies[endpointName] > 0) {
            avgTime = formatLatency(healthCheckLatencies[endpointName]);
        } else if (metric.avgResponseTime > 0) {
            avgTime = formatDuration(metric.avgResponseTime);
        }
        const successRate = metric.totalRequests > 0 ? metric.successRate.toFixed(1) + '%' : '-';
        const hasError = metric.lastError ? true : false;

        html += `
            <div class="stat-box-compact stat-box-condensed${hasError ? ' has-error' : ''}">
                <div class="stat-info">
                    <div class="stat-label">${escapeHtml(endpointName)}</div>
                    <div class="stat-detail">
                        <span>${successRate}</span>
                        <span class="stat-text"> ${t('monitor.successRate')}</span>
                        ${metric.activeCount > 0 ? `<span class="stat-divider">/</span><span>${metric.activeCount}</span><span class="stat-text"> ${t('monitor.active')}</span>` : ''}
                    </div>
                </div>
                <div class="stat-value">
                    <span class="stat-primary">${avgTime}</span>
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
}

// Reset all metrics
export async function resetMetrics() {
    try {
        await window.go.main.App.ResetMonitorMetrics();
        endpointMetrics.clear();
        renderEndpointMetrics();
    } catch (error) {
        console.error('Failed to reset metrics:', error);
    }
}

// Set monitor visibility (for performance optimization)
export function setMonitorVisible(visible) {
    isMonitorVisible = visible;
}

// Helper functions

function formatDuration(seconds) {
    if (seconds < 1) {
        return (seconds * 1000).toFixed(0) + 'ms';
    } else if (seconds < 60) {
        return seconds.toFixed(1) + 's';
    } else {
        const mins = Math.floor(seconds / 60);
        const secs = (seconds % 60).toFixed(0);
        return `${mins}m ${secs}s`;
    }
}

function getDurationClass(seconds) {
    if (seconds < 5) return 'duration-normal';
    if (seconds < 10) return 'duration-slow';
    if (seconds < 30) return 'duration-warning';
    return 'duration-critical';
}

function formatBytes(bytes) {
    if (bytes < 1024) return bytes + 'B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + 'KB';
    return (bytes / (1024 * 1024)).toFixed(1) + 'MB';
}

function escapeHtml(str) {
    if (!str) return '';
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

// Get active request count (for external use)
export function getActiveRequestCount() {
    return activeRequests.size;
}

// Get endpoint metrics (for external use)
export function getEndpointMetricsData() {
    return Object.fromEntries(endpointMetrics);
}

// ========== New Functions for Enhanced Monitoring ==========

// Load recent requests from backend
async function loadRecentRequests() {
    try {
        const resultStr = await window.go.main.App.GetRecentRequests(10);
        const result = JSON.parse(resultStr);

        if (result.requests && Array.isArray(result.requests)) {
            recentRequests = result.requests.map(req => ({
                requestId: req.requestId,
                endpointName: req.endpointName,
                model: req.model,
                completedAt: new Date(req.timestamp),
                duration: req.durationMs / 1000,
                inputTokens: req.inputTokens + req.cacheCreationTokens + req.cacheReadTokens,
                outputTokens: req.outputTokens,
                success: req.success
            }));
            renderRecentRequests();
        }
    } catch (error) {
        console.error('Failed to load recent requests:', error);
    }
}

// Load endpoint health status from backend
async function loadEndpointHealth() {
    try {
        const resultStr = await window.go.main.App.GetEndpointHealth();
        const healthList = JSON.parse(resultStr);

        endpointHealth.clear();
        if (Array.isArray(healthList)) {
            for (const h of healthList) {
                endpointHealth.set(h.endpointName, h);
            }
        }

        // Also refresh health check latencies
        try {
            const snapshotStr = await window.go.main.App.GetMonitorSnapshot();
            const snapshot = JSON.parse(snapshotStr);
            if (snapshot.healthCheckLatencies) {
            healthCheckLatencies = snapshot.healthCheckLatencies;
            }
        } catch (e) {
            // Ignore
        }

        renderEndpointHealth();
    } catch (error) {
        console.error('Failed to load endpoint health:', error);
    }
}

// Add completed request to recent history
function addToRecentRequests(request) {
    const completedRequest = {
        requestId: request.requestId,
        endpointName: request.endpointName,
        model: request.model,
        completedAt: new Date(),
        duration: (Date.now() - new Date(request.startTime).getTime()) / 1000,
        inputTokens: 0, // Will be updated from stats if available
        outputTokens: 0,
        success: request.phase === 'completed',
        messagePreview: request.messagePreview || ''
    };

    recentRequests.unshift(completedRequest);

    // Keep max 10 items
    if (recentRequests.length > 10) {
        recentRequests = recentRequests.slice(0, 10);
    }

    renderRecentRequests();
}

// Update throughput statistics on request completion
function updateThroughputOnCompletion(request) {
    const now = Date.now();
    const completion = {
        timestamp: now,
        tokens: request.bytesReceived || 0
    };

    throughputStats.recentCompletions.push(completion);

    // Clean up entries older than 1 minute
    const oneMinuteAgo = now - 60000;
    throughputStats.recentCompletions = throughputStats.recentCompletions.filter(
        c => c.timestamp > oneMinuteAgo
    );
}

// Update endpoint health from metrics event
function updateEndpointHealthFromMetrics(metrics) {
    const health = {
        endpointName: metrics.endpointName,
        status: calculateHealthStatusFromMetrics(metrics),
        activeCount: metrics.activeCount,
        successRate: metrics.successRate,
        avgResponseTime: metrics.avgResponseTime,
        lastError: metrics.lastError,
        lastErrorTime: metrics.lastErrorTime
    };

    endpointHealth.set(metrics.endpointName, health);
    renderEndpointHealth();
}

// Calculate health status from metrics
function calculateHealthStatusFromMetrics(metrics) {
    // Check for recent errors (within 5 minutes)
    if (metrics.lastErrorTime > 0) {
        const fiveMinutesAgo = Date.now() - 5 * 60 * 1000;
        if (metrics.lastErrorTime > fiveMinutesAgo) {
            return 'error';
        }
    }

    // Check success rate
    if (metrics.totalRequests > 0) {
        if (metrics.successRate < 80) {
            return 'error';
        }
        if (metrics.successRate < 95) {
            return 'warning';
        }
    }

    return 'healthy';
}

// Start periodic throughput updates
function startThroughputUpdates() {
    if (throughputUpdateInterval) {
        clearInterval(throughputUpdateInterval);
    }

    throughputUpdateInterval = setInterval(() => {
        if (isMonitorVisible) {
            calculateAndDisplayThroughput();
        }
    }, 5000); // Update every 5 seconds
}

// Calculate and display throughput
async function calculateAndDisplayThroughput() {
    const now = Date.now();
    const oneMinuteAgo = now - 60000;

    // Clean up expired entries
    throughputStats.recentCompletions = throughputStats.recentCompletions.filter(
        c => c.timestamp > oneMinuteAgo
    );

    // Calculate rates
    throughputStats.requestsPerMin = throughputStats.recentCompletions.length;
    throughputStats.tokensPerMin = throughputStats.recentCompletions.reduce(
        (sum, c) => sum + c.tokens, 0
    );

    // Refresh global average latency from backend
    try {
        const snapshotStr = await window.go.main.App.GetMonitorSnapshot();
        const snapshot = JSON.parse(snapshotStr);
        // Prefer health check latency if available, otherwise use request latency
        if (snapshot.healthCheckAvgLatencyMs !== undefined && snapshot.healthCheckAvgLatencyMs > 0) {
            throughputStats.globalAvgLatencyMs = snapshot.healthCheckAvgLatencyMs;
        } else if (snapshot.globalAvgLatencyMs !== undefined) {
            throughputStats.globalAvgLatencyMs = snapshot.globalAvgLatencyMs;
        }
        // Update health check latencies per endpoint
        if (snapshot.healthCheckLatencies) {
            healthCheckLatencies = snapshot.healthCheckLatencies;
            renderEndpointMetrics();
        }
    } catch (error) {
        // Ignore errors, keep existing value
    }

    updateThroughputDisplay();
    updateAvgLatencyDisplay();
}

// Update throughput display in UI
function updateThroughputDisplay() {
    const reqEl = document.getElementById('requestsPerMin');
    const tokenEl = document.getElementById('tokensPerMin');

    if (reqEl) {
        reqEl.textContent = throughputStats.requestsPerMin;
    }
    if (tokenEl) {
        tokenEl.textContent = formatNumber(throughputStats.tokensPerMin);
    }
}

// Update average latency display in UI
function updateAvgLatencyDisplay() {
    const latencyEl = document.getElementById('avgLatency');
    if (latencyEl) {
        if (throughputStats.globalAvgLatencyMs > 0) {
            latencyEl.textContent = formatLatency(throughputStats.globalAvgLatencyMs);
        } else {
            latencyEl.textContent = '-';
        }
    }
}

// Format latency for display
function formatLatency(ms) {
    if (ms < 1000) {
        return Math.round(ms) + 'ms';
    } else if (ms < 60000) {
        return (ms / 1000).toFixed(1) + 's';
    } else {
        const mins = Math.floor(ms / 60000);
        const secs = ((ms % 60000) / 1000).toFixed(0);
        return `${mins}m ${secs}s`;
    }
}

// Render recent requests list
function renderRecentRequests() {
    const container = document.getElementById('recentRequestsList');
    if (!container) return;

    if (recentRequests.length === 0) {
        container.innerHTML = `
            <div class="monitor-empty">
                <span class="monitor-empty-icon">📭</span>
                <span class="monitor-empty-text">${t('monitor.noRecentRequests')}</span>
            </div>
        `;
        return;
    }

    let html = '';
    for (const req of recentRequests) {
        const statusClass = req.success ? 'status-success' : 'status-failed';
        const statusIcon = req.success ? '✓' : '✗';
        const time = req.completedAt instanceof Date
            ? req.completedAt.toLocaleTimeString('zh-CN', { hour12: false })
            : new Date(req.completedAt).toLocaleTimeString('zh-CN', { hour12: false });

        html += `
            <div class="recent-request-item ${statusClass}">
                <div class="recent-request-time">${time}</div>
                <div class="recent-request-info">
                    <span class="recent-endpoint">${escapeHtml(req.endpointName)}</span>
                    ${req.messagePreview ? `<span class="recent-message" title="${escapeHtml(req.messagePreview)}">${escapeHtml(req.messagePreview)}</span>` : `<span class="recent-model">${escapeHtml(req.model || '-')}</span>`}
                </div>
                <div class="recent-request-stats">
                    <span class="recent-duration">${formatDuration(req.duration)}</span>
                    <span class="recent-tokens">${formatTokens(req.inputTokens)} / ${formatTokens(req.outputTokens)}</span>
                </div>
                <div class="recent-request-status">
                    <span class="status-badge ${req.success ? 'success' : 'failed'}">${statusIcon}</span>
                </div>
            </div>
        `;
    }

    container.innerHTML = html;
}

// Render endpoint health status
function renderEndpointHealth() {
    const container = document.getElementById('endpointHealthList');
    if (!container) return;

    if (endpointHealth.size === 0) {
        container.innerHTML = `
            <div class="monitor-empty">
                <span class="monitor-empty-icon">📡</span>
                <span class="monitor-empty-text">${t('monitor.noEndpoints')}</span>
            </div>
        `;
        return;
    }

    let html = '';
    endpointHealth.forEach((health, name) => {
        const statusClass = `status-${health.status}`;
        // Prefer health check latency if available
        let avgTime = '-';
        if (healthCheckLatencies[name] !== undefined && healthCheckLatencies[name] > 0) {
            avgTime = formatLatency(healthCheckLatencies[name]);
        } else if (health.avgResponseTime > 0) {
            avgTime = formatDuration(health.avgResponseTime);
        }
        const successRate = health.successRate > 0
            ? health.successRate.toFixed(1) + '%'
            : '-';

        html += `
            <div class="endpoint-health-item ${statusClass}">
                <div class="health-status-indicator"></div>
                <div class="health-info">
                    <span class="health-endpoint-name">${escapeHtml(name)}</span>
                    <span class="health-stats">
                        ${health.activeCount > 0 ? `<span class="health-active">${health.activeCount} ${t('monitor.active')}</span><span class="health-divider">|</span>` : ''}
                        <span>${successRate}</span>
                        <span class="health-divider">|</span>
                        <span>${avgTime}</span>
                    </span>
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
}

// Helper: Format tokens for display
function formatTokens(tokens) {
    return formatNumber(tokens);
}

// Helper: Format number for display
function formatNumber(num) {
    if (!num || num === 0) return '0';
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}
