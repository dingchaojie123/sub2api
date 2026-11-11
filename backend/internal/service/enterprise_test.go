package service

import (
	"context"
	"errors"
	"testing"
)

func TestValidateCreditCode(t *testing.T) {
	valid := []string{
		"91350100M000100Y43", // GB 32100-2015 标准示例
		"91110000600001760P",
	}
	for _, code := range valid {
		if !ValidateCreditCode(code) {
			t.Errorf("expected %q to be valid", code)
		}
	}
	invalid := []string{
		"",
		"91350100M000100Y44",  // 校验位错误
		"91350100M000100Y4",   // 长度不足
		"91350100M000100Y433", // 长度过长
		"91350100m000100y43",  // 未转大写（规范化在上层完成）
		"91350100I000100Y43",  // 含非法字符 I
		"91350100O000100Y43",  // 含非法字符 O
	}
	for _, code := range invalid {
		if ValidateCreditCode(code) {
			t.Errorf("expected %q to be invalid", code)
		}
	}
}

func TestNormalizeAndValidateEnterpriseInput(t *testing.T) {
	good := EnterpriseInput{
		CompanyName:  "  示例科技有限公司 ",
		CreditCode:   " 91350100m000100y43 ",
		Address:      "上海市浦东新区世纪大道 100 号",
		ContactName:  "张三",
		ContactPhone: "138 0013 8000",
	}
	out, err := NormalizeAndValidateEnterpriseInput(good)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.CompanyName != "示例科技有限公司" || out.CreditCode != "91350100M000100Y43" || out.ContactPhone != "13800138000" {
		t.Fatalf("input was not normalized: %+v", out)
	}

	for _, phone := range []string{"021-12345678", "02112345678", "0755-1234567-88", "12345678"} {
		in := good
		in.ContactPhone = phone
		if _, err := NormalizeAndValidateEnterpriseInput(in); err != nil {
			t.Errorf("phone %q should be accepted: %v", phone, err)
		}
	}
	for _, phone := range []string{"", "12345", "23800138000", "abcdefghijk"} {
		in := good
		in.ContactPhone = phone
		if _, err := NormalizeAndValidateEnterpriseInput(in); err == nil {
			t.Errorf("phone %q should be rejected", phone)
		}
	}

	cases := map[string]func(*EnterpriseInput){
		"company name too short": func(in *EnterpriseInput) { in.CompanyName = "A" },
		"bad credit code":        func(in *EnterpriseInput) { in.CreditCode = "91350100M000100Y44" },
		"address too short":      func(in *EnterpriseInput) { in.Address = "上海" },
		"contact name empty":     func(in *EnterpriseInput) { in.ContactName = " " },
	}
	for name, mutate := range cases {
		in := good
		mutate(&in)
		if _, err := NormalizeAndValidateEnterpriseInput(in); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// fakeEnterpriseRepo 内存实现，覆盖服务层的状态流转规则。
type fakeEnterpriseRepo struct {
	profiles   map[int64]*EnterpriseProfile
	membership map[int64]bool
}

func newFakeEnterpriseRepo() *fakeEnterpriseRepo {
	return &fakeEnterpriseRepo{profiles: map[int64]*EnterpriseProfile{}, membership: map[int64]bool{}}
}

func (f *fakeEnterpriseRepo) Get(_ context.Context, userID int64) (*EnterpriseProfile, error) {
	if p, ok := f.profiles[userID]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeEnterpriseRepo) Create(_ context.Context, userID int64, in EnterpriseInput) (*EnterpriseProfile, error) {
	p := &EnterpriseProfile{UserID: userID, CompanyName: in.CompanyName, CreditCode: in.CreditCode,
		Address: in.Address, ContactName: in.ContactName, ContactPhone: in.ContactPhone, Status: EnterpriseStatusPending}
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeEnterpriseRepo) Update(_ context.Context, userID int64, in EnterpriseInput, status string) (*EnterpriseProfile, error) {
	p := f.profiles[userID]
	p.CompanyName, p.CreditCode, p.Address, p.ContactName, p.ContactPhone, p.Status =
		in.CompanyName, in.CreditCode, in.Address, in.ContactName, in.ContactPhone, status
	return p, nil
}

func (f *fakeEnterpriseRepo) CreditCodeExists(_ context.Context, code string, exclude int64) (bool, error) {
	for id, p := range f.profiles {
		if id != exclude && p.CreditCode == code {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeEnterpriseRepo) HasActiveMembership(_ context.Context, userID int64) (bool, error) {
	return f.membership[userID], nil
}

func (f *fakeEnterpriseRepo) AdminList(context.Context, EnterpriseAdminFilter) ([]EnterpriseAdminItem, int64, error) {
	return nil, 0, nil
}

func (f *fakeEnterpriseRepo) Review(_ context.Context, userID, _ int64, status, reason string) (*EnterpriseProfile, error) {
	p := f.profiles[userID]
	p.Status, p.RejectReason = status, reason
	return p, nil
}

func validEnterpriseInput() EnterpriseInput {
	return EnterpriseInput{
		CompanyName:  "示例科技有限公司",
		CreditCode:   "91350100M000100Y43",
		Address:      "上海市浦东新区世纪大道 100 号",
		ContactName:  "张三",
		ContactPhone: "13800138000",
	}
}

func TestEnterpriseWorkspaceAccess(t *testing.T) {
	repo := newFakeEnterpriseRepo()
	svc := NewEnterpriseService(repo)
	ctx := context.Background()

	// 个人账号、未加入团队：不展示团队页，也不能建团队。
	access, err := svc.WorkspaceAccess(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if access.AccountType != AccountTypePersonal || access.HasTeamAccess || access.CanCreateTeam {
		t.Fatalf("personal non-member: %+v", access)
	}

	// 个人账号、已加入团队：保留团队页，但不能建团队。
	repo.membership[1] = true
	access, _ = svc.WorkspaceAccess(ctx, 1)
	if !access.HasTeamAccess || access.CanCreateTeam || !access.IsTeamMember {
		t.Fatalf("personal member: %+v", access)
	}

	// 企业账号（待认证即可）：可建团队、展示团队页。
	if _, err := svc.CreateProfile(ctx, 2, validEnterpriseInput()); err != nil {
		t.Fatal(err)
	}
	access, _ = svc.WorkspaceAccess(ctx, 2)
	if access.AccountType != AccountTypeEnterprise || !access.CanCreateTeam || !access.HasTeamAccess {
		t.Fatalf("enterprise pending: %+v", access)
	}
	if access.Enterprise == nil || access.Enterprise.Status != EnterpriseStatusPending {
		t.Fatalf("expected pending status, got %+v", access.Enterprise)
	}
}

func TestEnterpriseCreateRejectsDuplicates(t *testing.T) {
	svc := NewEnterpriseService(newFakeEnterpriseRepo())
	ctx := context.Background()
	if _, err := svc.CreateProfile(ctx, 1, validEnterpriseInput()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateProfile(ctx, 1, validEnterpriseInput()); !errors.Is(err, ErrEnterpriseAlreadyRegistered) {
		t.Fatalf("same account twice: got %v", err)
	}
	if _, err := svc.ValidateForRegistration(ctx, validEnterpriseInput()); !errors.Is(err, ErrEnterpriseCreditCodeExists) {
		t.Fatalf("duplicate credit code: got %v", err)
	}
}

func TestEnterpriseUpdateAndReviewFlow(t *testing.T) {
	repo := newFakeEnterpriseRepo()
	svc := NewEnterpriseService(repo)
	ctx := context.Background()
	if _, err := svc.CreateProfile(ctx, 1, validEnterpriseInput()); err != nil {
		t.Fatal(err)
	}

	// 驳回必须填写原因。
	if _, err := svc.Review(ctx, 1, 99, "reject", " "); !errors.Is(err, ErrEnterpriseInvalidReview) {
		t.Fatalf("reject without reason: got %v", err)
	}
	if _, err := svc.Review(ctx, 1, 99, "bogus", ""); !errors.Is(err, ErrEnterpriseInvalidReview) {
		t.Fatalf("bad action: got %v", err)
	}

	// 驳回后修改并重新提交，状态回到 pending。
	if p, err := svc.Review(ctx, 1, 99, "reject", "营业执照信息不一致"); err != nil || p.Status != EnterpriseStatusRejected {
		t.Fatalf("reject: %+v %v", p, err)
	}
	in := validEnterpriseInput()
	in.ContactName = "李四"
	if p, err := svc.UpdateProfile(ctx, 1, in); err != nil || p.Status != EnterpriseStatusPending {
		t.Fatalf("resubmit: %+v %v", p, err)
	}

	// 通过后，公司名称与信用代码锁定，联系人可改。
	if p, err := svc.Review(ctx, 1, 99, "approve", ""); err != nil || p.Status != EnterpriseStatusApproved {
		t.Fatalf("approve: %+v %v", p, err)
	}
	locked := in
	locked.CompanyName = "另一家公司"
	if _, err := svc.UpdateProfile(ctx, 1, locked); !errors.Is(err, ErrEnterpriseIdentityLocked) {
		t.Fatalf("locked identity: got %v", err)
	}
	in.ContactPhone = "13900139000"
	if p, err := svc.UpdateProfile(ctx, 1, in); err != nil || p.Status != EnterpriseStatusApproved {
		t.Fatalf("update contact after approval: %+v %v", p, err)
	}
}

func TestEnterpriseIncompleteLegacyOwnerCompletesProfile(t *testing.T) {
	repo := newFakeEnterpriseRepo()
	repo.profiles[7] = &EnterpriseProfile{UserID: 7, CompanyName: "Crogi-Play团队", Status: EnterpriseStatusIncomplete}
	svc := NewEnterpriseService(repo)
	ctx := context.Background()

	// 信息不全不能直接通过审核。
	if _, err := svc.Review(ctx, 7, 99, "approve", ""); err == nil {
		t.Fatal("incomplete profile must not be approved")
	}
	// 存量团队创建者仍然是企业账号，可继续建团队。
	access, _ := svc.WorkspaceAccess(ctx, 7)
	if !access.CanCreateTeam {
		t.Fatalf("legacy owner should keep team creation: %+v", access)
	}
	// 补全后进入待认证。
	p, err := svc.UpdateProfile(ctx, 7, validEnterpriseInput())
	if err != nil || p.Status != EnterpriseStatusPending {
		t.Fatalf("complete profile: %+v %v", p, err)
	}
}
