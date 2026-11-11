package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// EnterpriseHandler 企业账号：用户侧（团队空间访问权限、升级、修改企业信息）与后台审核。
type EnterpriseHandler struct {
	enterprise *service.EnterpriseService
}

func NewEnterpriseHandler(enterprise *service.EnterpriseService) *EnterpriseHandler {
	return &EnterpriseHandler{enterprise: enterprise}
}

func enterpriseSubject(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

// GetWorkspaceAccess 返回当前账号类型、企业信息及团队空间可见性。
// GET /api/v1/user/workspace-access
func (h *EnterpriseHandler) GetWorkspaceAccess(c *gin.Context) {
	userID, ok := enterpriseSubject(c)
	if !ok {
		return
	}
	access, err := h.enterprise.WorkspaceAccess(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, access)
}

// Upgrade 个人账号升级为企业账号。
// POST /api/v1/user/enterprise
func (h *EnterpriseHandler) Upgrade(c *gin.Context) {
	userID, ok := enterpriseSubject(c)
	if !ok {
		return
	}
	var req service.EnterpriseInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	profile, err := h.enterprise.CreateProfile(c.Request.Context(), userID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, profile)
}

// Update 修改企业信息（已认证后公司名称、信用代码不可改）。
// PUT /api/v1/user/enterprise
func (h *EnterpriseHandler) Update(c *gin.Context) {
	userID, ok := enterpriseSubject(c)
	if !ok {
		return
	}
	var req service.EnterpriseInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	profile, err := h.enterprise.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

// AdminList 后台企业审核列表。
// GET /api/v1/admin/enterprises?status=&search=&page=&page_size=
func (h *EnterpriseHandler) AdminList(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))
	switch status {
	case "", service.EnterpriseStatusPending, service.EnterpriseStatusApproved,
		service.EnterpriseStatusRejected, service.EnterpriseStatusIncomplete:
	default:
		response.BadRequest(c, "Invalid status")
		return
	}
	items, total, err := h.enterprise.AdminList(c.Request.Context(), service.EnterpriseAdminFilter{
		Status: status,
		Search: c.Query("search"),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// AdminReview 后台审核：通过或驳回。
// POST /api/v1/admin/enterprises/:user_id/review  {action: approve|reject, reason}
func (h *EnterpriseHandler) AdminReview(c *gin.Context) {
	reviewerID, ok := enterpriseSubject(c)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	profile, err := h.enterprise.Review(c.Request.Context(), userID, reviewerID, req.Action, req.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}
