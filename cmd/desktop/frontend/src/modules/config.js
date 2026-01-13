// Configuration management
import { getCurrentClientType } from './endpoints.js';

export async function loadConfig() {
    try {
        if (!window.go?.main?.App) {
            console.error('Not running in Wails environment');
            document.getElementById('endpointList').innerHTML = `
                <div class="empty-state">
                    <p>⚠️ Please run this app through Wails</p>
                    <p>Use: wails dev or run the built application</p>
                </div>
            `;
            return null;
        }

        const configStr = await window.go.main.App.GetConfig();
        const config = JSON.parse(configStr);

        document.getElementById('proxyPort').textContent = config.port;
        document.getElementById('totalEndpoints').textContent = config.endpoints.length;

        const activeCount = config.endpoints.filter(ep => ep.enabled !== false).length;
        document.getElementById('activeEndpoints').textContent = activeCount;

        return config;
    } catch (error) {
        console.error('Failed to load config:', error);
        return null;
    }
}

export async function updatePort(port) {
    await window.go.main.App.UpdatePort(port);
}

export async function addEndpoint(clientType, name, url, key, transformer, model, remark, tags) {
    await window.go.main.App.AddEndpoint(clientType, name, url, key, transformer, model, remark || '', tags || '');
}

export async function updateEndpoint(clientType, index, name, url, key, transformer, model, remark, tags) {
    await window.go.main.App.UpdateEndpoint(clientType, index, name, url, key, transformer, model, remark || '', tags || '');
}

export async function removeEndpoint(clientType, index) {
    await window.go.main.App.RemoveEndpoint(clientType, index);
}

export async function toggleEndpoint(clientType, index, enabled) {
    await window.go.main.App.ToggleEndpoint(clientType, index, enabled);
}

export async function testEndpoint(clientType, index) {
    const resultStr = await window.go.main.App.TestEndpoint(clientType, index);
    return JSON.parse(resultStr);
}

export async function testEndpointLight(clientType, index) {
    const resultStr = await window.go.main.App.TestEndpointLight(clientType, index);
    return JSON.parse(resultStr);
}

export async function testAllEndpointsZeroCost(clientType) {
    const resultStr = await window.go.main.App.TestAllEndpointsZeroCost(clientType);
    return JSON.parse(resultStr);
}
