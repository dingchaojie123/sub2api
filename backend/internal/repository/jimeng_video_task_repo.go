package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type jimengVideoTaskScanner interface {
	Scan(dest ...any) error
}

func (r *usageBillingRepository) CreateJimengVideoTask(ctx context.Context, params service.CreateJimengVideoTaskParams) (*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO jimeng_video_tasks (
			local_task_id, user_id, api_key_id, group_id, account_id, model, status,
			billing_status, request_hash, idempotency_key, hold_id, capture_id,
			release_id, estimated_total_cost, hold_amount, currency, video_duration_seconds, video_resolution
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, NULLIF($10, ''), $11, $12,
			$13, $14, $15, $16, $17, $18
		)
		RETURNING `+jimengVideoTaskColumns(),
		strings.TrimSpace(params.LocalTaskID),
		params.UserID,
		params.APIKeyID,
		sqlNullInt64(params.GroupID),
		params.AccountID,
		strings.TrimSpace(params.Model),
		strings.TrimSpace(params.Status),
		strings.TrimSpace(params.BillingStatus),
		strings.TrimSpace(params.RequestHash),
		strings.TrimSpace(params.IdempotencyKey),
		strings.TrimSpace(params.HoldID),
		strings.TrimSpace(params.CaptureID),
		strings.TrimSpace(params.ReleaseID),
		params.EstimatedTotalCost,
		params.HoldAmount,
		strings.TrimSpace(params.Currency),
		params.VideoDurationSeconds,
		strings.TrimSpace(params.VideoResolution),
	)
	task, err := scanJimengVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, nil, service.ErrJimengVideoIdempotencyConflict)
	}
	return task, nil
}

func (r *usageBillingRepository) GetJimengVideoTaskByIdempotencyKey(ctx context.Context, userID int64, apiKeyID int64, idempotencyKey string) (*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+jimengVideoTaskColumns()+`
		FROM jimeng_video_tasks
		WHERE user_id = $1
			AND api_key_id = $2
			AND idempotency_key = $3
		ORDER BY id DESC
		LIMIT 1
	`, userID, apiKeyID, strings.TrimSpace(idempotencyKey))
	task, err := scanJimengVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrJimengVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) GetJimengVideoTaskForOwner(ctx context.Context, userID int64, apiKeyID int64, taskID string) (*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+jimengVideoTaskColumns()+`
		FROM jimeng_video_tasks
		WHERE user_id = $1
			AND api_key_id = $2
			AND (task_id = $3 OR local_task_id = $3)
		ORDER BY id DESC
		LIMIT 1
	`, userID, apiKeyID, strings.TrimSpace(taskID))
	task, err := scanJimengVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrJimengVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) MarkJimengVideoTaskSubmitted(ctx context.Context, params service.MarkJimengVideoSubmittedParams) (*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE jimeng_video_tasks
		SET task_id = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN NULLIF($2, '')
				ELSE task_id
			END,
			status = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN $3
				ELSE status
			END,
			response_status = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN $4
				ELSE response_status
			END,
			response_content_type = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN $5
				ELSE response_content_type
			END,
			response_body = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN $6
				ELSE response_body
			END,
			submitted_at = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN COALESCE(submitted_at, NOW())
				ELSE submitted_at
			END,
			updated_at = NOW()
		WHERE local_task_id = $1
		RETURNING `+jimengVideoTaskColumns(),
		strings.TrimSpace(params.LocalTaskID),
		strings.TrimSpace(params.TaskID),
		strings.TrimSpace(params.Status),
		params.ResponseStatus,
		strings.TrimSpace(params.ResponseContentType),
		params.ResponseBody,
	)
	task, err := scanJimengVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrJimengVideoTaskNotFound, service.ErrJimengVideoIdempotencyConflict)
	}
	return task, nil
}

func (r *usageBillingRepository) MarkJimengVideoTaskStatus(ctx context.Context, params service.MarkJimengVideoStatusParams) (*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	status := strings.TrimSpace(params.Status)
	row := r.db.QueryRowContext(ctx, `
		UPDATE jimeng_video_tasks
		SET status = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN $2
				ELSE status
			END,
			last_error_code = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN NULLIF($3, '')
				ELSE last_error_code
			END,
			last_error_message = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') THEN NULLIF($4, '')
				ELSE last_error_message
			END,
			response_status = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $5 > 0 THEN $5
				ELSE response_status
			END,
			response_content_type = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND NULLIF($6, '') IS NOT NULL THEN $6
				ELSE response_content_type
			END,
			response_body = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $7 <> '' THEN $7
				ELSE response_body
			END,
			finished_at = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $2 IN ('succeeded', 'failed') THEN COALESCE(finished_at, NOW())
				ELSE finished_at
			END,
			updated_at = NOW()
		WHERE task_id = $1
		RETURNING `+jimengVideoTaskColumns(),
		strings.TrimSpace(params.TaskID),
		status,
		strings.TrimSpace(params.LastErrorCode),
		strings.TrimSpace(params.LastErrorMessage),
		params.ResponseStatus,
		strings.TrimSpace(params.ResponseContentType),
		params.ResponseBody,
	)
	task, err := scanJimengVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrJimengVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) MarkJimengVideoTaskSubmitFailed(ctx context.Context, localTaskID string, code string, message string) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	res, err := r.db.ExecContext(ctx, `
		WITH release_state AS (
			SELECT task.id,
				CASE
					WHEN task.billing_status IN ('none', 'settling_none') THEN true
					WHEN task.billing_status IN ('held', 'settling') THEN (
						task.hold_amount <= 0
						OR NOT EXISTS (
							SELECT 1
							FROM usage_billing_dedup hold_dedup
							WHERE hold_dedup.request_id = task.hold_id
								AND hold_dedup.api_key_id = task.api_key_id
						) AND NOT EXISTS (
							SELECT 1
							FROM usage_billing_dedup_archive hold_archive
							WHERE hold_archive.request_id = task.hold_id
								AND hold_archive.api_key_id = task.api_key_id
						)
						OR EXISTS (
							SELECT 1
							FROM usage_billing_dedup release_dedup
							WHERE release_dedup.request_id = task.release_id
								AND release_dedup.api_key_id = task.api_key_id
						)
						OR EXISTS (
							SELECT 1
							FROM usage_billing_dedup_archive release_archive
							WHERE release_archive.request_id = task.release_id
								AND release_archive.api_key_id = task.api_key_id
						)
					)
					ELSE false
				END AS can_mark_released
			FROM jimeng_video_tasks task
			WHERE task.local_task_id = $1
		)
		UPDATE jimeng_video_tasks task
		SET status = CASE
				WHEN task.settled_at IS NULL THEN 'failed'
				ELSE task.status
			END,
			billing_status = CASE
				WHEN task.settled_at IS NULL AND release_state.can_mark_released THEN 'released'
				ELSE task.billing_status
			END,
			actual_cost = CASE
				WHEN task.settled_at IS NULL AND release_state.can_mark_released THEN 0
				ELSE task.actual_cost
			END,
			last_error_code = CASE
				WHEN task.settled_at IS NULL THEN NULLIF($2, '')
				ELSE task.last_error_code
			END,
			last_error_message = CASE
				WHEN task.settled_at IS NULL THEN NULLIF($3, '')
				ELSE task.last_error_message
			END,
			finished_at = CASE
				WHEN task.settled_at IS NULL THEN COALESCE(task.finished_at, NOW())
				ELSE task.finished_at
			END,
			settled_at = CASE
				WHEN task.settled_at IS NULL AND release_state.can_mark_released THEN NOW()
				ELSE task.settled_at
			END,
			updated_at = NOW()
		FROM release_state
		WHERE task.id = release_state.id
	`, strings.TrimSpace(localTaskID), strings.TrimSpace(code), strings.TrimSpace(message))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrJimengVideoTaskNotFound
	}
	return nil
}

func (r *usageBillingRepository) ClaimJimengVideoTaskSettlement(ctx context.Context, params service.ClaimJimengVideoSettlementParams) (*service.JimengVideoTask, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, sql.ErrConnDone
	}
	taskID := strings.TrimSpace(params.TaskID)
	finalStatus := strings.TrimSpace(params.FinalStatus)
	if taskID == "" || finalStatus == "" {
		return nil, false, service.ErrJimengVideoTaskNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE jimeng_video_tasks
		SET status = CASE
				WHEN billing_status IN ('held', 'none') THEN $2
				ELSE status
			END,
			billing_status = CASE
				WHEN billing_status IN ('none', 'settling_none') THEN 'settling_none'
				ELSE 'settling'
			END,
			finished_at = CASE
				WHEN $2 IN ('succeeded', 'failed') THEN COALESCE(finished_at, NOW())
				ELSE finished_at
			END,
			updated_at = NOW()
		WHERE task_id = $1
			AND settled_at IS NULL
			AND (
				billing_status IN ('held', 'none')
				OR (billing_status IN ('settling', 'settling_none') AND status = $2)
			)
		RETURNING `+jimengVideoTaskColumns(),
		taskID,
		finalStatus,
	)
	task, err := scanJimengVideoTask(row)
	if err == nil {
		return task, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return nil, false, err
}

func (r *usageBillingRepository) MarkJimengVideoTaskSettled(ctx context.Context, params service.MarkJimengVideoSettledParams) (*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE jimeng_video_tasks
		SET status = $2,
			billing_status = $3,
			actual_cost = $4,
			last_error_code = NULLIF($5, ''),
			last_error_message = NULLIF($6, ''),
			finished_at = COALESCE(finished_at, NOW()),
			settled_at = COALESCE(settled_at, NOW()),
			updated_at = NOW()
		WHERE task_id = $1
			AND billing_status IN ('settling', 'settling_none')
			AND settled_at IS NULL
		RETURNING `+jimengVideoTaskColumns(),
		strings.TrimSpace(params.TaskID),
		strings.TrimSpace(params.Status),
		strings.TrimSpace(params.BillingStatus),
		params.ActualCost,
		strings.TrimSpace(params.LastErrorCode),
		strings.TrimSpace(params.LastErrorMessage),
	)
	task, err := scanJimengVideoTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existing, existingErr := r.getJimengVideoTaskByTaskID(ctx, params.TaskID)
			if existingErr == nil {
				if existing != nil && existing.SettledAt != nil {
					return existing, nil
				}
				return nil, errors.New("jimeng video task settlement was not claimed")
			}
			return nil, translatePersistenceError(existingErr, service.ErrJimengVideoTaskNotFound, nil)
		}
		return nil, translatePersistenceError(err, service.ErrJimengVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) getJimengVideoTaskByTaskID(ctx context.Context, taskID string) (*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+jimengVideoTaskColumns()+`
		FROM jimeng_video_tasks
		WHERE task_id = $1
		LIMIT 1
	`, strings.TrimSpace(taskID))
	task, err := scanJimengVideoTask(row)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (r *usageBillingRepository) ClaimJimengVideoTasksForPolling(ctx context.Context, params service.ClaimJimengVideoTasksForPollingParams) ([]*service.JimengVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	leaseOwner := strings.TrimSpace(params.LeaseOwner)
	if leaseOwner == "" {
		return nil, errors.New("jimeng video poll lease owner is required")
	}
	limit := params.Limit
	if limit <= 0 {
		return nil, nil
	}
	leaseUntil := params.LeaseUntil
	if leaseUntil.IsZero() {
		leaseUntil = time.Now().UTC().Add(2 * time.Minute)
	}
	submitCutoff := params.SubmitCutoff
	if submitCutoff.IsZero() {
		submitCutoff = time.Now().UTC().Add(-10 * time.Minute)
	}
	rows, err := r.db.QueryContext(ctx, `
		UPDATE jimeng_video_tasks AS task
		SET poll_lease_owner = $1,
			poll_lease_until = $3,
			poll_attempts = COALESCE(task.poll_attempts, 0) + 1,
			last_polled_at = NOW(),
			updated_at = NOW()
		WHERE task.id IN (
			SELECT candidate.id
			FROM jimeng_video_tasks AS candidate
			WHERE candidate.billing_status IN ('held', 'none', 'settling', 'settling_none')
				AND candidate.settled_at IS NULL
				AND (candidate.poll_lease_until IS NULL OR candidate.poll_lease_until <= NOW())
				AND (
					NULLIF(candidate.task_id, '') IS NOT NULL
					OR candidate.status IN ('processing', 'succeeded', 'failed')
					OR (candidate.status = 'submitting' AND candidate.submitted_at IS NOT NULL)
					OR (candidate.status = 'submitting' AND candidate.submitted_at IS NULL AND candidate.created_at <= $4)
				)
			ORDER BY COALESCE(candidate.submitted_at, candidate.created_at), candidate.id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING `+jimengVideoTaskColumns(),
		leaseOwner,
		limit,
		leaseUntil,
		submitCutoff,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tasks []*service.JimengVideoTask
	for rows.Next() {
		task, err := scanJimengVideoTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *usageBillingRepository) ReleaseJimengVideoTaskPollLease(ctx context.Context, localTaskID string, leaseOwner string) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE jimeng_video_tasks
		SET poll_lease_owner = NULL,
			poll_lease_until = NULL,
			updated_at = NOW()
		WHERE local_task_id = $1
			AND poll_lease_owner = $2
	`, strings.TrimSpace(localTaskID), strings.TrimSpace(leaseOwner))
	return err
}

func jimengVideoTaskColumns() string {
	return `
		id, local_task_id, COALESCE(task_id, ''), user_id, api_key_id, group_id,
		account_id, model, status, billing_status, request_hash,
		COALESCE(idempotency_key, ''), hold_id, capture_id, release_id,
		estimated_total_cost, hold_amount, actual_cost, currency, video_duration_seconds,
		video_resolution, response_status, response_content_type, response_body,
		COALESCE(last_error_code, ''), COALESCE(last_error_message, ''),
		created_at, updated_at, submitted_at, finished_at, settled_at
	`
}

func scanJimengVideoTask(row jimengVideoTaskScanner) (*service.JimengVideoTask, error) {
	var task service.JimengVideoTask
	var groupID sql.NullInt64
	var actualCost sql.NullFloat64
	var submittedAt, finishedAt, settledAt sql.NullTime
	if err := row.Scan(
		&task.ID,
		&task.LocalTaskID,
		&task.TaskID,
		&task.UserID,
		&task.APIKeyID,
		&groupID,
		&task.AccountID,
		&task.Model,
		&task.Status,
		&task.BillingStatus,
		&task.RequestHash,
		&task.IdempotencyKey,
		&task.HoldID,
		&task.CaptureID,
		&task.ReleaseID,
		&task.EstimatedTotalCost,
		&task.HoldAmount,
		&actualCost,
		&task.Currency,
		&task.VideoDurationSeconds,
		&task.VideoResolution,
		&task.ResponseStatus,
		&task.ResponseContentType,
		&task.ResponseBody,
		&task.LastErrorCode,
		&task.LastErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&submittedAt,
		&finishedAt,
		&settledAt,
	); err != nil {
		return nil, err
	}
	if groupID.Valid {
		task.GroupID = &groupID.Int64
	}
	if actualCost.Valid {
		task.ActualCost = &actualCost.Float64
	}
	if submittedAt.Valid {
		task.SubmittedAt = &submittedAt.Time
	}
	if finishedAt.Valid {
		task.FinishedAt = &finishedAt.Time
	}
	if settledAt.Valid {
		task.SettledAt = &settledAt.Time
	}
	return &task, nil
}

func sqlNullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
