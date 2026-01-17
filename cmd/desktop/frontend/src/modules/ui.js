import { t } from '../i18n/index.js';

export function initUI() {
    const app = document.getElementById('app');
    app.innerHTML = `
        <div class="header">
            <div style="display: flex; justify-content: space-between; align-items: center; width: 100%;">
                <div>
                    <h1>🚀 ${t('app.title')}<span id="broadcast-banner" class="broadcast-banner hidden"></span></h1>
                    <p>${t('header.title')}<span id="festivalToggle" class="festival-toggle hidden" onclick="window.toggleFestivalEffect(); event.stopPropagation();" title="${t('festival.toggle') || '切换氛围效果'}"><span class="festival-toggle-name" id="festivalToggleName"></span><span class="festival-toggle-switch" id="festivalToggleSwitch"></span></span></p>
                </div>
                <div style="display: flex; gap: 15px; align-items: center;">
                    <div class="port-display" onclick="window.showEditPortModal()" title="${t('header.port')}">
                        <span style="color: #666; font-size: 14px;">${t('header.port')}: </span>
                        <span class="port-number" id="proxyPort">3003</span>
                    </div>
                    <div style="display: flex; gap: 10px;">
                        <button class="header-link" onclick="window.openGitHub()" title="${t('header.githubRepo')}">
                            <svg width="24" height="24" viewBox="0 0 16 16" fill="currentColor">
                                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                            </svg>
                        </button>
                        <button class="header-link about-btn" id="aboutBtn" onclick="window.showWelcomeModal()" title="${t('header.about')}">
                            📖
                        </button>
                        <button class="header-link" onclick="window.showSettingsModal()" title="${t('settings.title')}">
                            ⚙️
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <div class="container">
            <!-- Statistics -->
            <div class="card">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                    <h2 style="margin: 0;">📊 ${t('statistics.title')}</h2>
                    <div class="stats-tabs">
                        <button class="stats-detail-btn" onclick="window.showDailyDetailsModal()" title="${t('statistics.viewDetails')}">
                            📋 ${t('statistics.details')}
                        </button>
                        <button class="stats-tab-btn active" data-period="daily" onclick="window.switchStatsPeriod('daily')">
                            📅 ${t('statistics.daily')}
                        </button>
                        <button class="stats-tab-btn" data-period="yesterday" onclick="window.switchStatsPeriod('yesterday')">
                            📆 ${t('statistics.yesterday')}
                        </button>
                        <button class="stats-tab-btn" data-period="weekly" onclick="window.switchStatsPeriod('weekly')">
                            📊 ${t('statistics.weekly')}
                        </button>
                        <button class="stats-tab-btn" data-period="monthly" onclick="window.switchStatsPeriod('monthly')">
                            📈 ${t('statistics.monthly')}
                        </button>
                        <button class="stats-tab-btn" data-period="history" onclick="window.switchStatsPeriod('history')">
                            📚 ${t('statistics.history')}
                        </button>
                    </div>
                </div>

                <!-- Current Stats View -->
                <div id="currentStatsView">
                    <div class="stats-layout">
                        <!-- Compressed Stats Grid -->
                        <div class="stats-compact-grid">
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label">${t('statistics.endpoints')}</div>
                                    <div class="stat-detail">${t('statistics.activeTotal')}</div>
                                </div>
                                <div class="stat-value">
                                    <span id="activeEndpointsDisplay" class="stat-primary">0</span>
                                    <span class="stat-secondary"> / </span>
                                    <span id="totalEndpointsDisplay" class="stat-secondary">0</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label-row">
                                        <div class="stat-label">${t('statistics.totalRequests')}</div>
                                        <span class="trend" id="requestsTrend">→ 0%</span>
                                    </div>
                                    <div class="stat-detail">
                                        <span id="periodSuccess">0</span>
                                        <span class="stat-text"> ${t('statistics.success')}</span>
                                        <span class="stat-divider">/</span>
                                        <span id="periodFailed">0</span>
                                        <span class="stat-text"> ${t('statistics.failed')}</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="periodTotalRequests" class="stat-primary">0</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label-row">
                                        <div class="stat-label">${t('statistics.totalTokens')}</div>
                                        <span class="trend" id="tokensTrend">→ 0%</span>
                                    </div>
                                    <div class="stat-detail">
                                        <span id="periodInputTokens">0</span>
                                        <span class="stat-text"> ${t('statistics.in')}</span>
                                        <span class="stat-divider">/</span>
                                        <span id="periodOutputTokens">0</span>
                                        <span class="stat-text"> ${t('statistics.out')}</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="periodTotalTokens" class="stat-primary">0</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label">${t('statistics.successRate')}</div>
                                    <div class="stat-detail">
                                        ${t('statistics.errorRate')}: <span id="periodErrorRate">0%</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="periodSuccessRate" class="stat-primary">0%</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label">${t('statistics.avgTokens')}</div>
                                    <div class="stat-detail">
                                        <span id="periodAvgInputTokens">0</span>
                                        <span class="stat-text"> ${t('statistics.avgIn')}</span>
                                        <span class="stat-divider">/</span>
                                        <span id="periodAvgOutputTokens">0</span>
                                        <span class="stat-text"> ${t('statistics.avgOut')}</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="periodAvgTokens" class="stat-primary">0</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label">${t('statistics.cacheEfficiency')}</div>
                                    <div class="stat-detail">
                                        <span id="cacheCreationTokens">0</span>
                                        <span class="stat-text"> ${t('statistics.cacheWrites')}</span>
                                        <span class="stat-divider">/</span>
                                        <span id="cacheReadTokens">0</span>
                                        <span class="stat-text"> ${t('statistics.cacheReads')}</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="cacheHitRate" class="stat-primary">0%</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label">${t('statistics.avgDuration')}</div>
                                    <div class="stat-detail">
                                        <span id="minDurationMs">-</span>
                                        <span class="stat-text"> ${t('statistics.min')}</span>
                                        <span class="stat-divider">/</span>
                                        <span id="maxDurationMs">-</span>
                                        <span class="stat-text"> ${t('statistics.max')}</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="avgDurationMs" class="stat-primary">-</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label">${t('statistics.avgOutputSpeed')}</div>
                                    <div class="stat-detail">
                                        <span id="avgInputTokensPerSec">-</span>
                                        <span class="stat-text"> ${t('statistics.inputSpeed')}</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="avgOutputTokensPerSec" class="stat-primary">-</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed">
                                <div class="stat-info">
                                    <div class="stat-label">${t('statistics.streamingRatio')}</div>
                                    <div class="stat-detail">
                                        <span id="streamingCount">0</span>
                                        <span class="stat-text"> ${t('statistics.streaming')}</span>
                                        <span class="stat-divider">/</span>
                                        <span id="nonStreamingCount">0</span>
                                        <span class="stat-text"> ${t('statistics.nonStreaming')}</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="streamingPercentage" class="stat-primary">-</span>
                                </div>
                            </div>
                            <!-- Cost Statistics -->
                            <div class="stat-box-compact stat-box-condensed cost-stat-box">
                                <div class="stat-info">
                                    <div class="stat-label">💰 ${t('cost.totalCost')}</div>
                                    <div class="stat-detail">
                                        <span>⬇️</span>
                                        <span id="periodInputCost">$0.00</span>
                                        <span class="stat-divider">/</span>
                                        <span>⬆️</span>
                                        <span id="periodOutputCost">$0.00</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="periodTotalCost" class="stat-primary cost-value">$0.00</span>
                                    <span id="costTrend" class="trend-indicator trend-neutral">→ 0%</span>
                                </div>
                            </div>
                            <div class="stat-box-compact stat-box-condensed savings-stat-box">
                                <div class="stat-info">
                                    <div class="stat-label">💎 ${t('cost.cacheSavings')}</div>
                                    <div class="stat-detail">
                                        <span>📝</span>
                                        <span id="periodCacheWriteCost">$0.00</span>
                                        <span class="stat-divider">/</span>
                                        <span>📖</span>
                                        <span id="periodCacheReadCost">$0.00</span>
                                    </div>
                                </div>
                                <div class="stat-value">
                                    <span id="periodCacheSavings" class="stat-primary savings-value">$0.00</span>
                                </div>
                            </div>
                            <!-- Endpoint Metrics (merged into same grid) -->
                            <div id="endpointMetricsGrid" style="display: contents;"></div>
                        </div>

                        <!-- Token Trend Chart -->
                        <div class="chart-container">
                            <div class="chart-header">
                                <!-- Left: Time Range Selector -->
                                <div class="chart-time-selector" id="chartTimeSelector">
                                    <select id="chartStartTime" class="time-select" onchange="window.onChartTimeChange()">
                                    </select>
                                    <span class="time-separator">—</span>
                                    <select id="chartEndTime" class="time-select" onchange="window.onChartTimeChange()">
                                    </select>
                                    <button id="chartTimeReset" class="btn-icon" title="${t('chart.resetTime') || '重置为自动'}" onclick="window.resetChartTimeRange()">↺</button>
                                </div>

                                <!-- Right: Chart Type + Granularity -->
                                <div class="chart-controls">
                                    <!-- Chart Type Toggle -->
                                    <div class="chart-type-btns">
                                        <button class="chart-type-btn active" data-type="line" title="${t('chart.lineChart') || '折线图'}" onclick="window.switchChartType('line')">
                                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                                <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                                            </svg>
                                        </button>
                                        <button class="chart-type-btn" data-type="bar" title="${t('chart.barChart') || '柱状图'}" onclick="window.switchChartType('bar')">
                                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                                <rect x="3" y="12" width="4" height="9"></rect>
                                                <rect x="10" y="6" width="4" height="15"></rect>
                                                <rect x="17" y="3" width="4" height="18"></rect>
                                            </svg>
                                        </button>
                                    </div>
                                    <div class="control-divider"></div>
                                    <!-- Granularity Buttons -->
                                    <button class="granularity-btn active" data-granularity="5min" onclick="window.switchGranularity('5min')">
                                        5${t('chart.minutes') || '分钟'}
                                    </button>
                                    <button class="granularity-btn" data-granularity="30min" onclick="window.switchGranularity('30min')">
                                        30${t('chart.minutes') || '分钟'}
                                    </button>
                                    <button class="granularity-btn" data-granularity="request" onclick="window.switchGranularity('request')">
                                        ${t('chart.perRequest') || '每次请求'}
                                    </button>
                                </div>
                            </div>
                            <canvas id="tokenChartContainer"></canvas>
                        </div>
                    </div>

                <!-- Hidden cumulative stats for endpoint cards -->
                <div style="display: none;">
                    <span id="activeEndpoints">0</span>
                    <span id="totalEndpoints">0</span>
                    <span id="totalRequests">0</span>
                    <span id="successRequests">0</span>
                    <span id="failedRequests">0</span>
                    <span id="totalTokens">0</span>
                    <span id="totalInputTokens">0</span>
                    <span id="totalOutputTokens">0</span>
                </div>
                </div>
            </div>

            <!-- Session Affinity Statistics -->
            <div class="card" id="sessionStatsCard" style="display: none;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                    <h2 style="margin: 0;">🔗 ${t('statistics.sessionStatsTitle')}</h2>
                    <button class="btn btn-secondary" onclick="window.refreshSessionStats()" style="padding: 6px 12px;">
                        🔄 ${t('common.refresh')}
                    </button>
                </div>
                <div id="sessionStatsContent">
                    <div style="text-align: center; padding: 20px; color: var(--text-secondary);">
                        ${t('statistics.sessionStatsDisabled')}
                    </div>
                </div>
            </div>

            <!-- Active Requests Monitor -->
            <div class="card monitor-section">
                <div class="monitor-header">
                    <h2 style="margin: 0;">🔄 ${t('monitor.title')}</h2>
                    <div class="throughput-indicators">
                        <span class="throughput-item">
                            <span class="throughput-value" id="requestsPerMin">0</span>
                            <span class="throughput-unit">${t('monitor.reqPerMin')}</span>
                        </span>
                        <span class="throughput-divider">|</span>
                        <span class="throughput-item">
                            <span class="throughput-value" id="tokensPerMin">0</span>
                            <span class="throughput-unit">${t('monitor.tokensPerMin')}</span>
                        </span>
                     <span class="throughput-divider">|</span>
                 <span class="throughput-item">
                            <span class="throughput-value" id="avgLatency">-</span>
                            <span class="throughput-unit">${t('monitor.avgLatency')}</span>
                        </span>
                    </div>
                </div>

                <div class="monitor-columns">
                    <!-- Left Column: Endpoint Health + Active Requests -->
                    <div class="monitor-column-left">
                        <!-- Endpoint Health Panel -->
                        <div class="endpoint-health-panel">
                            <div class="panel-header">
                                <h4><span class="section-icon">💚</span> ${t('monitor.endpointHealth')}</h4>
                            </div>
                            <div id="endpointHealthList" class="endpoint-health-grid">
                                <div class="monitor-empty">
                                    <span class="monitor-empty-icon">📡</span>
                                    <span class="monitor-empty-text">${t('monitor.noEndpoints')}</span>
                                </div>
                            </div>
                        </div>

                        <!-- Active Requests Panel -->
                        <div class="active-requests-panel">
                            <div class="panel-header">
                                <h4><span class="section-icon">⚡</span> ${t('monitor.activeRequests')}</h4>
                            </div>
                            <div id="activeRequestsList">
                                <div class="monitor-empty">
                                    <span class="monitor-empty-icon">✓</span>
                                    <span class="monitor-empty-text">${t('monitor.idle')}</span>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Right Column: Recent Requests -->
                    <div class="monitor-column-right">
                        <!-- Recent Requests Panel -->
                        <div class="recent-requests-panel">
                            <div class="panel-header">
                                <h4><span class="section-icon">📜</span> ${t('monitor.recentRequests')}</h4>
                            </div>
                            <div id="recentRequestsList">
                                <div class="monitor-empty">
                                    <span class="monitor-empty-icon">📭</span>
                                    <span class="monitor-empty-text">${t('monitor.noRecentRequests')}</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Health History Panel -->
                <div id="healthHistoryPanel"></div>
            </div>

            <!-- History Modal (弹窗) -->
            <div id="historyModal" class="modal" style="display: none;">
                <div class="modal-content">
                    <div class="modal-header">
                        <h2>📚 ${t('history.title')}</h2>
                        <button class="modal-close" onclick="window.closeHistoryModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="history-selector">
                            <label>${t('history.selectMonth')}:</label>
                            <select id="historyMonthSelect"></select>
                        </div>

                        <div class="history-stats-wrapper">
                            <div class="stats-grid">
                            <div class="stat-box">
                                <div class="stat-header">
                                    <div class="stat-label">${t('statistics.totalRequests')}</div>
                                    <span class="trend" id="historyRequestsTrend">→ 0%</span>
                                </div>
                                <div class="stat-value">
                                    <span id="historyTotalRequests">0</span>
                                </div>
                                <div class="stat-detail">
                                    <span id="historySuccess">0</span>
                                    <span class="stat-text"> ${t('statistics.success')}</span>
                                    <span class="stat-divider">/</span>
                                    <span id="historyFailed">0</span>
                                    <span class="stat-text"> ${t('statistics.failed')}</span>
                                </div>
                            </div>
                            <div class="stat-box">
                                <div class="stat-header">
                                    <div class="stat-label">${t('statistics.totalTokens')}</div>
                                    <span class="trend" id="historyTokensTrend">→ 0%</span>
                                </div>
                                <div class="stat-value">
                                    <span id="historyTotalTokens">0</span>
                                </div>
                                <div class="stat-detail">
                                    <span id="historyInputTokens">0</span>
                                    <span class="stat-text"> ${t('statistics.in')}</span>
                                    <span class="stat-divider">/</span>
                                    <span id="historyOutputTokens">0</span>
                                    <span class="stat-text"> ${t('statistics.out')}</span>
                                </div>
                            </div>
                        </div>
                        </div>

                        <div class="history-details">
                            <div class="history-details-header">
                                <h3>${t('history.dailyDetails')}</h3>
                                <button id="historyDeleteBtn" class="history-delete-btn" onclick="window.deleteHistoryArchive()" title="${t('history.deleteTitle')}">
                                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                        <path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                                    </svg>
                                    ${t('history.delete')}
                                </button>
                            </div>
                            <div class="table-container">
                                <table id="historyDailyTable">
                                    <thead>
                                        <tr>
                                            <th>${t('history.date')}</th>
                                            <th>${t('history.requests')}</th>
                                            <th>${t('history.errors')}</th>
                                            <th>${t('history.inputTokens')}</th>
                                            <th>${t('history.outputTokens')}</th>
                                            <th>${t('history.totalTokens')}</th>
                                        </tr>
                                    </thead>
                                    <tbody></tbody>
                                </table>
                            </div>
                        </div>

                        <div id="historyError" class="error-message" style="display: none;"></div>
                    </div>
                </div>
            </div>

            <!-- Daily Details Modal (今日详情弹窗) -->
            <div id="dailyDetailsModal" class="modal" style="display: none;">
                <div class="modal-content" style="width: 70%; max-width: 1400px;">
                    <div class="modal-header">
                        <h2>📋 ${t('statistics.dailyDetails')}</h2>
                        <button class="modal-close" onclick="window.closeDailyDetailsModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="details-controls">
                            <div class="details-info">
                                <span>${t('statistics.totalRecords')}: <strong id="detailsTotalCount">0</strong></span>
                            </div>
                            <div class="details-pagination">
                                <label>${t('statistics.pageSize')}:</label>
                                <select id="detailsPageSize" onchange="window.changeDetailsPageSize()">
                                    <option value="20">20</option>
                                    <option value="50">50</option>
                                    <option value="100">100</option>
                                </select>
                            </div>
                        </div>

                        <div class="table-container">
                            <table id="dailyDetailsTable">
                                <thead>
                                    <tr>
                                        <th>${t('statistics.time')}</th>
                                        <th>${t('statistics.endpoint')}</th>
                                        <th>${t('statistics.model')}</th>
                                        <th>${t('statistics.inputTokens')}</th>
                                        <th>${t('statistics.outputTokens')}</th>
                                        <th>${t('statistics.duration')}</th>
                                        <th>${t('statistics.outputTokensPerSec')}</th>
                                        <th>${t('statistics.status')}</th>
                                    </tr>
                                </thead>
                                <tbody></tbody>
                            </table>
                        </div>

                        <div class="details-pagination-controls">
                            <button id="detailsPrevBtn" class="btn btn-secondary" onclick="window.loadPreviousDetailsPage()" disabled>
                                ${t('statistics.previous')}
                            </button>
                            <span id="detailsPageInfo">1 / 1</span>
                            <button id="detailsNextBtn" class="btn btn-secondary" onclick="window.loadNextDetailsPage()" disabled>
                                ${t('statistics.next')}
                            </button>
                        </div>

                        <div id="detailsError" class="error-message" style="display: none;"></div>
                    </div>
                </div>
            </div>

            <!-- Endpoints -->
            <div class="card">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                    <div style="display: flex; align-items: center; gap: 15px;">
                        <h2 style="margin: 0;">🔗 ${t('endpoints.title')}</h2>
                        <button class="endpoint-toggle-btn" onclick="window.toggleEndpointPanel()">
                            <span id="endpointToggleIcon">🔼</span> <span id="endpointToggleText">${t('endpoints.collapse')}</span>
                        </button>
                        <div id="clientTypeSelector"></div>
                        <div id="tagFilterContainer"></div>
                        <div class="view-mode-tabs">
                            <button class="view-mode-btn active" data-view="detail" onclick="window.switchEndpointViewMode('detail')" title="${t('endpoints.viewDetail')}">
                                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                                    <rect x="3" y="3" width="8" height="8" rx="1"/>
                                    <rect x="13" y="3" width="8" height="8" rx="1"/>
                                    <rect x="3" y="13" width="8" height="8" rx="1"/>
                                    <rect x="13" y="13" width="8" height="8" rx="1"/>
                                </svg>
                            </button>
                            <button class="view-mode-btn" data-view="compact" onclick="window.switchEndpointViewMode('compact')" title="${t('endpoints.viewCompact')}">
                                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                                    <rect x="3" y="4" width="18" height="3" rx="1"/>
                                    <rect x="3" y="10.5" width="18" height="3" rx="1"/>
                                    <rect x="3" y="17" width="18" height="3" rx="1"/>
                                </svg>
                            </button>
                        </div>
                    </div>
                    <div style="display: flex; gap: 10px;">
                        <button id="testAllEndpointsBtn" class="btn btn-secondary" onclick="window.testAllEndpointsAndOptimize && window.testAllEndpointsAndOptimize()">
                            🔍 ${t('monitor.testAllEndpoints')}
                        </button>
                        <button class="btn btn-secondary" onclick="window.showInteractionsModal()">
                            📝 ${t('interactions.viewInteractions')}
                        </button>
                        <button class="btn btn-secondary" onclick="window.showConnectedClientsModal()">
                            👥 ${t('clients.viewClients')}
                        </button>
                        <button class="btn btn-secondary" onclick="window.showDataSyncDialog()">
                          🔄 ${t('webdav.dataSync')}
                        </button>
                        <button class="btn btn-secondary" onclick="window.showImportExportModal()">
                         📦 ${t('endpoints.export')}/${t('endpoints.import')}
                        </button>
                        <button class="btn btn-primary" onclick="window.showAddEndpointModal()">
                            ➕ ${t('header.addEndpoint')}
                        </button>
                    </div>
                </div>
                <div id="endpointPanel" class="endpoint-panel">
                    <div id="endpointList" class="endpoint-list">
                        <div class="loading">${t('endpoints.title')}...</div>
                    </div>
                </div>
            </div>

            <!-- Logs Panel -->
            <div class="card">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                    <div style="display: flex; align-items: center; gap: 15px;">
                        <h2 style="margin: 0;">📋 ${t('logs.title')}</h2>
                        <button class="endpoint-toggle-btn" onclick="window.toggleLogPanel()">
                            <span id="logToggleIcon">🔼</span> <span id="logToggleText">${t('logs.collapse')}</span>
                        </button>
                    </div>
                    <div style="display: flex; gap: 10px;">
                        <select id="logLevel" class="log-level-select-btn" onchange="window.changeLogLevel()">
                            <option value="0">🔍 ${t('logs.levels.0')}</option>
                            <option value="1" selected>ℹ️ ${t('logs.levels.1')}</option>
                            <option value="2">⚠️ ${t('logs.levels.2')}</option>
                            <option value="3">❌ ${t('logs.levels.3')}</option>
                        </select>
                        <button class="btn btn-secondary btn-sm" onclick="window.copyLogs()">
                            📋 ${t('logs.copy')}
                        </button>
                        <button class="btn btn-secondary btn-sm" onclick="window.clearLogs()">
                            🗑️ ${t('logs.clear')}
                        </button>
                    </div>
                </div>
                <div id="logPanel" class="log-panel">
                    <textarea id="logContent" class="log-textarea" readonly></textarea>
                </div>
            </div>
        </div>

        <!-- Footer -->
        <div class="footer">
            <div class="footer-content">
                <div class="footer-left">
                    <span style="opacity: 0.8;">© 2025 ccNexus</span>
                </div>
                <div class="footer-center">
                    <div class="tips-container">
                        <span id="scrollingTip" class="tip-scroll"></span>
                    </div>
                </div>
                <div class="footer-right">
                    <span style="opacity: 0.7; margin-right: 5px;">v</span>
                    <span id="appVersion" style="font-weight: 500;">1.0.0</span>
                </div>
            </div>
        </div>

        <!-- Add/Edit Endpoint Modal -->
        <div id="endpointModal" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h2 id="modalTitle">➕ ${t('modal.addEndpoint')}</h2>
                    <button class="modal-close" onclick="window.closeModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label><span class="required">*</span>${t('modal.name')}</label>
                        <input type="text" id="endpointName" placeholder="${t('modal.namePlaceholder')}">
                    </div>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('modal.apiUrl')}</label>
                        <input type="text" id="endpointUrl" placeholder="${t('modal.apiUrlPlaceholder')}">
                    </div>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('modal.apiKey')}</label>
                        <div class="password-input-wrapper">
                            <input type="password" id="endpointKey" placeholder="${t('modal.apiKeyPlaceholder')}">
                            <button type="button" class="password-toggle" onclick="window.togglePasswordVisibility()" title="${t('modal.togglePassword')}">
                                <svg id="eyeIcon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                                    <circle cx="12" cy="12" r="3"></circle>
                                </svg>
                            </button>
                        </div>
                    </div>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('modal.transformer')}</label>
                        <select id="endpointTransformer" onchange="window.handleTransformerChange()">
                            <option value="claude">Claude (Default)</option>
                            <option value="openai">OpenAI</option>
                            <option value="openai2">OpenAI2 (Responses API)</option>
                            <option value="gemini">Gemini</option>
                        </select>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('modal.transformerHelp')}
                        </p>
                    </div>
                    <div class="form-group" id="modelFieldGroup" style="display: block;">
                        <label><span class="required" id="modelRequired" style="display: none;">*</span>${t('modal.model')}</label>
                        <div class="model-input-wrapper">
                            <div class="model-select-container">
                                <input type="text" id="endpointModel" placeholder="${t('modal.modelPlaceholder')}" autocomplete="off">
                                <button type="button" class="model-dropdown-toggle" onclick="window.toggleModelDropdown()">
                                    <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
                                        <path d="M2 4L6 8L10 4" stroke="currentColor" stroke-width="2" fill="none"/>
                                    </svg>
                                </button>
                                <div class="model-dropdown" id="modelDropdown"></div>
                            </div>
                            <button type="button" class="btn btn-secondary" id="fetchModelsBtn" onclick="window.fetchModels()" title="${t('modal.fetchModels')}">
                                <span id="fetchModelsIcon">${t('modal.fetchModelsBtn')}</span>
                            </button>
                        </div>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;" id="modelHelpText">
                            ${t('modal.modelHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('modal.remark')}</label>
                        <input type="text" id="endpointRemark" placeholder="${t('modal.remarkHelp')}">
                    </div>
                    <div class="form-group">
                        <label>${t('modal.tags')}</label>
                        <input type="text" id="endpointTags" placeholder="${t('modal.tagsPlaceholder')}">
                        <p class="form-help">${t('modal.tagsHelp')}</p>
                    </div>

                    <!-- 智能路由高级设置 -->
                    <div class="form-section-divider" onclick="window.toggleRoutingSettings()">
                        <span class="divider-label">${t('modal.routingSettings') || '智能路由设置'}</span>
                        <span class="divider-icon" id="routingSettingsIcon">▶</span>
                    </div>
                    <div id="routingSettingsPanel" class="routing-settings-panel" style="display: none;">
                        <div class="form-group">
                            <label>${t('modal.modelPatterns') || '模型匹配模式'}</label>
                            <input type="text" id="endpointModelPatterns" placeholder="${t('modal.modelPatternsPlaceholder') || 'claude-*,gpt-4*'}">
                            <p class="form-help">${t('modal.modelPatternsHelp') || '逗号分隔，支持通配符 * 如 claude-*,gpt-4*'}</p>
                        </div>
                        <div class="form-row">
                            <div class="form-group form-group-half">
                                <label>${t('modal.costPerInputToken') || '输入成本 ($/M)'}</label>
                                <input type="number" id="endpointCostInput" step="0.01" min="0" placeholder="0.00">
                                <p class="form-help">${t('modal.costHelp') || '每百万 Token 美元'}</p>
                            </div>
                            <div class="form-group form-group-half">
                                <label>${t('modal.costPerOutputToken') || '输出成本 ($/M)'}</label>
                                <input type="number" id="endpointCostOutput" step="0.01" min="0" placeholder="0.00">
                            </div>
                        </div>
                        <div class="form-row">
                            <div class="form-group form-group-half">
                                <label>${t('modal.quotaLimit') || 'Token 配额'}</label>
                                <input type="number" id="endpointQuotaLimit" min="0" placeholder="${t('modal.quotaUnlimited') || '0 = 无限制'}">
                            </div>
                            <div class="form-group form-group-half">
                                <label>${t('modal.quotaResetCycle') || '重置周期'}</label>
                                <select id="endpointQuotaResetCycle">
                                    <option value="">${t('modal.quotaNever') || '不重置'}</option>
                                    <option value="daily">${t('modal.quotaDaily') || '每日'}</option>
                                    <option value="weekly">${t('modal.quotaWeekly') || '每周'}</option>
                                    <option value="monthly">${t('modal.quotaMonthly') || '每月'}</option>
                                </select>
                            </div>
                        </div>
                        <div class="form-group">
                            <label>${t('modal.priority') || '优先级'}</label>
                            <input type="number" id="endpointPriority" min="1" max="999" placeholder="100">
                            <p class="form-help">${t('modal.priorityHelp') || '数字越小优先级越高，默认100'}</p>
                        </div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="window.closeModal()">${t('modal.cancel')}</button>
                    <button class="btn btn-primary" onclick="window.saveEndpoint()">${t('modal.save')}</button>
                </div>
            </div>
        </div>

        <!-- Edit Port Modal -->
        <div id="portModal" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h2>⚙️ ${t('modal.changePort')}</h2>
                    <button class="modal-close" onclick="window.closePortModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label><span class="required">*</span>${t('modal.portLabel')}</label>
                        <input type="number" id="portInput" min="1" max="65535" placeholder="3003">
                    </div>
                    <p style="color: #666; font-size: 14px; margin-top: 10px;">
                        ⚠️ ${t('modal.portNote')}
                    </p>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="window.closePortModal()">${t('modal.cancel')}</button>
                    <button class="btn btn-primary" onclick="window.savePort()">${t('modal.save')}</button>
                </div>
            </div>
        </div>

        <!-- Welcome Modal -->
        <div id="welcomeModal" class="modal">
            <div class="modal-content" style="max-width: min(600px, 90vw);">
                <div class="modal-header">
                    <h2>👋 ${t('welcome.title')}</h2>
                    <button class="modal-close" onclick="window.closeWelcomeModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div style="margin-bottom: 25px;">
                        <h3 style="font-size: 18px; margin-bottom: 12px; color: var(--primary-color);">🚀 ${t('welcome.whatIs')}</h3>
                        <p style="font-size: 15px; line-height: 1.8; color: var(--text-primary);">
                            ${t('welcome.description')}
                        </p>
                    </div>

                    <div style="margin-bottom: 25px;">
                        <h3 style="font-size: 18px; margin-bottom: 12px; color: var(--primary-color);">✨ ${t('welcome.features')}</h3>
                        <ul style="font-size: 14px; line-height: 2; color: var(--text-primary); padding-left: 20px;">
                            <li>${t('welcome.feature1')}</li>
                            <li>${t('welcome.feature2')}</li>
                            <li>${t('welcome.feature3')}</li>
                            <li>${t('welcome.feature4')}</li>
                            <li>${t('welcome.feature5')}</li>
                        </ul>
                    </div>

                    <div style="margin-bottom: 25px;">
                        <h3 style="font-size: 18px; margin-bottom: 12px; color: var(--primary-color);">🎯 ${t('welcome.quickStart')}</h3>
                        <ol style="font-size: 14px; line-height: 2; color: var(--text-primary); padding-left: 20px;">
                            <li>${t('welcome.step1')}</li>
                            <li>${t('welcome.step2')}</li>
                            <li>${t('welcome.step3')}</li>
                            <li>${t('welcome.step4')}</li>
                        </ol>
                    </div>

                    <div style="display: flex; gap: 15px; justify-content: center; margin-top: 25px;">
                        <button class="btn btn-secondary" onclick="window.openGitHub()">
                            ${t('welcome.viewGitHub')}
                        </button>
                        <button class="btn btn-secondary" onclick="window.showChangelogModal()">
                            ${t('welcome.changelog')}
                        </button>
                    </div>
                </div>
                <div class="modal-footer" style="display: flex; justify-content: flex-end; align-items: center; gap: 20px;">
                    <label style="display: flex; align-items: center; cursor: pointer;">
                        <input type="checkbox" id="dontShowAgain" style="margin-right: 8px;">
                        <span style="font-size: 14px; color: #666;">${t('welcome.dontShow')}</span>
                    </label>
                    <button class="btn btn-primary" onclick="window.closeWelcomeModal()">${t('welcome.getStarted')}</button>
                </div>
            </div>
        </div>

        <!-- Test Result Modal -->
        <div id="testResultModal" class="modal">
            <div class="modal-content" style="max-width: min(600px, 90vw);">
                <div class="modal-header">
                    <h2 id="testResultTitle">🧪 ${t('test.title')}</h2>
                    <button class="modal-close" onclick="window.closeTestResultModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div id="testResultContent" style="font-size: 14px; line-height: 1.6;">
                        <!-- Test result will be inserted here -->
                    </div>
                </div>
            </div>
        </div>

        <!-- Changelog Modal -->
        <div id="changelogModal" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h2>📋 ${t('changelog.title')}</h2>
                    <button class="modal-close" onclick="window.closeChangelogModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div id="changelogContent" style="font-size: 14px; line-height: 1.8;">
                    </div>
                </div>
            </div>
        </div>

        <!-- Error Toast -->
        <div id="errorToast" class="error-toast">
            <div class="error-toast-content">
                <span class="error-toast-icon">⚠️</span>
                <span id="errorToastMessage"></span>
            </div>
        </div>

        <!-- Confirm Dialog -->
        <div id="confirmDialog" class="modal">
            <div class="confirm-dialog-content">
                <div class="confirm-body">
                    <div class="confirm-icon">
                        <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M12 9v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                        </svg>
                    </div>
                    <div class="confirm-content">
                        <h4 class="confirm-title">${t('common.confirmDeleteTitle')}</h4>
                        <p id="confirmMessage" class="confirm-message"></p>
                    </div>
                </div>
                <div class="confirm-divider"></div>
                <div class="confirm-footer">
                    <button class="btn-confirm-delete" onclick="window.acceptConfirm()">${t('common.delete')}</button>
                    <button class="btn-confirm-cancel" onclick="window.cancelConfirm()">${t('common.cancel')}</button>
                </div>
            </div>
        </div>

        <!-- Close Action Dialog -->
        <div id="closeActionDialog" class="modal">
            <div class="confirm-dialog-content">
                <div class="confirm-body">
                    <div class="confirm-icon" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);">
                        <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M6 18L18 6M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                        </svg>
                    </div>
                    <div class="confirm-content">
                        <h4 class="confirm-title">关闭窗口</h4>
                        <p class="confirm-message">您希望如何处理？</p>
                    </div>
                </div>
                <div class="confirm-divider"></div>
                <div class="confirm-footer">
                    <button class="btn-confirm-delete" onclick="window.quitApplication()">退出程序</button>
                    <button class="btn-confirm-cancel" onclick="window.minimizeToTray()">最小化到托盘</button>
                </div>
            </div>
        </div>

        <!-- Settings Modal -->
        <div id="settingsModal" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h2>⚙️ ${t('settings.title')}</h2>
                    <button class="modal-close" onclick="window.closeSettingsModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label><span class="required">*</span>${t('settings.language')}</label>
                        <select id="settingsLanguage">
                            <option value="zh-CN">${t('settings.languages.zh-CN')}</option>
                            <option value="en">${t('settings.languages.en')}</option>
                        </select>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.languageHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('settings.theme')}</label>
                        <div style="display: flex; align-items: center; gap: 12px;">
                            <select id="settingsTheme" style="flex: 1;">
                                <option value="light">${t('settings.themes.light')}</option>
                                <option value="dark">${t('settings.themes.dark')}</option>
                                <option value="green">${t('settings.themes.green')}</option>
                                <option value="starry">${t('settings.themes.starry')}</option>
                                <option value="sakura">${t('settings.themes.sakura')}</option>
                                <option value="sunset">${t('settings.themes.sunset')}</option>
                                <option value="ocean">${t('settings.themes.ocean')}</option>
                                <option value="mocha">${t('settings.themes.mocha')}</option>
                                <option value="cyberpunk">${t('settings.themes.cyberpunk')}</option>
                                <option value="aurora">${t('settings.themes.aurora')}</option>
                                <option value="holographic">${t('settings.themes.holographic')}</option>
                                <option value="quantum">${t('settings.themes.quantum')}</option>
                            </select>
                            <div style="display: flex; align-items: center; gap: 8px; white-space: nowrap;" title="${t('settings.themeAutoHelp')}">
                                <span style="font-size: 13px; color: var(--text-secondary);">${t('settings.themeAuto')}</span>
                                <label class="toggle-switch" style="width: 40px; height: 20px; margin-top: 7px;">
                                    <input type="checkbox" id="settingsThemeAuto">
                                    <span class="toggle-slider" style="border-radius: 20px;"></span>
                                </label>
                            </div>
                        </div>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.themeHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('settings.closeWindowBehavior')}</label>
                        <select id="settingsCloseWindowBehavior">
                            <option value="quit">${t('settings.closeWindowOptions.quit')}</option>
                            <option value="ask">${t('settings.closeWindowOptions.ask')}</option>
                            <option value="minimize">${t('settings.closeWindowOptions.minimize')}</option>
                        </select>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.closeWindowBehaviorHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.proxy')}</label>
                        <input type="text" id="settingsProxyUrl" placeholder="${t('settings.proxyUrlPlaceholder')}">
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.proxyHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.healthCheck')}</label>
                        <select id="settingsHealthCheckInterval">
                       <option value="0">${t('settings.healthCheckOptions.disabled')}</option>
                            <option value="30">${t('settings.healthCheckOptions.sec30')}</option>
                  <option value="60">${t('settings.healthCheckOptions.min1')}</option>
                            <option value="300">${t('settings.healthCheckOptions.min5')}</option>
                            <option value="600">${t('settings.healthCheckOptions.min10')}</option>
                        </select>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                       ${t('settings.healthCheckHelp')}
                        </p>
                 </div>
                    <div class="form-group">
                        <label>${t('settings.requestTimeout')}</label>
                        <select id="settingsRequestTimeout">
                            <option value="0">${t('settings.requestTimeoutOptions.default')}</option>
                            <option value="60">${t('settings.requestTimeoutOptions.min1')}</option>
                            <option value="120">${t('settings.requestTimeoutOptions.min2')}</option>
                            <option value="180">${t('settings.requestTimeoutOptions.min3')}</option>
                            <option value="300">${t('settings.requestTimeoutOptions.min5')}</option>
                            <option value="600">${t('settings.requestTimeoutOptions.min10')}</option>
                            <option value="900">${t('settings.requestTimeoutOptions.min15')}</option>
                            <option value="1800">${t('settings.requestTimeoutOptions.min30')}</option>
                        </select>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.requestTimeoutHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.healthHistoryRetention')}</label>
                        <select id="settingsHealthHistoryRetention">
                            <option value="1">1 ${t('settings.healthHistoryRetentionDays')}</option>
                            <option value="3">3 ${t('settings.healthHistoryRetentionDays')}</option>
                            <option value="7">7 ${t('settings.healthHistoryRetentionDays')}</option>
                            <option value="14">14 ${t('settings.healthHistoryRetentionDays')}</option>
                            <option value="30">30 ${t('settings.healthHistoryRetentionDays')}</option>
                        </select>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.healthHistoryRetentionHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.alertConfig')}</label>
                        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                            <span style="font-size: 13px; color: var(--text-secondary);">${t('settings.alertEnabled')}</span>
                            <label class="toggle-switch" style="width: 40px; height: 20px; margin-top: 7px;">
                                <input type="checkbox" id="settingsAlertEnabled">
                                <span class="toggle-slider" style="border-radius: 20px;"></span>
                            </label>
                        </div>
                        <div id="alertConfigDetails" style="display: none; padding: 10px; background: var(--bg-secondary); border-radius: 8px;">
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.alertConsecutiveFailures')}</label>
                                <select id="settingsAlertConsecutiveFailures" style="width: 100%; margin-top: 5px;">
                                    <option value="1">1 ${t('settings.alertTimes')}</option>
                                    <option value="2">2 ${t('settings.alertTimes')}</option>
                                    <option value="3">3 ${t('settings.alertTimes')}</option>
                                    <option value="5">5 ${t('settings.alertTimes')}</option>
                                    <option value="10">10 ${t('settings.alertTimes')}</option>
                                </select>
                            </div>
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.alertCooldown')}</label>
                                <select id="settingsAlertCooldown" style="width: 100%; margin-top: 5px;">
                                    <option value="1">1 ${t('settings.alertMinutes')}</option>
                                    <option value="5">5 ${t('settings.alertMinutes')}</option>
                                    <option value="10">10 ${t('settings.alertMinutes')}</option>
                                    <option value="30">30 ${t('settings.alertMinutes')}</option>
                                    <option value="60">60 ${t('settings.alertMinutes')}</option>
                                </select>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                                <input type="checkbox" id="settingsAlertNotifyOnRecovery" checked>
                                <span style="font-size: 13px;">${t('settings.alertNotifyOnRecovery')}</span>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px;">
                                <input type="checkbox" id="settingsAlertSystemNotification" checked>
                                <span style="font-size: 13px;">${t('settings.alertSystemNotification')}</span>
                            </div>
                            <div style="margin-top: 15px; padding-top: 10px; border-top: 1px solid var(--border-color);">
                                <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                                    <input type="checkbox" id="settingsPerformanceAlertEnabled">
                                    <span style="font-size: 13px;">${t('settings.performanceAlertEnabled')}</span>
                                </div>
                                <div id="performanceAlertDetails" style="display: none;">
                                    <div style="margin-bottom: 10px;">
                                        <label style="font-size: 13px;">${t('settings.performanceLatencyThreshold')}</label>
                                        <select id="settingsLatencyThreshold" style="width: 100%; margin-top: 5px;">
                                            <option value="2000">2000 ${t('settings.performanceLatencyMs')}</option>
                                            <option value="3000">3000 ${t('settings.performanceLatencyMs')}</option>
                                            <option value="5000">5000 ${t('settings.performanceLatencyMs')}</option>
                                            <option value="10000">10000 ${t('settings.performanceLatencyMs')}</option>
                                            <option value="15000">15000 ${t('settings.performanceLatencyMs')}</option>
                                        </select>
                                    </div>
                                    <div style="margin-bottom: 10px;">
                                        <label style="font-size: 13px;">${t('settings.performanceLatencyIncrease')}</label>
                                        <select id="settingsLatencyIncrease" style="width: 100%; margin-top: 5px;">
                                            <option value="100">100 ${t('settings.performanceLatencyPercent')}</option>
                                            <option value="150">150 ${t('settings.performanceLatencyPercent')}</option>
                                            <option value="200">200 ${t('settings.performanceLatencyPercent')}</option>
                                            <option value="300">300 ${t('settings.performanceLatencyPercent')}</option>
                                            <option value="500">500 ${t('settings.performanceLatencyPercent')}</option>
                                        </select>
                                    </div>
                                </div>
                                <p style="color: #666; font-size: 12px; margin-top: 5px;">
                                    ${t('settings.performanceAlertHelp')}
                                </p>
                            </div>
                            <div style="margin-top: 15px; padding-top: 10px; border-top: 1px solid var(--border-color);">
                                <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                                    <input type="checkbox" id="settingsAutoEnableOnRecovery">
                                    <span style="font-size: 13px;">${t('settings.autoEnableOnRecovery')}</span>
                                </div>
                                <div style="margin-bottom: 10px;">
                                    <label style="font-size: 13px;">${t('settings.autoEnableSuccessThreshold')}</label>
                                    <select id="settingsAutoEnableSuccessThreshold" style="width: 100%; margin-top: 5px;">
                                        <option value="1">1 ${t('settings.alertTimes')}</option>
                                        <option value="2">2 ${t('settings.alertTimes')}</option>
                                        <option value="3">3 ${t('settings.alertTimes')}</option>
                                        <option value="5">5 ${t('settings.alertTimes')}</option>
                                        <option value="10">10 ${t('settings.alertTimes')}</option>
                                    </select>
                                </div>
                                <p style="color: #666; font-size: 12px; margin-top: 5px;">
                                    ${t('settings.autoEnableHelp')}
                                </p>
                            </div>
                        </div>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.alertConfigHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.sessionAffinityConfig')}</label>
                        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                            <span style="font-size: 13px; color: var(--text-secondary);">${t('settings.sessionAffinityEnabled')}</span>
                            <label class="toggle-switch" style="width: 40px; height: 20px; margin-top: 7px;">
                                <input type="checkbox" id="settingsSessionAffinityEnabled">
                                <span class="toggle-slider" style="border-radius: 20px;"></span>
                            </label>
                        </div>
                        <div id="sessionAffinityConfigDetails" style="display: none; padding: 10px; background: var(--bg-secondary); border-radius: 8px;">
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.sessionAffinityTimeout')}</label>
                                <select id="settingsSessionAffinityTimeout" style="width: 100%; margin-top: 5px;">
                                    <option value="1">1 ${t('settings.sessionAffinityTimeoutHours')}</option>
                                    <option value="6">6 ${t('settings.sessionAffinityTimeoutHours')}</option>
                                    <option value="12">12 ${t('settings.sessionAffinityTimeoutHours')}</option>
                                    <option value="24">24 ${t('settings.sessionAffinityTimeoutHours')}</option>
                                    <option value="48">48 ${t('settings.sessionAffinityTimeoutHours')}</option>
                                    <option value="72">72 ${t('settings.sessionAffinityTimeoutHours')}</option>
                                </select>
                            </div>
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.sessionAffinityMaxConcurrent')}</label>
                                <select id="settingsSessionAffinityMaxConcurrent" style="width: 100%; margin-top: 5px;">
                                    <option value="0">0 (${t('settings.sessionAffinityMaxConcurrentHelp')})</option>
                                    <option value="1">1</option>
                                    <option value="2">2</option>
                                    <option value="3">3</option>
                                    <option value="5">5</option>
                                    <option value="10">10</option>
                                </select>
                            </div>
                        </div>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.sessionAffinityHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.cacheConfig')}</label>
                        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                            <span style="font-size: 13px; color: var(--text-secondary);">${t('settings.cacheEnabled')}</span>
                            <label class="toggle-switch" style="width: 40px; height: 20px; margin-top: 7px;">
                                <input type="checkbox" id="settingsCacheEnabled">
                                <span class="toggle-slider" style="border-radius: 20px;"></span>
                            </label>
                        </div>
                        <div id="cacheConfigDetails" style="display: none; padding: 10px; background: var(--bg-secondary); border-radius: 8px;">
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.cacheTTL')}</label>
                                <select id="settingsCacheTTL" style="width: 100%; margin-top: 5px;">
                                    <option value="60">60 ${t('settings.cacheSeconds')}</option>
                                    <option value="120">120 ${t('settings.cacheSeconds')}</option>
                                    <option value="300">300 ${t('settings.cacheSeconds')}</option>
                                    <option value="600">600 ${t('settings.cacheSeconds')}</option>
                                    <option value="1800">1800 ${t('settings.cacheSeconds')}</option>
                                    <option value="3600">3600 ${t('settings.cacheSeconds')}</option>
                                </select>
                            </div>
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.cacheMaxEntries')}</label>
                                <select id="settingsCacheMaxEntries" style="width: 100%; margin-top: 5px;">
                                    <option value="100">100 ${t('settings.cacheEntries')}</option>
                                    <option value="500">500 ${t('settings.cacheEntries')}</option>
                                    <option value="1000">1000 ${t('settings.cacheEntries')}</option>
                                    <option value="2000">2000 ${t('settings.cacheEntries')}</option>
                                    <option value="5000">5000 ${t('settings.cacheEntries')}</option>
                                </select>
                            </div>
                            <div style="margin-top: 15px; padding-top: 10px; border-top: 1px solid var(--border-color);">
                                <label style="font-size: 13px; margin-bottom: 8px; display: block;">${t('settings.cacheStats')}</label>
                                <div id="cacheStatsDisplay" style="font-size: 12px; color: var(--text-secondary);">
                                    <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
                                        <span>${t('settings.cacheTotalEntries')}:</span>
                                        <span id="cacheStatEntries">0</span>
                                    </div>
                                    <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
                                        <span>${t('settings.cacheTotalHits')}:</span>
                                        <span id="cacheStatHits">0</span>
                                    </div>
                                    <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
                                        <span>${t('settings.cacheTotalMisses')}:</span>
                                        <span id="cacheStatMisses">0</span>
                                    </div>
                                    <div style="display: flex; justify-content: space-between; margin-bottom: 8px;">
                                        <span>${t('settings.cacheTotalSize')}:</span>
                                        <span id="cacheStatSize">0 B</span>
                                    </div>
                                </div>
                                <button class="btn btn-secondary" style="width: 100%; padding: 6px;" onclick="window.clearCache()">${t('settings.cacheClear')}</button>
                            </div>
                        </div>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.cacheConfigHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.rateLimitConfig')}</label>
                        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                            <span style="font-size: 13px; color: var(--text-secondary);">${t('settings.rateLimitEnabled')}</span>
                            <label class="toggle-switch" style="width: 40px; height: 20px; margin-top: 7px;">
                                <input type="checkbox" id="settingsRateLimitEnabled">
                                <span class="toggle-slider" style="border-radius: 20px;"></span>
                            </label>
                        </div>
                        <div id="rateLimitConfigDetails" style="display: none; padding: 10px; background: var(--bg-secondary); border-radius: 8px;">
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.rateLimitGlobal')}</label>
                                <select id="settingsRateLimitGlobal" style="width: 100%; margin-top: 5px;">
                                    <option value="30">30 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="60">60 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="120">120 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="300">300 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="600">600 ${t('settings.rateLimitRequestsPerMin')}</option>
                                </select>
                            </div>
                            <div style="margin-bottom: 10px;">
                                <label style="font-size: 13px;">${t('settings.rateLimitPerEndpoint')}</label>
                                <select id="settingsRateLimitPerEndpoint" style="width: 100%; margin-top: 5px;">
                                    <option value="15">15 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="30">30 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="60">60 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="120">120 ${t('settings.rateLimitRequestsPerMin')}</option>
                                    <option value="300">300 ${t('settings.rateLimitRequestsPerMin')}</option>
                                </select>
                            </div>
                            <div style="margin-top: 15px; padding-top: 10px; border-top: 1px solid var(--border-color);">
                                <label style="font-size: 13px; margin-bottom: 8px; display: block;">${t('settings.rateLimitStats')}</label>
                                <div id="rateLimitStatsDisplay" style="font-size: 12px; color: var(--text-secondary);">
                                    <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
                                        <span>${t('settings.rateLimitCurrentRpm')}:</span>
                                        <span id="rateLimitStatRpm">0</span>
                                    </div>
                                    <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
                                        <span>${t('settings.rateLimitAllowed')}:</span>
                                        <span id="rateLimitStatAllowed">0</span>
                                    </div>
                                    <div style="display: flex; justify-content: space-between; margin-bottom: 8px;">
                                        <span>${t('settings.rateLimitRejected')}:</span>
                                        <span id="rateLimitStatRejected">0</span>
                                    </div>
                                </div>
                                <button class="btn btn-secondary" style="width: 100%; padding: 6px;" onclick="window.resetRateLimitStats()">${t('settings.rateLimitReset')}</button>
                            </div>
                        </div>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.rateLimitConfigHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label>${t('settings.routingConfig')}</label>
                        <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                            <span style="font-size: 13px; color: var(--text-secondary);">${t('settings.routingEnabled')}</span>
                            <label class="toggle-switch" style="width: 40px; height: 20px; margin-top: 7px;">
                                <input type="checkbox" id="settingsRoutingEnabled">
                                <span class="toggle-slider" style="border-radius: 20px;"></span>
                            </label>
                        </div>
                        <div id="routingConfigDetails" style="display: none; padding: 10px; background: var(--bg-secondary); border-radius: 8px;">
                            <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                                <input type="checkbox" id="settingsModelRouting">
                                <span style="font-size: 13px;">${t('settings.modelRouting')}</span>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                                <input type="checkbox" id="settingsLoadBalance">
                                <span style="font-size: 13px;">${t('settings.loadBalance')}</span>
                            </div>
                            <div id="loadBalanceAlgorithmContainer" style="margin-left: 24px; margin-bottom: 10px; display: none;">
                                <label style="font-size: 12px;">${t('settings.loadBalanceAlgorithm')}</label>
                                <select id="settingsLoadBalanceAlgorithm" style="width: 100%; margin-top: 5px;">
                                    <option value="fastest">${t('settings.loadBalanceAlgorithms.fastest')}</option>
                                    <option value="weighted">${t('settings.loadBalanceAlgorithms.weighted')}</option>
                                    <option value="round_robin">${t('settings.loadBalanceAlgorithms.roundRobin')}</option>
                                </select>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                                <input type="checkbox" id="settingsCostPriority">
                                <span style="font-size: 13px;">${t('settings.costPriority')}</span>
                            </div>
                            <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
                                <input type="checkbox" id="settingsQuotaRouting">
                                <span style="font-size: 13px;">${t('settings.quotaRouting')}</span>
                            </div>
                            <div style="margin-top: 15px; padding-top: 10px; border-top: 1px solid var(--border-color);">
                                <label style="font-size: 13px; margin-bottom: 8px; display: block;">${t('settings.quotaStatus')}</label>
                                <div id="quotaStatusDisplay" style="font-size: 12px; max-height: 150px; overflow-y: auto;">
                                    <p style="color: var(--text-secondary);">${t('settings.quotaStatusLoading')}</p>
                                </div>
                            </div>
                        </div>
                        <p style="color: #666; font-size: 12px; margin-top: 5px;">
                            ${t('settings.routingConfigHelp')}
                        </p>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="window.closeSettingsModal()">${t('settings.cancel')}</button>
                    <button class="btn btn-primary" onclick="window.saveSettings()">${t('settings.save')}</button>
                </div>
            </div>
        </div>

        <!-- Auto Theme Config Modal -->
        <div id="autoThemeConfigModal" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h2>🌓 ${t('settings.autoThemeConfigTitle')}</h2>
                    <button class="modal-close" onclick="window.closeAutoThemeConfigModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <p style="color: var(--text-secondary); font-size: 14px; margin-bottom: 20px;">
                        ${t('settings.autoThemeConfigDesc')}
                    </p>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('settings.autoThemeMode')}</label>
                        <select id="autoThemeMode" onchange="window.updateAutoThemeModeHelp()">
                            <option value="time">${t('settings.autoThemeModeTime')}</option>
                            <option value="system">${t('settings.autoThemeModeSystem')}</option>
                        </select>
                        <p id="autoThemeModeHelp" style="color: var(--text-secondary); font-size: 12px; margin-top: 5px;">
                            ${t('settings.autoThemeModeTimeHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('settings.lightThemeLabel')}</label>
                        <select id="autoLightTheme">
                            <option value="light">${t('settings.themes.light')}</option>
                            <option value="green">${t('settings.themes.green')}</option>
                            <option value="sakura">${t('settings.themes.sakura')}</option>
                            <option value="sunset">${t('settings.themes.sunset')}</option>
                            <option value="ocean">${t('settings.themes.ocean')}</option>
                            <option value="mocha">${t('settings.themes.mocha')}</option>
                        </select>
                        <p style="color: var(--text-secondary); font-size: 12px; margin-top: 5px;">
                            ${t('settings.lightThemeHelp')}
                        </p>
                    </div>
                    <div class="form-group">
                        <label><span class="required">*</span>${t('settings.darkThemeLabel')}</label>
                        <select id="autoDarkTheme">
                            <option value="dark">${t('settings.themes.dark')}</option>
                            <option value="starry">${t('settings.themes.starry')}</option>
                            <option value="cyberpunk">${t('settings.themes.cyberpunk')}</option>
                            <option value="aurora">${t('settings.themes.aurora')}</option>
                            <option value="holographic">${t('settings.themes.holographic')}</option>
                            <option value="quantum">${t('settings.themes.quantum')}</option>
                        </select>
                        <p style="color: var(--text-secondary); font-size: 12px; margin-top: 5px;">
                            ${t('settings.darkThemeHelp')}
                        </p>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="window.closeAutoThemeConfigModal()">${t('settings.cancel')}</button>
                    <button class="btn btn-primary" onclick="window.saveAutoThemeConfig()">${t('settings.save')}</button>
                </div>
            </div>
        </div>

        <!-- Connected Clients Modal -->
        <div id="connectedClientsModal" class="modal">
            <div class="modal-content modal-lg">
                <div class="modal-header">
                    <h2>👥 ${t('clients.title')}</h2>
                    <button class="modal-close" onclick="window.closeConnectedClientsModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="clients-filters">
                        <div style="display: flex; align-items: center; gap: 12px;">
                            <label>${t('clients.timeRange')}:</label>
                            <select id="clientsHoursFilter" onchange="window.changeClientsHoursFilter(this.value)">
                                <option value="1">${t('clients.lastHour')}</option>
                                <option value="6">${t('clients.last6Hours')}</option>
                                <option value="24" selected>${t('clients.last24Hours')}</option>
                            </select>
                        </div>
                        <div style="display: flex; align-items: center; gap: 12px;">
                            <span>${t('clients.count')}: <strong id="clientsCount">0</strong></span>
                            <button class="btn btn-secondary btn-sm" onclick="window.refreshConnectedClients()">
                                🔄 ${t('clients.refresh')}
                            </button>
                        </div>
                    </div>
                    <div class="clients-table-wrapper">
                        <table class="clients-table">
                            <thead>
                                <tr>
                                    <th>${t('clients.ip')}</th>
                                    <th>${t('clients.lastSeen')}</th>
                                    <th>${t('clients.requests')}</th>
                                    <th>${t('clients.inputTokens')}</th>
                                    <th>${t('clients.outputTokens')}</th>
                                    <th>${t('clients.endpoints')}</th>
                                </tr>
                            </thead>
                            <tbody id="clientsTableBody">
                            </tbody>
                        </table>
                    </div>
                    <div id="clientsEmptyState" class="empty-state" style="display: none;">
                        <p>${t('clients.noClients')}</p>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-primary" onclick="window.closeConnectedClientsModal()">${t('clients.close')}</button>
                </div>
            </div>
        </div>

        <!-- Interactions Modal -->
        <div id="interactionsModal" class="modal">
            <div class="modal-content modal-xl">
                <div class="modal-header">
                    <h2>📝 ${t('interactions.title')}</h2>
                    <button class="modal-close" onclick="window.closeInteractionsModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="interactions-toolbar">
                        <div class="interactions-filters">
                            <label class="toggle-label">
                                <input type="checkbox" id="interactionEnabledToggle" onchange="window.toggleInteractionEnabled()">
                                <span>${t('interactions.enableRecording')}</span>
                            </label>
                            <select id="interactionDateSelect" onchange="window.changeInteractionDate()">
                                <option value="">${t('interactions.selectDate')}</option>
                            </select>
                            <button class="btn btn-secondary" onclick="window.exportInteractions()">
                                📥 ${t('interactions.export')}
                            </button>
                        </div>
                    </div>
                    <div class="table-container">
                  <table id="interactionsTable" class="data-table">
                            <thead>
                                <tr>
                                    <th>${t('interactions.time')}</th>
                                    <th>${t('interactions.type')}</th>
                                    <th>${t('interactions.intentType')}</th>
                                    <th style="min-width: 200px;">${t('interactions.messagePreview')}</th>
                                    <th>${t('interactions.toolCalls')}</th>
                                    <th>${t('interactions.endpoint')}</th>
                                    <th>${t('interactions.model')}</th>
                                    <th>${t('interactions.inputTokens')}</th>
                                    <th>${t('interactions.outputTokens')}</th>
                                    <th>${t('interactions.duration')}</th>
                                    <th>${t('interactions.status')}</th>
                                    <th>${t('interactions.actions')}</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr>
                                    <td colspan="12" class="empty-message">${t('interactions.noData')}</td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                    <p class="interactions-hint">${t('interactions.retention')}</p>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-primary" onclick="window.closeInteractionsModal()">${t('interactions.close')}</button>
                </div>
            </div>
        </div>

        <!-- Interaction Detail Modal -->
        <div id="interactionDetailModal" class="modal">
            <div class="modal-content modal-xl">
                <div class="modal-header">
                    <h2>🔍 ${t('interactions.detailTitle')}</h2>
                    <button class="modal-close" onclick="window.closeInteractionDetailModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div id="interactionDetailMeta" class="interaction-meta"></div>
                    <div class="interaction-tabs">
                        <button class="interaction-tab-btn active" data-tab="request-raw" onclick="window.switchDetailTab('request-raw')">
                            ${t('interactions.requestRaw')}
                        </button>
                        <button class="interaction-tab-btn" data-tab="request-transformed" onclick="window.switchDetailTab('request-transformed')">
                            ${t('interactions.requestTransformed')}
                        </button>
                        <button class="interaction-tab-btn" data-tab="response-raw" onclick="window.switchDetailTab('response-raw')">
                            ${t('interactions.responseRaw')}
                        </button>
                        <button class="interaction-tab-btn" data-tab="response-transformed" onclick="window.switchDetailTab('response-transformed')">
                            ${t('interactions.responseTransformed')}
                        </button>
                    </div>
                    <div id="interactionDetailContent" class="interaction-content">
                        <pre class="json-viewer"></pre>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-primary" onclick="window.closeInteractionDetailModal()">${t('interactions.close')}</button>
                </div>
            </div>
        </div>
    `;

    setupModalEventListeners();
}

function setupModalEventListeners() {
    // Close modals on background click (endpointModal, portModal, welcomeModal do NOT close on background click)
     document.getElementById('testResultModal').addEventListener('click', (e) => {
        if (e.target.id === 'testResultModal') {
            window.closeTestResultModal();
        }
    });
}

export async function changeLanguage(lang) {
    try {
        await window.go.main.App.SetLanguage(lang);
        location.reload();
    } catch (error) {
        console.error('Failed to change language:', error);
    }
}
