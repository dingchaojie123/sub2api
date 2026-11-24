package service

import (
	"context"
	"errors"
	"time"
)

// 团队「成员明细」：超级管理员 / 管理员查看某个成员具体用了哪些模型、提交了哪些生成任务。
//
// 内容范围（和用户确认过，只用已有数据，不新增内容存储）：
//   - 模型用量：来自 usage_logs（按 organization_id + user_id 聚合）。
//   - 生成任务：视频任务（pp_video_tasks，含提示词）与批量图片任务（batch_image_jobs，含各条提示词预览），
//     按任务提交时固化的 billing_organization_id / billing_member_user_id 归属，
//     不受之后 Key 换绑或成员被移除的影响。
//   - 对话 / 文本请求、单张同步出图不保存正文，因此这里没有内容可看。

const (
	OrganizationMemberTaskKindVideo      = "video"
	OrganizationMemberTaskKindImageBatch = "image_batch"
)

var ErrOrganizationMemberDetailUnsupported = errors.New("organization member detail is not supported by the repository")

// OrganizationMemberModelUsage 成员在某个模型上的用量。
type OrganizationMemberModelUsage struct {
	Model         string     `json:"model"`
	TotalRequests int64      `json:"total_requests"`
	TotalCost     float64    `json:"total_cost"`
	InputTokens   int64      `json:"input_tokens"`
	OutputTokens  int64      `json:"output_tokens"`
	ImageCount    int64      `json:"image_count"`
	VideoCount    int64      `json:"video_count"`
	LastUsedAt    *time.Time `json:"last_used_at"`
}

// OrganizationMemberUsageDetail 某个成员的汇总 + 按模型拆分。
type OrganizationMemberUsageDetail struct {
	UserID   int64                          `json:"user_id"`
	Email    string                         `json:"email"`
	Username string                         `json:"username"`
	Summary  OrganizationUsageSummary       `json:"summary"`
	Models   []OrganizationMemberModelUsage `json:"models"`
}

// OrganizationMemberTask 成员提交的一条生成任务（视频任务或批量图片任务）。
type OrganizationMemberTask struct {
	// Kind: video | image_batch
	Kind   string `json:"kind"`
	TaskID string `json:"task_id"`
	Model  string `json:"model"`
	Status string `json:"status"`
	// Prompt 视频任务为完整提示词；批量图片任务为前几条提示词预览（换行分隔）。历史任务可能为空。
	Prompt string `json:"prompt"`
	// 视频：生成条数 / 分辨率 / 请求时长（秒）；批量图片：ItemCount 为提交的图片条数。
	ItemCount       int        `json:"item_count"`
	SuccessCount    int        `json:"success_count"`
	Resolution      string     `json:"resolution"`
	DurationSeconds int        `json:"duration_seconds"`
	Cost            *float64   `json:"cost"`
	ErrorMessage    string     `json:"error_message"`
	CreatedAt       time.Time  `json:"created_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	// CanPlay 仅视频任务：结果视频可以由本站代理播放 / 下载（GET /organizations/member-tasks/content）。
	CanPlay bool `json:"can_play"`
}

// OrganizationMemberDetailRepository 是 OrganizationRepository 之外的可选能力，
// 用类型断言接入，避免改动已有接口导致测试桩全部要改。
type OrganizationMemberDetailRepository interface {
	MemberUsageDetail(ctx context.Context, organizationID, actorUserID, memberUserID int64, start, end time.Time) (*OrganizationMemberUsageDetail, error)
	MemberTasks(ctx context.Context, organizationID, actorUserID, memberUserID int64, start, end time.Time, limit, offset int) ([]OrganizationMemberTask, int64, error)
}

func (s *OrganizationService) memberDetailRepo() (OrganizationMemberDetailRepository, error) {
	repo, ok := s.repo.(OrganizationMemberDetailRepository)
	if !ok {
		return nil, ErrOrganizationMemberDetailUnsupported
	}
	return repo, nil
}

// MemberUsageDetail 成员的模型用量明细（仅超级管理员 / 管理员）。
func (s *OrganizationService) MemberUsageDetail(ctx context.Context, organizationID, actorUserID, memberUserID int64, start, end time.Time) (*OrganizationMemberUsageDetail, error) {
	repo, err := s.memberDetailRepo()
	if err != nil {
		return nil, err
	}
	return repo.MemberUsageDetail(ctx, organizationID, actorUserID, memberUserID, start, end)
}

// MemberTasks 成员提交的视频 / 批量图片任务（含提示词），分页（仅超级管理员 / 管理员）。
func (s *OrganizationService) MemberTasks(ctx context.Context, organizationID, actorUserID, memberUserID int64, start, end time.Time, limit, offset int) ([]OrganizationMemberTask, int64, error) {
	repo, err := s.memberDetailRepo()
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return repo.MemberTasks(ctx, organizationID, actorUserID, memberUserID, start, end, limit, offset)
}
