package service

import (
	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/proxy"
	"github.com/lich0821/ccNexus/internal/storage"
)

// QuotaStatus 配额状态信息
type QuotaStatus struct {
	EndpointName   string  `json:"endpointName"`
	ClientType     string  `json:"clientType"`
	TokensUsed     int64   `json:"tokensUsed"`
	QuotaLimit     int64   `json:"quotaLimit"`
	RemainingQuota int64   `json:"remainingQuota"`
	UsagePercent   float64 `json:"usagePercent"` // 已使用百分比
	PeriodStart    string  `json:"periodStart"`
	PeriodEnd      string  `json:"periodEnd"`
	IsExhausted    bool    `json:"isExhausted"`
}

// RoutingService 路由服务
type RoutingService struct {
	config  *config.Config
	storage *storage.SQLiteStorage
	proxy   *proxy.Proxy
}

// NewRoutingService 创建路由服务
func NewRoutingService(cfg *config.Config, store *storage.SQLiteStorage, p *proxy.Proxy) *RoutingService {
	return &RoutingService{
		config:  cfg,
		storage: store,
		proxy:   p,
	}
}

// GetRoutingConfig 获取路由配置
func (s *RoutingService) GetRoutingConfig() *config.RoutingConfig {
	return s.config.GetRoutingConfig()
}

// UpdateRoutingConfig 更新路由配置
func (s *RoutingService) UpdateRoutingConfig(cfg *config.RoutingConfig) error {
	s.config.UpdateRoutingConfig(cfg)

	// 持久化到存储
	if s.storage != nil {
		configAdapter := storage.NewConfigStorageAdapter(s.storage)
		if err := s.config.SaveToStorage(configAdapter); err != nil {
			return err
		}
	}

	// 更新代理的路由器配置
	if s.proxy != nil {
		s.proxy.UpdateRouterConfig(s.config)
	}

	return nil
}

// GetQuotaStatuses 获取所有端点的配额状态
func (s *RoutingService) GetQuotaStatuses(clientType string) []QuotaStatus {
	quotaTracker := s.proxy.GetQuotaTracker()
	if quotaTracker == nil {
		return []QuotaStatus{}
	}

	if clientType == "" {
		clientType = "claude"
	}

	records := quotaTracker.GetAllQuotaStatuses(clientType)
	statuses := make([]QuotaStatus, 0, len(records))

	for _, record := range records {
		remaining, _ := quotaTracker.GetRemainingQuota(record.EndpointName, record.ClientType)

		var usagePercent float64
		if record.QuotaLimit > 0 {
			usagePercent = float64(record.TokensUsed) / float64(record.QuotaLimit) * 100
			if usagePercent > 100 {
				usagePercent = 100
			}
		}

		statuses = append(statuses, QuotaStatus{
			EndpointName:   record.EndpointName,
			ClientType:     record.ClientType,
			TokensUsed:     record.TokensUsed,
			QuotaLimit:     record.QuotaLimit,
			RemainingQuota: remaining,
			UsagePercent:   usagePercent,
			PeriodStart:    record.PeriodStart.Format("2006-01-02 15:04:05"),
			PeriodEnd:      record.PeriodEnd.Format("2006-01-02 15:04:05"),
			IsExhausted:    quotaTracker.IsExhausted(record.EndpointName, record.ClientType),
		})
	}

	return statuses
}

// GetQuotaStatus 获取单个端点的配额状态
func (s *RoutingService) GetQuotaStatus(endpointName, clientType string) *QuotaStatus {
	quotaTracker := s.proxy.GetQuotaTracker()
	if quotaTracker == nil {
		return nil
	}

	if clientType == "" {
		clientType = "claude"
	}

	record := quotaTracker.GetQuotaStatus(endpointName, clientType)
	if record == nil {
		return nil
	}

	remaining, _ := quotaTracker.GetRemainingQuota(endpointName, clientType)

	var usagePercent float64
	if record.QuotaLimit > 0 {
		usagePercent = float64(record.TokensUsed) / float64(record.QuotaLimit) * 100
		if usagePercent > 100 {
			usagePercent = 100
		}
	}

	return &QuotaStatus{
		EndpointName:   record.EndpointName,
		ClientType:     record.ClientType,
		TokensUsed:     record.TokensUsed,
		QuotaLimit:     record.QuotaLimit,
		RemainingQuota: remaining,
		UsagePercent:   usagePercent,
		PeriodStart:    record.PeriodStart.Format("2006-01-02 15:04:05"),
		PeriodEnd:      record.PeriodEnd.Format("2006-01-02 15:04:05"),
		IsExhausted:    quotaTracker.IsExhausted(endpointName, clientType),
	}
}

// ResetQuota 重置端点配额
func (s *RoutingService) ResetQuota(endpointName, clientType string) error {
	quotaTracker := s.proxy.GetQuotaTracker()
	if quotaTracker == nil {
		return nil
	}

	return quotaTracker.ResetQuota(endpointName, clientType)
}
