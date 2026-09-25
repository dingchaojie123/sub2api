package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type defaultAPIKeyResponse struct {
	Purpose string      `json:"purpose"`
	APIKey  *dto.APIKey `json:"api_key"`
}

// GetDefaults initializes missing purposes before returning the user's credentials.
func (h *APIKeyHandler) GetDefaults(c *gin.Context) {
	h.defaultKeys(c)
}

func (h *APIKeyHandler) EnsureDefaults(c *gin.Context) {
	h.defaultKeys(c)
}

func (h *APIKeyHandler) GetDefault(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	item, err := h.apiKeyService.GetDefaultAPIKey(c.Request.Context(), subject.UserID, c.Param("purpose"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, defaultAPIKeyResponse{Purpose: item.Purpose, APIKey: dto.APIKeyFromService(item.APIKey)})
}

type updateDefaultAPIKeyRequest struct {
	APIKeyID int64 `json:"api_key_id" binding:"required,gt=0"`
}

// UpdateDefault assigns an existing customer-owned key to one default purpose.
func (h *APIKeyHandler) UpdateDefault(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req updateDefaultAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.apiKeyService.UpdateDefaultAPIKey(c.Request.Context(), subject.UserID, c.Param("purpose"), req.APIKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, defaultAPIKeyResponse{Purpose: item.Purpose, APIKey: dto.APIKeyFromService(item.APIKey)})
}

func (h *APIKeyHandler) defaultKeys(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keys, err := h.apiKeyService.EnsureDefaultAPIKeys(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]defaultAPIKeyResponse, 0, len(keys))
	for _, item := range keys {
		items = append(items, defaultAPIKeyResponse{Purpose: item.Purpose, APIKey: dto.APIKeyFromService(item.APIKey)})
	}
	response.Success(c, items)
}
