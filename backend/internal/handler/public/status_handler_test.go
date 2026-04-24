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

// TestListModels_ResponseShape guards the public contract: the JSON must be a
// top-level object with a `models` key whose value is an array. The frontend
// StatusView reads `res.data.models`; a regression back to a bare array would
// crash the page on first load.
func TestListModels_ResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newTestEntClient(t)

	// Seed one group with two concrete routing keys so ListModels has data.
	_, err := client.Group.Create().
		SetName("status-handler-test-group").
		SetRateMultiplier(1.0).
		SetModelRouting(map[string][]int64{
			"claude-opus-4-7":   nil,
			"claude-sonnet-4-6": nil,
		}).
		Save(context.Background())
	require.NoError(t, err)

	svc := service.NewStatusPageService(client)
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
