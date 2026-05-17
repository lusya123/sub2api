package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var knownAttackPayloads = []string{
	`"123123123123@qq.com"`,
	`"123456@qq.com"`,
	`"admin@admin.com"`,
	`123123123123`,
}

var suspiciousUAs = []string{
	"sqlmap", "nikto", "nmap", "masscan", "zmeu",
}

var attackPathPatterns = []string{
	"/wp-", "/.git", "/.env", "/.svn",
	"/phpmyadmin", "/admin.php", "/wp-login",
	"%00", "../", "%2e%2e/", "%3cscript", "%3e",
	"/shell.php", "/cmd.aspx",
}

var honeypotPaths = []string{
	"/.env", "/.git/config", "/wp-admin", "/wp-login.php",
	"/.aws/credentials", "/phpmyadmin", "/phpinfo.php",
}

type AttackDetector struct {
	rdb    *redis.Client
	credit *CookieCreditSystem
}

func NewAttackDetector(rdb *redis.Client, credit *CookieCreditSystem) *AttackDetector {
	return &AttackDetector{rdb: rdb, credit: credit}
}

func (a *AttackDetector) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("DEFENSE_ATTACK_DETECTOR_ENABLED") == "false" {
			c.Next()
			return
		}
		if c == nil || c.Request == nil || c.Request.URL == nil {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if IsGatewayAPIPath(path) ||
			GetTrustTier(c) == TierAPIKey ||
			GetTrustTier(c) == TierUser ||
			strings.HasPrefix(path, "/api/public/visitor/") {
			c.Next()
			return
		}

		cookieValue, _ := c.Cookie(visitorCookieName)
		cookieHash := ""
		if cookieValue != "" {
			cookieHash = CookieHash(cookieValue)
		}
		deduct := func(delta int, reason string) {
			if cookieHash != "" && a != nil && a.credit != nil {
				a.credit.ChangeCredit(c.Request.Context(), cookieHash, delta, reason)
			}
		}

		for _, h := range honeypotPaths {
			if path == h {
				deduct(CreditChangeHoneypot, "honeypot:"+path)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}

		pathLower := strings.ToLower(path)
		for _, p := range attackPathPatterns {
			if strings.Contains(pathLower, p) {
				deduct(CreditChangeScannerPath, "scanner:"+p)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}

		uaLower := strings.ToLower(c.GetHeader("User-Agent"))
		for _, s := range suspiciousUAs {
			if strings.Contains(uaLower, s) {
				deduct(CreditChangeScannerPath, "scanner_ua:"+s)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}

		if c.Request.Method == http.MethodPost && c.Request.ContentLength > 0 && c.Request.ContentLength < 4096 {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
				bodyStr := string(body)
				for _, p := range knownAttackPayloads {
					if strings.Contains(bodyStr, p) {
						deduct(CreditChangeAttackPayload, "payload:"+p)
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
						return
					}
				}

				if len(body) > 50 && a != nil && a.rdb != nil {
					h := sha256.Sum256(body)
					bodyKey := "atk:body:" + hex.EncodeToString(h[:8])
					cnt, _ := a.rdb.Incr(c.Request.Context(), bodyKey).Result()
					if cnt == 1 {
						_ = a.rdb.Expire(c.Request.Context(), bodyKey, time.Minute).Err()
					}
					if cnt > 50 {
						deduct(CreditChangeAttackPayload, "body_replay")
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
						return
					}
				}
			}
		}

		c.Next()
	}
}
