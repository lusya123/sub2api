package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	auditService *service.AdminAuditService
}

func NewAuditHandler(auditService *service.AdminAuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

func (h *AuditHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := &service.AdminAuditLogFilter{
		Page:              page,
		PageSize:          pageSize,
		ActorRole:         c.Query("actor_role"),
		Module:            c.Query("module"),
		ActionType:        c.Query("action_type"),
		ExcludeActionType: c.Query("exclude_action_type"),
		TargetType:        c.Query("target_type"),
		Query:             c.Query("q"),
		Route:             c.Query("route"),
		Method:            c.Query("method"),
	}
	if start, ok := parseAuditTimeQuery(c.Query("start_time")); ok {
		filter.StartTime = &start
	}
	if end, ok := parseAuditTimeQuery(c.Query("end_time")); ok {
		filter.EndTime = &end
	}
	if v, ok := parseAuditInt64Query(c.Query("actor_user_id")); ok {
		filter.ActorUserID = &v
	}
	if v, ok := parseAuditInt64Query(c.Query("target_id")); ok {
		filter.TargetID = &v
	}
	if v, ok := parseAuditIntQuery(c.Query("status_code")); ok {
		filter.StatusCode = &v
	}
	if v, ok, invalid := parseAuditBoolQuery(c.Query("exclude_successful_read")); invalid {
		response.BadRequest(c, "Invalid exclude_successful_read")
		return
	} else if ok {
		filter.ExcludeSuccessfulRead = v
	}
	if raw := strings.TrimSpace(c.Query("success")); raw != "" {
		if v, ok, invalid := parseAuditBoolQuery(raw); invalid || !ok {
			response.BadRequest(c, "Invalid success")
			return
		} else {
			filter.Success = &v
		}
	}

	result, err := h.auditService.List(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result.Logs, result.Total, result.Page, result.PageSize)
}

func (h *AuditHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid audit log ID")
		return
	}
	item, err := h.auditService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func parseAuditTimeQuery(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func parseAuditInt64Query(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err == nil && v > 0
}

func parseAuditIntQuery(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	return v, err == nil
}

func parseAuditBoolQuery(raw string) (bool, bool, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return false, false, false
	}
	switch raw {
	case "true", "1":
		return true, true, false
	case "false", "0":
		return false, true, false
	default:
		return false, false, true
	}
}
