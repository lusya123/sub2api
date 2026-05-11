package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type ModelMarketplaceTemplate struct {
	ID               int64
	Name             string
	Provider         string
	Description      string
	ExtraHeaders     map[string]string
	BodyOverrideMode string
	BodyOverride     map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ModelMarketplaceTemplateListParams struct {
	Provider string
}

type ModelMarketplaceTemplateCreateParams struct {
	Name             string
	Provider         string
	Description      string
	ExtraHeaders     map[string]string
	BodyOverrideMode string
	BodyOverride     map[string]any
}

type ModelMarketplaceTemplateUpdateParams struct {
	Name             *string
	Description      *string
	ExtraHeaders     *map[string]string
	BodyOverrideMode *string
	BodyOverride     *map[string]any
}

type ModelMarketplaceAssociatedMonitorBrief struct {
	ID       int64
	Name     string
	Provider string
	Enabled  bool
}

type ModelMarketplaceTemplateRepository interface {
	Create(ctx context.Context, t *ModelMarketplaceTemplate) error
	GetByID(ctx context.Context, id int64) (*ModelMarketplaceTemplate, error)
	Update(ctx context.Context, t *ModelMarketplaceTemplate) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params ModelMarketplaceTemplateListParams) ([]*ModelMarketplaceTemplate, error)
	ApplyToMonitors(ctx context.Context, id int64, monitorIDs []int64) (int64, error)
	CountAssociatedMonitors(ctx context.Context, id int64) (int64, error)
	ListAssociatedMonitors(ctx context.Context, id int64) ([]*ModelMarketplaceAssociatedMonitorBrief, error)
}

var (
	ErrModelMarketplaceTemplateNotFound = infraerrors.NotFound(
		"MODEL_MARKETPLACE_TEMPLATE_NOT_FOUND", "model marketplace request template not found",
	)
	ErrModelMarketplaceTemplateInvalidProvider = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_INVALID_PROVIDER", "provider must be one of openai/anthropic/gemini",
	)
	ErrModelMarketplaceTemplateMissingName = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_MISSING_NAME", "template name is required",
	)
	ErrModelMarketplaceTemplateProviderMismatch = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_PROVIDER_MISMATCH", "template provider must match monitor provider",
	)
	ErrModelMarketplaceTemplateInvalidBodyMode = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_INVALID_BODY_MODE", "body_override_mode must be off/merge/replace",
	)
	ErrModelMarketplaceTemplateBodyRequired = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_BODY_REQUIRED", "body_override is required when body_override_mode is merge or replace",
	)
	ErrModelMarketplaceTemplateHeaderInvalidName = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_HEADER_INVALID_NAME", "extra_headers contains an invalid header name",
	)
	ErrModelMarketplaceTemplateHeaderForbidden = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_HEADER_FORBIDDEN", "extra_headers contains a forbidden header name",
	)
	ErrModelMarketplaceTemplateApplyEmpty = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_TEMPLATE_APPLY_EMPTY", "monitor_ids is required",
	)
)
