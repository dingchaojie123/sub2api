package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 管理后台「视频任务」接口（管理员鉴权）。
//
//	GET /api/v1/admin/video-tasks               列表（筛选 + 分页，跨用户）
//	GET /api/v1/admin/video-tasks/:id           详情
//	GET /api/v1/admin/video-tasks/:id/content   视频内容（播放 / 下载）
//
// 展示模型沿用用户端的 userVideoTask，并附带任务所属用户、上游账号与分组。
// 同样不返回上游原始响应与轮询租约等内部信息。

// adminVideoTask 是管理端任务视图。
type adminVideoTask struct {
	userVideoTask

	// UpstreamTaskID 是上游任务 ID；任务尚未拿到上游 ID 时为空。
	UpstreamTaskID string `json:"upstream_task_id"`
	Platform       string `json:"platform"`
	BillingStatus  string `json:"billing_status"`
	UserID         int64  `json:"user_id"`
	UserEmail      string `json:"user_email"`
	Username       string `json:"username"`
	APIKeyID       int64  `json:"api_key_id"`
	AccountID      int64  `json:"account_id"`
	AccountName    string `json:"account_name"`
	GroupID        *int64 `json:"group_id"`
	GroupName      string `json:"group_name"`
}

func adminVideoTaskFromRecord(record *service.PPVideoTaskAdminRecord) adminVideoTask {
	return adminVideoTask{
		userVideoTask:  userVideoTaskFromRecord(&record.PPVideoTaskRecord),
		UpstreamTaskID: strings.TrimSpace(record.TaskID),
		Platform:       record.Platform,
		BillingStatus:  record.BillingStatus,
		UserID:         record.UserID,
		UserEmail:      record.UserEmail,
		Username:       record.Username,
		APIKeyID:       record.APIKeyID,
		AccountID:      record.AccountID,
		AccountName:    record.AccountName,
		GroupID:        record.GroupID,
		GroupName:      record.GroupName,
	}
}

// parseVideoTaskDateRange 解析 start_date / end_date（YYYY-MM-DD，按 timezone 参数所在时区，结束日含当天）。
// 返回 false 表示已写出 400 响应。
func parseVideoTaskDateRange(c *gin.Context) (start, end *time.Time, ok bool) {
	userTZ := c.Query("timezone")
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		parsed, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return nil, nil, false
		}
		start = &parsed
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		parsed, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return nil, nil, false
		}
		parsed = parsed.AddDate(0, 0, 1)
		end = &parsed
	}
	return start, end, true
}

// AdminListVideoTasks 返回全部用户的视频任务列表。
func (h *OpenAIGatewayHandler) AdminListVideoTasks(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.PPVideoTaskAdminListFilter{
		PPVideoTaskListFilter: service.PPVideoTaskListFilter{
			TaskID:   c.Query("task_id"),
			Model:    c.Query("model"),
			APIKey:   c.Query("api_key"),
			Status:   strings.ToLower(strings.TrimSpace(c.Query("status"))),
			Page:     page,
			PageSize: pageSize,
		},
		User:     c.Query("user"),
		Platform: c.Query("platform"),
	}
	if !validUserVideoTaskStatusFilter(filter.Status) {
		response.BadRequest(c, "Invalid status")
		return
	}
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = userID
	}
	start, end, ok := parseVideoTaskDateRange(c)
	if !ok {
		return
	}
	filter.StartTime, filter.EndTime = start, end

	records, total, err := h.gatewayService.ListPPVideoTasksForAdmin(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]adminVideoTask, 0, len(records))
	for _, record := range records {
		items = append(items, adminVideoTaskFromRecord(record))
	}
	normalized := filter.Normalized()
	response.Paginated(c, items, total, normalized.Page, normalized.PageSize)
}

// AdminGetVideoTask 返回单个视频任务详情（任务 ID 可为本地 ID 或上游 ID）。
func (h *OpenAIGatewayHandler) AdminGetVideoTask(c *gin.Context) {
	record, err := h.gatewayService.GetPPVideoTaskRecordForAdmin(c.Request.Context(), c.Param("id"))
	if err != nil || record == nil {
		response.NotFound(c, "Video task not found")
		return
	}
	response.Success(c, adminVideoTaskFromRecord(record))
}

// AdminGetVideoTaskContent 代理输出任务的结果视频。规则与用户端一致：
// 只有已成功且允许对外展示的任务可用，其余一律 404。
func (h *OpenAIGatewayHandler) AdminGetVideoTaskContent(c *gin.Context) {
	reqLog := requestLogger(c, "handler.openai_gateway.pp_video_admin_tasks",
		zap.String("task", c.Param("id")),
	)
	task, err := h.gatewayService.GetPPVideoTaskForAdmin(c.Request.Context(), c.Param("id"))
	if err != nil || task == nil || !service.PPVideoTaskContentAvailable(task.Platform, task.Status, task.BillingStatus) {
		response.NotFound(c, "Video content is not available")
		return
	}
	reqLog = reqLog.With(zap.Int64("user_id", task.UserID))
	h.streamPPVideoTaskContent(c, reqLog, task)
}
