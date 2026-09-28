//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestOrganizationDeleteRequiresOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT owner_user_id,status,balance,display_balance,frozen_balance,frozen_display_balance`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "balance", "display_balance", "frozen_balance", "frozen_display_balance"}).
			AddRow(int64(99), "active", 10.0, 20.0, 0.0, 0.0))
	mock.ExpectRollback()

	_, err = (&organizationRepository{db: db}).Delete(context.Background(), 7, 42)
	require.ErrorIs(t, err, service.ErrOrganizationOwnerOnly)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrganizationDeleteRejectsFrozenUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT owner_user_id,status,balance,display_balance,frozen_balance,frozen_display_balance`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "balance", "display_balance", "frozen_balance", "frozen_display_balance"}).
			AddRow(int64(42), "active", 10.0, 20.0, 1.0, 2.0))
	mock.ExpectRollback()

	_, err = (&organizationRepository{db: db}).Delete(context.Background(), 7, 42)
	require.ErrorIs(t, err, service.ErrOrganizationHasFrozenUsage)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOrganizationDeleteRefundsOwnerAndDisablesAccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT owner_user_id,status,balance,display_balance,frozen_balance,frozen_display_balance`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"owner_user_id", "status", "balance", "display_balance", "frozen_balance", "frozen_display_balance"}).
			AddRow(int64(42), "active", 10.0, 20.0, 0.0, 0.0))
	mock.ExpectQuery(`SELECT k.key FROM api_keys`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"key"}).AddRow("sk-team-one").AddRow("sk-team-two"))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1,display_balance=display_balance\+\$2`).
		WithArgs(10.0, 20.0, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO organization_quota_ledger`).
		WithArgs(int64(7), int64(42), -10.0, 20.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO organization_audit_logs`).
		WithArgs(int64(7), int64(42), "organization.delete", "organization", "7", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE api_keys SET status='inactive'`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`UPDATE organization_api_keys SET status='inactive'`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`UPDATE organization_members SET status='disabled'`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`UPDATE organization_invitations SET revoked_at`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE organizations SET status='deleted'`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	keys, err := (&organizationRepository{db: db}).Delete(context.Background(), 7, 42)
	require.NoError(t, err)
	require.Equal(t, []string{"sk-team-one", "sk-team-two"}, keys)
	require.NoError(t, mock.ExpectationsWereMet())
}
