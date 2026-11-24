package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 团队「成员明细」查询（仅超级管理员 / 管理员）。内容范围与归属规则见 service/organization_member_detail.go。

// memberDetailAccess 校验操作者是该团队的超级管理员 / 管理员，并确认目标成员属于该团队
// （包括已被移除的成员：他们之前的用量与任务记录仍然可查）。返回成员的邮箱和用户名。
func (r *organizationRepository) memberDetailAccess(ctx context.Context, organizationID, actorUserID, memberUserID int64) (email, username string, err error) {
	org, err := r.GetForUser(ctx, organizationID, actorUserID)
	if err != nil {
		return "", "", err
	}
	if org.Role != service.OrganizationRoleOwner && org.Role != service.OrganizationRoleAdmin {
		return "", "", service.ErrOrganizationForbidden
	}
	err = r.db.QueryRowContext(ctx,
		`SELECT u.email,u.username FROM organization_members m JOIN users u ON u.id=m.user_id WHERE m.organization_id=$1 AND m.user_id=$2`,
		organizationID, memberUserID).Scan(&email, &username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", service.ErrOrganizationNotFound
	}
	return email, username, err
}

func (r *organizationRepository) MemberUsageDetail(ctx context.Context, organizationID, actorUserID, memberUserID int64, start, end time.Time) (*service.OrganizationMemberUsageDetail, error) {
	email, username, err := r.memberDetailAccess(ctx, organizationID, actorUserID, memberUserID)
	if err != nil {
		return nil, err
	}
	detail := &service.OrganizationMemberUsageDetail{
		UserID:   memberUserID,
		Email:    email,
		Username: username,
		Models:   []service.OrganizationMemberModelUsage{},
	}
	rows, err := r.db.QueryContext(ctx, `SELECT model,COUNT(*),COALESCE(SUM(actual_cost),0),COALESCE(SUM(input_tokens),0),COALESCE(SUM(output_tokens),0),COALESCE(SUM(image_count),0),COALESCE(SUM(video_count),0),MAX(created_at)
		FROM usage_logs
		WHERE organization_id=$1::bigint AND user_id=$2::bigint AND created_at >= $3::timestamptz AND created_at < $4::timestamptz
		GROUP BY model ORDER BY COALESCE(SUM(actual_cost),0) DESC,COUNT(*) DESC`, organizationID, memberUserID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m service.OrganizationMemberModelUsage
		var last sql.NullTime
		if err := rows.Scan(&m.Model, &m.TotalRequests, &m.TotalCost, &m.InputTokens, &m.OutputTokens, &m.ImageCount, &m.VideoCount, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			t := last.Time
			m.LastUsedAt = &t
		}
		detail.Models = append(detail.Models, m)
		detail.Summary.TotalRequests += m.TotalRequests
		detail.Summary.TotalCost += m.TotalCost
		detail.Summary.InputTokens += m.InputTokens
		detail.Summary.OutputTokens += m.OutputTokens
		detail.Summary.ImageCount += m.ImageCount
		detail.Summary.VideoCount += m.VideoCount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return detail, nil
}

// memberTasksCTE 把视频任务和批量图片任务统一成同一组列。任务归属用提交时固化的
// billing_organization_id / billing_member_user_id，不依赖 Key 当前的绑定关系。
const memberTasksCTE = `WITH tasks AS (
	SELECT 'video'::text AS kind, t.local_task_id::text AS task_id, t.model::text AS model, t.status::text AS status,
		COALESCE(t.prompt,'')::text AS prompt, t.video_count::int AS item_count, 0::int AS success_count,
		COALESCE(t.video_resolution,'')::text AS resolution, t.requested_video_duration_seconds::int AS duration_seconds,
		t.actual_cost::float8 AS cost, COALESCE(t.last_error_message,'')::text AS error_message,
		t.created_at AS created_at, t.finished_at AS finished_at,
		t.platform::text AS platform, t.billing_status::text AS billing_status
	FROM pp_video_tasks t
	WHERE t.billing_organization_id=$1::bigint AND t.billing_member_user_id=$2::bigint
		AND t.created_at >= $3::timestamptz AND t.created_at < $4::timestamptz
	UNION ALL
	SELECT 'image_batch'::text, j.batch_id::text, j.model::text, j.status::text,
		COALESCE((SELECT string_agg(p.prompt_preview, E'\n' ORDER BY p.id)
			FROM (SELECT i.id, i.prompt_preview FROM batch_image_items i
				WHERE i.job_id=j.batch_id AND COALESCE(i.prompt_preview,'') <> '' ORDER BY i.id LIMIT 3) p),'')::text,
		j.item_count::int, j.success_count::int, ''::text, 0::int,
		j.actual_cost::float8, COALESCE(j.last_error_message,'')::text,
		j.created_at, j.finished_at, ''::text, ''::text
	FROM batch_image_jobs j
	WHERE j.billing_organization_id=$1::bigint AND j.billing_member_user_id=$2::bigint
		AND j.created_at >= $3::timestamptz AND j.created_at < $4::timestamptz
)`

func (r *organizationRepository) MemberTasks(ctx context.Context, organizationID, actorUserID, memberUserID int64, start, end time.Time, limit, offset int) ([]service.OrganizationMemberTask, int64, error) {
	if _, _, err := r.memberDetailAccess(ctx, organizationID, actorUserID, memberUserID); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, memberTasksCTE+` SELECT COUNT(*) FROM tasks`, organizationID, memberUserID, start, end).Scan(&total); err != nil {
		return nil, 0, err
	}
	items := []service.OrganizationMemberTask{}
	if total == 0 {
		return items, 0, nil
	}
	rows, err := r.db.QueryContext(ctx, memberTasksCTE+` SELECT kind,task_id,model,status,prompt,item_count,success_count,resolution,duration_seconds,cost,error_message,created_at,finished_at,platform,billing_status
		FROM tasks ORDER BY created_at DESC,task_id DESC LIMIT $5 OFFSET $6`, organizationID, memberUserID, start, end, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.OrganizationMemberTask
		var cost sql.NullFloat64
		var finished sql.NullTime
		var platform, billingStatus string
		if err := rows.Scan(&item.Kind, &item.TaskID, &item.Model, &item.Status, &item.Prompt, &item.ItemCount, &item.SuccessCount, &item.Resolution, &item.DurationSeconds, &cost, &item.ErrorMessage, &item.CreatedAt, &finished, &platform, &billingStatus); err != nil {
			return nil, 0, err
		}
		if item.Kind == service.OrganizationMemberTaskKindVideo {
			item.CanPlay = service.PPVideoTaskContentAvailable(platform, item.Status, billingStatus)
			item.Status = service.PPVideoTaskStatusGroup(item.Status)
		} else {
			item.Status = memberImageBatchStatusGroup(item.Status)
		}
		if cost.Valid {
			v := cost.Float64
			item.Cost = &v
		}
		if finished.Valid {
			t := finished.Time
			item.FinishedAt = &t
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// memberImageBatchStatusGroup 把批量图片任务的状态归并成与视频任务一致的四类展示状态。
// completed / output_deleted（结果已过期清理）算成功，failed / cancelled 算失败，其余视为处理中。
func memberImageBatchStatusGroup(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "output_deleted":
		return service.PPVideoTaskStatusSucceeded
	case "failed", "cancelled", "canceled":
		return service.PPVideoTaskStatusFailed
	default:
		return service.PPVideoTaskStatusProcessing
	}
}
