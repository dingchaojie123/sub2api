package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetOrganizationMemberVideoContent 让团队的超级管理员 / 管理员在「成员明细」里播放、下载成员的视频成果。
//
//	GET /api/v1/organizations/member-tasks/content?organization_id=&user_id=&task_id=
//
// 只有「提交时计费给该团队、计费成员是 user_id」的已成功任务可取，操作者必须是该团队在职的超级管理员 /
// 管理员（这两条都在仓储的同一条查询里校验），其余一律 404。取流方式与用户端「视频任务」页相同。
func (h *OpenAIGatewayHandler) GetOrganizationMemberVideoContent(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	orgID, err := strconv.ParseInt(c.Query("organization_id"), 10, 64)
	if err != nil || orgID <= 0 {
		response.BadRequest(c, "Invalid organization_id query parameter")
		return
	}
	memberID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || memberID <= 0 {
		response.BadRequest(c, "Invalid user_id query parameter")
		return
	}
	taskID := strings.TrimSpace(c.Query("task_id"))
	if taskID == "" {
		response.BadRequest(c, "task_id is required")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.pp_video_organization_tasks",
		zap.Int64("actor_user_id", subject.UserID),
		zap.Int64("organization_id", orgID),
		zap.Int64("member_user_id", memberID),
		zap.String("task", taskID),
	)
	task, err := h.gatewayService.GetPPVideoTaskForOrganizationAdmin(c.Request.Context(), orgID, subject.UserID, memberID, taskID)
	if err != nil || task == nil || !service.PPVideoTaskContentAvailable(task.Platform, task.Status, task.BillingStatus) {
		response.NotFound(c, "Video content is not available")
		return
	}
	h.streamPPVideoTaskContent(c, reqLog, task)
}
