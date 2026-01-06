import { api } from '../api.js';
import { state } from '../state.js';
import { notifications } from '../utils/notifications.js';
import { getTransformerLabel, getStatusBadge } from '../utils/formatters.js';

class Endpoints {
    constructor() {
        this.container = document.getElementById('view-container');
        this.endpoints = [];
        this.currentEndpoint = null;
        this.currentClientType = localStorage.getItem('ccNexus_clientType') || 'claude';
        this.draggedIndex = null;
        this.clientsHoursFilter = 24;
    }

    async render() {
        this.container.innerHTML = `
            <div class="endpoints">
                <div class="flex-between mb-3">
                    <div class="flex gap-3 align-center">
                        <h1>Endpoints</h1>
                        <select id="client-type-selector" class="form-select" style="width: auto;">
                            <option value="claude" ${this.currentClientType === 'claude' ? 'selected' : ''}>Claude Code</option>
                            <option value="gemini" ${this.currentClientType === 'gemini' ? 'selected' : ''}>Gemini CLI</option>
                            <option value="codex" ${this.currentClientType === 'codex' ? 'selected' : ''}>Codex CLI</option>
                        </select>
                    </div>
                    <div class="flex gap-2">
                        <button class="btn btn-secondary" id="view-clients-btn">
                            <span>👥 View Clients</span>
                        </button>
                        <button class="btn btn-primary" id="add-endpoint-btn">
                            <span>+ Add Endpoint</span>
                        </button>
                    </div>
                </div>

                <div class="card">
                    <div class="card-body">
                        <div id="endpoints-table"></div>
                    </div>
                </div>
            </div>
        `;

        document.getElementById('add-endpoint-btn').addEventListener('click', () => this.showAddModal());
        document.getElementById('view-clients-btn').addEventListener('click', () => this.showConnectedClientsModal());
        document.getElementById('client-type-selector').addEventListener('change', (e) => this.onClientTypeChange(e.target.value));

        await this.loadEndpoints();
    }

    onClientTypeChange(clientType) {
        this.currentClientType = clientType;
        localStorage.setItem('ccNexus_clientType', clientType);
        this.loadEndpoints();
    }

    async loadEndpoints() {
        try {
            const data = await api.getEndpoints(this.currentClientType);
            this.endpoints = data.endpoints || [];

            // Get current endpoint for this client type
            try {
                const currentData = await api.getCurrentEndpoint(this.currentClientType);
                this.currentEndpoint = currentData.name || null;
            } catch (error) {
                console.error('Failed to get current endpoint:', error);
                this.currentEndpoint = null;
            }

            this.renderTable();
        } catch (error) {
            notifications.error('Failed to load endpoints: ' + error.message);
        }
    }

    renderTable() {
        const container = document.getElementById('endpoints-table');

        if (this.endpoints.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">🔗</div>
                    <div class="empty-state-title">No Endpoints</div>
                    <div class="empty-state-message">Add your first endpoint to get started</div>
                </div>
            `;
            return;
        }

        container.innerHTML = `
            <div class="table-container">
                <table class="table">
                    <thead>
                        <tr>
                            <th style="width: 30px;"></th>
                            <th>Name</th>
                            <th>API URL</th>
                            <th>Transformer</th>
                            <th>Model</th>
                            <th>Status</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody id="endpoints-tbody">
                        ${this.endpoints.map((ep, index) => this.renderEndpointRow(ep, index)).join('')}
                    </tbody>
                </table>
            </div>
        `;

        // Attach event listeners
        this.attachEventListeners();
        this.attachDragListeners();
    }

    renderEndpointRow(ep, index) {
        const isCurrentEndpoint = ep.name === this.currentEndpoint;
        const testStatus = this.getTestStatus(ep.name, this.currentClientType);
        let testStatusIcon = '⚠️';
        let testStatusTitle = 'Not tested';

        if (testStatus === true) {
            testStatusIcon = '✅';
            testStatusTitle = 'Test passed';
        } else if (testStatus === false) {
            testStatusIcon = '❌';
            testStatusTitle = 'Test failed';
        }

        return `
            <tr data-endpoint="${this.escapeHtml(ep.name)}" data-index="${index}" draggable="true" style="cursor: move;">
                <td style="cursor: grab; text-align: center;">⋮⋮</td>
                <td>
                    <strong>${this.escapeHtml(ep.name)}</strong>
                    <span title="${testStatusTitle}" style="margin-left: 5px;">${testStatusIcon}</span>
                    ${isCurrentEndpoint ? '<span class="badge badge-primary" style="margin-left: 5px;">Current</span>' : ''}
                </td>
                <td>
                    <code style="font-size: 12px;">${this.escapeHtml(ep.apiUrl)}</code>
                    <button class="btn-icon copy-btn" data-copy="${this.escapeHtml(ep.apiUrl)}" title="Copy URL">
                        📋
                    </button>
                </td>
                <td>${getTransformerLabel(ep.transformer)}</td>
                <td>${this.escapeHtml(ep.model || '-')}</td>
                <td>${getStatusBadge(ep.enabled)}</td>
                <td>
                    <div class="flex gap-2">
                        ${ep.enabled && !isCurrentEndpoint ? `
                            <button class="btn btn-sm btn-secondary switch-btn" data-name="${this.escapeHtml(ep.name)}" title="Switch to this endpoint">
                                Switch
                            </button>
                        ` : ''}
                        <button class="btn btn-sm btn-secondary test-btn" data-name="${this.escapeHtml(ep.name)}">
                            Test
                        </button>
                        <label class="toggle-switch">
                            <input type="checkbox" class="toggle-endpoint" data-name="${this.escapeHtml(ep.name)}" ${ep.enabled ? 'checked' : ''}>
                            <span class="toggle-slider"></span>
                        </label>
                        <button class="btn btn-sm btn-secondary edit-btn" data-name="${this.escapeHtml(ep.name)}">
                            Edit
                        </button>
                        <button class="btn btn-sm btn-danger delete-btn" data-name="${this.escapeHtml(ep.name)}">
                            Delete
                        </button>
                    </div>
                </td>
            </tr>
        `;
    }

    attachEventListeners() {
        // Test buttons
        document.querySelectorAll('.test-btn').forEach(btn => {
            btn.addEventListener('click', () => this.testEndpoint(btn.dataset.name));
        });

        // Toggle switches
        document.querySelectorAll('.toggle-endpoint').forEach(toggle => {
            toggle.addEventListener('change', () => this.toggleEndpoint(toggle.dataset.name, toggle.checked));
        });

        // Edit buttons
        document.querySelectorAll('.edit-btn').forEach(btn => {
            btn.addEventListener('click', () => this.showEditModal(btn.dataset.name));
        });

        // Delete buttons
        document.querySelectorAll('.delete-btn').forEach(btn => {
            btn.addEventListener('click', () => this.deleteEndpoint(btn.dataset.name));
        });

        // Switch buttons
        document.querySelectorAll('.switch-btn').forEach(btn => {
            btn.addEventListener('click', () => this.switchEndpoint(btn.dataset.name));
        });

        // Copy buttons
        document.querySelectorAll('.copy-btn').forEach(btn => {
            btn.addEventListener('click', () => this.copyToClipboard(btn.dataset.copy, btn));
        });
    }

    attachDragListeners() {
        const rows = document.querySelectorAll('#endpoints-tbody tr[draggable="true"]');

        rows.forEach(row => {
            row.addEventListener('dragstart', (e) => {
                this.draggedIndex = parseInt(row.dataset.index);
                row.style.opacity = '0.5';
            });

            row.addEventListener('dragend', (e) => {
                row.style.opacity = '1';
            });

            row.addEventListener('dragover', (e) => {
                e.preventDefault();
                row.style.borderTop = '2px solid #3b82f6';
            });

            row.addEventListener('dragleave', (e) => {
                row.style.borderTop = '';
            });

            row.addEventListener('drop', async (e) => {
                e.preventDefault();
                row.style.borderTop = '';

                const dropIndex = parseInt(row.dataset.index);
                if (this.draggedIndex !== null && this.draggedIndex !== dropIndex) {
                    await this.reorderEndpoints(this.draggedIndex, dropIndex);
                }
                this.draggedIndex = null;
            });
        });
    }

    async reorderEndpoints(fromIndex, toIndex) {
        try {
            // Reorder the array
            const [movedItem] = this.endpoints.splice(fromIndex, 1);
            this.endpoints.splice(toIndex, 0, movedItem);

            // Send new order to backend
            const names = this.endpoints.map(ep => ep.name);
            await api.reorderEndpoints(names, this.currentClientType);

            notifications.success('Endpoints reordered successfully');
            await this.loadEndpoints();
        } catch (error) {
            notifications.error('Failed to reorder endpoints: ' + error.message);
            await this.loadEndpoints(); // Reload to reset order
        }
    }

    async switchEndpoint(name) {
        try {
            await api.switchEndpoint(name, this.currentClientType);
            notifications.success(`Switched to endpoint: ${name}`);
            await this.loadEndpoints();
        } catch (error) {
            notifications.error('Failed to switch endpoint: ' + error.message);
        }
    }

    copyToClipboard(text, button) {
        navigator.clipboard.writeText(text).then(() => {
            const originalText = button.textContent;
            button.textContent = '✓';
            setTimeout(() => {
                button.textContent = originalText;
            }, 1000);
        }).catch(err => {
            notifications.error('Failed to copy to clipboard');
        });
    }

    getTestStatus(endpointName, clientType) {
        try {
            const key = `${clientType}:${endpointName}`;
            const statusMap = JSON.parse(localStorage.getItem('ccNexus_endpointTestStatus') || '{}');
            return statusMap[key];
        } catch {
            return undefined;
        }
    }

    saveTestStatus(endpointName, clientType, success) {
        try {
            const key = `${clientType}:${endpointName}`;
            const statusMap = JSON.parse(localStorage.getItem('ccNexus_endpointTestStatus') || '{}');
            statusMap[key] = success;
            localStorage.setItem('ccNexus_endpointTestStatus', JSON.stringify(statusMap));
        } catch (error) {
            console.error('Failed to save test status:', error);
        }
    }

    showAddModal() {
        this.showEndpointModal(null);
    }

    showEditModal(name) {
        const endpoint = this.endpoints.find(ep => ep.name === name);
        if (endpoint) {
            this.showEndpointModal(endpoint);
        }
    }

    showEndpointModal(endpoint) {
        const isEdit = !!endpoint;
        const modalContainer = document.getElementById('modal-container');

        modalContainer.innerHTML = `
            <div class="modal-overlay">
                <div class="modal">
                    <div class="modal-header">
                        <h3 class="modal-title">${isEdit ? 'Edit' : 'Add'} Endpoint</h3>
                        <button class="modal-close" id="close-modal">×</button>
                    </div>
                    <div class="modal-body">
                        <form id="endpoint-form">
                            <div class="form-group">
                                <label class="form-label">Name *</label>
                                <input type="text" class="form-input" name="name" value="${endpoint ? this.escapeHtml(endpoint.name) : ''}" required ${isEdit ? 'readonly' : ''}>
                            </div>
                            <div class="form-group">
                                <label class="form-label">API URL *</label>
                                <input type="text" class="form-input" name="apiUrl" value="${endpoint ? this.escapeHtml(endpoint.apiUrl) : ''}" placeholder="https://api.example.com" required>
                            </div>
                            <div class="form-group">
                                <label class="form-label">API Key *</label>
                                <input type="password" class="form-input" name="apiKey" value="${endpoint ? '****' : ''}" placeholder="sk-..." required>
                                ${endpoint ? '<small class="text-muted">Leave as **** to keep existing key</small>' : ''}
                            </div>
                            <div class="form-group">
                                <label class="form-label">Transformer *</label>
                                <select class="form-select" name="transformer" required>
                                    <option value="claude" ${endpoint?.transformer === 'claude' ? 'selected' : ''}>Claude</option>
                                    <option value="openai" ${endpoint?.transformer === 'openai' ? 'selected' : ''}>OpenAI</option>
                                    <option value="openai2" ${endpoint?.transformer === 'openai2' ? 'selected' : ''}>OpenAI Responses</option>
                                    <option value="gemini" ${endpoint?.transformer === 'gemini' ? 'selected' : ''}>Gemini</option>
                                    <option value="deepseek" ${endpoint?.transformer === 'deepseek' ? 'selected' : ''}>DeepSeek</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label class="form-label">Model</label>
                                <div style="display: flex; gap: 8px;">
                                    <input type="text" class="form-input" name="model" id="model-input" value="${endpoint ? this.escapeHtml(endpoint.model || '') : ''}" placeholder="gpt-4, gemini-pro, etc." style="flex: 1;">
                                    <button type="button" class="btn btn-secondary" id="fetch-models-btn" style="white-space: nowrap;">
                                        Fetch Models
                                    </button>
                                </div>
                                <small class="text-muted">Click "Fetch Models" to load available models from the API</small>
                            </div>
                            <div class="form-group">
                                <label class="form-label">Remark</label>
                                <textarea class="form-textarea" name="remark">${endpoint ? this.escapeHtml(endpoint.remark || '') : ''}</textarea>
                            </div>
                            <div class="form-group">
                                <label>
                                    <input type="checkbox" class="form-checkbox" name="enabled" ${endpoint?.enabled !== false ? 'checked' : ''}>
                                    Enabled
                                </label>
                            </div>
                        </form>
                    </div>
                    <div class="modal-footer">
                        <button class="btn btn-secondary" id="cancel-btn">Cancel</button>
                        <button class="btn btn-primary" id="save-btn">${isEdit ? 'Update' : 'Create'}</button>
                    </div>
                </div>
            </div>
        `;

        document.getElementById('close-modal').addEventListener('click', () => this.closeModal());
        document.getElementById('cancel-btn').addEventListener('click', () => this.closeModal());
        document.getElementById('save-btn').addEventListener('click', () => this.saveEndpoint(isEdit, endpoint?.name));
        document.getElementById('fetch-models-btn').addEventListener('click', () => this.fetchModels());
    }

    async fetchModels() {
        const apiUrlInput = document.querySelector('input[name="apiUrl"]');
        const apiKeyInput = document.querySelector('input[name="apiKey"]');
        const transformerSelect = document.querySelector('select[name="transformer"]');
        const modelInput = document.getElementById('model-input');
        const fetchBtn = document.getElementById('fetch-models-btn');

        const apiUrl = apiUrlInput.value.trim();
        const apiKey = apiKeyInput.value.trim();
        const transformer = transformerSelect.value;

        if (!apiUrl || !apiKey || apiKey === '****') {
            notifications.error('Please enter API URL and API Key first');
            return;
        }

        try {
            fetchBtn.disabled = true;
            fetchBtn.textContent = 'Fetching...';

            const result = await api.fetchModels(apiUrl, apiKey, transformer);

            if (result.models && result.models.length > 0) {
                // Show model selection modal
                this.showModelSelectionModal(result.models, modelInput);
            } else {
                notifications.info('No models found');
            }
        } catch (error) {
            notifications.error('Failed to fetch models: ' + error.message);
        } finally {
            fetchBtn.disabled = false;
            fetchBtn.textContent = 'Fetch Models';
        }
    }

    showModelSelectionModal(models, modelInput) {
        const modalContainer = document.getElementById('modal-container');
        const currentModal = modalContainer.querySelector('.modal');

        // Create a second modal overlay
        const modelModal = document.createElement('div');
        modelModal.className = 'modal-overlay';
        modelModal.style.zIndex = '1001';
        modelModal.innerHTML = `
            <div class="modal" style="max-width: 500px;">
                <div class="modal-header">
                    <h3 class="modal-title">Select Model</h3>
                    <button class="modal-close" id="close-model-modal">×</button>
                </div>
                <div class="modal-body">
                    <div style="max-height: 400px; overflow-y: auto;">
                        ${models.map(model => `
                            <div class="model-item" style="padding: 10px; border-bottom: 1px solid #e5e7eb; cursor: pointer;" data-model="${this.escapeHtml(model)}">
                                <strong>${this.escapeHtml(model)}</strong>
                            </div>
                        `).join('')}
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" id="cancel-model-btn">Cancel</button>
                </div>
            </div>
        `;

        modalContainer.appendChild(modelModal);

        // Attach event listeners
        document.getElementById('close-model-modal').addEventListener('click', () => {
            modelModal.remove();
        });

        document.getElementById('cancel-model-btn').addEventListener('click', () => {
            modelModal.remove();
        });

        document.querySelectorAll('.model-item').forEach(item => {
            item.addEventListener('click', () => {
                const selectedModel = item.dataset.model;
                modelInput.value = selectedModel;
                notifications.success(`Model selected: ${selectedModel}`);
                modelModal.remove();
            });

            item.addEventListener('mouseenter', () => {
                item.style.backgroundColor = '#f3f4f6';
            });

            item.addEventListener('mouseleave', () => {
                item.style.backgroundColor = '';
            });
        });
    }

    async saveEndpoint(isEdit, originalName) {
        const form = document.getElementById('endpoint-form');
        const formData = new FormData(form);

        const data = {
            name: formData.get('name'),
            clientType: this.currentClientType,
            apiUrl: formData.get('apiUrl'),
            apiKey: formData.get('apiKey'),
            transformer: formData.get('transformer'),
            model: formData.get('model'),
            remark: formData.get('remark'),
            enabled: formData.get('enabled') === 'on'
        };

        // If editing and API key is ****, don't send it
        if (isEdit && data.apiKey === '****') {
            delete data.apiKey;
        }

        try {
            if (isEdit) {
                await api.updateEndpoint(originalName, data);
                notifications.success('Endpoint updated successfully');
            } else {
                await api.createEndpoint(data);
                notifications.success('Endpoint created successfully');
            }

            this.closeModal();
            await this.loadEndpoints();
        } catch (error) {
            notifications.error('Failed to save endpoint: ' + error.message);
        }
    }

    async toggleEndpoint(name, enabled) {
        try {
            await api.toggleEndpoint(name, enabled, this.currentClientType);
            notifications.success(`Endpoint ${enabled ? 'enabled' : 'disabled'}`);
            await this.loadEndpoints();
        } catch (error) {
            notifications.error('Failed to toggle endpoint: ' + error.message);
            await this.loadEndpoints(); // Reload to reset toggle state
        }
    }

    async testEndpoint(name) {
        try {
            notifications.info('Testing endpoint...');
            const result = await api.testEndpoint(name, this.currentClientType);

            if (result.success) {
                this.saveTestStatus(name, this.currentClientType, true);
                notifications.success(`Test successful! Latency: ${result.latency}ms`);
                this.showTestResultModal(name, result);
                await this.loadEndpoints(); // Refresh to show test status
            } else {
                this.saveTestStatus(name, this.currentClientType, false);
                notifications.error(`Test failed: ${result.error}`);
                await this.loadEndpoints(); // Refresh to show test status
            }
        } catch (error) {
            this.saveTestStatus(name, this.currentClientType, false);
            notifications.error('Test failed: ' + error.message);
            await this.loadEndpoints(); // Refresh to show test status
        }
    }

    showTestResultModal(name, result) {
        const modalContainer = document.getElementById('modal-container');

        modalContainer.innerHTML = `
            <div class="modal-overlay">
                <div class="modal">
                    <div class="modal-header">
                        <h3 class="modal-title">Test Result: ${this.escapeHtml(name)}</h3>
                        <button class="modal-close" id="close-modal">×</button>
                    </div>
                    <div class="modal-body">
                        <div class="mb-2">
                            <strong>Status:</strong> <span class="badge badge-success">Success</span>
                        </div>
                        <div class="mb-2">
                            <strong>Latency:</strong> ${result.latency}ms
                        </div>
                        <div class="mb-2">
                            <strong>Response:</strong>
                            <div class="code-block mt-1">${this.escapeHtml(result.response || 'No response')}</div>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button class="btn btn-primary" id="close-btn">Close</button>
                    </div>
                </div>
            </div>
        `;

        document.getElementById('close-modal').addEventListener('click', () => this.closeModal());
        document.getElementById('close-btn').addEventListener('click', () => this.closeModal());
    }

    async deleteEndpoint(name) {
        if (!confirm(`Are you sure you want to delete endpoint "${name}"?`)) {
            return;
        }

        try {
            await api.deleteEndpoint(name, this.currentClientType);
            notifications.success('Endpoint deleted successfully');
            await this.loadEndpoints();
        } catch (error) {
            notifications.error('Failed to delete endpoint: ' + error.message);
        }
    }

    closeModal() {
        document.getElementById('modal-container').innerHTML = '';
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // Connected Clients Modal
    async showConnectedClientsModal() {
        const modalContainer = document.getElementById('modal-container');

        modalContainer.innerHTML = `
            <div class="modal-overlay">
                <div class="modal" style="max-width: 900px; width: 90%;">
                    <div class="modal-header">
                        <h3 class="modal-title">👥 Connected Clients</h3>
                        <button class="modal-close" id="close-clients-modal">×</button>
                    </div>
                    <div class="modal-body">
                        <div class="flex-between mb-3">
                            <div class="flex gap-2 align-center">
                                <label>Time Range:</label>
                                <select id="clients-hours-filter" class="form-select" style="width: auto;">
                                    <option value="1" ${this.clientsHoursFilter === 1 ? 'selected' : ''}>Last 1 Hour</option>
                                    <option value="6" ${this.clientsHoursFilter === 6 ? 'selected' : ''}>Last 6 Hours</option>
                                    <option value="24" ${this.clientsHoursFilter === 24 ? 'selected' : ''}>Last 24 Hours</option>
                                </select>
                            </div>
                            <button class="btn btn-secondary" id="refresh-clients-btn">
                                🔄 Refresh
                            </button>
                        </div>
                        <div id="clients-table-container">
                            <div class="text-center py-4">Loading...</div>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button class="btn btn-primary" id="close-clients-btn">Close</button>
                    </div>
                </div>
            </div>
        `;

        document.getElementById('close-clients-modal').addEventListener('click', () => this.closeModal());
        document.getElementById('close-clients-btn').addEventListener('click', () => this.closeModal());
        document.getElementById('refresh-clients-btn').addEventListener('click', () => this.loadConnectedClients());
        document.getElementById('clients-hours-filter').addEventListener('change', (e) => {
            this.clientsHoursFilter = parseInt(e.target.value);
            this.loadConnectedClients();
        });

        await this.loadConnectedClients();
    }

    async loadConnectedClients() {
        const container = document.getElementById('clients-table-container');
        if (!container) return;

        container.innerHTML = '<div class="text-center py-4">Loading...</div>';

        try {
            const data = await api.getConnectedClients(this.clientsHoursFilter);
            const clients = data.clients || [];

            if (clients.length === 0) {
                container.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">👥</div>
                        <div class="empty-state-title">No Clients</div>
                        <div class="empty-state-message">No client connection records in the selected time range</div>
                    </div>
                `;
                return;
            }

            container.innerHTML = `
                <div class="mb-2 text-muted">Client Count: ${clients.length}</div>
                <div class="table-container">
                    <table class="table">
                        <thead>
                            <tr>
                                <th>IP Address</th>
                                <th>Last Request</th>
                                <th>Requests</th>
                                <th>Input Tokens</th>
                                <th>Output Tokens</th>
                                <th>Endpoints Used</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${clients.map(client => this.renderClientRow(client)).join('')}
                        </tbody>
                    </table>
                </div>
            `;
        } catch (error) {
            container.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">❌</div>
                    <div class="empty-state-title">Failed to Load</div>
                    <div class="empty-state-message">${this.escapeHtml(error.message)}</div>
                </div>
            `;
        }
    }

    renderClientRow(client) {
        const lastSeen = this.formatRelativeTime(client.lastSeen);
        const endpointsUsed = (client.endpointsUsed || []).join(', ') || '-';

        return `
            <tr>
                <td><code>${this.escapeHtml(client.clientIp)}</code></td>
                <td>${lastSeen}</td>
                <td>${client.requestCount || 0}</td>
                <td>${this.formatNumber(client.inputTokens || 0)}</td>
                <td>${this.formatNumber(client.outputTokens || 0)}</td>
                <td><small>${this.escapeHtml(endpointsUsed)}</small></td>
            </tr>
        `;
    }

    formatRelativeTime(timestamp) {
        if (!timestamp) return '-';

        const now = new Date();
        const time = new Date(timestamp);
        const diffMs = now - time;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);

        if (diffMins < 1) return 'Just now';
        if (diffMins < 60) return `${diffMins} min ago`;
        if (diffHours < 24) return `${diffHours} hours ago`;
        return `${diffDays} days ago`;
    }

    formatNumber(num) {
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        }
        if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        }
        return num.toString();
    }
}

export const endpoints = new Endpoints();
