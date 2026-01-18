import { t } from '../i18n/index.js';
import { showNotification } from './modal.js';
import { getCurrentClientType, refreshEndpoints } from './endpoints.js';

let importExportModal = null;

// Create the import/export modal HTML
function createImportExportModal() {
    const modal = document.createElement('div');
    modal.id = 'importExportModal';
    modal.className = 'modal';
    modal.innerHTML = `
        <div class="modal-content" style="max-width: 600px;">
            <div class="modal-header">
                <h2 id="importExportTitle"></h2>
                <button class="modal-close" onclick="window.hideImportExportModal()">&times;</button>
            </div>
            <div class="modal-body">
                <!-- Tabs -->
                <div class="tabs" style="margin-bottom: 20px;">
                    <button class="tab-btn active" data-tab="export" onclick="window.switchImportExportTab('export')">
                        <span id="exportTabLabel"></span>
                    </button>
                    <button class="tab-btn" data-tab="import" onclick="window.switchImportExportTab('import')">
                        <span id="importTabLabel"></span>
                    </button>
                </div>

                <!-- Export Tab -->
                <div id="exportTab" class="tab-content active">
                    <div class="form-group">
                        <label style="display: flex; align-items: center; gap: 8px;">
                            <input type="checkbox" id="exportIncludeKeys" style="margin: 0;" />
                            <span id="includeApiKeysLabel"></span>
                        </label>
                        <small class="form-help" style="color: var(--warning-color); display: block; margin-top: 5px;" id="includeApiKeysHelp"></small>
                    </div>
                    <div class="form-group" style="margin-top: 15px;">
                        <div style="display: flex; gap: 10px;">
                            <button class="btn btn-primary" onclick="window.exportCurrentEndpoints()">
                                <span id="exportCurrentLabel"></span>
                            </button>
                            <button class="btn btn-secondary" onclick="window.exportAllEndpoints()">
                                <span id="exportAllLabel"></span>
                            </button>
                        </div>
                    </div>
                    <div id="exportResult" style="display: none; margin-top: 15px;">
                        <div class="form-group">
                            <label id="exportDataLabel"></label>
                            <textarea id="exportData" class="form-control" rows="10" readonly style="font-family: monospace; font-size: 12px;"></textarea>
                        </div>
                        <div style="display: flex; gap: 10px; margin-top: 10px;">
                            <button class="btn btn-secondary" onclick="window.copyExportData()">
                                <span id="copyToClipboardLabel"></span>
                            </button>
                            <button class="btn btn-secondary" onclick="window.downloadExportData()">
                                <span id="downloadAsFileLabel"></span>
                            </button>
                        </div>
                    </div>
                </div>

                <!-- Import Tab -->
                <div id="importTab" class="tab-content" style="display: none;">
                    <div class="form-group">
                        <label id="importModeLabel"></label>
                        <select id="importMode" class="form-control">
                            <option value="skip" id="importModeSkipOption"></option>
                            <option value="overwrite" id="importModeOverwriteOption"></option>
                            <option value="rename" id="importModeRenameOption"></option>
                        </select>
                        <small class="form-help" id="importModeHelp"></small>
                    </div>
                    <div class="form-group" style="margin-top: 15px;">
                        <label id="selectFileLabel"></label>
                        <div id="dropZone" class="drop-zone" onclick="document.getElementById('importFileInput').click()">
                            <input type="file" id="importFileInput" accept=".json" style="display: none;" onchange="window.handleImportFile(event)" />
                            <div class="drop-zone-text" id="dropFileHereLabel"></div>
                        </div>
                    </div>
                    <div class="form-group" style="margin-top: 15px;">
                        <label id="importDataLabel"></label>
                        <textarea id="importData" class="form-control" rows="8" placeholder='{"endpoints": [...]}' style="font-family: monospace; font-size: 12px;"></textarea>
                    </div>
                    <div style="margin-top: 15px;">
                        <button class="btn btn-primary" onclick="window.importEndpoints()">
                            <span id="importBtnLabel"></span>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `;
    document.body.appendChild(modal);

    // Add drop zone styles
    const style = document.createElement('style');
    style.textContent = `
        .drop-zone {
            border: 2px dashed var(--border-color);
            border-radius: 8px;
            padding: 30px;
            text-align: center;
            cursor: pointer;
            transition: all 0.3s ease;
        }
        .drop-zone:hover, .drop-zone.dragover {
            border-color: var(--primary-color);
            background: var(--hover-bg);
        }
        .drop-zone-text {
            color: var(--text-secondary);
            font-size: 14px;
        }
        .tabs {
            display: flex;
            gap: 10px;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 10px;
        }
        .tab-btn {
            padding: 8px 16px;
            border: none;
            background: transparent;
            cursor: pointer;
            border-radius: 4px;
            color: var(--text-color);
            transition: all 0.2s ease;
        }
        .tab-btn:hover {
            background: var(--hover-bg);
        }
        .tab-btn.active {
            background: var(--primary-color);
            color: white;
        }
        .tab-content {
            padding-top: 15px;
        }
    `;
    document.head.appendChild(style);

    // Setup drag and drop
    const dropZone = document.getElementById('dropZone');
    dropZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropZone.classList.add('dragover');
    });
    dropZone.addEventListener('dragleave', () => {
        dropZone.classList.remove('dragover');
    });
    dropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropZone.classList.remove('dragover');
        const file = e.dataTransfer.files[0];
        if (file) {
            handleFile(file);
        }
    });

    // Setup import mode change
    document.getElementById('importMode').addEventListener('change', (e) => {
        updateImportModeHelp(e.target.value);
    });

    return modal;
}

// Update all labels with translations
function updateLabels() {
    const titleEl = document.getElementById('importExportTitle');
    if (titleEl) {
        titleEl.textContent = '📦 ' + t('endpoints.exportEndpoints') + ' / ' + t('endpoints.importEndpoints');
    }

    const exportTabLabel = document.getElementById('exportTabLabel');
    if (exportTabLabel) {
        exportTabLabel.textContent = '📤 ' + t('endpoints.export');
    }

    const importTabLabel = document.getElementById('importTabLabel');
    if (importTabLabel) {
        importTabLabel.textContent = '📥 ' + t('endpoints.import');
    }

    const includeApiKeysLabel = document.getElementById('includeApiKeysLabel');
    if (includeApiKeysLabel) {
        includeApiKeysLabel.textContent = t('endpoints.includeApiKeys');
    }

    const includeApiKeysHelp = document.getElementById('includeApiKeysHelp');
    if (includeApiKeysHelp) {
        includeApiKeysHelp.textContent = t('endpoints.includeApiKeysHelp');
    }

    const exportCurrentLabel = document.getElementById('exportCurrentLabel');
    if (exportCurrentLabel) {
        exportCurrentLabel.textContent = '📤 ' + t('endpoints.exportCurrent');
    }

    const exportAllLabel = document.getElementById('exportAllLabel');
    if (exportAllLabel) {
        exportAllLabel.textContent = '📤 ' + t('endpoints.exportAll');
    }

    const exportDataLabel = document.getElementById('exportDataLabel');
    if (exportDataLabel) {
        exportDataLabel.textContent = t('endpoints.exportEndpoints');
    }

    const copyToClipboardLabel = document.getElementById('copyToClipboardLabel');
    if (copyToClipboardLabel) {
        copyToClipboardLabel.textContent = '📋 ' + t('endpoints.copyToClipboard');
    }

    const downloadAsFileLabel = document.getElementById('downloadAsFileLabel');
    if (downloadAsFileLabel) {
        downloadAsFileLabel.textContent = '💾 ' + t('endpoints.downloadAsFile');
    }

    const importModeLabel = document.getElementById('importModeLabel');
    if (importModeLabel) {
        importModeLabel.textContent = t('endpoints.importMode');
    }

    const importModeSkipOption = document.getElementById('importModeSkipOption');
    if (importModeSkipOption) {
        importModeSkipOption.textContent = t('endpoints.importModeSkip');
    }

    const importModeOverwriteOption = document.getElementById('importModeOverwriteOption');
    if (importModeOverwriteOption) {
        importModeOverwriteOption.textContent = t('endpoints.importModeOverwrite');
    }

    const importModeRenameOption = document.getElementById('importModeRenameOption');
    if (importModeRenameOption) {
        importModeRenameOption.textContent = t('endpoints.importModeRename');
    }

    const selectFileLabel = document.getElementById('selectFileLabel');
    if (selectFileLabel) {
        selectFileLabel.textContent = t('endpoints.selectFile');
    }

    const dropFileHereLabel = document.getElementById('dropFileHereLabel');
    if (dropFileHereLabel) {
        dropFileHereLabel.textContent = '📁 ' + t('endpoints.dropFileHere');
    }

    const importDataLabel = document.getElementById('importDataLabel');
    if (importDataLabel) {
        importDataLabel.textContent = t('endpoints.importEndpoints');
    }

    const importBtnLabel = document.getElementById('importBtnLabel');
    if (importBtnLabel) {
        importBtnLabel.textContent = '📥 ' + t('endpoints.import');
    }

    // Update import mode help
    const importMode = document.getElementById('importMode');
    if (importMode) {
        updateImportModeHelp(importMode.value);
    }
}

// Update import mode help text
function updateImportModeHelp(mode) {
    const helpTexts = {
        'skip': t('endpoints.importModeSkipHelp'),
        'overwrite': t('endpoints.importModeOverwriteHelp'),
        'rename': t('endpoints.importModeRenameHelp')
    };
    const helpEl = document.getElementById('importModeHelp');
    if (helpEl) {
        helpEl.textContent = helpTexts[mode] || '';
    }
}

// Handle file selection
function handleFile(file) {
    if (!file.name.endsWith('.json')) {
        showNotification(t('endpoints.invalidFileFormat'), 'error');
        return;
    }

    const reader = new FileReader();
    reader.onload = (e) => {
        document.getElementById('importData').value = e.target.result;
    };
    reader.readAsText(file);
}

// Show the modal
export function showImportExportModal() {
    if (!importExportModal) {
        importExportModal = createImportExportModal();
    }
    updateLabels();
    importExportModal.classList.add('active');
    // Reset to export tab
    switchImportExportTab('export');
    document.getElementById('exportResult').style.display = 'none';
    document.getElementById('importData').value = '';
}

// Hide the modal
export function hideImportExportModal() {
    if (importExportModal) {
        importExportModal.classList.remove('active');
    }
}

// Switch tabs
export function switchImportExportTab(tab) {
    document.querySelectorAll('#importExportModal .tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tab);
    });
    document.getElementById('exportTab').style.display = tab === 'export' ? 'block' : 'none';
    document.getElementById('importTab').style.display = tab === 'import' ? 'block' : 'none';
}

// Export current client type endpoints
export async function exportCurrentEndpoints() {
    try {
        const clientType = getCurrentClientType();
        const includeKeys = document.getElementById('exportIncludeKeys').checked;
        const result = await window.go.main.App.ExportEndpoints(clientType, includeKeys);

        if (result.includes('"error"')) {
            const data = JSON.parse(result);
            showNotification(data.error || t('endpoints.exportFailed'), 'error');
            return;
        }

        document.getElementById('exportData').value = result;
        document.getElementById('exportResult').style.display = 'block';
        showNotification(t('endpoints.exportSuccess'), 'success');
    } catch (err) {
        showNotification(t('endpoints.exportFailed') + ': ' + err.message, 'error');
    }
}

// Export all endpoints
export async function exportAllEndpoints() {
    try {
        const includeKeys = document.getElementById('exportIncludeKeys').checked;
        const result = await window.go.main.App.ExportAllEndpoints(includeKeys);

        if (result.includes('"error"')) {
            const data = JSON.parse(result);
            showNotification(data.error || t('endpoints.exportFailed'), 'error');
            return;
        }

        document.getElementById('exportData').value = result;
        document.getElementById('exportResult').style.display = 'block';
        showNotification(t('endpoints.exportSuccess'), 'success');
    } catch (err) {
        showNotification(t('endpoints.exportFailed') + ': ' + err.message, 'error');
    }
}

// Copy export data to clipboard
export function copyExportData() {
    const data = document.getElementById('exportData').value;
    navigator.clipboard.writeText(data).then(() => {
        showNotification(t('endpoints.copiedToClipboard'), 'success');
    }).catch(() => {
        showNotification('Failed to copy', 'error');
    });
}

// Download export data as file
export function downloadExportData() {
    const data = document.getElementById('exportData').value;
    if (!data) return;

    try {
        const parsed = JSON.parse(data);
        const clientType = parsed.clientType || 'all';
        const timestamp = new Date().toISOString().slice(0, 10);
        const filename = `ccnexus-endpoints-${clientType}-${timestamp}.json`;

        const blob = new Blob([data], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    } catch (err) {
        showNotification('Failed to download', 'error');
    }
}

// Handle import file input
export function handleImportFile(event) {
    const file = event.target.files[0];
    if (file) {
        handleFile(file);
    }
}

// Import endpoints
export async function importEndpoints() {
    try {
        const jsonData = document.getElementById('importData').value.trim();
        if (!jsonData) {
            showNotification(t('endpoints.invalidFileFormat'), 'error');
            return;
        }

        // Validate JSON
        try {
            JSON.parse(jsonData);
        } catch {
            showNotification(t('endpoints.invalidFileFormat'), 'error');
            return;
        }

        const mode = document.getElementById('importMode').value;
        const result = await window.go.main.App.ImportEndpoints(jsonData, mode);
        const data = JSON.parse(result);

        if (data.success) {
            const message = t('endpoints.importSuccess')
                .replace('{imported}', data.imported)
                .replace('{skipped}', data.skipped);
            showNotification(message, 'success');

            // Refresh endpoints list
            await refreshEndpoints();

            // Close modal after successful import
            setTimeout(() => {
                hideImportExportModal();
            }, 1500);
        } else {
            showNotification(data.message || t('endpoints.importFailed'), 'error');
            if (data.errors && data.errors.length > 0) {
                console.error('Import errors:', data.errors);
            }
        }
    } catch (err) {
        showNotification(t('endpoints.importFailed') + ': ' + err.message, 'error');
    }
}

// Register global functions
window.showImportExportModal = showImportExportModal;
window.hideImportExportModal = hideImportExportModal;
window.switchImportExportTab = switchImportExportTab;
window.exportCurrentEndpoints = exportCurrentEndpoints;
window.exportAllEndpoints = exportAllEndpoints;
window.copyExportData = copyExportData;
window.downloadExportData = downloadExportData;
window.handleImportFile = handleImportFile;
window.importEndpoints = importEndpoints;
