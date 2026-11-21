package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 管理后台「视频任务」的只读查询：跨用户，不带 user_id 限定。
// 调用方（管理端路由）必须已通过管理员鉴权；这里不做访问控制。

const ppVideoTaskAdminFrom = ppVideoTaskRecordFrom + `
		LEFT JOIN users u ON u.id = t.user_id
		LEFT JOIN accounts a ON a.id = t.account_id
		LEFT JOIN groups g ON g.id = t.group_id`

// ppVideoTaskAdminColumns 在用户端列之后追加归属信息，顺序与 scanPPVideoTaskAdminRecord 一致。
var ppVideoTaskAdminColumns = ppVideoTaskRecordColumns + `,
		t.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
		t.api_key_id, t.account_id, COALESCE(a.name, ''),
		t.group_id, COALESCE(g.name, '')`

// buildPPVideoTaskAdminWhere 生成管理端列表 / 计数共用的 WHERE 子句与参数。
// 没有任何筛选时返回恒真条件，调用方可直接拼接 "WHERE " + where。
func buildPPVideoTaskAdminWhere(filter service.PPVideoTaskAdminListFilter) (string, []any) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)

	if filter.UserID > 0 {
		args = append(args, filter.UserID)
		clauses = append(clauses, "t.user_id = $"+strconv.Itoa(len(args)))
	}
	if filter.User != "" {
		args = append(args, "%"+escapeLike(filter.User)+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		clauses = append(clauses, "(COALESCE(u.email, '') ILIKE "+placeholder+" OR COALESCE(u.username, '') ILIKE "+placeholder+")")
	}
	if filter.Platform != "" {
		args = append(args, "%"+escapeLike(filter.Platform)+"%")
		clauses = append(clauses, "t.platform ILIKE $"+strconv.Itoa(len(args)))
	}
	clauses, args = appendPPVideoTaskFilterClauses(clauses, args, filter.PPVideoTaskListFilter)

	if len(clauses) == 0 {
		return "TRUE", args
	}
	return strings.Join(clauses, " AND "), args
}

func (r *usageBillingRepository) ListPPVideoTasksForAdmin(ctx context.Context, filter service.PPVideoTaskAdminListFilter) ([]*service.PPVideoTaskAdminRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, sql.ErrConnDone
	}
	filter = filter.Normalized()
	where, args := buildPPVideoTaskAdminWhere(filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+ppVideoTaskAdminFrom+` WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*service.PPVideoTaskAdminRecord{}, 0, nil
	}

	limitPlaceholder := "$" + strconv.Itoa(len(args)+1)
	offsetPlaceholder := "$" + strconv.Itoa(len(args)+2)
	listArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+ppVideoTaskAdminColumns+ppVideoTaskAdminFrom+` WHERE `+where+
			` ORDER BY t.created_at DESC, t.id DESC LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder,
		listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	records := make([]*service.PPVideoTaskAdminRecord, 0, filter.PageSize)
	for rows.Next() {
		record, scanErr := scanPPVideoTaskAdminRecord(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *usageBillingRepository) GetPPVideoTaskRecordForAdmin(ctx context.Context, taskID string) (*service.PPVideoTaskAdminRecord, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT`+ppVideoTaskAdminColumns+ppVideoTaskAdminFrom+`
		WHERE t.local_task_id = $1 OR t.task_id = $1
		ORDER BY t.id DESC
		LIMIT 1`,
		strings.TrimSpace(taskID))
	record, err := scanPPVideoTaskAdminRecord(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return record, nil
}

func (r *usageBillingRepository) GetPPVideoTaskForAdmin(ctx context.Context, taskID string) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+ppVideoTaskColumns()+`
		FROM pp_video_tasks
		WHERE task_id = $1 OR local_task_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, strings.TrimSpace(taskID))
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return task, nil
}

func scanPPVideoTaskAdminRecord(row ppVideoTaskScanner) (*service.PPVideoTaskAdminRecord, error) {
	var record service.PPVideoTaskAdminRecord
	var groupID sql.NullInt64
	if err := scanPPVideoTaskRecordInto(row, &record.PPVideoTaskRecord,
		&record.UserID, &record.UserEmail, &record.Username,
		&record.APIKeyID, &record.AccountID, &record.AccountName,
		&groupID, &record.GroupName,
	); err != nil {
		return nil, err
	}
	if groupID.Valid {
		record.GroupID = &groupID.Int64
	}
	return &record, nil
}

var _ service.PPVideoTaskAdminQueryRepository = (*usageBillingRepository)(nil)
