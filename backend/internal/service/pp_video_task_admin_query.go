package service

import (
	"context"
	"strings"
)

// 管理后台「视频任务」使用的跨用户只读查询。
// 与用户端（pp_video_task_query.go）共用筛选与展示模型，区别是不限定 user_id，
// 并额外带出任务所属用户、API Key、上游账号与分组，便于管理员排查下游客户的任务。

// PPVideoTaskAdminListFilter 在用户端筛选条件之上增加按用户 / 平台筛选。
type PPVideoTaskAdminListFilter struct {
	PPVideoTaskListFilter
	// UserID 精确匹配任务所属用户；0 表示不限。
	UserID int64
	// User 按用户邮箱或用户名子串匹配（不区分大小写）。
	User string
	// Platform 按平台标识子串匹配（不区分大小写）。
	Platform string
}

// Normalized 返回带默认值与边界保护的拷贝。
func (f PPVideoTaskAdminListFilter) Normalized() PPVideoTaskAdminListFilter {
	f.PPVideoTaskListFilter = f.PPVideoTaskListFilter.Normalized()
	f.User = strings.TrimSpace(f.User)
	f.Platform = strings.TrimSpace(f.Platform)
	if f.UserID < 0 {
		f.UserID = 0
	}
	return f
}

// PPVideoTaskAdminRecord 是管理端展示用的只读视图：在用户端视图之上附带归属信息。
// 仍不含上游原始响应、轮询租约等内部字段。
type PPVideoTaskAdminRecord struct {
	PPVideoTaskRecord

	UserID      int64
	UserEmail   string
	Username    string
	APIKeyID    int64
	AccountID   int64
	AccountName string
	GroupID     *int64
	GroupName   string
}

// PPVideoTaskAdminQueryRepository 是管理端任务列表需要的可选仓储能力。
// 与 PPVideoTaskQueryRepository 一样单独成接口，避免扩大既有接口。
type PPVideoTaskAdminQueryRepository interface {
	ListPPVideoTasksForAdmin(ctx context.Context, filter PPVideoTaskAdminListFilter) ([]*PPVideoTaskAdminRecord, int64, error)
	GetPPVideoTaskRecordForAdmin(ctx context.Context, taskID string) (*PPVideoTaskAdminRecord, error)
	GetPPVideoTaskForAdmin(ctx context.Context, taskID string) (*PPVideoTask, error)
}

func (s *OpenAIGatewayService) ppVideoTaskAdminQueryRepo() (PPVideoTaskAdminQueryRepository, error) {
	if s == nil || s.usageBillingRepo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	repo, ok := s.usageBillingRepo.(PPVideoTaskAdminQueryRepository)
	if !ok || repo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	return repo, nil
}

// ListPPVideoTasksForAdmin 分页查询全部用户的视频任务。
func (s *OpenAIGatewayService) ListPPVideoTasksForAdmin(ctx context.Context, filter PPVideoTaskAdminListFilter) ([]*PPVideoTaskAdminRecord, int64, error) {
	repo, err := s.ppVideoTaskAdminQueryRepo()
	if err != nil {
		return nil, 0, err
	}
	return repo.ListPPVideoTasksForAdmin(ctx, filter.Normalized())
}

// GetPPVideoTaskRecordForAdmin 按任务 ID（本地或上游）查询单个任务。
func (s *OpenAIGatewayService) GetPPVideoTaskRecordForAdmin(ctx context.Context, taskID string) (*PPVideoTaskAdminRecord, error) {
	repo, err := s.ppVideoTaskAdminQueryRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPPVideoTaskRecordForAdmin(ctx, strings.TrimSpace(taskID))
}

// GetPPVideoTaskForAdmin 返回完整任务（含账号信息），仅供内容代理使用。
func (s *OpenAIGatewayService) GetPPVideoTaskForAdmin(ctx context.Context, taskID string) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskAdminQueryRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPPVideoTaskForAdmin(ctx, strings.TrimSpace(taskID))
}
