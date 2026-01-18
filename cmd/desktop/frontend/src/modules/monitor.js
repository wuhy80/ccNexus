import { t } from '../i18n/index.js';
import { getCurrentClientType, refreshEndpoints } from './endpoints.js';

// State
let activeRequests = new Map(); // requestId -> request data
let endpointMetrics = new Map(); // endpointName -> metrics
let recentRequests = [];           // Recent completed requests
let endpointHealth = new Map();    // endpointName -> health status
let healthCheckLatencies = {};     // endpointName -> latency in ms (from health checks)
let endpointCheckResults = new Map(); // endpointName -> {lastCheckAt, success, latencyMs, errorMessage}
let throughputStats = {            // Throughput statistics
    requestsPerMin: 0,
    tokensPerMin: 0,
    recentCompletions: [],          // Rolling window for last 1 minute
    globalAvgLatencyMs: 0           // Global average latency in milliseconds
};
let durationUpdateInterval = null;
let throughputUpdateInterval = null;
let checkTimeUpdateInterval = null; // 检测时间更新定时器
let isMonitorVisible = true;
let isTestingAllEndpoints = false; // 是否正在进行一键检测

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

    // Start check time update timer (每秒更新检测时间显示)
    startCheckTimeUpdates();

    // Load initial data
    loadMonitorSnapshot();
    loadRecentRequests();
    loadEndpointHealth();
    loadEndpointCheckResults();
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

        // 同时加载检测结果
        try {
            const checkResultStr = await window.go.main.App.GetEndpointCheckResults();
            const results = JSON.parse(checkResultStr);

            endpointCheckResults.clear();
            for (const [name, result] of Object.entries(results)) {
                endpointCheckResults.set(name, {
                    lastCheckAt: new Date(result.lastCheckAt),
                    success: result.success,
                    latencyMs: result.latencyMs,
                    errorMessage: result.errorMessage
                });
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
    // Get existing health check latency if available
    const existingHealth = endpointHealth.get(metrics.endpointName);
    const healthCheckLatency = healthCheckLatencies[metrics.endpointName] ||
                               (existingHealth ? existingHealth.healthCheckLatency : 0);

    const health = {
        endpointName: metrics.endpointName,
        status: calculateHealthStatusFromMetrics(metrics, healthCheckLatency),
        activeCount: metrics.activeCount,
        successRate: metrics.successRate,
        avgResponseTime: metrics.avgResponseTime,
        healthCheckLatency: healthCheckLatency,
        lastError: metrics.lastError,
        lastErrorTime: metrics.lastErrorTime
    };

    endpointHealth.set(metrics.endpointName, health);
    renderEndpointHealth();
}

// Calculate health status from metrics
function calculateHealthStatusFromMetrics(metrics, healthCheckLatency = 0) {
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

    // If health check latency exists, endpoint is healthy
    if (healthCheckLatency > 0) {
        return 'healthy';
    }

    // If no metrics and no health check, status is unknown
    if (!metrics.totalRequests && healthCheckLatency === 0) {
        return 'unknown';
    }

    return 'healthy';
}

// Update endpoint health status with latest health check latencies
function updateEndpointHealthWithLatencies() {
    endpointHealth.forEach((health, name) => {
        const latency = healthCheckLatencies[name];
        if (latency !== undefined && latency > 0) {
            health.healthCheckLatency = latency;
            // Update status if it was unknown
            if (health.status === 'unknown') {
                health.status = 'healthy';
            }
        }
    });
    renderEndpointHealth();
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
            // Also update endpoint health status with new latencies
            updateEndpointHealthWithLatencies();
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

    // 添加结果通知容器
    let html = `
        <div id="testResultNotification"></div>
    `;

    endpointHealth.forEach((health, name) => {
        const statusClass = `status-${health.status}`;
        // Prefer healthCheckLatency from health object, then from healthCheckLatencies map, then avgResponseTime
        let avgTime = '-';
        if (health.healthCheckLatency > 0) {
            avgTime = formatLatency(health.healthCheckLatency);
        } else if (healthCheckLatencies[name] !== undefined && healthCheckLatencies[name] > 0) {
            avgTime = formatLatency(healthCheckLatencies[name]);
        } else if (health.avgResponseTime > 0) {
            avgTime = formatDuration(health.avgResponseTime);
        }
        const successRate = health.successRate > 0
            ? health.successRate.toFixed(1) + '%'
            : '-';

        // 获取检测结果
        const checkResult = endpointCheckResults.get(name);
        let checkTimeText = t('monitor.neverChecked');
        let checkStatusIcon = '';
        let checkStatusClass = '';

        if (checkResult && checkResult.lastCheckAt) {
            const secondsAgo = Math.floor((Date.now() - checkResult.lastCheckAt.getTime()) / 1000);
            checkTimeText = formatTimeAgo(secondsAgo);
            checkStatusIcon = checkResult.success ? '✓' : '✗';
            checkStatusClass = checkResult.success ? 'check-success' : 'check-failed';
        }

        // 优先级显示
        const priority = health.priority || 100;
        const priorityClass = priority < 50 ? 'priority-high' : priority < 100 ? 'priority-medium' : 'priority-low';

        // 5分钟统计
        const recentSuccess = health.recentSuccess || 0;
        const recentFailure = health.recentFailure || 0;
        const recentTotal = recentSuccess + recentFailure;

        html += `
            <div class="endpoint-health-item ${statusClass}" data-endpoint-name="${escapeHtml(name)}">
                <div class="health-status-indicator"></div>
                <div class="health-main-info">
                    <div class="health-header">
                        <span class="health-endpoint-name">${escapeHtml(name)}</span>
                        <span class="health-priority ${priorityClass}" title="${t('monitor.priority')}: ${priority}">P${priority}</span>
                    </div>
                    <div class="health-stats-row">
                        <span class="health-stat-item">
                            ${health.activeCount > 0 ? `<span class="health-active">${health.activeCount} ${t('monitor.active')}</span>` : ''}
                            ${health.activeCount > 0 ? '<span class="health-divider">|</span>' : ''}
                            <span class="health-success-rate">${successRate}</span>
                            <span class="health-divider">|</span>
                            <span class="health-latency">${avgTime}</span>
                        </span>
                    </div>
                    ${recentTotal > 0 ? `
                    <div class="health-recent-stats">
                        <span class="recent-label">${t('monitor.recent5min')}:</span>
                        <span class="recent-success">✓ ${recentSuccess}</span>
                        <span class="recent-divider">/</span>
                        <span class="recent-failure">✗ ${recentFailure}</span>
                    </div>
                    ` : ''}
                </div>
                <div class="health-check-info ${checkStatusClass}">
                    ${checkStatusIcon ? `<span class="check-status-icon">${checkStatusIcon}</span>` : ''}
                    <span class="check-time">${checkTimeText}</span>
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

// ========== Health History Chart Functions ==========

// State for health history
let healthHistoryData = [];
let healthHistoryDataMap = new Map(); // 存储多个端点数据
let selectedHistoryEndpoint = ''; // 保留兼容性
let selectedHistoryEndpoints = []; // 选中的端点列表
let selectedHistoryHours = 24;
let syncWithStatsPeriod = true; // 是否同步统计周期
let showLatencyChart = true; // 是否显示延迟图表

// Initialize health history panel
export function initHealthHistoryPanel() {
    renderHealthHistoryPanel();

    // 监听统计周期变化事件
    window.addEventListener('statsPeriodChanged', (e) => {
        syncHealthHistoryWithStatsPeriod(e.detail.period);
    });
}

// Render the health history panel HTML
function renderHealthHistoryPanel() {
    const container = document.getElementById('healthHistoryPanel');
    if (!container) return;

    container.innerHTML = `
        <div class="health-history-panel">
            <div class="health-history-header">
                <h4><span class="section-icon">📈</span> ${t('monitor.healthHistory')}</h4>
                <div class="health-history-controls">
                    <div class="endpoint-selector-group">
                        <label class="control-label">${t('monitor.selectEndpoints')}：</label>
                        <div class="endpoint-checkboxes" id="healthHistoryEndpoints">
                            <!-- 动态生成复选框 -->
                        </div>
                        <div class="endpoint-selector-actions">
                            <button class="btn-link" onclick="window.selectAllEndpoints()">${t('monitor.selectAll')}</button>
                            <button class="btn-link" onclick="window.deselectAllEndpoints()">${t('monitor.deselectAll')}</button>
                        </div>
                    </div>
                    <div class="time-range-group">
                        <label class="control-label">
                            <input type="checkbox" id="syncWithStats" checked />
                            ${t('monitor.syncWithStats')}
                        </label>
                        <select id="healthHistoryHours">
                            <option value="6">6 ${t('monitor.hours')}</option>
                            <option value="12">12 ${t('monitor.hours')}</option>
                            <option value="24" selected>24 ${t('monitor.hours')}</option>
                            <option value="48">48 ${t('monitor.hours')}</option>
                            <option value="168">7 ${t('monitor.days')}</option>
                        </select>
                    </div>
                    <div class="latency-toggle-group">
                        <label class="control-label">
                            <input type="checkbox" id="showLatencyChart" checked />
                            ${t('monitor.showLatencyChart')}
                        </label>
                    </div>
                </div>
            </div>
            <div id="healthHistoryChart" class="health-history-chart">
                <div class="health-history-empty">
                    <span class="health-history-empty-icon">📊</span>
                    <span>${t('monitor.noEndpointSelected')}</span>
                </div>
            </div>
        </div>
    `;

    // Populate endpoint dropdown
    populateHealthHistoryEndpoints();

    // Add event listeners
    const syncCheckbox = document.getElementById('syncWithStats');
    const hoursSelect = document.getElementById('healthHistoryHours');
    const latencyCheckbox = document.getElementById('showLatencyChart');

    if (syncCheckbox) {
        syncCheckbox.addEventListener('change', (e) => {
            syncWithStatsPeriod = e.target.checked;
            if (hoursSelect) {
                hoursSelect.disabled = syncWithStatsPeriod;
            }
            if (!syncWithStatsPeriod) {
                // 手动模式，保存当前选择
                localStorage.setItem('healthHistory_syncWithStats', 'false');
            } else {
                localStorage.setItem('healthHistory_syncWithStats', 'true');
            }
        });
    }

    if (hoursSelect) {
        hoursSelect.addEventListener('change', (e) => {
            selectedHistoryHours = parseInt(e.target.value);
            loadMultipleEndpointsHealthHistory();
        });
    }

    if (latencyCheckbox) {
        latencyCheckbox.addEventListener('change', (e) => {
            showLatencyChart = e.target.checked;
            localStorage.setItem('healthHistory_showLatencyChart', showLatencyChart);
            const container = document.getElementById('latencyChartContainer');
            if (container) {
                container.style.display = showLatencyChart ? 'block' : 'none';
            }
            if (showLatencyChart && getSelectedEndpoints().length > 0) {
                renderLatencyChartForMultipleEndpoints();
            }
        });
    }

    // 恢复保存的状态
    const savedSync = localStorage.getItem('healthHistory_syncWithStats');
    if (savedSync !== null) {
        syncWithStatsPeriod = savedSync === 'true';
        if (syncCheckbox) syncCheckbox.checked = syncWithStatsPeriod;
        if (hoursSelect) hoursSelect.disabled = syncWithStatsPeriod;
    }

    const savedShowLatency = localStorage.getItem('healthHistory_showLatencyChart');
    if (savedShowLatency !== null) {
        showLatencyChart = savedShowLatency === 'true';
        if (latencyCheckbox) latencyCheckbox.checked = showLatencyChart;
    }
}

// Populate endpoint checkboxes with available endpoints
async function populateHealthHistoryEndpoints() {
    const container = document.getElementById('healthHistoryEndpoints');
    if (!container) return;

    // Get endpoints from health status
    const endpoints = Array.from(endpointHealth.keys());

    // If no endpoints from health, try to get from config
    if (endpoints.length === 0) {
        try {
            const configStr = await window.go.main.App.GetConfig();
            const config = JSON.parse(configStr);
            if (config.endpoints) {
                for (const ep of config.endpoints) {
                    if (!endpoints.includes(ep.name)) {
                        endpoints.push(ep.name);
                    }
                }
            }
        } catch (error) {
            console.error('Failed to get endpoints for health history:', error);
        }
    }

    // Build checkboxes
    let html = '';
    for (const name of endpoints) {
        const escapedName = escapeHtml(name);
        html += `
            <label class="endpoint-checkbox-item">
                <input type="checkbox" value="${escapedName}" onchange="window.handleEndpointCheckboxChange(this)" />
                <span>${escapedName}</span>
            </label>
        `;
    }

    if (html === '') {
        html = '<div class="health-history-empty-text">' + t('monitor.noEndpoints') + '</div>';
    }

    container.innerHTML = html;

    // 恢复之前保存的选中状态
    loadSelectedEndpoints();
}

// 获取选中的端点列表
function getSelectedEndpoints() {
    const checkboxes = document.querySelectorAll('#healthHistoryEndpoints input[type="checkbox"]:checked');
    return Array.from(checkboxes).map(cb => cb.value);
}

// 保存选中状态到 localStorage
function saveSelectedEndpoints() {
    const selected = getSelectedEndpoints();
    localStorage.setItem('healthHistory_selectedEndpoints', JSON.stringify(selected));
}

// 从 localStorage 恢复选中状态
function loadSelectedEndpoints() {
    try {
        const saved = localStorage.getItem('healthHistory_selectedEndpoints');
        if (saved) {
            const selectedNames = JSON.parse(saved);
            selectedHistoryEndpoints = selectedNames;

            // 设置复选框状态
            const checkboxes = document.querySelectorAll('#healthHistoryEndpoints input[type="checkbox"]');
            checkboxes.forEach(cb => {
                if (selectedNames.includes(cb.value)) {
                    cb.checked = true;
                }
            });

            // 如果有选中的端点，加载数据
            if (selectedNames.length > 0) {
                loadMultipleEndpointsHealthHistory();
            }
        }
    } catch (error) {
        console.error('Failed to load selected endpoints:', error);
    }
}

// 全选端点
window.selectAllEndpoints = function() {
    const checkboxes = document.querySelectorAll('#healthHistoryEndpoints input[type="checkbox"]');
    const maxEndpoints = 5;
    let count = 0;

    checkboxes.forEach(cb => {
        if (count < maxEndpoints) {
            cb.checked = true;
            count++;
        }
    });

    saveSelectedEndpoints();
    loadMultipleEndpointsHealthHistory();
};

// 取消全选
window.deselectAllEndpoints = function() {
    const checkboxes = document.querySelectorAll('#healthHistoryEndpoints input[type="checkbox"]');
    checkboxes.forEach(cb => {
        cb.checked = false;
    });

    saveSelectedEndpoints();
    renderHealthHistoryEmpty();
};

// 处理复选框变化
window.handleEndpointCheckboxChange = function(checkbox) {
    const selectedCount = getSelectedEndpoints().length;
    const maxEndpoints = 5;

    if (checkbox.checked && selectedCount > maxEndpoints) {
        checkbox.checked = false;
        const message = t('monitor.maxEndpointsWarning').replace('{max}', maxEndpoints);
        // 显示提示（如果有通知系统）
        if (window.showNotification) {
            window.showNotification(message, 'warning');
        } else {
            alert(message);
        }
        return;
    }

    saveSelectedEndpoints();
    loadMultipleEndpointsHealthHistory();
};

// 转换统计周期为小时数
function convertPeriodToHours(period) {
    const mapping = {
        'daily': 24,
        'yesterday': 24,
        'weekly': 168,
        'monthly': 720
    };
    return mapping[period] || 24;
}

// 同步健康历史时间范围与统计周期
function syncHealthHistoryWithStatsPeriod(period) {
    if (!syncWithStatsPeriod) return;

    const hours = convertPeriodToHours(period);
    selectedHistoryHours = hours;

    // 更新小时选择器
    const hoursSelect = document.getElementById('healthHistoryHours');
    if (hoursSelect) {
        // 如果选项中有对应的值，选中它
        const option = Array.from(hoursSelect.options).find(opt => parseInt(opt.value) === hours);
        if (option) {
            hoursSelect.value = hours;
        } else {
            // 如果没有对应选项，添加一个临时选项
            const tempOption = document.createElement('option');
            tempOption.value = hours;
            tempOption.text = hours + ' ' + t('monitor.hours');
            tempOption.selected = true;
            hoursSelect.appendChild(tempOption);
        }
        hoursSelect.disabled = true;
    }

    // 重新加载数据
    if (getSelectedEndpoints().length > 0) {
        loadMultipleEndpointsHealthHistory();
    }
}

// Load health history data from backend
async function loadHealthHistory() {
    if (!selectedHistoryEndpoint) {
        renderHealthHistoryEmpty();
        return;
    }

    try {
        // Get current client type
        const clientType = getCurrentClientType() || 'claude';

        const historyData = await window.go.main.App.GetHealthHistory(
            selectedHistoryEndpoint,
            clientType,
            selectedHistoryHours
        );

        if (!historyData || historyData.length === 0) {
            renderHealthHistoryEmpty(true);
            return;
        }

        healthHistoryData = historyData;
        renderHealthHistoryChart();
    } catch (error) {
        console.error('Failed to load health history:', error);
        renderHealthHistoryEmpty(true);
    }
}

// 加载多个端点的健康历史数据
async function loadMultipleEndpointsHealthHistory() {
    const selectedEndpoints = getSelectedEndpoints();

    if (selectedEndpoints.length === 0) {
        renderHealthHistoryEmpty();
        return;
    }

    try {
        const clientType = getCurrentClientType() || 'claude';

        // 并行加载所有端点的数据
        const promises = selectedEndpoints.map(endpointName =>
            window.go.main.App.GetHealthHistory(endpointName, clientType, selectedHistoryHours)
                .then(data => ({ endpointName, data, error: null }))
                .catch(error => ({ endpointName, data: [], error: error.message || 'Unknown error' }))
        );

        const results = await Promise.all(promises);

        // 存储到 Map 中
        healthHistoryDataMap.clear();
        for (const result of results) {
            if (result.data && result.data.length > 0) {
                healthHistoryDataMap.set(result.endpointName, result.data);
            } else if (result.error) {
                console.warn(`Failed to load health history for ${result.endpointName}:`, result.error);
            }
        }

        // 渲染时间轴
        renderMultipleEndpointTimelines();

        // 渲染延迟图表（如果启用）
        if (showLatencyChart) {
            renderLatencyChartForMultipleEndpoints();
        }
    } catch (error) {
        console.error('Failed to load multiple endpoints health history:', error);
        renderHealthHistoryEmpty(true);
    }
}

// Render empty state for health history
function renderHealthHistoryEmpty(noData = false) {
    const container = document.getElementById('healthHistoryChart');
    if (!container) return;

    container.innerHTML = `
        <div class="health-history-empty">
            <span class="health-history-empty-icon">${noData ? '📭' : '📊'}</span>
            <span>${noData ? t('monitor.noHealthHistory') : t('monitor.noEndpointSelected')}</span>
        </div>
    `;
}

// Render health history timeline chart
function renderHealthHistoryChart() {
    const container = document.getElementById('healthHistoryChart');
    if (!container || !healthHistoryData || healthHistoryData.length === 0) {
        renderHealthHistoryEmpty(true);
        return;
    }

    // Group data by time segments
    const segments = processHealthHistoryData(healthHistoryData, selectedHistoryHours);

    if (segments.length === 0) {
        renderHealthHistoryEmpty(true);
        return;
    }

    // Calculate segment widths
    const totalDuration = selectedHistoryHours * 60 * 60 * 1000; // in ms

    let html = '<div class="health-timeline">';
    html += '<div class="timeline-row">';
    html += `<span class="timeline-label">${selectedHistoryEndpoint.length > 10 ? selectedHistoryEndpoint.substring(0, 10) + '...' : selectedHistoryEndpoint}</span>`;
    html += '<div class="timeline-bar">';

    for (const segment of segments) {
        const widthPercent = (segment.duration / totalDuration) * 100;
        const statusClass = `status-${segment.status}`;
        const tooltipTime = new Date(segment.startTime).toLocaleString();
        const tooltipLatency = segment.latencyMs > 0 ? ` | ${Math.round(segment.latencyMs)}ms` : '';
        const tooltipError = segment.errorMessage ? ` | ${segment.errorMessage}` : '';

        html += `
            <div class="timeline-segment ${statusClass}"
                 style="width: ${Math.max(widthPercent, 0.5)}%"
                 title="${tooltipTime}${tooltipLatency}${tooltipError}">
            </div>
        `;
    }

    html += '</div></div>';

    // Add latency chart if we have latency data
    const latencyData = healthHistoryData.filter(d => d.latencyMs > 0);
    if (latencyData.length > 1) {
        html += renderLatencyChart(latencyData);
    }

    html += '</div>';
    container.innerHTML = html;
}

// 渲染多个端点的时间轴
function renderMultipleEndpointTimelines() {
    const container = document.getElementById('healthHistoryChart');
    if (!container) return;

    const selectedEndpoints = getSelectedEndpoints();

    if (selectedEndpoints.length === 0) {
        renderHealthHistoryEmpty();
        return;
    }

    let html = '<div class="health-timelines-container">';

    // 为每个选中的端点生成时间轴
    for (const endpointName of selectedEndpoints) {
        const endpointData = healthHistoryDataMap.get(endpointName);
        if (!endpointData || endpointData.length === 0) {
            // 显示无数据状态
            html += renderSingleEndpointTimeline(endpointName, []);
            continue;
        }

        const segments = processHealthHistoryData(endpointData, selectedHistoryHours);
        html += renderSingleEndpointTimeline(endpointName, segments);
    }

    html += '</div>';

    // 添加延迟图表容器
    html += '<div id="latencyChartContainer" class="latency-chart-container" style="display: ' + (showLatencyChart ? 'block' : 'none') + '"></div>';

    container.innerHTML = html;

    // 如果启用延迟图表，渲染它
    if (showLatencyChart) {
        renderLatencyChartForMultipleEndpoints();
    }
}

// 渲染单个端点的时间轴
function renderSingleEndpointTimeline(endpointName, segments) {
    const totalDuration = selectedHistoryHours * 60 * 60 * 1000; // in ms
    const truncatedName = truncateEndpointName(endpointName, 12);

    let html = '<div class="timeline-row" data-endpoint="' + escapeHtml(endpointName) + '">';
    html += '<span class="timeline-label" title="' + escapeHtml(endpointName) + '">' + escapeHtml(truncatedName) + '</span>';
    html += '<div class="timeline-bar">';

    if (segments.length === 0) {
        // 无数据状态
        html += '<div class="timeline-segment status-unknown" style="width: 100%" title="' + t('monitor.noHealthHistory') + '"></div>';
    } else {
        for (const segment of segments) {
            const widthPercent = (segment.duration / totalDuration) * 100;
            const statusClass = `status-${segment.status}`;
            const tooltipTime = new Date(segment.startTime).toLocaleString();
            const tooltipLatency = segment.latencyMs > 0 ? ` | ${Math.round(segment.latencyMs)}ms` : '';
            const tooltipError = segment.errorMessage ? ` | ${segment.errorMessage}` : '';

            html += `
                <div class="timeline-segment ${statusClass}"
                     style="width: ${Math.max(widthPercent, 0.5)}%"
                     title="${tooltipTime}${tooltipLatency}${tooltipError}">
                </div>
            `;
        }
    }

    html += '</div></div>';
    return html;
}

// 截断端点名称
function truncateEndpointName(name, maxLength = 12) {
    if (name.length <= maxLength) return name;
    return name.substring(0, maxLength - 3) + '...';
}

// Process health history data into timeline segments
function processHealthHistoryData(data, hours) {
    if (!data || data.length === 0) return [];

    // Sort by timestamp
    const sorted = [...data].sort((a, b) =>
        new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
    );

    const segments = [];
    const now = Date.now();
    const startTime = now - (hours * 60 * 60 * 1000);

    for (let i = 0; i < sorted.length; i++) {
        const record = sorted[i];
        const recordTime = new Date(record.timestamp).getTime();

        // Skip records outside our time window
        if (recordTime < startTime) continue;

        const segmentStart = recordTime;
        let segmentEnd;

        if (i < sorted.length - 1) {
            segmentEnd = new Date(sorted[i + 1].timestamp).getTime();
        } else {
            segmentEnd = now;
        }

        segments.push({
            startTime: segmentStart,
            endTime: segmentEnd,
            duration: segmentEnd - segmentStart,
            status: record.status || 'unknown',
            latencyMs: record.latencyMs || 0,
            errorMessage: record.errorMessage || ''
        });
    }

    // If first segment doesn't start at our window start, add unknown segment
    if (segments.length > 0 && segments[0].startTime > startTime) {
        segments.unshift({
            startTime: startTime,
            endTime: segments[0].startTime,
            duration: segments[0].startTime - startTime,
            status: 'unknown',
            latencyMs: 0,
            errorMessage: ''
        });
    }

    return segments;
}

// Render latency trend chart (simple SVG line chart)
function renderLatencyChart(latencyData) {
    if (!latencyData || latencyData.length < 2) return '';

    const width = 100;
    const height = 120;
    const padding = 5;

    // Filter data points to reduce density - keep only significant changes
    // We'll keep at most 30 points, sampling evenly across the data
    const maxPoints = 30;
    let filteredData = latencyData;
    if (latencyData.length > maxPoints) {
        const step = Math.ceil(latencyData.length / maxPoints);
        filteredData = latencyData.filter((_, i) => i % step === 0);
        // Always include the last point
        if (filteredData[filteredData.length - 1] !== latencyData[latencyData.length - 1]) {
            filteredData.push(latencyData[latencyData.length - 1]);
        }
    }

    // Get min/max values
    const latencies = filteredData.map(d => d.latencyMs);
    const maxLatency = Math.max(...latencies);
    const minLatency = Math.min(...latencies);
    const range = maxLatency - minLatency || 1;

    // Calculate points
    const points = filteredData.map((d, i) => {
        const x = (i / (filteredData.length - 1)) * (100 - padding * 2) + padding;
        const y = height - padding - ((d.latencyMs - minLatency) / range) * (height - padding * 2);
        return { x, y, latency: d.latencyMs, time: d.timestamp };
    });

    // Create SVG path (straight lines only)
    const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ');

    // Get start and end times for axis labels
    const startTime = new Date(filteredData[0].timestamp);
    const endTime = new Date(filteredData[filteredData.length - 1].timestamp);
    const timeFormat = (date) => {
        return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    };

    return `
        <div style="display: flex; gap: 8px; align-items: stretch; margin-top: 6px;">
            <!-- 左侧：图例 -->
            <div class="latency-legend">
                <div class="legend-item-vertical">
                    <span class="legend-dot healthy"></span>
                    <span>${t('monitor.statusHealthy')}</span>
                </div>
                <div class="legend-item-vertical">
                    <span class="legend-dot warning"></span>
                    <span>${t('monitor.statusWarning')}</span>
                </div>
                <div class="legend-item-vertical">
                    <span class="legend-dot error"></span>
                    <span>${t('monitor.statusError')}</span>
                </div>
                <div class="legend-item-vertical">
                    <span class="legend-dot unknown"></span>
                    <span>${t('monitor.statusUnknown')}</span>
                </div>
            </div>

            <!-- 右侧：延迟趋势图 -->
            <div class="latency-chart" style="flex: 1; min-width: 0;">
                <svg class="latency-chart-svg" viewBox="0 0 100 ${height}" preserveAspectRatio="none">
                    <!-- Latency line -->
                    <path class="latency-line" d="${pathD}" />

                    <!-- Data points -->
                    ${points.map((p, i) => `
                        <circle class="latency-point" cx="${p.x}" cy="${p.y}" r="${i % 3 === 0 ? '2.5' : '1.5'}">
                            <title>${new Date(p.time).toLocaleTimeString()} - ${Math.round(p.latency)}ms</title>
                        </circle>
                    `).join('')}
                </svg>

                <!-- Time and value axis -->
                <div style="display: flex; justify-content: space-between; font-size: 9px; color: var(--text-tertiary); margin-top: 2px;">
                    <span>${timeFormat(startTime)}</span>
                    <span>${Math.round(minLatency)}-${Math.round(maxLatency)}ms</span>
                    <span>${timeFormat(endTime)}</span>
                </div>
            </div>
        </div>
    `;
}

// 渲染多端点延迟图表
function renderLatencyChartForMultipleEndpoints() {
    const container = document.getElementById('latencyChartContainer');
    if (!container) return;

    const selectedEndpoints = getSelectedEndpoints();
    const colors = ['#667eea', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6'];

    // 收集所有端点的延迟数据
    const endpointLatencyData = [];
    for (const endpointName of selectedEndpoints) {
        const data = healthHistoryDataMap.get(endpointName);
        if (data && data.length > 0) {
            const latencyData = data.filter(d => d.latencyMs > 0);
            if (latencyData.length > 1) {
                endpointLatencyData.push({ name: endpointName, data: latencyData });
            }
        }
    }

    if (endpointLatencyData.length === 0) {
        container.innerHTML = '<div class="health-history-empty-text">' + t('monitor.noHealthHistory') + '</div>';
        return;
    }

    const width = 100;
    const height = 120;
    const padding = 5;

    // 找到所有数据的最小和最大延迟
    let globalMinLatency = Infinity;
    let globalMaxLatency = -Infinity;

    for (const endpoint of endpointLatencyData) {
        const latencies = endpoint.data.map(d => d.latencyMs);
        globalMinLatency = Math.min(globalMinLatency, ...latencies);
        globalMaxLatency = Math.max(globalMaxLatency, ...latencies);
    }

    const range = globalMaxLatency - globalMinLatency || 1;

    let html = '<div class="multi-latency-chart">';
    html += '<h5 style="margin: 0 0 8px 0; font-size: 13px; color: var(--text-primary);">' + t('monitor.latencyChartTitle') + '</h5>';

    // 水平图例
    html += '<div class="latency-legend-horizontal">';
    endpointLatencyData.forEach((endpoint, index) => {
        const color = colors[index % colors.length];
        const truncatedName = truncateEndpointName(endpoint.name, 15);
        html += `
            <div class="legend-item">
                <span class="legend-dot" style="background: ${color}"></span>
                <span title="${escapeHtml(endpoint.name)}">${escapeHtml(truncatedName)}</span>
            </div>
        `;
    });
    html += '</div>';

    // SVG 图表
    html += `<svg class="latency-chart-svg" viewBox="0 0 ${width} ${height}" preserveAspectRatio="none">`;

    // 为每个端点绘制曲线
    endpointLatencyData.forEach((endpoint, index) => {
        const color = colors[index % colors.length];
        const latencyData = endpoint.data;

        // 采样数据点
        const maxPoints = 30;
        let filteredData = latencyData;
        if (latencyData.length > maxPoints) {
            const step = Math.ceil(latencyData.length / maxPoints);
            filteredData = latencyData.filter((_, i) => i % step === 0);
            if (filteredData[filteredData.length - 1] !== latencyData[latencyData.length - 1]) {
                filteredData.push(latencyData[latencyData.length - 1]);
            }
        }

        // 计算点坐标
        const points = filteredData.map((d, i) => {
            const x = (i / (filteredData.length - 1)) * (width - padding * 2) + padding;
            const y = height - padding - ((d.latencyMs - globalMinLatency) / range) * (height - padding * 2);
            return { x, y, latency: d.latencyMs, time: d.timestamp };
        });

        // 创建路径
        const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ');

        html += `<path class="latency-line" d="${pathD}" stroke="${color}" fill="none" stroke-width="2" />`;

        // 添加数据点
        points.forEach((p, i) => {
            if (i % 3 === 0) {
                html += `
                    <circle cx="${p.x}" cy="${p.y}" r="2" fill="${color}">
                        <title>${escapeHtml(endpoint.name)} - ${new Date(p.time).toLocaleTimeString()} - ${Math.round(p.latency)}ms</title>
                    </circle>
                `;
            }
        });
    });

    html += '</svg>';

    // 时间和数值轴
    if (endpointLatencyData.length > 0 && endpointLatencyData[0].data.length > 0) {
        const firstData = endpointLatencyData[0].data;
        const startTime = new Date(firstData[0].timestamp);
        const endTime = new Date(firstData[firstData.length - 1].timestamp);
        const timeFormat = (date) => date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

        html += `
            <div style="display: flex; justify-content: space-between; font-size: 10px; color: var(--text-tertiary); margin-top: 4px;">
                <span>${timeFormat(startTime)}</span>
                <span>${Math.round(globalMinLatency)}-${Math.round(globalMaxLatency)}ms</span>
                <span>${timeFormat(endTime)}</span>
            </div>
        `;
    }

    html += '</div>';
    container.innerHTML = html;
}

// Refresh health history (called when endpoint health is updated)
export function refreshHealthHistory() {
    if (selectedHistoryEndpoint) {
        loadHealthHistory();
    }
    populateHealthHistoryEndpoints();
}

// ========== 端点检测结果相关函数 ==========

// 加载端点检测结果
async function loadEndpointCheckResults() {
    try {
        const resultStr = await window.go.main.App.GetEndpointCheckResults();
        const results = JSON.parse(resultStr);

        endpointCheckResults.clear();
        for (const [name, result] of Object.entries(results)) {
            endpointCheckResults.set(name, {
                lastCheckAt: new Date(result.lastCheckAt),
                success: result.success,
                latencyMs: result.latencyMs,
                errorMessage: result.errorMessage
            });
        }

        renderEndpointHealth();
    } catch (error) {
        console.error('Failed to load endpoint check results:', error);
    }
}

// 启动检测时间更新定时器
function startCheckTimeUpdates() {
    if (checkTimeUpdateInterval) {
        clearInterval(checkTimeUpdateInterval);
    }

    // 每秒更新检测时间显示
    checkTimeUpdateInterval = setInterval(() => {
        if (isMonitorVisible && endpointCheckResults.size > 0) {
            updateCheckTimeDisplays();
        }
    }, 1000);

    // 每 10 秒刷新一次健康数据和检测结果
    setInterval(() => {
        if (isMonitorVisible) {
            loadEndpointHealth().catch(console.error);
        }
    }, 10000);
}

// 更新检测时间显示
function updateCheckTimeDisplays() {
    const now = Date.now();
    endpointCheckResults.forEach((result, name) => {
        const timeEl = document.querySelector(`[data-endpoint-name="${name}"] .check-time`);
        if (timeEl && result.lastCheckAt) {
            const secondsAgo = Math.floor((now - result.lastCheckAt.getTime()) / 1000);
            timeEl.textContent = formatTimeAgo(secondsAgo);
        }
    });
}

// 格式化时间差
function formatTimeAgo(seconds) {
    if (seconds < 60) {
        return t('monitor.secondsAgo').replace('{count}', seconds);
    } else if (seconds < 3600) {
        const minutes = Math.floor(seconds / 60);
        return t('monitor.minutesAgo').replace('{count}', minutes);
    } else if (seconds < 86400) {
        const hours = Math.floor(seconds / 3600);
        return t('monitor.hoursAgo').replace('{count}', hours);
    } else {
        return t('monitor.neverChecked');
    }
}

// 一键检测所有端点并优化（异步执行，不阻塞 UI）
export async function testAllEndpointsAndOptimize() {
    if (isTestingAllEndpoints) {
        return;
    }

    isTestingAllEndpoints = true;
    const btn = document.getElementById('testAllEndpointsBtn');
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = `⏳ ${t('monitor.testing')}`;
    }

    // 异步执行检测，不等待结果
    const clientType = getCurrentClientType() || 'claude';

    // 使用 setTimeout 确保 UI 更新后再执行
    setTimeout(async () => {
        try {
            const resultStr = await window.go.main.App.TestAllEndpointsAndOptimize(clientType);
            const result = JSON.parse(resultStr);

            if (result.success) {
                // 显示结果通知
                showTestResultNotification(result);

                // 异步刷新数据，不阻塞
                loadEndpointCheckResults().catch(console.error);
                if (typeof refreshEndpoints === 'function') {
                    refreshEndpoints().catch(console.error);
                }
                loadEndpointHealth().catch(console.error);
            } else {
                showTestErrorNotification(result.message || t('monitor.testFailed'));
            }
        } catch (error) {
            console.error('Failed to test all endpoints:', error);
            showTestErrorNotification(t('monitor.testFailed') + ': ' + error.message);
        } finally {
            isTestingAllEndpoints = false;
            if (btn) {
                btn.disabled = false;
                btn.innerHTML = `🔍 ${t('monitor.testAllEndpoints')}`;
            }
        }
    }, 0);
}

// 显示检测错误通知
function showTestErrorNotification(message) {
    const container = document.getElementById('testResultNotification');
    if (!container) return;

    container.innerHTML = `
        <div class="test-result-notification error">
            <div class="test-result-header">
                <span class="test-result-icon">✗</span>
                <span class="test-result-title">${t('monitor.testFailed')}</span>
                <button class="test-result-close" onclick="this.parentElement.parentElement.remove()">×</button>
            </div>
            <div class="test-result-body">
                <div class="test-result-error">${escapeHtml(message)}</div>
            </div>
        </div>
    `;

    // 5秒后自动隐藏
    setTimeout(() => {
        const notification = container.querySelector('.test-result-notification');
        if (notification) {
            notification.classList.add('fade-out');
            setTimeout(() => notification.remove(), 300);
        }
    }, 5000);
}

// 显示检测结果通知
function showTestResultNotification(result) {
    const container = document.getElementById('testResultNotification');
    if (!container) return;

    let html = `
        <div class="test-result-notification">
            <div class="test-result-header">
                <span class="test-result-icon">✓</span>
                <span class="test-result-title">${t('monitor.testComplete')}</span>
                <button class="test-result-close" onclick="this.parentElement.parentElement.remove()">×</button>
            </div>
            <div class="test-result-body">
                <div class="test-result-summary">
                    ${result.bestEndpoint ? `<div class="test-result-best"><span class="label">${t('monitor.bestEndpoint')}:</span> <span class="value">${escapeHtml(result.bestEndpoint)}</span></div>` : ''}
                    ${result.enabledCount > 0 ? `<div class="test-result-enabled"><span class="label">${t('monitor.enabledCount')}:</span> <span class="value">${result.enabledCount}</span></div>` : ''}
                    ${result.disabledCount > 0 ? `<div class="test-result-disabled"><span class="label">${t('monitor.disabledCount')}:</span> <span class="value">${result.disabledCount}</span></div>` : ''}
                </div>
                <div class="test-result-details">
    `;

    for (const r of result.results) {
        const statusClass = r.success ? 'success' : 'failed';
        const statusIcon = r.success ? '✓' : '✗';
        const actionText = getActionText(r.action);

        html += `
            <div class="test-result-item ${statusClass}">
                <span class="result-status">${statusIcon}</span>
                <span class="result-name">${escapeHtml(r.name)}</span>
                <span class="result-latency">${r.success ? Math.round(r.latencyMs) + 'ms' : (r.errorMessage || t('monitor.checkFailed'))}</span>
                ${actionText ? `<span class="result-action">${actionText}</span>` : ''}
            </div>
        `;
    }

    html += `
                </div>
            </div>
        </div>
    `;

    container.innerHTML = html;

    // 5秒后自动隐藏
    setTimeout(() => {
        const notification = container.querySelector('.test-result-notification');
        if (notification) {
            notification.classList.add('fade-out');
            setTimeout(() => notification.remove(), 300);
        }
    }, 5000);
}

// 获取操作文本
function getActionText(action) {
    switch (action) {
        case 'set_current':
            return t('monitor.actionSetCurrent');
        case 'enabled':
            return t('monitor.actionEnabled');
        case 'disabled':
            return t('monitor.actionDisabled');
        default:
            return '';
    }
}

// 导出加载检测结果函数供外部调用
export { loadEndpointCheckResults };

// 将一键检测函数暴露到 window 对象
if (typeof window !== 'undefined') {
    window.testAllEndpointsAndOptimize = testAllEndpointsAndOptimize;
}

