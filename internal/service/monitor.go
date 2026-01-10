package service

import (
	"github.com/lich0821/ccNexus/internal/proxy"
)

// MonitorService provides monitoring data to the frontend
type MonitorService struct {
	monitor *proxy.Monitor
}

// NewMonitorService creates a new MonitorService
func NewMonitorService(monitor *proxy.Monitor) *MonitorService {
	return &MonitorService{monitor: monitor}
}

// GetSnapshot returns a snapshot of current monitoring data
func (s *MonitorService) GetSnapshot() string {
	if s.monitor == nil {
		return toJSON(proxy.MonitorSnapshot{
			ActiveRequests:  []proxy.ActiveRequest{},
			EndpointMetrics: []proxy.EndpointMetric{},
		})
	}
	return toJSON(s.monitor.GetSnapshot())
}

// GetActiveRequests returns all active requests as JSON
func (s *MonitorService) GetActiveRequests() string {
	if s.monitor == nil {
		return toJSON([]proxy.ActiveRequest{})
	}
	return toJSON(s.monitor.GetActiveRequests())
}

// GetEndpointMetrics returns metrics for all endpoints as JSON
func (s *MonitorService) GetEndpointMetrics() string {
	if s.monitor == nil {
		return toJSON([]proxy.EndpointMetric{})
	}
	return toJSON(s.monitor.GetEndpointMetrics())
}

// ResetMetrics resets all endpoint metrics
func (s *MonitorService) ResetMetrics() {
	if s.monitor != nil {
		s.monitor.ResetMetrics()
	}
}
