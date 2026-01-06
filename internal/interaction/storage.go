package interaction

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Storage 交互记录文件存储
type Storage struct {
	baseDir string
	enabled bool
	mu      sync.RWMutex
}

// NewStorage 创建新的存储实例
func NewStorage(baseDir string) *Storage {
	return &Storage{
		baseDir: baseDir,
		enabled: true, // 默认启用
	}
}

// SetEnabled 设置是否启用交互记录
func (s *Storage) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// IsEnabled 获取是否启用交互记录
func (s *Storage) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// GenerateRequestID 生成唯一的请求 ID
func GenerateRequestID() string {
	return uuid.New().String()
}

// Save 保存交互记录到文件
func (s *Storage) Save(record *Record) error {
	if !s.IsEnabled() {
		return nil
	}

	// 创建日期目录
	dateDir := record.Timestamp.Format("2006-01-02")
	dirPath := filepath.Join(s.baseDir, dateDir)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 生成文件名: HH-MM-SS-requestId前8位.json
	timeStr := record.Timestamp.Format("15-04-05")
	shortID := record.RequestID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	fileName := fmt.Sprintf("%s-%s.json", timeStr, shortID)
	filePath := filepath.Join(dirPath, fileName)

	// 序列化为 JSON
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetDates 获取所有有记录的日期列表（降序排列）
func (s *Storage) GetDates() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if entry.IsDir() {
			// 验证是否为有效的日期格式
			name := entry.Name()
			if _, err := time.Parse("2006-01-02", name); err == nil {
				dates = append(dates, name)
			}
		}
	}

	// 降序排列（最新的在前）
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	return dates, nil
}

// GetInteractionsByDate 获取某天的所有交互记录索引
func (s *Storage) GetInteractionsByDate(date string) ([]IndexEntry, error) {
	dirPath := filepath.Join(s.baseDir, date)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []IndexEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var results []IndexEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		record, err := s.readRecordFile(filePath)
		if err != nil {
			continue // 跳过无法读取的文件
		}

		results = append(results, record.ToIndexEntry())
	}

	// 按时间降序排列
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	return results, nil
}

// GetInteraction 获取单个交互记录详情
func (s *Storage) GetInteraction(date, requestID string) (*Record, error) {
	dirPath := filepath.Join(s.baseDir, date)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// 查找匹配的文件
	shortID := requestID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// 检查文件名是否包含 requestID
		if strings.Contains(entry.Name(), shortID) {
			filePath := filepath.Join(dirPath, entry.Name())
			record, err := s.readRecordFile(filePath)
			if err != nil {
				return nil, err
			}

			// 验证完整的 requestID
			if record.RequestID == requestID || strings.HasPrefix(record.RequestID, shortID) {
				return record, nil
			}
		}
	}

	return nil, fmt.Errorf("interaction not found: %s", requestID)
}

// readRecordFile 读取单个记录文件
func (s *Storage) readRecordFile(filePath string) (*Record, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}

	return &record, nil
}

// Cleanup 清理超过指定天数的旧记录
func (s *Storage) Cleanup(daysToKeep int) (int, error) {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep).Format("2006-01-02")

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read directory: %w", err)
	}

	deletedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 验证是否为有效的日期格式
		if _, err := time.Parse("2006-01-02", name); err != nil {
			continue
		}

		// 如果日期早于截止日期，删除整个目录
		if name < cutoffDate {
			dirPath := filepath.Join(s.baseDir, name)
			if err := os.RemoveAll(dirPath); err != nil {
				continue // 跳过删除失败的目录
			}
			deletedCount++
		}
	}

	return deletedCount, nil
}

// Export 导出某天的所有交互记录
func (s *Storage) Export(date string) ([]Record, error) {
	dirPath := filepath.Join(s.baseDir, date)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var results []Record
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		record, err := s.readRecordFile(filePath)
		if err != nil {
			continue // 跳过无法读取的文件
		}

		results = append(results, *record)
	}

	// 按时间排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, nil
}

// GetBaseDir 获取基础目录路径
func (s *Storage) GetBaseDir() string {
	return s.baseDir
}
