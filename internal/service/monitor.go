package service

import (
	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/proxy"
)

// MonitorService provides monitoring data to the frontend
type MonitorService struct {
	monitor *proxy.Monitor
	config  *config.Config
}

// NewMonitorService creates a new MonitorService
func NewMonitorService(monitor *proxy.Monitor, cfg *config.Config) *MonitorService {
	return &MonitorService{monitor: monitor, config: cfg}
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

// GetEndpointHealth returns health status for all enabled endpoints
func (s *MonitorService) GetEndpointHealth() string {
	if s.monitor == nil {
		return toJSON([]proxy.EndpointHealth{})
	}

	// Get all enabled endpoint names
	enabledEndpoints := s.getEnabledEndpointNames()

	return toJSON(s.monitor.GetEndpointHealth(enabledEndpoints))
}

// getEnabledEndpointNames returns names of all enabled endpoints
func (s *MonitorService) getEnabledEndpointNames() []string {
	if s.config == nil {
		return []string{}
	}

	endpoints := s.config.GetEndpoints()
	names := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Enabled {
			names = append(names, ep.Name)
		}
	}
	return names
}
