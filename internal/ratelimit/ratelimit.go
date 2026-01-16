package ratelimit

import (
	"sync"
	"time"

	"github.com/lich0821/ccNexus/internal/logger"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	enabled         bool
	globalLimit     int           // 全局每分钟最大请求数
	perEndpointLimit int          // 每端点每分钟最大请求数
	windowSize      time.Duration // 时间窗口大小

	globalRequests    []time.Time            // 全局请求时间戳
	endpointRequests  map[string][]time.Time // 每端点请求时间戳

	mu sync.RWMutex

	// 统计
	totalAllowed  int64
	totalRejected int64
}

// RateLimitStats 速率限制统计
type RateLimitStats struct {
	Enabled          bool  `json:"enabled"`
	GlobalLimit      int   `json:"globalLimit"`
	PerEndpointLimit int   `json:"perEndpointLimit"`
	TotalAllowed     int64 `json:"totalAllowed"`
	TotalRejected    int64 `json:"totalRejected"`
	CurrentGlobalRPM int   `json:"currentGlobalRpm"` // 当前全局每分钟请求数
}

// New 创建新的速率限制器
func New(enabled bool, globalLimit, perEndpointLimit int) *RateLimiter {
	if globalLimit <= 0 {
		globalLimit = 60 // 默认每分钟60个请求
	}
	if perEndpointLimit <= 0 {
		perEndpointLimit = 30 // 默认每端点每分钟30个请求
	}

	rl := &RateLimiter{
		enabled:          enabled,
		globalLimit:      globalLimit,
		perEndpointLimit: perEndpointLimit,
		windowSize:       time.Minute,
		globalRequests:   make([]time.Time, 0),
		endpointRequests: make(map[string][]time.Time),
	}

	// 启动清理协程
	go rl.cleanupLoop()

	return rl
}

// IsEnabled 返回是否启用
func (rl *RateLimiter) IsEnabled() bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.enabled
}

// SetEnabled 设置启用状态
func (rl *RateLimiter) SetEnabled(enabled bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.enabled = enabled
}

// Allow 检查是否允许请求
// 返回 (是否允许, 等待时间建议)
func (rl *RateLimiter) Allow(endpointName string) (bool, time.Duration) {
	if !rl.enabled {
		return true, 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.windowSize)

	// 清理过期的全局请求记录
	rl.globalRequests = filterRecent(rl.globalRequests, windowStart)

	// 检查全局限制
	if len(rl.globalRequests) >= rl.globalLimit {
		rl.totalRejected++
		waitTime := rl.globalRequests[0].Add(rl.windowSize).Sub(now)
		logger.Debug("[RATELIMIT] Global limit reached (%d/%d), wait: %v",
			len(rl.globalRequests), rl.globalLimit, waitTime)
		return false, waitTime
	}

	// 清理过期的端点请求记录
	if rl.endpointRequests[endpointName] != nil {
		rl.endpointRequests[endpointName] = filterRecent(rl.endpointRequests[endpointName], windowStart)
	}

	// 检查端点限制
	if len(rl.endpointRequests[endpointName]) >= rl.perEndpointLimit {
		rl.totalRejected++
		waitTime := rl.endpointRequests[endpointName][0].Add(rl.windowSize).Sub(now)
		logger.Debug("[RATELIMIT] Endpoint %s limit reached (%d/%d), wait: %v",
			endpointName, len(rl.endpointRequests[endpointName]), rl.perEndpointLimit, waitTime)
		return false, waitTime
	}

	// 记录请求
	rl.globalRequests = append(rl.globalRequests, now)
	if rl.endpointRequests[endpointName] == nil {
		rl.endpointRequests[endpointName] = make([]time.Time, 0)
	}
	rl.endpointRequests[endpointName] = append(rl.endpointRequests[endpointName], now)
	rl.totalAllowed++

	return true, 0
}

// filterRecent 过滤出时间窗口内的记录
func filterRecent(times []time.Time, windowStart time.Time) []time.Time {
	result := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(windowStart) {
			result = append(result, t)
		}
	}
	return result
}

// cleanupLoop 定期清理过期记录
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup 清理过期记录
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	windowStart := time.Now().Add(-rl.windowSize)

	// 清理全局记录
	rl.globalRequests = filterRecent(rl.globalRequests, windowStart)

	// 清理端点记录
	for name, times := range rl.endpointRequests {
		rl.endpointRequests[name] = filterRecent(times, windowStart)
		// 删除空的端点记录
		if len(rl.endpointRequests[name]) == 0 {
			delete(rl.endpointRequests, name)
		}
	}
}

// GetStats 获取统计信息
func (rl *RateLimiter) GetStats() RateLimitStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// 计算当前每分钟请求数
	windowStart := time.Now().Add(-rl.windowSize)
	currentRPM := 0
	for _, t := range rl.globalRequests {
		if t.After(windowStart) {
			currentRPM++
		}
	}

	return RateLimitStats{
		Enabled:          rl.enabled,
		GlobalLimit:      rl.globalLimit,
		PerEndpointLimit: rl.perEndpointLimit,
		TotalAllowed:     rl.totalAllowed,
		TotalRejected:    rl.totalRejected,
		CurrentGlobalRPM: currentRPM,
	}
}

// UpdateConfig 更新配置
func (rl *RateLimiter) UpdateConfig(enabled bool, globalLimit, perEndpointLimit int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.enabled = enabled
	if globalLimit > 0 {
		rl.globalLimit = globalLimit
	}
	if perEndpointLimit > 0 {
		rl.perEndpointLimit = perEndpointLimit
	}

	logger.Info("[RATELIMIT] Config updated: enabled=%v, globalLimit=%d, perEndpointLimit=%d",
		enabled, rl.globalLimit, rl.perEndpointLimit)
}

// Reset 重置统计
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.totalAllowed = 0
	rl.totalRejected = 0
	rl.globalRequests = make([]time.Time, 0)
	rl.endpointRequests = make(map[string][]time.Time)

	logger.Info("[RATELIMIT] Stats reset")
}
