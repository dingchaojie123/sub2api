package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// organizationDateRange 解析 ?start=YYYY-MM-DD&end=YYYY-MM-DD（含 end 当天），默认最近 30 天。
func organizationDateRange(c *gin.Context) (time.Time, time.Time, bool) {
	now := time.Now()
	start := now.AddDate(0, 0, -30)
	end := now.Add(24 * time.Hour)
	if raw := c.Query("start"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.BadRequest(c, "Invalid start date")
			return start, end, false
		}
		start = parsed
	}
	if raw := c.Query("end"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			response.BadRequest(c, "Invalid end date")
			return start, end, false
		}
		end = parsed.Add(24 * time.Hour)
	}
	if !end.After(start) {
		response.BadRequest(c, "End date must be after start date")
		return start, end, false
	}
	return start, end, true
}

// MemberUsage 某个成员按模型拆分的用量（仅超级管理员 / 管理员）。
// GET /api/v1/organizations/member-usage?organization_id=&user_id=&start=&end=
func (h *OrganizationHandler) MemberUsage(c *gin.Context) {
	actorID, ok := organizationSubject(c)
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
	start, end, ok := organizationDateRange(c)
	if !ok {
		return
	}
	item, err := h.organizations.MemberUsageDetail(c.Request.Context(), orgID, actorID, memberID, start, end)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Success(c, item)
}

// MemberTasks 某个成员提交的视频 / 批量图片任务（含提示词），分页（仅超级管理员 / 管理员）。
// GET /api/v1/organizations/member-tasks?organization_id=&user_id=&start=&end=&page=&page_size=
func (h *OrganizationHandler) MemberTasks(c *gin.Context) {
	actorID, ok := organizationSubject(c)
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
	start, end, ok := organizationDateRange(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.organizations.MemberTasks(c.Request.Context(), orgID, actorID, memberID, start, end, pageSize, (page-1)*pageSize)
	if err != nil {
		writeOrganizationError(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}
