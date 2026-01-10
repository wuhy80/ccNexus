package service

import (
	"time"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/proxy"
	"github.com/lich0821/ccNexus/internal/storage"
)

// StatsService handles statistics operations
type StatsService struct {
	proxy   *proxy.Proxy
	config  *config.Config
	storage storage.Storage
}

// NewStatsService creates a new stats service
func NewStatsService(p *proxy.Proxy, cfg *config.Config) *StatsService {
	return &StatsService{proxy: p, config: cfg}
}

// SetStorage sets the storage for accessing request details
func (s *StatsService) SetStorage(st storage.Storage) {
	s.storage = st
}

// GetStats returns current statistics
func (s *StatsService) GetStats() string {
	totalRequests, endpointStats := s.proxy.GetStats().GetStats()
	return toJSON(map[string]interface{}{
		"totalRequests": totalRequests,
		"endpoints":     endpointStats,
	})
}

// GetStatsDaily returns statistics for today
func (s *StatsService) GetStatsDaily() string {
	return s.getPeriodStats("daily", time.Now().Format("2006-01-02"), time.Now().Format("2006-01-02"))
}

// GetStatsYesterday returns statistics for yesterday
func (s *StatsService) GetStatsYesterday() string {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	return s.getPeriodStats("yesterday", yesterday, yesterday)
}

// GetStatsWeekly returns statistics for this week
func (s *StatsService) GetStatsWeekly() string {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startDate := now.AddDate(0, 0, -(weekday - 1)).Format("2006-01-02")
	return s.getPeriodStats("weekly", startDate, now.Format("2006-01-02"))
}

// GetStatsMonthly returns statistics for this month
func (s *StatsService) GetStatsMonthly() string {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	return s.getPeriodStats("monthly", startDate, now.Format("2006-01-02"))
}

func (s *StatsService) getPeriodStats(period, startDate, endDate string) string {
	var stats map[string]*proxy.DailyStats
	if startDate == endDate {
		stats = s.proxy.GetStats().GetDailyStats(startDate)
	} else {
		stats = s.proxy.GetStats().GetPeriodStats(startDate, endDate)
	}

	var totalRequests, totalErrors int
	var totalInputTokens, totalCacheCreationTokens, totalCacheReadTokens, totalOutputTokens int
	for _, st := range stats {
		totalRequests += st.Requests
		totalErrors += st.Errors
		totalInputTokens += st.InputTokens
		totalCacheCreationTokens += st.CacheCreationTokens
		totalCacheReadTokens += st.CacheReadTokens
		totalOutputTokens += st.OutputTokens
	}

	activeEndpoints, totalEndpoints := s.countEndpoints()

	result := map[string]interface{}{
		"period":                   period,
		"totalRequests":            totalRequests,
		"totalErrors":              totalErrors,
		"totalSuccess":             totalRequests - totalErrors,
		"totalInputTokens":         totalInputTokens,
		"totalCacheCreationTokens": totalCacheCreationTokens,
		"totalCacheReadTokens":     totalCacheReadTokens,
		"totalOutputTokens":        totalOutputTokens,
		"activeEndpoints":          activeEndpoints,
		"totalEndpoints":           totalEndpoints,
		"endpoints":                stats,
	}
	if startDate == endDate {
		result["date"] = startDate
	} else {
		result["startDate"] = startDate
		result["endDate"] = endDate
	}

	return toJSON(result)
}

func (s *StatsService) countEndpoints() (active, total int) {
	endpoints := s.config.GetEndpoints()
	total = len(endpoints)
	for _, ep := range endpoints {
		if ep.Enabled {
			active++
		}
	}
	return
}

// GetStatsTrend returns trend comparison data
func (s *StatsService) GetStatsTrend() string {
	return s.GetStatsTrendByPeriod("daily")
}

// GetStatsTrendByPeriod returns trend comparison data for specified period
func (s *StatsService) GetStatsTrendByPeriod(period string) string {
	now := time.Now()
	var currentStart, currentEnd, prevStart, prevEnd string

	switch period {
	case "yesterday":
		currentStart = now.AddDate(0, 0, -1).Format("2006-01-02")
		currentEnd = currentStart
		prevStart = now.AddDate(0, 0, -2).Format("2006-01-02")
		prevEnd = prevStart
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		thisWeekStart := now.AddDate(0, 0, -(weekday - 1))
		currentStart = thisWeekStart.Format("2006-01-02")
		currentEnd = now.Format("2006-01-02")
		prevStart = thisWeekStart.AddDate(0, 0, -7).Format("2006-01-02")
		prevEnd = thisWeekStart.AddDate(0, 0, -1).Format("2006-01-02")
	case "monthly":
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		currentStart = thisMonthStart.Format("2006-01-02")
		currentEnd = now.Format("2006-01-02")
		lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
		prevStart = lastMonthStart.Format("2006-01-02")
		prevEnd = thisMonthStart.AddDate(0, 0, -1).Format("2006-01-02")
	default: // daily
		currentStart = now.Format("2006-01-02")
		currentEnd = currentStart
		prevStart = now.AddDate(0, 0, -1).Format("2006-01-02")
		prevEnd = prevStart
	}

	current := s.sumStats(currentStart, currentEnd)
	prev := s.sumStats(prevStart, prevEnd)

	return toJSON(map[string]interface{}{
		"current":        current.requests,
		"previous":       prev.requests,
		"trend":          calculateTrend(current.requests, prev.requests),
		"currentErrors":  current.errors,
		"previousErrors": prev.errors,
		"errorsTrend":    calculateTrend(current.errors, prev.errors),
		"currentTokens":  current.tokens,
		"previousTokens": prev.tokens,
		"tokensTrend":    calculateTrend(current.tokens, prev.tokens),
	})
}

type statsSummary struct {
	requests, errors, tokens int
}

func (s *StatsService) sumStats(startDate, endDate string) statsSummary {
	var stats map[string]*proxy.DailyStats
	if startDate == endDate {
		stats = s.proxy.GetStats().GetDailyStats(startDate)
	} else {
		stats = s.proxy.GetStats().GetPeriodStats(startDate, endDate)
	}

	var sum statsSummary
	for _, st := range stats {
		sum.requests += st.Requests
		sum.errors += st.Errors
		// Include cache tokens in total (cache_creation + cache_read are part of input)
		sum.tokens += st.InputTokens + st.CacheCreationTokens + st.CacheReadTokens + st.OutputTokens
	}
	return sum
}

func calculateTrend(current, previous int) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100.0
	}
	trend := ((float64(current) - float64(previous)) / float64(previous)) * 100.0
	if trend > 100.0 {
		return 100.0
	}
	if trend < -100.0 {
		return -100.0
	}
	return trend
}

// GetDailyRequestDetails returns detailed request-level statistics for today with pagination
func (s *StatsService) GetDailyRequestDetails(limit, offset int) string {
	today := time.Now().Format("2006-01-02")
	return s.getRequestDetailsByDate(today, limit, offset)
}

// getRequestDetailsByDate returns detailed request-level statistics for a specific date
func (s *StatsService) getRequestDetailsByDate(date string, limit, offset int) string {
	if s.storage == nil {
		return toJSON(map[string]interface{}{
			"success":  false,
			"date":     date,
			"requests": []interface{}{},
			"total":    0,
			"message":  "Storage not initialized",
		})
	}

	// Get total count
	total, err := s.storage.GetRequestStatsCount("", "", date, date)
	if err != nil {
		return toJSON(map[string]interface{}{
			"success": false,
			"date":    date,
			"message": "Failed to get request count: " + err.Error(),
		})
	}

	// Get request stats with pagination
	requests, err := s.storage.GetRequestStats("", "", date, date, limit, offset)
	if err != nil {
		return toJSON(map[string]interface{}{
			"success": false,
			"date":    date,
			"message": "Failed to get request details: " + err.Error(),
		})
	}

	// Calculate performance metrics from all requests (not just paginated ones)
	allRequests, _ := s.storage.GetRequestStats("", "", date, date, 10000, 0)
	metrics := calculatePerformanceMetrics(allRequests)

	return successJSON(map[string]interface{}{
		"date":     date,
		"requests": requests,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"metrics":  metrics,
	})
}

// GetTokenTrendData returns token usage trend data for charting
// startTime and endTime are optional time filters in "HH:MM" format (empty string means auto)
func (s *StatsService) GetTokenTrendData(granularity, period, startTime, endTime string) string {
	if s.storage == nil {
		return jsonError("Storage not initialized")
	}

	// Calculate date range based on period
	var startDate, endDate string
	now := time.Now()

	switch period {
	case "yesterday":
		startDate = now.AddDate(0, 0, -1).Format("2006-01-02")
		endDate = startDate
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startDate = now.AddDate(0, 0, -(weekday - 1)).Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	case "monthly":
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	default: // daily
		startDate = now.Format("2006-01-02")
		endDate = startDate
	}

	// Fetch request stats from storage
	requests, err := s.storage.GetRequestStats("", "", startDate, endDate, 10000, 0)
	if err != nil {
		return jsonError("Failed to get request stats: " + err.Error())
	}

	// Validate granularity for multi-day periods
	// 5min and 30min granularity only make sense for single-day views
	if (period == "weekly" || period == "monthly") && (granularity == "5min" || granularity == "30min") {
		return jsonError("Time-based granularity (5min/30min) is not supported for multi-day periods. Use 'request' granularity instead.")
	}

	// Aggregate data based on granularity
	var result map[string]interface{}
	switch granularity {
	case "5min":
		result = s.aggregateByMinutes(requests, startDate, endDate, period, 5, startTime, endTime)
	case "30min":
		result = s.aggregateByMinutes(requests, startDate, endDate, period, 30, startTime, endTime)
	case "request":
		result = s.aggregateByRequest(requests, period)
	default:
		return jsonError("Invalid granularity: " + granularity)
	}

	return toJSON(result)
}

// aggregateByMinutes aggregates requests into time slots with smart time range compression
// intervalMinutes: 5 or 30
// startTime/endTime: optional "HH:MM" format, empty means auto-calculate
func (s *StatsService) aggregateByMinutes(requests []storage.RequestStat, startDate, endDate, period string, intervalMinutes int, startTime, endTime string) map[string]interface{} {
	// Find first and last request times for auto range calculation
	var firstRequestTime, lastRequestTime string
	if len(requests) > 0 {
		// Requests are in DESC order, so last element is earliest
		firstReq := requests[len(requests)-1]
		lastReq := requests[0]
		firstRequestTime = firstReq.Timestamp.Format("15:04")
		lastRequestTime = lastReq.Timestamp.Format("15:04")
	}

	// Calculate effective time range
	effectiveStart, effectiveEnd := calculateEffectiveTimeRange(
		startTime, endTime, firstRequestTime, lastRequestTime, intervalMinutes,
	)

	// Generate time slots only for the effective range
	timeSlots := generateTimeSlotsInRange(effectiveStart, effectiveEnd, intervalMinutes)
	startSlotIndex := getTimeSlotIndexFromString(effectiveStart, intervalMinutes)

	endpointData := make(map[string]map[string][]int)
	totalInput := make([]int, len(timeSlots))
	totalOutput := make([]int, len(timeSlots))

	for _, req := range requests {
		absoluteSlotIndex := getTimeSlotIndex(req.Timestamp, intervalMinutes)
		// Convert to relative index within our range
		relativeSlotIndex := absoluteSlotIndex - startSlotIndex
		if relativeSlotIndex < 0 || relativeSlotIndex >= len(timeSlots) {
			continue
		}

		// Initialize endpoint data if not exists
		if _, exists := endpointData[req.EndpointName]; !exists {
			endpointData[req.EndpointName] = map[string][]int{
				"inputTokens":  make([]int, len(timeSlots)),
				"outputTokens": make([]int, len(timeSlots)),
			}
		}

		// Merge cache tokens into input
		inputTotal := req.InputTokens + req.CacheCreationTokens + req.CacheReadTokens

		endpointData[req.EndpointName]["inputTokens"][relativeSlotIndex] += inputTotal
		endpointData[req.EndpointName]["outputTokens"][relativeSlotIndex] += req.OutputTokens
		totalInput[relativeSlotIndex] += inputTotal
		totalOutput[relativeSlotIndex] += req.OutputTokens
	}

	granularityStr := "5min"
	if intervalMinutes == 30 {
		granularityStr = "30min"
	}

	return map[string]interface{}{
		"success":     true,
		"granularity": granularityStr,
		"period":      period,
		"dateRange":   map[string]string{"start": startDate, "end": endDate},
		"dataRange": map[string]string{
			"firstRequest":   firstRequestTime,
			"lastRequest":    lastRequestTime,
			"effectiveStart": effectiveStart,
			"effectiveEnd":   effectiveEnd,
		},
		"data": map[string]interface{}{
			"timestamps": timeSlots,
			"endpoints":  endpointData,
			"total": map[string][]int{
				"inputTokens":  totalInput,
				"outputTokens": totalOutput,
			},
		},
	}
}

// calculateEffectiveTimeRange calculates the effective time range for display
// If startTime/endTime are provided, use them; otherwise auto-calculate based on data
// Default range is 9:00~18:00, expanded by hour if data exists outside this range
func calculateEffectiveTimeRange(startTime, endTime, firstRequest, lastRequest string, intervalMinutes int) (string, string) {
	// Default time range: 9:00 ~ 18:00
	defaultStart := 9 * 60   // 9:00 in minutes
	defaultEnd := 18 * 60    // 18:00 in minutes

	// If custom time range is specified, use it directly
	if startTime != "" && endTime != "" {
		return startTime, endTime
	}

	// If no data, return default range
	if firstRequest == "" || lastRequest == "" {
		if startTime != "" {
			return startTime, minutesToTimeString(defaultEnd)
		}
		if endTime != "" {
			return minutesToTimeString(defaultStart), endTime
		}
		return minutesToTimeString(defaultStart), minutesToTimeString(defaultEnd)
	}

	var effectiveStart, effectiveEnd int

	// Calculate effective start
	if startTime == "" {
		firstMinutes := parseTimeToMinutes(firstRequest)
		// Start with default, expand by hour if data is earlier
		effectiveStart = defaultStart
		if firstMinutes < defaultStart {
			// Round down to hour boundary
			effectiveStart = (firstMinutes / 60) * 60
		}
	} else {
		effectiveStart = parseTimeToMinutes(startTime)
	}

	// Calculate effective end
	if endTime == "" {
		lastMinutes := parseTimeToMinutes(lastRequest)
		// Start with default, expand by hour if data is later
		effectiveEnd = defaultEnd
		if lastMinutes >= defaultEnd {
			// Round up to next hour boundary
			effectiveEnd = ((lastMinutes / 60) + 1) * 60
			if effectiveEnd > 24*60 {
				effectiveEnd = 24 * 60
			}
		}
	} else {
		effectiveEnd = parseTimeToMinutes(endTime)
	}

	return minutesToTimeString(effectiveStart), minutesToTimeString(effectiveEnd)
}

// generateTimeSlotsInRange generates time slot labels for a given time range
func generateTimeSlotsInRange(startTime, endTime string, intervalMinutes int) []string {
	startMinutes := parseTimeToMinutes(startTime)
	endMinutes := parseTimeToMinutes(endTime)

	if endMinutes <= startMinutes {
		endMinutes = startMinutes + intervalMinutes
	}

	numSlots := (endMinutes - startMinutes) / intervalMinutes
	slots := make([]string, numSlots)
	for i := 0; i < numSlots; i++ {
		totalMinutes := startMinutes + i*intervalMinutes
		hour := totalMinutes / 60
		minute := totalMinutes % 60
		slots[i] = time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC).Format("15:04")
	}

	return slots
}

// parseTimeToMinutes converts "HH:MM" to total minutes
func parseTimeToMinutes(timeStr string) int {
	if timeStr == "" {
		return 0
	}
	if timeStr == "24:00" {
		return 24 * 60
	}
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return 0
	}
	return t.Hour()*60 + t.Minute()
}

// minutesToTimeString converts total minutes to "HH:MM" format
func minutesToTimeString(minutes int) string {
	if minutes >= 24*60 {
		return "24:00"
	}
	hour := minutes / 60
	minute := minutes % 60
	return time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC).Format("15:04")
}

// getTimeSlotIndexFromString calculates slot index from "HH:MM" string
func getTimeSlotIndexFromString(timeStr string, intervalMinutes int) int {
	minutes := parseTimeToMinutes(timeStr)
	return minutes / intervalMinutes
}

// aggregateByRequest aggregates by individual requests (max 200 points)
func (s *StatsService) aggregateByRequest(requests []storage.RequestStat, period string) map[string]interface{} {
	maxRequests := 200
	if len(requests) > maxRequests {
		requests = requests[:maxRequests]
	}

	// Reverse the requests slice to get chronological order (database returns DESC)
	// This ensures the chart displays oldest to newest from left to right
	for i, j := 0, len(requests)-1; i < j; i, j = i+1, j-1 {
		requests[i], requests[j] = requests[j], requests[i]
	}

	numRequests := len(requests)
	timestamps := make([]string, numRequests)

	// First pass: collect all unique endpoint names
	endpointNames := make(map[string]bool)
	for _, req := range requests {
		endpointNames[req.EndpointName] = true
	}

	// Initialize data structures with correct length for all endpoints
	endpointData := make(map[string]map[string][]int)
	for name := range endpointNames {
		endpointData[name] = map[string][]int{
			"inputTokens":  make([]int, numRequests),
			"outputTokens": make([]int, numRequests),
		}
	}

	totalInput := make([]int, numRequests)
	totalOutput := make([]int, numRequests)

	// Second pass: fill in the data
	for i, req := range requests {
		timestamps[i] = req.Timestamp.Format("15:04:05")

		// Merge cache tokens into input
		inputTotal := req.InputTokens + req.CacheCreationTokens + req.CacheReadTokens

		// Only the current endpoint has data, others remain 0
		endpointData[req.EndpointName]["inputTokens"][i] = inputTotal
		endpointData[req.EndpointName]["outputTokens"][i] = req.OutputTokens

		totalInput[i] = inputTotal
		totalOutput[i] = req.OutputTokens
	}

	return map[string]interface{}{
		"success":     true,
		"granularity": "request",
		"period":      period,
		"data": map[string]interface{}{
			"timestamps": timestamps,
			"endpoints":  endpointData,
			"total": map[string][]int{
				"inputTokens":  totalInput,
				"outputTokens": totalOutput,
			},
		},
	}
}

// generateTimeSlots generates time slot labels for a given interval in minutes
func generateTimeSlots(intervalMinutes int) []string {
	slotsPerDay := (24 * 60) / intervalMinutes
	slots := make([]string, slotsPerDay)
	for i := 0; i < slotsPerDay; i++ {
		totalMinutes := i * intervalMinutes
		hour := totalMinutes / 60
		minute := totalMinutes % 60
		slots[i] = time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC).Format("15:04")
	}
	return slots
}

// getTimeSlotIndex calculates which time slot a timestamp belongs to
func getTimeSlotIndex(timestamp time.Time, intervalMinutes int) int {
	hour := timestamp.Hour()
	minute := timestamp.Minute()
	totalMinutes := hour*60 + minute
	return totalMinutes / intervalMinutes
}

// jsonError returns a JSON error response
// Deprecated: use errorJSON from json_helper.go instead
func jsonError(message string) string {
	return errorJSON(message)
}

// calculateTokensPerSecond calculates tokens per second from duration
func calculateTokensPerSecond(tokens int, durationMs int64) float64 {
	if durationMs <= 0 {
		return 0
	}
	durationSec := float64(durationMs) / 1000.0
	return float64(tokens) / durationSec
}

// calculatePerformanceMetrics calculates performance metrics from request stats
// Includes all requests (both successful and failed) with non-zero duration
func calculatePerformanceMetrics(requests []storage.RequestStat) map[string]interface{} {
	var totalOutputTokens, totalTokens int
	var totalDurationMs int64
	validCount := 0

	for _, req := range requests {
		if req.DurationMs > 0 { // Only filter out zero duration (old records from before migration)
			inputTotal := req.InputTokens + req.CacheCreationTokens + req.CacheReadTokens
			totalOutputTokens += req.OutputTokens
			totalTokens += inputTotal + req.OutputTokens
			totalDurationMs += req.DurationMs
			validCount++
		}
	}

	if validCount == 0 || totalDurationMs == 0 {
		return map[string]interface{}{
			"outputTokensPerSec": 0.0,
			"totalTokensPerSec":  0.0,
			"avgDurationMs":      0.0,
			"validRequests":      0,
		}
	}

	durationSec := float64(totalDurationMs) / 1000.0
	avgDurationMs := float64(totalDurationMs) / float64(validCount)

	return map[string]interface{}{
		"outputTokensPerSec": float64(totalOutputTokens) / durationSec,
		"totalTokensPerSec":  float64(totalTokens) / durationSec,
		"avgDurationMs":      avgDurationMs,
		"validRequests":      validCount,
	}
}

// GetPerformanceStats returns performance metrics for a time period
func (s *StatsService) GetPerformanceStats(period string) string {
	if s.storage == nil {
		return jsonError("Storage not initialized")
	}

	// Calculate date range based on period
	var startDate, endDate string
	now := time.Now()

	switch period {
	case "yesterday":
		startDate = now.AddDate(0, 0, -1).Format("2006-01-02")
		endDate = startDate
	case "weekly":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startDate = now.AddDate(0, 0, -(weekday - 1)).Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	case "monthly":
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	default: // daily
		startDate = now.Format("2006-01-02")
		endDate = startDate
	}

	// Fetch all requests for the period
	requests, err := s.storage.GetRequestStats("", "", startDate, endDate, 10000, 0)
	if err != nil {
		return jsonError("Failed to get request stats: " + err.Error())
	}

	// Calculate overall metrics
	overallMetrics := calculatePerformanceMetrics(requests)

	// Group requests by endpoint
	endpointRequests := make(map[string][]storage.RequestStat)
	for _, req := range requests {
		key := req.ClientType + ":" + req.EndpointName
		endpointRequests[key] = append(endpointRequests[key], req)
	}

	// Calculate per-endpoint metrics
	endpointMetrics := make(map[string]map[string]interface{})
	for key, reqs := range endpointRequests {
		endpointMetrics[key] = calculatePerformanceMetrics(reqs)
	}

	return successJSON(map[string]interface{}{
		"period":          period,
		"dateRange":       map[string]string{"start": startDate, "end": endDate},
		"overallMetrics":  overallMetrics,
		"endpointMetrics": endpointMetrics,
	})
}
