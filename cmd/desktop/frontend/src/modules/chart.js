import { formatTokens } from '../utils/format.js';
import { t } from '../i18n/index.js';

let chartInstance = null;
let currentGranularity = '5min';
let currentPeriod = 'daily';
let currentChartType = 'line';  // 'line' | 'bar'
let customStartTime = null;     // null means auto, otherwise "HH:MM"
let customEndTime = null;       // null means auto, otherwise "HH:MM"
let dataRange = null;           // Store data range info from backend
let refreshInterval = null;
let timeSelectorInitialized = false;

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

    // Initialize time selectors if not done yet
    if (!timeSelectorInitialized) {
        initTimeSelectors();
        timeSelectorInitialized = true;
    }

    // Update time selector visibility based on period and granularity
    updateTimeSelectorVisibility();

    // Clear any existing error messages
    const existingError = document.getElementById('chartError');
    if (existingError) {
        existingError.remove();
    }

    // Show loading state
    const parent = container.parentElement;
    const loadingDiv = document.createElement('div');
    loadingDiv.id = 'chartLoading';
    const textSecondary = getCSSVariable('--text-secondary') || '#999';
    loadingDiv.style.cssText = `text-align: center; padding: 40px; color: ${textSecondary}; flex: 1; display: flex; align-items: center; justify-content: center;`;
    loadingDiv.innerHTML = `<p style="font-size: 14px;">${t('chart.loading')}</p>`;
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

        const isBarChart = currentChartType === 'bar';

        // Get theme colors for chart elements
        const textPrimary = getCSSVariable('--text-primary') || '#333';
        const borderLight = getCSSVariable('--border-light') || '#e0e0e0';

        // Create chart instance
        chartInstance = new Chart(ctx, {
            type: currentChartType,
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
                layout: {
                    padding: {
                        top: 5,
                        bottom: 0,
                        left: 5,
                        right: 5
                    }
                },
                plugins: {
                    legend: {
                        position: 'top',
                        labels: {
                            usePointStyle: true,
                            padding: 10,
                            font: {
                                size: 11
                            },
                            boxWidth: 8,
                            boxHeight: 8,
                            color: textPrimary
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
                        stacked: isBarChart,
                        ticks: {
                            callback: function(value) {
                                return formatTokens(value);
                            },
                            padding: 5,
                            color: textPrimary
                        },
                        grid: {
                            drawBorder: false,
                            color: borderLight
                        }
                    },
                    x: {
                        stacked: isBarChart,
                        ticks: {
                            maxRotation: 45,
                            minRotation: 0,
                            autoSkip: true,
                            maxTicksLimit: 20,
                            padding: 5,
                            color: textPrimary
                        },
                        grid: {
                            drawBorder: false,
                            color: borderLight
                        }
                    }
                }
            }
        });

        // Update button states based on current period
        updateGranularityButtons();
        updateChartTypeButtons();

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
 * Get CSS variable value from root
 * @param {string} varName - CSS variable name (e.g., '--primary-color')
 * @returns {string} The CSS variable value
 */
function getCSSVariable(varName) {
    return getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
}

/**
 * Get chart colors from CSS variables
 * @returns {array} Array of color objects with border and background colors
 */
function getChartColors() {
    const primaryColor = getCSSVariable('--primary-color') || '#667eea';

    // Generate a palette based on primary color and complementary colors
    const colors = [
        { border: primaryColor, bg: `${primaryColor}cc` }, // Primary color with 80% opacity
        { border: '#f59e0b', bg: 'rgba(245, 158, 11, 0.8)' }, // Orange
        { border: '#10b981', bg: 'rgba(16, 185, 129, 0.8)' }, // Green
        { border: '#ef4444', bg: 'rgba(239, 68, 68, 0.8)' },  // Red
        { border: '#8b5cf6', bg: 'rgba(139, 92, 246, 0.8)' }  // Purple
    ];

    return colors;
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

    // Get colors from theme
    const colors = getChartColors();

    let colorIndex = 0;
    const isBarChart = currentChartType === 'bar';

    // Create datasets for each endpoint (merged input + output tokens)
    for (const [endpointName, endpointData] of Object.entries(data.endpoints || {})) {
        const color = colors[colorIndex % colors.length];

        // Merge input and output tokens into total tokens
        const totalTokens = endpointData.inputTokens.map((inputVal, index) => {
            const outputVal = endpointData.outputTokens[index] || 0;
            return inputVal + outputVal;
        });

        const dataset = {
            label: endpointName,
            data: totalTokens,
            borderColor: color.border,
            backgroundColor: isBarChart ? color.border : `rgba(${hexToRgb(color.border)}, 0.1)`,
        };

        if (isBarChart) {
            // Bar chart stacked config
            dataset.stack = 'tokens';
            dataset.borderWidth = 1;
            dataset.borderRadius = 2;
        } else {
            // Line chart config
            dataset.borderWidth = 2;
            dataset.pointRadius = 0;
            dataset.tension = 0.3;
            dataset.fill = false;
        }

        datasets.push(dataset);
        colorIndex++;
    }

    // Add total summary curve only for line chart (bar chart stacking shows total automatically)
    if (!isBarChart && data.total) {
        // Merge total input and output tokens
        const totalTokens = data.total.inputTokens.map((inputVal, index) => {
            const outputVal = data.total.outputTokens[index] || 0;
            return inputVal + outputVal;
        });

        const primaryHover = getCSSVariable('--primary-hover') || '#764ba2';

        datasets.push({
            label: t('chart.total'),
            data: totalTokens,
            borderColor: primaryHover,
            backgroundColor: `${primaryHover}26`, // 15% opacity
            borderWidth: 3,
            pointRadius: 0,
            tension: 0.3,
            fill: false
        });
    }

    return datasets;
}

/**
 * Convert hex color to RGB values
 */
function hexToRgb(hex) {
    const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
    if (result) {
        return `${parseInt(result[1], 16)}, ${parseInt(result[2], 16)}, ${parseInt(result[3], 16)}`;
    }
    return '102, 126, 234';
}

/**
 * Fetch chart data from backend API
 * @param {string} granularity - Time granularity ('5min', '30min', 'request')
 * @param {string} period - Time period ('daily', 'yesterday', 'weekly', 'monthly')
 * @returns {object} API response data
 */
async function fetchChartData(granularity, period) {
    try {
        const startTime = customStartTime || '';
        const endTime = customEndTime || '';
        const dataStr = await window.go.main.App.GetTokenTrendData(granularity, period, startTime, endTime);
        const result = JSON.parse(dataStr);

        // Store data range info and update time selectors
        if (result.success && result.dataRange) {
            dataRange = result.dataRange;
            // Update time selector values if in auto mode
            if (!customStartTime && !customEndTime) {
                updateTimeSelectorValues(result.dataRange.effectiveStart, result.dataRange.effectiveEnd);
            }
        }

        return result;
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
    const textSecondary = getCSSVariable('--text-secondary') || '#999';
    errorDiv.style.cssText = `text-align: center; padding: 80px 20px; color: ${textSecondary}; flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;`;
    errorDiv.innerHTML = `
        <p style="font-size: 14px; margin-bottom: 8px;">📊 ${message}</p>
        <p style="font-size: 12px; opacity: 0.7;">${t('chart.noDataHint')}</p>
    `;

    container.style.display = 'none';
    parent.appendChild(errorDiv);
}

// Expose functions to window for onclick handlers
window.switchGranularity = switchGranularity;

/**
 * Switch chart type
 * @param {string} type - Chart type ('line' or 'bar')
 */
export async function switchChartType(type) {
    if (type === currentChartType) return;

    currentChartType = type;
    updateChartTypeButtons();
    await initTokenChart(currentPeriod);
}

/**
 * Update chart type button states
 */
function updateChartTypeButtons() {
    document.querySelectorAll('.chart-type-btn').forEach(btn => {
        const type = btn.dataset.type;
        btn.classList.toggle('active', type === currentChartType);
    });
}

/**
 * Initialize time selectors with 5-minute intervals
 */
function initTimeSelectors() {
    const startSelect = document.getElementById('chartStartTime');
    const endSelect = document.getElementById('chartEndTime');

    if (!startSelect || !endSelect) return;

    // Generate time options (30-minute intervals)
    const options = [];
    for (let h = 0; h < 24; h++) {
        for (let m = 0; m < 60; m += 30) {
            const time = `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`;
            options.push(time);
        }
    }

    // Populate start time selector
    startSelect.innerHTML = options.map(time =>
        `<option value="${time}">${time}</option>`
    ).join('');

    // Populate end time selector (add 24:00 option)
    const endOptions = [...options, '24:00'];
    endSelect.innerHTML = endOptions.map(time =>
        `<option value="${time}">${time}</option>`
    ).join('');

    // Set default values
    startSelect.value = '00:00';
    endSelect.value = '24:00';
}

/**
 * Update time selector values based on data range
 */
function updateTimeSelectorValues(effectiveStart, effectiveEnd) {
    const startSelect = document.getElementById('chartStartTime');
    const endSelect = document.getElementById('chartEndTime');

    if (!startSelect || !endSelect) return;

    if (effectiveStart) {
        startSelect.value = effectiveStart;
    }
    if (effectiveEnd) {
        endSelect.value = effectiveEnd;
    }
}

/**
 * Update time selector visibility based on period and granularity
 */
function updateTimeSelectorVisibility() {
    const selector = document.getElementById('chartTimeSelector');
    if (!selector) return;

    // Hide time selector for multi-day periods or request granularity
    const isMultiDay = (currentPeriod === 'weekly' || currentPeriod === 'monthly');
    const isRequestGranularity = (currentGranularity === 'request');

    if (isMultiDay || isRequestGranularity) {
        selector.style.display = 'none';
        // Reset custom time when hiding
        customStartTime = null;
        customEndTime = null;
    } else {
        selector.style.display = 'flex';
    }
}

/**
 * Handle time range change from selectors
 */
async function onChartTimeChange() {
    const startSelect = document.getElementById('chartStartTime');
    const endSelect = document.getElementById('chartEndTime');

    if (!startSelect || !endSelect) return;

    const newStart = startSelect.value;
    const newEnd = endSelect.value;

    // Validate: end must be after start
    if (newEnd <= newStart) {
        // Auto-correct: set end to start + 1 hour or 24:00
        const startMinutes = parseTimeToMinutes(newStart);
        const newEndMinutes = Math.min(startMinutes + 60, 24 * 60);
        endSelect.value = minutesToTimeString(newEndMinutes);
    }

    customStartTime = startSelect.value;
    customEndTime = endSelect.value;

    await initTokenChart(currentPeriod);
}

/**
 * Reset time range to auto mode
 */
async function resetChartTimeRange() {
    customStartTime = null;
    customEndTime = null;
    await initTokenChart(currentPeriod);
}

/**
 * Parse time string to minutes
 */
function parseTimeToMinutes(timeStr) {
    if (!timeStr) return 0;
    if (timeStr === '24:00') return 24 * 60;
    const [h, m] = timeStr.split(':').map(Number);
    return h * 60 + m;
}

/**
 * Convert minutes to time string
 */
function minutesToTimeString(minutes) {
    if (minutes >= 24 * 60) return '24:00';
    const h = Math.floor(minutes / 60);
    const m = minutes % 60;
    return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`;
}

// Expose additional functions to window for onclick handlers
window.switchChartType = switchChartType;
window.onChartTimeChange = onChartTimeChange;
window.resetChartTimeRange = resetChartTimeRange;
