package proxy

import (
	"sync"
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/storage"
)

// QuotaTracker 配额跟踪器
type QuotaTracker struct {
	storage storage.Storage
	config  *config.Config
	cache   sync.Map // map[string]*QuotaRecord
}

// QuotaRecord 配额记录（内存缓存）
type QuotaRecord struct {
	EndpointName string
	ClientType   string
	PeriodStart  time.Time
	PeriodEnd    time.Time
	TokensUsed   int64
	QuotaLimit   int64
	LastUpdated  time.Time
}

// NewQuotaTracker 创建配额跟踪器
func NewQuotaTracker(cfg *config.Config, store storage.Storage) *QuotaTracker {
	qt := &QuotaTracker{
		storage: store,
		config:  cfg,
	}
	// 启动时加载现有配额数据
	qt.loadExistingQuotas()
	return qt
}

// loadExistingQuotas 加载现有配额数据到内存
func (q *QuotaTracker) loadExistingQuotas() {
	endpoints := q.config.GetEndpoints()
	for _, ep := range endpoints {
		if ep.QuotaLimit > 0 {
			clientType := ep.ClientType
			if clientType == "" {
				clientType = "claude"
			}
			quota, err := q.storage.GetEndpointQuota(ep.Name, clientType)
			if err == nil && quota != nil {
				key := clientType + ":" + ep.Name
				q.cache.Store(key, &QuotaRecord{
					EndpointName: quota.EndpointName,
					ClientType:   quota.ClientType,
					PeriodStart:  quota.PeriodStart,
					PeriodEnd:    quota.PeriodEnd,
					TokensUsed:   quota.TokensUsed,
					QuotaLimit:   quota.QuotaLimit,
					LastUpdated:  quota.LastUpdated,
				})
			}
		}
	}
}

// RecordUsage 记录 Token 使用
func (q *QuotaTracker) RecordUsage(endpointName, clientType string, tokens int64) {
	if clientType == "" {
		clientType = "claude"
	}

	// 获取端点配置
	var endpoint *config.Endpoint
	for _, ep := range q.config.GetEndpoints() {
		if ep.Name == endpointName {
			epClientType := ep.ClientType
			if epClientType == "" {
				epClientType = "claude"
			}
			if epClientType == clientType {
				endpoint = &ep
				break
			}
		}
	}

	// 没有配额限制的端点不需要跟踪
	if endpoint == nil || endpoint.QuotaLimit == 0 {
		return
	}

	key := clientType + ":" + endpointName
	now := time.Now()

	// 获取或创建配额记录
	var record *QuotaRecord
	if cached, ok := q.cache.Load(key); ok {
		record = cached.(*QuotaRecord)
	} else {
		// 从存储加载或创建新记录
		record = q.loadOrCreateQuota(endpointName, clientType, endpoint)
	}

	// 检查是否需要重置周期
	if now.After(record.PeriodEnd) {
		record = q.resetQuota(endpointName, clientType, endpoint)
	}

	// 更新使用量
	record.TokensUsed += tokens
	record.LastUpdated = now
	q.cache.Store(key, record)

	// 异步持久化
	go q.persistQuota(record)
}

// loadOrCreateQuota 加载或创建配额记录
func (q *QuotaTracker) loadOrCreateQuota(endpointName, clientType string, endpoint *config.Endpoint) *QuotaRecord {
	// 尝试从存储加载
	quota, err := q.storage.GetEndpointQuota(endpointName, clientType)
	if err == nil && quota != nil {
		return &QuotaRecord{
			EndpointName: quota.EndpointName,
			ClientType:   quota.ClientType,
			PeriodStart:  quota.PeriodStart,
			PeriodEnd:    quota.PeriodEnd,
			TokensUsed:   quota.TokensUsed,
			QuotaLimit:   quota.QuotaLimit,
			LastUpdated:  quota.LastUpdated,
		}
	}

	// 创建新记录
	return q.createNewQuotaRecord(endpointName, clientType, endpoint)
}

// createNewQuotaRecord 创建新的配额记录
func (q *QuotaTracker) createNewQuotaRecord(endpointName, clientType string, endpoint *config.Endpoint) *QuotaRecord {
	now := time.Now()
	periodStart, periodEnd := q.calculatePeriod(endpoint.QuotaResetCycle, now)

	return &QuotaRecord{
		EndpointName: endpointName,
		ClientType:   clientType,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		TokensUsed:   0,
		QuotaLimit:   endpoint.QuotaLimit,
		LastUpdated:  now,
	}
}

// resetQuota 重置配额记录
func (q *QuotaTracker) resetQuota(endpointName, clientType string, endpoint *config.Endpoint) *QuotaRecord {
	now := time.Now()
	periodStart, periodEnd := q.calculatePeriod(endpoint.QuotaResetCycle, now)

	record := &QuotaRecord{
		EndpointName: endpointName,
		ClientType:   clientType,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		TokensUsed:   0,
		QuotaLimit:   endpoint.QuotaLimit,
		LastUpdated:  now,
	}

	key := clientType + ":" + endpointName
	q.cache.Store(key, record)

	return record
}

// calculatePeriod 计算周期的开始和结束时间
func (q *QuotaTracker) calculatePeriod(resetCycle string, now time.Time) (start, end time.Time) {
	switch resetCycle {
	case "daily":
		// 当天开始到当天结束
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 1).Add(-time.Second)
	case "weekly":
		// 本周一开始到本周日结束
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // 周日算作第7天
		}
		// 使用 AddDate 来正确处理跨月情况
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(weekday - 1))
		end = start.AddDate(0, 0, 7).Add(-time.Second)
	case "monthly":
		// 本月1日开始到本月最后一天结束
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 1, 0).Add(-time.Second)
	default: // "never" 或空
		// 使用很远的未来作为结束时间（相当于永不重置）
		start = time.Date(2020, 1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(2099, 12, 31, 23, 59, 59, 0, now.Location())
	}
	return
}

// persistQuota 持久化配额记录
func (q *QuotaTracker) persistQuota(record *QuotaRecord) {
	quota := &storage.EndpointQuota{
		EndpointName: record.EndpointName,
		ClientType:   record.ClientType,
		PeriodStart:  record.PeriodStart,
		PeriodEnd:    record.PeriodEnd,
		TokensUsed:   record.TokensUsed,
		QuotaLimit:   record.QuotaLimit,
		LastUpdated:  record.LastUpdated,
	}
	_ = q.storage.UpdateEndpointQuota(quota)
}

// IsExhausted 检查配额是否用尽
func (q *QuotaTracker) IsExhausted(endpointName, clientType string) bool {
	if clientType == "" {
		clientType = "claude"
	}

	key := clientType + ":" + endpointName
	cached, ok := q.cache.Load(key)
	if !ok {
		return false // 没有记录说明没有配额限制或尚未使用
	}

	record := cached.(*QuotaRecord)

	// 检查周期是否已过期
	if time.Now().After(record.PeriodEnd) {
		return false // 周期已过期，会在下次使用时重置
	}

	// 无限配额
	if record.QuotaLimit == 0 {
		return false
	}

	return record.TokensUsed >= record.QuotaLimit
}

// GetRemainingQuota 获取剩余配额
func (q *QuotaTracker) GetRemainingQuota(endpointName, clientType string) (remaining int64, percentage float64) {
	if clientType == "" {
		clientType = "claude"
	}

	key := clientType + ":" + endpointName
	cached, ok := q.cache.Load(key)
	if !ok {
		return -1, 100.0 // 无限配额
	}

	record := cached.(*QuotaRecord)

	// 无限配额
	if record.QuotaLimit == 0 {
		return -1, 100.0
	}

	// 检查周期是否已过期
	if time.Now().After(record.PeriodEnd) {
		return record.QuotaLimit, 100.0 // 周期已过期，视为满配额
	}

	remaining = record.QuotaLimit - record.TokensUsed
	if remaining < 0 {
		remaining = 0
	}
	percentage = float64(remaining) / float64(record.QuotaLimit) * 100

	return remaining, percentage
}

// GetQuotaStatus 获取配额状态
func (q *QuotaTracker) GetQuotaStatus(endpointName, clientType string) *QuotaRecord {
	if clientType == "" {
		clientType = "claude"
	}

	key := clientType + ":" + endpointName
	cached, ok := q.cache.Load(key)
	if !ok {
		return nil
	}

	return cached.(*QuotaRecord)
}

// GetAllQuotaStatuses 获取所有配额状态
func (q *QuotaTracker) GetAllQuotaStatuses(clientType string) []*QuotaRecord {
	if clientType == "" {
		clientType = "claude"
	}

	var records []*QuotaRecord
	q.cache.Range(func(key, value interface{}) bool {
		record := value.(*QuotaRecord)
		if record.ClientType == clientType {
			records = append(records, record)
		}
		return true
	})

	return records
}

// ResetQuota 手动重置配额
func (q *QuotaTracker) ResetQuota(endpointName, clientType string) error {
	if clientType == "" {
		clientType = "claude"
	}

	// 获取端点配置
	var endpoint *config.Endpoint
	for _, ep := range q.config.GetEndpoints() {
		if ep.Name == endpointName {
			epClientType := ep.ClientType
			if epClientType == "" {
				epClientType = "claude"
			}
			if epClientType == clientType {
				endpoint = &ep
				break
			}
		}
	}

	if endpoint == nil {
		return nil
	}

	record := q.resetQuota(endpointName, clientType, endpoint)
	q.persistQuota(record)
	return nil
}

// UpdateConfig 更新配置引用
func (q *QuotaTracker) UpdateConfig(cfg *config.Config) {
	q.config = cfg
}
