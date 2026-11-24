package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetPPVideoTaskForOrganizationAdmin 取团队成员的一个视频任务，供团队管理员在「成员明细」里播放结果。
//
// 一条查询同时保证：
//   - 任务提交时就是计费给 organizationID、计费成员是 memberUserID（billing_* 列在提交时固化，
//     不受之后 Key 换绑 / 成员被移除影响）；
//   - 操作者 actorUserID 是该团队的在职超级管理员 / 管理员，且团队仍有效。
//
// 任何一条不满足都按「找不到」处理，不区分原因，避免被用来探测他人任务。
func (r *usageBillingRepository) GetPPVideoTaskForOrganizationAdmin(ctx context.Context, organizationID, actorUserID, memberUserID int64, taskID string) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, service.ErrPPVideoTaskNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+ppVideoTaskColumns()+`
		FROM pp_video_tasks
		WHERE billing_organization_id = $1::bigint
			AND billing_member_user_id = $2::bigint
			AND (task_id = $3::text OR local_task_id = $3::text)
			AND EXISTS (
				SELECT 1
				FROM organization_members om
				JOIN organizations o ON o.id = om.organization_id
				WHERE om.organization_id = $1::bigint
					AND om.user_id = $4::bigint
					AND om.status = 'active'
					AND om.role IN ('owner', 'admin')
					AND o.status = 'active'
			)
		ORDER BY id DESC
		LIMIT 1
	`, organizationID, memberUserID, taskID, actorUserID)
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return task, nil
}
