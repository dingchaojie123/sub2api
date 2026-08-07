package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLotteryChanceGrantForBalanceValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value float64
		want  int
	}{
		{name: "88 grants one chance", value: 88, want: 1},
		{name: "168 grants two chances", value: 168, want: 2},
		{name: "258 grants three chances", value: 258, want: 3},
		{name: "358 grants five chances", value: 358, want: 5},
		{name: "458 grants seven chances", value: 458, want: 7},
		{name: "688 grants nine chances", value: 688, want: 9},
		{name: "888 grants twelve chances", value: 888, want: 12},
		{name: "1288 grants fifteen chances", value: 1288, want: 15},
		{name: "old lower tier no longer grants chances", value: 1688, want: 0},
		{name: "lower non-tier value grants none", value: 87.99, want: 0},
		{name: "fractional value that rounds to a tier grants none", value: 87.995, want: 0},
		{name: "value with more than cents precision grants none", value: 88.001, want: 0},
		{name: "higher non-tier value grants none", value: 89, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, LotteryChanceGrantForBalanceValue(tc.value))
		})
	}
}

func TestLotteryPrizesUseUpdatedDenominations(t *testing.T) {
	t.Parallel()

	require.Equal(t, []LotteryPrize{
		{Name: "first", Value: 30, Probability: 1},
		{Name: "second", Value: 10, Probability: 5},
		{Name: "third", Value: 5, Probability: 20},
		{Name: "fourth", Value: 2, Probability: 74},
	}, LotteryPrizes)
}

func TestLotteryServiceDrawEnforcesDailyLimit(t *testing.T) {
	t.Parallel()

	repo := &lotteryRepositoryStub{availableChances: 1, drawsToday: LotteryDailyDrawLimit}
	svc := NewLotteryService(repo)

	_, err := svc.Draw(context.Background(), 101)
	require.ErrorIs(t, err, ErrLotteryDailyDrawLimit)
	require.False(t, repo.drawCalled)
}

func TestLotteryServiceDrawRejectsInsufficientChances(t *testing.T) {
	t.Parallel()

	repo := &lotteryRepositoryStub{}
	svc := NewLotteryService(repo)

	_, err := svc.Draw(context.Background(), 101)
	require.ErrorIs(t, err, ErrLotteryInsufficientChances)
	require.False(t, repo.drawCalled)
}

func TestLotteryServiceDrawReturnsNoPrizeStock(t *testing.T) {
	t.Parallel()

	repo := &lotteryRepositoryStub{
		availableChances: 1,
		drawErr:          ErrLotteryPrizeOutOfStock,
	}
	svc := NewLotteryService(repo)
	svc.drawRoll = func(int) (int, error) { return 0, nil }

	_, err := svc.Draw(context.Background(), 101)
	require.ErrorIs(t, err, ErrLotteryPrizeOutOfStock)
	require.True(t, repo.drawCalled)
}

func TestLotteryServiceDrawReturnsDuplicatePrizeAssignment(t *testing.T) {
	t.Parallel()

	repo := &lotteryRepositoryStub{
		availableChances: 1,
		drawErr:          ErrLotteryPrizeAlreadyAssigned,
	}
	svc := NewLotteryService(repo)
	svc.drawRoll = func(int) (int, error) { return 0, nil }

	_, err := svc.Draw(context.Background(), 101)
	require.ErrorIs(t, err, ErrLotteryPrizeAlreadyAssigned)
	require.True(t, repo.drawCalled)
}

func TestLotteryServiceDrawPassesConfiguredTimezoneToTransactionalRecheck(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("Campaign", 8*60*60)
	repo := &lotteryRepositoryStub{availableChances: 1}
	svc := NewLotteryService(repo)
	svc.location = loc
	svc.now = func() time.Time {
		return time.Date(2026, 1, 2, 16, 30, 0, 0, time.UTC)
	}
	svc.drawRoll = func(int) (int, error) { return 99, nil }

	_, err := svc.Draw(context.Background(), 101)
	require.NoError(t, err)
	require.True(t, repo.drawCalled)
	require.Equal(t, loc, repo.drawNow.Location())
	require.Equal(t, time.Date(2026, 1, 3, 0, 30, 0, 0, loc), repo.drawNow)
}

type lotteryRepositoryStub struct {
	availableChances  int
	drawsToday        int
	addChanceGrantErr error
	drawErr           error
	drawCalled        bool
	drawNow           time.Time
}

func (r *lotteryRepositoryStub) AddChanceGrant(_ context.Context, _, _ int64, _ int) error {
	return r.addChanceGrantErr
}

func (r *lotteryRepositoryStub) GetAvailableChances(_ context.Context, _ int64) (int, error) {
	return r.availableChances, nil
}

func (r *lotteryRepositoryStub) CountDrawsToday(_ context.Context, _ int64, _ time.Time, _ *time.Location) (int, error) {
	return r.drawsToday, nil
}

func (r *lotteryRepositoryStub) DrawWithPrizeCode(_ context.Context, _ int64, _ LotteryPrize, now time.Time) (*LotteryDrawRecord, error) {
	r.drawCalled = true
	r.drawNow = now
	if r.drawErr != nil {
		return nil, r.drawErr
	}
	return &LotteryDrawRecord{ID: 1}, nil
}

func (r *lotteryRepositoryStub) ListUserDrawRecords(_ context.Context, _ int64, _ int) ([]LotteryDrawRecord, error) {
	return nil, nil
}

func (r *lotteryRepositoryStub) BindPrizeCodes(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

func (r *lotteryRepositoryStub) UnbindPrizeCodes(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

func (r *lotteryRepositoryStub) GetPoolSummary(_ context.Context) ([]LotteryPoolSummaryItem, error) {
	return nil, nil
}

var _ LotteryRepository = (*lotteryRepositoryStub)(nil)

func TestLotteryServiceGrantIsIdempotentWhenRedeemWasAlreadyRecorded(t *testing.T) {
	t.Parallel()

	repo := &lotteryRepositoryStub{}
	svc := NewLotteryService(repo)
	repo.addChanceGrantErr = ErrLotteryChanceAlreadyGranted

	require.NoError(t, svc.GrantChancesForRedeem(context.Background(), 101, 202, 888))
}
