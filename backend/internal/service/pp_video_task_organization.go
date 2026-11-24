package service

import (
	"context"
	"strings"
)

// 团队空间「成员明细」里查看成员视频成果用的只读能力：超级管理员 / 管理员只能取到
// 「提交时计费给本团队、且计费成员就是目标成员」的任务。归属条件与操作者身份校验都在仓储的
// 同一条查询里完成（操作者必须是该团队的在职超级管理员 / 管理员）。

// PPVideoTaskOrganizationQueryRepository 是可选仓储能力，单独成接口以免影响既有实现与测试桩。
type PPVideoTaskOrganizationQueryRepository interface {
	GetPPVideoTaskForOrganizationAdmin(ctx context.Context, organizationID, actorUserID, memberUserID int64, taskID string) (*PPVideoTask, error)
}

// GetPPVideoTaskForOrganizationAdmin 返回完整任务（含账号信息），仅供内容代理使用。
// 任务不存在、不属于该团队 / 成员，或操作者不是该团队的管理员时，都返回 ErrPPVideoTaskNotFound。
func (s *OpenAIGatewayService) GetPPVideoTaskForOrganizationAdmin(ctx context.Context, organizationID, actorUserID, memberUserID int64, taskID string) (*PPVideoTask, error) {
	if s == nil || s.usageBillingRepo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	repo, ok := s.usageBillingRepo.(PPVideoTaskOrganizationQueryRepository)
	if !ok || repo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	return repo.GetPPVideoTaskForOrganizationAdmin(ctx, organizationID, actorUserID, memberUserID, strings.TrimSpace(taskID))
}
