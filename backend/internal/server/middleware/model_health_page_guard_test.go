package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelHealthPageSettingRepo struct {
	values map[string]string
	err    error
}

func (r *modelHealthPageSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (r *modelHealthPageSettingRepo) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (r *modelHealthPageSettingRepo) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (r *modelHealthPageSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *modelHealthPageSettingRepo) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (r *modelHealthPageSettingRepo) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (r *modelHealthPageSettingRepo) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestModelHealthPageGuard(t *testing.T) {
	tests := []struct {
		name       string
		nilService bool
		value      string
		readErr    error
		wantStatus int
	}{
		{name: "nil service allows", nilService: true, wantStatus: http.StatusOK},
		{name: "missing setting allows", wantStatus: http.StatusOK},
		{name: "true allows", value: "true", wantStatus: http.StatusOK},
		{name: "false blocks", value: "false", wantStatus: http.StatusForbidden},
		{name: "false with whitespace blocks", value: " FALSE ", wantStatus: http.StatusForbidden},
		{name: "read error allows", readErr: context.DeadlineExceeded, wantStatus: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			var settingService *service.SettingService
			if !tc.nilService {
				values := map[string]string{}
				if tc.value != "" {
					values[service.SettingKeyModelHealthPageEnabled] = tc.value
				}
				settingService = service.NewSettingService(&modelHealthPageSettingRepo{
					values: values,
					err:    tc.readErr,
				}, &config.Config{})
			}

			router := gin.New()
			router.Use(ModelHealthPageGuard(settingService))
			router.GET("/model-marketplace", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/model-marketplace", nil)
			router.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}
