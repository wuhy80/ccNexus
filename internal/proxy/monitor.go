package proxy

import (
	"sync"
	"time"
)

// RequestPhase represents the current phase of a request
type RequestPhase string

const (
	PhaseWaiting    RequestPhase = "waiting"
	PhaseConnecting RequestPhase = "connecting"
	PhaseSending    RequestPhase = "sending"
	PhaseStreaming  RequestPhase = "streaming"
	PhaseCompleted  RequestPhase = "completed"
	PhaseFailed     RequestPhase = "failed"
)

// ActiveRequest represents a request currently being processed
type ActiveRequest struct {
	RequestID     string       `json:"requestId"`
	EndpointName  string       `json:"endpointName"`
	ClientType    string       `json:"clientType"`
	Model         string       `json:"model"`
	StartTime     time.Time    `json:"startTime"`
	Phase         RequestPhase `json:"phase"`
	BytesReceived int64        `json:"bytesReceived"`
}

// EndpointMetric holds performance metrics for an endpoint
type EndpointMetric struct {
	EndpointName    string  `json:"endpointName"`
	ActiveCount     int     `json:"activeCount"`
	TotalRequests   int     `json:"totalRequests"`
	SuccessCount    int     `json:"successCount"`
	AvgResponseTime float64 `json:"avgResponseTime"`
	SuccessRate     float64 `json:"successRate"`
	LastError       string  `json:"lastError"`
	LastErrorTime   int64   `json:"lastErrorTime"`
}

// MonitorSnapshot represents a point-in-time snapshot of monitoring data
type MonitorSnapshot struct {
	ActiveRequests  []ActiveRequest  `json:"activeRequests"`
	EndpointMetrics []EndpointMetric `json:"endpointMetrics"`
}

// MonitorEventType represents the type of monitor event
type MonitorEventType string

const (
	EventRequestStarted   MonitorEventType = "request_started"
	EventRequestUpdated   MonitorEventType = "request_updated"
	EventRequestCompleted MonitorEventType = "request_completed"
	EventMetricsUpdated   MonitorEventType = "metrics_updated"
)

// MonitorEvent represents an event to be sent to the frontend
type MonitorEvent struct {
	Type    MonitorEventType `json:"type"`
	Request *ActiveRequest   `json:"request,omitempty"`
	Metrics *EndpointMetric  `json:"metrics,omitempty"`
}

// EventCallback is a function that handles monitor events
type EventCallback func(event MonitorEvent)

// Monitor tracks active requests and endpoint metrics
type Monitor struct {
	mu              sync.RWMutex
	activeRequests  map[string]*ActiveRequest
	endpointMetrics map[string]*EndpointMetric
	eventCallback   EventCallback

	// Rolling window for metrics (last 100 requests per endpoint)
	responseTimes map[string][]float64
	maxSamples    int
}

// NewMonitor creates a new Monitor instance
func NewMonitor() *Monitor {
	return &Monitor{
		activeRequests:  make(map[string]*ActiveRequest),
		endpointMetrics: make(map[string]*EndpointMetric),
		responseTimes:   make(map[string][]float64),
		maxSamples:      100,
	}
}

// SetEventCallback sets the callback function for monitor events
func (m *Monitor) SetEventCallback(callback EventCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = callback
}

// StartRequest records the start of a new request
func (m *Monitor) StartRequest(requestID, endpointName, clientType, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req := &ActiveRequest{
		RequestID:     requestID,
		EndpointName:  endpointName,
		ClientType:    clientType,
		Model:         model,
		StartTime:     time.Now(),
		Phase:         PhaseConnecting,
		BytesReceived: 0,
	}

	m.activeRequests[requestID] = req

	// Update endpoint active count
	metric := m.getOrCreateMetric(endpointName)
	metric.ActiveCount++

	// Emit events
	if m.eventCallback != nil {
		m.eventCallback(MonitorEvent{
			Type:    EventRequestStarted,
			Request: req.clone(),
		})
		m.eventCallback(MonitorEvent{
			Type:    EventMetricsUpdated,
			Metrics: metric.clone(),
		})
	}
}

// UpdatePhase updates the phase of an active request
func (m *Monitor) UpdatePhase(requestID string, phase RequestPhase) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.activeRequests[requestID]
	if !exists {
		return
	}

	req.Phase = phase

	if m.eventCallback != nil {
		m.eventCallback(MonitorEvent{
			Type:    EventRequestUpdated,
			Request: req.clone(),
		})
	}
}

// UpdateBytes updates the bytes received for a streaming request
func (m *Monitor) UpdateBytes(requestID string, bytesReceived int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.activeRequests[requestID]
	if !exists {
		return
	}

	req.BytesReceived = bytesReceived
	req.Phase = PhaseStreaming

	if m.eventCallback != nil {
		m.eventCallback(MonitorEvent{
			Type:    EventRequestUpdated,
			Request: req.clone(),
		})
	}
}

// CompleteRequest marks a request as completed
func (m *Monitor) CompleteRequest(requestID string, success bool, errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.activeRequests[requestID]
	if !exists {
		return
	}

	// Calculate response time
	responseTime := time.Since(req.StartTime).Seconds()
	endpointName := req.EndpointName

	// Update request phase
	if success {
		req.Phase = PhaseCompleted
	} else {
		req.Phase = PhaseFailed
	}

	// Remove from active requests
	delete(m.activeRequests, requestID)

	// Update endpoint metrics
	metric := m.getOrCreateMetric(endpointName)
	metric.ActiveCount--
	if metric.ActiveCount < 0 {
		metric.ActiveCount = 0
	}
	metric.TotalRequests++
	if success {
		metric.SuccessCount++
	} else {
		metric.LastError = errorMsg
		metric.LastErrorTime = time.Now().UnixMilli()
	}

	// Update rolling average response time
	m.addResponseTime(endpointName, responseTime)
	metric.AvgResponseTime = m.calculateAvgResponseTime(endpointName)

	// Calculate success rate
	if metric.TotalRequests > 0 {
		metric.SuccessRate = float64(metric.SuccessCount) / float64(metric.TotalRequests) * 100
	}

	// Emit events
	if m.eventCallback != nil {
		m.eventCallback(MonitorEvent{
			Type:    EventRequestCompleted,
			Request: req.clone(),
		})
		m.eventCallback(MonitorEvent{
			Type:    EventMetricsUpdated,
			Metrics: metric.clone(),
		})
	}
}

// GetSnapshot returns a snapshot of current monitoring data
func (m *Monitor) GetSnapshot() MonitorSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := MonitorSnapshot{
		ActiveRequests:  make([]ActiveRequest, 0, len(m.activeRequests)),
		EndpointMetrics: make([]EndpointMetric, 0, len(m.endpointMetrics)),
	}

	for _, req := range m.activeRequests {
		snapshot.ActiveRequests = append(snapshot.ActiveRequests, *req.clone())
	}

	for _, metric := range m.endpointMetrics {
		snapshot.EndpointMetrics = append(snapshot.EndpointMetrics, *metric.clone())
	}

	return snapshot
}

// GetActiveRequests returns all active requests
func (m *Monitor) GetActiveRequests() []ActiveRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]ActiveRequest, 0, len(m.activeRequests))
	for _, req := range m.activeRequests {
		requests = append(requests, *req.clone())
	}
	return requests
}

// GetEndpointMetrics returns metrics for all endpoints
func (m *Monitor) GetEndpointMetrics() []EndpointMetric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make([]EndpointMetric, 0, len(m.endpointMetrics))
	for _, metric := range m.endpointMetrics {
		metrics = append(metrics, *metric.clone())
	}
	return metrics
}

// ResetMetrics resets all endpoint metrics (but keeps active requests)
func (m *Monitor) ResetMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, metric := range m.endpointMetrics {
		metric.TotalRequests = 0
		metric.SuccessCount = 0
		metric.AvgResponseTime = 0
		metric.SuccessRate = 0
		metric.LastError = ""
		metric.LastErrorTime = 0
		m.responseTimes[name] = nil
	}
}

// Helper methods

func (m *Monitor) getOrCreateMetric(endpointName string) *EndpointMetric {
	metric, exists := m.endpointMetrics[endpointName]
	if !exists {
		metric = &EndpointMetric{
			EndpointName: endpointName,
		}
		m.endpointMetrics[endpointName] = metric
	}
	return metric
}

func (m *Monitor) addResponseTime(endpointName string, responseTime float64) {
	times := m.responseTimes[endpointName]
	times = append(times, responseTime)
	if len(times) > m.maxSamples {
		times = times[1:]
	}
	m.responseTimes[endpointName] = times
}

func (m *Monitor) calculateAvgResponseTime(endpointName string) float64 {
	times := m.responseTimes[endpointName]
	if len(times) == 0 {
		return 0
	}

	var sum float64
	for _, t := range times {
		sum += t
	}
	return sum / float64(len(times))
}

func (r *ActiveRequest) clone() *ActiveRequest {
	return &ActiveRequest{
		RequestID:     r.RequestID,
		EndpointName:  r.EndpointName,
		ClientType:    r.ClientType,
		Model:         r.Model,
		StartTime:     r.StartTime,
		Phase:         r.Phase,
		BytesReceived: r.BytesReceived,
	}
}

func (m *EndpointMetric) clone() *EndpointMetric {
	return &EndpointMetric{
		EndpointName:    m.EndpointName,
		ActiveCount:     m.ActiveCount,
		TotalRequests:   m.TotalRequests,
		SuccessCount:    m.SuccessCount,
		AvgResponseTime: m.AvgResponseTime,
		SuccessRate:     m.SuccessRate,
		LastError:       m.LastError,
		LastErrorTime:   m.LastErrorTime,
	}
}

// EndpointHealth represents the health status of an endpoint
type EndpointHealth struct {
	EndpointName    string  `json:"endpointName"`
	Status          string  `json:"status"` // "healthy", "warning", "error"
	ActiveCount     int     `json:"activeCount"`
	SuccessRate     float64 `json:"successRate"`
	AvgResponseTime float64 `json:"avgResponseTime"`
	LastError       string  `json:"lastError,omitempty"`
	LastErrorTime   int64   `json:"lastErrorTime,omitempty"`
}

// GetEndpointHealth returns health status for all specified endpoints
func (m *Monitor) GetEndpointHealth(enabledEndpoints []string) []EndpointHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := make([]EndpointHealth, 0, len(enabledEndpoints))

	for _, name := range enabledEndpoints {
		h := EndpointHealth{
			EndpointName: name,
			Status:       "healthy", // Default status
		}

		if metric, exists := m.endpointMetrics[name]; exists {
			h.ActiveCount = metric.ActiveCount
			h.SuccessRate = metric.SuccessRate
			h.AvgResponseTime = metric.AvgResponseTime
			h.LastError = metric.LastError
			h.LastErrorTime = metric.LastErrorTime

			// Calculate health status
			h.Status = calculateHealthStatus(metric)
		}

		health = append(health, h)
	}

	return health
}

// calculateHealthStatus determines the health status based on metrics
func calculateHealthStatus(metric *EndpointMetric) string {
	// Check for recent errors (within 5 minutes)
	if metric.LastErrorTime > 0 {
		fiveMinutesAgo := time.Now().Add(-5 * time.Minute).UnixMilli()
		if metric.LastErrorTime > fiveMinutesAgo {
			return "error"
		}
	}

	// Check success rate (only if we have enough data)
	if metric.TotalRequests > 0 {
		if metric.SuccessRate < 80 {
			return "error"
		}
		if metric.SuccessRate < 95 {
			return "warning"
		}
	}

	return "healthy"
}
