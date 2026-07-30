package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// LotteryHandler handles admin lottery prize-pool operations.
type LotteryHandler struct {
	lotteryService *service.LotteryService
}

func NewLotteryHandler(lotteryService *service.LotteryService) *LotteryHandler {
	return &LotteryHandler{lotteryService: lotteryService}
}

type lotteryPoolCodeIDsRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// GetPool returns prize-pool inventory grouped by prize value.
// GET /api/v1/admin/lottery/pool
func (h *LotteryHandler) GetPool(c *gin.Context) {
	summary, err := h.lotteryService.GetPoolSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

// BindPoolCodes binds eligible unused balance redeem codes into the prize pool.
// POST /api/v1/admin/lottery/pool/bind
func (h *LotteryHandler) BindPoolCodes(c *gin.Context) {
	var req lotteryPoolCodeIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	bound, err := h.lotteryService.BindPrizeCodes(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"bound": bound})
}

// UnbindPoolCodes removes still-available prize-pool bindings.
// POST /api/v1/admin/lottery/pool/unbind
func (h *LotteryHandler) UnbindPoolCodes(c *gin.Context) {
	var req lotteryPoolCodeIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	unbound, err := h.lotteryService.UnbindPrizeCodes(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"unbound": unbound})
}
