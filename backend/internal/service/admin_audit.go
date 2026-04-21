package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AdminAuditLog struct {
	ID              int64           `json:"id"`
	CreatedAt       time.Time       `json:"created_at"`
	ActorUserID     int64           `json:"actor_user_id"`
	ActorEmail      string          `json:"actor_email"`
	ActorRole       string          `json:"actor_role"`
	Method          string          `json:"method"`
	RouteTemplate   string          `json:"route_template"`
	Path            string          `json:"path"`
	Module          string          `json:"module"`
	Action          string          `json:"action"`
	ActionType      string          `json:"action_type"`
	TargetType      string          `json:"target_type"`
	TargetID        *int64          `json:"target_id,omitempty"`
	StatusCode      int             `json:"status_code"`
	Success         bool            `json:"success"`
	ErrorCode       string          `json:"error_code,omitempty"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	IPAddress       string          `json:"ip_address"`
	UserAgent       string          `json:"user_agent"`
	Summary         string          `json:"summary"`
	QueryParamsJSON json.RawMessage `json:"query_params,omitempty"`
	RequestBodyJSON json.RawMessage `json:"request_body,omitempty"`
	DurationMS      int64           `json:"duration_ms"`
}

type AdminAuditLogInput struct {
	CreatedAt       time.Time
	ActorUserID     int64
	ActorEmail      string
	ActorRole       string
	Method          string
	RouteTemplate   string
	Path            string
	Module          string
	Action          string
	ActionType      string
	TargetType      string
	TargetID        *int64
	StatusCode      int
	Success         bool
	ErrorCode       string
	ErrorMessage    string
	IPAddress       string
	UserAgent       string
	Summary         string
	QueryParamsJSON string
	RequestBodyJSON string
	DurationMS      int64
}

type AdminAuditLogFilter struct {
	Page                  int
	PageSize              int
	StartTime             *time.Time
	EndTime               *time.Time
	ActorUserID           *int64
	ActorRole             string
	Module                string
	ActionType            string
	ExcludeActionType     string
	ExcludeSuccessfulRead bool
	TargetType            string
	TargetID              *int64
	Success               *bool
	StatusCode            *int
	Query                 string
	Route                 string
	Method                string
}

type AdminAuditLogList struct {
	Logs     []AdminAuditLog
	Total    int64
	Page     int
	PageSize int
}

var ErrAdminAuditLogNotFound = infraerrors.NotFound("ADMIN_AUDIT_LOG_NOT_FOUND", "audit log not found")

type AdminAuditRepository interface {
	Insert(ctx context.Context, input *AdminAuditLogInput) error
	List(ctx context.Context, filter *AdminAuditLogFilter) (*AdminAuditLogList, error)
	GetByID(ctx context.Context, id int64) (*AdminAuditLog, error)
}

type AdminAuditService struct {
	repo AdminAuditRepository
}

func NewAdminAuditService(repo AdminAuditRepository) *AdminAuditService {
	return &AdminAuditService{repo: repo}
}

func (s *AdminAuditService) Record(ctx context.Context, input *AdminAuditLogInput) {
	if s == nil || s.repo == nil || input == nil {
		return
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	input.QueryParamsJSON = normalizeAuditJSON(input.QueryParamsJSON)
	input.RequestBodyJSON = normalizeAuditJSON(input.RequestBodyJSON)
	_ = s.repo.Insert(ctx, input)
}

func (s *AdminAuditService) List(ctx context.Context, filter *AdminAuditLogFilter) (*AdminAuditLogList, error) {
	if s == nil || s.repo == nil {
		return &AdminAuditLogList{Logs: []AdminAuditLog{}, Page: 1, PageSize: 20}, nil
	}
	if filter == nil {
		filter = &AdminAuditLogFilter{}
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	normalizeAuditFilter(filter)
	return s.repo.List(ctx, filter)
}

func (s *AdminAuditService) GetByID(ctx context.Context, id int64) (*AdminAuditLog, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAdminAuditLogNotFound
	}
	return s.repo.GetByID(ctx, id)
}

func normalizeAuditFilter(filter *AdminAuditLogFilter) {
	filter.ActorRole = strings.TrimSpace(filter.ActorRole)
	filter.Module = strings.TrimSpace(filter.Module)
	filter.ActionType = strings.TrimSpace(filter.ActionType)
	filter.ExcludeActionType = strings.TrimSpace(filter.ExcludeActionType)
	filter.TargetType = strings.TrimSpace(filter.TargetType)
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Route = strings.TrimSpace(filter.Route)
	filter.Method = strings.ToUpper(strings.TrimSpace(filter.Method))
}

func normalizeAuditJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	if json.Valid([]byte(raw)) {
		return raw
	}
	b, err := json.Marshal(map[string]string{"raw": raw})
	if err != nil {
		return "{}"
	}
	return string(b)
}
