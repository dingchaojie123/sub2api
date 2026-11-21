package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// paidRedeemRepo 是支持 pay_amount 的兑换码仓储 stub：嵌入 redeemRejectRepo 以满足 RedeemCodeRepository，
// 只覆盖折扣码相关测试会用到的方法。
type paidRedeemRepo struct {
	redeemRejectRepo

	nextID    int64
	created   []RedeemCode
	deleted   []int64
	setCalls  map[int64]float64
	setErrOn  int64 // 对该 ID 调用 SetPayAmount 时返回错误
	payAmount *float64
	getErr    error
	getCalls  int
}

func (r *paidRedeemRepo) Create(_ context.Context, code *RedeemCode) error {
	r.nextID++
	code.ID = r.nextID
	r.created = append(r.created, *code)
	return nil
}

func (r *paidRedeemRepo) Delete(_ context.Context, id int64) error {
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *paidRedeemRepo) SetPayAmount(_ context.Context, id int64, payAmount float64) error {
	if id == r.setErrOn {
		return errors.New("write failed")
	}
	if r.setCalls == nil {
		r.setCalls = map[int64]float64{}
	}
	r.setCalls[id] = payAmount
	return nil
}

func (r *paidRedeemRepo) GetPayAmount(_ context.Context, _ int64) (*float64, error) {
	r.getCalls++
	return r.payAmount, r.getErr
}

type paidBalanceCall struct {
	userID     int64
	credits    float64
	realAmount float64
}

type paidBalanceRepoStub struct {
	calls []paidBalanceCall
	err   error
}

func (s *paidBalanceRepoStub) ApplyRedeemPaidBalanceCredit(_ context.Context, id int64, credits, realAmount float64) error {
	s.calls = append(s.calls, paidBalanceCall{userID: id, credits: credits, realAmount: realAmount})
	return s.err
}

func floatPtr(v float64) *float64 { return &v }

func TestRedeemCodeRealAmount(t *testing.T) {
	t.Parallel()

	require.Equal(t, 1667.0, (&RedeemCode{Value: 1667}).RealAmount())
	require.Equal(t, 1000.0, (&RedeemCode{Value: 1667, PayAmount: floatPtr(1000)}).RealAmount())
	// 非正的 PayAmount 视为无效，回退到面值
	require.Equal(t, 1667.0, (&RedeemCode{Value: 1667, PayAmount: floatPtr(0)}).RealAmount())
}

func TestRedeemServiceLoadRedeemPayAmount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("discount balance code returns the stored pay amount", func(t *testing.T) {
		repo := &paidRedeemRepo{payAmount: floatPtr(1000)}
		svc := &RedeemService{redeemRepo: repo}

		got, err := svc.loadRedeemPayAmount(ctx, &RedeemCode{ID: 5, Type: RedeemTypeBalance, Value: 1667})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, 1000.0, *got)
	})

	t.Run("regular code has no pay amount", func(t *testing.T) {
		repo := &paidRedeemRepo{}
		svc := &RedeemService{redeemRepo: repo}

		got, err := svc.loadRedeemPayAmount(ctx, &RedeemCode{ID: 5, Type: RedeemTypeBalance, Value: 30})
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("non-balance and non-positive codes skip the lookup", func(t *testing.T) {
		repo := &paidRedeemRepo{payAmount: floatPtr(1000)}
		svc := &RedeemService{redeemRepo: repo}

		for _, code := range []*RedeemCode{
			{ID: 1, Type: RedeemTypeConcurrency, Value: 5},
			{ID: 2, Type: RedeemTypeBalance, Value: -10},
			{ID: 3, Type: RedeemTypeBalance, Value: 0},
			nil,
		} {
			got, err := svc.loadRedeemPayAmount(ctx, code)
			require.NoError(t, err)
			require.Nil(t, got)
		}
		require.Zero(t, repo.getCalls)
	})

	t.Run("repository without pay amount support is treated as a regular code", func(t *testing.T) {
		svc := &RedeemService{redeemRepo: &redeemRejectRepo{}}

		got, err := svc.loadRedeemPayAmount(ctx, &RedeemCode{ID: 5, Type: RedeemTypeBalance, Value: 1667})
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("non-positive stored pay amount is ignored", func(t *testing.T) {
		repo := &paidRedeemRepo{payAmount: floatPtr(0)}
		svc := &RedeemService{redeemRepo: repo}

		got, err := svc.loadRedeemPayAmount(ctx, &RedeemCode{ID: 5, Type: RedeemTypeBalance, Value: 1667})
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("read failure aborts instead of crediting the full value", func(t *testing.T) {
		repo := &paidRedeemRepo{getErr: errors.New("db down")}
		svc := &RedeemService{redeemRepo: repo}

		got, err := svc.loadRedeemPayAmount(ctx, &RedeemCode{ID: 5, Type: RedeemTypeBalance, Value: 1667})
		require.ErrorContains(t, err, "get redeem code pay amount")
		require.Nil(t, got)
	})
}

func TestRedeemServiceCreditPaidRedeemBalance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("passes credits and real amount through", func(t *testing.T) {
		stub := &paidBalanceRepoStub{}
		svc := &RedeemService{redeemPaidUserRepo: stub}

		require.NoError(t, svc.creditPaidRedeemBalance(ctx, 7, 1666.67, 1000))
		require.Equal(t, []paidBalanceCall{{userID: 7, credits: 1666.67, realAmount: 1000}}, stub.calls)
	})

	t.Run("fails when the user repository does not support it", func(t *testing.T) {
		svc := &RedeemService{}

		require.ErrorContains(t, svc.creditPaidRedeemBalance(ctx, 7, 1666.67, 1000), "does not support paid redeem balance credits")
	})

	t.Run("wraps repository errors", func(t *testing.T) {
		stub := &paidBalanceRepoStub{err: ErrUserNotFound}
		svc := &RedeemService{redeemPaidUserRepo: stub}

		err := svc.creditPaidRedeemBalance(ctx, 7, 1666.67, 1000)
		require.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestRedeemServiceGrantsLotteryChancesByPayAmountForDiscountCode(t *testing.T) {
	t.Parallel()

	granter := &lotteryChanceGranterStub{}
	svc := &RedeemService{lotteryService: granter}

	// 面值 1667、实付 888：抽奖次数按实付金额 888 计，与同金额的真实充值一致
	err := svc.grantLotteryChancesForRedeem(context.Background(), 11, &RedeemCode{
		ID: 21, Type: RedeemTypeBalance, Value: 1667, PayAmount: floatPtr(888),
	})
	require.NoError(t, err)
	require.Equal(t, []lotteryChanceGrantCall{{userID: 11, redeemCodeID: 21, value: 888}}, granter.calls)
}

func TestAdminGenerateRedeemCodesDiscountStoresPayAmountAndNotes(t *testing.T) {
	t.Parallel()

	repo := &paidRedeemRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:     2,
		Type:      RedeemTypeBalance,
		Value:     1666.67,
		PayAmount: floatPtr(1000),
		Notes:     "张三",
	})
	require.NoError(t, err)
	require.Len(t, codes, 2)
	for _, code := range codes {
		require.Equal(t, 1666.67, code.Value)
		require.Equal(t, "张三", code.Notes)
		require.NotNil(t, code.PayAmount)
		require.Equal(t, 1000.0, *code.PayAmount)
	}
	require.Equal(t, map[int64]float64{1: 1000, 2: 1000}, repo.setCalls)
	require.Empty(t, repo.deleted)
}

func TestAdminGenerateRedeemCodesWithoutPayAmountIsUnchanged(t *testing.T) {
	t.Parallel()

	repo := &paidRedeemRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 2,
		Type:  RedeemTypeBalance,
		Value: 30,
	})
	require.NoError(t, err)
	require.Len(t, codes, 2)
	for _, code := range codes {
		require.Nil(t, code.PayAmount)
	}
	require.Empty(t, repo.setCalls)
}

func TestAdminGenerateRedeemCodesRejectsInvalidDiscountInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  GenerateRedeemCodesInput
		reason string
	}{
		{
			name:   "pay amount on a non-balance code",
			input:  GenerateRedeemCodesInput{Count: 1, Type: RedeemTypeConcurrency, Value: 5, PayAmount: floatPtr(100)},
			reason: "REDEEM_CODE_PAY_AMOUNT_TYPE",
		},
		{
			name:   "zero pay amount",
			input:  GenerateRedeemCodesInput{Count: 1, Type: RedeemTypeBalance, Value: 100, PayAmount: floatPtr(0)},
			reason: "REDEEM_CODE_PAY_AMOUNT_INVALID",
		},
		{
			name:   "credit value below the pay amount",
			input:  GenerateRedeemCodesInput{Count: 1, Type: RedeemTypeBalance, Value: 999.99, PayAmount: floatPtr(1000)},
			reason: "REDEEM_CODE_PAY_AMOUNT_EXCEEDS_VALUE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &paidRedeemRepo{}
			svc := &adminServiceImpl{redeemCodeRepo: repo}

			input := tt.input
			codes, err := svc.GenerateRedeemCodes(context.Background(), &input)
			require.Error(t, err)
			require.Nil(t, codes)
			require.Equal(t, tt.reason, infraerrors.Reason(err))
			require.Empty(t, repo.created, "nothing must be created when validation fails")
		})
	}
}

func TestAdminGenerateRedeemCodesAllowsPayAmountEqualToValue(t *testing.T) {
	t.Parallel()

	repo := &paidRedeemRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	// 10 折（无折扣）也是合法的折扣码
	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1, Type: RedeemTypeBalance, Value: 100, PayAmount: floatPtr(100),
	})
	require.NoError(t, err)
	require.Len(t, codes, 1)
}

func TestAdminGenerateRedeemCodesRejectsRepositoryWithoutPayAmountSupport(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{redeemCodeRepo: &redeemRejectRepo{}}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1, Type: RedeemTypeBalance, Value: 1667, PayAmount: floatPtr(1000),
	})
	require.Error(t, err)
	require.Nil(t, codes)
	require.Equal(t, "REDEEM_CODE_PAY_AMOUNT_UNSUPPORTED", infraerrors.Reason(err))
}

func TestAdminGenerateRedeemCodesDiscardsWholeBatchWhenPayAmountWriteFails(t *testing.T) {
	t.Parallel()

	// 第 2 张码写 pay_amount 失败：缺少 pay_amount 的码会被当成全额码，必须连同已生成的码一起删除
	repo := &paidRedeemRepo{setErrOn: 2}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 3, Type: RedeemTypeBalance, Value: 1666.67, PayAmount: floatPtr(1000),
	})
	require.ErrorContains(t, err, "set redeem code pay amount")
	require.Nil(t, codes)
	require.Len(t, repo.created, 2, "generation stops at the failing code")
	require.ElementsMatch(t, []int64{1, 2}, repo.deleted)
}
