package handler

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 「视频任务」记录页（登录态）的接口。
//
//	GET /api/v1/video-tasks               列表（筛选 + 分页）
//	GET /api/v1/video-tasks/:id           详情
//	GET /api/v1/video-tasks/:id/content   视频内容（播放 / 下载）
//
// 所有查询都限定为当前登录用户自己的任务；返回的是展示用视图，
// 不包含上游原始响应、账号 ID 与轮询租约等内部信息。

const userVideoTaskErrorMessageMaxRunes = 500

// userVideoTask 是任务记录页使用的任务视图。积分字段沿用任务表里的 cost 口径（价格即积分）。
type userVideoTask struct {
	ID              string     `json:"id"`
	RequestID       string     `json:"request_id"`
	Model           string     `json:"model"`
	Status          string     `json:"status"`
	Prompt          string     `json:"prompt"`
	OutputCount     int        `json:"output_count"`
	Resolution      string     `json:"resolution"`
	DurationSeconds float64    `json:"duration_seconds"`
	Endpoint        string     `json:"endpoint"`
	APIKeyName      string     `json:"api_key_name"`
	CreditsCharged  *float64   `json:"credits_charged"`
	CreditsHeld     float64    `json:"credits_held"`
	CreditsRefunded *float64   `json:"credits_refunded"`
	TotalSeconds    *float64   `json:"total_seconds"`
	ErrorCode       string     `json:"error_code"`
	ErrorMessage    string     `json:"error_message"`
	CanPlay         bool       `json:"can_play"`
	CreatedAt       time.Time  `json:"created_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

func userVideoTaskFromRecord(record *service.PPVideoTaskRecord) userVideoTask {
	status := service.PPVideoTaskStatusGroup(record.Status)
	requestID := strings.TrimSpace(record.TaskID)
	if requestID == "" {
		requestID = record.LocalTaskID
	}
	outputCount := 0
	if status == service.PPVideoTaskStatusSucceeded {
		outputCount = record.VideoCount
	}
	return userVideoTask{
		ID:              record.LocalTaskID,
		RequestID:       requestID,
		Model:           record.Model,
		Status:          status,
		Prompt:          record.Prompt,
		OutputCount:     outputCount,
		Resolution:      record.VideoResolution,
		DurationSeconds: record.DurationSeconds(),
		Endpoint:        service.PPVideoTaskEndpoint(record.Operation),
		APIKeyName:      record.APIKeyName,
		CreditsCharged:  record.ActualCost,
		CreditsHeld:     record.HoldAmount,
		CreditsRefunded: record.CreditsRefunded(),
		TotalSeconds:    record.TotalSeconds(),
		ErrorCode:       record.LastErrorCode,
		ErrorMessage:    truncateRunes(record.LastErrorMessage, userVideoTaskErrorMessageMaxRunes),
		CanPlay: status == service.PPVideoTaskStatusSucceeded &&
			service.PPVideoTaskCanStreamContent(record.Platform),
		CreatedAt:  record.CreatedAt,
		FinishedAt: record.FinishedAt,
	}
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func validUserVideoTaskStatusFilter(status string) bool {
	switch status {
	case "",
		service.PPVideoTaskStatusProcessing,
		service.PPVideoTaskStatusSucceeded,
		service.PPVideoTaskStatusFailed,
		service.PPVideoTaskStatusRequiresAttention:
		return true
	default:
		return false
	}
}

// ListUserVideoTasks 返回当前用户的视频任务列表。
func (h *OpenAIGatewayHandler) ListUserVideoTasks(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	filter := service.PPVideoTaskListFilter{
		TaskID:   c.Query("task_id"),
		Model:    c.Query("model"),
		APIKey:   c.Query("api_key"),
		Status:   strings.ToLower(strings.TrimSpace(c.Query("status"))),
		Page:     page,
		PageSize: pageSize,
	}
	if !validUserVideoTaskStatusFilter(filter.Status) {
		response.BadRequest(c, "Invalid status")
		return
	}

	userTZ := c.Query("timezone")
	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		start, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		filter.StartTime = &start
	}
	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		end, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		end = end.AddDate(0, 0, 1)
		filter.EndTime = &end
	}

	records, total, err := h.gatewayService.ListPPVideoTasksForUser(c.Request.Context(), subject.UserID, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]userVideoTask, 0, len(records))
	for _, record := range records {
		items = append(items, userVideoTaskFromRecord(record))
	}
	normalized := filter.Normalized()
	response.Paginated(c, items, total, normalized.Page, normalized.PageSize)
}

// GetUserVideoTask 返回当前用户的单个视频任务详情。
func (h *OpenAIGatewayHandler) GetUserVideoTask(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	record, err := h.gatewayService.GetPPVideoTaskRecordForUser(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil || record == nil {
		response.NotFound(c, "Video task not found")
		return
	}
	response.Success(c, userVideoTaskFromRecord(record))
}

// GetUserVideoTaskContent 代理输出任务的结果视频，供详情里播放和下载。
// 只有已成功的视频任务可用，其余一律 404，避免让用户借此读取他人任务或暴露内部信息。
// 视频来源：CTMO / Xingapi / OPONE 走带凭证的内容代理，其他平台读取任务保存的视频直链。
func (h *OpenAIGatewayHandler) GetUserVideoTaskContent(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.pp_video_user_tasks",
		zap.Int64("user_id", subject.UserID),
		zap.String("task", c.Param("id")),
	)
	task, err := h.gatewayService.GetPPVideoTaskForUser(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil || task == nil ||
		!service.PPVideoTaskCanStreamContent(task.Platform) ||
		service.PPVideoTaskStatusGroup(task.Status) != service.PPVideoTaskStatusSucceeded {
		response.NotFound(c, "Video content is not available")
		return
	}
	reqLog = reqLog.With(zap.String("platform", task.Platform), zap.Int64("account_id", task.AccountID))

	selection, err := h.gatewayService.SelectPPVideoTaskAccountForStatus(c.Request.Context(), task.AccountID, task.Platform)
	if err != nil || selection == nil || selection.Account == nil {
		reqLog.Warn("pp_video.user_content_account_unavailable", zap.Error(err))
		response.Error(c, http.StatusServiceUnavailable, "Video content is temporarily unavailable")
		return
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}

	// 记录页总是整段取回视频再播放，所以不转发客户端的 Range / If-Range：
	// 否则上游（或中间层）可能只回一小段 206，浏览器拿到的就是残缺文件。
	clientRange := c.GetHeader("Range")
	resp, err := h.gatewayService.ForwardPPVideoTaskUserContent(c.Request.Context(), selection.Account, task, "", "")
	if err != nil {
		reqLog.Warn("pp_video.user_content_forward_failed", zap.String("client_range", clientRange), zap.Error(err))
		response.Error(c, http.StatusBadGateway, "Video download failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// 只接受完整的 200：上游返回其他状态（含未请求却返回的 206）时，
	// 不把上游的响应体转给浏览器，记录状态与响应片段便于排查，并给用户一个明确的结果。
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		reqLog.Warn("pp_video.user_content_upstream_status",
			zap.Int("upstream_status", resp.StatusCode),
			zap.String("upstream_content_range", resp.Header.Get("Content-Range")),
			zap.String("client_range", clientRange),
			zap.String("upstream_body", string(snippet)),
		)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			response.NotFound(c, "The video is no longer available")
			return
		}
		response.Error(c, http.StatusBadGateway, "Video download failed")
		return
	}
	reqLog.Info("pp_video.user_content_streaming",
		zap.String("content_length", resp.Header.Get("Content-Length")),
		zap.String("content_type", resp.Header.Get("Content-Type")),
		zap.String("client_range", clientRange),
	)

	// 只转发内容本身的头；不转发上游的缓存 / 范围相关头。
	// 视频属于用户私有数据，禁止浏览器共享缓存和 CDN 缓存。
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Disposition"} {
		if value := strings.TrimSpace(resp.Header.Get(header)); value != "" {
			c.Header(header, value)
		}
	}
	if strings.TrimSpace(c.Writer.Header().Get("Content-Type")) == "" {
		c.Header("Content-Type", "application/octet-stream")
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Vary", "Authorization")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	if c.Request.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(c.Writer, resp.Body)
}
