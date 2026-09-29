//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestOrganizationAPIKeyBillingSourcesPreserveInactiveBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ids := []int64{10, 11}
	mock.ExpectQuery(`SELECT ok.api_key_id,o.id,o.name,o.status,m.status,ok.status`).
		WithArgs(pq.Array(ids)).
		WillReturnRows(sqlmock.NewRows([]string{"api_key_id", "organization_id", "name", "organization_status", "member_status", "binding_status"}).
			AddRow(int64(10), int64(7), "研发团队", "active", "active", "active").
			AddRow(int64(11), int64(8), "已删除团队", "deleted", "disabled", "inactive"))

	sources, err := (&organizationRepository{db: db}).GetAPIKeyBillingSources(context.Background(), ids)
	require.NoError(t, err)
	require.Equal(t, service.APIKeyBillingSourceActive, sources[10].Status)
	require.Equal(t, service.APIKeyBillingSourceInactive, sources[11].Status)
	require.Equal(t, "已删除团队", sources[11].OrganizationName)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetBillingSubjectTreatsDetachedBindingAsPersonal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`WHERE ok.api_key_id=\$1 AND ok.status <> 'detached'`).WithArgs(int64(205)).
		WillReturnRows(sqlmock.NewRows([]string{
			"organization_id", "member_user_id", "balance", "frozen_balance", "monthly_limit",
			"monthly_used", "monthly_frozen", "organization_status", "member_status", "key_status",
		}))

	subject, err := (&organizationRepository{db: db}).GetBillingSubjectByAPIKey(context.Background(), 205)
	require.NoError(t, err)
	require.Nil(t, subject)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetAPIKeyBillingSourceToPersonal(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT organization_id FROM organization_api_keys`).WithArgs(int64(205)).
		WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(int64(7)))
	mock.ExpectQuery(`SELECT id FROM organizations WHERE id=\$1 FOR UPDATE`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectExec(`INSERT INTO organization_audit_logs`).
		WithArgs(int64(7), int64(9), "api_key.billing_source.change", "api_key", "205", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT user_id,key FROM api_keys`).WithArgs(int64(205)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "key"}).AddRow(int64(9), "sk-existing"))
	mock.ExpectExec(`UPDATE organization_api_keys SET status='detached'`).WithArgs(int64(205)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	source, credential, err := (&organizationRepository{db: db}).SetAPIKeyBillingSource(context.Background(), 205, 9, service.APIKeyBillingSourcePersonal, nil)
	require.NoError(t, err)
	require.Equal(t, "sk-existing", credential)
	require.Equal(t, service.APIKeyBillingSourcePersonal, source.Type)
	require.Equal(t, service.APIKeyBillingSourceActive, source.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetAPIKeyBillingSourceToOrganization(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	organizationID := int64(12)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT m.role,m.status,o.status`).WithArgs(organizationID, int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "member_status", "organization_status"}).AddRow("member", "active", "active"))
	mock.ExpectQuery(`SELECT name FROM organizations`).WithArgs(organizationID).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("产品研发团队"))
	mock.ExpectExec(`INSERT INTO organization_audit_logs`).
		WithArgs(organizationID, int64(9), "api_key.billing_source.change", "api_key", "205", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT user_id,key FROM api_keys`).WithArgs(int64(205)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "key"}).AddRow(int64(9), "sk-existing"))
	mock.ExpectExec(`INSERT INTO organization_api_keys`).WithArgs(int64(205), organizationID, int64(9)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	source, credential, err := (&organizationRepository{db: db}).SetAPIKeyBillingSource(context.Background(), 205, 9, service.APIKeyBillingSourceOrganization, &organizationID)
	require.NoError(t, err)
	require.Equal(t, "sk-existing", credential)
	require.Equal(t, service.APIKeyBillingSourceOrganization, source.Type)
	require.Equal(t, organizationID, *source.OrganizationID)
	require.Equal(t, "产品研发团队", source.OrganizationName)
	require.NoError(t, mock.ExpectationsWereMet())
}

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
