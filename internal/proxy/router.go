package proxy

import (
	"errors"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/logger"
)

// Router 智能路由选择器
type Router struct {
	config  *config.Config
	monitor *Monitor
	mu      sync.RWMutex

	// 轮询索引（每个客户端类型独立）
	roundRobinIndex map[ClientType]int

	// 线程安全的随机数生成器
	rng *rand.Rand
}

// NewRouter 创建路由器
func NewRouter(cfg *config.Config, monitor *Monitor) *Router {
	return &Router{
		config:          cfg,
		monitor:         monitor,
		roundRobinIndex: make(map[ClientType]int),
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectEndpoint 选择端点（组合策略）
// 1. 模型匹配过滤 → 2. 配额过滤 → 3. 按成本/负载/优先级排序选择
// 注意：只从可用状态(available)的端点中选择
func (r *Router) SelectEndpoint(clientType ClientType, requestModel string, quotaTracker *QuotaTracker) (config.Endpoint, error) {
	routingCfg := r.config.GetRoutingConfig()
	endpoints := r.config.GetAvailableEndpointsByClient(string(clientType))

	if len(endpoints) == 0 {
		return config.Endpoint{}, errors.New("no available endpoints for client type")
	}

	// 记录初始可用端点
	r.logEndpointSelection("初始可用端点", endpoints)

	// 步骤1: 模型匹配过滤
	if routingCfg.EnableModelRouting && requestModel != "" {
		beforeCount := len(endpoints)
		endpoints = r.filterByModel(endpoints, requestModel)
		r.logFilterStep("模型匹配过滤", requestModel, beforeCount, endpoints)
	}

	// 步骤2: 配额过滤（排除已用尽的）
	if routingCfg.EnableQuotaRouting && quotaTracker != nil {
		beforeCount := len(endpoints)
		endpoints = r.filterByQuota(endpoints, clientType, quotaTracker)
		r.logFilterStep("配额过滤", "", beforeCount, endpoints)
	}

	if len(endpoints) == 0 {
		return config.Endpoint{}, errors.New("no available endpoints after filtering")
	}

	// 步骤3: 排序选择
	var selectedEndpoint config.Endpoint
	var err error
	var selectionMethod string

	if routingCfg.EnableCostPriority {
		selectionMethod = "成本优先"
		selectedEndpoint, err = r.selectByCost(endpoints)
	} else if routingCfg.EnableLoadBalance {
		algorithm := r.config.GetLoadBalanceAlgorithm()
		selectionMethod = "负载均衡-" + algorithm
		selectedEndpoint, err = r.selectByLoad(endpoints, clientType)
	} else {
		selectionMethod = "优先级"
		selectedEndpoint, err = r.selectByPriority(endpoints)
	}

	if err == nil {
		r.logFinalSelection(selectionMethod, selectedEndpoint, endpoints)
	}

	return selectedEndpoint, err
}

// filterByModel 按模型模式过滤端点
func (r *Router) filterByModel(endpoints []config.Endpoint, model string) []config.Endpoint {
	var matched []config.Endpoint
	for _, ep := range endpoints {
		if r.matchesModelPattern(model, ep.ModelPatterns) {
			matched = append(matched, ep)
		}
	}
	if len(matched) == 0 {
		return endpoints // 无匹配时返回全部
	}
	return matched
}

// matchesModelPattern 通配符匹配
// 支持格式：空（匹配所有）、"*"（匹配所有）、"claude-*"（前缀匹配）、"*-opus"（后缀匹配）、精确匹配
func (r *Router) matchesModelPattern(model, patterns string) bool {
	if patterns == "" {
		return true // 空模式匹配所有
	}

	for _, pattern := range strings.Split(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == "*" {
			return true
		}
		// 前缀匹配
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(model, prefix) {
				return true
			}
		}
		// 后缀匹配
		if strings.HasPrefix(pattern, "*") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(model, suffix) {
				return true
			}
		}
		// 精确匹配
		if model == pattern {
			return true
		}
	}
	return false
}

// filterByQuota 过滤掉配额用尽的端点
func (r *Router) filterByQuota(endpoints []config.Endpoint, clientType ClientType, quotaTracker *QuotaTracker) []config.Endpoint {
	var available []config.Endpoint
	for _, ep := range endpoints {
		// 没有配额限制的端点始终可用
		if ep.QuotaLimit == 0 {
			available = append(available, ep)
			continue
		}
		// 检查配额是否用尽
		if !quotaTracker.IsExhausted(ep.Name, string(clientType)) {
			available = append(available, ep)
		}
	}
	if len(available) == 0 {
		return endpoints // 所有端点配额用尽时返回全部（让请求继续但可能失败）
	}
	return available
}

// selectByCost 按成本排序选择（成本越低越优先）
func (r *Router) selectByCost(endpoints []config.Endpoint) (config.Endpoint, error) {
	if len(endpoints) == 0 {
		return config.Endpoint{}, errors.New("no endpoints")
	}

	// 复制切片以避免修改原数组
	sorted := make([]config.Endpoint, len(endpoints))
	copy(sorted, endpoints)

	sort.Slice(sorted, func(i, j int) bool {
		costI := sorted[i].CostPerInputToken + sorted[i].CostPerOutputToken
		costJ := sorted[j].CostPerInputToken + sorted[j].CostPerOutputToken
		if costI == costJ {
			return sorted[i].Priority < sorted[j].Priority
		}
		return costI < costJ
	})

	return sorted[0], nil
}

// selectByLoad 基于负载均衡选择端点
func (r *Router) selectByLoad(endpoints []config.Endpoint, clientType ClientType) (config.Endpoint, error) {
	if len(endpoints) == 0 {
		return config.Endpoint{}, errors.New("no endpoints")
	}

	algorithm := r.config.GetLoadBalanceAlgorithm()

	switch algorithm {
	case "fastest":
		return r.selectFastest(endpoints)
	case "weighted":
		return r.selectWeightedRandom(endpoints)
	default: // "round_robin"
		return r.selectRoundRobin(endpoints, clientType)
	}
}

// selectFastest 选择响应最快的端点
func (r *Router) selectFastest(endpoints []config.Endpoint) (config.Endpoint, error) {
	if r.monitor == nil {
		return endpoints[0], nil
	}

	var best config.Endpoint
	var bestLatency float64 = math.MaxFloat64
	hasMetrics := false

	for _, ep := range endpoints {
		metric := r.monitor.GetEndpointMetric(ep.Name)
		if metric == nil || metric.TotalRequests == 0 {
			continue
		}
		hasMetrics = true
		if metric.AvgResponseTime < bestLatency {
			bestLatency = metric.AvgResponseTime
			best = ep
		}
	}

	if !hasMetrics {
		// 没有历史数据，返回第一个
		return endpoints[0], nil
	}

	return best, nil
}

// selectWeightedRandom 加权随机选择（响应时间越短权重越高）
func (r *Router) selectWeightedRandom(endpoints []config.Endpoint) (config.Endpoint, error) {
	if r.monitor == nil || len(endpoints) == 1 {
		return endpoints[0], nil
	}

	// 计算权重：使用响应时间的倒数
	weights := make([]float64, len(endpoints))
	totalWeight := 0.0

	for i, ep := range endpoints {
		metric := r.monitor.GetEndpointMetric(ep.Name)
		if metric == nil || metric.AvgResponseTime <= 0 {
			weights[i] = 1.0 // 默认权重
		} else {
			weights[i] = 1.0 / metric.AvgResponseTime
		}
		totalWeight += weights[i]
	}

	// 加权随机选择（使用线程安全的随机数生成器）
	r.mu.Lock()
	randVal := r.rng.Float64() * totalWeight
	r.mu.Unlock()

	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if randVal <= cumulative {
			return endpoints[i], nil
		}
	}

	return endpoints[len(endpoints)-1], nil
}

// selectRoundRobin 轮询选择
func (r *Router) selectRoundRobin(endpoints []config.Endpoint, clientType ClientType) (config.Endpoint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	index := r.roundRobinIndex[clientType] % len(endpoints)
	r.roundRobinIndex[clientType] = (index + 1) % len(endpoints)

	return endpoints[index], nil
}

// selectByPriority 按优先级选择端点（相同优先级随机选择）
func (r *Router) selectByPriority(endpoints []config.Endpoint) (config.Endpoint, error) {
	if len(endpoints) == 0 {
		return config.Endpoint{}, errors.New("no endpoints")
	}

	// 复制切片以避免修改原数组
	sorted := make([]config.Endpoint, len(endpoints))
	copy(sorted, endpoints)

	// 按优先级排序（数字越小优先级越高）
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	// 找出所有具有最高优先级的端点
	highestPriority := sorted[0].Priority
	var highestPriorityEndpoints []config.Endpoint
	for _, ep := range sorted {
		if ep.Priority == highestPriority {
			highestPriorityEndpoints = append(highestPriorityEndpoints, ep)
		} else {
			break // 已排序，后面的优先级更低
		}
	}

	logger.Debug("[路由选择] 最高优先级=%d, 该优先级端点数=%d", highestPriority, len(highestPriorityEndpoints))

	// 如果只有一个最高优先级端点，直接返回
	if len(highestPriorityEndpoints) == 1 {
		logger.Debug("[路由选择] 唯一最高优先级端点: %s", highestPriorityEndpoints[0].Name)
		return highestPriorityEndpoints[0], nil
	}

	// 多个相同优先级的端点，随机选择一个
	r.mu.Lock()
	randomIndex := r.rng.Intn(len(highestPriorityEndpoints))
	r.mu.Unlock()

	names := make([]string, len(highestPriorityEndpoints))
	for i, ep := range highestPriorityEndpoints {
		names[i] = ep.Name
	}
	logger.Debug("[路由选择] 相同优先级端点: %v, 随机选择索引=%d", names, randomIndex)

	return highestPriorityEndpoints[randomIndex], nil
}

// ResetRoundRobinIndex 重置轮询索引
func (r *Router) ResetRoundRobinIndex(clientType ClientType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roundRobinIndex[clientType] = 0
}

// UpdateConfig 更新配置引用
func (r *Router) UpdateConfig(cfg *config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = cfg
}

// logEndpointSelection 记录初始端点列表
func (r *Router) logEndpointSelection(stage string, endpoints []config.Endpoint) {
	names := make([]string, len(endpoints))
	priorities := make([]int, len(endpoints))
	for i, ep := range endpoints {
		names[i] = ep.Name
		priorities[i] = ep.Priority
	}
	logger.Info("[路由选择] %s: 共%d个 [%v] 优先级: %v", stage, len(endpoints), names, priorities)
}

// logFilterStep 记录过滤步骤
func (r *Router) logFilterStep(filterName, filterValue string, beforeCount int, afterEndpoints []config.Endpoint) {
	afterCount := len(afterEndpoints)
	if filterValue != "" {
		logger.Info("[路由选择] %s (模型=%s): %d个 → %d个", filterName, filterValue, beforeCount, afterCount)
	} else {
		logger.Info("[路由选择] %s: %d个 → %d个", filterName, beforeCount, afterCount)
	}
	if afterCount > 0 {
		names := make([]string, afterCount)
		for i, ep := range afterEndpoints {
			names[i] = ep.Name
		}
		logger.Info("[路由选择] 剩余端点: %v", names)
	}
}

// logFinalSelection 记录最终选择结果
func (r *Router) logFinalSelection(method string, selected config.Endpoint, candidates []config.Endpoint) {
	logger.Info("[路由选择] 选择方法: %s", method)
	logger.Info("[路由选择] ✅ 最终选择: %s (优先级=%d)", selected.Name, selected.Priority)

	// 显示候选端点的详细信息
	if len(candidates) > 1 {
		logger.Info("[路由选择] 候选端点详情:")
		for _, ep := range candidates {
			marker := "  "
			if ep.Name == selected.Name {
				marker = "✅"
			}
			logger.Info("[路由选择]   %s %s (优先级=%d, 成本=%.6f)",
				marker, ep.Name, ep.Priority,
				ep.CostPerInputToken+ep.CostPerOutputToken)
		}
	}
}
