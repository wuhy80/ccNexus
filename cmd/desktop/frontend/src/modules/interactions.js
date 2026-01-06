// Interaction recording module
import { t } from '../i18n/index.js'
import { formatTokens } from '../utils/format.js'

let currentDate = ''
let interactionEnabled = false

// Show interactions modal
export async function showInteractionsModal() {
    const modal = document.getElementById('interactionsModal')
    if (!modal) return

    modal.classList.add('active')
    await loadEnabled()
    await loadDates()
}

// Close interactions modal
export function closeInteractionsModal() {
    const modal = document.getElementById('interactionsModal')
    if (modal) {
        modal.classList.remove('active')
    }
}

// Load enabled status
async function loadEnabled() {
    try {
        const result = await window.go.main.App.GetInteractionEnabled()
        const data = JSON.parse(result)
        if (data.success) {
            interactionEnabled = data.enabled
            const toggle = document.getElementById('interactionEnabledToggle')
            if (toggle) {
                toggle.checked = interactionEnabled
            }
        }
    } catch (err) {
        console.error('Failed to load interaction enabled status:', err)
    }
}

// Toggle enabled status
export async function toggleInteractionEnabled() {
    const toggle = document.getElementById('interactionEnabledToggle')
    if (!toggle) return

    try {
        const result = await window.go.main.App.SetInteractionEnabled(toggle.checked)
        const data = JSON.parse(result)
        if (data.success) {
            interactionEnabled = data.enabled
        } else {
            toggle.checked = !toggle.checked // Revert on failure
        }
    } catch (err) {
        console.error('Failed to toggle interaction enabled:', err)
        toggle.checked = !toggle.checked // Revert on failure
    }
}

// Load available dates
async function loadDates() {
    try {
        const result = await window.go.main.App.GetInteractionDates()
        const data = JSON.parse(result)

        const select = document.getElementById('interactionDateSelect')
        if (!select) return

        select.innerHTML = ''

        if (data.success && data.dates && data.dates.length > 0) {
            data.dates.forEach((date, index) => {
                const option = document.createElement('option')
                option.value = date
                option.textContent = date
                select.appendChild(option)
            })

            // Select first date and load interactions
            currentDate = data.dates[0]
            select.value = currentDate
            await loadInteractions(currentDate)
        } else {
            const option = document.createElement('option')
            option.value = ''
            option.textContent = t('interactions.noData')
            select.appendChild(option)
            renderEmptyTable()
        }
    } catch (err) {
        console.error('Failed to load dates:', err)
        renderEmptyTable()
    }
}

// Change date selection
export async function changeInteractionDate() {
    const select = document.getElementById('interactionDateSelect')
    if (!select || !select.value) return

    currentDate = select.value
    await loadInteractions(currentDate)
}

// Load interactions for a specific date
async function loadInteractions(date) {
    try {
        const result = await window.go.main.App.GetInteractions(date)
        const data = JSON.parse(result)

        if (data.success) {
            renderInteractionsTable(data.interactions || [])
        } else {
            renderEmptyTable()
        }
    } catch (err) {
        console.error('Failed to load interactions:', err)
        renderEmptyTable()
    }
}

// Render interactions table
function renderInteractionsTable(interactions) {
    const tbody = document.querySelector('#interactionsTable tbody')
    if (!tbody) return

    tbody.innerHTML = ''

    if (!interactions || interactions.length === 0) {
        renderEmptyTable()
        return
    }

    interactions.forEach(item => {
        const row = document.createElement('tr')
        const time = new Date(item.timestamp).toLocaleTimeString()
        const statusIcon = item.success ? '✓' : '✗'
        const statusClass = item.success ? 'success' : 'error'

        row.innerHTML = `
            <td>${escapeHtml(time)}</td>
            <td>${escapeHtml(item.endpointName || '-')}</td>
            <td>${escapeHtml(item.clientType || '-')}</td>
            <td>${escapeHtml(item.model || '-')}</td>
            <td>${formatTokens(item.inputTokens || 0)}</td>
            <td>${formatTokens(item.outputTokens || 0)}</td>
            <td>${item.durationMs || 0}ms</td>
            <td class="${statusClass}">${statusIcon}</td>
            <td>
                <button class="btn btn-sm btn-secondary" onclick="window.showInteractionDetail('${currentDate}', '${escapeHtml(item.requestId)}')">
                    ${t('interactions.viewDetail')}
                </button>
            </td>
        `
        tbody.appendChild(row)
    })
}

// Render empty table
function renderEmptyTable() {
    const tbody = document.querySelector('#interactionsTable tbody')
    if (!tbody) return

    tbody.innerHTML = `
        <tr>
            <td colspan="9" class="empty-message">${t('interactions.noData')}</td>
        </tr>
    `
}

// Show interaction detail modal
export async function showInteractionDetail(date, requestId) {
    try {
        const result = await window.go.main.App.GetInteractionDetail(date, requestId)
        const data = JSON.parse(result)

        if (!data.success) {
            console.error('Failed to load interaction detail:', data.error)
            return
        }

        const interaction = data.interaction
        renderDetailModal(interaction)

        const modal = document.getElementById('interactionDetailModal')
        if (modal) {
            modal.classList.add('active')
        }
    } catch (err) {
        console.error('Failed to load interaction detail:', err)
    }
}

// Close interaction detail modal
export function closeInteractionDetailModal() {
    const modal = document.getElementById('interactionDetailModal')
    if (modal) {
        modal.classList.remove('active')
    }
}

// Render detail modal
function renderDetailModal(interaction) {
    // Render metadata
    const metaContainer = document.getElementById('interactionDetailMeta')
    if (metaContainer) {
        const time = new Date(interaction.timestamp).toLocaleString()
        const statusText = interaction.stats.success ? t('interactions.statusSuccess') : t('interactions.statusFailed')
        const statusClass = interaction.stats.success ? 'success' : 'error'

        metaContainer.innerHTML = `
            <div class="meta-item">
                <span class="meta-label">${t('interactions.time')}:</span>
                <span class="meta-value">${escapeHtml(time)}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.endpoint')}:</span>
                <span class="meta-value">${escapeHtml(interaction.endpoint?.name || '-')}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.transformer')}:</span>
                <span class="meta-value">${escapeHtml(interaction.endpoint?.transformer || '-')}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.client')}:</span>
                <span class="meta-value">${escapeHtml(interaction.client?.type || '-')}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.model')}:</span>
                <span class="meta-value">${escapeHtml(interaction.request?.model || '-')}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.inputTokens')}:</span>
                <span class="meta-value">${formatTokens(interaction.stats?.inputTokens || 0)}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.outputTokens')}:</span>
                <span class="meta-value">${formatTokens(interaction.stats?.outputTokens || 0)}</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.duration')}:</span>
                <span class="meta-value">${interaction.stats?.durationMs || 0}ms</span>
            </div>
            <div class="meta-item">
                <span class="meta-label">${t('interactions.status')}:</span>
                <span class="meta-value ${statusClass}">${statusText}</span>
            </div>
        `
    }

    // Store interaction data for tab switching
    window._currentInteraction = interaction

    // Show first tab by default
    switchDetailTab('request-raw')
}

// Switch detail tab
export function switchDetailTab(tabName) {
    // Update buttons
    const tabs = document.querySelectorAll('.interaction-tab-btn')
    tabs.forEach(tab => {
        tab.classList.remove('active')
        if (tab.dataset.tab === tabName) {
            tab.classList.add('active')
        }
    })

    // Update content
    const content = document.getElementById('interactionDetailContent')
    if (!content || !window._currentInteraction) return

    const interaction = window._currentInteraction
    let data = null

    switch (tabName) {
        case 'request-raw':
            data = interaction.request?.raw
            break
        case 'request-transformed':
            data = interaction.request?.transformed
            break
        case 'response-raw':
            data = interaction.response?.raw
            break
        case 'response-transformed':
            data = interaction.response?.transformed
            break
    }

    content.innerHTML = `<pre class="json-viewer">${formatJson(data)}</pre>`
}

// Export interactions
export async function exportInteractions() {
    if (!currentDate) return

    try {
        const result = await window.go.main.App.ExportInteractions(currentDate)
        const data = JSON.parse(result)

        if (data.success) {
            // Create download
            const blob = new Blob([JSON.stringify(data.interactions, null, 2)], { type: 'application/json' })
            const url = URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = `interactions-${currentDate}.json`
            document.body.appendChild(a)
            a.click()
            document.body.removeChild(a)
            URL.revokeObjectURL(url)
        }
    } catch (err) {
        console.error('Failed to export interactions:', err)
    }
}

// Format JSON with syntax highlighting
function formatJson(obj) {
    if (obj === null || obj === undefined) {
        return '<span class="json-null">null</span>'
    }

    const json = JSON.stringify(obj, null, 2)
    return escapeHtml(json)
        .replace(/"([^"]+)":/g, '<span class="json-key">"$1"</span>:')
        .replace(/: "([^"]*)"/g, ': <span class="json-string">"$1"</span>')
        .replace(/: (\d+)/g, ': <span class="json-number">$1</span>')
        .replace(/: (true|false)/g, ': <span class="json-boolean">$1</span>')
        .replace(/: (null)/g, ': <span class="json-null">$1</span>')
}

// Escape HTML
function escapeHtml(str) {
    if (str === null || str === undefined) return ''
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;')
}
