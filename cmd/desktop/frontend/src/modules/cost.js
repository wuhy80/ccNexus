// 成本统计模块
import { t } from '../i18n/index.js';

let currentCostPeriod = 'daily';
let costData = null;

// 格式化成本金额
export function formatCost(cost) {
    if (cost === null || cost === undefined) return '$0.00';
    if (cost < 0.01) return `$${cost.toFixed(4)}`;
    if (cost < 1) return `$${cost.toFixed(3)}`;
    return `$${cost.toFixed(2)}`;
}

// 获取当前成本周期
export function getCurrentCostPeriod() {
    return currentCostPeriod;
}

// 加载成本数据
export async function loadCostByPeriod(period = 'daily') {
    try {
        currentCostPeriod = period;

        let costStr;
        switch (period) {
            case 'daily':
                costStr = await window.go.main.App.GetCostDaily();
                break;
            case 'yesterday':
                costStr = await window.go.main.App.GetCostYesterday();
                break;
            case 'weekly':
                costStr = await window.go.main.App.GetCostWeekly();
                break;
            case 'monthly':
                costStr = await window.go.main.App.GetCostMonthly();
                break;
            default:
                costStr = await window.go.main.App.GetCostDaily();
        }

        costData = JSON.parse(costStr);

        if (!costData.success) {
            console.error('Failed to load cost data:', costData.error);
            return null;
        }

        updateCostUI(costData);
        return costData;
    } catch (error) {
        console.error('Failed to load cost:', error);
        return null;
    }
}

// 更新成本 UI
function updateCostUI(data) {
    // 更新总成本
    const totalCostEl = document.getElementById('periodTotalCost');
    if (totalCostEl) {
        totalCostEl.textContent = formatCost(data.totalCost);
    }

    // 更新成本明细
    const inputCostEl = document.getElementById('periodInputCost');
    if (inputCostEl) {
        inputCostEl.textContent = formatCost(data.inputCost);
    }

    const outputCostEl = document.getElementById('periodOutputCost');
    if (outputCostEl) {
        outputCostEl.textContent = formatCost(data.outputCost);
    }

    const cacheWriteCostEl = document.getElementById('periodCacheWriteCost');
    if (cacheWriteCostEl) {
        cacheWriteCostEl.textContent = formatCost(data.cacheWriteCost);
    }

    const cacheReadCostEl = document.getElementById('periodCacheReadCost');
    if (cacheReadCostEl) {
        cacheReadCostEl.textContent = formatCost(data.cacheReadCost);
    }

    // 更新缓存节省
    const cacheSavingsEl = document.getElementById('periodCacheSavings');
    if (cacheSavingsEl) {
        cacheSavingsEl.textContent = formatCost(data.cacheSavings);
    }
}

// 加载成本趋势
export async function loadCostTrend(period = 'daily') {
    try {
        const trendStr = await window.go.main.App.GetCostTrend(period);
        const trend = JSON.parse(trendStr);

        if (!trend.success) {
            return null;
        }

        // 更新趋势显示
        const trendEl = document.getElementById('costTrend');
        if (trendEl) {
            const trendValue = trend.trend || 0;
            let trendText = '→ 0%';
            let trendClass = 'trend-neutral';

            if (trendValue > 0) {
                trendText = `↑ ${trendValue.toFixed(1)}%`;
                trendClass = 'trend-up';
            } else if (trendValue < 0) {
                trendText = `↓ ${Math.abs(trendValue).toFixed(1)}%`;
                trendClass = 'trend-down';
            }

            trendEl.textContent = trendText;
            trendEl.className = `trend-indicator ${trendClass}`;
        }

        return trend;
    } catch (error) {
        console.error('Failed to load cost trend:', error);
        return null;
    }
}

// 获取当前成本数据
export function getCostData() {
    return costData;
}

// 获取定价信息
export async function getPricingInfo() {
    try {
        const pricingStr = await window.go.main.App.GetPricingInfo();
        return JSON.parse(pricingStr);
    } catch (error) {
        console.error('Failed to get pricing info:', error);
        return null;
    }
}

// 生成成本卡片 HTML
export function generateCostCardHTML() {
    return `
        <div class="stat-card cost-card">
            <div class="stat-label">
                <span class="stat-icon">💰</span>
                ${t('cost.totalCost')}
            </div>
            <div class="stat-value cost-value" id="periodTotalCost">$0.00</div>
            <div class="stat-detail">
                <span class="cost-breakdown">
                    <span class="cost-item" title="${t('cost.inputCost')}">
                        ⬇️ <span id="periodInputCost">$0.00</span>
                    </span>
                    <span class="cost-item" title="${t('cost.outputCost')}">
                        ⬆️ <span id="periodOutputCost">$0.00</span>
                    </span>
                </span>
            </div>
            <div class="stat-trend">
                <span id="costTrend" class="trend-indicator trend-neutral">→ 0%</span>
            </div>
        </div>
        <div class="stat-card cache-savings-card">
            <div class="stat-label">
                <span class="stat-icon">💎</span>
                ${t('cost.cacheSavings')}
            </div>
            <div class="stat-value savings-value" id="periodCacheSavings">$0.00</div>
            <div class="stat-detail">
                <span class="cost-breakdown">
                    <span class="cost-item" title="${t('cost.cacheWriteCost')}">
                        📝 <span id="periodCacheWriteCost">$0.00</span>
                    </span>
                    <span class="cost-item" title="${t('cost.cacheReadCost')}">
                        📖 <span id="periodCacheReadCost">$0.00</span>
                    </span>
                </span>
            </div>
        </div>
    `;
}
