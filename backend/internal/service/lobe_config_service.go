package service

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

type LobeModelConfig struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type LobeProviderConfig struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name"`
	SDKType     string            `json:"sdk_type"`
	BaseURL     string            `json:"base_url"`
	APIKey      string            `json:"api_key"`
	Models      []LobeModelConfig `json:"models"`
}

type LobeUserConfig struct {
	UserID    string               `json:"user_id"`
	Providers []LobeProviderConfig `json:"providers"`
}

type LobeConfigService struct {
	apiKeyService *APIKeyService
	gateway       *GatewayService
	cfg           *config.Config
}

func NewLobeConfigService(apiKeyService *APIKeyService, gateway *GatewayService, cfg *config.Config) *LobeConfigService {
	return &LobeConfigService{apiKeyService: apiKeyService, gateway: gateway, cfg: cfg}
}

func (s *LobeConfigService) GetUserConfig(ctx context.Context, userID int64) (*LobeUserConfig, error) {
	if s == nil || s.apiKeyService == nil {
		return nil, ErrServiceUnavailable
	}

	groups, err := s.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := &LobeUserConfig{
		UserID:    fmt.Sprintf("%d", userID),
		Providers: make([]LobeProviderConfig, 0, len(groups)),
	}
	for i := range groups {
		group := groups[i]
		if !isLobeChatSupportedGroup(&group) {
			continue
		}

		chatKey, err := s.apiKeyService.EnsureChatKeyForGroup(ctx, userID, &group)
		if err != nil {
			return nil, err
		}

		models := s.modelsForGroup(ctx, &group)
		if len(models) == 0 {
			continue
		}

		out.Providers = append(out.Providers, LobeProviderConfig{
			ID:          fmt.Sprintf("sub2api-group-%d", group.ID),
			DisplayName: group.Name,
			SDKType:     lobeSDKTypeForPlatform(group.Platform),
			BaseURL:     s.baseURLForPlatform(group.Platform),
			APIKey:      chatKey.Key,
			Models:      models,
		})
	}

	return out, nil
}

func (s *LobeConfigService) modelsForGroup(ctx context.Context, group *Group) []LobeModelConfig {
	var ids []string
	if s != nil && s.gateway != nil && group != nil {
		ids = s.gateway.GetAvailableModels(ctx, &group.ID, group.Platform)
	}
	if len(ids) == 0 {
		ids = defaultLobeModelIDs(group)
	}
	ids = filterLobeModelsByGroup(group, ids)
	sort.Strings(ids)

	models := make([]LobeModelConfig, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(strings.TrimPrefix(id, "models/"))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, LobeModelConfig{ID: id, DisplayName: displayNameForModel(id)})
	}
	return models
}

func (s *LobeConfigService) baseURLForPlatform(platform string) string {
	base := ""
	if s != nil && s.cfg != nil {
		base = strings.TrimSpace(s.cfg.Lobe.GatewayBaseURL)
		if base == "" {
			base = strings.TrimSpace(s.cfg.Server.FrontendURL)
		}
		if base == "" {
			host := strings.TrimSpace(s.cfg.Server.Host)
			if host == "" || host == "0.0.0.0" {
				host = "127.0.0.1"
			}
			port := s.cfg.Server.Port
			if port <= 0 {
				port = 8080
			}
			base = fmt.Sprintf("http://%s:%d", host, port)
		}
	}
	base = strings.TrimRight(base, "/")
	if lobeSDKTypeForPlatform(platform) == "google" {
		return joinURLPath(base, "v1beta")
	}
	return joinURLPath(base, "v1")
}

func isLobeChatSupportedGroup(group *Group) bool {
	if group == nil || !group.IsActive() || group.ClaudeCodeOnly {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(group.Platform)) {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity:
		return true
	default:
		return false
	}
}

func lobeSDKTypeForPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformAnthropic, PlatformAntigravity:
		return "anthropic"
	case PlatformGemini:
		return "google"
	default:
		return "openai"
	}
}

func defaultLobeModelIDs(group *Group) []string {
	if group == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(group.Platform)) {
	case PlatformAnthropic:
		return claude.DefaultModelIDs()
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGemini:
		models := gemini.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, strings.TrimPrefix(model.Name, "models/"))
		}
		return ids
	case PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	default:
		return nil
	}
}

func filterLobeModelsByGroup(group *Group, ids []string) []string {
	if group == nil || group.Platform != PlatformAntigravity || len(group.SupportedModelScopes) == 0 {
		return ids
	}
	allowed := make(map[string]struct{}, len(group.SupportedModelScopes))
	for _, scope := range group.SupportedModelScopes {
		allowed[strings.ToLower(strings.TrimSpace(scope))] = struct{}{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		lower := strings.ToLower(strings.TrimPrefix(id, "models/"))
		if strings.HasPrefix(lower, "claude-") {
			if _, ok := allowed["claude"]; ok {
				out = append(out, id)
			}
			continue
		}
		if strings.HasPrefix(lower, "gemini-") {
			if strings.Contains(lower, "image") {
				if _, ok := allowed["gemini_image"]; ok {
					out = append(out, id)
				}
				continue
			}
			if _, ok := allowed["gemini_text"]; ok {
				out = append(out, id)
			}
		}
	}
	return out
}

func displayNameForModel(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_' || r == ':'
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		if isKnownModelToken(parts[i]) {
			parts[i] = strings.ToUpper(parts[i])
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func isKnownModelToken(token string) bool {
	switch strings.ToLower(token) {
	case "gpt", "glm", "ai", "m2.5", "hd", "tts":
		return true
	default:
		return false
	}
}

func joinURLPath(base string, elem string) string {
	if base == "" {
		return "/" + strings.Trim(elem, "/")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(base, "/") + "/" + strings.Trim(elem, "/")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.Trim(elem, "/")
	return parsed.String()
}
