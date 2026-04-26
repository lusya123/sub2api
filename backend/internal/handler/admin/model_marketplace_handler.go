package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ModelMarketplaceHandler struct {
	service *service.ModelMarketplaceService
	status  *service.StatusPageService
}

func NewModelMarketplaceHandler(service *service.ModelMarketplaceService, status *service.StatusPageService) *ModelMarketplaceHandler {
	return &ModelMarketplaceHandler{service: service, status: status}
}

// List handles listing the currently callable model catalog with pricing.
// GET /api/v1/admin/model-marketplace
func (h *ModelMarketplaceHandler) List(c *gin.Context) {
	if h == nil || h.service == nil {
		response.Success(c, service.ModelMarketplaceResponse{Models: []service.ModelMarketplaceItem{}})
		return
	}
	result, err := h.service.ListModels(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetPublicStatusConfig returns the admin-editable public status config plus
// current group suggestions.
// GET /api/v1/admin/model-marketplace/status-config
func (h *ModelMarketplaceHandler) GetPublicStatusConfig(c *gin.Context) {
	if h == nil || h.status == nil {
		response.InternalError(c, "status service not available")
		return
	}
	result, err := h.status.GetPublicStatusConfigAdmin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// UpdatePublicStatusConfig persists the public status config.
// PUT /api/v1/admin/model-marketplace/status-config
func (h *ModelMarketplaceHandler) UpdatePublicStatusConfig(c *gin.Context) {
	if h == nil || h.status == nil {
		response.InternalError(c, "status service not available")
		return
	}
	var req service.PublicStatusConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.status.SetPublicStatusConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
