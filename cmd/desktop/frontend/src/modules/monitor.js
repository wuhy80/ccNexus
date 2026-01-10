import { t } from '../i18n/index.js';

// State
let activeRequests = new Map(); // requestId -> request data
let endpointMetrics = new Map(); // endpointName -> metrics
let durationUpdateInterval = null;
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

    // Load initial snapshot
    loadMonitorSnapshot();
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
            }
            break;

        case 'metrics_updated':
            if (event.metrics) {
                endpointMetrics.set(event.metrics.endpointName, event.metrics);
                renderEndpointMetrics();
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

// Render endpoint metrics section
function renderEndpointMetrics() {
    const container = document.getElementById('endpointMetricsList');
    if (!container) return;

    if (endpointMetrics.size === 0) {
        container.innerHTML = `
            <div class="monitor-empty">
                <span class="monitor-empty-text">${t('monitor.noMetrics')}</span>
            </div>
        `;
        return;
    }

    let html = '';

    endpointMetrics.forEach((metric, endpointName) => {
        const avgTime = metric.avgResponseTime > 0 ? formatDuration(metric.avgResponseTime) : '-';
        const successRate = metric.totalRequests > 0 ? metric.successRate.toFixed(1) + '%' : '-';

        html += `
            <div class="endpoint-metric-item">
                <div class="metric-header">
                    <span class="metric-endpoint">${escapeHtml(endpointName)}</span>
                    ${metric.activeCount > 0 ? `<span class="metric-active">${metric.activeCount} ${t('monitor.active')}</span>` : ''}
                </div>
                <div class="metric-details">
                    <span class="metric-avg-time" title="${t('monitor.avgResponseTime')}">${avgTime}</span>
                    <span class="metric-success-rate" title="${t('monitor.successRate')}">${successRate}</span>
                    ${metric.lastError ? `<span class="metric-error" title="${escapeHtml(metric.lastError)}">!</span>` : ''}
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
}

// Toggle endpoint metrics panel visibility
export function toggleMetricsPanel() {
    const panel = document.getElementById('endpointMetricsPanel');
    const toggle = document.getElementById('metricsToggle');
    if (panel && toggle) {
        panel.classList.toggle('collapsed');
        toggle.classList.toggle('collapsed');
    }
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
