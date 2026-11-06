//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanCreditDeduction_CreditsAreIndependentOfAccountRatio(t *testing.T) {
	// 充 1288 得 2088 的账号：真实余额 1288，积分余额 2088。
	a := planCreditDeduction(1288, 2088, 6.5)
	// 比值为 1 的账号。
	b := planCreditDeduction(100, 100, 6.5)

	require.InDelta(t, 6.5, a.Credits, 1e-9)
	require.InDelta(t, 6.5, b.Credits, 1e-9)
	require.InDelta(t, 6.5*1288/2088, a.Real, 1e-9)
	require.InDelta(t, 6.5, b.Real, 1e-9)
	require.True(t, a.Sufficient)
	require.True(t, b.Sufficient)
}

func TestPlanCreditDeduction_BothBalancesReachZeroTogether(t *testing.T) {
	plan := planCreditDeduction(1288, 2088, 2088)

	require.InDelta(t, 1288, plan.Real, 1e-9)
	require.True(t, plan.Sufficient)
}

func TestPlanCreditDeduction_FallsBackToOneToOneWithoutRatio(t *testing.T) {
	noDisplay := planCreditDeduction(50, 0, 6.5)
	require.InDelta(t, 6.5, noDisplay.Real, 1e-9)
	require.InDelta(t, 0, noDisplay.FrozenCredits, 1e-9)
	require.True(t, noDisplay.Sufficient)

	overdrawn := planCreditDeduction(-3, 10, 6.5)
	require.InDelta(t, 6.5, overdrawn.Real, 1e-9)
	require.False(t, overdrawn.Sufficient)
}

func TestPlanCreditDeduction_SufficiencyFollowsCreditBalance(t *testing.T) {
	require.True(t, planCreditDeduction(10, 20, 20).Sufficient)
	require.False(t, planCreditDeduction(10, 20, 20.01).Sufficient)
}

func TestPlanCreditDeduction_RecordsFrozenCreditsOnlyWhenDisplayEnabled(t *testing.T) {
	require.InDelta(t, 24, planCreditDeduction(1288, 2088, 24).FrozenCredits, 1e-9)
	require.InDelta(t, 0, planCreditDeduction(1288, 0, 24).FrozenCredits, 1e-9)
}

func TestPlanCreditSettlement_FullCaptureMatchesDesignExample(t *testing.T) {
	// 设计文档示例：2088 积分，15 秒视频 1.6 积分/秒，冻结并全额结算后剩 2064。
	hold := planCreditDeduction(1288, 2088, 24)
	balance := 1288 - hold.Real
	display := 2088 - hold.Credits
	frozenBalance, frozenDisplay := hold.Real, hold.FrozenCredits

	plan, ok := planCreditSettlement(frozenBalance, frozenDisplay, 24, 24)
	require.True(t, ok)

	display += plan.RefundCredits
	balance += plan.RefundReal
	require.InDelta(t, 2064, display, 1e-9)
	require.InDelta(t, 1288-hold.Real, balance, 1e-9)
	require.InDelta(t, hold.Real, plan.ReleasedReal, 1e-9)
	require.InDelta(t, 0, frozenBalance-plan.ReleasedReal, 1e-9)
}

func TestPlanCreditSettlement_PartialCaptureRefundsProportionally(t *testing.T) {
	hold := planCreditDeduction(1288, 2088, 24)

	plan, ok := planCreditSettlement(hold.Real, hold.FrozenCredits, 24, 12)
	require.True(t, ok)

	require.InDelta(t, 12, plan.RefundCredits, 1e-9)
	require.InDelta(t, hold.Real/2, plan.RefundReal, 1e-9)
	require.InDelta(t, hold.Real, plan.ReleasedReal, 1e-9)
	require.InDelta(t, 24, plan.ReleasedCredits, 1e-9)
}

func TestPlanCreditSettlement_ReleaseRefundsEverything(t *testing.T) {
	hold := planCreditDeduction(1288, 2088, 24)

	plan, ok := planCreditSettlement(hold.Real, hold.FrozenCredits, 24, 0)
	require.True(t, ok)

	require.InDelta(t, 24, plan.RefundCredits, 1e-9)
	require.InDelta(t, hold.Real, plan.RefundReal, 1e-9)
}

func TestPlanCreditSettlement_SeveralHoldsReleaseTheirOwnShare(t *testing.T) {
	// 两笔冻结：24 积分对应真实 15，16 积分对应真实 10。
	plan, ok := planCreditSettlement(25, 40, 16, 0)
	require.True(t, ok)

	require.InDelta(t, 10, plan.ReleasedReal, 1e-9)
	require.InDelta(t, 10, plan.RefundReal, 1e-9)
	require.InDelta(t, 16, plan.RefundCredits, 1e-9)
}

func TestPlanCreditSettlement_AccountsWithoutFrozenCreditsKeepDisplayUntouched(t *testing.T) {
	plan, ok := planCreditSettlement(5, 0, 5, 2)
	require.True(t, ok)

	require.InDelta(t, 5, plan.ReleasedReal, 1e-9)
	require.InDelta(t, 3, plan.RefundReal, 1e-9)
	require.InDelta(t, 0, plan.RefundCredits, 1e-9)
}

func TestPlanCreditSettlement_RejectsWhenFrozenBalanceIsTooSmall(t *testing.T) {
	_, ok := planCreditSettlement(1, 0, 5, 5)
	require.False(t, ok)
}

func TestPlanCreditSettlement_ClampsActualIntoHoldRange(t *testing.T) {
	over, ok := planCreditSettlement(10, 20, 20, 99)
	require.True(t, ok)
	require.InDelta(t, 0, over.RefundCredits, 1e-9)

	negative, ok := planCreditSettlement(10, 20, 20, -5)
	require.True(t, ok)
	require.InDelta(t, 20, negative.RefundCredits, 1e-9)
}
