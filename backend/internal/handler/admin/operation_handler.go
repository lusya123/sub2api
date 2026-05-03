package admin

import (
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type OperationHandler struct {
	analytics *service.OperationAnalyticsService
}

func NewOperationHandler(analytics *service.OperationAnalyticsService) *OperationHandler {
	return &OperationHandler{analytics: analytics}
}

// GetSnapshot returns the read-only business operation dashboard snapshot.
// GET /api/v1/admin/operations/snapshot
func (h *OperationHandler) GetSnapshot(c *gin.Context) {
	if h.analytics == nil {
		response.InternalError(c, "Operation analytics service not available")
		return
	}

	filter, err := parseOperationAnalyticsFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	snapshot, err := h.analytics.GetSnapshot(c.Request.Context(), filter)
	if err != nil {
		slog.Error("failed to get operation analytics snapshot", "error", err)
		response.InternalError(c, "Failed to get operation analytics snapshot")
		return
	}
	response.Success(c, snapshot)
}

func parseOperationAnalyticsFilter(c *gin.Context) (service.OperationAnalyticsFilter, error) {
	userTZ := strings.TrimSpace(c.DefaultQuery("timezone", "Asia/Shanghai"))
	granularity := strings.TrimSpace(c.DefaultQuery("granularity", "day"))
	if granularity != "day" && granularity != "hour" {
		return service.OperationAnalyticsFilter{}, errInvalidOperationGranularity
	}
	if _, err := time.LoadLocation(userTZ); err != nil {
		return service.OperationAnalyticsFilter{}, errInvalidOperationTimezone
	}

	now := timezone.NowInUserLocation(userTZ)
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	rangeMode := strings.TrimSpace(c.Query("range"))
	modules, err := parseOperationAnalyticsModules(c.DefaultQuery("modules", "summary"))
	if err != nil {
		return service.OperationAnalyticsFilter{}, err
	}

	var startTime time.Time
	var endTime time.Time

	if rangeMode != "" && rangeMode != "all" {
		return service.OperationAnalyticsFilter{}, errInvalidOperationRangeMode
	}

	if rangeMode == "all" {
		loc, _ := time.LoadLocation(userTZ)
		startTime = time.Date(1970, 1, 1, 0, 0, 0, 0, loc)
		endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
		granularity = "day"
		return service.OperationAnalyticsFilter{
			StartTime:   startTime,
			EndTime:     endTime,
			Granularity: granularity,
			Timezone:    userTZ,
			Modules:     restrictAllDataOperationModules(modules),
			AllData:     true,
		}, nil
	}

	if startDate == "" {
		startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -13), userTZ)
	} else {
		startTime, err = timezone.ParseInUserLocation("2006-01-02", startDate, userTZ)
		if err != nil {
			return service.OperationAnalyticsFilter{}, errInvalidOperationStartDate
		}
	}
	if endDate == "" {
		endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	} else {
		endTime, err = timezone.ParseInUserLocation("2006-01-02", endDate, userTZ)
		if err != nil {
			return service.OperationAnalyticsFilter{}, errInvalidOperationEndDate
		}
		endTime = endTime.Add(24 * time.Hour)
	}
	if !endTime.After(startTime) {
		return service.OperationAnalyticsFilter{}, errInvalidOperationRange
	}
	if endTime.Sub(startTime) > maxOperationDashboardRange {
		return service.OperationAnalyticsFilter{}, errOperationRangeTooLarge
	}

	return service.OperationAnalyticsFilter{
		StartTime:   startTime,
		EndTime:     endTime,
		Granularity: granularity,
		Timezone:    userTZ,
		Modules:     modules,
	}, nil
}

func parseOperationAnalyticsModules(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "summary" {
		return []string{"core", "trend"}, nil
	}
	if raw == "all" {
		return []string{
			"core", "trend", "baselines", "funnel", "trial", "lists",
			"cohorts", "distribution", "churn", "pyramid", "financial", "product_matrix",
		}, nil
	}

	allowed := map[string]bool{
		"core": true, "trend": true, "baselines": true, "funnel": true, "trial": true, "lists": true,
		"cohorts": true, "distribution": true, "churn": true, "pyramid": true, "financial": true, "product_matrix": true,
	}
	seen := make(map[string]bool)
	modules := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		module := strings.TrimSpace(part)
		if module == "" {
			continue
		}
		if !allowed[module] {
			return nil, errInvalidOperationModules
		}
		if !seen[module] {
			seen[module] = true
			modules = append(modules, module)
		}
	}
	if len(modules) == 0 {
		return []string{"core", "trend"}, nil
	}
	return modules, nil
}

func restrictAllDataOperationModules(modules []string) []string {
	allowed := map[string]bool{"core": true}
	restricted := make([]string, 0, len(modules))
	for _, module := range modules {
		if allowed[module] {
			restricted = append(restricted, module)
		}
	}
	if len(restricted) == 0 {
		return []string{"core"}
	}
	return restricted
}

type operationParseError string

func (e operationParseError) Error() string { return string(e) }

const (
	errInvalidOperationGranularity operationParseError = "Invalid granularity, use day or hour"
	errInvalidOperationStartDate   operationParseError = "Invalid start_date, use YYYY-MM-DD"
	errInvalidOperationEndDate     operationParseError = "Invalid end_date, use YYYY-MM-DD"
	errInvalidOperationTimezone    operationParseError = "Invalid timezone"
	errInvalidOperationRange       operationParseError = "Invalid date range"
	errInvalidOperationRangeMode   operationParseError = "Invalid range, use all or omit it"
	errInvalidOperationModules     operationParseError = "Invalid modules"
	errOperationRangeTooLarge      operationParseError = "Date range is too large, maximum is 370 days"
)

const maxOperationDashboardRange = 370 * 24 * time.Hour
