package service

import (
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/pricing"
	"github.com/lich0821/ccNexus/internal/proxy"
	"github.com/lich0821/ccNexus/internal/storage"
)

// CostService 成本统计服务
type CostService struct {
	proxy   *proxy.Proxy
	config  *config.Config
	storage storage.Storage
}

// NewCostService 创建成本服务
func NewCostService(p *proxy.Proxy, cfg *config.Config) *CostService {
	return &CostService{proxy: p, config: cfg}
}

// SetStorage 设置存储
func (s *CostService) SetStorage(st storage.Storage) {
	s.storage = st
}

// CostStats 成本统计结果
type CostStats struct {
	TotalCost       float64                    `json:"totalCost"`       // 总成本（美元）
	InputCost       float64                    `json:"inputCost"`       // 输入成本
	OutputCost      float64                    `json:"outputCost"`      // 输出成本
	CacheWriteCost  float64                    `json:"cacheWriteCost"`  // 缓存写入成本
	CacheReadCost   float64                    `json:"cacheReadCost"`   // 缓存读取成本
	CacheSavings    float64                    `json:"cacheSavings"`    // 缓存节省的成本
	EndpointCosts   map[string]*EndpointCost   `json:"endpointCosts"`   // 按端点的成本
	TransformerCosts map[string]*TransformerCost `json:"transformerCosts"` // 按转换器类型的成本
}

// EndpointCost 端点成本
type EndpointCost struct {
	EndpointName    string  `json:"endpointName"`
	Transformer     string  `json:"transformer"`
	Model           string  `json:"model"`
	TotalCost       float64 `json:"totalCost"`
	InputCost       float64 `json:"inputCost"`
	OutputCost      float64 `json:"outputCost"`
	CacheWriteCost  float64 `json:"cacheWriteCost"`
	CacheReadCost   float64 `json:"cacheReadCost"`
	InputTokens     int     `json:"inputTokens"`
	OutputTokens    int     `json:"outputTokens"`
	CacheWriteTokens int    `json:"cacheWriteTokens"`
	CacheReadTokens int     `json:"cacheReadTokens"`
	Requests        int     `json:"requests"`
}

// TransformerCost 按转换器类型的成本
type TransformerCost struct {
	Transformer string  `json:"transformer"`
	TotalCost   float64 `json:"totalCost"`
	InputCost   float64 `json:"inputCost"`
	OutputCost  float64 `json:"outputCost"`
	Requests    int     `json:"requests"`
}

// GetCostDaily 获取今日成本统计
func (s *CostService) GetCostDaily() string {
	today := time.Now().Format("2006-01-02")
	return s.getCostByDateRange(today, today, "daily")
}

// GetCostYesterday 获取昨日成本统计
func (s *CostService) GetCostYesterday() string {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	return s.getCostByDateRange(yesterday, yesterday, "yesterday")
}

// GetCostWeekly 获取本周成本统计
func (s *CostService) GetCostWeekly() string {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startDate := now.AddDate(0, 0, -(weekday - 1)).Format("2006-01-02")
	return s.getCostByDateRange(startDate, now.Format("2006-01-02"), "weekly")
}

// GetCostMonthly 获取本月成本统计
func (s *CostService) GetCostMonthly() string {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	return s.getCostByDateRange(startDate, now.Format("2006-01-02"), "monthly")
}

// GetCostByPeriod 根据周期获取成本统计
func (s *CostService) GetCostByPeriod(period string) string {
	switch period {
	case "yesterday":
		return s.GetCostYesterday()
	case "weekly":
		return s.GetCostWeekly()
	case "monthly":
		return s.GetCostMonthly()
	default:
		return s.GetCostDaily()
	}
}

// getCostByDateRange 根据日期范围计算成本
func (s *CostService) getCostByDateRange(startDate, endDate, period string) string {
	// 获取统计数据
	var stats map[string]*proxy.DailyStats
	if startDate == endDate {
		stats = s.proxy.GetStats().GetDailyStats(startDate)
	} else {
		stats = s.proxy.GetStats().GetPeriodStats(startDate, endDate)
	}

	// 获取端点配置以确定转换器类型
	endpoints := s.config.GetEndpoints()
	endpointMap := make(map[string]config.Endpoint)
	for _, ep := range endpoints {
		key := ep.ClientType + ":" + ep.Name
		if ep.ClientType == "" {
			key = "claude:" + ep.Name
		}
		endpointMap[key] = ep
	}

	// 计算成本
	costStats := &CostStats{
		EndpointCosts:    make(map[string]*EndpointCost),
		TransformerCosts: make(map[string]*TransformerCost),
	}

	for key, stat := range stats {
		// 解析端点 key (clientType:endpointName)
		ep, ok := endpointMap[key]
		if !ok {
			// 尝试不带 clientType 的 key
			for k, e := range endpointMap {
				if k == "claude:"+key || k == key {
					ep = e
					ok = true
					break
				}
			}
		}

		// 确定转换器和模型
		transformer := "claude"
		model := ""
		if ok {
			if ep.Transformer != "" {
				transformer = ep.Transformer
			}
			model = ep.Model
		}

		// 获取定价
		pricingInfo := pricing.GetPricing(transformer, model)

		// 计算成本明细
		breakdown := pricing.CalculateCostBreakdown(
			stat.InputTokens,
			stat.OutputTokens,
			stat.CacheCreationTokens,
			stat.CacheReadTokens,
			pricingInfo,
		)

		// 计算缓存节省的成本（如果没有缓存，这些 token 会按输入价格计费）
		cacheSavings := float64(stat.CacheReadTokens) * (pricingInfo.InputPrice - pricingInfo.CacheReadPrice) / 1_000_000

		// 累加总成本
		costStats.TotalCost += breakdown.TotalCost
		costStats.InputCost += breakdown.InputCost
		costStats.OutputCost += breakdown.OutputCost
		costStats.CacheWriteCost += breakdown.CacheWriteCost
		costStats.CacheReadCost += breakdown.CacheReadCost
		costStats.CacheSavings += cacheSavings

		// 按端点统计
		if _, exists := costStats.EndpointCosts[key]; !exists {
			costStats.EndpointCosts[key] = &EndpointCost{
				EndpointName: key,
				Transformer:  transformer,
				Model:        model,
			}
		}
		epCost := costStats.EndpointCosts[key]
		epCost.TotalCost += breakdown.TotalCost
		epCost.InputCost += breakdown.InputCost
		epCost.OutputCost += breakdown.OutputCost
		epCost.CacheWriteCost += breakdown.CacheWriteCost
		epCost.CacheReadCost += breakdown.CacheReadCost
		epCost.InputTokens += stat.InputTokens
		epCost.OutputTokens += stat.OutputTokens
		epCost.CacheWriteTokens += stat.CacheCreationTokens
		epCost.CacheReadTokens += stat.CacheReadTokens
		epCost.Requests += stat.Requests

		// 按转换器类型统计
		if _, exists := costStats.TransformerCosts[transformer]; !exists {
			costStats.TransformerCosts[transformer] = &TransformerCost{
				Transformer: transformer,
			}
		}
		tCost := costStats.TransformerCosts[transformer]
		tCost.TotalCost += breakdown.TotalCost
		tCost.InputCost += breakdown.InputCost
		tCost.OutputCost += breakdown.OutputCost
		tCost.Requests += stat.Requests
	}

	result := map[string]interface{}{
		"success":          true,
		"period":           period,
		"totalCost":        costStats.TotalCost,
		"inputCost":        costStats.InputCost,
		"outputCost":       costStats.OutputCost,
		"cacheWriteCost":   costStats.CacheWriteCost,
		"cacheReadCost":    costStats.CacheReadCost,
		"cacheSavings":     costStats.CacheSavings,
		"endpointCosts":    costStats.EndpointCosts,
		"transformerCosts": costStats.TransformerCosts,
	}

	if startDate == endDate {
		result["date"] = startDate
	} else {
		result["startDate"] = startDate
		result["endDate"] = endDate
	}

	return toJSON(result)
}

// GetCostTrend 获取成本趋势对比
func (s *CostService) GetCostTrend(period string) string {
	now := time.Now()
	var currentStart, currentEnd, prevStart, prevEnd string

	switch period {
	case "yesterday":
		currentStart = now.AddDate(0, 0, -1).Format("2006-01-02")
		currentEnd = currentStart
		prevStart = now.AddDate(0, 0, -2).Format("2006-01-02")
		prevEnd = prevStart
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		thisWeekStart := now.AddDate(0, 0, -(weekday - 1))
		currentStart = thisWeekStart.Format("2006-01-02")
		currentEnd = now.Format("2006-01-02")
		prevStart = thisWeekStart.AddDate(0, 0, -7).Format("2006-01-02")
		prevEnd = thisWeekStart.AddDate(0, 0, -1).Format("2006-01-02")
	case "monthly":
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		currentStart = thisMonthStart.Format("2006-01-02")
		currentEnd = now.Format("2006-01-02")
		lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
		prevStart = lastMonthStart.Format("2006-01-02")
		prevEnd = thisMonthStart.AddDate(0, 0, -1).Format("2006-01-02")
	default: // daily
		currentStart = now.Format("2006-01-02")
		currentEnd = currentStart
		prevStart = now.AddDate(0, 0, -1).Format("2006-01-02")
		prevEnd = prevStart
	}

	currentCost := s.calculateTotalCost(currentStart, currentEnd)
	prevCost := s.calculateTotalCost(prevStart, prevEnd)

	trend := 0.0
	if prevCost > 0 {
		trend = ((currentCost - prevCost) / prevCost) * 100
		if trend > 100 {
			trend = 100
		} else if trend < -100 {
			trend = -100
		}
	} else if currentCost > 0 {
		trend = 100
	}

	return toJSON(map[string]interface{}{
		"success":      true,
		"currentCost":  currentCost,
		"previousCost": prevCost,
		"trend":        trend,
		"period":       period,
	})
}

// calculateTotalCost 计算指定日期范围的总成本
func (s *CostService) calculateTotalCost(startDate, endDate string) float64 {
	var stats map[string]*proxy.DailyStats
	if startDate == endDate {
		stats = s.proxy.GetStats().GetDailyStats(startDate)
	} else {
		stats = s.proxy.GetStats().GetPeriodStats(startDate, endDate)
	}

	endpoints := s.config.GetEndpoints()
	endpointMap := make(map[string]config.Endpoint)
	for _, ep := range endpoints {
		key := ep.ClientType + ":" + ep.Name
		if ep.ClientType == "" {
			key = "claude:" + ep.Name
		}
		endpointMap[key] = ep
	}

	var totalCost float64
	for key, stat := range stats {
		ep, ok := endpointMap[key]
		transformer := "claude"
		model := ""
		if ok {
			if ep.Transformer != "" {
				transformer = ep.Transformer
			}
			model = ep.Model
		}

		pricingInfo := pricing.GetPricing(transformer, model)
		totalCost += pricing.CalculateCost(
			stat.InputTokens,
			stat.OutputTokens,
			stat.CacheCreationTokens,
			stat.CacheReadTokens,
			pricingInfo,
		)
	}

	return totalCost
}

// GetPricingInfo 获取定价信息（供前端展示）
func (s *CostService) GetPricingInfo() string {
	return toJSON(map[string]interface{}{
		"success": true,
		"pricing": pricing.GetAllPricing(),
	})
}
