package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 视频任务记录页（用户端）的只读查询。所有查询都强制带 user_id 条件，
// 即使任务 ID 被猜中，也无法读到其他用户的任务。

const ppVideoTaskRecordBaseColumns = `
		t.local_task_id, COALESCE(t.task_id, ''), COALESCE(ak.name, ''), t.platform, t.operation, t.model,
		t.status, t.billing_status, COALESCE(t.prompt, ''), t.video_count, t.video_resolution,
		t.requested_video_duration_seconds, t.requested_video_duration_milliseconds,
		t.generated_video_duration_milliseconds, t.hold_amount, t.actual_cost,
		COALESCE(t.last_error_code, ''), COALESCE(t.last_error_message, ''),
		t.created_at, t.finished_at, t.settled_at`

// ppVideoTaskRecordColumns 在基础列之后追加 failure_response_body：
// 仅当任务失败且没有记录错误信息时才读取保存的上游响应（截取前 8KB），
// 供记录页从中提取失败原因；其余任务返回空串，避免在列表里搬运大字段。
var ppVideoTaskRecordColumns = ppVideoTaskRecordBaseColumns + `,
		` + ppVideoTaskFailureBodyExpr()

func ppVideoTaskFailureBodyExpr() string {
	statuses := service.PPVideoTaskStatusValues(service.PPVideoTaskStatusFailed)
	quoted := make([]string, 0, len(statuses))
	for _, status := range statuses {
		// 状态值来自服务层写死的常量，不含用户输入。
		quoted = append(quoted, "'"+status+"'")
	}
	return "CASE WHEN COALESCE(t.last_error_message, '') = '' AND LOWER(t.status) IN (" + strings.Join(quoted, ", ") +
		") THEN LEFT(t.response_body, " + strconv.Itoa(service.PPVideoFailureBodyMaxChars) + ") ELSE '' END"
}

const ppVideoTaskRecordFrom = `
		FROM pp_video_tasks t
		LEFT JOIN api_keys ak ON ak.id = t.api_key_id`

// buildPPVideoTaskListWhere 生成列表 / 计数共用的 WHERE 子句与参数。user_id 恒为 $1。
func buildPPVideoTaskListWhere(userID int64, filter service.PPVideoTaskListFilter) (string, []any) {
	clauses := []string{"t.user_id = $1"}
	args := []any{userID}
	next := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}

	if filter.TaskID != "" {
		placeholder := next("%" + escapeLike(filter.TaskID) + "%")
		clauses = append(clauses, "(t.local_task_id ILIKE "+placeholder+" OR COALESCE(t.task_id, '') ILIKE "+placeholder+")")
	}
	if filter.Model != "" {
		clauses = append(clauses, "t.model ILIKE "+next("%"+escapeLike(filter.Model)+"%"))
	}
	if filter.APIKey != "" {
		clauses = append(clauses, "COALESCE(ak.name, '') ILIKE "+next("%"+escapeLike(filter.APIKey)+"%"))
	}
	if statuses := service.PPVideoTaskStatusValues(filter.Status); len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, next(status))
		}
		clauses = append(clauses, "LOWER(t.status) IN ("+strings.Join(placeholders, ", ")+")")
	}
	if filter.StartTime != nil {
		clauses = append(clauses, "t.created_at >= "+next(*filter.StartTime))
	}
	if filter.EndTime != nil {
		clauses = append(clauses, "t.created_at < "+next(*filter.EndTime))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *usageBillingRepository) ListPPVideoTasksForUser(ctx context.Context, userID int64, filter service.PPVideoTaskListFilter) ([]*service.PPVideoTaskRecord, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, sql.ErrConnDone
	}
	filter = filter.Normalized()
	where, args := buildPPVideoTaskListWhere(userID, filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+ppVideoTaskRecordFrom+` WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*service.PPVideoTaskRecord{}, 0, nil
	}

	limitPlaceholder := "$" + strconv.Itoa(len(args)+1)
	offsetPlaceholder := "$" + strconv.Itoa(len(args)+2)
	listArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.QueryContext(ctx,
		`SELECT`+ppVideoTaskRecordColumns+ppVideoTaskRecordFrom+` WHERE `+where+
			` ORDER BY t.created_at DESC, t.id DESC LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder,
		listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	records := make([]*service.PPVideoTaskRecord, 0, filter.PageSize)
	for rows.Next() {
		record, scanErr := scanPPVideoTaskRecord(rows)
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

func (r *usageBillingRepository) GetPPVideoTaskRecordForUser(ctx context.Context, userID int64, taskID string) (*service.PPVideoTaskRecord, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT`+ppVideoTaskRecordColumns+ppVideoTaskRecordFrom+`
		WHERE t.user_id = $1
			AND (t.local_task_id = $2 OR t.task_id = $2)
		ORDER BY t.id DESC
		LIMIT 1`,
		userID, strings.TrimSpace(taskID))
	record, err := scanPPVideoTaskRecord(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return record, nil
}

func (r *usageBillingRepository) GetPPVideoTaskForUser(ctx context.Context, userID int64, taskID string) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+ppVideoTaskColumns()+`
		FROM pp_video_tasks
		WHERE user_id = $1
			AND (task_id = $2 OR local_task_id = $2)
		ORDER BY id DESC
		LIMIT 1
	`, userID, strings.TrimSpace(taskID))
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return task, nil
}

// SetPPVideoTaskPrompt 只在提示词仍为空时写入，不覆盖已有内容，也不改 updated_at（避免干扰轮询排序）。
func (r *usageBillingRepository) SetPPVideoTaskPrompt(ctx context.Context, localTaskID, prompt string) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE pp_video_tasks
		SET prompt = $2
		WHERE local_task_id = $1
			AND prompt = ''
	`, strings.TrimSpace(localTaskID), prompt)
	return err
}

func scanPPVideoTaskRecord(row ppVideoTaskScanner) (*service.PPVideoTaskRecord, error) {
	var record service.PPVideoTaskRecord
	var actualCost sql.NullFloat64
	var finishedAt, settledAt sql.NullTime
	if err := row.Scan(
		&record.LocalTaskID,
		&record.TaskID,
		&record.APIKeyName,
		&record.Platform,
		&record.Operation,
		&record.Model,
		&record.Status,
		&record.BillingStatus,
		&record.Prompt,
		&record.VideoCount,
		&record.VideoResolution,
		&record.RequestedVideoDurationSeconds,
		&record.RequestedVideoDurationMilliseconds,
		&record.GeneratedVideoDurationMilliseconds,
		&record.HoldAmount,
		&actualCost,
		&record.LastErrorCode,
		&record.LastErrorMessage,
		&record.CreatedAt,
		&finishedAt,
		&settledAt,
		&record.FailureResponseBody,
	); err != nil {
		return nil, err
	}
	if actualCost.Valid {
		record.ActualCost = &actualCost.Float64
	}
	if finishedAt.Valid {
		record.FinishedAt = &finishedAt.Time
	}
	if settledAt.Valid {
		record.SettledAt = &settledAt.Time
	}
	return &record, nil
}
