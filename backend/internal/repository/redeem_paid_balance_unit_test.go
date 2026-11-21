package repository

import (
	"context"
	"database/sql"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type paidBalanceSnapshot struct {
	balance       float64
	display       float64
	totalRecharge float64
}

func loadPaidBalanceSnapshot(t *testing.T, client *dbent.Client, userID int64) paidBalanceSnapshot {
	t.Helper()

	u, err := client.User.Get(context.Background(), userID)
	require.NoError(t, err)
	return paidBalanceSnapshot{balance: u.Balance, display: u.DisplayBalance, totalRecharge: u.TotalRecharged}
}

func TestUserRepositoryApplyRedeemPaidBalanceCreditFreshAccount(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()

	u, err := client.User.Create().
		SetEmail("paid-fresh@example.com").
		SetPasswordHash("hash").
		SetUsername("paid-fresh").
		Save(ctx)
	require.NoError(t, err)

	// 客户实付 1000，6 折：积分 1666.67，真实余额与累计充值按 1000 记
	require.NoError(t, repo.ApplyRedeemPaidBalanceCredit(ctx, u.ID, 1666.67, 1000))

	got := loadPaidBalanceSnapshot(t, client, u.ID)
	require.InDelta(t, 1000.0, got.balance, 1e-6)
	require.InDelta(t, 1666.67, got.display, 1e-6)
	require.InDelta(t, 1000.0, got.totalRecharge, 1e-6)
}

func TestUserRepositoryApplyRedeemPaidBalanceCreditAccountWithCreditBalance(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()

	// 已有积分余额（真实余额 27.7 / 积分 41.82）：两个余额各自累加，不互相换算
	u, err := client.User.Create().
		SetEmail("paid-existing@example.com").
		SetPasswordHash("hash").
		SetUsername("paid-existing").
		SetBalance(27.7).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, "UPDATE users SET display_balance = ? WHERE id = ?", 41.82, u.ID)
	require.NoError(t, err)

	require.NoError(t, repo.ApplyRedeemPaidBalanceCredit(ctx, u.ID, 1666.67, 1000))

	got := loadPaidBalanceSnapshot(t, client, u.ID)
	require.InDelta(t, 1027.7, got.balance, 1e-6)
	require.InDelta(t, 1708.49, got.display, 1e-6)
	require.InDelta(t, 1000.0, got.totalRecharge, 1e-6)
}

func TestUserRepositoryApplyRedeemPaidBalanceCreditLegacyAccountWithoutDisplayBalance(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()

	// 历史账号：没有积分余额（0）但有真实余额，积分余额以真实余额为基数再累加面值
	u, err := client.User.Create().
		SetEmail("paid-legacy@example.com").
		SetPasswordHash("hash").
		SetUsername("paid-legacy").
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, repo.ApplyRedeemPaidBalanceCredit(ctx, u.ID, 100, 60))

	got := loadPaidBalanceSnapshot(t, client, u.ID)
	require.InDelta(t, 70.0, got.balance, 1e-6)
	require.InDelta(t, 110.0, got.display, 1e-6)
	require.InDelta(t, 60.0, got.totalRecharge, 1e-6)
}

func TestUserRepositoryApplyRedeemPaidBalanceCreditUnknownUser(t *testing.T) {
	repo, _ := newUserEntRepo(t)

	err := repo.ApplyRedeemPaidBalanceCredit(context.Background(), 999999, 100, 60)
	require.ErrorIs(t, err, service.ErrUserNotFound)
}

func TestBuildRedeemPaidBalanceCreditSQLPostgresPlaceholderOrder(t *testing.T) {
	// 仅用于构造 Postgres 方言的 client 来检查生成的 SQL，不会真正发起连接
	db, err := sql.Open("sqlite", "file:redeem_paid_pg_dialect?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))

	query, args := buildRedeemPaidBalanceCreditSQL(client, 9, 1666.67, 1000)

	// $1 = 实付金额（真实余额、累计充值），$2 = 积分面值，$3 = 用户 ID
	require.Contains(t, query, "balance = balance + $1")
	require.Contains(t, query, "total_recharged = total_recharged + $1")
	require.Contains(t, query, "COALESCE(display_balance, 0) + $2")
	require.Contains(t, query, "ELSE balance + $2")
	require.Contains(t, query, "WHERE id = $3")
	require.NotContains(t, query, "?")
	require.Equal(t, []any{1000.0, 1666.67, int64(9)}, args)
}

func TestRedeemCodeRepositoryPayAmountRoundTrip(t *testing.T) {
	_, client := newUserEntRepo(t)
	ctx := context.Background()

	// pay_amount 列由 SQL 迁移添加，不在 ent schema 中；测试库用 ent 建表，需手动补列
	_, err := client.ExecContext(ctx, "ALTER TABLE redeem_codes ADD COLUMN pay_amount REAL")
	require.NoError(t, err)

	repo := &redeemCodeRepository{client: client}
	rc, err := client.RedeemCode.Create().
		SetCode("PAID-ROUNDTRIP").
		SetType("balance").
		SetValue(1666.67).
		Save(ctx)
	require.NoError(t, err)

	// 普通码（列为 NULL）读到 nil
	got, err := repo.GetPayAmount(ctx, rc.ID)
	require.NoError(t, err)
	require.Nil(t, got)

	require.NoError(t, repo.SetPayAmount(ctx, rc.ID, 1000))

	got, err = repo.GetPayAmount(ctx, rc.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.InDelta(t, 1000.0, *got, 1e-6)

	require.ErrorIs(t, repo.SetPayAmount(ctx, 999999, 1000), service.ErrRedeemCodeNotFound)
	_, err = repo.GetPayAmount(ctx, 999999)
	require.ErrorIs(t, err, service.ErrRedeemCodeNotFound)
}
