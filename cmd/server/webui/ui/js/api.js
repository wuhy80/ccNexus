// API Client for ccNexus
class APIClient {
    constructor(baseURL = '/api') {
        this.baseURL = baseURL;
    }

    async request(method, path, data = null) {
        const options = {
            method,
            headers: {
                'Content-Type': 'application/json'
            }
        };

        if (data) {
            options.body = JSON.stringify(data);
        }

        try {
            const response = await fetch(`${this.baseURL}${path}`, options);
            const result = await response.json();

            if (!response.ok) {
                throw new Error(result.error || 'Request failed');
            }

            return result.data || result;
        } catch (error) {
            console.error(`API Error [${method} ${path}]:`, error);
            throw error;
        }
    }

    // Endpoint management
    async getEndpoints(clientType = '') {
        const query = clientType ? `?clientType=${clientType}` : '';
        return this.request('GET', `/endpoints${query}`);
    }

    async createEndpoint(data) {
        return this.request('POST', '/endpoints', data);
    }

    async updateEndpoint(name, data) {
        return this.request('PUT', `/endpoints/${encodeURIComponent(name)}`, data);
    }

    async deleteEndpoint(name, clientType = 'claude') {
        return this.request('DELETE', `/endpoints/${encodeURIComponent(name)}?clientType=${clientType}`);
    }

    async toggleEndpoint(name, enabled, clientType = 'claude') {
        return this.request('PATCH', `/endpoints/${encodeURIComponent(name)}/toggle`, { enabled, clientType });
    }

    async testEndpoint(name, clientType = 'claude') {
        return this.request('POST', `/endpoints/${encodeURIComponent(name)}/test?clientType=${clientType}`);
    }

    async reorderEndpoints(names, clientType = 'claude') {
        return this.request('POST', '/endpoints/reorder', { names, clientType });
    }

    async getCurrentEndpoint(clientType = 'claude') {
        return this.request('GET', `/endpoints/current?clientType=${clientType}`);
    }

    async switchEndpoint(name, clientType = 'claude') {
        return this.request('POST', '/endpoints/switch', { name, clientType });
    }

    async fetchModels(apiUrl, apiKey, transformer) {
        return this.request('POST', '/endpoints/fetch-models', { apiUrl, apiKey, transformer });
    }

    // Statistics
    async getStatsSummary() {
        return this.request('GET', '/stats/summary');
    }

    async getStatsDaily() {
        return this.request('GET', '/stats/daily');
    }

    async getStatsWeekly() {
        return this.request('GET', '/stats/weekly');
    }

    async getStatsMonthly() {
        return this.request('GET', '/stats/monthly');
    }

    async getStatsTrends() {
        return this.request('GET', '/stats/trends');
    }

    // Configuration
    async getConfig() {
        return this.request('GET', '/config');
    }

    async updateConfig(data) {
        return this.request('PUT', '/config', data);
    }

    async getPort() {
        return this.request('GET', '/config/port');
    }

    async updatePort(port) {
        return this.request('PUT', '/config/port', { port });
    }

    async getLogLevel() {
        return this.request('GET', '/config/log-level');
    }

    async updateLogLevel(logLevel) {
        return this.request('PUT', '/config/log-level', { logLevel });
    }

    // Connected clients
    async getConnectedClients(hours = 24) {
        return this.request('GET', `/clients?hours=${hours}`);
    }
}

export const api = new APIClient();
