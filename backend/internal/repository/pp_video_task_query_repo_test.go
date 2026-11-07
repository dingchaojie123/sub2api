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

func newPPVideoTaskRecordRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"local_task_id", "task_id", "api_key_name", "platform", "operation", "model",
		"status", "billing_status", "prompt", "video_count", "video_resolution",
		"requested_video_duration_seconds", "requested_video_duration_milliseconds",
		"generated_video_duration_milliseconds", "hold_amount", "actual_cost",
		"last_error_code", "last_error_message", "created_at", "finished_at", "settled_at", "failure_response_body",
	})
}

func TestBuildPPVideoTaskListWhere_AlwaysScopesToUser(t *testing.T) {
	where, args := buildPPVideoTaskListWhere(42, service.PPVideoTaskListFilter{})

	require.Equal(t, "t.user_id = $1", where)
	require.Equal(t, []any{int64(42)}, args)
}

func TestBuildPPVideoTaskListWhere_AppliesEveryFilterInOrder(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)
	where, args := buildPPVideoTaskListWhere(42, service.PPVideoTaskListFilter{
		TaskID:    "abc",
		Model:     "seedance",
		APIKey:    "prod",
		Status:    service.PPVideoTaskStatusRequiresAttention,
		StartTime: &start,
		EndTime:   &end,
	})

	require.Equal(t,
		"t.user_id = $1"+
			" AND (t.local_task_id ILIKE $2 OR COALESCE(t.task_id, '') ILIKE $2)"+
			" AND t.model ILIKE $3"+
			" AND COALESCE(ak.name, '') ILIKE $4"+
			" AND LOWER(t.status) IN ($5, $6)"+
			" AND t.created_at >= $7"+
			" AND t.created_at < $8",
		where)
	require.Equal(t, []any{
		int64(42), "%abc%", "%seedance%", "%prod%", "requires_attention", "needs_attention", start, end,
	}, args)
}

func TestBuildPPVideoTaskListWhere_EscapesLikeWildcards(t *testing.T) {
	_, args := buildPPVideoTaskListWhere(1, service.PPVideoTaskListFilter{TaskID: `50%_off\x`})

	require.Equal(t, `%50\%\_off\\x%`, args[1])
}

func TestBuildPPVideoTaskListWhere_UnknownStatusIsIgnored(t *testing.T) {
	where, args := buildPPVideoTaskListWhere(1, service.PPVideoTaskListFilter{Status: "bogus"})

	require.Equal(t, "t.user_id = $1", where)
	require.Len(t, args, 1)
}

func TestListPPVideoTasksForUser_ReturnsPageAndTotal(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	created := time.Date(2026, 11, 6, 14, 34, 8, 0, time.UTC)
	finished := created.Add(9*time.Minute + 36*time.Second)
	// requires_attention 分组只有 2 个原始状态值，因此参数为 $1 用户 + $2/$3 状态，LIMIT/OFFSET 为 $4/$5。
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM pp_video_tasks t.*LEFT JOIN api_keys ak.*WHERE t\.user_id = \$1 AND LOWER\(t\.status\) IN \(\$2, \$3\)`).
		WithArgs(int64(42), "requires_attention", "needs_attention").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(41)))
	mock.ExpectQuery(`(?s)SELECT.*t\.local_task_id.*FROM pp_video_tasks t.*WHERE t\.user_id = \$1.*ORDER BY t\.created_at DESC, t\.id DESC LIMIT \$4 OFFSET \$5`).
		WithArgs(int64(42), "requires_attention", "needs_attention", 20, 20).
		WillReturnRows(newPPVideoTaskRecordRows().
			AddRow("ppvidtask_1", "upstream_1", "prod-key", service.PlatformCTMOSeedance, service.PPVideoOperationGeneric,
				"seedance2.0-933-C", "succeeded", service.PPVideoBillingStatusCaptured, "a cat", 1, "720p",
				15, int64(15000), int64(15000), 6.5, 6.5, "", "", created, finished, finished, "").
			AddRow("ppvidtask_2", "", "", service.PlatformKling, service.PPVideoOperationKlingTextToVideo,
				"kling-v3", "failed", service.PPVideoBillingStatusReleased, "", 1, "",
				5, int64(0), int64(0), 3.0, nil, "upstream_error", "boom", created, nil, nil, "").
			AddRow("ppvidtask_3", "job_3", "", service.PlatformGuangying, service.PPVideoOperationGeneric,
				"seedance", "failed", service.PPVideoBillingStatusReleased, "", 1, "720p",
				15, int64(15000), int64(0), 2.24, nil, "", "", created, created, created,
				`{"data":{"status":"failed","message":"generation failed"}}`))

	repo := &usageBillingRepository{db: db}
	records, total, err := repo.ListPPVideoTasksForUser(ctx, 42, service.PPVideoTaskListFilter{
		Status: service.PPVideoTaskStatusRequiresAttention, Page: 2, PageSize: 20,
	})

	require.NoError(t, err)
	require.Equal(t, int64(41), total)
	require.Len(t, records, 3)
	require.Equal(t, "ppvidtask_1", records[0].LocalTaskID)
	require.Equal(t, "prod-key", records[0].APIKeyName)
	require.NotNil(t, records[0].ActualCost)
	require.InDelta(t, 6.5, *records[0].ActualCost, 1e-9)
	require.NotNil(t, records[0].FinishedAt)
	require.Nil(t, records[1].ActualCost)
	require.Nil(t, records[1].FinishedAt)
	require.Equal(t, "boom", records[1].LastErrorMessage)
	require.Equal(t, "boom", records[1].FailureReason())
	require.Equal(t, "generation failed", records[2].FailureReason(), "the reason is recovered from the stored upstream response")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListPPVideoTasksForUser_SkipsRowQueryWhenNothingMatches(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	repo := &usageBillingRepository{db: db}
	records, total, err := repo.ListPPVideoTasksForUser(ctx, 42, service.PPVideoTaskListFilter{})

	require.NoError(t, err)
	require.Equal(t, int64(0), total)
	require.NotNil(t, records)
	require.Empty(t, records)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPPVideoTaskRecordForUser_ScopesByUserAndMapsNotFound(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)WHERE t\.user_id = \$1\s+AND \(t\.local_task_id = \$2 OR t\.task_id = \$2\)`).
		WithArgs(int64(7), "ppvidtask_other").
		WillReturnError(sql.ErrNoRows)

	repo := &usageBillingRepository{db: db}
	record, err := repo.GetPPVideoTaskRecordForUser(ctx, 7, "  ppvidtask_other ")

	require.ErrorIs(t, err, service.ErrPPVideoTaskNotFound)
	require.Nil(t, record)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPPVideoTaskForUser_ScopesByUserAndMapsNotFound(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)FROM pp_video_tasks\s+WHERE user_id = \$1\s+AND \(task_id = \$2 OR local_task_id = \$2\)`).
		WithArgs(int64(7), "ppvidtask_other").
		WillReturnError(sql.ErrNoRows)

	repo := &usageBillingRepository{db: db}
	task, err := repo.GetPPVideoTaskForUser(ctx, 7, "ppvidtask_other")

	require.ErrorIs(t, err, service.ErrPPVideoTaskNotFound)
	require.Nil(t, task)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetPPVideoTaskPrompt_OnlyFillsEmptyPrompt(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`(?s)UPDATE pp_video_tasks\s+SET prompt = \$2\s+WHERE local_task_id = \$1\s+AND prompt = ''`).
		WithArgs("ppvidtask_1", "a cat").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &usageBillingRepository{db: db}
	require.NoError(t, repo.SetPPVideoTaskPrompt(ctx, " ppvidtask_1 ", "a cat"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPPVideoTaskQueryRepositoryNilDBReturnsConnDone(t *testing.T) {
	repo := &usageBillingRepository{}
	ctx := context.Background()

	_, _, err := repo.ListPPVideoTasksForUser(ctx, 1, service.PPVideoTaskListFilter{})
	require.ErrorIs(t, err, sql.ErrConnDone)
	_, err = repo.GetPPVideoTaskRecordForUser(ctx, 1, "x")
	require.ErrorIs(t, err, sql.ErrConnDone)
	_, err = repo.GetPPVideoTaskForUser(ctx, 1, "x")
	require.ErrorIs(t, err, sql.ErrConnDone)
	require.ErrorIs(t, repo.SetPPVideoTaskPrompt(ctx, "x", "y"), sql.ErrConnDone)
}

var _ service.PPVideoTaskQueryRepository = (*usageBillingRepository)(nil)
