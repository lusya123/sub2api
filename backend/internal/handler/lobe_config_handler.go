package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type LobeConfigHandler struct {
	lobeConfigService *service.LobeConfigService
}

func NewLobeConfigHandler(lobeConfigService *service.LobeConfigService) *LobeConfigHandler {
	return &LobeConfigHandler{lobeConfigService: lobeConfigService}
}

func (h *LobeConfigHandler) GetUserConfig(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	cfg, err := h.lobeConfigService.GetUserConfig(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(200, cfg)
}
