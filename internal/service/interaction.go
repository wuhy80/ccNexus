package service

import (
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
	return successJSON(map[string]interface{}{
		"enabled": enabled,
	})
}

// SetEnabled 设置交互记录是否启用
func (s *InteractionService) SetEnabled(enabled bool) string {
	s.storage.SetEnabled(enabled)

	// 保存到数据库
	if err := s.sqlStorage.SetConfig("interaction_enabled", strconv.FormatBool(enabled)); err != nil {
		return errorJSON(err.Error())
	}

	return successJSON(map[string]interface{}{
		"enabled": enabled,
	})
}

// GetDates 获取所有有记录的日期列表
func (s *InteractionService) GetDates() string {
	dates, err := s.storage.GetDates()
	if err != nil {
		return errorJSON(err.Error())
	}

	return successJSON(map[string]interface{}{
		"dates": dates,
	})
}

// GetInteractions 获取某天的交互记录列表
func (s *InteractionService) GetInteractions(date string) string {
	interactions, err := s.storage.GetInteractionsByDate(date)
	if err != nil {
		return errorJSON(err.Error())
	}

	if interactions == nil {
		interactions = []interaction.IndexEntry{}
	}

	return successJSON(map[string]interface{}{
		"interactions": interactions,
		"count":        len(interactions),
	})
}

// GetInteractionDetail 获取单个交互记录详情
func (s *InteractionService) GetInteractionDetail(date, requestID string) string {
	record, err := s.storage.GetInteraction(date, requestID)
	if err != nil {
		return errorJSON(err.Error())
	}

	return successJSON(map[string]interface{}{
		"interaction": record,
	})
}

// ExportInteractions 导出某天的所有交互记录
func (s *InteractionService) ExportInteractions(date string) string {
	records, err := s.storage.Export(date)
	if err != nil {
		return errorJSON(err.Error())
	}

	if records == nil {
		records = []interaction.Record{}
	}

	return successJSON(map[string]interface{}{
		"interactions": records,
		"count":        len(records),
		"date":         date,
	})
}

// Cleanup 清理旧记录
func (s *InteractionService) Cleanup(daysToKeep int) string {
	deleted, err := s.storage.Cleanup(daysToKeep)
	if err != nil {
		return errorJSON(err.Error())
	}

	return successJSON(map[string]interface{}{
		"deleted": deleted,
	})
}

// GetStoragePath 获取存储路径
func (s *InteractionService) GetStoragePath() string {
	return successJSON(map[string]interface{}{
		"path": s.storage.GetBaseDir(),
	})
}
