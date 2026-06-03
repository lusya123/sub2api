package handler

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type logoCacheEntry struct {
	data        []byte
	contentType string
	etag        string
	expiresAt   time.Time
}

var logoCache atomic.Pointer[logoCacheEntry]

const (
	defaultLogoMemoryCacheTTL = 5 * time.Minute
	logoHTTPCacheMaxAge       = 3600
	logoPreloadDelay          = 2 * time.Second
	logoPreloadInterval       = 2 * time.Second
	logoPreloadAttempts       = 5
	logoPreloadTimeout        = 5 * time.Second
)

// InvalidateLogoCache clears the in-process public logo cache after settings change.
func InvalidateLogoCache() {
	logoCache.Store(nil)
}

func PreloadLogoCache(settingService *service.SettingService) {
	if settingService == nil || os.Getenv("LOGO_CACHE_PRELOAD_ENABLED") == "false" {
		return
	}
	go func() {
		time.Sleep(logoPreloadDelay)
		for attempt := 1; attempt <= logoPreloadAttempts; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), logoPreloadTimeout)
			ok := preloadLogoCacheOnce(ctx, settingService)
			cancel()
			if ok {
				log.Printf("[logo] cache preloaded successfully attempt=%d", attempt)
				return
			}
			if attempt < logoPreloadAttempts {
				time.Sleep(logoPreloadInterval)
			}
		}
		log.Printf("[logo] cache preload failed after %d retries", logoPreloadAttempts)
	}()
}

// SettingHandler 公开设置处理器（无需认证）
type SettingHandler struct {
	settingService           *service.SettingService
	notificationEmailService *service.NotificationEmailService
	version                  string
}

// NewSettingHandler 创建公开设置处理器
func NewSettingHandler(settingService *service.SettingService, version string) *SettingHandler {
	return &SettingHandler{
		settingService: settingService,
		version:        version,
	}
}

// SetNotificationEmailService attaches the public notification email service without
// changing the constructor signature used by existing tests.
func (h *SettingHandler) SetNotificationEmailService(notificationEmailService *service.NotificationEmailService) {
	h.notificationEmailService = notificationEmailService
}

// GetPublicSettings 获取公开设置
// GET /api/v1/settings/public
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.PublicSettings{
		RegistrationEnabled:              settings.RegistrationEnabled,
		EmailVerifyEnabled:               settings.EmailVerifyEnabled,
		ForceEmailOnThirdPartySignup:     settings.ForceEmailOnThirdPartySignup,
		RegistrationEmailSuffixWhitelist: settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 settings.PromoCodeEnabled,
		PasswordResetEnabled:             settings.PasswordResetEnabled,
		InvitationCodeEnabled:            settings.InvitationCodeEnabled,
		TotpEnabled:                      settings.TotpEnabled,
		LoginAgreementEnabled:            settings.LoginAgreementEnabled,
		LoginAgreementMode:               settings.LoginAgreementMode,
		LoginAgreementUpdatedAt:          settings.LoginAgreementUpdatedAt,
		LoginAgreementRevision:           settings.LoginAgreementRevision,
		LoginAgreementDocuments:          publicLoginAgreementDocumentsToDTO(settings.LoginAgreementDocuments),
		TurnstileEnabled:                 settings.TurnstileEnabled,
		TurnstileSiteKey:                 settings.TurnstileSiteKey,
		SiteName:                         settings.SiteName,
		SiteLogo:                         service.PublicSiteLogoForClient(settings.SiteLogo),
		SiteSubtitle:                     settings.SiteSubtitle,
		APIBaseURL:                       settings.APIBaseURL,
		ContactInfo:                      settings.ContactInfo,
		DocURL:                           settings.DocURL,
		HomeContent:                      settings.HomeContent,
		HideCcsImportButton:              settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:      settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionMode:         settings.PurchaseSubscriptionMode,
		PurchaseSubscriptionEmbeddedURL:  settings.PurchaseSubscriptionEmbeddedURL,
		PurchaseSubscriptionRedirectURL:  settings.PurchaseSubscriptionRedirectURL,
		PurchaseSubscriptionURL:          settings.PurchaseSubscriptionURL,
		ModelHealthPageEnabled:           settings.ModelHealthPageEnabled,
		TableDefaultPageSize:             settings.TableDefaultPageSize,
		TablePageSizeOptions:             settings.TablePageSizeOptions,
		CustomMenuItems:                  dto.ParseUserVisibleMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                  dto.ParseCustomEndpoints(settings.CustomEndpoints),
		DingTalkOAuthEnabled:             settings.DingTalkOAuthEnabled,
		LinuxDoOAuthEnabled:              settings.LinuxDoOAuthEnabled,
		WeChatOAuthEnabled:               settings.WeChatOAuthEnabled,
		WeChatOAuthOpenEnabled:           settings.WeChatOAuthOpenEnabled,
		WeChatOAuthMPEnabled:             settings.WeChatOAuthMPEnabled,
		WeChatOAuthMobileEnabled:         settings.WeChatOAuthMobileEnabled,
		OIDCOAuthEnabled:                 settings.OIDCOAuthEnabled,
		OIDCOAuthProviderName:            settings.OIDCOAuthProviderName,
		GitHubOAuthEnabled:               settings.GitHubOAuthEnabled,
		GoogleOAuthEnabled:               settings.GoogleOAuthEnabled,
		BackendModeEnabled:               settings.BackendModeEnabled,
		PaymentEnabled:                   settings.PaymentEnabled,
		Version:                          h.version,
		BalanceLowNotifyEnabled:          settings.BalanceLowNotifyEnabled,
		AccountQuotaNotifyEnabled:        settings.AccountQuotaNotifyEnabled,
		BalanceLowNotifyThreshold:        settings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:      settings.BalanceLowNotifyRechargeURL,

		ChannelMonitorEnabled:                settings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,

		AvailableChannelsEnabled: settings.AvailableChannelsEnabled,

		ChatPageEnabled:  settings.ChatPageEnabled,
		ChatPageURL:      settings.ChatPageURL,
		AgentPageEnabled: settings.AgentPageEnabled,
		AgentPageURL:     settings.AgentPageURL,

		AffiliateEnabled: settings.AffiliateEnabled,

		RiskControlEnabled: settings.RiskControlEnabled,
	})
}

// GetPublicLogo serves the configured inline logo as a versioned public asset.
// GET /api/v1/settings/logo?v=<hash>
func (h *SettingHandler) GetPublicLogo(c *gin.Context) {
	if cached := logoCache.Load(); cached != nil && time.Now().Before(cached.expiresAt) {
		setPublicLogoCacheHeaders(c, cached.etag)
		if c.GetHeader("If-None-Match") == cached.etag {
			c.Status(http.StatusNotModified)
			return
		}
		c.Header("X-Cache", "HIT-MEM")
		c.Data(http.StatusOK, cached.contentType, cached.data)
		return
	}

	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		log.Printf("[logo] GetPublicSettings failed: %v", err)
		response.ErrorFrom(c, err)
		return
	}

	cached, ok := buildLogoCacheEntry(settings.SiteLogo)
	if !ok {
		if strings.TrimSpace(settings.SiteLogo) == "" {
			log.Printf("[logo] settings.SiteLogo is empty")
		} else {
			log.Printf("[logo] decode failed, SiteLogo prefix: %q", logoValuePrefix(settings.SiteLogo))
		}
		c.Status(http.StatusNotFound)
		return
	}

	logoCache.Store(cached)

	setPublicLogoCacheHeaders(c, cached.etag)
	if c.GetHeader("If-None-Match") == cached.etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, cached.contentType, cached.data)
}

func preloadLogoCacheOnce(ctx context.Context, settingService *service.SettingService) bool {
	settings, err := settingService.GetPublicSettings(ctx)
	if err != nil {
		log.Printf("[logo] preload GetPublicSettings failed: %v", err)
		return false
	}
	cached, ok := buildLogoCacheEntry(settings.SiteLogo)
	if !ok {
		if strings.TrimSpace(settings.SiteLogo) == "" {
			log.Printf("[logo] preload settings.SiteLogo is empty")
		} else {
			log.Printf("[logo] preload decode failed, SiteLogo prefix: %q", logoValuePrefix(settings.SiteLogo))
		}
		return false
	}
	logoCache.Store(cached)
	return true
}

func buildLogoCacheEntry(siteLogo string) (*logoCacheEntry, bool) {
	data, contentType, ok := service.DecodeInlineImageDataURL(siteLogo)
	if !ok {
		return nil, false
	}
	etag := `"` + service.PublicSiteLogoVersion(siteLogo) + `"`
	return &logoCacheEntry{
		data:        data,
		contentType: contentType,
		etag:        etag,
		expiresAt:   time.Now().Add(logoMemoryCacheTTL()),
	}, true
}

func logoValuePrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 80 {
		return value
	}
	return value[:80]
}

func setPublicLogoCacheHeaders(c *gin.Context, etag string) {
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", logoHTTPCacheMaxAge))
	c.Header("ETag", etag)
}

func logoMemoryCacheTTL() time.Duration {
	raw := os.Getenv("LOGO_CACHE_TTL_SECONDS")
	if raw == "" {
		return defaultLogoMemoryCacheTTL
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return defaultLogoMemoryCacheTTL
	}
	return time.Duration(seconds) * time.Second
}

// UnsubscribeNotificationEmail handles optional notification email opt-outs.
// GET /api/v1/settings/email-unsubscribe?token=...
func (h *SettingHandler) UnsubscribeNotificationEmail(c *gin.Context) {
	if h.notificationEmailService == nil {
		response.InternalError(c, "notification email service is not configured")
		return
	}
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		response.BadRequest(c, "token is required")
		return
	}
	result, err := h.notificationEmailService.Unsubscribe(c.Request.Context(), token)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	body := "<!doctype html><html><head><meta charset=\"utf-8\"><title>Unsubscribed</title></head><body style=\"font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;padding:32px;\"><h1>Unsubscribed</h1><p>You have unsubscribed <strong>" + html.EscapeString(result.Email) + "</strong> from <strong>" + html.EscapeString(result.Event) + "</strong> emails.</p></body></html>"
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
}

func publicLoginAgreementDocumentsToDTO(items []service.LoginAgreementDocument) []dto.LoginAgreementDocument {
	result := make([]dto.LoginAgreementDocument, 0, len(items))
	for _, item := range items {
		result = append(result, dto.LoginAgreementDocument{
			ID:        item.ID,
			Title:     item.Title,
			ContentMD: item.ContentMD,
		})
	}
	return result
}
