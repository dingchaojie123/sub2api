package service

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type RedeemCode struct {
	ID        int64
	Code      string
	Type      string
	Value     float64
	Status    string
	UsedBy    *int64
	UsedAt    *time.Time
	Notes     string
	CreatedAt time.Time
	ExpiresAt *time.Time

	GroupID      *int64
	ValidityDays int

	// PayAmount 是折扣兑换码对应的客户实付金额，仅余额类型使用。
	// 非空时 Value 是积分面值：兑换时积分余额加 Value，真实余额/累计充值/邀请返利/抽奖次数按 PayAmount 计。
	// 该字段不在 ent schema 中，由 RedeemCodePayAmountStore 通过原生 SQL 读写，列表/详情查询不会填充它。
	PayAmount *float64

	User  *User
	Group *Group
}

// RealAmount 返回该兑换码对应的实际支付金额：折扣码取 PayAmount，普通码取 Value。
func (r *RedeemCode) RealAmount() float64 {
	if r.PayAmount != nil && *r.PayAmount > 0 {
		return *r.PayAmount
	}
	return r.Value
}

func (r *RedeemCode) IsUsed() bool {
	return r.Status == StatusUsed
}

func (r *RedeemCode) IsExpired() bool {
	return r.IsExpiredAt(time.Now())
}

func (r *RedeemCode) IsExpiredAt(now time.Time) bool {
	if r == nil {
		return false
	}
	if r.Status == StatusExpired {
		return true
	}
	return r.Status == StatusUnused && r.ExpiresAt != nil && !r.ExpiresAt.After(now)
}

func (r *RedeemCode) CanUse() bool {
	return r.Status == StatusUnused && !r.IsExpired()
}

func GenerateRedeemCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
