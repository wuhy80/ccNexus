import { formatTokens } from '../utils/format.js';
import { loadCostByPeriod, loadCostTrend } from './cost.js';

let endpointStats = {};
let currentPeriod = 'daily'; // 'daily', 'weekly', 'monthly'

export function getEndpointStats(clientType) {
    if (!clientType) {
        return endpointStats;
    }
    return endpointStats[clientType] || {};
}

export function getCurrentPeriod() {
    return currentPeriod;
}

// Load statistics (legacy function for backward compatibility)
export async function loadStats() {
    try {
        const statsStr = await window.go.main.App.GetStats();
        const stats = JSON.parse(statsStr);

        document.getElementById('totalRequests').textContent = stats.totalRequests;

        let totalSuccess = 0;
        let totalFailed = 0;
        let totalInputTokens = 0;
        let totalCacheCreationTokens = 0;
        let totalCacheReadTokens = 0;
        let totalOutputTokens = 0;

        for (const epStats of Object.values(stats.endpoints || {})) {
            totalSuccess += (epStats.requests - epStats.errors);
            totalFailed += epStats.errors;
            totalInputTokens += epStats.inputTokens || 0;
            totalCacheCreationTokens += epStats.cacheCreationTokens || 0;
            totalCacheReadTokens += epStats.cacheReadTokens || 0;
            totalOutputTokens += epStats.outputTokens || 0;
        }

        document.getElementById('successRequests').textContent = totalSuccess;
        document.getElementById('failedRequests').textContent = totalFailed;

        // Include cache tokens in the total (cache_creation + cache_read are part of input)
        const totalInputWithCache = totalInputTokens + totalCacheCreationTokens + totalCacheReadTokens;
        const totalTokens = totalInputWithCache + totalOutputTokens;
        document.getElementById('totalTokens').textContent = formatTokens(totalTokens);
        document.getElementById('totalInputTokens').textContent = formatTokens(totalInputWithCache);
        document.getElementById('totalOutputTokens').textContent = formatTokens(totalOutputTokens);

        endpointStats = normalizeEndpointStats(stats.endpoints);

        return stats;
    } catch (error) {
        console.error('Failed to load stats:', error);
        return null;
    }
}

// Load statistics by period (daily, yesterday, weekly, monthly)
export async function loadStatsByPeriod(period = 'daily') {
    try {
        currentPeriod = period;

        let statsStr;
        switch (period) {
            case 'daily':
                statsStr = await window.go.main.App.GetStatsDaily();
                break;
            case 'yesterday':
                statsStr = await window.go.main.App.GetStatsYesterday();
                break;
            case 'weekly':
                statsStr = await window.go.main.App.GetStatsWeekly();
                break;
            case 'monthly':
                statsStr = await window.go.main.App.GetStatsMonthly();
                break;
            default:
                statsStr = await window.go.main.App.GetStatsDaily();
        }

        const stats = JSON.parse(statsStr);

        // Update UI elements
        const totalRequests = stats.totalRequests || 0;
        const totalSuccess = stats.totalSuccess || 0;
        const totalErrors = stats.totalErrors || 0;

        document.getElementById('periodTotalRequests').textContent = totalRequests;
        document.getElementById('periodSuccess').textContent = totalSuccess;
        document.getElementById('periodFailed').textContent = totalErrors;

        // Include cache tokens in the total (cache_creation + cache_read are part of input)
        const totalCacheCreationTokens = stats.totalCacheCreationTokens || 0;
        const totalCacheReadTokens = stats.totalCacheReadTokens || 0;
        const totalInputWithCache = (stats.totalInputTokens || 0) +
            totalCacheCreationTokens +
            totalCacheReadTokens;
        const totalOutputTokens = stats.totalOutputTokens || 0;
        const totalTokens = totalInputWithCache + totalOutputTokens;
        document.getElementById('periodTotalTokens').textContent = formatTokens(totalTokens);
        document.getElementById('periodInputTokens').textContent = formatTokens(totalInputWithCache);
        document.getElementById('periodOutputTokens').textContent = formatTokens(totalOutputTokens);

        // Derived metrics for better diagnostics
        const successRateValue = totalRequests ? (totalSuccess / totalRequests) * 100 : 0;
        const errorRateValue = totalRequests ? (totalErrors / totalRequests) * 100 : 0;
        document.getElementById('periodSuccessRate').textContent = formatPercentageValue(successRateValue);
        document.getElementById('periodErrorRate').textContent = formatPercentageValue(errorRateValue);

        const avgTokensPerRequest = totalRequests ? totalTokens / totalRequests : 0;
        const avgInputPerRequest = totalRequests ? totalInputWithCache / totalRequests : 0;
        const avgOutputPerRequest = totalRequests ? totalOutputTokens / totalRequests : 0;
        document.getElementById('periodAvgTokens').textContent = formatAverageTokens(avgTokensPerRequest);
        document.getElementById('periodAvgInputTokens').textContent = formatAverageTokens(avgInputPerRequest);
        document.getElementById('periodAvgOutputTokens').textContent = formatAverageTokens(avgOutputPerRequest);

        document.getElementById('cacheCreationTokens').textContent = formatTokens(totalCacheCreationTokens);
        document.getElementById('cacheReadTokens').textContent = formatTokens(totalCacheReadTokens);
        const totalCacheTokens = totalCacheCreationTokens + totalCacheReadTokens;
        const cacheHitRateValue = totalCacheTokens ? (totalCacheReadTokens / totalCacheTokens) * 100 : 0;
        document.getElementById('cacheHitRate').textContent = formatPercentageValue(cacheHitRateValue);

        // Update endpoint stats (active / total)
        const activeEndpoints = stats.activeEndpoints || 0;
        const totalEndpoints = stats.totalEndpoints || 0;
        document.getElementById('activeEndpointsDisplay').textContent = activeEndpoints;
        document.getElementById('totalEndpointsDisplay').textContent = totalEndpoints;

        // Load and display trend for current period
        await loadTrend(period);

        // Load performance metrics for current period
        await loadPerformanceMetrics(period);

        // Load cost statistics for current period
        await loadCostByPeriod(period);
        await loadCostTrend(period);

        // Store endpoint stats for today
        endpointStats = normalizeEndpointStats(stats.endpoints);

        return stats;
    } catch (error) {
        console.error('Failed to load stats by period:', error);
        return null;
    }
}

// Load performance metrics for specified period
async function loadPerformanceMetrics(period = 'daily') {
    try {
        const metricsStr = await window.go.main.App.GetPerformanceStats(period);
        const metrics = JSON.parse(metricsStr);

        if (!metrics.success) {
            console.error('Failed to load performance metrics:', metrics.message);
            return;
        }

        const overall = metrics.overallMetrics;

        // Update UI elements - Average Duration
        const avgDurationEl = document.getElementById('avgDurationMs');
        if (avgDurationEl && overall) {
            avgDurationEl.textContent = overall.avgDurationMs > 0
                ? `${(overall.avgDurationMs / 1000).toFixed(1)}s`
                : '-';
        }

        // Min/Max Duration
        const minDurationEl = document.getElementById('minDurationMs');
        const maxDurationEl = document.getElementById('maxDurationMs');
        if (minDurationEl && overall) {
            minDurationEl.textContent = overall.minDurationMs > 0
                ? `${(overall.minDurationMs / 1000).toFixed(1)}s`
                : '-';
        }
        if (maxDurationEl && overall) {
            maxDurationEl.textContent = overall.maxDurationMs > 0
                ? `${(overall.maxDurationMs / 1000).toFixed(1)}s`
                : '-';
        }

        // Output tokens per second
        const outputTokensPerSecEl = document.getElementById('avgOutputTokensPerSec');
        if (outputTokensPerSecEl && overall) {
            outputTokensPerSecEl.textContent = overall.outputTokensPerSec > 0
                ? `${overall.outputTokensPerSec.toFixed(1)}`
                : '-';
        }

        // Input tokens per second
        const inputTokensPerSecEl = document.getElementById('avgInputTokensPerSec');
        if (inputTokensPerSecEl && overall) {
            inputTokensPerSecEl.textContent = overall.inputTokensPerSec > 0
                ? `${overall.inputTokensPerSec.toFixed(1)}`
                : '-';
        }

        // Streaming stats
        const streamingCountEl = document.getElementById('streamingCount');
        const nonStreamingCountEl = document.getElementById('nonStreamingCount');
        const streamingPercentageEl = document.getElementById('streamingPercentage');

        if (streamingCountEl && overall) {
            streamingCountEl.textContent = overall.streamingCount || 0;
        }
        if (nonStreamingCountEl && overall) {
            nonStreamingCountEl.textContent = overall.nonStreamingCount || 0;
        }
        if (streamingPercentageEl && overall) {
            streamingPercentageEl.textContent = overall.validRequests > 0
                ? `${overall.streamingPercentage.toFixed(0)}%`
                : '-';
        }

    } catch (error) {
        console.error('Failed to load performance metrics:', error);
    }
}

// Load trend comparison data for specified period
async function loadTrend(period = 'daily') {
    try {
        const trendStr = await window.go.main.App.GetStatsTrendByPeriod(period);
        const trend = JSON.parse(trendStr);

        const requestsTrend = formatTrend(trend.trend);
        const errorsTrend = formatTrend(trend.errorsTrend);
        const tokensTrend = formatTrend(trend.tokensTrend);

        const requestsEl = document.getElementById('requestsTrend');
        const errorsEl = document.getElementById('errorsTrend');
        const tokensEl = document.getElementById('tokensTrend');

        if (requestsEl) {
            requestsEl.textContent = requestsTrend.text;
            requestsEl.className = 'trend ' + requestsTrend.className;
        }

        if (errorsEl) {
            // For errors, negative trend is good
            errorsEl.textContent = errorsTrend.text;
            errorsEl.className = 'trend ' + (trend.errorsTrend < 0 ? 'trend-down' : trend.errorsTrend > 0 ? 'trend-up' : 'trend-flat');
        }

        if (tokensEl) {
            tokensEl.textContent = tokensTrend.text;
            tokensEl.className = 'trend ' + tokensTrend.className;
        }
    } catch (error) {
        console.error('Failed to load trend:', error);
    }
}

// Format trend value for display
function formatTrend(value) {
    const absValue = Math.abs(value);
    const formattedValue = absValue.toFixed(1);

    if (value > 0) {
        return {
            text: `↑ ${formattedValue}%`,
            className: 'trend-up'
        };
    } else if (value < 0) {
        return {
            text: `↓ ${formattedValue}%`,
            className: 'trend-down'
        };
    } else {
        return {
            text: '→ 0%',
            className: 'trend-flat'
        };
    }
}

function formatPercentageValue(value) {
    if (!isFinite(value) || value <= 0) {
        return '0%';
    }
    const rounded = Number(value.toFixed(1));
    return `${rounded}%`;
}

function formatAverageTokens(value) {
    if (!isFinite(value) || value <= 0) {
        return '0';
    }

    if (value >= 1000) {
        return formatTokens(Math.round(value));
    }
    if (value >= 100) {
        return Math.round(value).toString();
    }
    if (value >= 1) {
        return value.toFixed(1);
    }
    return value.toFixed(2);
}

function normalizeEndpointStats(rawStats) {
    const normalized = {};
    if (!rawStats) {
        return normalized;
    }

    for (const [key, stats] of Object.entries(rawStats)) {
        const [clientType, endpointName] = splitEndpointKey(key);
        if (!normalized[clientType]) {
            normalized[clientType] = {};
        }
        normalized[clientType][endpointName] = stats;
    }

    return normalized;
}

function splitEndpointKey(key) {
    if (!key) {
        return ['claude', ''];
    }
    const separatorIndex = key.indexOf(':');
    if (separatorIndex === -1) {
        return ['claude', key];
    }

    const clientType = separatorIndex === 0 ? 'claude' : key.slice(0, separatorIndex);
    const endpointName = key.slice(separatorIndex + 1);
    return [clientType || 'claude', endpointName];
}

// Switch statistics period
export async function switchStatsPeriod(period) {
    // Handle history modal separately
    if (period === 'history') {
        // Open history modal without changing active tab
        import('./history.js').then(module => {
            module.showHistoryModal();
        });
        return;
    }

    currentPeriod = period;

    // Update tab buttons
    const tabs = document.querySelectorAll('.stats-tab-btn');
    tabs.forEach(tab => {
        if (tab.dataset.period === period) {
            tab.classList.add('active');
        } else {
            tab.classList.remove('active');
        }
    });

    // Load stats for the selected period
    await loadStatsByPeriod(period);

    // Reload endpoint list to update endpoint stats cards
    if (window.loadConfig) {
        window.loadConfig();
    }

    // Sync chart with period change
    try {
        const { switchChartPeriod } = await import('./chart.js');
        if (switchChartPeriod) {
            await switchChartPeriod(period);
        }
    } catch (error) {
        console.error('Failed to sync chart:', error);
        // Chart module may not be loaded yet, this is not critical
    }
}

// Refresh session statistics
export async function refreshSessionStats() {
    try {
        const statsStr = await window.go.main.App.GetSessionStats();
        const stats = JSON.parse(statsStr);

        const card = document.getElementById('sessionStatsCard');
        const content = document.getElementById('sessionStatsContent');

        if (!stats.enabled) {
            card.style.display = 'none';
            return;
        }

        card.style.display = 'block';

        if (!stats.sessionBindings || stats.sessionBindings.length === 0) {
            content.innerHTML = `
                <div style="text-align: center; padding: 20px; color: var(--text-secondary);">
                    ${window.t('statistics.sessionStatsEmpty')}
                </div>
            `;
            return;
        }

        // Build session bindings table
        let html = `
            <div style="margin-bottom: 15px;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
                    <span style="font-weight: 500;">${window.t('statistics.totalSessions')}: ${stats.totalSessions}</span>
                </div>
            </div>
            <div style="overflow-x: auto;">
                <table style="width: 100%; border-collapse: collapse;">
                    <thead>
                        <tr style="background: var(--bg-secondary); border-bottom: 2px solid var(--border-color);">
                            <th style="padding: 10px; text-align: left; font-size: 13px;">${window.t('statistics.sessionId')}</th>
                            <th style="padding: 10px; text-align: left; font-size: 13px;">${window.t('statistics.boundEndpoint')}</th>
                            <th style="padding: 10px; text-align: center; font-size: 13px;">${window.t('statistics.requestCount')}</th>
                            <th style="padding: 10px; text-align: center; font-size: 13px;">${window.t('statistics.lastAccess')}</th>
                            <th style="padding: 10px; text-align: center; font-size: 13px;">${window.t('common.actions')}</th>
                        </tr>
                    </thead>
                    <tbody>
        `;

        stats.sessionBindings.forEach(binding => {
            const lastAccessDate = new Date(binding.lastAccess * 1000);
            const now = new Date();
            const diffMs = now - lastAccessDate;
            const diffMins = Math.floor(diffMs / 60000);
            const diffHours = Math.floor(diffMins / 60);

            let timeAgo;
            if (diffMins < 1) {
                timeAgo = window.t('common.justNow') || '刚刚';
            } else if (diffMins < 60) {
                timeAgo = `${diffMins} ${window.t('common.minutesAgo') || '分钟前'}`;
            } else if (diffHours < 24) {
                timeAgo = `${diffHours} ${window.t('common.hoursAgo') || '小时前'}`;
            } else {
                const diffDays = Math.floor(diffHours / 24);
                timeAgo = `${diffDays} ${window.t('common.daysAgo') || '天前'}`;
            }

            html += `
                <tr style="border-bottom: 1px solid var(--border-color);">
                    <td style="padding: 10px; font-family: monospace; font-size: 12px;">${binding.sessionId.substring(0, 16)}...</td>
                    <td style="padding: 10px; font-size: 13px;">${binding.endpointName}</td>
                    <td style="padding: 10px; text-align: center; font-size: 13px;">${binding.requestCount}</td>
                    <td style="padding: 10px; text-align: center; font-size: 12px; color: var(--text-secondary);">${timeAgo}</td>
                    <td style="padding: 10px; text-align: center;">
                        <button class="btn btn-secondary" style="padding: 4px 8px; font-size: 12px;" onclick="window.unbindSession('${binding.sessionId}')">
                            ${window.t('statistics.unbind')}
                        </button>
                    </td>
                </tr>
            `;
        });

        html += `
                    </tbody>
                </table>
            </div>
        `;

        content.innerHTML = html;
    } catch (error) {
        console.error('Failed to refresh session stats:', error);
    }
}

// Unbind session
export async function unbindSession(sessionId) {
    try {
        await window.go.main.App.UnbindSession(sessionId);
        showNotification(window.t('statistics.unbindSuccess'), 'success');
        await refreshSessionStats();
    } catch (error) {
        console.error('Failed to unbind session:', error);
        showNotification(window.t('statistics.unbindFailed') + ': ' + error, 'error');
    }
}

// Show notification
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.textContent = message;
    notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 15px 20px;
        background: ${type === 'success' ? '#10b981' : type === 'error' ? '#ef4444' : '#3b82f6'};
        color: white;
        border-radius: 8px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        z-index: 10000;
        animation: slideInRight 0.3s ease-out;
    `;

    document.body.appendChild(notification);

    setTimeout(() => {
        notification.style.animation = 'slideOutRight 0.3s ease-out';
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

// Export to window for onclick handlers
window.refreshSessionStats = refreshSessionStats;
window.unbindSession = unbindSession;
