package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// BindDefaultKeysToOrganization 成员通过邀请加入团队后，把他名下「系统自动创建的默认 Key」绑定到该团队计费。
//
// 背景：默认 Key 在注册时一律按个人计费创建；通过邀请链接新注册的用户个人余额为 0，
// 入团后这些 Key 会因余额不足全部不可用。
//
// 范围（都必须同时满足「Key 有效 且 还没有任何团队绑定记录」）：
//   - user_default_api_keys 里的文本 / 图片 / 视频 / 音频默认 Key；
//   - 注册时按固定分组名自动发的 Key（名称与所在分组名一致，且分组名在 signupGroupNames 里）。
//
// 已有绑定记录的 Key 一律不动：包括已绑定其他团队的、成员手动切回个人（status=detached）的、
// 团队共享副本（已有记录）。因此重复调用是幂等的，也不会覆盖用户的手动选择。
// 返回被绑定的 Key 字符串，供调用方清理鉴权缓存。成员不在团（或已停用）时是空操作。
func (r *organizationRepository) BindDefaultKeysToOrganization(ctx context.Context, organizationID, userID int64, signupGroupNames []string) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 与 AcceptInvitation / UpdateMember 一样先锁团队行，避免和成员状态变更并发。
	var lockedID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM organizations WHERE id=$1 AND status='active' FOR UPDATE`, organizationID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `WITH target AS (
			SELECT k.id FROM api_keys k
			WHERE k.user_id=$2::bigint AND k.deleted_at IS NULL AND k.status='active'
			AND EXISTS (SELECT 1 FROM organization_members m WHERE m.organization_id=$1::bigint AND m.user_id=$2::bigint AND m.status='active')
			AND NOT EXISTS (SELECT 1 FROM organization_api_keys ok WHERE ok.api_key_id=k.id)
			AND (
				k.id IN (SELECT d.api_key_id FROM user_default_api_keys d WHERE d.user_id=$2::bigint AND d.api_key_id IS NOT NULL)
				OR EXISTS (SELECT 1 FROM groups g WHERE g.id=k.group_id AND g.name=k.name AND g.name = ANY($3::text[]))
			)
		), bound AS (
			INSERT INTO organization_api_keys(api_key_id,organization_id,member_user_id,status,created_at)
			SELECT id,$1::bigint,$2::bigint,'active',NOW() FROM target
			ON CONFLICT(api_key_id) DO NOTHING
			RETURNING api_key_id
		)
		SELECT k.key FROM api_keys k JOIN bound ON bound.api_key_id=k.id ORDER BY k.id`,
		organizationID, userID, pq.Array(signupGroupNames))
	if err != nil {
		return nil, err
	}
	credentials := []string{}
	for rows.Next() {
		var credential string
		if err := rows.Scan(&credential); err != nil {
			_ = rows.Close()
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	if len(credentials) > 0 {
		if err = insertOrganizationAudit(ctx, tx, organizationID, userID, "api_key.auto_bind", "member", fmt.Sprint(userID), map[string]any{"bound": len(credentials)}); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return credentials, nil
}
