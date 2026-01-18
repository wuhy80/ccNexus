// 端点状态管理模块
// 统一管理端点状态，避免数据不一致

import { t } from '../i18n/index.js';

// 状态缓存
let endpointStatusCache = new Map(); // name -> StatusInfo
let healthCheckInterval = 60; // 默认60秒
let refreshPromise = null; // 用于防止并发刷新

// StatusInfo 结构
// {
//   status: 'available' | 'unavailable' | 'disabled' | 'unknown',
//   source: 'health_check' | 'manual_test' | 'config',
//   lastCheckAt: Date,
//   latencyMs: number,
//   errorMessage: string,
//   testIcon: '✅' | '❌' | '⚠️' | '🚫',
//   testTip: string
// }

// 初始化状态管理
export async function initEndpointStatus() {
    // 加载健康检查间隔配置
    try {
        const configStr = await window.go.main.App.GetConfig();
        const config = JSON.parse(configStr);
        healthCheckInterval = config.healthCheckInterval || 60;
    } catch (error) {
        console.error('Failed to load health check interval:', error);
    }

    // 加载初始数据
    await refreshEndpointStatus();
}

// 刷新所有端点状态
export async function refreshEndpointStatus() {
    // 如果已有刷新操作在进行，返回该 Promise
    if (refreshPromise) {
        return refreshPromise;
    }

    refreshPromise = (async () => {
        try {
            // 并行加载所有数据源
            const [checkResultsStr, configStr] = await Promise.all([
                window.go.main.App.GetEndpointCheckResults(),
                window.go.main.App.GetConfig()
            ]);

            const checkResults = JSON.parse(checkResultsStr);
            const config = JSON.parse(configStr);

            // 更新缓存
            endpointStatusCache.clear();

            for (const ep of config.endpoints) {
                const statusInfo = calculateEndpointStatus(ep, checkResults);
                endpointStatusCache.set(ep.name, statusInfo);
            }

            return endpointStatusCache;
        } catch (error) {
            console.error('Failed to refresh endpoint status:', error);
            return endpointStatusCache;
        } finally {
            // 清除 Promise 引用，允许下次刷新
            refreshPromise = null;
        }
    })();

    return refreshPromise;
}

// 计算单个端点状态
function calculateEndpointStatus(endpoint, checkResults) {
    // 1. 检查是否禁用
    if (endpoint.status === 'disabled') {
        return {
            status: 'disabled',
            source: 'config',
            testIcon: '🚫',
            testTip: t('endpoints.disabled')
        };
    }

    // 2. 检查后端健康检查结果（优先级最高）
    const checkResult = checkResults[endpoint.name];
    if (checkResult && checkResult.lastCheckAt) {
        const lastCheckAt = new Date(checkResult.lastCheckAt);
        const expireTime = healthCheckInterval * 2 * 1000;

        if (!isExpired(lastCheckAt, expireTime)) {
            return {
                status: checkResult.success ? 'available' : 'unavailable',
                source: 'health_check',
                lastCheckAt: lastCheckAt,
                latencyMs: checkResult.latencyMs,
                errorMessage: checkResult.errorMessage,
                testIcon: checkResult.success ? '✅' : '❌',
                testTip: checkResult.success
                    ? `${t('monitor.healthCheckPassed')} (${Math.round(checkResult.latencyMs)}ms)`
                    : `${t('monitor.healthCheckFailed')}: ${checkResult.errorMessage}`
            };
        }
    }

    // 3. 检查手动测试结果（localStorage）
    const testStatus = getEndpointTestStatus(endpoint.name);
    const testTime = getEndpointTestTime(endpoint.name);
    // 排除 'unknown' 字符串，只处理布尔值
    if (typeof testStatus === 'boolean' && testTime && !isExpired(testTime, 3600000)) {
        return {
            status: testStatus ? 'available' : 'unavailable',
            source: 'manual_test',
            lastCheckAt: testTime,
            testIcon: testStatus ? '✅' : '❌',
            testTip: testStatus ? t('endpoints.manualTestPassed') : t('endpoints.manualTestFailed')
        };
    }

    // 4. 使用配置文件状态（兜底）
    return {
        status: endpoint.status || 'unknown',
        source: 'config',
        testIcon: '⚠️',
        testTip: t('monitor.notTested')
    };
}

// 检查是否过期
function isExpired(checkTime, maxAge) {
    if (!checkTime) return true;
    const now = Date.now();
    const time = checkTime instanceof Date ? checkTime.getTime() : new Date(checkTime).getTime();
    return (now - time) > maxAge;
}

// 获取端点状态
export function getEndpointStatus(endpointName) {
    return endpointStatusCache.get(endpointName);
}

// 获取所有端点状态
export function getAllEndpointStatus() {
    return endpointStatusCache;
}

// 更新单个端点状态（手动测试后调用）
export async function updateEndpointStatus(endpointName, success, latencyMs, errorMessage) {
    // 保存到 localStorage（保留兼容性）
    saveEndpointTestStatus(endpointName, success);
    saveEndpointTestTime(endpointName, new Date());

    // 更新缓存
    const statusInfo = {
        status: success ? 'available' : 'unavailable',
        source: 'manual_test',
        lastCheckAt: new Date(),
        latencyMs: latencyMs,
        errorMessage: errorMessage,
        testIcon: success ? '✅' : '❌',
        testTip: success
            ? `${t('endpoints.manualTestPassed')} (${Math.round(latencyMs)}ms)`
            : `${t('endpoints.manualTestFailed')}: ${errorMessage}`
    };

    endpointStatusCache.set(endpointName, statusInfo);

    return statusInfo;
}

// localStorage 辅助函数
function getEndpointTestStatus(name) {
    try {
        const statusMap = JSON.parse(localStorage.getItem('ccNexus_endpointTestStatus') || '{}');
        return statusMap[name];
    } catch {
        return undefined;
    }
}

function getEndpointTestTime(name) {
    try {
        const timeMap = JSON.parse(localStorage.getItem('ccNexus_endpointTestTime') || '{}');
        return timeMap[name] ? new Date(timeMap[name]) : null;
    } catch {
        return null;
    }
}

function saveEndpointTestStatus(name, success) {
    try {
        const statusMap = JSON.parse(localStorage.getItem('ccNexus_endpointTestStatus') || '{}');
        statusMap[name] = success;
        localStorage.setItem('ccNexus_endpointTestStatus', JSON.stringify(statusMap));
    } catch (error) {
        console.error('Failed to save test status:', error);
    }
}

function saveEndpointTestTime(name, time) {
    try {
        const timeMap = JSON.parse(localStorage.getItem('ccNexus_endpointTestTime') || '{}');
        timeMap[name] = time.toISOString();
        localStorage.setItem('ccNexus_endpointTestTime', JSON.stringify(timeMap));
    } catch (error) {
        console.error('Failed to save test time:', error);
    }
}
