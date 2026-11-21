//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	selectDedupClaimSQL        = `SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`
	selectDedupArchiveClaimSQL = `SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`
)

// A hold that was already released (the money is back with the user) must not be captured: the
// capture would unfreeze the same amount again and take it from the user's other frozen holds.
// The error rolls the transaction back, so no capture record is left behind.
func TestCaptureUsageBillingBatchImageBalance_RefusesAHoldThatWasAlreadyReleased(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(selectDedupClaimSQL).
		WithArgs("pp_video_release:t1", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectRollback()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{
		UserID: 42, APIKeyID: 7, BatchID: "t1",
		HoldRequestID: "pp_video_hold:t1", ReleaseRequestID: "pp_video_release:t1",
		HoldAmount: 1, ActualAmount: 0.25,
	})
	require.ErrorIs(t, err, service.ErrBatchImageHoldAlreadyReleased)
	require.Nil(t, result)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

// A release record that was archived still counts as released.
func TestCaptureUsageBillingBatchImageBalance_RefusesAHoldReleasedInTheArchive(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(selectDedupClaimSQL).
		WithArgs("pp_video_release:t1", int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(selectDedupArchiveClaimSQL).
		WithArgs("pp_video_release:t1", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectRollback()

	_, err = captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{
		UserID: 42, APIKeyID: 7, BatchID: "t1",
		HoldRequestID: "pp_video_hold:t1", ReleaseRequestID: "pp_video_release:t1",
		HoldAmount: 1, ActualAmount: 0.25,
	})
	require.ErrorIs(t, err, service.ErrBatchImageHoldAlreadyReleased)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

// A hold that was not released is captured exactly as before.
func TestCaptureUsageBillingBatchImageBalance_CapturesWhenTheHoldWasNotReleased(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(selectDedupClaimSQL).
		WithArgs("pp_video_release:t1", int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(selectDedupArchiveClaimSQL).
		WithArgs("pp_video_release:t1", int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(selectDedupClaimSQL).
		WithArgs("pp_video_hold:t1", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectQuery(selectUserFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"frozen_balance", "frozen_display_balance"}).AddRow(0.5, 1.0))
	mock.ExpectQuery(settleBatchImageHoldSQL).
		WithArgs(0.375, 0.75, 0.5, 1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.875, 0.0))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{
		UserID: 42, APIKeyID: 7, BatchID: "t1",
		HoldRequestID: "pp_video_hold:t1", ReleaseRequestID: "pp_video_release:t1",
		HoldAmount: 1, ActualAmount: 0.25,
	})
	require.NoError(t, err)
	require.InDelta(t, 9.875, *result.NewBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
