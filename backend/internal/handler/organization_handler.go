package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type OrganizationHandler struct {
	organizations *service.OrganizationService
	apiKeys       *service.APIKeyService
}

func NewOrganizationHandler(organizations *service.OrganizationService, apiKeys *service.APIKeyService) *OrganizationHandler {
	return &OrganizationHandler{organizations: organizations, apiKeys: apiKeys}
}

func organizationSubject(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func organizationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Query("organization_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid organization_id query parameter")
		return 0, false
	}
	return id, true
}

func organizationMemberUserID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid user_id query parameter")
		return 0, false
	}
	return id, true
}

func writeOrganizationError(c *gin.Context, err error) {
	response.ErrorFrom(c, err)
}

func (h *OrganizationHandler) List(c *gin.Context) {
	if strings.TrimSpace(c.Query("organization_id")) != "" {
		h.Get(c)
		return
	}
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	items, err := h.organizations.List(c.Request.Context(), userID)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, items)
}

func (h *OrganizationHandler) Create(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	var req struct {
		Name      string `json:"name"`
		SeatLimit int    `json:"seat_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	item, err := h.organizations.Create(c.Request.Context(), userID, service.CreateOrganizationInput{Name: req.Name, SeatLimit: req.SeatLimit})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *OrganizationHandler) Delete(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	orgID, ok := organizationID(c)
	if !ok {
		return
	}
	keys, err := h.organizations.Delete(c.Request.Context(), orgID, userID)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	for _, key := range keys {
		h.apiKeys.InvalidateAuthCacheByKey(c.Request.Context(), key)
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *OrganizationHandler) Get(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	orgID, ok := organizationID(c)
	if !ok {
		return
	}
	item, err := h.organizations.Get(c.Request.Context(), orgID, userID)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *OrganizationHandler) Members(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	orgID, ok := organizationID(c)
	if !ok {
		return
	}
	items, err := h.organizations.Members(c.Request.Context(), orgID, userID)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, items)
}

func (h *OrganizationHandler) CreateInvitation(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	var req struct {
		OrganizationID int64  `json:"organization_id"`
		Email          string `json:"email"`
		Role           string `json:"role"`
		MaxUses        int    `json:"max_uses"`
		ExpiresInHours int    `json:"expires_in_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if req.OrganizationID <= 0 {
		response.BadRequest(c, "Invalid organization_id")
		return
	}
	if req.ExpiresInHours <= 0 {
		req.ExpiresInHours = 168
	}
	item, err := h.organizations.CreateInvitation(c.Request.Context(), req.OrganizationID, userID, service.CreateOrganizationInvitationInput{Email: req.Email, Role: req.Role, MaxUses: req.MaxUses, ExpiresAt: time.Now().Add(time.Duration(req.ExpiresInHours) * time.Hour)})
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *OrganizationHandler) Invitations(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	orgID, ok := organizationID(c)
	if !ok {
		return
	}
	items, err := h.organizations.Invitations(c.Request.Context(), orgID, userID)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, items)
}

func (h *OrganizationHandler) AcceptInvitation(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		response.BadRequest(c, "Invitation token is required")
		return
	}
	item, err := h.organizations.AcceptInvitation(c.Request.Context(), userID, strings.TrimSpace(req.Token))
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *OrganizationHandler) UpdateMember(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	var req struct {
		OrganizationID int64    `json:"organization_id"`
		MemberUserID   int64    `json:"user_id"`
		Role           *string  `json:"role"`
		Status         *string  `json:"status"`
		MonthlyLimit   *float64 `json:"monthly_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if req.OrganizationID <= 0 || req.MemberUserID <= 0 {
		response.BadRequest(c, "Invalid organization_id or user_id")
		return
	}
	if err := h.organizations.UpdateMember(c.Request.Context(), req.OrganizationID, userID, req.MemberUserID, req.Role, req.Status, req.MonthlyLimit); err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

func (h *OrganizationHandler) RemoveMember(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	orgID, ok := organizationID(c)
	if !ok {
		return
	}
	memberID, ok := organizationMemberUserID(c)
	if !ok {
		return
	}
	keys, err := h.organizations.RemoveMember(c.Request.Context(), orgID, userID, memberID)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	for _, key := range keys {
		h.apiKeys.InvalidateAuthCacheByKey(c.Request.Context(), key)
	}
	response.Success(c, gin.H{"removed": true})
}

func (h *OrganizationHandler) Fund(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	var req struct {
		OrganizationID int64   `json:"organization_id"`
		Amount         float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if req.OrganizationID <= 0 {
		response.BadRequest(c, "Invalid organization_id")
		return
	}
	item, err := h.organizations.Fund(c.Request.Context(), req.OrganizationID, userID, req.Amount)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *OrganizationHandler) Usage(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	orgID, ok := organizationID(c)
	if !ok {
		return
	}
	now := time.Now()
	start := now.AddDate(0, 0, -30)
	end := now.Add(24 * time.Hour)
	if raw := c.Query("start"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.BadRequest(c, "Invalid start date")
			return
		}
		start = parsed
	}
	if raw := c.Query("end"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.BadRequest(c, "Invalid end date")
			return
		}
		end = parsed.Add(24 * time.Hour)
	}
	if !end.After(start) {
		response.BadRequest(c, "End date must be after start date")
		return
	}
	item, err := h.organizations.UsageReport(c.Request.Context(), orgID, userID, start, end)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *OrganizationHandler) AuditLogs(c *gin.Context) {
	userID, ok := organizationSubject(c)
	if !ok {
		return
	}
	orgID, ok := organizationID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.organizations.AuditLogs(c.Request.Context(), orgID, userID, limit)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, items)
}
