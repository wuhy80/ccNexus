package service

import (
	"encoding/json"
	"strconv"

	"github.com/lich0821/ccNexus/internal/interaction"
	"github.com/lich0821/ccNexus/internal/storage"
)

// InteractionService 交互记录服务
type InteractionService struct {
	storage    *interaction.Storage
	sqlStorage *storage.SQLiteStorage
}

// NewInteractionService 创建交互服务实例
func NewInteractionService(interactionStorage *interaction.Storage, sqlStorage *storage.SQLiteStorage) *InteractionService {
	s := &InteractionService{
		storage:    interactionStorage,
		sqlStorage: sqlStorage,
	}

	// 从数据库加载 enabled 状态
	if enabledStr, err := sqlStorage.GetConfig("interaction_enabled"); err == nil && enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			interactionStorage.SetEnabled(enabled)
		}
	}
	// 如果数据库中没有配置，使用 Storage 的默认值（true）

	return s
}

// GetEnabled 获取交互记录是否启用
func (s *InteractionService) GetEnabled() string {
	enabled := s.storage.IsEnabled()
	result := map[string]interface{}{
		"success": true,
		"enabled": enabled,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// SetEnabled 设置交互记录是否启用
func (s *InteractionService) SetEnabled(enabled bool) string {
	s.storage.SetEnabled(enabled)

	// 保存到数据库
	if err := s.sqlStorage.SetConfig("interaction_enabled", strconv.FormatBool(enabled)); err != nil {
		result := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	result := map[string]interface{}{
		"success": true,
		"enabled": enabled,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// GetDates 获取所有有记录的日期列表
func (s *InteractionService) GetDates() string {
	dates, err := s.storage.GetDates()
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	result := map[string]interface{}{
		"success": true,
		"dates":   dates,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// GetInteractions 获取某天的交互记录列表
func (s *InteractionService) GetInteractions(date string) string {
	interactions, err := s.storage.GetInteractionsByDate(date)
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	if interactions == nil {
		interactions = []interaction.IndexEntry{}
	}

	result := map[string]interface{}{
		"success":      true,
		"interactions": interactions,
		"count":        len(interactions),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// GetInteractionDetail 获取单个交互记录详情
func (s *InteractionService) GetInteractionDetail(date, requestID string) string {
	record, err := s.storage.GetInteraction(date, requestID)
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	result := map[string]interface{}{
		"success":     true,
		"interaction": record,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ExportInteractions 导出某天的所有交互记录
func (s *InteractionService) ExportInteractions(date string) string {
	records, err := s.storage.Export(date)
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	if records == nil {
		records = []interaction.Record{}
	}

	result := map[string]interface{}{
		"success":      true,
		"interactions": records,
		"count":        len(records),
		"date":         date,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// Cleanup 清理旧记录
func (s *InteractionService) Cleanup(daysToKeep int) string {
	deleted, err := s.storage.Cleanup(daysToKeep)
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	result := map[string]interface{}{
		"success": true,
		"deleted": deleted,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// GetStoragePath 获取存储路径
func (s *InteractionService) GetStoragePath() string {
	result := map[string]interface{}{
		"success": true,
		"path":    s.storage.GetBaseDir(),
	}
	data, _ := json.Marshal(result)
	return string(data)
}
