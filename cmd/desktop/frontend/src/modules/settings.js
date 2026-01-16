import { t } from '../i18n/index.js';
import { changeLanguage } from './ui.js';
import { destroyFestivalEffects, initFestivalEffects } from './festival.js';

// Auto theme check interval ID
let autoThemeIntervalId = null;
// System theme change listener
let systemThemeMediaQuery = null;

// Apply theme to body element
export function applyTheme(theme) {
    // Remove all theme classes first
    document.body.classList.remove('dark-theme', 'green-theme', 'starry-theme', 'sakura-theme', 'sunset-theme', 'ocean-theme', 'mocha-theme', 'cyberpunk-theme', 'aurora-theme', 'holographic-theme', 'quantum-theme');

    // Apply the selected theme
    if (theme === 'dark') {
        document.body.classList.add('dark-theme');
    } else if (theme === 'green') {
        document.body.classList.add('green-theme');
    } else if (theme === 'starry') {
        document.body.classList.add('starry-theme');
    } else if (theme === 'sakura') {
        document.body.classList.add('sakura-theme');
    } else if (theme === 'sunset') {
        document.body.classList.add('sunset-theme');
    } else if (theme === 'ocean') {
        document.body.classList.add('ocean-theme');
    } else if (theme === 'mocha') {
        document.body.classList.add('mocha-theme');
    } else if (theme === 'cyberpunk') {
        document.body.classList.add('cyberpunk-theme');
    } else if (theme === 'aurora') {
        document.body.classList.add('aurora-theme');
    } else if (theme === 'holographic') {
        document.body.classList.add('holographic-theme');
    } else if (theme === 'quantum') {
        document.body.classList.add('quantum-theme');
    }
    // 'light' theme uses default styles, no class needed

    // 重新初始化节日效果（根据主题判断默认效果）
    destroyFestivalEffects();
    initFestivalEffects();

    // 重新渲染图表以应用新主题颜色
    refreshChartWithTheme();
}

// Get theme based on current time and user's auto theme settings
// Day (7:00-19:00): use auto light theme
// Night: use auto dark theme
async function getTimeBasedTheme() {
    const hour = new Date().getHours();
    const isDaytime = hour >= 7 && hour < 19;

    try {
        if (isDaytime) {
            // Daytime: use auto light theme
            const lightTheme = await window.go.main.App.GetAutoLightTheme();
            return lightTheme || 'light';
        } else {
            // Nighttime: use auto dark theme
            const darkTheme = await window.go.main.App.GetAutoDarkTheme();
            return darkTheme || 'dark';
        }
    } catch (error) {
        console.error('Failed to get auto theme settings:', error);
        // Fallback to simple light/dark switching
        return isDaytime ? 'light' : 'dark';
    }
}

// Get theme based on system preference
async function getSystemTheme() {
    const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;

    try {
        if (prefersDark) {
            const darkTheme = await window.go.main.App.GetAutoDarkTheme();
            return darkTheme || 'dark';
        } else {
            const lightTheme = await window.go.main.App.GetAutoLightTheme();
            return lightTheme || 'light';
        }
    } catch (error) {
        console.error('Failed to get system theme settings:', error);
        return prefersDark ? 'dark' : 'light';
    }
}

// Check and apply auto theme (time-based or system-based)
export async function checkAndApplyAutoTheme() {
    try {
        // Check if we should follow system theme
        const autoMode = await window.go.main.App.GetAutoThemeMode();
        let theme;

        if (autoMode === 'system') {
            theme = await getSystemTheme();
        } else {
            // Default to time-based
            theme = await getTimeBasedTheme();
        }

        applyTheme(theme);
    } catch (error) {
        console.error('Failed to check auto theme:', error);
        // Fallback to time-based light/dark switching
        const hour = new Date().getHours();
        applyTheme((hour >= 7 && hour < 19) ? 'light' : 'dark');
    }
}

// Start auto theme checking (check every minute for time-based, or listen for system changes)
export async function startAutoThemeCheck() {
    // Apply immediately and wait for it to complete
    await checkAndApplyAutoTheme();

    // Clear existing interval if any
    if (autoThemeIntervalId) {
        clearInterval(autoThemeIntervalId);
        autoThemeIntervalId = null;
    }

    // Remove existing system theme listener if any
    if (systemThemeMediaQuery) {
        systemThemeMediaQuery.removeEventListener('change', handleSystemThemeChange);
        systemThemeMediaQuery = null;
    }

    try {
        const autoMode = await window.go.main.App.GetAutoThemeMode();

        if (autoMode === 'system') {
            // Listen for system theme changes
            systemThemeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
            systemThemeMediaQuery.addEventListener('change', handleSystemThemeChange);
        } else {
            // Time-based: check every minute
            autoThemeIntervalId = setInterval(checkAndApplyAutoTheme, 60000);
        }
    } catch (error) {
        // Fallback to time-based checking
        autoThemeIntervalId = setInterval(checkAndApplyAutoTheme, 60000);
    }
}

// Handle system theme change event
async function handleSystemThemeChange() {
    await checkAndApplyAutoTheme();
}

// Stop auto theme checking
export function stopAutoThemeCheck() {
    if (autoThemeIntervalId) {
        clearInterval(autoThemeIntervalId);
        autoThemeIntervalId = null;
    }
    if (systemThemeMediaQuery) {
        systemThemeMediaQuery.removeEventListener('change', handleSystemThemeChange);
        systemThemeMediaQuery = null;
    }
}

// Initialize theme based on settings
export async function initTheme() {
    try {
        const themeAuto = await window.go.main.App.GetThemeAuto();
        if (themeAuto) {
            startAutoThemeCheck();
        } else {
            const theme = await window.go.main.App.GetTheme();
            applyTheme(theme);
        }
    } catch (error) {
        console.error('Failed to init theme:', error);
        const theme = await window.go.main.App.GetTheme();
        applyTheme(theme);
    }
}

// Show settings modal
export async function showSettingsModal() {
    const modal = document.getElementById('settingsModal');
    if (!modal) return;

    // Load current config
    await loadCurrentSettings();

    // Clear confirmed flag when opening settings
    const themeAutoCheckbox = document.getElementById('settingsThemeAuto');
    if (themeAutoCheckbox) {
        delete themeAutoCheckbox.dataset.confirmed;
    }

    // Show modal
    modal.classList.add('active');
}

// Close settings modal
export function closeSettingsModal() {
    const modal = document.getElementById('settingsModal');
    if (modal) {
        modal.classList.remove('active');
    }
}

// Load current settings from backend
async function loadCurrentSettings() {
    try {
        const configStr = await window.go.main.App.GetConfig();
        const config = JSON.parse(configStr);

        // Set close window behavior
        const closeWindowBehavior = config.closeWindowBehavior || 'ask';
        const behaviorSelect = document.getElementById('settingsCloseWindowBehavior');
        if (behaviorSelect) {
            behaviorSelect.value = closeWindowBehavior;
        }

        // Set language
        const language = config.language || 'zh-CN';
        const languageSelect = document.getElementById('settingsLanguage');
        if (languageSelect) {
            languageSelect.value = language;
        }

        // Set theme
        const theme = config.theme || 'light';
        const themeSelect = document.getElementById('settingsTheme');
        if (themeSelect) {
            themeSelect.value = theme;
            // Apply theme immediately when changed
            themeSelect.onchange = function() {
                applyTheme(this.value);
            };
        }

        // Set theme auto
        const themeAuto = config.themeAuto || false;
        const themeAutoCheckbox = document.getElementById('settingsThemeAuto');
        if (themeAutoCheckbox) {
            themeAutoCheckbox.checked = themeAuto;
            // Disable theme select when auto mode is enabled
            if (themeSelect) {
                themeSelect.disabled = themeAuto;
            }
        }

        // Add event listener for auto checkbox
        if (themeAutoCheckbox) {
            themeAutoCheckbox.onchange = async function() {
                if (themeSelect) {
                    themeSelect.disabled = this.checked;
                }

                // If user enables auto mode, show config modal immediately
                if (this.checked) {
                    await showAutoThemeConfigModal();
                }
            };
        }

        // Load proxy URL
        const proxyUrl = await window.go.main.App.GetProxyURL();
        const proxyInput = document.getElementById('settingsProxyUrl');
        if (proxyInput) {
            proxyInput.value = proxyUrl || '';
        }

        // Load health check interval
        const healthCheckInterval = await window.go.main.App.GetHealthCheckInterval();
        const healthCheckSelect = document.getElementById('settingsHealthCheckInterval');
        if (healthCheckSelect) {
            healthCheckSelect.value = healthCheckInterval.toString();
        }

        // Load request timeout
        const requestTimeout = await window.go.main.App.GetRequestTimeout();
        const requestTimeoutSelect = document.getElementById('settingsRequestTimeout');
        if (requestTimeoutSelect) {
            requestTimeoutSelect.value = requestTimeout.toString();
        }

        // Load health history retention days
        const healthHistoryRetention = await window.go.main.App.GetHealthHistoryRetentionDays();
        const healthHistoryRetentionSelect = document.getElementById('settingsHealthHistoryRetention');
        if (healthHistoryRetentionSelect) {
            healthHistoryRetentionSelect.value = healthHistoryRetention.toString();
        }

        // Load alert config
        const alertConfigStr = await window.go.main.App.GetAlertConfig();
        const alertConfig = JSON.parse(alertConfigStr);
        const alertEnabledCheckbox = document.getElementById('settingsAlertEnabled');
        const alertConfigDetails = document.getElementById('alertConfigDetails');
        const alertConsecutiveFailuresSelect = document.getElementById('settingsAlertConsecutiveFailures');
        const alertCooldownSelect = document.getElementById('settingsAlertCooldown');
        const alertNotifyOnRecoveryCheckbox = document.getElementById('settingsAlertNotifyOnRecovery');
        const alertSystemNotificationCheckbox = document.getElementById('settingsAlertSystemNotification');

        if (alertEnabledCheckbox) {
            alertEnabledCheckbox.checked = alertConfig.enabled;
            // Show/hide details based on enabled state
            if (alertConfigDetails) {
                alertConfigDetails.style.display = alertConfig.enabled ? 'block' : 'none';
            }
            // Add event listener for toggle
            alertEnabledCheckbox.onchange = function() {
                if (alertConfigDetails) {
                    alertConfigDetails.style.display = this.checked ? 'block' : 'none';
                }
            };
        }
        if (alertConsecutiveFailuresSelect) {
            alertConsecutiveFailuresSelect.value = (alertConfig.consecutiveFailures || 3).toString();
        }
        if (alertCooldownSelect) {
            alertCooldownSelect.value = (alertConfig.alertCooldownMinutes || 5).toString();
        }
        if (alertNotifyOnRecoveryCheckbox) {
            alertNotifyOnRecoveryCheckbox.checked = alertConfig.notifyOnRecovery !== false;
        }
        if (alertSystemNotificationCheckbox) {
            alertSystemNotificationCheckbox.checked = alertConfig.systemNotification !== false;
        }

        // Load performance alert config
        const performanceAlertEnabledCheckbox = document.getElementById('settingsPerformanceAlertEnabled');
        const performanceAlertDetails = document.getElementById('performanceAlertDetails');
        const latencyThresholdSelect = document.getElementById('settingsLatencyThreshold');
        const latencyIncreaseSelect = document.getElementById('settingsLatencyIncrease');

        if (performanceAlertEnabledCheckbox) {
            performanceAlertEnabledCheckbox.checked = alertConfig.performanceAlertEnabled || false;
            // Show/hide details based on enabled state
            if (performanceAlertDetails) {
                performanceAlertDetails.style.display = alertConfig.performanceAlertEnabled ? 'block' : 'none';
            }
            // Add event listener for toggle
            performanceAlertEnabledCheckbox.onchange = function() {
                if (performanceAlertDetails) {
                    performanceAlertDetails.style.display = this.checked ? 'block' : 'none';
                }
            };
        }
        if (latencyThresholdSelect) {
            latencyThresholdSelect.value = (alertConfig.latencyThresholdMs || 5000).toString();
        }
        if (latencyIncreaseSelect) {
            latencyIncreaseSelect.value = (alertConfig.latencyIncreasePercent || 200).toString();
        }

        // Load cache config
        const cacheConfigStr = await window.go.main.App.GetCacheConfig();
        const cacheConfig = JSON.parse(cacheConfigStr);
        const cacheEnabledCheckbox = document.getElementById('settingsCacheEnabled');
        const cacheConfigDetails = document.getElementById('cacheConfigDetails');
        const cacheTTLSelect = document.getElementById('settingsCacheTTL');
        const cacheMaxEntriesSelect = document.getElementById('settingsCacheMaxEntries');

        if (cacheEnabledCheckbox) {
            cacheEnabledCheckbox.checked = cacheConfig.enabled;
            // Show/hide details based on enabled state
            if (cacheConfigDetails) {
                cacheConfigDetails.style.display = cacheConfig.enabled ? 'block' : 'none';
            }
            // Add event listener for toggle
            cacheEnabledCheckbox.onchange = function() {
                if (cacheConfigDetails) {
                    cacheConfigDetails.style.display = this.checked ? 'block' : 'none';
                }
                // Refresh cache stats when enabled
                if (this.checked) {
                    refreshCacheStats();
                }
            };
        }
        if (cacheTTLSelect) {
            cacheTTLSelect.value = (cacheConfig.ttlSeconds || 300).toString();
        }
        if (cacheMaxEntriesSelect) {
            cacheMaxEntriesSelect.value = (cacheConfig.maxEntries || 1000).toString();
        }

        // Load cache stats if enabled
        if (cacheConfig.enabled) {
            refreshCacheStats();
        }

        // Load rate limit config
        const rateLimitConfigStr = await window.go.main.App.GetRateLimitConfig();
        const rateLimitConfig = JSON.parse(rateLimitConfigStr);
        const rateLimitEnabledCheckbox = document.getElementById('settingsRateLimitEnabled');
        const rateLimitConfigDetails = document.getElementById('rateLimitConfigDetails');
        const rateLimitGlobalSelect = document.getElementById('settingsRateLimitGlobal');
        const rateLimitPerEndpointSelect = document.getElementById('settingsRateLimitPerEndpoint');

        if (rateLimitEnabledCheckbox) {
            rateLimitEnabledCheckbox.checked = rateLimitConfig.enabled;
            // Show/hide details based on enabled state
            if (rateLimitConfigDetails) {
                rateLimitConfigDetails.style.display = rateLimitConfig.enabled ? 'block' : 'none';
            }
            // Add event listener for toggle
            rateLimitEnabledCheckbox.onchange = function() {
                if (rateLimitConfigDetails) {
                    rateLimitConfigDetails.style.display = this.checked ? 'block' : 'none';
                }
                // Refresh rate limit stats when enabled
                if (this.checked) {
                    refreshRateLimitStats();
                }
            };
        }
        if (rateLimitGlobalSelect) {
            rateLimitGlobalSelect.value = (rateLimitConfig.globalLimit || 60).toString();
        }
        if (rateLimitPerEndpointSelect) {
            rateLimitPerEndpointSelect.value = (rateLimitConfig.perEndpointLimit || 30).toString();
        }

        // Load rate limit stats if enabled
        if (rateLimitConfig.enabled) {
            refreshRateLimitStats();
        }
    } catch (error) {
        console.error('Failed to load settings:', error);
    }
}

// Show auto theme config modal
export async function showAutoThemeConfigModal() {
    const modal = document.getElementById('autoThemeConfigModal');
    if (!modal) return;

    try {
        // Load current auto theme settings
        const autoMode = await window.go.main.App.GetAutoThemeMode();
        const lightTheme = await window.go.main.App.GetAutoLightTheme();
        const darkTheme = await window.go.main.App.GetAutoDarkTheme();

        const modeSelect = document.getElementById('autoThemeMode');
        const lightSelect = document.getElementById('autoLightTheme');
        const darkSelect = document.getElementById('autoDarkTheme');

        if (modeSelect) modeSelect.value = autoMode || 'time';
        if (lightSelect) lightSelect.value = lightTheme || 'light';
        if (darkSelect) darkSelect.value = darkTheme || 'dark';

        // Update help text
        updateAutoThemeModeHelp();
    } catch (error) {
        console.error('Failed to load auto theme settings:', error);
    }

    // Show modal
    modal.classList.add('active');
}

// Close auto theme config modal
export function closeAutoThemeConfigModal() {
    const modal = document.getElementById('autoThemeConfigModal');
    if (modal) {
        modal.classList.remove('active');
    }

    // If user cancels, uncheck the auto mode checkbox
    const themeAutoCheckbox = document.getElementById('settingsThemeAuto');
    const themeSelect = document.getElementById('settingsTheme');
    if (themeAutoCheckbox && !themeAutoCheckbox.dataset.confirmed) {
        themeAutoCheckbox.checked = false;
        if (themeSelect) {
            themeSelect.disabled = false;
        }
    }
}

// Update auto theme mode help text
export function updateAutoThemeModeHelp() {
    const modeSelect = document.getElementById('autoThemeMode');
    const helpEl = document.getElementById('autoThemeModeHelp');
    if (modeSelect && helpEl) {
        if (modeSelect.value === 'system') {
            helpEl.textContent = t('settings.autoThemeModeSystemHelp');
        } else {
            helpEl.textContent = t('settings.autoThemeModeTimeHelp');
        }
    }
}

// Save auto theme config
export async function saveAutoThemeConfig() {
    try {
        const autoMode = document.getElementById('autoThemeMode').value;
        const lightTheme = document.getElementById('autoLightTheme').value;
        const darkTheme = document.getElementById('autoDarkTheme').value;

        // Save mode and themes
        await window.go.main.App.SetAutoThemeMode(autoMode);
        await window.go.main.App.SetAutoLightTheme(lightTheme);
        await window.go.main.App.SetAutoDarkTheme(darkTheme);

        // Enable auto mode
        await window.go.main.App.SetThemeAuto(true);

        // Mark checkbox as confirmed so it won't be unchecked when closing modal
        const themeAutoCheckbox = document.getElementById('settingsThemeAuto');
        if (themeAutoCheckbox) {
            themeAutoCheckbox.dataset.confirmed = 'true';
        }

        // Close the config modal
        closeAutoThemeConfigModal();

        // Apply theme immediately based on current time or system
        stopAutoThemeCheck();
        await startAutoThemeCheck();
    } catch (error) {
        console.error('Failed to save auto theme config:', error);
        showNotification(t('settings.saveFailed') + ': ' + error, 'error');
    }
}

// Save settings
export async function saveSettings() {
    try {
        // Get values from form
        const closeWindowBehavior = document.getElementById('settingsCloseWindowBehavior').value;
        const language = document.getElementById('settingsLanguage').value;
        const theme = document.getElementById('settingsTheme').value;
        const themeAuto = document.getElementById('settingsThemeAuto').checked;
        const proxyUrl = document.getElementById('settingsProxyUrl').value.trim();
        const healthCheckInterval = parseInt(document.getElementById('settingsHealthCheckInterval').value, 10);
        const requestTimeout = parseInt(document.getElementById('settingsRequestTimeout').value, 10);
        const healthHistoryRetention = parseInt(document.getElementById('settingsHealthHistoryRetention').value, 10);

        // Save close window behavior
        await window.go.main.App.SetCloseWindowBehavior(closeWindowBehavior);

        // Save proxy URL
        await window.go.main.App.SetProxyURL(proxyUrl);

        // Save health check interval
        await window.go.main.App.SetHealthCheckInterval(healthCheckInterval);

        // Save request timeout
        await window.go.main.App.SetRequestTimeout(requestTimeout);

        // Save health history retention days
        await window.go.main.App.SetHealthHistoryRetentionDays(healthHistoryRetention);

        // Save alert config
        const alertEnabled = document.getElementById('settingsAlertEnabled').checked;
        const alertConsecutiveFailures = parseInt(document.getElementById('settingsAlertConsecutiveFailures').value, 10);
        const alertCooldown = parseInt(document.getElementById('settingsAlertCooldown').value, 10);
        const alertNotifyOnRecovery = document.getElementById('settingsAlertNotifyOnRecovery').checked;
        const alertSystemNotification = document.getElementById('settingsAlertSystemNotification').checked;
        const performanceAlertEnabled = document.getElementById('settingsPerformanceAlertEnabled').checked;
        const latencyThreshold = parseInt(document.getElementById('settingsLatencyThreshold').value, 10);
        const latencyIncrease = parseInt(document.getElementById('settingsLatencyIncrease').value, 10);
        await window.go.main.App.SetAlertConfig(
            alertEnabled,
            alertConsecutiveFailures,
            alertNotifyOnRecovery,
            alertSystemNotification,
            alertCooldown,
            performanceAlertEnabled,
            latencyThreshold,
            latencyIncrease
        );

        // Save cache config
        const cacheEnabled = document.getElementById('settingsCacheEnabled').checked;
        const cacheTTL = parseInt(document.getElementById('settingsCacheTTL').value, 10);
        const cacheMaxEntries = parseInt(document.getElementById('settingsCacheMaxEntries').value, 10);
        await window.go.main.App.SetCacheConfig(cacheEnabled, cacheTTL, cacheMaxEntries);

        // Save rate limit config
        const rateLimitEnabled = document.getElementById('settingsRateLimitEnabled').checked;
        const rateLimitGlobal = parseInt(document.getElementById('settingsRateLimitGlobal').value, 10);
        const rateLimitPerEndpoint = parseInt(document.getElementById('settingsRateLimitPerEndpoint').value, 10);
        await window.go.main.App.SetRateLimitConfig(rateLimitEnabled, rateLimitGlobal, rateLimitPerEndpoint);

        // Get current config
        const configStr = await window.go.main.App.GetConfig();
        const config = JSON.parse(configStr);

        // Save theme if changed
        if (config.theme !== theme) {
            await window.go.main.App.SetTheme(theme);
        }

        // Handle auto mode changes
        if (config.themeAuto !== themeAuto) {
            await window.go.main.App.SetThemeAuto(themeAuto);
        }

        // Apply theme based on final settings
        stopAutoThemeCheck();
        if (themeAuto) {
            // Auto mode: apply time-based theme
            await startAutoThemeCheck();
        } else {
            // Manual mode: apply selected theme directly
            applyTheme(theme);
        }

        // Update language if changed
        if (config.language !== language) {
            // Apply language change immediately (will reload page)
            changeLanguage(language);
        }

        // Close modal
        closeSettingsModal();
    } catch (error) {
        console.error('Failed to save settings:', error);
        showNotification(t('settings.saveFailed') + ': ' + error, 'error');
    }
}

// Show notification (reuse from webdav.js if available, or implement simple version)
function showNotification(message, type = 'info') {
    // Create notification element
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

    // Auto remove after 3 seconds
    setTimeout(() => {
        notification.style.animation = 'slideOutRight 0.3s ease-out';
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

// Refresh chart with new theme colors
async function refreshChartWithTheme() {
    // Check if chart module is loaded and chart exists
    if (typeof window.initTokenChart === 'function') {
        try {
            // Get current period from the active stats tab
            const activeTab = document.querySelector('.stats-tab-btn.active');
            const currentPeriod = activeTab ? activeTab.dataset.period : 'daily';

            // Re-initialize chart with current period to apply new theme colors
            await window.initTokenChart(currentPeriod);
        } catch (error) {
            console.error('Failed to refresh chart with theme:', error);
        }
    }
}

// 刷新缓存统计
async function refreshCacheStats() {
    try {
        const statsStr = await window.go.main.App.GetCacheStats();
        const stats = JSON.parse(statsStr);

        const entriesEl = document.getElementById('cacheStatEntries');
        const hitsEl = document.getElementById('cacheStatHits');
        const missesEl = document.getElementById('cacheStatMisses');
        const sizeEl = document.getElementById('cacheStatSize');

        if (entriesEl) entriesEl.textContent = stats.totalEntries || 0;
        if (hitsEl) hitsEl.textContent = stats.totalHits || 0;
        if (missesEl) missesEl.textContent = stats.totalMisses || 0;
        if (sizeEl) sizeEl.textContent = formatCacheSize(stats.totalSize || 0);
    } catch (error) {
        console.error('Failed to refresh cache stats:', error);
    }
}

// 格式化缓存大小
function formatCacheSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// 清空缓存
export async function clearCache() {
    try {
        await window.go.main.App.ClearCache();
        await refreshCacheStats();
        showNotification('Cache cleared', 'success');
    } catch (error) {
        console.error('Failed to clear cache:', error);
        showNotification('Failed to clear cache: ' + error, 'error');
    }
}

// 导出 clearCache 到 window 对象
window.clearCache = clearCache;

// 刷新速率限制统计
async function refreshRateLimitStats() {
    try {
        const statsStr = await window.go.main.App.GetRateLimitStats();
        const stats = JSON.parse(statsStr);

        const rpmEl = document.getElementById('rateLimitStatRpm');
        const allowedEl = document.getElementById('rateLimitStatAllowed');
        const rejectedEl = document.getElementById('rateLimitStatRejected');

        if (rpmEl) rpmEl.textContent = stats.currentGlobalRpm || 0;
        if (allowedEl) allowedEl.textContent = stats.totalAllowed || 0;
        if (rejectedEl) rejectedEl.textContent = stats.totalRejected || 0;
    } catch (error) {
        console.error('Failed to refresh rate limit stats:', error);
    }
}

// 重置速率限制统计
export async function resetRateLimitStats() {
    try {
        await window.go.main.App.ResetRateLimitStats();
        await refreshRateLimitStats();
        showNotification('Rate limit stats reset', 'success');
    } catch (error) {
        console.error('Failed to reset rate limit stats:', error);
        showNotification('Failed to reset rate limit stats: ' + error, 'error');
    }
}

// 导出 resetRateLimitStats 到 window 对象
window.resetRateLimitStats = resetRateLimitStats;
