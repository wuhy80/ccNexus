package service

import (
	"encoding/json"
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
	data, _ := json.Marshal(map[string]interface{}{
		"totalRequests": totalRequests,
		"endpoints":     endpointStats,
	})
	return string(data)
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

	data, _ := json.Marshal(result)
	return string(data)
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

	result := map[string]interface{}{
		"current":        current.requests,
		"previous":       prev.requests,
		"trend":          calculateTrend(current.requests, prev.requests),
		"currentErrors":  current.errors,
		"previousErrors": prev.errors,
		"errorsTrend":    calculateTrend(current.errors, prev.errors),
		"currentTokens":  current.tokens,
		"previousTokens": prev.tokens,
		"tokensTrend":    calculateTrend(current.tokens, prev.tokens),
	}

	data, _ := json.Marshal(result)
	return string(data)
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
		sum.tokens += st.InputTokens + st.OutputTokens
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
		result := map[string]interface{}{
			"success":  false,
			"date":     date,
			"requests": []interface{}{},
			"total":    0,
			"message":  "Storage not initialized",
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	// Get total count
	total, err := s.storage.GetRequestStatsCount("", date, date)
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"date":    date,
			"message": "Failed to get request count: " + err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	// Get request stats with pagination
	requests, err := s.storage.GetRequestStats("", date, date, limit, offset)
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"date":    date,
			"message": "Failed to get request details: " + err.Error(),
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	result := map[string]interface{}{
		"success":  true,
		"date":     date,
		"requests": requests,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// GetTokenTrendData returns token usage trend data for charting
func (s *StatsService) GetTokenTrendData(granularity string, period string) string {
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
	requests, err := s.storage.GetRequestStats("", startDate, endDate, 10000, 0)
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
		result = s.aggregateBy5Minutes(requests, startDate, endDate, period)
	case "30min":
		result = s.aggregateBy30Minutes(requests, startDate, endDate, period)
	case "request":
		result = s.aggregateByRequest(requests, period)
	default:
		return jsonError("Invalid granularity: " + granularity)
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// aggregateBy5Minutes aggregates requests into 5-minute time slots (288 slots per day)
func (s *StatsService) aggregateBy5Minutes(requests []storage.RequestStat, startDate, endDate, period string) map[string]interface{} {
	timeSlots := generateTimeSlots(5)
	endpointData := make(map[string]map[string][]int)
	totalInput := make([]int, len(timeSlots))
	totalOutput := make([]int, len(timeSlots))

	for _, req := range requests {
		slotIndex := getTimeSlotIndex(req.Timestamp, 5)
		if slotIndex < 0 || slotIndex >= len(timeSlots) {
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

		endpointData[req.EndpointName]["inputTokens"][slotIndex] += inputTotal
		endpointData[req.EndpointName]["outputTokens"][slotIndex] += req.OutputTokens
		totalInput[slotIndex] += inputTotal
		totalOutput[slotIndex] += req.OutputTokens
	}

	return map[string]interface{}{
		"success":     true,
		"granularity": "5min",
		"period":      period,
		"dateRange":   map[string]string{"start": startDate, "end": endDate},
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

// aggregateBy30Minutes aggregates requests into 30-minute time slots (48 slots per day)
func (s *StatsService) aggregateBy30Minutes(requests []storage.RequestStat, startDate, endDate, period string) map[string]interface{} {
	timeSlots := generateTimeSlots(30)
	endpointData := make(map[string]map[string][]int)
	totalInput := make([]int, len(timeSlots))
	totalOutput := make([]int, len(timeSlots))

	for _, req := range requests {
		slotIndex := getTimeSlotIndex(req.Timestamp, 30)
		if slotIndex < 0 || slotIndex >= len(timeSlots) {
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

		endpointData[req.EndpointName]["inputTokens"][slotIndex] += inputTotal
		endpointData[req.EndpointName]["outputTokens"][slotIndex] += req.OutputTokens
		totalInput[slotIndex] += inputTotal
		totalOutput[slotIndex] += req.OutputTokens
	}

	return map[string]interface{}{
		"success":     true,
		"granularity": "30min",
		"period":      period,
		"dateRange":   map[string]string{"start": startDate, "end": endDate},
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
func jsonError(message string) string {
	result := map[string]interface{}{
		"success": false,
		"message": message,
	}
	data, _ := json.Marshal(result)
	return string(data)
}
