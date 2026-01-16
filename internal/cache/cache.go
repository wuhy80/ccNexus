package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/lich0821/ccNexus/internal/logger"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string    `json:"key"`
	Response   []byte    `json:"response"`    // 缓存的响应数据
	Headers    []byte    `json:"headers"`     // 缓存的响应头
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	HitCount   int       `json:"hitCount"`    // 命中次数
	IsStreaming bool     `json:"isStreaming"` // 是否为流式响应
}

// CacheStats 缓存统计
type CacheStats struct {
	TotalEntries int   `json:"totalEntries"`
	TotalHits    int64 `json:"totalHits"`
	TotalMisses  int64 `json:"totalMisses"`
	TotalSize    int64 `json:"totalSize"` // 字节
}

// Cache 请求缓存
type Cache struct {
	entries    map[string]*CacheEntry
	mu         sync.RWMutex
	ttl        time.Duration
	maxEntries int
	enabled    bool
	stats      CacheStats
}

// New 创建新的缓存实例
func New(enabled bool, ttlSeconds, maxEntries int) *Cache {
	if ttlSeconds <= 0 {
		ttlSeconds = 300 // 默认5分钟
	}
	if maxEntries <= 0 {
		maxEntries = 1000 // 默认1000条
	}

	c := &Cache{
		entries:    make(map[string]*CacheEntry),
		ttl:        time.Duration(ttlSeconds) * time.Second,
		maxEntries: maxEntries,
		enabled:    enabled,
	}

	// 启动清理协程
	if enabled {
		go c.cleanupLoop()
	}

	return c
}

// IsEnabled 返回缓存是否启用
func (c *Cache) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置缓存启用状态
func (c *Cache) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
}

// GenerateKey 根据请求内容生成缓存键
// 使用 model + messages 的 SHA256 哈希作为键
func GenerateKey(body []byte) string {
	// 解析请求体，提取关键字段
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		// 如果解析失败，直接对整个 body 哈希
		hash := sha256.Sum256(body)
		return hex.EncodeToString(hash[:])
	}

	// 提取用于缓存键的字段
	keyData := make(map[string]interface{})

	// 模型名称
	if model, ok := req["model"]; ok {
		keyData["model"] = model
	}

	// 消息内容
	if messages, ok := req["messages"]; ok {
		keyData["messages"] = messages
	}

	// 系统提示
	if system, ok := req["system"]; ok {
		keyData["system"] = system
	}

	// 温度参数（影响输出）
	if temp, ok := req["temperature"]; ok {
		keyData["temperature"] = temp
	}

	// max_tokens
	if maxTokens, ok := req["max_tokens"]; ok {
		keyData["max_tokens"] = maxTokens
	}

	// 序列化并哈希
	keyBytes, _ := json.Marshal(keyData)
	hash := sha256.Sum256(keyBytes)
	return hex.EncodeToString(hash[:])
}

// Get 获取缓存条目
func (c *Cache) Get(key string) (*CacheEntry, bool) {
	if !c.enabled {
		return nil, false
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.stats.TotalMisses++
		c.mu.Unlock()
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.stats.TotalMisses++
		c.mu.Unlock()
		return nil, false
	}

	// 更新命中计数
	c.mu.Lock()
	entry.HitCount++
	c.stats.TotalHits++
	c.mu.Unlock()

	logger.Debug("[CACHE] Hit: %s (hits: %d)", key[:16], entry.HitCount)
	return entry, true
}

// Set 设置缓存条目
func (c *Cache) Set(key string, response, headers []byte, isStreaming bool) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否需要清理空间
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	now := time.Now()
	c.entries[key] = &CacheEntry{
		Key:         key,
		Response:    response,
		Headers:     headers,
		CreatedAt:   now,
		ExpiresAt:   now.Add(c.ttl),
		HitCount:    0,
		IsStreaming: isStreaming,
	}

	logger.Debug("[CACHE] Set: %s (ttl: %v, streaming: %v)", key[:16], c.ttl, isStreaming)
}

// evictOldest 清除最旧的缓存条目（需要持有锁）
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		logger.Debug("[CACHE] Evicted oldest entry: %s", oldestKey[:16])
	}
}

// cleanupLoop 定期清理过期条目
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if !c.enabled {
			continue
		}
		c.cleanup()
	}
}

// cleanup 清理过期条目
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expired := 0
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			expired++
		}
	}

	if expired > 0 {
		logger.Debug("[CACHE] Cleaned up %d expired entries", expired)
	}
}

// GetStats 获取缓存统计信息
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.TotalEntries = len(c.entries)

	// 计算总大小
	var totalSize int64
	for _, entry := range c.entries {
		totalSize += int64(len(entry.Response) + len(entry.Headers))
	}
	stats.TotalSize = totalSize

	return stats
}

// Clear 清空所有缓存
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	logger.Info("[CACHE] Cache cleared")
}

// UpdateConfig 更新缓存配置
func (c *Cache) UpdateConfig(enabled bool, ttlSeconds, maxEntries int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = enabled
	if ttlSeconds > 0 {
		c.ttl = time.Duration(ttlSeconds) * time.Second
	}
	if maxEntries > 0 {
		c.maxEntries = maxEntries
	}

	logger.Info("[CACHE] Config updated: enabled=%v, ttl=%v, maxEntries=%d",
		enabled, c.ttl, c.maxEntries)
}
