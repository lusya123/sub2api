package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/ent/accountgroup"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

const (
	SettingKeyPublicStatusConfig = "public_status_config"

	publicStatusAggregateMonthly = "monthly_card"
	publicStatusAggregateLite    = "lite"
	publicStatusAggregatePro     = "pro"
	publicStatusAggregateMax     = "max"
	publicStatusAggregateMaxPure = "max_pure"
	publicStatusAggregateAWS     = "aws"
	publicStatusAggregateSpecial = "special_offer"
)

const (
	publicStatusGroupRankMaxPure = iota
	publicStatusGroupRankAWS
	publicStatusGroupRankLite
	publicStatusGroupRankSpecial
	publicStatusGroupRankPro
	publicStatusGroupRankMonthly
	publicStatusGroupRankMax
	publicStatusGroupRankUnknown = 100
)

type PublicStatusModelConfig struct {
	Name          string        `json:"name"`
	Provider      string        `json:"provider,omitempty"`
	ReleaseDate   string        `json:"release_date,omitempty"`
	PromptCaching bool          `json:"prompt_caching"`
	Note          string        `json:"note,omitempty"`
	Pricing       StatusPricing `json:"pricing,omitempty"`
	Enabled       bool          `json:"enabled"`
}

type PublicStatusProbeLineConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Region    string `json:"region,omitempty"`
	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sort_order,omitempty"`
}

type PublicStatusGroupConfig struct {
	GroupID      int64                         `json:"group_id"`
	Enabled      bool                          `json:"enabled"`
	DisplayName  string                        `json:"display_name,omitempty"`
	AggregateKey string                        `json:"aggregate_key,omitempty"`
	SortOrder    int                           `json:"sort_order,omitempty"`
	Models       []string                      `json:"models,omitempty"`
	ProbeLines   []PublicStatusProbeLineConfig `json:"probe_lines,omitempty"`
}

type PublicStatusConfig struct {
	Models []PublicStatusModelConfig `json:"models"`
	Groups []PublicStatusGroupConfig `json:"groups"`
}

type PublicStatusGroupOption struct {
	GroupID          int64                         `json:"group_id"`
	Name             string                        `json:"name"`
	Platform         string                        `json:"platform"`
	SubscriptionType string                        `json:"subscription_type"`
	Status           string                        `json:"status"`
	SupportedScopes  []string                      `json:"supported_scopes"`
	AccountCount     int                           `json:"account_count"`
	Enabled          bool                          `json:"enabled"`
	DisplayName      string                        `json:"display_name,omitempty"`
	AggregateKey     string                        `json:"aggregate_key,omitempty"`
	SortOrder        int                           `json:"sort_order,omitempty"`
	SuggestedName    string                        `json:"suggested_name"`
	SuggestedKey     string                        `json:"suggested_key,omitempty"`
	ProbeLines       []PublicStatusProbeLineConfig `json:"probe_lines,omitempty"`
}

type PublicStatusConfigAdminView struct {
	Config       PublicStatusConfig        `json:"config"`
	GroupOptions []PublicStatusGroupOption `json:"group_options"`
}

func defaultPublicStatusModels() []PublicStatusModelConfig {
	return []PublicStatusModelConfig{
		modelConfigFromCatalog("claude-sonnet-4-6"),
		modelConfigFromCatalog("claude-opus-4-6"),
		modelConfigFromCatalog("claude-opus-4-7"),
		modelConfigFromCatalog("claude-sonnet-4-5-20250929"),
		modelConfigFromCatalog("claude-haiku-4-5-20251001"),
		modelConfigFromCatalog("glm-5"),
		modelConfigFromCatalog("minimax-m2.5"),
	}
}

func defaultPublicStatusConfig() PublicStatusConfig {
	return PublicStatusConfig{
		Models: defaultPublicStatusModels(),
		Groups: []PublicStatusGroupConfig{},
	}
}

func modelConfigFromCatalog(name string) PublicStatusModelConfig {
	md := lookupMetadata(name)
	return PublicStatusModelConfig{
		Name:          name,
		Provider:      md.Provider,
		ReleaseDate:   md.ReleaseDate,
		PromptCaching: md.PromptCaching,
		Note:          md.Note,
		Pricing:       md.Pricing,
		Enabled:       true,
	}
}

func normalizePublicStatusConfig(in PublicStatusConfig) PublicStatusConfig {
	out := PublicStatusConfig{
		Models: make([]PublicStatusModelConfig, 0, len(in.Models)),
		Groups: make([]PublicStatusGroupConfig, 0, len(in.Groups)),
	}

	seenModels := map[string]struct{}{}
	for _, m := range in.Models {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seenModels[key]; ok {
			continue
		}
		seenModels[key] = struct{}{}
		if strings.TrimSpace(m.Provider) == "" {
			md := lookupMetadata(name)
			m.Provider = md.Provider
			if m.ReleaseDate == "" {
				m.ReleaseDate = md.ReleaseDate
			}
			if m.Note == "" {
				m.Note = md.Note
			}
			if isZeroPricing(m.Pricing) {
				m.Pricing = md.Pricing
			}
			m.PromptCaching = m.PromptCaching || md.PromptCaching
		}
		m.Name = name
		out.Models = append(out.Models, m)
	}
	if len(out.Models) == 0 {
		out.Models = defaultPublicStatusModels()
	}

	seenGroups := map[int64]struct{}{}
	for _, g := range in.Groups {
		if g.GroupID <= 0 {
			continue
		}
		if _, ok := seenGroups[g.GroupID]; ok {
			continue
		}
		seenGroups[g.GroupID] = struct{}{}
		g.DisplayName = strings.TrimSpace(g.DisplayName)
		g.AggregateKey = strings.TrimSpace(g.AggregateKey)
		g.Models = normalizeStringSlice(g.Models)
		g.ProbeLines = normalizePublicStatusProbeLines(g.ProbeLines)
		out.Groups = append(out.Groups, g)
	}
	sort.Slice(out.Groups, func(i, j int) bool { return out.Groups[i].GroupID < out.Groups[j].GroupID })
	return out
}

func isZeroPricing(p StatusPricing) bool {
	return p.InputPerMTok == 0 && p.OutputPerMTok == 0 && p.CacheWrite == 0 && p.CacheRead == 0
}

func (s *StatusPageService) WithSettingRepo(repo SettingRepository) *StatusPageService {
	if s == nil {
		return s
	}
	s.settingRepo = repo
	return s
}

func (s *StatusPageService) WithPublicStatusConfig(cfg PublicStatusConfig) *StatusPageService {
	if s == nil {
		return s
	}
	normalized := normalizePublicStatusConfig(cfg)
	s.fixedConfig = &normalized
	return s
}

func (p *ChannelHealthProber) WithSettingRepo(repo SettingRepository) *ChannelHealthProber {
	if p == nil {
		return p
	}
	p.settingRepo = repo
	return p
}

func (p *ChannelHealthProber) WithPublicStatusConfig(cfg PublicStatusConfig) *ChannelHealthProber {
	if p == nil {
		return p
	}
	normalized := normalizePublicStatusConfig(cfg)
	p.fixedConfig = &normalized
	return p
}

func (s *StatusPageService) loadPublicStatusConfig(ctx context.Context) (PublicStatusConfig, error) {
	if s == nil {
		cfg, _, err := loadPublicStatusConfig(ctx, nil)
		return cfg, err
	}
	if s != nil && s.fixedConfig != nil {
		return *s.fixedConfig, nil
	}
	cfg, configured, err := loadPublicStatusConfig(ctx, s.settingRepo)
	if err != nil || configured || s.entClient == nil {
		return cfg, err
	}
	return derivePublicStatusConfig(ctx, s.entClient, cfg)
}

func (p *ChannelHealthProber) loadPublicStatusConfig(ctx context.Context) (PublicStatusConfig, error) {
	if p == nil {
		cfg, _, err := loadPublicStatusConfig(ctx, nil)
		return cfg, err
	}
	if p != nil && p.fixedConfig != nil {
		return *p.fixedConfig, nil
	}
	cfg, configured, err := loadPublicStatusConfig(ctx, p.settingRepo)
	if err != nil || configured || p.entClient == nil {
		return cfg, err
	}
	return derivePublicStatusConfig(ctx, p.entClient, cfg)
}

func loadPublicStatusConfig(ctx context.Context, repo SettingRepository) (PublicStatusConfig, bool, error) {
	if repo == nil {
		return defaultPublicStatusConfig(), false, nil
	}
	raw, err := repo.GetValue(ctx, SettingKeyPublicStatusConfig)
	if err != nil {
		if isSettingNotFound(err) {
			return defaultPublicStatusConfig(), false, nil
		}
		return PublicStatusConfig{}, false, err
	}
	if strings.TrimSpace(raw) == "" {
		return defaultPublicStatusConfig(), false, nil
	}
	var cfg PublicStatusConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return PublicStatusConfig{}, false, fmt.Errorf("parse public status config: %w", err)
	}
	return normalizePublicStatusConfig(cfg), true, nil
}

func isSettingNotFound(err error) bool {
	return errors.Is(err, ErrSettingNotFound)
}

func (s *StatusPageService) GetPublicStatusConfig(ctx context.Context) (PublicStatusConfig, error) {
	return s.loadPublicStatusConfig(ctx)
}

func derivePublicStatusConfig(ctx context.Context, entClient *dbent.Client, base PublicStatusConfig) (PublicStatusConfig, error) {
	cfg := normalizePublicStatusConfig(base)
	if entClient == nil {
		return cfg, nil
	}

	models, err := derivePublicStatusModels(ctx, entClient, cfg.Models)
	if err != nil {
		return PublicStatusConfig{}, err
	}
	if len(models) > 0 {
		cfg.Models = models
	}

	groups, err := derivePublicStatusGroups(ctx, entClient)
	if err != nil {
		return PublicStatusConfig{}, err
	}
	if len(groups) > 0 {
		cfg.Groups = groups
	}
	return normalizePublicStatusConfig(cfg), nil
}

func derivePublicStatusModels(ctx context.Context, entClient *dbent.Client, base []PublicStatusModelConfig) ([]PublicStatusModelConfig, error) {
	out := make([]PublicStatusModelConfig, 0, len(base))
	seen := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, modelConfigFromCatalog(name))
	}
	for _, m := range base {
		if !m.Enabled {
			continue
		}
		add(m.Name)
	}

	accounts, err := entClient.Account.Query().
		Where(account.DeletedAtIsNil(), account.StatusEQ(StatusActive), account.SchedulableEQ(true)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("derive public status models: %w", err)
	}
	for _, acc := range accounts {
		for _, model := range supportedMarketplaceModels(publicStatusAccountFromEnt(acc)) {
			add(model.modelID)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := publicStatusModelRank(out[i].Name), publicStatusModelRank(out[j].Name)
		if ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func publicStatusAccountFromEnt(acc *dbent.Account) *Account {
	if acc == nil {
		return nil
	}
	return &Account{
		ID:          acc.ID,
		Platform:    acc.Platform,
		Type:        acc.Type,
		Credentials: acc.Credentials,
		Extra:       acc.Extra,
		Status:      acc.Status,
		Schedulable: acc.Schedulable,
	}
}

func publicStatusModelRank(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "claude-opus-4-7":
		return 0
	case "claude-sonnet-4-6":
		return 1
	case "claude-opus-4-6":
		return 2
	case "claude-sonnet-4-5-20250929":
		return 3
	case "claude-haiku-4-5-20251001":
		return 4
	case "glm-5":
		return 5
	case "minimax-m2.5":
		return 6
	default:
		return 100
	}
}

func derivePublicStatusGroups(ctx context.Context, entClient *dbent.Client) ([]PublicStatusGroupConfig, error) {
	groups, err := entClient.Group.Query().
		Where(group.DeletedAtIsNil(), group.StatusEQ(StatusActive)).
		Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("derive public status groups: %w", err)
	}
	counts, err := countSchedulableAccountsByGroup(ctx, entClient)
	if err != nil {
		return nil, err
	}
	out := make([]PublicStatusGroupConfig, 0, len(groups))
	for _, g := range groups {
		if counts[g.ID] == 0 {
			continue
		}
		name, key := suggestedPublicStatusGroup(g)
		if key == "" || !defaultPublicStatusAggregateEnabled(key) {
			continue
		}
		out = append(out, PublicStatusGroupConfig{
			GroupID:      g.ID,
			Enabled:      true,
			DisplayName:  name,
			AggregateKey: key,
			SortOrder:    publicStatusGroupRank(key, name) + 1,
			ProbeLines:   defaultPublicStatusProbeLines(key, name),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		if out[i].AggregateKey != out[j].AggregateKey {
			return out[i].AggregateKey < out[j].AggregateKey
		}
		return out[i].GroupID < out[j].GroupID
	})
	return out, nil
}

func defaultPublicStatusAggregateEnabled(key string) bool {
	switch strings.TrimSpace(key) {
	case publicStatusAggregateMaxPure, publicStatusAggregateAWS, publicStatusAggregateLite,
		publicStatusAggregateSpecial, publicStatusAggregatePro, publicStatusAggregateMonthly:
		return true
	default:
		return false
	}
}

func defaultPublicStatusProbeLines(key, name string) []PublicStatusProbeLineConfig {
	if key == publicStatusAggregateAWS {
		return []PublicStatusProbeLineConfig{
			{ID: "us", Name: "US", Region: "Virginia", Enabled: true, SortOrder: 1},
			{ID: "eu", Name: "EU", Region: "Frankfurt", Enabled: true, SortOrder: 2},
			{ID: "asia", Name: "Asia", Region: "Singapore", Enabled: true, SortOrder: 3},
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "默认线路"
	}
	return []PublicStatusProbeLineConfig{{ID: "default", Name: name, Enabled: true, SortOrder: 1}}
}

func (s *StatusPageService) GetPublicStatusConfigAdmin(ctx context.Context) (*PublicStatusConfigAdminView, error) {
	if s == nil || s.entClient == nil {
		return nil, fmt.Errorf("status_page_service: entClient is nil")
	}
	cfg, err := s.loadPublicStatusConfig(ctx)
	if err != nil {
		return nil, err
	}
	options, err := s.listPublicStatusGroupOptions(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &PublicStatusConfigAdminView{Config: cfg, GroupOptions: options}, nil
}

func (s *StatusPageService) SetPublicStatusConfig(ctx context.Context, cfg PublicStatusConfig) (*PublicStatusConfigAdminView, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("status_page_service: setting repo is nil")
	}
	normalized := normalizePublicStatusConfig(cfg)
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal public status config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyPublicStatusConfig, string(data)); err != nil {
		return nil, err
	}
	s.clearCaches()
	return s.GetPublicStatusConfigAdmin(ctx)
}

func (s *StatusPageService) listPublicStatusGroupOptions(ctx context.Context, cfg PublicStatusConfig) ([]PublicStatusGroupOption, error) {
	groups, err := s.entClient.Group.Query().
		Where(group.DeletedAtIsNil()).
		Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	enabledByID := map[int64]PublicStatusGroupConfig{}
	for _, g := range cfg.Groups {
		enabledByID[g.GroupID] = g
	}
	counts, err := s.countSchedulableAccountsByGroup(ctx)
	if err != nil {
		return nil, err
	}
	options := make([]PublicStatusGroupOption, 0, len(groups))
	for _, g := range groups {
		item := PublicStatusGroupOption{
			GroupID:          g.ID,
			Name:             g.Name,
			Platform:         g.Platform,
			SubscriptionType: g.SubscriptionType,
			Status:           g.Status,
			SupportedScopes:  normalizeStringSlice(g.SupportedModelScopes),
			AccountCount:     counts[g.ID],
		}
		item.SuggestedName, item.SuggestedKey = suggestedPublicStatusGroup(g)
		if configured, ok := enabledByID[g.ID]; ok {
			item.Enabled = configured.Enabled
			item.DisplayName = configured.DisplayName
			item.AggregateKey = configured.AggregateKey
			item.SortOrder = configured.SortOrder
			item.ProbeLines = configured.ProbeLines
		}
		options = append(options, item)
	}
	return options, nil
}

func (s *StatusPageService) countSchedulableAccountsByGroup(ctx context.Context) (map[int64]int, error) {
	return countSchedulableAccountsByGroup(ctx, s.entClient)
}

func countSchedulableAccountsByGroup(ctx context.Context, entClient *dbent.Client) (map[int64]int, error) {
	rows, err := entClient.AccountGroup.Query().
		Where(accountgroup.HasAccountWith(
			account.DeletedAtIsNil(),
			account.StatusEQ(StatusActive),
			account.SchedulableEQ(true),
		)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int)
	seen := make(map[int64]map[int64]struct{})
	for _, row := range rows {
		if _, ok := seen[row.GroupID]; !ok {
			seen[row.GroupID] = map[int64]struct{}{}
		}
		if _, ok := seen[row.GroupID][row.AccountID]; ok {
			continue
		}
		seen[row.GroupID][row.AccountID] = struct{}{}
		out[row.GroupID]++
	}
	return out, nil
}

func suggestedPublicStatusGroup(g *dbent.Group) (string, string) {
	if g == nil {
		return "", ""
	}
	if g.SubscriptionType == SubscriptionTypeSubscription || strings.Contains(g.Name, "月卡") {
		return "月卡", publicStatusAggregateMonthly
	}
	name := strings.ToLower(strings.TrimSpace(g.Name))
	switch {
	case strings.Contains(name, "aws"):
		return "AWS", publicStatusAggregateAWS
	case strings.Contains(g.Name, "特惠") || strings.Contains(name, "special"):
		return "特惠", publicStatusAggregateSpecial
	case strings.Contains(g.Name, "纯血"):
		return "MAX 纯血", publicStatusAggregateMaxPure
	case strings.Contains(name, "max"):
		return "MAX", publicStatusAggregateMax
	case strings.Contains(name, "lite") || strings.Contains(name, "轻量"):
		return "lite", publicStatusAggregateLite
	case strings.Contains(name, "pro") || strings.Contains(g.Name, "专业"):
		return "PRO", publicStatusAggregatePro
	}
	return g.Name, ""
}

func publicStatusGroupRank(aggregateKey, displayName string) int {
	key := strings.ToLower(strings.TrimSpace(aggregateKey))
	switch key {
	case publicStatusAggregateMaxPure:
		return publicStatusGroupRankMaxPure
	case publicStatusAggregateAWS:
		return publicStatusGroupRankAWS
	case publicStatusAggregateLite:
		return publicStatusGroupRankLite
	case publicStatusAggregateSpecial:
		return publicStatusGroupRankSpecial
	case publicStatusAggregatePro:
		return publicStatusGroupRankPro
	case publicStatusAggregateMonthly:
		return publicStatusGroupRankMonthly
	case publicStatusAggregateMax:
		return publicStatusGroupRankMax
	}

	name := strings.ToLower(strings.TrimSpace(displayName))
	switch {
	case strings.Contains(displayName, "纯血"):
		return publicStatusGroupRankMaxPure
	case strings.Contains(name, "aws"):
		return publicStatusGroupRankAWS
	case strings.Contains(name, "light") || strings.Contains(name, "lite") || strings.Contains(displayName, "轻量"):
		return publicStatusGroupRankLite
	case strings.Contains(displayName, "特惠") || strings.Contains(name, "special"):
		return publicStatusGroupRankSpecial
	case strings.Contains(name, "pro") || strings.Contains(displayName, "专业"):
		return publicStatusGroupRankPro
	case strings.Contains(displayName, "月卡"):
		return publicStatusGroupRankMonthly
	case strings.Contains(name, "max"):
		return publicStatusGroupRankMax
	default:
		return publicStatusGroupRankUnknown
	}
}

func enabledPublicStatusModels(cfg PublicStatusConfig) []PublicStatusModelConfig {
	models := make([]PublicStatusModelConfig, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		if !m.Enabled {
			continue
		}
		if strings.TrimSpace(m.Name) == "" {
			continue
		}
		models = append(models, m)
	}
	return models
}

func enabledPublicStatusGroups(cfg PublicStatusConfig) map[int64]PublicStatusGroupConfig {
	out := make(map[int64]PublicStatusGroupConfig, len(cfg.Groups))
	for _, g := range cfg.Groups {
		if !g.Enabled || g.GroupID <= 0 {
			continue
		}
		out[g.GroupID] = g
	}
	return out
}

func metadataFromConfig(m PublicStatusModelConfig) modelMetadata {
	md := lookupMetadata(m.Name)
	if strings.TrimSpace(m.Provider) != "" {
		md.Provider = strings.TrimSpace(m.Provider)
	}
	if m.ReleaseDate != "" {
		md.ReleaseDate = m.ReleaseDate
	}
	if m.Note != "" {
		md.Note = m.Note
	}
	if !isZeroPricing(m.Pricing) {
		md.Pricing = m.Pricing
	}
	md.PromptCaching = md.PromptCaching || m.PromptCaching
	return md
}

func normalizeStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func groupSupportsStatusModel(g *dbent.Group, modelName string) bool {
	if g == nil || g.Status != StatusActive {
		return false
	}
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if modelName == "" {
		return false
	}
	for pattern := range g.ModelRouting {
		if matchModelPattern(strings.ToLower(strings.TrimSpace(pattern)), modelName) {
			return true
		}
	}
	scopes := normalizeStringSlice(g.SupportedModelScopes)
	if len(scopes) == 0 {
		// Legacy Claude groups often have no explicit scope metadata. Treat
		// those as Claude-only so future Gemini/OpenAI/GLM models configured by
		// admins do not accidentally leak onto every old group.
		return strings.HasPrefix(modelName, "claude-")
	}
	if strings.HasPrefix(modelName, "claude-") {
		return containsString(scopes, "claude")
	}
	if strings.HasPrefix(modelName, "gemini-") {
		return containsString(scopes, "gemini_text") || containsString(scopes, "gemini")
	}
	if strings.HasPrefix(modelName, "gpt-") || strings.HasPrefix(modelName, "o1") || strings.HasPrefix(modelName, "o3") {
		return containsString(scopes, "openai")
	}
	if family := strings.SplitN(modelName, "-", 2)[0]; family != "" {
		return containsString(scopes, family)
	}
	return false
}

func publicStatusGroupConfigSupportsModel(gc PublicStatusGroupConfig, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" || len(gc.Models) == 0 {
		return true
	}
	for _, allowed := range gc.Models {
		if strings.EqualFold(strings.TrimSpace(allowed), modelName) {
			return true
		}
	}
	return false
}

func publicStatusGroupProbeLines(gc PublicStatusGroupConfig, fallbackName string) []PublicStatusProbeLineConfig {
	lines := normalizePublicStatusProbeLines(gc.ProbeLines)
	out := make([]PublicStatusProbeLineConfig, 0, len(lines))
	for _, line := range lines {
		if line.Enabled {
			out = append(out, line)
		}
	}
	if len(out) > 0 {
		return out
	}
	name := strings.TrimSpace(fallbackName)
	if name == "" {
		name = strings.TrimSpace(gc.DisplayName)
	}
	if name == "" {
		name = "默认线路"
	}
	return []PublicStatusProbeLineConfig{{
		ID:      "default",
		Name:    name,
		Enabled: true,
	}}
}

func publicStatusProbeLineDisplayName(line PublicStatusProbeLineConfig) string {
	name := strings.TrimSpace(line.Name)
	region := strings.TrimSpace(line.Region)
	if name == "" {
		name = region
	}
	if name == "" {
		return ""
	}
	if region == "" || strings.EqualFold(name, region) {
		return name
	}
	return fmt.Sprintf("%s · %s", name, region)
}

func normalizePublicStatusProbeLines(in []PublicStatusProbeLineConfig) []PublicStatusProbeLineConfig {
	out := make([]PublicStatusProbeLineConfig, 0, len(in))
	seen := map[string]struct{}{}
	for idx, line := range in {
		line.Name = strings.TrimSpace(line.Name)
		line.Region = strings.TrimSpace(line.Region)
		if line.Name == "" && line.Region != "" {
			line.Name = line.Region
		}
		if line.Name == "" {
			line.Name = fmt.Sprintf("线路 %d", idx+1)
		}
		line.ID = strings.TrimSpace(line.ID)
		if line.ID == "" {
			line.ID = publicStatusProbeLineID(line.Name, idx)
		}
		baseID := line.ID
		for n := 2; ; n++ {
			if _, ok := seen[strings.ToLower(line.ID)]; !ok {
				break
			}
			line.ID = fmt.Sprintf("%s-%d", baseID, n)
		}
		seen[strings.ToLower(line.ID)] = struct{}{}
		out = append(out, line)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func publicStatusProbeLineID(name string, idx int) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			_, _ = b.WriteRune(r)
		case r >= '0' && r <= '9':
			_, _ = b.WriteRune(r)
		default:
			if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
				_ = b.WriteByte('-')
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = fmt.Sprintf("line-%d", idx+1)
	}
	return id
}

func containsString(values []string, needle string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), needle) {
			return true
		}
	}
	return false
}

func normalizeProbeModelName(modelName string) string {
	return claude.NormalizeModelID(strings.TrimSpace(modelName))
}
