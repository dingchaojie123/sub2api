//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// secondLevelRepoStub 内嵌接口（nil），只实现 GetAffiliateDetail 路径会调用的方法；
// 其余方法若被误调用会因 nil 接口 panic，从而暴露测试外的意外依赖。
type secondLevelRepoStub struct {
	AffiliateRepository
	invitees         []AffiliateInvitee
	secondLevel      []AffiliateSecondLevelInvitee
	secondLevelTotal int
	secondLevelErr   error
}

func (r *secondLevelRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (r *secondLevelRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	return &AffiliateSummary{UserID: userID, AffCode: "ROOTCODE", CreatedAt: time.Now()}, nil
}

func (r *secondLevelRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return r.invitees, nil
}

func (r *secondLevelRepoStub) ListSecondLevelInvitees(context.Context, int64, int) ([]AffiliateSecondLevelInvitee, int, error) {
	return r.secondLevel, r.secondLevelTotal, r.secondLevelErr
}

// TestGetAffiliateDetail_SecondLevelInvitees 验证二级用户：邮箱（含「经由」的一级用户邮箱）
// 被脱敏，总数取自仓储（可大于返回条数），且不影响一级列表。
func TestGetAffiliateDetail_SecondLevelInvitees(t *testing.T) {
	t.Parallel()
	created := time.Now()
	svc := &AffiliateService{repo: &secondLevelRepoStub{
		invitees: []AffiliateInvitee{{UserID: 2, Email: "bob@gmail.com"}},
		secondLevel: []AffiliateSecondLevelInvitee{
			{UserID: 3, Email: "carol@gmail.com", Username: "carol", CreatedAt: &created, ViaUserID: 2, ViaEmail: "bob@gmail.com"},
		},
		secondLevelTotal: 250, // 超过单次返回上限时，总数仍要如实反映
	}}

	detail, err := svc.GetAffiliateDetail(context.Background(), 1)
	require.NoError(t, err)

	require.Len(t, detail.Invitees, 1)
	require.Equal(t, "b***@g***.com", detail.Invitees[0].Email)

	require.Equal(t, 250, detail.SecondLevelCount)
	require.Len(t, detail.SecondLevelInvitees, 1)
	got := detail.SecondLevelInvitees[0]
	require.Equal(t, int64(3), got.UserID)
	require.Equal(t, "c***@g***.com", got.Email, "second-level email must be masked")
	require.Equal(t, "b***@g***.com", got.ViaEmail, "via email must be masked")
	require.Equal(t, int64(2), got.ViaUserID)
}

// TestGetAffiliateDetail_SecondLevelFailureDegrades 验证二级查询失败时整页不报错：
// 一级数据、额度等照常返回，二级降级为空列表（非 nil，避免前端拿到 null）。
func TestGetAffiliateDetail_SecondLevelFailureDegrades(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{repo: &secondLevelRepoStub{
		invitees:         []AffiliateInvitee{{UserID: 2, Email: "bob@gmail.com"}},
		secondLevelTotal: 9,
		secondLevelErr:   errors.New("db down"),
	}}

	detail, err := svc.GetAffiliateDetail(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, detail.Invitees, 1)
	require.Equal(t, 0, detail.SecondLevelCount)
	require.NotNil(t, detail.SecondLevelInvitees)
	require.Empty(t, detail.SecondLevelInvitees)
}

// TestGetAffiliateDetail_SecondLevelEmptyIsNotNil 验证无二级用户时返回空数组而非 null。
func TestGetAffiliateDetail_SecondLevelEmptyIsNotNil(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{repo: &secondLevelRepoStub{}} // 仓储返回 nil 切片

	detail, err := svc.GetAffiliateDetail(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, detail.SecondLevelInvitees)
	require.Empty(t, detail.SecondLevelInvitees)
	require.Equal(t, 0, detail.SecondLevelCount)
}

// TestResolveRebateRatePercent_PerUserOverride verifies that per-inviter
// AffRebateRatePercent overrides the global rate, that NULL falls back to the
// global rate, and that out-of-range exclusive rates are clamped silently.
//
// SettingService is left nil here so globalRebateRatePercent returns the
// documented default (AffiliateRebateRateDefault = 20%) — this exercises the
// fallback path without spinning up a settings stub.
func TestResolveRebateRatePercent_PerUserOverride(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	// nil exclusive rate → falls back to global default (20%)
	require.InDelta(t, AffiliateRebateRateDefault,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}), 1e-9)

	// exclusive rate set → overrides global
	rate := 50.0
	require.InDelta(t, 50.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &rate}), 1e-9)

	// exclusive rate 0 → returns 0 (no rebate, intentional)
	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &zero}), 1e-9)

	// exclusive rate above max → clamped to Max
	tooHigh := 250.0
	require.InDelta(t, AffiliateRebateRateMax,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}), 1e-9)

	// exclusive rate below min → clamped to Min
	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooLow}), 1e-9)
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled
// safely handles a nil settingService dependency by returning the default
// (off). This protects callers from nil-pointer crashes in misconfigured
// environments.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
