package repository

import "math"

// 账号（个人 / 组织）有两个余额：
//   - balance：真实结算余额（按充值实付金额累计）；
//   - display_balance：前台展示的积分余额。
//
// 模型价格（cost）直接以积分计价。因此一次扣费必须满足：
//   - 积分余额只扣 cost，与账号的充值档位无关；
//   - 真实余额按账号当前「真实余额 / 积分余额」比值同步扣减，使两个余额同时归零，
//     网关准入、冻结等仍按真实余额判断的逻辑无需改动。
//
// 积分余额或真实余额非正时无法推算比值，此时按 1:1 处理。

// creditBalanceEpsilon 是充足性判断的浮点容差，与表中 NUMERIC(20,8) 精度一致。
const creditBalanceEpsilon = 1e-8

// creditsToReal 把积分金额换算成真实余额金额。先乘后除以减少浮点误差。
func creditsToReal(balance, display, credits float64) float64 {
	if balance > 0 && display > 0 {
		return credits * balance / display
	}
	return credits
}

// creditDeductionPlan 描述一次积分扣减 / 冻结对两个余额的影响。
type creditDeductionPlan struct {
	// Credits 是积分余额的扣减量，等于传入的 cost。
	Credits float64
	// Real 是真实余额的扣减量。
	Real float64
	// FrozenCredits 是冻结时 frozen_display_balance 的增加量。
	// 积分余额未启用（<=0）的账号不记录冻结积分，与展示层回退到真实余额的行为一致。
	FrozenCredits float64
	// Sufficient 表示真实余额是否足够；积分与真实余额同比例，等价于积分余额足够。
	Sufficient bool
}

func planCreditDeduction(balance, display, credits float64) creditDeductionPlan {
	realAmount := creditsToReal(balance, display, credits)
	frozenCredits := 0.0
	if display > 0 {
		frozenCredits = credits
	}
	return creditDeductionPlan{
		Credits:       credits,
		Real:          realAmount,
		FrozenCredits: frozenCredits,
		Sufficient:    balance+creditBalanceEpsilon >= realAmount,
	}
}

// creditSettlementPlan 描述结算（capture）或释放（release）一笔冻结时对余额的影响。
type creditSettlementPlan struct {
	// ReleasedReal 是 frozen_balance 的减少量。
	ReleasedReal float64
	// ReleasedCredits 是 frozen_display_balance 的减少量（SQL 侧下限为 0）。
	ReleasedCredits float64
	// RefundReal 是退回真实余额的金额。
	RefundReal float64
	// RefundCredits 是退回积分余额的金额。
	RefundCredits float64
}

// planCreditSettlement 计算把 holdCredits 积分的冻结结算为 actualCredits 的结果。
// actualCredits 为 0 即整笔释放。ok=false 表示冻结余额不足以覆盖本次释放。
func planCreditSettlement(frozenBalance, frozenDisplay, holdCredits, actualCredits float64) (creditSettlementPlan, bool) {
	if actualCredits < 0 {
		actualCredits = 0
	}
	if actualCredits > holdCredits {
		actualCredits = holdCredits
	}

	var releasedReal float64
	switch {
	case frozenDisplay > 0 && holdCredits >= frozenDisplay:
		// 最后一笔冻结：释放全部冻结的真实余额，避免浮点残留。
		releasedReal = frozenBalance
	case frozenDisplay > 0 && frozenBalance > 0:
		releasedReal = holdCredits * frozenBalance / frozenDisplay
	default:
		releasedReal = holdCredits
	}
	if releasedReal > frozenBalance+creditBalanceEpsilon {
		return creditSettlementPlan{}, false
	}
	releasedReal = math.Min(releasedReal, frozenBalance)

	refundCredits := holdCredits - actualCredits
	refundReal := 0.0
	if holdCredits > 0 {
		refundReal = releasedReal * refundCredits / holdCredits
	}
	if frozenDisplay <= 0 {
		// 未记录冻结积分的账号，积分余额不随冻结变动，退回时同样不增加。
		refundCredits = 0
	}

	return creditSettlementPlan{
		ReleasedReal:    releasedReal,
		ReleasedCredits: holdCredits,
		RefundReal:      refundReal,
		RefundCredits:   refundCredits,
	}, true
}
