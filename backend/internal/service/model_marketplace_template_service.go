package service

import (
	"context"
	"strings"
)

type ModelMarketplaceTemplateService struct {
	repo ModelMarketplaceTemplateRepository
}

func NewModelMarketplaceTemplateService(repo ModelMarketplaceTemplateRepository) *ModelMarketplaceTemplateService {
	return &ModelMarketplaceTemplateService{repo: repo}
}

func (s *ModelMarketplaceTemplateService) List(ctx context.Context, params ModelMarketplaceTemplateListParams) ([]*ModelMarketplaceTemplate, error) {
	if strings.TrimSpace(params.Provider) != "" {
		if err := validateModelMarketplaceProvider(params.Provider); err != nil {
			return nil, ErrModelMarketplaceTemplateInvalidProvider
		}
	}
	return s.repo.List(ctx, params)
}

func (s *ModelMarketplaceTemplateService) Get(ctx context.Context, id int64) (*ModelMarketplaceTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ModelMarketplaceTemplateService) Create(ctx context.Context, p ModelMarketplaceTemplateCreateParams) (*ModelMarketplaceTemplate, error) {
	if strings.TrimSpace(p.Name) == "" {
		return nil, ErrModelMarketplaceTemplateMissingName
	}
	if err := validateModelMarketplaceProvider(p.Provider); err != nil {
		return nil, ErrModelMarketplaceTemplateInvalidProvider
	}
	if err := validateModelMarketplaceBodyModeParams(p.BodyOverrideMode, p.BodyOverride); err != nil {
		return nil, err
	}
	if err := validateModelMarketplaceExtraHeaders(p.ExtraHeaders); err != nil {
		return nil, err
	}
	t := &ModelMarketplaceTemplate{
		Name:             strings.TrimSpace(p.Name),
		Provider:         p.Provider,
		Description:      strings.TrimSpace(p.Description),
		ExtraHeaders:     emptyModelMarketplaceHeadersIfNil(p.ExtraHeaders),
		BodyOverrideMode: defaultModelMarketplaceBodyMode(p.BodyOverrideMode),
		BodyOverride:     p.BodyOverride,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ModelMarketplaceTemplateService) Update(ctx context.Context, id int64, p ModelMarketplaceTemplateUpdateParams) (*ModelMarketplaceTemplate, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		if strings.TrimSpace(*p.Name) == "" {
			return nil, ErrModelMarketplaceTemplateMissingName
		}
		existing.Name = strings.TrimSpace(*p.Name)
	}
	if p.Description != nil {
		existing.Description = strings.TrimSpace(*p.Description)
	}
	if p.ExtraHeaders != nil {
		if err := validateModelMarketplaceExtraHeaders(*p.ExtraHeaders); err != nil {
			return nil, err
		}
		existing.ExtraHeaders = emptyModelMarketplaceHeadersIfNil(*p.ExtraHeaders)
	}
	newMode := existing.BodyOverrideMode
	newBody := existing.BodyOverride
	if p.BodyOverrideMode != nil {
		newMode = *p.BodyOverrideMode
	}
	if p.BodyOverride != nil {
		newBody = *p.BodyOverride
	}
	if p.BodyOverrideMode != nil || p.BodyOverride != nil {
		if err := validateModelMarketplaceBodyModeParams(newMode, newBody); err != nil {
			return nil, err
		}
		existing.BodyOverrideMode = defaultModelMarketplaceBodyMode(newMode)
		existing.BodyOverride = newBody
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *ModelMarketplaceTemplateService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *ModelMarketplaceTemplateService) ApplyToMonitors(ctx context.Context, id int64, monitorIDs []int64) (int64, error) {
	if len(monitorIDs) == 0 {
		return 0, ErrModelMarketplaceTemplateApplyEmpty
	}
	return s.repo.ApplyToMonitors(ctx, id, monitorIDs)
}

func (s *ModelMarketplaceTemplateService) CountAssociatedMonitors(ctx context.Context, id int64) (int64, error) {
	return s.repo.CountAssociatedMonitors(ctx, id)
}

func (s *ModelMarketplaceTemplateService) ListAssociatedMonitors(ctx context.Context, id int64) ([]*ModelMarketplaceAssociatedMonitorBrief, error) {
	return s.repo.ListAssociatedMonitors(ctx, id)
}
