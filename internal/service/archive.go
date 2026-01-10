package service

import (
    "fmt"
    "time"

    "github.com/lich0821/ccNexus/internal/logger"
    "github.com/lich0821/ccNexus/internal/storage"
)

// ArchiveService handles archive data operations
type ArchiveService struct {
    storage *storage.SQLiteStorage
}

// NewArchiveService creates a new ArchiveService
func NewArchiveService(s *storage.SQLiteStorage) *ArchiveService {
    return &ArchiveService{storage: s}
}

// ListArchives returns a list of all available archive months
func (a *ArchiveService) ListArchives() string {
    if a.storage == nil {
        return toJSON(map[string]interface{}{
            "success":  false,
            "message":  "Storage not initialized",
            "archives": []string{},
        })
    }

    months, err := a.storage.GetArchiveMonths()
    if err != nil {
        logger.Error("Failed to get archive months: %v", err)
        return toJSON(map[string]interface{}{
            "success":  false,
            "message":  fmt.Sprintf("Failed to load archives: %v", err),
            "archives": []string{},
        })
    }

    return successJSON(map[string]interface{}{
        "archives": months,
    })
}

// GetArchiveData returns archived data for a specific month
func (a *ArchiveService) GetArchiveData(month string) string {
    if a.storage == nil {
        return errorJSON("Storage not initialized")
    }

    archiveData, err := a.storage.GetMonthlyArchiveData(month)
    if err != nil {
        logger.Error("Failed to get archive data for %s: %v", month, err)
        return errorJSON(fmt.Sprintf("Failed to load archive: %v", err))
    }

    endpoints := make(map[string]map[string]interface{})
    var totalRequests, totalErrors, totalInputTokens, totalCacheCreationTokens, totalCacheReadTokens, totalOutputTokens int

    for _, record := range archiveData {
        if endpoints[record.EndpointName] == nil {
            endpoints[record.EndpointName] = map[string]interface{}{
                "dailyHistory": make(map[string]interface{}),
            }
        }

        dailyHistory := endpoints[record.EndpointName]["dailyHistory"].(map[string]interface{})
        dailyHistory[record.Date] = map[string]interface{}{
            "date":                record.Date,
            "requests":            record.Requests,
            "errors":              record.Errors,
            "inputTokens":         record.InputTokens,
            "cacheCreationTokens": record.CacheCreationTokens,
            "cacheReadTokens":     record.CacheReadTokens,
            "outputTokens":        record.OutputTokens,
        }

        totalRequests += record.Requests
        totalErrors += record.Errors
        totalInputTokens += record.InputTokens
        totalCacheCreationTokens += record.CacheCreationTokens
        totalCacheReadTokens += record.CacheReadTokens
        totalOutputTokens += record.OutputTokens
    }

    summary := map[string]interface{}{
        "totalRequests":            totalRequests,
        "totalErrors":              totalErrors,
        "totalInputTokens":         totalInputTokens,
        "totalCacheCreationTokens": totalCacheCreationTokens,
        "totalCacheReadTokens":     totalCacheReadTokens,
        "totalOutputTokens":        totalOutputTokens,
    }

    archive := map[string]interface{}{
        "endpoints": endpoints,
        "summary":   summary,
    }

    return successJSON(map[string]interface{}{
        "archive": archive,
    })
}

// GetArchiveTrend returns trend comparison between selected month and previous month
func (a *ArchiveService) GetArchiveTrend(month string) string {
    if a.storage == nil {
        return errorJSON("Storage not initialized")
    }

    t, err := time.Parse("2006-01", month)
    if err != nil {
        return errorJSON(fmt.Sprintf("Invalid month format: %v", err))
    }

    previousMonth := t.AddDate(0, -1, 0).Format("2006-01")

    currentData, err := a.storage.GetMonthlyArchiveData(month)
    if err != nil {
        logger.Error("Failed to get current month data: %v", err)
        return errorJSON(fmt.Sprintf("Failed to load current month: %v", err))
    }

    previousData, err := a.storage.GetMonthlyArchiveData(previousMonth)
    if err != nil {
        logger.Debug("Previous month %s has no data, returning flat trend", previousMonth)
        return successJSON(map[string]interface{}{
            "trend":       0.0,
            "errorsTrend": 0.0,
            "tokensTrend": 0.0,
        })
    }

    var currentRequests, currentErrors, currentTokens int
    for _, record := range currentData {
        currentRequests += record.Requests
        currentErrors += record.Errors
        // Include cache tokens in total
        currentTokens += record.InputTokens + record.CacheCreationTokens + record.CacheReadTokens + record.OutputTokens
    }

    var previousRequests, previousErrors, previousTokens int
    for _, record := range previousData {
        previousRequests += record.Requests
        previousErrors += record.Errors
        // Include cache tokens in total
        previousTokens += record.InputTokens + record.CacheCreationTokens + record.CacheReadTokens + record.OutputTokens
    }

    requestsTrend := calculateTrend(currentRequests, previousRequests)
    errorsTrend := calculateTrend(currentErrors, previousErrors)
    tokensTrend := calculateTrend(currentTokens, previousTokens)

    return successJSON(map[string]interface{}{
        "trend":       requestsTrend,
        "errorsTrend": errorsTrend,
        "tokensTrend": tokensTrend,
    })
}

// GenerateMockArchives is deprecated - returns error message
func (a *ArchiveService) GenerateMockArchives(monthsCount int) string {
    return errorJSON("Mock archives are no longer supported. Use real data from SQLite.")
}

// DeleteArchive deletes all data for a specific month
func (a *ArchiveService) DeleteArchive(month string) string {
    if a.storage == nil {
        return errorJSON("Storage not initialized")
    }

    err := a.storage.DeleteMonthlyStats(month)
    if err != nil {
        logger.Error("Failed to delete archive for %s: %v", month, err)
        return errorJSON(fmt.Sprintf("Failed to delete archive: %v", err))
    }

    logger.Info("Archive deleted for month: %s", month)
    return successJSON(map[string]interface{}{
        "message": fmt.Sprintf("Archive for %s deleted successfully", month),
    })
}
