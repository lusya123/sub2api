package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBatchImageListRejectsUnknownStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewBatchImageHandler(&service.BatchImagePublicService{}, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 22, UserID: 11})
		c.Next()
	})
	router.GET("/v1/images/batches", h.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/images/batches?status=not-a-real-status", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "BATCH_IMAGE_INVALID_STATUS", got.Error.Code)
}
