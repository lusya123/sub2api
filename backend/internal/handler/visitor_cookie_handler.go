package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type VisitorCookieHandler struct {
	mgr    *middleware.VisitorCookieManager
	credit *middleware.CookieCreditSystem
}

func NewVisitorCookieHandler(mgr *middleware.VisitorCookieManager, credit *middleware.CookieCreditSystem) *VisitorCookieHandler {
	return &VisitorCookieHandler{mgr: mgr, credit: credit}
}

func (h *VisitorCookieHandler) Challenge(c *gin.Context) {
	difficulty := middleware.DefenseEnvInt("DEFENSE_POW_DIFFICULTY", 4)
	if c.Query("recover") == "1" {
		difficulty = middleware.DefenseEnvInt("DEFENSE_POW_DIFFICULTY_RECOVER", 5)
	}
	challenge := strconv.FormatInt(time.Now().UnixNano(), 36) + "." + randomChallengeString(16)
	if h != nil && h.mgr != nil {
		h.mgr.StoreChallenge(c.Request.Context(), challenge, time.Minute)
	}

	c.JSON(http.StatusOK, gin.H{
		"challenge":  challenge,
		"difficulty": difficulty,
		"expires_in": 60,
	})
}

func (h *VisitorCookieHandler) IssueCookie(c *gin.Context) {
	if h == nil || h.mgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "visitor cookie unavailable"})
		return
	}
	var req struct {
		Challenge   string `json:"challenge"`
		Nonce       string `json:"nonce"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	if !validPoWChallenge(req.Challenge) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge"})
		return
	}
	if !h.mgr.AllowIssueRequest(c.Request.Context(), req.Fingerprint) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "visitor cookie issue rate limited"})
		return
	}
	consumed, err := h.mgr.ConsumeChallenge(c.Request.Context(), req.Challenge)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "challenge unavailable"})
		return
	}
	if !consumed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired challenge"})
		return
	}

	difficulty := middleware.DefenseEnvInt("DEFENSE_POW_DIFFICULTY", 4)
	if c.Query("recover") == "1" {
		difficulty = middleware.DefenseEnvInt("DEFENSE_POW_DIFFICULTY_RECOVER", 5)
	}
	prefix := strings.Repeat("0", difficulty)
	hash := sha256.Sum256([]byte(req.Challenge + req.Nonce))
	if !strings.HasPrefix(hex.EncodeToString(hash[:]), prefix) {
		if existing, err := c.Cookie("_xdt_v"); err == nil && existing != "" && h.credit != nil {
			h.credit.ChangeCredit(c.Request.Context(), middleware.CookieHash(existing), middleware.CreditChangePoWBypass, "pow_bypass")
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid proof of work"})
		return
	}

	value, ttl := h.mgr.IssueCookieWithFingerprint(req.Fingerprint)
	if h.credit != nil {
		h.credit.Reset(c.Request.Context(), middleware.CookieHash(value))
	}
	c.SetCookie("_xdt_v", value, ttl, "/", "", true, false)
	if strings.TrimSpace(req.Fingerprint) != "" {
		c.SetCookie("_xdt_fp", strings.TrimSpace(req.Fingerprint), ttl, "/", "", true, false)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func validPoWChallenge(challenge string) bool {
	parts := strings.SplitN(challenge, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	tsInt, err := strconv.ParseInt(parts[0], 36, 64)
	if err != nil {
		return false
	}
	age := time.Now().UnixNano() - tsInt
	return age >= 0 && age <= int64(time.Minute)
}

func randomChallengeString(n int) string {
	var b = make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)[:n]
}
