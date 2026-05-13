package service

import (
	"context"
	"time"
)

type ModelMarketplaceMonitor struct {
	ID                  int64
	Name                string
	Provider            string
	Endpoint            string
	APIKey              string
	PrimaryModel        string
	ExtraModels         []string
	ModelDisplayNames   map[string]ModelMarketplaceModelDisplayName
	ModelCallConfigs    map[string]ModelMarketplaceModelCallConfig
	GroupName           string
	Enabled             bool
	IntervalSeconds     int
	LastCheckedAt       *time.Time
	CreatedBy           int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
	TemplateID          *int64
	ExtraHeaders        map[string]string
	BodyOverrideMode    string
	BodyOverride        map[string]any
	APIKeyDecryptFailed bool
}

type ModelMarketplaceModelDisplayName struct {
	Zh string `json:"zh,omitempty"`
	En string `json:"en,omitempty"`
}

type ModelMarketplaceModelCallConfig struct {
	Model      string `json:"model,omitempty"`
	RequestURL string `json:"request_url,omitempty"`
}

type ModelMarketplaceMonitorListParams struct {
	Page     int
	PageSize int
	Provider string
	Enabled  *bool
	Search   string
}

type ModelMarketplaceMonitorCreateParams struct {
	Name              string
	Provider          string
	Endpoint          string
	APIKey            string
	PrimaryModel      string
	ExtraModels       []string
	ModelDisplayNames map[string]ModelMarketplaceModelDisplayName
	ModelCallConfigs  map[string]ModelMarketplaceModelCallConfig
	GroupName         string
	Enabled           bool
	IntervalSeconds   int
	CreatedBy         int64
	TemplateID        *int64
	ExtraHeaders      map[string]string
	BodyOverrideMode  string
	BodyOverride      map[string]any
}

type ModelMarketplaceMonitorUpdateParams struct {
	Name              *string
	Provider          *string
	Endpoint          *string
	APIKey            *string
	PrimaryModel      *string
	ExtraModels       *[]string
	ModelDisplayNames *map[string]ModelMarketplaceModelDisplayName
	ModelCallConfigs  *map[string]ModelMarketplaceModelCallConfig
	GroupName         *string
	Enabled           *bool
	IntervalSeconds   *int
	TemplateID        *int64
	ClearTemplate     bool
	ExtraHeaders      *map[string]string
	BodyOverrideMode  *string
	BodyOverride      *map[string]any
}

type ModelMarketplaceCheckResult struct {
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
}

type ModelMarketplaceMonitorHistoryRow struct {
	MonitorID     int64
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
}

type ModelMarketplaceMonitorHistoryEntry struct {
	ID            int64
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
}

type ModelMarketplaceMonitorLatest struct {
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	CheckedAt     time.Time
}

type ModelMarketplaceMonitorAvailability struct {
	Model           string
	WindowDays      int
	AvailabilityPct float64
	AvgLatencyMs    *int
}

type ModelMarketplaceExtraModelStatus struct {
	Model          string
	DisplayNameZh  string
	DisplayNameEn  string
	CallModel      string
	RequestURL     string
	Status         string
	LatencyMs      *int
	PingLatencyMs  *int
	Availability7d float64
	Pricing        *ChannelModelPricing
	Timeline       []ModelMarketplaceUserTimelinePoint
}

type ModelMarketplaceMonitorStatusSummary struct {
	PrimaryStatus    string
	PrimaryLatencyMs *int
	Availability7d   float64
	ExtraModels      []ModelMarketplaceExtraModelStatus
}

type ModelMarketplaceUserMonitorView struct {
	ID                   int64
	Name                 string
	Provider             string
	GroupName            string
	PrimaryModel         string
	PrimaryDisplayNameZh string
	PrimaryDisplayNameEn string
	PrimaryCallModel     string
	PrimaryRequestURL    string
	PrimaryStatus        string
	PrimaryLatencyMs     *int
	PrimaryPingLatencyMs *int
	Availability7d       float64
	PrimaryPricing       *ChannelModelPricing
	ExtraModels          []ModelMarketplaceExtraModelStatus
	Timeline             []ModelMarketplaceUserTimelinePoint
}

type ModelMarketplaceUserTimelinePoint struct {
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	CheckedAt     time.Time
}

type ModelMarketplaceUserMonitorDetail struct {
	ID        int64
	Name      string
	Provider  string
	GroupName string
	Models    []ModelMarketplaceModelDetail
}

type ModelMarketplaceModelDetail struct {
	Model           string
	DisplayNameZh   string
	DisplayNameEn   string
	CallModel       string
	RequestURL      string
	LatestStatus    string
	LatestLatencyMs *int
	Availability7d  float64
	Availability15d float64
	Availability30d float64
	AvgLatency7dMs  *int
	Pricing         *ChannelModelPricing
}

type ModelMarketplaceMonitorRepository interface {
	Create(ctx context.Context, m *ModelMarketplaceMonitor) error
	GetByID(ctx context.Context, id int64) (*ModelMarketplaceMonitor, error)
	Update(ctx context.Context, m *ModelMarketplaceMonitor) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params ModelMarketplaceMonitorListParams) ([]*ModelMarketplaceMonitor, int64, error)
	ListEnabled(ctx context.Context) ([]*ModelMarketplaceMonitor, error)
	MarkChecked(ctx context.Context, id int64, checkedAt time.Time) error
	InsertHistoryBatch(ctx context.Context, rows []*ModelMarketplaceMonitorHistoryRow) error
	DeleteHistoryBefore(ctx context.Context, before time.Time) (int64, error)
	ListHistory(ctx context.Context, monitorID int64, model string, limit int) ([]*ModelMarketplaceMonitorHistoryEntry, error)
	ListLatestPerModel(ctx context.Context, monitorID int64) ([]*ModelMarketplaceMonitorLatest, error)
	ComputeAvailability(ctx context.Context, monitorID int64, windowDays int) ([]*ModelMarketplaceMonitorAvailability, error)
	ListLatestForMonitorIDs(ctx context.Context, ids []int64) (map[int64][]*ModelMarketplaceMonitorLatest, error)
	ComputeAvailabilityForMonitors(ctx context.Context, ids []int64, windowDays int) (map[int64][]*ModelMarketplaceMonitorAvailability, error)
	ListRecentHistoryForMonitors(ctx context.Context, ids []int64, primaryModels map[int64]string, perMonitorLimit int) (map[int64][]*ModelMarketplaceMonitorHistoryEntry, error)
	ListRecentHistoryForMonitorModels(ctx context.Context, modelsByID map[int64][]string, perModelLimit int) (map[int64]map[string][]*ModelMarketplaceMonitorHistoryEntry, error)
}
