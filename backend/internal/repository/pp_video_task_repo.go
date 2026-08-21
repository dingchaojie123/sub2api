package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ppVideoTaskScanner interface {
	Scan(dest ...any) error
}

func (r *usageBillingRepository) CreatePPVideoTask(ctx context.Context, params service.CreatePPVideoTaskParams) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
			INSERT INTO pp_video_tasks (
				local_task_id, user_id, api_key_id, group_id, account_id, platform, operation, model, status,
				billing_status, request_hash, idempotency_key, hold_id, capture_id,
				release_id, estimated_total_cost, hold_amount, currency, requested_video_duration_seconds,
				requested_video_duration_milliseconds, input_video_duration_milliseconds,
				generated_video_duration_milliseconds, video_count, video_resolution,
				output_width, output_height, frame_rate, has_audio, kling_mode,
				billing_formula, billing_units, billing_unit_price
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9,
				$10, $11, NULLIF($12, ''), $13, $14,
				$15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
				$25, $26, $27, $28, $29, $30, $31, $32
			)
			RETURNING `+ppVideoTaskColumns(),
		strings.TrimSpace(params.LocalTaskID),
		params.UserID,
		params.APIKeyID,
		sqlNullInt64(params.GroupID),
		params.AccountID,
		strings.TrimSpace(params.Platform),
		strings.TrimSpace(string(params.Operation)),
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
		params.RequestedVideoDurationSeconds,
		params.RequestedVideoDurationMilliseconds,
		params.InputVideoDurationMilliseconds,
		int64(0),
		params.VideoCount,
		strings.TrimSpace(params.VideoResolution),
		params.OutputWidth,
		params.OutputHeight,
		params.FrameRate,
		params.HasAudio,
		strings.TrimSpace(params.KlingMode),
		strings.TrimSpace(params.BillingFormula),
		params.BillingUnits,
		params.BillingUnitPrice,
	)
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, nil, service.ErrPPVideoIdempotencyConflict)
	}
	return task, nil
}

func (r *usageBillingRepository) GetPPVideoTaskByIdempotencyKey(ctx context.Context, userID int64, apiKeyID int64, idempotencyKey string) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+ppVideoTaskColumns()+`
		FROM pp_video_tasks
		WHERE user_id = $1
			AND api_key_id = $2
			AND idempotency_key = $3
		ORDER BY id DESC
		LIMIT 1
	`, userID, apiKeyID, strings.TrimSpace(idempotencyKey))
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) GetPPVideoTaskForOwner(ctx context.Context, userID int64, apiKeyID int64, taskID string) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+ppVideoTaskColumns()+`
		FROM pp_video_tasks
		WHERE user_id = $1
			AND api_key_id = $2
			AND (task_id = $3 OR local_task_id = $3)
		ORDER BY id DESC
		LIMIT 1
	`, userID, apiKeyID, strings.TrimSpace(taskID))
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) MarkPPVideoTaskSubmitted(ctx context.Context, params service.MarkPPVideoTaskSubmittedParams) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE pp_video_tasks
		SET task_id = NULLIF($2, ''),
			status = $3,
			response_status = $4,
			response_content_type = $5,
			response_body = $6,
			submitted_at = COALESCE(submitted_at, NOW()),
			updated_at = NOW()
		WHERE local_task_id = $1
			AND status = 'submitting'
			AND (task_id IS NULL OR task_id = '')
			AND NULLIF($2, '') IS NOT NULL
			AND settled_at IS NULL
			AND billing_status IN ('held', 'none')
		RETURNING `+ppVideoTaskColumns(),
		strings.TrimSpace(params.LocalTaskID),
		strings.TrimSpace(params.TaskID),
		strings.TrimSpace(params.Status),
		params.ResponseStatus,
		strings.TrimSpace(params.ResponseContentType),
		params.ResponseBody,
	)
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, service.ErrPPVideoIdempotencyConflict)
	}
	return task, nil
}

func (r *usageBillingRepository) MarkPPVideoTaskStatus(ctx context.Context, params service.MarkPPVideoTaskStatusParams) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	status := strings.TrimSpace(params.Status)
	row := r.db.QueryRowContext(ctx, `
		UPDATE pp_video_tasks
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
				generated_video_duration_milliseconds = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $5 > 0 THEN $5
					ELSE generated_video_duration_milliseconds
				END,
				video_count = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $6 > 0 THEN $6
					ELSE video_count
				END,
				video_resolution = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND NULLIF($7, '') IS NOT NULL THEN $7
					ELSE video_resolution
				END,
				output_width = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $8 > 0 THEN $8
					ELSE output_width
				END,
				output_height = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $9 > 0 THEN $9
					ELSE output_height
				END,
				frame_rate = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $10 > 0 THEN $10
					ELSE frame_rate
				END,
				input_video_duration_milliseconds = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $11 > 0 THEN $11
					ELSE input_video_duration_milliseconds
				END,
				response_status = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $12 > 0 THEN $12
					ELSE response_status
				END,
				response_content_type = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND NULLIF($13, '') IS NOT NULL THEN $13
					ELSE response_content_type
				END,
				response_body = CASE
					WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $14 <> '' THEN $14
					ELSE response_body
			END,
			finished_at = CASE
				WHEN settled_at IS NULL AND billing_status IN ('held', 'none') AND $2 IN ('succeeded', 'failed') THEN COALESCE(finished_at, NOW())
				ELSE finished_at
			END,
			updated_at = NOW()
		WHERE task_id = $1
		RETURNING `+ppVideoTaskColumns(),
		strings.TrimSpace(params.TaskID),
		status,
		strings.TrimSpace(params.LastErrorCode),
		strings.TrimSpace(params.LastErrorMessage),
		params.GeneratedVideoDurationMilliseconds,
		params.VideoCount,
		strings.TrimSpace(params.VideoResolution),
		params.OutputWidth,
		params.OutputHeight,
		params.FrameRate,
		params.InputVideoDurationMilliseconds,
		params.ResponseStatus,
		strings.TrimSpace(params.ResponseContentType),
		params.ResponseBody,
	)
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) MarkPPVideoTaskSubmitFailed(ctx context.Context, localTaskID string, code string, message string) error {
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
			FROM pp_video_tasks task
			WHERE task.local_task_id = $1
				AND task.status = 'submitting'
				AND (task.task_id IS NULL OR task.task_id = '')
				AND task.settled_at IS NULL
				AND task.billing_status IN ('held', 'none')
		)
		UPDATE pp_video_tasks task
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
			AND task.status = 'submitting'
			AND (task.task_id IS NULL OR task.task_id = '')
			AND task.settled_at IS NULL
			AND task.billing_status IN ('held', 'none')
	`, strings.TrimSpace(localTaskID), strings.TrimSpace(code), strings.TrimSpace(message))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrPPVideoTaskNotFound
	}
	return nil
}

func (r *usageBillingRepository) ClaimPPVideoTaskSettlement(ctx context.Context, params service.ClaimPPVideoTaskSettlementParams) (*service.PPVideoTask, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, sql.ErrConnDone
	}
	taskID := strings.TrimSpace(params.TaskID)
	finalStatus := strings.TrimSpace(params.FinalStatus)
	if taskID == "" || finalStatus == "" {
		return nil, false, service.ErrPPVideoTaskNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE pp_video_tasks
		SET status = CASE
				WHEN billing_status IN ('held', 'none') THEN $2
				ELSE status
			END,
			billing_status = CASE
				WHEN billing_status = 'none' THEN 'settling_none'
				ELSE 'settling'
			END,
			finished_at = CASE
				WHEN $2 IN ('succeeded', 'failed') THEN COALESCE(finished_at, NOW())
				ELSE finished_at
			END,
			updated_at = NOW()
		WHERE task_id = $1
			AND settled_at IS NULL
			AND billing_status IN ('held', 'none')
		RETURNING `+ppVideoTaskColumns(),
		taskID,
		finalStatus,
	)
	task, err := scanPPVideoTask(row)
	if err == nil {
		return task, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return nil, false, err
}

func (r *usageBillingRepository) MarkPPVideoTaskSettled(ctx context.Context, params service.MarkPPVideoTaskSettledParams) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE pp_video_tasks
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
		RETURNING `+ppVideoTaskColumns(),
		strings.TrimSpace(params.TaskID),
		strings.TrimSpace(params.Status),
		strings.TrimSpace(params.BillingStatus),
		params.ActualCost,
		strings.TrimSpace(params.LastErrorCode),
		strings.TrimSpace(params.LastErrorMessage),
	)
	task, err := scanPPVideoTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existing, existingErr := r.getPPVideoTaskByTaskID(ctx, params.TaskID)
			if existingErr == nil {
				if existing != nil && existing.SettledAt != nil {
					return existing, nil
				}
				return nil, errors.New("PP video task settlement was not claimed")
			}
			return nil, translatePersistenceError(existingErr, service.ErrPPVideoTaskNotFound, nil)
		}
		return nil, translatePersistenceError(err, service.ErrPPVideoTaskNotFound, nil)
	}
	return task, nil
}

func (r *usageBillingRepository) getPPVideoTaskByTaskID(ctx context.Context, taskID string) (*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+ppVideoTaskColumns()+`
		FROM pp_video_tasks
		WHERE task_id = $1
		LIMIT 1
	`, strings.TrimSpace(taskID))
	task, err := scanPPVideoTask(row)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (r *usageBillingRepository) ClaimPPVideoTasksForPolling(ctx context.Context, params service.ClaimPPVideoTasksForPollingParams) ([]*service.PPVideoTask, error) {
	if r == nil || r.db == nil {
		return nil, sql.ErrConnDone
	}
	leaseOwner := strings.TrimSpace(params.LeaseOwner)
	if leaseOwner == "" {
		return nil, errors.New("PP video poll lease owner is required")
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
	leaseToken := strings.TrimSpace(params.LeaseToken)
	if leaseToken == "" {
		var err error
		leaseToken, err = service.NewPPVideoPollLeaseToken()
		if err != nil {
			return nil, err
		}
	}
	rows, err := r.db.QueryContext(ctx, `
		UPDATE pp_video_tasks AS task
		SET poll_lease_owner = $1,
			poll_lease_token = $2,
			poll_lease_until = $4,
			poll_attempts = COALESCE(task.poll_attempts, 0) + 1,
			last_polled_at = NOW(),
			updated_at = NOW()
		WHERE task.id IN (
			SELECT candidate.id
			FROM pp_video_tasks AS candidate
			WHERE candidate.billing_status IN ('held', 'none', 'settling', 'settling_none')
				AND candidate.settled_at IS NULL
				AND (candidate.poll_lease_until IS NULL OR candidate.poll_lease_until <= NOW())
				AND (
					NULLIF(candidate.task_id, '') IS NOT NULL
					OR candidate.status IN ('processing', 'succeeded', 'failed')
					OR (candidate.status = 'submitting' AND candidate.submitted_at IS NOT NULL)
					OR (candidate.status = 'submitting' AND candidate.submitted_at IS NULL AND candidate.created_at <= $5)
				)
			ORDER BY COALESCE(candidate.submitted_at, candidate.created_at), candidate.id
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		RETURNING `+ppVideoTaskColumns(),
		leaseOwner,
		leaseToken,
		limit,
		leaseUntil,
		submitCutoff,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tasks []*service.PPVideoTask
	for rows.Next() {
		task, err := scanPPVideoTask(rows)
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

func (r *usageBillingRepository) ReleasePPVideoTaskPollLease(ctx context.Context, params service.ReleasePPVideoTaskPollLeaseParams) error {
	if r == nil || r.db == nil {
		return sql.ErrConnDone
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE pp_video_tasks
		SET poll_lease_owner = NULL,
			poll_lease_token = NULL,
			poll_lease_until = NULL,
			updated_at = NOW()
		WHERE local_task_id = $1
			AND poll_lease_owner = $2
			AND poll_lease_token = $3
	`, strings.TrimSpace(params.LocalTaskID), strings.TrimSpace(params.LeaseOwner), strings.TrimSpace(params.LeaseToken))
	return err
}

func ppVideoTaskColumns() string {
	return `
		id, local_task_id, COALESCE(task_id, ''), user_id, api_key_id, group_id,
		account_id, platform, operation, model, status, billing_status, request_hash,
		COALESCE(idempotency_key, ''), hold_id, capture_id, release_id,
		estimated_total_cost, hold_amount, actual_cost, currency, requested_video_duration_seconds,
		requested_video_duration_milliseconds, input_video_duration_milliseconds,
		generated_video_duration_milliseconds, video_count, video_resolution,
		output_width, output_height, frame_rate, has_audio, kling_mode,
		billing_formula, billing_units, billing_unit_price,
		response_status, response_content_type, response_body,
		COALESCE(last_error_code, ''), COALESCE(last_error_message, ''),
		COALESCE(poll_lease_token, ''), COALESCE(poll_lease_owner, ''), poll_lease_until, poll_attempts, last_polled_at,
		created_at, updated_at, submitted_at, finished_at, settled_at
	`
}

func scanPPVideoTask(row ppVideoTaskScanner) (*service.PPVideoTask, error) {
	var task service.PPVideoTask
	var groupID sql.NullInt64
	var actualCost sql.NullFloat64
	var pollLeaseUntil, lastPolledAt, submittedAt, finishedAt, settledAt sql.NullTime
	if err := row.Scan(
		&task.ID,
		&task.LocalTaskID,
		&task.TaskID,
		&task.UserID,
		&task.APIKeyID,
		&groupID,
		&task.AccountID,
		&task.Platform,
		&task.Operation,
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
		&task.RequestedVideoDurationSeconds,
		&task.RequestedVideoDurationMilliseconds,
		&task.InputVideoDurationMilliseconds,
		&task.GeneratedVideoDurationMilliseconds,
		&task.VideoCount,
		&task.VideoResolution,
		&task.OutputWidth,
		&task.OutputHeight,
		&task.FrameRate,
		&task.HasAudio,
		&task.KlingMode,
		&task.BillingFormula,
		&task.BillingUnits,
		&task.BillingUnitPrice,
		&task.ResponseStatus,
		&task.ResponseContentType,
		&task.ResponseBody,
		&task.LastErrorCode,
		&task.LastErrorMessage,
		&task.PollLeaseToken,
		&task.PollLeaseOwner,
		&pollLeaseUntil,
		&task.PollAttempts,
		&lastPolledAt,
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
	if pollLeaseUntil.Valid {
		task.PollLeaseUntil = &pollLeaseUntil.Time
	}
	if lastPolledAt.Valid {
		task.LastPolledAt = &lastPolledAt.Time
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
