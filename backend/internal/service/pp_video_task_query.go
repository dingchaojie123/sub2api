package service

import (
	"context"
	"strings"
	"time"
)

// 视频任务记录页（用户端）使用的查询模型与辅助函数。
// 只读，不参与计费；数据来自 pp_video_tasks，仅限任务所属用户访问。

const (
	// PPVideoTaskListMaxPageSize 是列表单页上限。
	PPVideoTaskListMaxPageSize = 100
	// PPVideoTaskListDefaultPageSize 是列表默认每页条数。
	PPVideoTaskListDefaultPageSize = 20
)

// PPVideoTaskListFilter 描述列表筛选条件。空字段表示不筛选。
type PPVideoTaskListFilter struct {
	// TaskID 按任务 ID 子串匹配（本地任务 ID 或上游任务 ID）。
	TaskID string
	// Model 按模型名子串匹配（不区分大小写）。
	Model string
	// APIKey 按 API Key 名称子串匹配。不会也不允许按密钥原文匹配。
	APIKey string
	// Status 是归一后的状态分组：processing / succeeded / failed / requires_attention。
	Status string
	// StartTime（含）与 EndTime（不含）限定创建时间范围。
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

// Normalized 返回带默认值与边界保护的拷贝。
func (f PPVideoTaskListFilter) Normalized() PPVideoTaskListFilter {
	f.TaskID = strings.TrimSpace(f.TaskID)
	f.Model = strings.TrimSpace(f.Model)
	f.APIKey = strings.TrimSpace(f.APIKey)
	f.Status = strings.ToLower(strings.TrimSpace(f.Status))
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = PPVideoTaskListDefaultPageSize
	}
	if f.PageSize > PPVideoTaskListMaxPageSize {
		f.PageSize = PPVideoTaskListMaxPageSize
	}
	return f
}

// PPVideoTaskRecord 是任务记录页展示用的只读视图，不含上游原始响应、账号与租约等内部字段。
type PPVideoTaskRecord struct {
	LocalTaskID                        string
	TaskID                             string
	APIKeyName                         string
	Platform                           string
	Operation                          PPVideoOperation
	Model                              string
	Status                             string
	BillingStatus                      string
	Prompt                             string
	VideoCount                         int
	VideoResolution                    string
	RequestedVideoDurationSeconds      int
	RequestedVideoDurationMilliseconds int64
	GeneratedVideoDurationMilliseconds int64
	HoldAmount                         float64
	ActualCost                         *float64
	LastErrorCode                      string
	LastErrorMessage                   string

	// FailureResponseBody 只在任务失败且没有记录错误信息时才有值：保存的上游响应前 8KB，
	// 仅用于从中提取失败原因，不会原样对外返回。
	FailureResponseBody string
	CreatedAt           time.Time
	FinishedAt          *time.Time
	SettledAt           *time.Time
}

// PPVideoTaskQueryRepository 是任务记录页需要的可选仓储能力。
// 单独成接口，避免扩大 PPVideoTaskRepository 而影响既有实现与测试桩。
type PPVideoTaskQueryRepository interface {
	ListPPVideoTasksForUser(ctx context.Context, userID int64, filter PPVideoTaskListFilter) ([]*PPVideoTaskRecord, int64, error)
	GetPPVideoTaskRecordForUser(ctx context.Context, userID int64, taskID string) (*PPVideoTaskRecord, error)
	GetPPVideoTaskForUser(ctx context.Context, userID int64, taskID string) (*PPVideoTask, error)
	SetPPVideoTaskPrompt(ctx context.Context, localTaskID, prompt string) error
}

func (s *OpenAIGatewayService) ppVideoTaskQueryRepo() (PPVideoTaskQueryRepository, error) {
	if s == nil || s.usageBillingRepo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	repo, ok := s.usageBillingRepo.(PPVideoTaskQueryRepository)
	if !ok || repo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	return repo, nil
}

// ListPPVideoTasksForUser 分页查询用户自己的视频任务。
func (s *OpenAIGatewayService) ListPPVideoTasksForUser(ctx context.Context, userID int64, filter PPVideoTaskListFilter) ([]*PPVideoTaskRecord, int64, error) {
	repo, err := s.ppVideoTaskQueryRepo()
	if err != nil {
		return nil, 0, err
	}
	return repo.ListPPVideoTasksForUser(ctx, userID, filter.Normalized())
}

// GetPPVideoTaskRecordForUser 按任务 ID（本地或上游）查询用户自己的单个任务。
func (s *OpenAIGatewayService) GetPPVideoTaskRecordForUser(ctx context.Context, userID int64, taskID string) (*PPVideoTaskRecord, error) {
	repo, err := s.ppVideoTaskQueryRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPPVideoTaskRecordForUser(ctx, userID, strings.TrimSpace(taskID))
}

// GetPPVideoTaskForUser 返回完整任务（含账号信息），仅供内容代理使用。
func (s *OpenAIGatewayService) GetPPVideoTaskForUser(ctx context.Context, userID int64, taskID string) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskQueryRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPPVideoTaskForUser(ctx, userID, strings.TrimSpace(taskID))
}

// RecordPPVideoTaskPrompt 记录任务提示词。提示词仅用于展示，任何错误都不应影响任务提交。
func (s *OpenAIGatewayService) RecordPPVideoTaskPrompt(ctx context.Context, localTaskID string, requestBody []byte) error {
	prompt := PPVideoPromptFromRequest(requestBody)
	if prompt == "" || strings.TrimSpace(localTaskID) == "" {
		return nil
	}
	repo, err := s.ppVideoTaskQueryRepo()
	if err != nil {
		return err
	}
	return repo.SetPPVideoTaskPrompt(ctx, localTaskID, prompt)
}

var (
	ppVideoProcessingStatuses = []string{
		PPVideoTaskStatusSubmitting, "accepted", "pending", "processing", "reconciling", "running",
		"queued", "created", "submitted", "in_progress", "waiting", "unknown",
	}
	ppVideoSucceededStatuses = []string{
		"success", "succeed", "succeeded", "completed", "complete", "done", "finished", "successful",
	}
	ppVideoFailedStatuses = []string{
		"fail", "failed", "failure", "error", "cancelled", "canceled", "rejected", "refunded",
		"timeout", "timed_out", "expired",
	}
	ppVideoAttentionStatuses = []string{"requires_attention", "needs_attention"}
)

// PPVideoTaskStatusValues 返回某个状态分组对应的全部原始状态值，用于 SQL IN 过滤。
// 未知分组返回 nil，表示不按状态过滤。
func PPVideoTaskStatusValues(group string) []string {
	switch strings.ToLower(strings.TrimSpace(group)) {
	case PPVideoTaskStatusProcessing:
		return append([]string(nil), ppVideoProcessingStatuses...)
	case PPVideoTaskStatusSucceeded:
		return append([]string(nil), ppVideoSucceededStatuses...)
	case PPVideoTaskStatusFailed:
		return append([]string(nil), ppVideoFailedStatuses...)
	case PPVideoTaskStatusRequiresAttention:
		return append([]string(nil), ppVideoAttentionStatuses...)
	default:
		return nil
	}
}

// PPVideoTaskStatusGroup 把原始状态归一成展示分组。submitting 视为处理中。
func PPVideoTaskStatusGroup(status string) string {
	normalized := NormalizePPVideoTaskStatus(status)
	if normalized == PPVideoTaskStatusSubmitting {
		return PPVideoTaskStatusProcessing
	}
	return normalized
}

// PPVideoTaskCanStreamContent 表示该平台的结果视频可由本站代理播放 / 下载。
// 所有视频任务平台都支持：CTMO / Xingapi / OPONE 走带凭证的内容代理，
// 其余平台读取任务保存的视频直链（见 ForwardPPVideoTaskUserContent）。
func PPVideoTaskCanStreamContent(platform string) bool {
	return IsPPVideoPlatform(platform)
}

// PPVideoTaskContentAvailable reports whether a finished task may expose its video.
// Mai Token results stay private until the held balance has been captured.
func PPVideoTaskContentAvailable(platform, status, billingStatus string) bool {
	if !PPVideoTaskCanStreamContent(platform) || PPVideoTaskStatusGroup(status) != PPVideoTaskStatusSucceeded {
		return false
	}
	return platform != PlatformMaiTokenSeedance || billingStatus == PPVideoBillingStatusCaptured
}

// PPVideoTaskEndpoint 返回创建该任务时调用的接口路径，用于详情展示。
func PPVideoTaskEndpoint(operation PPVideoOperation) string {
	switch operation {
	case PPVideoOperationKlingTextToVideo:
		return "/v1/videos/text2video"
	case PPVideoOperationKlingImageToVideo:
		return "/v1/videos/image2video"
	default:
		return "/v1/videos/generations"
	}
}

// DurationSeconds 返回视频时长（秒）：优先用上游确认的生成时长，否则用请求时长。
func (r *PPVideoTaskRecord) DurationSeconds() float64 {
	if r == nil {
		return 0
	}
	if r.GeneratedVideoDurationMilliseconds > 0 {
		return float64(r.GeneratedVideoDurationMilliseconds) / 1000
	}
	if r.RequestedVideoDurationMilliseconds > 0 {
		return float64(r.RequestedVideoDurationMilliseconds) / 1000
	}
	return float64(r.RequestedVideoDurationSeconds)
}

// TotalSeconds 返回从创建到结束的总用时（秒）。任务未结束时返回 nil。
func (r *PPVideoTaskRecord) TotalSeconds() *float64 {
	if r == nil || r.FinishedAt == nil || r.FinishedAt.Before(r.CreatedAt) {
		return nil
	}
	seconds := r.FinishedAt.Sub(r.CreatedAt).Seconds()
	return &seconds
}

// CreditsRefunded 返回已退回的积分：
//   - 冻结已整笔释放（失败 / 取消）：退回全部冻结额；
//   - 已结算且实扣小于冻结额：退回差额；
//   - 其余（未结算、订阅抵扣、无冻结）返回 nil。
func (r *PPVideoTaskRecord) CreditsRefunded() *float64 {
	if r == nil || r.HoldAmount <= 0 {
		return nil
	}
	switch r.BillingStatus {
	case PPVideoBillingStatusReleased:
		refund := r.HoldAmount
		return &refund
	case PPVideoBillingStatusCaptured:
		if r.ActualCost == nil {
			return nil
		}
		refund := r.HoldAmount - *r.ActualCost
		if refund < 0 {
			refund = 0
		}
		return &refund
	default:
		return nil
	}
}
