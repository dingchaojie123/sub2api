package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type redeemRejectRepo struct {
	code      RedeemCode
	useCalled bool
}

func TestRedeemServiceGrantsLotteryChancesOnlyForEligibleBalanceCode(t *testing.T) {
	t.Parallel()

	granter := &lotteryChanceGranterStub{}
	svc := &RedeemService{lotteryService: granter}

	require.NoError(t, svc.grantLotteryChancesForRedeem(context.Background(), 11, &RedeemCode{ID: 21, Type: RedeemTypeBalance, Value: 888}))
	require.Equal(t, []lotteryChanceGrantCall{{userID: 11, redeemCodeID: 21, value: 888}}, granter.calls)

	require.NoError(t, svc.grantLotteryChancesForRedeem(context.Background(), 11, &RedeemCode{ID: 22, Type: RedeemTypeConcurrency, Value: 888}))
	require.NoError(t, svc.grantLotteryChancesForRedeem(context.Background(), 11, &RedeemCode{ID: 23, Type: RedeemTypeBalance, Value: 0}))
	require.Len(t, granter.calls, 1)
}

func TestRedeemServiceLotteryDuplicateGrantDoesNotFailRedeem(t *testing.T) {
	t.Parallel()

	granter := &lotteryChanceGranterStub{err: ErrLotteryChanceAlreadyGranted}
	svc := &RedeemService{lotteryService: granter}

	require.NoError(t, svc.grantLotteryChancesForRedeem(context.Background(), 11, &RedeemCode{ID: 21, Type: RedeemTypeBalance, Value: 888}))
	require.Len(t, granter.calls, 1)
}

func TestRedeemServiceLotteryGrantFailureFailsRedeemStep(t *testing.T) {
	t.Parallel()

	granter := &lotteryChanceGranterStub{err: errors.New("database unavailable")}
	svc := &RedeemService{lotteryService: granter}

	err := svc.grantLotteryChancesForRedeem(context.Background(), 11, &RedeemCode{ID: 21, Type: RedeemTypeBalance, Value: 888})
	require.ErrorContains(t, err, "grant lottery chances")
	require.Len(t, granter.calls, 1)
}

type lotteryChanceGrantCall struct {
	userID       int64
	redeemCodeID int64
	value        float64
}

type lotteryChanceGranterStub struct {
	calls []lotteryChanceGrantCall
	err   error
}

func (s *lotteryChanceGranterStub) GrantChancesForRedeem(_ context.Context, userID, redeemCodeID int64, value float64) error {
	s.calls = append(s.calls, lotteryChanceGrantCall{userID: userID, redeemCodeID: redeemCodeID, value: value})
	return s.err
}

func (r *redeemRejectRepo) Create(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Create call")
}

func (r *redeemRejectRepo) CreateBatch(ctx context.Context, codes []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (r *redeemRejectRepo) GetByID(ctx context.Context, id int64) (*RedeemCode, error) {
	if r.code.ID != id {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}

func (r *redeemRejectRepo) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	if r.code.Code != code {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}

func (r *redeemRejectRepo) Update(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Update call")
}

func (r *redeemRejectRepo) BatchUpdate(ctx context.Context, ids []int64, fields RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (r *redeemRejectRepo) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (r *redeemRejectRepo) Use(ctx context.Context, id, userID int64) error {
	r.useCalled = true
	r.code.Status = StatusUsed
	r.code.UsedBy = &userID
	return nil
}

func (r *redeemRejectRepo) List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *redeemRejectRepo) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *redeemRejectRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (r *redeemRejectRepo) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (r *redeemRejectRepo) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func TestRedeemRejectsInvitationCodeBeforeTransaction(t *testing.T) {
	ctx := context.Background()
	redeemRepo := &redeemRejectRepo{
		code: RedeemCode{
			ID:     1,
			Code:   "INVITE-001",
			Type:   RedeemTypeInvitation,
			Status: StatusUnused,
		},
	}
	redeemService := NewRedeemService(redeemRepo, nil, nil, nil, nil, nil, nil, nil)

	got, err := redeemService.Redeem(ctx, 2, redeemRepo.code.Code)

	require.Nil(t, got)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "REDEEM_CODE_UNSUPPORTED_TYPE", infraerrors.Reason(err))
	require.Equal(t, "invitation codes can only be used during registration", infraerrors.Message(err))
	require.False(t, redeemRepo.useCalled)
	require.Equal(t, StatusUnused, redeemRepo.code.Status)
	require.Nil(t, redeemRepo.code.UsedBy)
}
