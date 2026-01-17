package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/logger"
)

// SessionAffinityManager 会话亲和性管理器
type SessionAffinityManager struct {
	mu               sync.RWMutex
	sessionBindings  map[string]*SessionBinding // sessionID -> binding
	endpointSessions map[string][]string        // endpointName -> []sessionID
	config           *config.Config

	sessionTimeout  time.Duration // 会话超时时间
	cleanupInterval time.Duration // 清理间隔
	stopChan        chan struct{}
}

// SessionBinding 会话绑定信息
type SessionBinding struct {
	SessionID    string
	EndpointName string
	ClientType   string
	LastAccess   time.Time
	RequestCount int64
	CreatedAt    time.Time
}

// NewSessionAffinityManager 创建会话亲和性管理器
func NewSessionAffinityManager(cfg *config.Config) *SessionAffinityManager {
	timeoutHours := 24 // 默认24小时
	if cfg.SessionAffinity != nil && cfg.SessionAffinity.SessionTimeoutHours > 0 {
		timeoutHours = cfg.SessionAffinity.SessionTimeoutHours
	}

	return &SessionAffinityManager{
		sessionBindings:  make(map[string]*SessionBinding),
		endpointSessions: make(map[string][]string),
		config:           cfg,
		sessionTimeout:   time.Duration(timeoutHours) * time.Hour,
		cleanupInterval:  time.Hour, // 每小时清理一次
		stopChan:         make(chan struct{}),
	}
}

// Start 启动后台清理任务
func (s *SessionAffinityManager) Start() {
	go s.cleanupLoop()
	logger.Info("Session affinity manager started (timeout: %v)", s.sessionTimeout)
}

// Stop 停止后台清理任务
func (s *SessionAffinityManager) Stop() {
	close(s.stopChan)
	logger.Info("Session affinity manager stopped")
}

// cleanupLoop 后台清理循环
func (s *SessionAffinityManager) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.CleanupExpiredSessions()
		case <-s.stopChan:
			return
		}
	}
}

// ExtractSessionID 从请求中提取会话ID
func (s *SessionAffinityManager) ExtractSessionID(r *http.Request) string {
	// 1. 优先使用客户端提供的会话ID
	if sessionID := r.Header.Get("X-CCNexus-Session-ID"); sessionID != "" {
		return sessionID
	}

	// 2. 使用请求ID（如果客户端提供）
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		return requestID
	}

	// 3. 回退：基于客户端IP生成会话ID
	clientIP := s.getClientIPFromRequest(r)
	userAgent := r.Header.Get("User-Agent")
	return generateSessionHash(clientIP, userAgent)
}

// getClientIPFromRequest 获取客户端IP（内部方法）
func (s *SessionAffinityManager) getClientIPFromRequest(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 从 X-Real-IP 获取
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 从 RemoteAddr 获取
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// generateSessionHash 生成会话哈希
func generateSessionHash(clientIP, userAgent string) string {
	data := clientIP + "|" + userAgent
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16] // 取前16个字符
}

// GetEndpointForSession 获取会话绑定的端点
func (s *SessionAffinityManager) GetEndpointForSession(sessionID, clientType string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	binding, exists := s.sessionBindings[sessionID]
	if !exists {
		return "", false
	}

	// 检查客户端类型是否匹配
	if binding.ClientType != clientType {
		return "", false
	}

	// 检查是否过期
	if time.Since(binding.LastAccess) > s.sessionTimeout {
		return "", false
	}

	return binding.EndpointName, true
}

// BindSession 绑定会话到端点
func (s *SessionAffinityManager) BindSession(sessionID, endpointName, clientType string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 检查是否已存在绑定
	if binding, exists := s.sessionBindings[sessionID]; exists {
		// 如果绑定到不同的端点，需要更新
		if binding.EndpointName != endpointName {
			// 从旧端点的会话列表中移除
			s.removeSessionFromEndpoint(sessionID, binding.EndpointName)
			// 添加到新端点的会话列表
			s.addSessionToEndpoint(sessionID, endpointName)
			binding.EndpointName = endpointName
		}
		binding.LastAccess = now
		binding.RequestCount++
		return
	}

	// 创建新绑定
	s.sessionBindings[sessionID] = &SessionBinding{
		SessionID:    sessionID,
		EndpointName: endpointName,
		ClientType:   clientType,
		LastAccess:   now,
		RequestCount: 1,
		CreatedAt:    now,
	}

	// 添加到端点的会话列表
	s.addSessionToEndpoint(sessionID, endpointName)

	logger.Debug("[SESSION:%s] Bound to endpoint: %s (client: %s)", sessionID, endpointName, clientType)
}

// UnbindSession 解除会话绑定
func (s *SessionAffinityManager) UnbindSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	binding, exists := s.sessionBindings[sessionID]
	if !exists {
		return
	}

	// 从端点的会话列表中移除
	s.removeSessionFromEndpoint(sessionID, binding.EndpointName)

	// 删除绑定
	delete(s.sessionBindings, sessionID)

	logger.Debug("[SESSION:%s] Unbound from endpoint: %s", sessionID, binding.EndpointName)
}

// IsNewSession 判断是否为新会话
func (s *SessionAffinityManager) IsNewSession(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.sessionBindings[sessionID]
	return !exists
}

// CleanupExpiredSessions 清理过期会话
func (s *SessionAffinityManager) CleanupExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for sessionID, binding := range s.sessionBindings {
		if now.Sub(binding.LastAccess) > s.sessionTimeout {
			// 从端点的会话列表中移除
			s.removeSessionFromEndpoint(sessionID, binding.EndpointName)
			// 删除绑定
			delete(s.sessionBindings, sessionID)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		logger.Info("Cleaned up %d expired sessions", expiredCount)
	}
}

// GetStats 获取会话统计信息
func (s *SessionAffinityManager) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 统计每个端点的会话数
	endpointStats := make(map[string]int)
	for endpointName, sessions := range s.endpointSessions {
		endpointStats[endpointName] = len(sessions)
	}

	// 构建会话绑定列表
	bindings := make([]map[string]interface{}, 0, len(s.sessionBindings))
	for _, binding := range s.sessionBindings {
		bindings = append(bindings, map[string]interface{}{
			"sessionId":    binding.SessionID,
			"endpointName": binding.EndpointName,
			"clientType":   binding.ClientType,
			"lastAccess":   binding.LastAccess.Unix(),
			"requestCount": binding.RequestCount,
			"createdAt":    binding.CreatedAt.Unix(),
		})
	}

	return map[string]interface{}{
		"enabled":           true,
		"totalSessions":     len(s.sessionBindings),
		"endpointStats":     endpointStats,
		"sessionBindings":   bindings,
		"sessionTimeout":    s.sessionTimeout.Hours(),
		"cleanupInterval":   s.cleanupInterval.Hours(),
	}
}

// CanBindToEndpoint 检查是否可以绑定到端点（考虑并发限制）
func (s *SessionAffinityManager) CanBindToEndpoint(endpointName string) bool {
	if s.config.SessionAffinity == nil || s.config.SessionAffinity.MaxConcurrentPerEndpoint == 0 {
		return true // 无限制
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := s.endpointSessions[endpointName]
	return len(sessions) < s.config.SessionAffinity.MaxConcurrentPerEndpoint
}

// addSessionToEndpoint 添加会话到端点的会话列表（内部方法，需要持有锁）
func (s *SessionAffinityManager) addSessionToEndpoint(sessionID, endpointName string) {
	sessions := s.endpointSessions[endpointName]
	// 检查是否已存在
	for _, sid := range sessions {
		if sid == sessionID {
			return
		}
	}
	s.endpointSessions[endpointName] = append(sessions, sessionID)
}

// removeSessionFromEndpoint 从端点的会话列表中移除会话（内部方法，需要持有锁）
func (s *SessionAffinityManager) removeSessionFromEndpoint(sessionID, endpointName string) {
	sessions := s.endpointSessions[endpointName]
	for i, sid := range sessions {
		if sid == sessionID {
			// 删除元素
			s.endpointSessions[endpointName] = append(sessions[:i], sessions[i+1:]...)
			break
		}
	}
	// 如果列表为空，删除键
	if len(s.endpointSessions[endpointName]) == 0 {
		delete(s.endpointSessions, endpointName)
	}
}
