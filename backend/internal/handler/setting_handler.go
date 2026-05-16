package handler

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
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
)

// InvalidateLogoCache clears the in-process public logo cache after settings change.
func InvalidateLogoCache() {
	logoCache.Store(nil)
}

// SettingHandler 公开设置处理器（无需认证）
type SettingHandler struct {
	settingService *service.SettingService
	version        string
}

// NewSettingHandler 创建公开设置处理器
func NewSettingHandler(settingService *service.SettingService, version string) *SettingHandler {
	return &SettingHandler{
		settingService: settingService,
		version:        version,
	}
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

		ChatPageEnabled: settings.ChatPageEnabled,
		ChatPageURL:     settings.ChatPageURL,

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
		response.ErrorFrom(c, err)
		return
	}

	data, contentType, ok := service.DecodeInlineImageDataURL(settings.SiteLogo)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	etag := `"` + service.PublicSiteLogoVersion(settings.SiteLogo) + `"`
	logoCache.Store(&logoCacheEntry{
		data:        data,
		contentType: contentType,
		etag:        etag,
		expiresAt:   time.Now().Add(logoMemoryCacheTTL()),
	})

	setPublicLogoCacheHeaders(c, etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, contentType, data)
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
