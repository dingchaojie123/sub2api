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
	claimUsageBillingRequestSQL = `(?s)INSERT INTO usage_billing_dedup \(request_id, api_key_id, request_fingerprint, organization_id, member_user_id\).*VALUES \(\$1, \$2, \$3, \$4, \$5\).*RETURNING id`
	selectUserBalanceSQL        = `(?s)SELECT balance, COALESCE\(display_balance, 0\)\s+FROM users\s+WHERE id = \$1 AND deleted_at IS NULL\s+FOR UPDATE`
	deductUserBalanceSQL        = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+display_balance = GREATEST\(COALESCE\(display_balance, 0\) - \$2, 0\),\s+updated_at = NOW\(\)\s+WHERE id = \$3 AND deleted_at IS NULL\s+RETURNING balance`
	resetOrganizationMemberSQL  = `UPDATE organization_members SET monthly_used=0,usage_period_start=date_trunc\('month',NOW\(\)\),updated_at=NOW\(\) WHERE organization_id=\$1 AND user_id=\$2 AND usage_period_start < date_trunc\('month',NOW\(\)\)`
	selectOrgBalanceSQL         = `SELECT balance,COALESCE\(display_balance,0\) FROM organizations WHERE id=\$1 FOR UPDATE`
	deductOrgBalanceSQL         = `(?s)UPDATE organizations SET balance=balance-\$1,\s*display_balance=GREATEST\(COALESCE\(display_balance,0\)-\$2,0\),\s*updated_at=NOW\(\) WHERE id=\$3 RETURNING balance`
	incrementOrgMemberUsageSQL  = `UPDATE organization_members SET monthly_used=monthly_used\+\$1,updated_at=NOW\(\) WHERE organization_id=\$2 AND user_id=\$3`
	insertOrgUsageLedgerSQL     = `(?s)INSERT INTO organization_quota_ledger\(organization_id,member_user_id,api_key_id,request_id,entry_type,amount,balance_after,detail\) VALUES\(\$1,\$2,\$3,\$4,'usage',\$5,\$6,jsonb_build_object\('model',\$7::text,'overdraft',true\)\)`
	reserveBatchImageHoldSQL    = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+display_balance = GREATEST\(COALESCE\(display_balance, 0\) - \$2, 0\),\s+frozen_balance = COALESCE\(frozen_balance, 0\) \+ \$1,\s+frozen_display_balance = COALESCE\(frozen_display_balance, 0\) \+ \$3,\s+updated_at = NOW\(\)\s+WHERE id = \$4 AND deleted_at IS NULL\s+RETURNING balance, frozen_balance`
	selectUserFrozenSQL         = `(?s)SELECT COALESCE\(frozen_balance, 0\), COALESCE\(frozen_display_balance, 0\)\s+FROM users\s+WHERE id = \$1 AND deleted_at IS NULL\s+FOR UPDATE`
	settleBatchImageHoldSQL     = `(?s)UPDATE users\s+SET balance = balance \+ \$1,\s+display_balance = COALESCE\(display_balance, 0\) \+ \$2,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$3,\s+frozen_display_balance = GREATEST\(COALESCE\(frozen_display_balance, 0\) - \$4, 0\),\s+updated_at = NOW\(\)\s+WHERE id = \$5 AND deleted_at IS NULL\s+RETURNING balance, frozen_balance`
)

func TestClaimUsageBillingRequest_PersistsBillingSnapshot(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	organizationID, memberUserID := int64(10), int64(42)
	mock.ExpectQuery(claimUsageBillingRequestSQL).
		WithArgs("req-1", int64(7), "fingerprint", organizationID, memberUserID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`SELECT request_fingerprint\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs("req-1", int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	applied, err := (&usageBillingRepository{}).claimUsageBillingRequest(
		ctx, tx, "req-1", 7, "fingerprint", &organizationID, &memberUserID,
	)
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_ChargesCreditsAndConvertsRealBalanceByRatio(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// 真实余额 10、积分余额 20，比值 0.5：扣 2.5 积分，真实余额只扣 1.25。
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(10.0, 20.0))
	mock.ExpectQuery(deductUserBalanceSQL).
		WithArgs(1.25, 2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(8.75))
	mock.ExpectCommit()

	newBalance, realDeducted, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 8.75, newBalance, 0.000001)
	require.InDelta(t, 1.25, realDeducted, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_SameCreditsForAccountsWithDifferentRatios(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// 比值为 1 的账号：真实余额与积分余额都扣 2.5。
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(10.0, 10.0))
	mock.ExpectQuery(deductUserBalanceSQL).
		WithArgs(2.5, 2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(7.5))
	mock.ExpectCommit()

	newBalance, realDeducted, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 7.5, newBalance, 0.000001)
	require.InDelta(t, 2.5, realDeducted, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_RecordsOverdraftWhenBalanceIsShort(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// 积分余额 20 不够扣 30：仍然记账，真实余额按比值扣 15，变成 -5。
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(10.0, 20.0))
	mock.ExpectQuery(deductUserBalanceSQL).
		WithArgs(15.0, 30.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	newBalance, realDeducted, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 30)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.InDelta(t, 15.0, realDeducted, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_FlagsBalanceOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(5.0, 5.0))
	mock.ExpectQuery(deductUserBalanceSQL).
		WithArgs(10.0, 10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.NotNil(t, result.BalanceDeducted)
	require.InDelta(t, 10.0, *result.BalanceDeducted, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingOrganization_UsesSnapshotAndRecordsOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectExec(resetOrganizationMemberSQL).
		WithArgs(int64(10), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(selectOrgBalanceSQL).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(5.0, 5.0))
	mock.ExpectQuery(deductOrgBalanceSQL).
		WithArgs(10.0, 10.0, int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectExec(incrementOrgMemberUsageSQL).
		WithArgs(10.0, int64(10), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(insertOrgUsageLedgerSQL).
		WithArgs(int64(10), int64(42), int64(7), "req-1", -10.0, -5.0, "gpt-test").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	organizationID, memberUserID := int64(10), int64(42)
	newBalance, sufficient, err := deductUsageBillingOrganization(ctx, tx, &service.UsageBillingCommand{
		RequestID:            "req-1",
		APIKeyID:             7,
		OrganizationID:       &organizationID,
		OrganizationMemberID: &memberUserID,
		Model:                "gpt-test",
	}, 10)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingOrganization_ChargesCreditsAndConvertsRealBalanceByRatio(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectExec(resetOrganizationMemberSQL).
		WithArgs(int64(10), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// 组织真实余额 10、积分余额 20：扣 2.5 积分，真实余额只扣 1.25。
	mock.ExpectQuery(selectOrgBalanceSQL).
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(10.0, 20.0))
	mock.ExpectQuery(deductOrgBalanceSQL).
		WithArgs(1.25, 2.5, int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(8.75))
	mock.ExpectExec(incrementOrgMemberUsageSQL).
		WithArgs(2.5, int64(10), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO organization_quota_ledger`).
		WithArgs(int64(10), int64(42), int64(7), "req-2", -2.5, 8.75, "gpt-test").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	organizationID, memberUserID := int64(10), int64(42)
	newBalance, sufficient, err := deductUsageBillingOrganization(ctx, tx, &service.UsageBillingCommand{
		RequestID:            "req-2",
		APIKeyID:             7,
		OrganizationID:       &organizationID,
		OrganizationMemberID: &memberUserID,
		Model:                "gpt-test",
	}, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 8.75, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchImageOrganizationBillingOwner_UsesTaskSnapshot(t *testing.T) {
	organizationID, memberUserID := int64(10), int64(42)
	actualOrganizationID, actualMemberUserID, ok, err := batchImageOrganizationBillingOwner(&service.BatchImageBalanceHoldCommand{
		BillingSource:        service.APIKeyBillingSourceOrganization,
		OrganizationID:       &organizationID,
		OrganizationMemberID: &memberUserID,
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, organizationID, actualOrganizationID)
	require.Equal(t, memberUserID, actualMemberUserID)

	_, _, ok, err = batchImageOrganizationBillingOwner(&service.BatchImageBalanceHoldCommand{
		BillingSource: service.APIKeyBillingSourcePersonal,
		APIKeyID:      7,
	})
	require.NoError(t, err)
	require.False(t, ok)
}

func TestDeductUsageBillingBalance_ReturnsUserNotFoundWhenUserMissing(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, _, _, err = deductUsageBillingBalance(ctx, tx, 42, 10)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_MovesAvailableToFrozen(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// 真实余额 10、积分余额 20：冻结 2.5 积分，真实余额冻结 1.25。
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(10.0, 20.0))
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(1.25, 2.5, 2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(8.75, 1.25))
	mock.ExpectCommit()

	result, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 2.5})
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.NotNil(t, result.FrozenBalance)
	require.InDelta(t, 8.75, *result.NewBalance, 0.000001)
	require.InDelta(t, 1.25, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "display_balance"}).AddRow(5.0, 5.0))
	mock.ExpectRollback()

	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 10})
	require.ErrorIs(t, err, service.ErrBatchImageInsufficientBalance)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_ReturnsUserNotFound(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(selectUserBalanceSQL).
		WithArgs(int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 10})
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_ReleasesRemainder(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_capture"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	// 冻结 1 积分对应真实余额 0.5；实际只用 0.25 积分，退回 0.75 积分和 0.375 真实余额。
	mock.ExpectQuery(selectUserFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"frozen_balance", "frozen_display_balance"}).AddRow(0.5, 1.0))
	mock.ExpectQuery(settleBatchImageHoldSQL).
		WithArgs(0.375, 0.75, 0.5, 1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.875, 0.0))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_capture", HoldAmount: 1, ActualAmount: 0.25})
	require.NoError(t, err)
	require.InDelta(t, 9.875, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_FailsWhenFrozenBalanceIsTooSmall(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_short"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectQuery(selectUserFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"frozen_balance", "frozen_display_balance"}).AddRow(0.5, 0.0))
	mock.ExpectRollback()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_short", HoldAmount: 1, ActualAmount: 1})
	require.Error(t, err)
	require.Nil(t, result)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_FailsWhenHoldNeverReserved(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_unheld"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_unheld"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_unheld", HoldAmount: 1, ActualAmount: 0.25})
	require.ErrorIs(t, err, service.ErrBatchImageHoldNotReserved)
	require.Nil(t, result)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_RejectsActualCostOverHold(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 0.5, ActualAmount: 1})
	require.ErrorIs(t, err, service.ErrBatchImageSettlementCostExceedsHold)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_ReturnsFrozenToAvailable(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_release"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	// 冻结 1 积分对应真实余额 0.5，整笔释放：退回 1 积分和 0.5 真实余额。
	mock.ExpectQuery(selectUserFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"frozen_balance", "frozen_display_balance"}).AddRow(0.5, 1.0))
	mock.ExpectQuery(settleBatchImageHoldSQL).
		WithArgs(0.5, 1.0, 0.5, 1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_release", HoldAmount: 1})
	require.NoError(t, err)
	require.InDelta(t, 10.0, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_UsesCustomHoldRequestID(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs("jimeng_video_hold:vidtask_123", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectQuery(selectUserFrozenSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"frozen_balance", "frozen_display_balance"}).AddRow(1.0, 1.0))
	mock.ExpectQuery(settleBatchImageHoldSQL).
		WithArgs(1.0, 1.0, 1.0, 1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{
		UserID:        42,
		APIKeyID:      7,
		BatchID:       "vidtask_123",
		HoldRequestID: "jimeng_video_hold:vidtask_123",
		HoldAmount:    1,
	})
	require.NoError(t, err)
	require.InDelta(t, 10.0, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_SkipsWhenHoldNeverReserved(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// dedup 与归档表均无 hold claim：说明该 job 从未成功冻结，
	// 释放必须跳过，不得从他人冻结资金池中凭空生成余额。
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_phantom", HoldAmount: 1})
	require.NoError(t, err)
	require.Nil(t, result.NewBalance)
	require.Nil(t, result.FrozenBalance)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
