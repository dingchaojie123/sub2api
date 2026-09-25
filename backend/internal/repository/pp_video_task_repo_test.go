package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestClaimPPVideoTasksForPollingLeasesHeldTasks(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)
	submitCutoff := now.Add(-10 * time.Minute)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)UPDATE pp_video_tasks AS task.*poll_lease_owner = \$1.*poll_lease_token = \$2.*poll_lease_until = \$4.*FOR UPDATE SKIP LOCKED.*RETURNING`).
		WithArgs("worker-a", "lease-token", 10, leaseUntil, submitCutoff).
		WillReturnRows(newPPVideoTaskRows(now).AddRow(
			int64(1), "vidtask_poll", "task_poll", int64(10), int64(20), nil, int64(30),
			service.PlatformHappyHourse, service.PPVideoOperationGeneric, "happyhorse-1.0-t2v",
			service.PPVideoTaskStatusProcessing, service.PPVideoBillingStatusHeld,
			"payload_hash", "idem", "hold", "capture", "release",
			5.0, 5.0, nil, "USD", 5, int64(5000), int64(0), int64(5041), 1, "720p",
			1280, 720, 24.0, false, "", service.PPVideoBillingFormulaPerSecond, 5.041, 0.86162, 0.0,
			200, "application/json", `{"status":"processing"}`,
			"", "", "lease-token", "worker-a", leaseUntil, 1, now, now, now, now, nil, nil,
		))

	repo := &usageBillingRepository{db: db}
	tasks, err := repo.ClaimPPVideoTasksForPolling(ctx, service.ClaimPPVideoTasksForPollingParams{
		LeaseOwner:   "worker-a",
		LeaseToken:   "lease-token",
		LeaseUntil:   leaseUntil,
		SubmitCutoff: submitCutoff,
		Limit:        10,
	})

	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, "vidtask_poll", tasks[0].LocalTaskID)
	require.Equal(t, "task_poll", tasks[0].TaskID)
	require.Equal(t, service.PPVideoBillingStatusHeld, tasks[0].BillingStatus)
	require.Equal(t, "lease-token", tasks[0].PollLeaseToken)
	require.Equal(t, "worker-a", tasks[0].PollLeaseOwner)
	require.Equal(t, 1, tasks[0].PollAttempts)
	require.NotNil(t, tasks[0].PollLeaseUntil)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleasePPVideoTaskPollLeaseIsBestEffort(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`(?s)UPDATE pp_video_tasks\s+SET poll_lease_owner = NULL,\s+poll_lease_token = NULL,\s+poll_lease_until = NULL,\s+updated_at = NOW\(\)\s+WHERE local_task_id = \$1\s+AND poll_lease_owner = \$2\s+AND poll_lease_token = \$3`).
		WithArgs("vidtask_poll", "worker-a", "lease-token").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := &usageBillingRepository{db: db}
	err = repo.ReleasePPVideoTaskPollLease(ctx, service.ReleasePPVideoTaskPollLeaseParams{
		LocalTaskID: "vidtask_poll",
		LeaseOwner:  "worker-a",
		LeaseToken:  "lease-token",
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePPVideoTaskPersistsPlatformOperationAndAccount(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)INSERT INTO pp_video_tasks.*account_id.*platform.*operation.*requested_video_duration_seconds.*generated_video_duration_milliseconds.*RETURNING`).
		WithArgs(
			"ppvidtask_1", int64(10), int64(20), nil, int64(30), service.PlatformKling,
			string(service.PPVideoOperationKlingTextToVideo), "kling-v3", service.PPVideoTaskStatusSubmitting,
			service.PPVideoBillingStatusHeld, "payload_hash", "idem", "hold", "capture", "release",
			5.0, 5.0, "USD", 5, int64(5000), int64(0), int64(0), 1, "720p",
			0, 0, 0.0, false, "", "", 0.0, 0.0, 0.0,
		).
		WillReturnRows(newPPVideoTaskRows(now).AddRow(
			int64(1), "ppvidtask_1", "", int64(10), int64(20), nil, int64(30),
			service.PlatformKling, service.PPVideoOperationKlingTextToVideo, "kling-v3",
			service.PPVideoTaskStatusSubmitting, service.PPVideoBillingStatusHeld,
			"payload_hash", "idem", "hold", "capture", "release",
			5.0, 5.0, nil, "USD", 5, int64(5000), int64(0), int64(0), 1, "720p",
			0, 0, 0.0, false, "", "", 0.0, 0.0, 0.0,
			200, "application/json", "{}",
			"", "", "", "", nil, 0, nil, now, now, nil, nil, nil,
		))

	repo := &usageBillingRepository{db: db}
	task, err := repo.CreatePPVideoTask(ctx, service.CreatePPVideoTaskParams{
		LocalTaskID:                        "ppvidtask_1",
		UserID:                             10,
		APIKeyID:                           20,
		AccountID:                          30,
		Platform:                           service.PlatformKling,
		Operation:                          service.PPVideoOperationKlingTextToVideo,
		Model:                              "kling-v3",
		Status:                             service.PPVideoTaskStatusSubmitting,
		BillingStatus:                      service.PPVideoBillingStatusHeld,
		RequestHash:                        "payload_hash",
		IdempotencyKey:                     "idem",
		HoldID:                             "hold",
		CaptureID:                          "capture",
		ReleaseID:                          "release",
		EstimatedTotalCost:                 5,
		HoldAmount:                         5,
		Currency:                           "USD",
		RequestedVideoDurationSeconds:      5,
		RequestedVideoDurationMilliseconds: 5000,
		VideoCount:                         1,
		VideoResolution:                    "720p",
	})

	require.NoError(t, err)
	require.Equal(t, service.PlatformKling, task.Platform)
	require.Equal(t, service.PPVideoOperationKlingTextToVideo, task.Operation)
	require.Equal(t, int64(30), task.AccountID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkPPVideoTaskSubmittedRejectsLateSubmission(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)UPDATE pp_video_tasks.*WHERE local_task_id = \$1.*status = 'submitting'.*task_id IS NULL.*settled_at IS NULL.*RETURNING`).
		WithArgs("ppvidtask_1", "provider-task-1", service.PPVideoTaskStatusProcessing, 200, "application/json", `{}`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	repo := &usageBillingRepository{db: db}
	_, err = repo.MarkPPVideoTaskSubmitted(ctx, service.MarkPPVideoTaskSubmittedParams{
		LocalTaskID:         "ppvidtask_1",
		TaskID:              "provider-task-1",
		Status:              service.PPVideoTaskStatusProcessing,
		ResponseStatus:      200,
		ResponseContentType: "application/json",
		ResponseBody:        `{}`,
	})

	require.ErrorIs(t, err, service.ErrPPVideoTaskNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkPPVideoTaskSubmitFailedRejectsAlreadySubmittedTask(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`(?s)WITH release_state AS.*task.status = 'submitting'.*task.task_id IS NULL.*UPDATE pp_video_tasks task.*WHERE task.id = release_state.id.*task.status = 'submitting'.*task.task_id IS NULL.*task.settled_at IS NULL`).
		WithArgs("ppvidtask_1", "UPSTREAM_ERROR", "late failure").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := &usageBillingRepository{db: db}
	err = repo.MarkPPVideoTaskSubmitFailed(ctx, "ppvidtask_1", "UPSTREAM_ERROR", "late failure")

	require.ErrorIs(t, err, service.ErrPPVideoTaskNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPPVideoTaskForOwnerDoesNotCrossAPIKeys(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)FROM pp_video_tasks.*WHERE user_id = \$1.*api_key_id = \$2.*task_id = \$3`).
		WithArgs(int64(10), int64(20), "task_1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	repo := &usageBillingRepository{db: db}
	_, err = repo.GetPPVideoTaskForOwner(ctx, 10, 20, "task_1")

	require.ErrorIs(t, err, service.ErrPPVideoTaskNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkPPVideoTaskStatusPersistsMillisecondDuration(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)UPDATE pp_video_tasks.*generated_video_duration_milliseconds = CASE.*\$5 > 0.*video_count = CASE.*\$6 > 0.*video_resolution = CASE.*NULLIF\(\$7, ''\).*RETURNING`).
		WithArgs(
			"task_1", service.PPVideoTaskStatusSucceeded, "", "", int64(5041), 1, "720p",
			0, 0, 0.0, 0, 200, "application/json", `{"status":"SUCCESS"}`,
		).
		WillReturnRows(newPPVideoTaskRows(now).AddRow(
			int64(1), "ppvidtask_1", "task_1", int64(10), int64(20), nil, int64(30),
			service.PlatformKling, service.PPVideoOperationKlingTextToVideo, "kling-v3",
			service.PPVideoTaskStatusSucceeded, service.PPVideoBillingStatusHeld,
			"payload_hash", "idem", "hold", "capture", "release",
			5.0, 5.0, nil, "USD", 5, int64(5000), int64(0), int64(5041), 1, "720p",
			1280, 720, 24.0, false, "std", service.PPVideoBillingFormulaPerSecond, 5.041, 0.57419, 0.0,
			200, "application/json", `{"status":"SUCCESS"}`,
			"", "", "", "", nil, 0, nil, now, now, now, now, nil,
		))

	repo := &usageBillingRepository{db: db}
	task, err := repo.MarkPPVideoTaskStatus(ctx, service.MarkPPVideoTaskStatusParams{
		TaskID:                             "task_1",
		Status:                             service.PPVideoTaskStatusSucceeded,
		GeneratedVideoDurationMilliseconds: 5041,
		VideoCount:                         1,
		VideoResolution:                    "720p",
		ResponseStatus:                     200,
		ResponseContentType:                "application/json",
		ResponseBody:                       `{"status":"SUCCESS"}`,
	})

	require.NoError(t, err)
	require.Equal(t, int64(5041), task.GeneratedVideoDurationMilliseconds)
	require.Equal(t, 1, task.VideoCount)
	require.Equal(t, "720p", task.VideoResolution)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimPPVideoTaskSettlementUsesCompareAndSet(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)UPDATE pp_video_tasks.*WHERE task_id = \$1.*settled_at IS NULL.*billing_status IN \('held', 'none'\).*OR \(billing_status IN \('settling', 'settling_none'\) AND status = \$2\).*RETURNING`).
		WithArgs("task_1", service.PPVideoTaskStatusSucceeded).
		WillReturnRows(newPPVideoTaskRows(now).AddRow(
			int64(1), "ppvidtask_1", "task_1", int64(10), int64(20), nil, int64(30),
			service.PlatformSeedance, service.PPVideoOperationGeneric, "doubao-seedance-2-0",
			service.PPVideoTaskStatusSucceeded, service.PPVideoBillingStatusSettling,
			"payload_hash", "idem", "hold", "capture", "release",
			5.0, 5.0, nil, "USD", 5, int64(5000), int64(0), int64(5000), 1, "720p",
			1280, 720, 24.0, false, "", service.PPVideoBillingFormulaPerSecond, 5, 0.4, 0.0,
			200, "application/json", `{"status":"SUCCESS"}`,
			"", "", "", "", nil, 0, nil, now, now, now, now, nil,
		))

	repo := &usageBillingRepository{db: db}
	task, claimed, err := repo.ClaimPPVideoTaskSettlement(ctx, service.ClaimPPVideoTaskSettlementParams{
		TaskID:      "task_1",
		FinalStatus: service.PPVideoTaskStatusSucceeded,
	})

	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, service.PPVideoBillingStatusSettling, task.BillingStatus)

	mock.ExpectQuery(`(?s)UPDATE pp_video_tasks.*WHERE task_id = \$1.*settled_at IS NULL.*billing_status IN \('held', 'none'\).*OR \(billing_status IN \('settling', 'settling_none'\) AND status = \$2\).*RETURNING`).
		WithArgs("task_1", service.PPVideoTaskStatusSucceeded).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, claimed, err = repo.ClaimPPVideoTaskSettlement(ctx, service.ClaimPPVideoTaskSettlementParams{
		TaskID:      "task_1",
		FinalStatus: service.PPVideoTaskStatusSucceeded,
	})

	require.NoError(t, err)
	require.False(t, claimed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newPPVideoTaskRows(_ time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"local_task_id",
		"task_id",
		"user_id",
		"api_key_id",
		"group_id",
		"account_id",
		"platform",
		"operation",
		"model",
		"status",
		"billing_status",
		"request_hash",
		"idempotency_key",
		"hold_id",
		"capture_id",
		"release_id",
		"estimated_total_cost",
		"hold_amount",
		"actual_cost",
		"currency",
		"requested_video_duration_seconds",
		"requested_video_duration_milliseconds",
		"input_video_duration_milliseconds",
		"generated_video_duration_milliseconds",
		"video_count",
		"video_resolution",
		"output_width",
		"output_height",
		"frame_rate",
		"has_audio",
		"kling_mode",
		"billing_formula",
		"billing_units",
		"billing_unit_price",
		"billing_fallback_unit_price",
		"response_status",
		"response_content_type",
		"response_body",
		"last_error_code",
		"last_error_message",
		"poll_lease_token",
		"poll_lease_owner",
		"poll_lease_until",
		"poll_attempts",
		"last_polled_at",
		"created_at",
		"updated_at",
		"submitted_at",
		"finished_at",
		"settled_at",
	})
}
