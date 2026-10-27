package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type MediaQuoteHandler struct {
	quotes *service.MediaQuoteService
}

func NewMediaQuoteHandler(quotes *service.MediaQuoteService) *MediaQuoteHandler {
	return &MediaQuoteHandler{quotes: quotes}
}

// Create returns a non-binding media generation estimate using the caller's current billing snapshot.
func (h *MediaQuoteHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var request service.MediaQuoteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	quote, err := h.quotes.Quote(c.Request.Context(), subject.UserID, request)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}
