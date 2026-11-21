package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func newPPVideoTaskAdminRecordRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"local_task_id", "task_id", "api_key_name", "platform", "operation", "model",
		"status", "billing_status", "prompt", "video_count", "video_resolution",
		"requested_video_duration_seconds", "requested_video_duration_milliseconds",
		"generated_video_duration_milliseconds", "hold_amount", "actual_cost",
		"last_error_code", "last_error_message", "created_at", "finished_at", "settled_at", "failure_response_body",
		"user_id", "user_email", "username", "api_key_id", "account_id", "account_name", "group_id", "group_name",
	})
}

func TestBuildPPVideoTaskAdminWhere_NoFilterMatchesEverything(t *testing.T) {
	where, args := buildPPVideoTaskAdminWhere(service.PPVideoTaskAdminListFilter{})

	require.Equal(t, "TRUE", where)
	require.Empty(t, args)
}

func TestBuildPPVideoTaskAdminWhere_AppliesUserFiltersBeforeSharedOnes(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	where, args := buildPPVideoTaskAdminWhere(service.PPVideoTaskAdminListFilter{
		PPVideoTaskListFilter: service.PPVideoTaskListFilter{
			TaskID:    "abc",
			Status:    service.PPVideoTaskStatusProcessing,
			StartTime: &start,
		},
		UserID:   42,
		User:     "alice@",
		Platform: "seedance",
	})

	require.Contains(t, where, "t.user_id = $1 AND (COALESCE(u.email, '') ILIKE $2 OR COALESCE(u.username, '') ILIKE $2)"+
		" AND t.platform ILIKE $3"+
		" AND (t.local_task_id ILIKE $4 OR COALESCE(t.task_id, '') ILIKE $4)"+
		" AND LOWER(t.status) IN ($5,")
	require.Contains(t, where, "t.created_at >= $")
	require.Equal(t, int64(42), args[0])
	require.Equal(t, "%alice@%", args[1])
	require.Equal(t, "%seedance%", args[2])
	require.Equal(t, "%abc%", args[3])
	require.Equal(t, start, args[len(args)-1])
}

func TestBuildPPVideoTaskAdminWhere_EscapesLikeWildcards(t *testing.T) {
	_, args := buildPPVideoTaskAdminWhere(service.PPVideoTaskAdminListFilter{User: `a_b%c`})

	require.Equal(t, `%a\_b\%c%`, args[0])
}

func TestListPPVideoTasksForAdmin_ReturnsOwnershipAcrossUsers(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	created := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM pp_video_tasks t.*LEFT JOIN users u.*LEFT JOIN accounts a.*LEFT JOIN groups g.*WHERE TRUE`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	mock.ExpectQuery(`(?s)SELECT.*t\.user_id.*FROM pp_video_tasks t.*WHERE TRUE ORDER BY t\.created_at DESC, t\.id DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 0).
		WillReturnRows(newPPVideoTaskAdminRecordRows().
			AddRow("ppvidtask_1", "upstream_1", "prod-key", service.PlatformCTMOSeedance, service.PPVideoOperationGeneric,
				"seedance2.0", "succeeded", service.PPVideoBillingStatusCaptured, "a cat", 1, "720p",
				5, int64(5000), int64(5000), 2.5, 2.5, "", "", created, created, created, "",
				int64(7), "alice@example.com", "alice", int64(11), int64(3), "ctmo-main", int64(5), "video-group").
			AddRow("ppvidtask_2", "", "", service.PlatformKling, service.PPVideoOperationKlingTextToVideo,
				"kling-v3", "failed", service.PPVideoBillingStatusReleased, "", 1, "",
				5, int64(0), int64(0), 3.0, nil, "upstream_error", "boom", created, nil, nil, "",
				int64(8), "bob@example.com", "", int64(12), int64(4), "", nil, ""))

	repo := &usageBillingRepository{db: db}
	records, total, err := repo.ListPPVideoTasksForAdmin(ctx, service.PPVideoTaskAdminListFilter{})

	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, records, 2)
	require.Equal(t, int64(7), records[0].UserID)
	require.Equal(t, "alice@example.com", records[0].UserEmail)
	require.Equal(t, "ctmo-main", records[0].AccountName)
	require.NotNil(t, records[0].GroupID)
	require.Equal(t, int64(5), *records[0].GroupID)
	require.Equal(t, "prod-key", records[0].APIKeyName)
	require.Equal(t, int64(8), records[1].UserID)
	require.Nil(t, records[1].GroupID, "a task without a group keeps group_id nil")
	require.Equal(t, "boom", records[1].FailureReason())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListPPVideoTasksForAdmin_SkipsRowQueryWhenNothingMatches(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	repo := &usageBillingRepository{db: db}
	records, total, err := repo.ListPPVideoTasksForAdmin(ctx, service.PPVideoTaskAdminListFilter{})

	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.NotNil(t, records)
	require.Empty(t, records)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPPVideoTaskForAdmin_NotScopedToUserAndMapsNotFound(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)FROM pp_video_tasks\s+WHERE task_id = \$1 OR local_task_id = \$1`).
		WithArgs("ppvidtask_missing").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)WHERE t\.local_task_id = \$1 OR t\.task_id = \$1`).
		WithArgs("ppvidtask_missing").
		WillReturnError(sql.ErrNoRows)

	repo := &usageBillingRepository{db: db}
	task, err := repo.GetPPVideoTaskForAdmin(ctx, " ppvidtask_missing ")
	require.ErrorIs(t, err, service.ErrPPVideoTaskNotFound)
	require.Nil(t, task)

	record, err := repo.GetPPVideoTaskRecordForAdmin(ctx, "ppvidtask_missing")
	require.ErrorIs(t, err, service.ErrPPVideoTaskNotFound)
	require.Nil(t, record)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPPVideoTaskAdminQueryRepositoryNilDBReturnsConnDone(t *testing.T) {
	repo := &usageBillingRepository{}
	ctx := context.Background()

	_, _, err := repo.ListPPVideoTasksForAdmin(ctx, service.PPVideoTaskAdminListFilter{})
	require.ErrorIs(t, err, sql.ErrConnDone)
	_, err = repo.GetPPVideoTaskRecordForAdmin(ctx, "x")
	require.ErrorIs(t, err, sql.ErrConnDone)
	_, err = repo.GetPPVideoTaskForAdmin(ctx, "x")
	require.ErrorIs(t, err, sql.ErrConnDone)
}
