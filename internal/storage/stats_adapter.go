package storage

import (
	"reflect"
	"time"
)

// StatsStorageAdapter adapts SQLiteStorage to be used by proxy.Stats
// It implements the proxy.StatsStorage interface
type StatsStorageAdapter struct {
	storage *SQLiteStorage
}

// NewStatsStorageAdapter creates a new adapter
func NewStatsStorageAdapter(storage *SQLiteStorage) *StatsStorageAdapter {
	return &StatsStorageAdapter{storage: storage}
}

// RecordDailyStat records a daily stat
func (a *StatsStorageAdapter) RecordDailyStat(stat interface{}) error {
	// Use reflection to extract fields from the stat record
	v := reflect.ValueOf(stat)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	dailyStat := &DailyStat{
		EndpointName:        v.FieldByName("EndpointName").String(),
		Date:                v.FieldByName("Date").String(),
		Requests:            int(v.FieldByName("Requests").Int()),
		Errors:              int(v.FieldByName("Errors").Int()),
		InputTokens:         int(v.FieldByName("InputTokens").Int()),
		CacheCreationTokens: int(v.FieldByName("CacheCreationTokens").Int()),
		CacheReadTokens:     int(v.FieldByName("CacheReadTokens").Int()),
		OutputTokens:        int(v.FieldByName("OutputTokens").Int()),
		DeviceID:            v.FieldByName("DeviceID").String(),
	}
	return a.storage.RecordDailyStat(dailyStat)
}

// RecordRequestStat records a request-level stat (新增)
func (a *StatsStorageAdapter) RecordRequestStat(stat interface{}) error {
	v := reflect.ValueOf(stat)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	requestStat := &RequestStat{
		EndpointName:        v.FieldByName("EndpointName").String(),
		RequestID:           v.FieldByName("RequestID").String(),
		Timestamp:           v.FieldByName("Timestamp").Interface().(time.Time),
		Date:                v.FieldByName("Date").String(),
		InputTokens:         int(v.FieldByName("InputTokens").Int()),
		CacheCreationTokens: int(v.FieldByName("CacheCreationTokens").Int()),
		CacheReadTokens:     int(v.FieldByName("CacheReadTokens").Int()),
		OutputTokens:        int(v.FieldByName("OutputTokens").Int()),
		Model:               v.FieldByName("Model").String(),
		IsStreaming:         v.FieldByName("IsStreaming").Bool(),
		Success:             v.FieldByName("Success").Bool(),
		DeviceID:            v.FieldByName("DeviceID").String(),
	}
	return a.storage.RecordRequestStat(requestStat)
}

// GetTotalStats gets total stats for all endpoints
func (a *StatsStorageAdapter) GetTotalStats() (int, map[string]interface{}, error) {
	totalRequests, endpointStats, err := a.storage.GetTotalStats()
	if err != nil {
		return 0, nil, err
	}

	result := make(map[string]interface{})
	for name, stats := range endpointStats {
		result[name] = &StatsDataCompat{
			Requests:            stats.Requests,
			Errors:              stats.Errors,
			InputTokens:         stats.InputTokens,
			CacheCreationTokens: stats.CacheCreationTokens,
			CacheReadTokens:     stats.CacheReadTokens,
			OutputTokens:        stats.OutputTokens,
		}
	}

	return totalRequests, result, nil
}

// StatsDataCompat is a compatible stats data structure
type StatsDataCompat struct {
	Requests            int
	Errors              int
	InputTokens         int64
	CacheCreationTokens int64 // 新增
	CacheReadTokens     int64 // 新增
	OutputTokens        int64
}

// GetDailyStats gets daily stats for an endpoint
func (a *StatsStorageAdapter) GetDailyStats(endpointName, startDate, endDate string) ([]interface{}, error) {
	dailyStats, err := a.storage.GetDailyStats(endpointName, startDate, endDate)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(dailyStats))
	for i, stat := range dailyStats {
		result[i] = &DailyRecordCompat{
			Date:                stat.Date,
			Requests:            stat.Requests,
			Errors:              stat.Errors,
			InputTokens:         stat.InputTokens,
			CacheCreationTokens: stat.CacheCreationTokens,
			CacheReadTokens:     stat.CacheReadTokens,
			OutputTokens:        stat.OutputTokens,
		}
	}

	return result, nil
}

// DailyRecordCompat is a compatible daily record structure
type DailyRecordCompat struct {
	Date                string
	Requests            int
	Errors              int
	InputTokens         int
	CacheCreationTokens int // 新增
	CacheReadTokens     int // 新增
	OutputTokens        int
}
