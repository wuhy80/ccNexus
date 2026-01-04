import { getLanguage } from '../i18n/index.js';

// Format tokens with localized units
export function formatTokens(tokens) {
    const numeric = Number(tokens);
    if (!Number.isFinite(numeric) || numeric === 0) {
        return '0';
    }

    const isNegative = numeric < 0;
    const value = Math.abs(numeric);
    const lang = getLanguage();
    const formatted = lang === 'zh-CN'
        ? formatTokensZh(value)
        : formatTokensEn(value);

    return isNegative ? `-${formatted}` : formatted;
}

// Mask API key
export function maskApiKey(key) {
    if (key.length <= 4) return '***';
    return '****' + key.substring(key.length - 4);
}

// Escape HTML
export function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatTokensEn(value) {
    if (value >= 1000000) {
        return (value / 1000000).toFixed(1) + 'M';
    }
    if (value >= 1000) {
        return (value / 1000).toFixed(1) + 'K';
    }
    return value.toString();
}

function formatTokensZh(value) {
    if (value >= 1000000) {
        return (value / 1000000).toFixed(1) + 'M';
    }
    if (value >= 1000) {
        return (value / 1000).toFixed(1) + 'K';
    }
    return value.toString();
}

function trimDecimals(value, decimals) {
    return Number(value.toFixed(decimals)).toString();
}
