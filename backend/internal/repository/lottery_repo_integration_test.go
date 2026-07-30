//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestLotteryRepositoryDrawConsumesChanceAndAssignsPrizeCodeOnce(t *testing.T) {
	ctx := context.Background()
	unique := time.Now().UTC().Format("20060102150405.000000000")
	user, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("lottery-repo-%s@example.test", unique)).
		SetPasswordHash("test-password-hash").
		Save(ctx)
	require.NoError(t, err)

	code, err := integrationEntClient.RedeemCode.Create().
		SetCode("LOT-" + unique).
		SetType(service.RedeemTypeBalance).
		SetValue(10).
		SetStatus(service.StatusUnused).
		SetNotes("").
		SetValidityDays(30).
		Save(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM lottery_chance_ledger WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM lottery_draw_records WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM lottery_prize_pool_codes WHERE redeem_code_id = $1", code.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM redeem_codes WHERE id = $1", code.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	repo := NewLotteryRepository(integrationEntClient, integrationDB)
	bound, err := repo.BindPrizeCodes(ctx, []int64{code.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, bound)

	require.NoError(t, repo.AddChanceGrant(ctx, user.ID, code.ID, 1))
	draw, err := repo.DrawWithPrizeCode(ctx, user.ID, service.LotteryPrize{Name: "fourth", Value: 10, Probability: 74}, time.Now())
	require.NoError(t, err)
	require.Equal(t, code.ID, draw.PrizeRedeemCodeID)
	require.Equal(t, code.Code, draw.Code)

	available, err := repo.GetAvailableChances(ctx, user.ID)
	require.NoError(t, err)
	require.Zero(t, available)

	_, err = repo.DrawWithPrizeCode(ctx, user.ID, service.LotteryPrize{Name: "fourth", Value: 10, Probability: 74}, time.Now())
	require.ErrorIs(t, err, service.ErrLotteryInsufficientChances)
}

func TestLotteryRepositoryPoolSummaryCountsOnlyRedeemableInventory(t *testing.T) {
	ctx := context.Background()
	repo := NewLotteryRepository(integrationEntClient, integrationDB)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	availableCode, err := integrationEntClient.RedeemCode.Create().
		SetCode("LSA-" + suffix).
		SetType(service.RedeemTypeBalance).
		SetValue(10).
		SetStatus(service.StatusUnused).
		SetNotes("").
		SetValidityDays(30).
		Save(ctx)
	require.NoError(t, err)
	usedCode, err := integrationEntClient.RedeemCode.Create().
		SetCode("LSU-" + suffix).
		SetType(service.RedeemTypeBalance).
		SetValue(10).
		SetStatus(service.StatusUsed).
		SetNotes("").
		SetValidityDays(30).
		Save(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM lottery_prize_pool_codes WHERE redeem_code_id = ANY($1)", pq.Array([]int64{availableCode.ID, usedCode.ID}))
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM redeem_codes WHERE id = ANY($1)", pq.Array([]int64{availableCode.ID, usedCode.ID}))
	})

	bound, err := repo.BindPrizeCodes(ctx, []int64{availableCode.ID, usedCode.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, bound)

	_, err = integrationDB.ExecContext(ctx, `
INSERT INTO lottery_prize_pool_codes (redeem_code_id, prize_value, status)
VALUES ($1, 10, 'available')
ON CONFLICT (redeem_code_id) DO NOTHING
`, usedCode.ID)
	require.NoError(t, err)

	summary, err := repo.GetPoolSummary(ctx)
	require.NoError(t, err)
	var fourth service.LotteryPoolSummaryItem
	for _, item := range summary {
		if item.PrizeName == "fourth" {
			fourth = item
			break
		}
	}
	require.EqualValues(t, 1, fourth.Available)
}
