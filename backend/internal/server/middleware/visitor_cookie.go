package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	visitorCookieName       = "_xdt_v"
	visitorCookieDefaultTTL = 1800
	visitorCookieMaxPerMin  = 300
)

type visitorCookieDecision int

const (
	visitorCookieNone visitorCookieDecision = iota
	visitorCookieAllowed
	visitorCookieLimited
)

// VisitorCookieManager manages signed anonymous visitor cookies for frontend documents.
type VisitorCookieManager struct {
	rdb    *redis.Client
	secret []byte
}

func NewVisitorCookieManager(rdb *redis.Client) *VisitorCookieManager {
	return NewVisitorCookieManagerWithSecret(rdb, "")
}

func NewVisitorCookieManagerWithSecret(rdb *redis.Client, fallbackSecret string) *VisitorCookieManager {
	secret := strings.TrimSpace(os.Getenv("DEFENSE_VISITOR_COOKIE_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(fallbackSecret)
	}
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if secret == "" {
		secret = "default-visitor-cookie-secret-please-set-JWT_SECRET"
	}
	return &VisitorCookieManager{
		rdb:    rdb,
		secret: []byte(secret),
	}
}

func (m *VisitorCookieManager) IssueCookie() (value string, maxAge int) {
	return m.IssueCookieWithFingerprint("")
}

func (m *VisitorCookieManager) IssueCookieWithFingerprint(fingerprint string) (value string, maxAge int) {
	ttl := defenseEnvInt("DEFENSE_VISITOR_COOKIE_TTL_SECONDS", visitorCookieDefaultTTL)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	fpHash := m.hashFingerprint(fingerprint)
	nonce := randomVisitorCookieNonce()
	payload := ts + "." + fpHash + "." + nonce
	return payload + "." + m.sign(payload), ttl
}

func (m *VisitorCookieManager) VerifyCookie(cookieValue string) bool {
	if m == nil || cookieValue == "" {
		return false
	}
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 2 && len(parts) != 3 && len(parts) != 4 {
		return false
	}
	if parts[0] == "" || parts[len(parts)-1] == "" {
		return false
	}
	if len(parts) == 3 && parts[1] == "" {
		return false
	}

	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	ttl := int64(defenseEnvInt("DEFENSE_VISITOR_COOKIE_TTL_SECONDS", visitorCookieDefaultTTL))
	if ttl <= 0 || time.Now().Unix()-ts > ttl || ts > time.Now().Unix()+60 {
		return false
	}

	payload := parts[0]
	signature := parts[1]
	if len(parts) == 3 {
		payload = parts[0] + "." + parts[1]
		signature = parts[2]
	} else if len(parts) == 4 {
		payload = parts[0] + "." + parts[1] + "." + parts[2]
		signature = parts[3]
	}
	expected := m.sign(payload)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func (m *VisitorCookieManager) VerifyCookieWithFingerprint(cookieValue, currentFingerprint string) bool {
	if !m.VerifyCookie(cookieValue) {
		return false
	}
	if os.Getenv("DEFENSE_VISITOR_COOKIE_BIND_FINGERPRINT") == "false" {
		return true
	}
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 4 {
		return currentFingerprint == ""
	}
	if strings.TrimSpace(currentFingerprint) == "" {
		return true
	}
	return hmac.Equal([]byte(parts[1]), []byte(m.hashFingerprint(currentFingerprint)))
}

func (m *VisitorCookieManager) hashFingerprint(fingerprint string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(fingerprint)))
	return hex.EncodeToString(h[:8])
}

func randomVisitorCookieNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

func (m *VisitorCookieManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func (m *VisitorCookieManager) AllowCookieRequest(ctx context.Context, cookieValue string) bool {
	if m == nil || m.rdb == nil {
		return true
	}
	limit := defenseEnvInt("DEFENSE_VISITOR_COOKIE_PER_MINUTE", visitorCookieMaxPerMin)
	if limit <= 0 {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	h := sha256.Sum256([]byte(cookieValue))
	key := "vck:" + hex.EncodeToString(h[:8])
	cnt, err := m.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true
	}
	if cnt == 1 {
		_ = m.rdb.Expire(ctx, key, time.Minute).Err()
	}
	return cnt <= int64(limit)
}

func VisitorCookieIssuerMiddleware(mgr *VisitorCookieManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("DEFENSE_VISITOR_COOKIE_ENABLED") == "false" || mgr == nil {
			c.Next()
			return
		}
		if !isFrontendDocumentRequest(c) {
			c.Next()
			return
		}
		// Cookie issuance is handled by the PoW visitor endpoints. This middleware
		// only keeps the existing first-request ordering for verification.
		c.Next()
	}
}

func HasValidVisitorCookie(c *gin.Context, mgr *VisitorCookieManager) bool {
	return checkVisitorCookie(c, mgr) == visitorCookieAllowed
}

func checkVisitorCookie(c *gin.Context, mgr *VisitorCookieManager) visitorCookieDecision {
	if mgr == nil || os.Getenv("DEFENSE_VISITOR_COOKIE_ENABLED") == "false" {
		return visitorCookieNone
	}
	cookieValue, err := c.Cookie(visitorCookieName)
	if err != nil || cookieValue == "" || !mgr.VerifyCookie(cookieValue) {
		return visitorCookieNone
	}
	if !mgr.AllowCookieRequest(c.Request.Context(), cookieValue) {
		return visitorCookieLimited
	}
	return visitorCookieAllowed
}
