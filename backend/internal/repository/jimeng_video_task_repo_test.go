package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestClaimJimengVideoTasksForPollingLeasesHeldTasks(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(2 * time.Minute)
	submitCutoff := now.Add(-10 * time.Minute)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)UPDATE jimeng_video_tasks AS task.*poll_lease_owner = \$1.*poll_lease_until = \$3.*FOR UPDATE SKIP LOCKED.*RETURNING`).
		WithArgs("worker-a", 10, leaseUntil, submitCutoff).
		WillReturnRows(newJimengVideoTaskRows(now).AddRow(
			int64(1), "vidtask_poll", "task_poll", int64(10), int64(20), nil, int64(30),
			service.JimengVideoBillingModel, service.JimengTaskStatusProcessing, service.JimengVideoBillingStatusHeld,
			"payload_hash", "idem", "hold", "capture", "release",
			5.0, 5.0, nil, "USD", 5, "720p", 200, "application/json", `{"status":"processing"}`,
			"", "", now, now, now, nil, nil,
		))

	repo := &usageBillingRepository{db: db}
	tasks, err := repo.ClaimJimengVideoTasksForPolling(ctx, service.ClaimJimengVideoTasksForPollingParams{
		LeaseOwner:   "worker-a",
		LeaseUntil:   leaseUntil,
		SubmitCutoff: submitCutoff,
		Limit:        10,
	})

	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, "vidtask_poll", tasks[0].LocalTaskID)
	require.Equal(t, "task_poll", tasks[0].TaskID)
	require.Equal(t, service.JimengVideoBillingStatusHeld, tasks[0].BillingStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseJimengVideoTaskPollLeaseIsBestEffort(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`(?s)UPDATE jimeng_video_tasks\s+SET poll_lease_owner = NULL,\s+poll_lease_until = NULL,\s+updated_at = NOW\(\)\s+WHERE local_task_id = \$1\s+AND poll_lease_owner = \$2`).
		WithArgs("vidtask_poll", "worker-a").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := &usageBillingRepository{db: db}
	err = repo.ReleaseJimengVideoTaskPollLease(ctx, "vidtask_poll", "worker-a")

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newJimengVideoTaskRows(_ time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"local_task_id",
		"task_id",
		"user_id",
		"api_key_id",
		"group_id",
		"account_id",
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
		"video_duration_seconds",
		"video_resolution",
		"response_status",
		"response_content_type",
		"response_body",
		"last_error_code",
		"last_error_message",
		"created_at",
		"updated_at",
		"submitted_at",
		"finished_at",
		"settled_at",
	})
}
