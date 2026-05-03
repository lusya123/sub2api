package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operationHandlerRepoStub struct {
	calls atomic.Int32
}

func (r *operationHandlerRepoStub) GetOperationAnalyticsSnapshot(ctx context.Context, filter service.OperationAnalyticsFilter) (*service.OperationAnalyticsSnapshot, error) {
	r.calls.Add(1)
	return &service.OperationAnalyticsSnapshot{
		StartTime:   filter.StartTime.Format(time.RFC3339),
		EndTime:     filter.EndTime.Format(time.RFC3339),
		Granularity: filter.Granularity,
		Timezone:    filter.Timezone,
		ModuleStatuses: map[string]string{
			"modules": strings.Join(filter.Modules, ","),
		},
		Core: service.OperationCoreMetrics{
			ActiveUsers: 1,
		},
	}, nil
}

func newOperationHandlerTestRouter(repo *operationHandlerRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := service.NewOperationAnalyticsService(repo)
	handler := NewOperationHandler(svc)
	router.GET("/admin/operations/snapshot", handler.GetSnapshot)
	return router
}

func TestOperationHandler_GetSnapshot_DefaultsAndCache(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	require.Equal(t, int32(1), repo.calls.Load())
}

func TestOperationHandler_GetSnapshot_InvalidGranularity(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?granularity=week", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOperationHandler_GetSnapshot_InvalidDate(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?start_date=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOperationHandler_GetSnapshot_InvalidTimezone(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?timezone=Invalid/Timezone", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, int32(0), repo.calls.Load())
}

func TestOperationHandler_GetSnapshot_InvalidModules(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?modules=core,unknown", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, int32(0), repo.calls.Load())
}

func TestOperationHandler_GetSnapshot_AllRangeUsesDailyGranularity(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?range=all&granularity=hour&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"granularity":"day"`)
	require.Contains(t, rec.Body.String(), `"modules":"core"`)
}

func TestOperationHandler_GetSnapshot_ParsesModules(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?modules=financial,product_matrix&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"modules":"financial,product_matrix"`)
}

func TestOperationHandler_GetSnapshot_AllowsHalfYearRange(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?start_date=2026-01-01&end_date=2026-06-29&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestOperationHandler_GetSnapshot_RejectsRangeBeyondLimit(t *testing.T) {
	repo := &operationHandlerRepoStub{}
	router := newOperationHandlerTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/operations/snapshot?start_date=2025-01-01&end_date=2026-06-01&timezone=Asia/Shanghai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
