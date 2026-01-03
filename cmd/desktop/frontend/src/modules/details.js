import { formatTokens } from '../utils/format.js';
import { t } from '../i18n/index.js';

let currentPage = 1;
let pageSize = 20;
let totalRecords = 0;
let totalPages = 1;

// Show daily details modal
export async function showDailyDetailsModal() {
    const modal = document.getElementById('dailyDetailsModal');
    if (!modal) return;

    // Reset to first page
    currentPage = 1;

    // Show modal
    modal.style.display = 'flex';

    // Load first page
    await loadDetailsPage(currentPage);
}

// Close daily details modal
export function closeDailyDetailsModal() {
    const modal = document.getElementById('dailyDetailsModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// Load details page
async function loadDetailsPage(page) {
    try {
        const offset = (page - 1) * pageSize;
        const result = await window.go.main.App.GetDailyRequestDetails(pageSize, offset);
        const data = JSON.parse(result);

        if (!data.success) {
            showError(data.message || t('statistics.loadFailed'));
            return;
        }

        totalRecords = data.total || 0;
        totalPages = Math.ceil(totalRecords / pageSize);
        currentPage = page;

        // Update UI
        updateTotalCount(totalRecords);
        updatePageInfo(currentPage, totalPages);
        updatePaginationButtons();
        renderDetailsTable(data.requests || []);

    } catch (error) {
        console.error('Failed to load details:', error);
        showError(t('statistics.loadFailed'));
    }
}

// Render details table
function renderDetailsTable(requests) {
    const tbody = document.querySelector('#dailyDetailsTable tbody');
    if (!tbody) return;

    // Clear existing rows
    tbody.innerHTML = '';

    if (requests.length === 0) {
        const row = document.createElement('tr');
        row.innerHTML = `<td colspan="9" style="text-align: center; padding: 20px;">${t('statistics.noData')}</td>`;
        tbody.appendChild(row);
        return;
    }

    // Create table rows
    requests.forEach(req => {
        const row = document.createElement('tr');

        // Format timestamp
        const time = formatTimestamp(req.Timestamp || req.timestamp);

        // Calculate total tokens (including cache)
        const totalTokens = (req.InputTokens || 0) +
                           (req.CacheCreationTokens || 0) +
                           (req.CacheReadTokens || 0) +
                           (req.OutputTokens || 0);

        // Status
        const status = req.Success ?
            `<span style="color: #4caf50;">✓ ${t('statistics.success')}</span>` :
            `<span style="color: #f44336;">✗ ${t('statistics.failed')}</span>`;

        row.innerHTML = `
            <td>${time}</td>
            <td>${escapeHtml(req.EndpointName || req.endpoint_name || '-')}</td>
            <td>${escapeHtml(req.Model || req.model || '-')}</td>
            <td>${formatTokens(req.InputTokens || 0)}</td>
            <td>${formatTokens(req.CacheCreationTokens || 0)}</td>
            <td>${formatTokens(req.CacheReadTokens || 0)}</td>
            <td>${formatTokens(req.OutputTokens || 0)}</td>
            <td>${formatTokens(totalTokens)}</td>
            <td>${status}</td>
        `;

        tbody.appendChild(row);
    });
}

// Format timestamp
function formatTimestamp(timestamp) {
    if (!timestamp) return '-';

    try {
        const date = new Date(timestamp);
        const hours = String(date.getHours()).padStart(2, '0');
        const minutes = String(date.getMinutes()).padStart(2, '0');
        const seconds = String(date.getSeconds()).padStart(2, '0');
        return `${hours}:${minutes}:${seconds}`;
    } catch (e) {
        return '-';
    }
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Update total count display
function updateTotalCount(count) {
    const el = document.getElementById('detailsTotalCount');
    if (el) {
        el.textContent = count;
    }
}

// Update page info display
function updatePageInfo(current, total) {
    const el = document.getElementById('detailsPageInfo');
    if (el) {
        el.textContent = `${current} / ${total}`;
    }
}

// Update pagination buttons state
function updatePaginationButtons() {
    const prevBtn = document.getElementById('detailsPrevBtn');
    const nextBtn = document.getElementById('detailsNextBtn');

    if (prevBtn) {
        prevBtn.disabled = currentPage <= 1;
    }

    if (nextBtn) {
        nextBtn.disabled = currentPage >= totalPages;
    }
}

// Change page size
export async function changeDetailsPageSize() {
    const select = document.getElementById('detailsPageSize');
    if (!select) return;

    pageSize = parseInt(select.value);
    currentPage = 1; // Reset to first page
    await loadDetailsPage(currentPage);
}

// Load previous page
export async function loadPreviousDetailsPage() {
    if (currentPage > 1) {
        await loadDetailsPage(currentPage - 1);
    }
}

// Load next page
export async function loadNextDetailsPage() {
    if (currentPage < totalPages) {
        await loadDetailsPage(currentPage + 1);
    }
}

// Show error message
function showError(message) {
    const errorEl = document.getElementById('detailsError');
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.style.display = 'block';

        setTimeout(() => {
            errorEl.style.display = 'none';
        }, 5000);
    }
}
