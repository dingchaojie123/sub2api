package service

import (
	"math"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

const (
	balanceRechargeSnapshotProductID      = "balance_product_id"
	balanceRechargeSnapshotPayAmount      = "balance_pay_amount"
	balanceRechargeSnapshotDisplayAmount  = "balance_display_amount"
	balanceRechargeSnapshotOriginalAmount = "balance_original_amount"
	balanceRechargeSnapshotLotteryChances = "balance_lottery_chances"
	balanceRechargeSnapshotStockLabel     = "balance_stock_label"
)

// BalanceRechargeProduct is the fixed user-facing balance product catalog.
// PayAmount is the real credited/deductible balance; DisplayAmount is what users see.
type BalanceRechargeProduct struct {
	ID             string  `json:"id"`
	PayAmount      float64 `json:"pay_amount"`
	DisplayAmount  float64 `json:"display_amount"`
	OriginalAmount float64 `json:"original_amount,omitempty"`
	LotteryChances int     `json:"lottery_chances"`
	StockLabel     string  `json:"stock_label,omitempty"`
}

var balanceRechargeProducts = []BalanceRechargeProduct{
	{ID: "balance_12", PayAmount: 12, DisplayAmount: 18, OriginalAmount: 18, LotteryChances: 0, StockLabel: "库存一般"},
	{ID: "balance_88", PayAmount: 88, DisplayAmount: 128, OriginalAmount: 128, LotteryChances: 1, StockLabel: "库存充足"},
	{ID: "balance_258", PayAmount: 258, DisplayAmount: 388, OriginalAmount: 388, LotteryChances: 3, StockLabel: "库存一般"},
	{ID: "balance_688", PayAmount: 688, DisplayAmount: 1088, OriginalAmount: 1088, LotteryChances: 14, StockLabel: "库存一般"},
	{ID: "balance_1288", PayAmount: 1288, DisplayAmount: 2088, OriginalAmount: 2088, LotteryChances: 30, StockLabel: "库存一般"},
}

// BalanceRechargeProducts returns a copy of the fixed product catalog.
func BalanceRechargeProducts() []BalanceRechargeProduct {
	out := make([]BalanceRechargeProduct, len(balanceRechargeProducts))
	copy(out, balanceRechargeProducts)
	return out
}

func ResolveBalanceRechargeProduct(payAmount float64) (BalanceRechargeProduct, bool) {
	roundedPayAmount := roundBalanceDisplayAmount(payAmount)
	for _, product := range balanceRechargeProducts {
		if roundBalanceDisplayAmount(product.PayAmount) == roundedPayAmount {
			return product, true
		}
	}
	return BalanceRechargeProduct{}, false
}

func balanceRechargeDisplayAmount(payAmount float64) float64 {
	if product, ok := ResolveBalanceRechargeProduct(payAmount); ok {
		return product.DisplayAmount
	}
	return roundBalanceDisplayAmount(payAmount)
}

func appendBalanceRechargeProductSnapshot(snapshot map[string]any, req CreateOrderRequest) map[string]any {
	if req.OrderType != payment.OrderTypeBalance {
		return snapshot
	}
	product, ok := ResolveBalanceRechargeProduct(req.Amount)
	if !ok {
		return snapshot
	}
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	snapshot[balanceRechargeSnapshotProductID] = product.ID
	snapshot[balanceRechargeSnapshotPayAmount] = product.PayAmount
	snapshot[balanceRechargeSnapshotDisplayAmount] = product.DisplayAmount
	snapshot[balanceRechargeSnapshotOriginalAmount] = product.OriginalAmount
	snapshot[balanceRechargeSnapshotLotteryChances] = product.LotteryChances
	snapshot[balanceRechargeSnapshotStockLabel] = product.StockLabel
	return snapshot
}

func roundBalanceDisplayAmount(amount float64) float64 {
	return decimal.NewFromFloat(amount).Round(2).InexactFloat64()
}

// PaymentOrderRealBalanceAmount returns the real balance amount that should be
// credited and later consumed by model usage.
func PaymentOrderRealBalanceAmount(order *dbent.PaymentOrder) float64 {
	if order == nil {
		return 0
	}
	if order.OrderType != payment.OrderTypeBalance {
		return order.Amount
	}
	if amount := psSnapshotFloatValue(order.ProviderSnapshot[balanceRechargeSnapshotPayAmount]); amount > 0 {
		return roundBalanceDisplayAmount(amount)
	}
	if product, ok := legacyBalanceRechargeProductFromPayAmount(order); ok {
		return product.PayAmount
	}
	return roundBalanceDisplayAmount(order.Amount)
}

// PaymentOrderDisplayAmount returns the user-facing balance amount for an order.
// For legacy rows without a product snapshot it falls back to the real order amount,
// except for the old first-tier "pay 12, credit 10" shape which maps to the new
// 12/18 balance product.
func PaymentOrderDisplayAmount(order *dbent.PaymentOrder) float64 {
	if order == nil {
		return 0
	}
	if order.OrderType != payment.OrderTypeBalance {
		return order.Amount
	}
	if amount := psSnapshotFloatValue(order.ProviderSnapshot[balanceRechargeSnapshotDisplayAmount]); amount > 0 {
		return roundBalanceDisplayAmount(amount)
	}
	if amount := psSnapshotFloatValue(order.ProviderSnapshot[balanceRechargeSnapshotPayAmount]); amount > 0 {
		if product, ok := ResolveBalanceRechargeProduct(amount); ok {
			return product.DisplayAmount
		}
		return roundBalanceDisplayAmount(amount)
	}
	if product, ok := legacyBalanceRechargeProductFromPayAmount(order); ok {
		return product.DisplayAmount
	}
	return roundBalanceDisplayAmount(order.Amount)
}

func legacyBalanceRechargeProductFromPayAmount(order *dbent.PaymentOrder) (BalanceRechargeProduct, bool) {
	if order == nil || order.OrderType != payment.OrderTypeBalance {
		return BalanceRechargeProduct{}, false
	}
	if math.Abs(order.FeeRate) > 0.00000001 || order.PayAmount <= order.Amount {
		return BalanceRechargeProduct{}, false
	}
	product, ok := ResolveBalanceRechargeProduct(order.PayAmount)
	if !ok {
		return BalanceRechargeProduct{}, false
	}
	return product, true
}

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

// normalizeSubscriptionUSDToCNYRate 将非法值归一为 0（换算关闭）。
// 与余额倍率不同，0 是合法状态：表示订阅保持 price 直付的存量行为。
func normalizeSubscriptionUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(2).
		InexactFloat64()
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}
