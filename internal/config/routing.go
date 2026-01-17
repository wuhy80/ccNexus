package config

// RoutingConfig 智能路由配置
type RoutingConfig struct {
	// 启用的策略（按顺序执行）
	EnableModelRouting bool `json:"enableModelRouting"` // 启用模型匹配路由
	EnableLoadBalance  bool `json:"enableLoadBalance"`  // 启用负载均衡
	EnableCostPriority bool `json:"enableCostPriority"` // 启用成本优先
	EnableQuotaRouting bool `json:"enableQuotaRouting"` // 启用配额路由

	// 负载均衡算法：fastest（最快响应）、weighted（加权随机）、round_robin（轮询）
	LoadBalanceAlgorithm string `json:"loadBalanceAlgorithm"`
}

// DefaultRoutingConfig 返回默认路由配置
func DefaultRoutingConfig() *RoutingConfig {
	return &RoutingConfig{
		EnableModelRouting:   false,
		EnableLoadBalance:    false,
		EnableCostPriority:   false,
		EnableQuotaRouting:   false,
		LoadBalanceAlgorithm: "round_robin",
	}
}

// GetRoutingConfig 获取路由配置（线程安全）
func (c *Config) GetRoutingConfig() *RoutingConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Routing == nil {
		return DefaultRoutingConfig()
	}
	return c.Routing
}

// UpdateRoutingConfig 更新路由配置（线程安全）
func (c *Config) UpdateRoutingConfig(routing *RoutingConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Routing = routing
}

// IsModelRoutingEnabled 是否启用模型匹配路由
func (c *Config) IsModelRoutingEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Routing != nil && c.Routing.EnableModelRouting
}

// IsLoadBalanceEnabled 是否启用负载均衡
func (c *Config) IsLoadBalanceEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Routing != nil && c.Routing.EnableLoadBalance
}

// IsCostPriorityEnabled 是否启用成本优先
func (c *Config) IsCostPriorityEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Routing != nil && c.Routing.EnableCostPriority
}

// IsQuotaRoutingEnabled 是否启用配额路由
func (c *Config) IsQuotaRoutingEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Routing != nil && c.Routing.EnableQuotaRouting
}

// GetLoadBalanceAlgorithm 获取负载均衡算法
func (c *Config) GetLoadBalanceAlgorithm() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Routing == nil || c.Routing.LoadBalanceAlgorithm == "" {
		return "round_robin"
	}
	return c.Routing.LoadBalanceAlgorithm
}
