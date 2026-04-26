package public

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// newTestEntClient mirrors the pattern used in internal/service tests — an
// in-memory SQLite database with the ent schema migrated.
func newTestEntClient(t *testing.T) *dbent.Client {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func testPublicStatusConfig(models []string, groupIDs ...int64) service.PublicStatusConfig {
	cfg := service.PublicStatusConfig{
		Models: make([]service.PublicStatusModelConfig, 0, len(models)),
		Groups: make([]service.PublicStatusGroupConfig, 0, len(groupIDs)),
	}
	for _, name := range models {
		cfg.Models = append(cfg.Models, service.PublicStatusModelConfig{
			Name:          name,
			Provider:      "ANTHROPIC",
			PromptCaching: true,
			Enabled:       true,
		})
	}
	for _, id := range groupIDs {
		cfg.Groups = append(cfg.Groups, service.PublicStatusGroupConfig{
			GroupID:     id,
			Enabled:     true,
			DisplayName: "Public Group",
		})
	}
	return cfg
}

func seedMonitorableStatusGroup(t *testing.T, client *dbent.Client, name string) int64 {
	t.Helper()
	account, err := client.Account.Create().
		SetName(name + "-account").
		SetPlatform("anthropic").
		SetType("oauth").
		SetCredentials(map[string]interface{}{}).
		SetExtra(map[string]interface{}{}).
		SetConcurrency(1).
		SetPriority(50).
		SetRateMultiplier(1.0).
		SetStatus("active").
		SetAutoPauseOnExpired(false).
		SetSchedulable(true).
		Save(context.Background())
	require.NoError(t, err)
	group, err := client.Group.Create().
		SetName(name).
		SetRateMultiplier(1.0).
		SetModelRouting(map[string][]int64{}).
		Save(context.Background())
	require.NoError(t, err)
	_, err = client.AccountGroup.Create().
		SetAccountID(account.ID).
		SetGroupID(group.ID).
		Save(context.Background())
	require.NoError(t, err)
	return group.ID
}

// TestListModels_ResponseShape guards the public contract: the JSON must be a
// top-level object with a `models` key whose value is an array. The frontend
// StatusView reads `res.data.models`; a regression back to a bare array would
// crash the page on first load.
func TestListModels_ResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)

	// Seed one group with a schedulable account so ListModels has monitorable data.
	groupID := seedMonitorableStatusGroup(t, client, "status-handler-test-group")

	svc := service.NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7", "claude-sonnet-4-6"}, groupID))
	h := NewPublicStatusHandler(svc)

	r := gin.New()
	r.GET("/api/public/status/models", h.ListModels)

	req := httptest.NewRequest(http.MethodGet, "/api/public/status/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "expected 200, got body=%s", w.Body.String())

	// Decode into a raw map to assert the top-level shape is an object, not
	// an array. json.Unmarshal into a slice would also work for bare-array,
	// so decoding into map[string]any is the load-bearing assertion.
	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
	raw, ok := decoded["models"]
	require.True(t, ok, "response must have top-level `models` key; got body=%s", w.Body.String())

	var models []service.StatusModel
	require.NoError(t, json.Unmarshal(raw, &models))
	require.GreaterOrEqual(t, len(models), 2)
}

// TestGetModelDetail_InputValidation guards the :name path parameter against
// the categories of hostile input we see in real DoS traffic: oversized
// blobs, HTML injection probes, null bytes, exotic characters. All must
// fail fast at the handler with 400 before service-layer DB queries.
func TestGetModelDetail_InputValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)
	svc := service.NewStatusPageService(client)
	h := NewPublicStatusHandler(svc)

	r := gin.New()
	r.GET("/api/public/status/model/:name", h.GetModelDetail)

	cases := []struct {
		label string
		name  string
	}{
		{"too_long", strings.Repeat("a", 129)},
		{"html_tag", "claude<script>"},
		{"null_byte", "claude\x00opus"},
		{"cjk", "模型名"},
		{"whitespace", "claude opus"},
		{"semicolon", "claude;drop"},
		{"backtick", "claude`rm"},
	}
	// Path-traversal sequences that include `/` never reach this handler
	// because gin's router treats `/` as a separator — those already 404 at
	// the router layer, a stricter (safer) outcome.

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/public/status/model/"+pathEscape(tc.name), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code,
				"case=%s body=%s", tc.label, w.Body.String())
		})
	}
}

// TestGetModelDetail_UnknownModelIs404 verifies the unknown-model fast-path
// returns 404 (not 500) when the model name is well-formed but not routed in
// any group.
func TestGetModelDetail_UnknownModelIs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)
	g, err := client.Group.Create().
		SetName("handler-unknown-test-group").
		SetRateMultiplier(1.0).
		SetModelRouting(map[string][]int64{}).
		Save(context.Background())
	require.NoError(t, err)

	svc := service.NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, g.ID))
	h := NewPublicStatusHandler(svc)

	r := gin.New()
	r.GET("/api/public/status/model/:name", h.GetModelDetail)

	req := httptest.NewRequest(http.MethodGet, "/api/public/status/model/no-such-model-exists", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "unknown model must 404, got body=%s", w.Body.String())
}

// pathEscape is a tiny URL-path escaper used only by the validation test
// suite. Keeps test inputs readable while still reaching the handler with
// bytes that would otherwise be rejected by the HTTP parser.
func pathEscape(s string) string {
	s = strings.ReplaceAll(s, "/", "%2F")
	s = strings.ReplaceAll(s, "\x00", "%00")
	s = strings.ReplaceAll(s, "<", "%3C")
	s = strings.ReplaceAll(s, ">", "%3E")
	s = strings.ReplaceAll(s, " ", "%20")
	return s
}

// TestListAndDetail_CacheControlHeaders: both public endpoints must emit the
// 30s Cache-Control header so reverse proxies / CDN layers can ride along.
func TestListAndDetail_CacheControlHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)
	groupID := seedMonitorableStatusGroup(t, client, "handler-cc-test-group")

	svc := service.NewStatusPageService(client).
		WithPublicStatusConfig(testPublicStatusConfig([]string{"claude-opus-4-7"}, groupID))
	h := NewPublicStatusHandler(svc)

	r := gin.New()
	r.GET("/api/public/status/models", h.ListModels)
	r.GET("/api/public/status/model/:name", h.GetModelDetail)

	// list
	req := httptest.NewRequest(http.MethodGet, "/api/public/status/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "public, max-age=30", w.Header().Get("Cache-Control"))
	require.Equal(t, "Accept-Language", w.Header().Get("Vary"))

	// detail
	req = httptest.NewRequest(http.MethodGet, "/api/public/status/model/claude-opus-4-7", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "public, max-age=30", w.Header().Get("Cache-Control"))
	require.Equal(t, "Accept-Language", w.Header().Get("Vary"))
}

// TestListModels_EmptyStillWrapsObject: zero models must still serialise as
// `{"models": []}`, not `null` or a bare `[]`.
func TestListModels_EmptyStillWrapsObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)

	svc := service.NewStatusPageService(client)
	h := NewPublicStatusHandler(svc)

	r := gin.New()
	r.GET("/api/public/status/models", h.ListModels)

	req := httptest.NewRequest(http.MethodGet, "/api/public/status/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
	raw, ok := decoded["models"]
	require.True(t, ok)
	var models []service.StatusModel
	require.NoError(t, json.Unmarshal(raw, &models))
	require.Equal(t, 0, len(models))
}
