package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 团队 Key 共享（创建者 → 成员副本）。
//
// 设计要点：
//   - 副本是成员名下一把「全新的 Key」（新的密钥字符串），配置（分组、额度、限速、IP 规则、过期时间）
//     复制自源 Key，用量归零，并绑定到同一个团队，所以团队额度、成员月度限额、按成员的用量统计都照常生效。
//   - 全部使用原生 SQL，不改 ent 的 api_keys schema（沙箱里没法重新生成 ent）。
//   - 所有写操作都先通过 organizationActorRole 锁住团队行（与 AcceptInvitation 的 FOR UPDATE 互斥），
//     所以「成员加入」和「创建者开启共享」并发时不会漏发、也不会重复发。

// newSharedAPIKey 生成与普通创建 Key 相同格式的随机密钥（前缀 + 64 位十六进制）。
func (r *organizationRepository) newSharedAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	prefix := "sk-"
	if r.cfg != nil && r.cfg.Default.APIKeyPrefix != "" {
		prefix = r.cfg.Default.APIKeyPrefix
	}
	return prefix + hex.EncodeToString(buf), nil
}

// SetAPIKeyShared 创建者（团队 owner）开启 / 关闭某把团队 Key 的「共享给团队成员」。
// 开启：给当前所有符合条件的在团成员补发副本（之后加入的成员由 AcceptInvitation 自动发）；
// 关闭：停用所有成员的副本，并返回这些副本的密钥，供调用方清理鉴权缓存。
func (r *organizationRepository) SetAPIKeyShared(ctx context.Context, apiKeyID, actorUserID int64, shared bool) (_ *service.APIKeyShareResult, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 先不加锁拿到团队 ID，再按「团队行 → Key 行」的顺序加锁，避免和加入团队流程死锁。
	var orgID int64
	if err = tx.QueryRowContext(ctx, `SELECT organization_id FROM organization_api_keys WHERE api_key_id=$1`, apiKeyID).Scan(&orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAPIKeyNotTeamBilled
		}
		return nil, err
	}
	role, err := organizationActorRole(ctx, tx, orgID, actorUserID)
	if err != nil {
		return nil, err
	}
	if role != service.OrganizationRoleOwner {
		return nil, service.ErrAPIKeyShareOwnerOnly
	}
	var bindingStatus string
	var isCopy bool
	if err = tx.QueryRowContext(ctx, `SELECT status,(source_api_key_id IS NOT NULL) FROM organization_api_keys WHERE api_key_id=$1 AND organization_id=$2 FOR UPDATE`, apiKeyID, orgID).Scan(&bindingStatus, &isCopy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAPIKeyNotTeamBilled
		}
		return nil, err
	}
	if isCopy {
		return nil, service.ErrAPIKeyShareCopy
	}
	if bindingStatus != "active" {
		return nil, service.ErrAPIKeyNotTeamBilled
	}
	var credential string
	if err = lockOwnedAPIKey(ctx, tx, apiKeyID, actorUserID, &credential); err != nil {
		return nil, err
	}

	result := &service.APIKeyShareResult{}
	if shared {
		if _, err = tx.ExecContext(ctx, `UPDATE organization_api_keys SET shared_with_team=TRUE WHERE api_key_id=$1`, apiKeyID); err != nil {
			return nil, err
		}
		restored, err := r.restoreSharedCopies(ctx, tx, orgID, apiKeyID, 0)
		if err != nil {
			return nil, err
		}
		created, err := r.cloneSharedKey(ctx, tx, orgID, apiKeyID, 0)
		if err != nil {
			return nil, err
		}
		result.Issued = restored + created
		// 没拿到副本的在团成员（例如无权使用该专属分组）数量，用于提示创建者。
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_members m
			WHERE m.organization_id=$1 AND m.status='active' AND m.user_id<>$2
			AND NOT EXISTS (SELECT 1 FROM organization_api_keys c WHERE c.source_api_key_id=$3 AND c.member_user_id=m.user_id AND c.status='active')`,
			orgID, actorUserID, apiKeyID).Scan(&result.Skipped); err != nil {
			return nil, err
		}
		if err = insertOrganizationAudit(ctx, tx, orgID, actorUserID, "api_key.share", "api_key", fmt.Sprint(apiKeyID), map[string]any{"issued": result.Issued, "skipped": result.Skipped}); err != nil {
			return nil, err
		}
	} else {
		revoked, err := revokeSharedCopiesTx(ctx, tx, apiKeyID)
		if err != nil {
			return nil, err
		}
		result.RevokedKeys = revoked
		if err = insertOrganizationAudit(ctx, tx, orgID, actorUserID, "api_key.unshare", "api_key", fmt.Sprint(apiKeyID), map[string]any{"revoked": len(revoked)}); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// RevokeSharedCopies 停用某把源 Key 派生的全部成员副本，并清掉源 Key 的共享标记。
// 用于：源 Key 被删除、源 Key 改成个人计费或换了团队。源 Key 不是共享 Key 时是空操作。
func (r *organizationRepository) RevokeSharedCopies(ctx context.Context, apiKeyID int64) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	keys, err := revokeSharedCopiesTx(ctx, tx, apiKeyID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return keys, nil
}

func revokeSharedCopiesTx(ctx context.Context, tx *sql.Tx, sourceAPIKeyID int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT k.key FROM api_keys k JOIN organization_api_keys ok ON ok.api_key_id=k.id WHERE ok.source_api_key_id=$1 AND ok.status='active'`, sourceAPIKeyID)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	// 先停 api_keys（依赖 ok.status 仍是 active），再改绑定表。
	if _, err = tx.ExecContext(ctx, `UPDATE api_keys SET status='inactive',updated_at=NOW() WHERE id IN (SELECT api_key_id FROM organization_api_keys WHERE source_api_key_id=$1 AND status='active')`, sourceAPIKeyID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_api_keys SET status='inactive' WHERE source_api_key_id=$1 AND status='active'`, sourceAPIKeyID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_api_keys SET shared_with_team=FALSE WHERE api_key_id=$1`, sourceAPIKeyID); err != nil {
		return nil, err
	}
	return keys, nil
}

// syncSharedKeysForMember 成员加入 / 重新启用时调用：恢复他之前的副本，并补发还没有的副本。
// 调用方必须已经持有团队行锁（AcceptInvitation 的 FOR UPDATE，或 UpdateMember 里的 organizationActorRole）。
func (r *organizationRepository) syncSharedKeysForMember(ctx context.Context, tx *sql.Tx, organizationID, memberUserID, actorUserID int64) error {
	restored, err := r.restoreSharedCopies(ctx, tx, organizationID, 0, memberUserID)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT src.api_key_id FROM organization_api_keys src
		JOIN api_keys k ON k.id=src.api_key_id AND k.deleted_at IS NULL AND k.status='active' AND (k.expires_at IS NULL OR k.expires_at>NOW())
		WHERE src.organization_id=$1 AND src.shared_with_team AND src.status='active' AND src.source_api_key_id IS NULL
		ORDER BY src.api_key_id`, organizationID)
	if err != nil {
		return err
	}
	sources := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	created := 0
	for _, sourceID := range sources {
		n, err := r.cloneSharedKey(ctx, tx, organizationID, sourceID, memberUserID)
		if err != nil {
			return err
		}
		created += n
	}
	if restored+created == 0 {
		return nil
	}
	return insertOrganizationAudit(ctx, tx, organizationID, actorUserID, "api_key.auto_issue", "member", fmt.Sprint(memberUserID), map[string]any{"created": created, "restored": restored})
}

// restoreSharedCopies 把因成员被停用 / 移除、或源 Key 曾被取消共享而停用的副本重新启用。
// sourceAPIKeyID / onlyMemberUserID 传 0 表示不限。只恢复「源 Key 仍在共享、成员仍在团」的副本。
func (r *organizationRepository) restoreSharedCopies(ctx context.Context, tx *sql.Tx, organizationID, sourceAPIKeyID, onlyMemberUserID int64) (int, error) {
	res, err := tx.ExecContext(ctx, `WITH target AS (
			SELECT ok.api_key_id FROM organization_api_keys ok
			JOIN organization_api_keys src ON src.api_key_id=ok.source_api_key_id
			JOIN api_keys sk ON sk.id=src.api_key_id AND sk.deleted_at IS NULL AND sk.status='active'
			JOIN api_keys ck ON ck.id=ok.api_key_id AND ck.deleted_at IS NULL
			JOIN organization_members m ON m.organization_id=ok.organization_id AND m.user_id=ok.member_user_id AND m.status='active'
			WHERE ok.organization_id=$1 AND ok.status='inactive'
			AND src.organization_id=$1 AND src.status='active' AND src.shared_with_team
			AND ($2::bigint=0 OR ok.source_api_key_id=$2::bigint)
			AND ($3::bigint=0 OR ok.member_user_id=$3::bigint)
		), activated AS (
			UPDATE api_keys SET status='active',updated_at=NOW() WHERE id IN (SELECT api_key_id FROM target) RETURNING id
		)
		UPDATE organization_api_keys SET status='active' WHERE api_key_id IN (SELECT id FROM activated)`,
		organizationID, sourceAPIKeyID, onlyMemberUserID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// cloneSharedKey 给符合条件且还没有副本的成员各发一份源 Key 的副本，返回新建的副本数。
// onlyMemberUserID 传 0 表示团队里所有在团成员。
//
// 跳过的成员：源 Key 的创建者本人、已停用 / 已移除成员、已有副本的成员（含已停用副本，由 restore 负责恢复）、
// 账号非正常状态的用户、没有权限使用源 Key 所绑专属分组的用户（否则副本调用时会被 GROUP_NOT_ALLOWED 拒绝）。
func (r *organizationRepository) cloneSharedKey(ctx context.Context, tx *sql.Tx, organizationID, sourceAPIKeyID, onlyMemberUserID int64) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT m.user_id FROM organization_members m
		JOIN users u ON u.id=m.user_id AND u.deleted_at IS NULL AND u.status='active'
		JOIN api_keys k ON k.id=$2 AND k.deleted_at IS NULL AND k.status='active' AND (k.expires_at IS NULL OR k.expires_at>NOW())
		LEFT JOIN groups g ON g.id=k.group_id
		WHERE m.organization_id=$1 AND m.status='active' AND m.user_id<>k.user_id
		AND ($3::bigint=0 OR m.user_id=$3::bigint)
		AND (g.id IS NULL OR g.is_exclusive=FALSE OR EXISTS (SELECT 1 FROM user_allowed_groups ug WHERE ug.user_id=m.user_id AND ug.group_id=g.id))
		AND NOT EXISTS (SELECT 1 FROM organization_api_keys c WHERE c.source_api_key_id=k.id AND c.member_user_id=m.user_id)
		ORDER BY m.user_id`, organizationID, sourceAPIKeyID, onlyMemberUserID)
	if err != nil {
		return 0, err
	}
	members := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		members = append(members, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	for _, memberID := range members {
		key, err := r.newSharedAPIKey()
		if err != nil {
			return 0, err
		}
		var newID int64
		// 显式 ::bigint / ::text 转换：INSERT ... SELECT 里裸的 $n 推不出类型。
		err = tx.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name,group_id,status,ip_whitelist,ip_blacklist,quota,quota_used,expires_at,
				rate_limit_5h,rate_limit_1d,rate_limit_7d,usage_5h,usage_1d,usage_7d,created_at,updated_at)
			SELECT $2::bigint,$3::text,name,group_id,'active',ip_whitelist,ip_blacklist,quota,0,expires_at,
				rate_limit_5h,rate_limit_1d,rate_limit_7d,0,0,0,NOW(),NOW()
			FROM api_keys WHERE id=$1 RETURNING id`, sourceAPIKeyID, memberID, key).Scan(&newID)
		if err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO organization_api_keys(api_key_id,organization_id,member_user_id,status,source_api_key_id)
			VALUES($1,$2,$3,'active',$4)`, newID, organizationID, memberID, sourceAPIKeyID); err != nil {
			return 0, err
		}
	}
	return len(members), nil
}
