package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestVisitorCookieHandlerIssuesCookieAfterPoW(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_POW_DIFFICULTY", "1")

	rdb, cleanup := newVisitorCookieHandlerTestRedis(t)
	defer cleanup()

	mgr := middleware.NewVisitorCookieManagerWithSecret(rdb, "visitor-handler-test-secret")
	credit := middleware.NewCookieCreditSystem(rdb)
	h := NewVisitorCookieHandler(mgr, credit, rdb)
	router := gin.New()
	router.POST("/api/public/visitor/challenge", h.Challenge)
	router.POST("/api/public/visitor/issue-cookie", h.IssueCookie)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/public/visitor/challenge", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var challenge struct {
		Challenge  string `json:"challenge"`
		Difficulty int    `json:"difficulty"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &challenge))
	require.NotEmpty(t, challenge.Challenge)
	require.Equal(t, 1, challenge.Difficulty)

	nonce := solvePoWForTest(challenge.Challenge, challenge.Difficulty)
	body, err := json.Marshal(map[string]string{
		"challenge":   challenge.Challenge,
		"nonce":       nonce,
		"fingerprint": "browser-fingerprint",
	})
	require.NoError(t, err)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/public/visitor/issue-cookie", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "_xdt_v", cookies[0].Name)
	require.False(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)
	require.True(t, mgr.VerifyCookieWithFingerprint(cookies[0].Value, "browser-fingerprint"))
	require.Equal(t, middleware.CreditDefault, credit.GetCredit(context.Background(), middleware.CookieHash(cookies[0].Value)))
}

func TestVisitorCookieHandlerRejectsBadPoWAndDeductsExistingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_POW_DIFFICULTY", "2")

	rdb, cleanup := newVisitorCookieHandlerTestRedis(t)
	defer cleanup()

	mgr := middleware.NewVisitorCookieManagerWithSecret(rdb, "visitor-handler-test-secret")
	credit := middleware.NewCookieCreditSystem(rdb)
	existing, _ := mgr.IssueCookieWithFingerprint("old-fingerprint")
	credit.Reset(context.Background(), middleware.CookieHash(existing))

	h := NewVisitorCookieHandler(mgr, credit, rdb)
	router := gin.New()
	router.POST("/api/public/visitor/challenge", h.Challenge)
	router.POST("/api/public/visitor/issue-cookie", h.IssueCookie)

	challenge := requestVisitorChallengeForTest(t, router, 2)
	body := []byte(`{"challenge":"` + challenge + `","nonce":"` + invalidPoWNonceForTest(challenge, 2) + `","fingerprint":"fp"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/public/visitor/issue-cookie", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_xdt_v", Value: existing})
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, 0, credit.GetCredit(context.Background(), middleware.CookieHash(existing)))
}

func TestVisitorCookieHandlerConsumesChallengeOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_POW_DIFFICULTY", "1")

	rdb, cleanup := newVisitorCookieHandlerTestRedis(t)
	defer cleanup()

	mgr := middleware.NewVisitorCookieManagerWithSecret(rdb, "visitor-handler-test-secret")
	credit := middleware.NewCookieCreditSystem(rdb)
	h := NewVisitorCookieHandler(mgr, credit, rdb)
	router := gin.New()
	router.POST("/api/public/visitor/challenge", h.Challenge)
	router.POST("/api/public/visitor/issue-cookie", h.IssueCookie)

	challenge := requestVisitorChallengeForTest(t, router, 1)
	nonce := solvePoWForTest(challenge, 1)
	body, err := json.Marshal(map[string]string{
		"challenge":   challenge,
		"nonce":       nonce,
		"fingerprint": "browser-fingerprint",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/public/visitor/issue-cookie", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/public/visitor/issue-cookie", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid or expired challenge")
}

func requestVisitorChallengeForTest(t *testing.T, router *gin.Engine, difficulty int) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/public/visitor/challenge", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var challenge struct {
		Challenge  string `json:"challenge"`
		Difficulty int    `json:"difficulty"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &challenge))
	require.NotEmpty(t, challenge.Challenge)
	require.Equal(t, difficulty, challenge.Difficulty)
	return challenge.Challenge
}

func solvePoWForTest(challenge string, difficulty int) string {
	prefix := strings.Repeat("0", difficulty)
	for nonce := 0; ; nonce++ {
		sum := sha256.Sum256([]byte(challenge + strconv.Itoa(nonce)))
		if strings.HasPrefix(hex.EncodeToString(sum[:]), prefix) {
			return strconv.Itoa(nonce)
		}
	}
}

func invalidPoWNonceForTest(challenge string, difficulty int) string {
	prefix := strings.Repeat("0", difficulty)
	for nonce := 0; ; nonce++ {
		candidate := "bad-" + strconv.Itoa(nonce)
		sum := sha256.Sum256([]byte(challenge + candidate))
		if !strings.HasPrefix(hex.EncodeToString(sum[:]), prefix) {
			return candidate
		}
	}
}

func newVisitorCookieHandlerTestRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return rdb, func() {
		_ = rdb.Close()
		s.Close()
	}
}
