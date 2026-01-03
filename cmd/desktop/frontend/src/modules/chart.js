import { formatTokens } from '../utils/format.js';
import { t } from '../i18n/index.js';

let chartInstance = null;
let currentGranularity = '5min';
let currentPeriod = 'daily';
let refreshInterval = null;

/**
 * Initialize token trend chart
 * @param {string} period - The period to display ('daily', 'yesterday', 'weekly', 'monthly')
 */
export async function initTokenChart(period = 'daily') {
    const container = document.getElementById('tokenChartContainer');
    if (!container) {
        console.error('Chart container not found');
        return;
    }

    currentPeriod = period;

    // Clear any existing error messages
    const existingError = document.getElementById('chartError');
    if (existingError) {
        existingError.remove();
    }

    // Show loading state
    const parent = container.parentElement;
    const loadingDiv = document.createElement('div');
    loadingDiv.id = 'chartLoading';
    loadingDiv.style.cssText = 'text-align: center; padding: 40px; color: #999; position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%);';
    loadingDiv.innerHTML = `<p style="font-size: 14px;">${t('chart.loading')}</p>`;
    parent.style.position = 'relative';
    parent.appendChild(loadingDiv);

    container.style.display = 'block';

    const ctx = container.getContext('2d');

    try {
        // Load Chart.js dynamically
        const { default: Chart } = await import('chart.js/auto');

        // Destroy existing chart instance
        if (chartInstance) {
            chartInstance.destroy();
            chartInstance = null;
        }

        // Fetch chart data
        const data = await fetchChartData(currentGranularity, currentPeriod);

        // Remove loading indicator before showing results or errors
        const loadingDiv = document.getElementById('chartLoading');
        if (loadingDiv) {
            loadingDiv.remove();
        }

        if (!data.success) {
            showChartError(data.message || t('chart.loadFailed'));
            return;
        }

        // Check if there's any data
        if (!data.data || !data.data.timestamps || data.data.timestamps.length === 0) {
            showChartError(t('chart.noData'));
            return;
        }

        // Build datasets
        const datasets = buildDatasets(data.data);

        // Create chart instance
        chartInstance = new Chart(ctx, {
            type: 'line',
            data: {
                labels: data.data.timestamps,
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'index',
                    intersect: false,
                },
                plugins: {
                    legend: {
                        position: 'top',
                        labels: {
                            usePointStyle: true,
                            padding: 15,
                            font: {
                                size: 11
                            }
                        },
                        onClick: function(e, legendItem, legend) {
                            // Default click handler - toggle dataset visibility
                            const index = legendItem.datasetIndex;
                            const chart = legend.chart;
                            const meta = chart.getDatasetMeta(index);

                            meta.hidden = meta.hidden === null ? !chart.data.datasets[index].hidden : null;
                            chart.update();
                        }
                    },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                let label = context.dataset.label || '';
                                if (label) {
                                    label += ': ';
                                }
                                label += formatTokens(context.parsed.y);
                                return label;
                            }
                        }
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: function(value) {
                                return formatTokens(value);
                            }
                        }
                    },
                    x: {
                        ticks: {
                            maxRotation: 45,
                            minRotation: 0,
                            autoSkip: true,
                            maxTicksLimit: 20
                        }
                    }
                }
            }
        });

        // Update button states based on current period
        updateGranularityButtons();

        // Start auto refresh
        startAutoRefresh();
    } catch (error) {
        console.error('Failed to initialize chart:', error);

        // Remove loading indicator on error
        const loadingDiv = document.getElementById('chartLoading');
        if (loadingDiv) {
            loadingDiv.remove();
        }

        showChartError(t('chart.libraryFailed'));
    }
}

/**
 * Build datasets from API response data
 * @param {object} data - The data object containing endpoints and total
 * @returns {array} Array of Chart.js dataset objects
 */
function buildDatasets(data) {
    const datasets = [];

    // Guard against missing or invalid data
    if (!data || !data.endpoints) {
        return datasets;
    }

    // Predefined colors for different endpoints
    const colors = [
        { border: '#667eea', bg: 'rgba(102, 126, 234, 0.1)' },
        { border: '#f59e0b', bg: 'rgba(245, 158, 11, 0.1)' },
        { border: '#10b981', bg: 'rgba(16, 185, 129, 0.1)' },
        { border: '#ef4444', bg: 'rgba(239, 68, 68, 0.1)' },
        { border: '#8b5cf6', bg: 'rgba(139, 92, 246, 0.1)' }
    ];

    let colorIndex = 0;

    // Create datasets for each endpoint
    for (const [endpointName, endpointData] of Object.entries(data.endpoints || {})) {
        const color = colors[colorIndex % colors.length];

        // Input tokens curve (solid line)
        datasets.push({
            label: `${endpointName} (${t('chart.inputTokens')})`,
            data: endpointData.inputTokens,
            borderColor: color.border,
            backgroundColor: color.bg,
            borderWidth: 1.5,
            pointRadius: 0,
            tension: 0.3,
            fill: false
        });

        // Output tokens curve (dashed line)
        datasets.push({
            label: `${endpointName} (${t('chart.outputTokens')})`,
            data: endpointData.outputTokens,
            borderColor: color.border,
            backgroundColor: color.bg,
            borderWidth: 1.5,
            borderDash: [5, 5],
            pointRadius: 0,
            tension: 0.3,
            fill: false
        });

        colorIndex++;
    }

    // Add total summary curves (bold lines)
    if (data.total) {
        datasets.push({
            label: `${t('chart.total')} ${t('chart.inputTokens')}`,
            data: data.total.inputTokens,
            borderColor: '#764ba2',
            backgroundColor: 'rgba(118, 75, 162, 0.15)',
            borderWidth: 3,
            pointRadius: 0,
            tension: 0.3,
            fill: false
        });

        datasets.push({
            label: `${t('chart.total')} ${t('chart.outputTokens')}`,
            data: data.total.outputTokens,
            borderColor: '#764ba2',
            backgroundColor: 'rgba(118, 75, 162, 0.15)',
            borderWidth: 3,
            borderDash: [5, 5],
            pointRadius: 0,
            tension: 0.3,
            fill: false
        });
    }

    return datasets;
}

/**
 * Fetch chart data from backend API
 * @param {string} granularity - Time granularity ('5min', '30min', 'request')
 * @param {string} period - Time period ('daily', 'yesterday', 'weekly', 'monthly')
 * @returns {object} API response data
 */
async function fetchChartData(granularity, period) {
    try {
        const dataStr = await window.go.main.App.GetTokenTrendData(granularity, period);
        return JSON.parse(dataStr);
    } catch (error) {
        console.error('Failed to fetch chart data:', error);
        return { success: false, message: error.message };
    }
}

/**
 * Switch time granularity
 * @param {string} granularity - New granularity ('5min', '30min', 'request')
 */
export async function switchGranularity(granularity) {
    // Prevent switching to time-based granularity for multi-day periods
    if ((currentPeriod === 'weekly' || currentPeriod === 'monthly') &&
        (granularity === '5min' || granularity === '30min')) {
        console.warn('Time-based granularity not supported for multi-day periods');
        return;
    }

    currentGranularity = granularity;

    // Update button states
    updateGranularityButtons();

    // Reload chart
    await initTokenChart(currentPeriod);
}

/**
 * Update granularity button states and disabled status
 */
function updateGranularityButtons() {
    const isMultiDay = (currentPeriod === 'weekly' || currentPeriod === 'monthly');

    document.querySelectorAll('.granularity-btn').forEach(btn => {
        const granularity = btn.dataset.granularity;
        const isActive = granularity === currentGranularity;
        const shouldDisable = isMultiDay && (granularity === '5min' || granularity === '30min');

        btn.classList.toggle('active', isActive);
        btn.disabled = shouldDisable;
        btn.style.opacity = shouldDisable ? '0.5' : '1';
        btn.style.cursor = shouldDisable ? 'not-allowed' : 'pointer';
    });
}

/**
 * Switch period (synced with stats period)
 * @param {string} period - New period ('daily', 'yesterday', 'weekly', 'monthly')
 */
export async function switchChartPeriod(period) {
    currentPeriod = period;

    // Auto-adjust granularity for multi-day periods
    // 5min and 30min only make sense for single-day views
    if ((period === 'weekly' || period === 'monthly') &&
        (currentGranularity === '5min' || currentGranularity === '30min')) {
        currentGranularity = 'request';
    }

    // Update button states
    updateGranularityButtons();

    await initTokenChart(period);
}

/**
 * Refresh chart data without animation
 */
export async function refreshChartData() {
    if (!chartInstance) {
        // If no chart instance, silently skip refresh
        // This happens when there's no data initially
        return;
    }

    try {
        const data = await fetchChartData(currentGranularity, currentPeriod);
        if (data.success && data.data && data.data.timestamps && data.data.timestamps.length > 0) {
            const datasets = buildDatasets(data.data);
            chartInstance.data.labels = data.data.timestamps;
            chartInstance.data.datasets = datasets;
            chartInstance.update('none'); // Update without animation
        } else if (!data.success || !data.data || data.data.timestamps.length === 0) {
            // Data became unavailable, reinitialize to show error
            await initTokenChart(currentPeriod);
        }
    } catch (error) {
        console.error('Failed to refresh chart:', error);
    }
}

/**
 * Start auto refresh timer
 */
export function startAutoRefresh() {
    stopAutoRefresh();
    // Refresh every 10 seconds
    refreshInterval = setInterval(() => {
        refreshChartData();
    }, 10000);
}

/**
 * Stop auto refresh timer
 */
export function stopAutoRefresh() {
    if (refreshInterval) {
        clearInterval(refreshInterval);
        refreshInterval = null;
    }
}

/**
 * Show error message in chart container
 * @param {string} message - Error message to display
 */
function showChartError(message) {
    const container = document.getElementById('tokenChartContainer');
    if (!container) return;

    // Destroy existing chart if any
    if (chartInstance) {
        chartInstance.destroy();
        chartInstance = null;
    }

    // Stop auto refresh when showing error
    stopAutoRefresh();

    // Remove any existing error message
    const existingError = document.getElementById('chartError');
    if (existingError) {
        existingError.remove();
    }

    // For canvas elements, we need to add an overlay
    const parent = container.parentElement;
    const errorDiv = document.createElement('div');
    errorDiv.id = 'chartError';
    errorDiv.style.cssText = 'text-align: center; padding: 80px 20px; color: #999; position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); width: 100%;';
    errorDiv.innerHTML = `
        <p style="font-size: 14px; margin-bottom: 8px;">📊 ${message}</p>
        <p style="font-size: 12px; opacity: 0.7;">${t('chart.noDataHint')}</p>
    `;

    container.style.display = 'none';
    parent.style.position = 'relative';
    parent.appendChild(errorDiv);
}

// Expose functions to window for onclick handlers
window.switchGranularity = switchGranularity;
