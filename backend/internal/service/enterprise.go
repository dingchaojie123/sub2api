package service

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 账号类型：存在企业档案即为企业账号。
const (
	AccountTypePersonal   = "personal"
	AccountTypeEnterprise = "enterprise"
)

// 企业认证状态。
const (
	EnterpriseStatusPending    = "pending"
	EnterpriseStatusApproved   = "approved"
	EnterpriseStatusRejected   = "rejected"
	EnterpriseStatusIncomplete = "incomplete"
)

var (
	ErrEnterpriseRequired          = infraerrors.Forbidden("ENTERPRISE_ACCOUNT_REQUIRED", "only enterprise accounts can create a team")
	ErrEnterpriseNotFound          = infraerrors.NotFound("ENTERPRISE_NOT_FOUND", "enterprise profile not found")
	ErrEnterpriseAlreadyRegistered = infraerrors.Conflict("ENTERPRISE_ALREADY_REGISTERED", "this account is already an enterprise account")
	ErrEnterpriseCreditCodeExists  = infraerrors.Conflict("ENTERPRISE_CREDIT_CODE_EXISTS", "this unified social credit code has already been registered")
	ErrEnterpriseIdentityLocked    = infraerrors.Forbidden("ENTERPRISE_IDENTITY_LOCKED", "company name and credit code cannot be changed after verification")
	ErrEnterpriseInvalidReview     = infraerrors.BadRequest("ENTERPRISE_INVALID_REVIEW", "action must be approve or reject, and a reason is required when rejecting")
)

// EnterpriseInput 企业信息填写项。
type EnterpriseInput struct {
	CompanyName  string `json:"company_name"`
	CreditCode   string `json:"credit_code"`
	Address      string `json:"address"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
}

// EnterpriseProfile 企业档案。
type EnterpriseProfile struct {
	UserID       int64      `json:"user_id"`
	CompanyName  string     `json:"company_name"`
	CreditCode   string     `json:"credit_code"`
	Address      string     `json:"address"`
	ContactName  string     `json:"contact_name"`
	ContactPhone string     `json:"contact_phone"`
	Status       string     `json:"verify_status"`
	RejectReason string     `json:"reject_reason"`
	ReviewedBy   *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// EnterpriseAdminItem 后台审核列表项（附带账号邮箱）。
type EnterpriseAdminItem struct {
	EnterpriseProfile
	Email    string `json:"email"`
	Username string `json:"username"`
}

// EnterpriseAdminFilter 后台列表筛选条件。
type EnterpriseAdminFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

// WorkspaceAccess 当前账号对团队空间的访问权限，供前端决定菜单、按钮是否展示。
type WorkspaceAccess struct {
	AccountType   string             `json:"account_type"`
	Enterprise    *EnterpriseProfile `json:"enterprise"`
	CanCreateTeam bool               `json:"can_create_team"`
	IsTeamMember  bool               `json:"is_team_member"`
	// HasTeamAccess：企业账号，或已加入（有效）团队的账号，才展示团队空间页面。
	HasTeamAccess bool `json:"has_team_access"`
}

type EnterpriseRepository interface {
	// Get 不存在时返回 (nil, nil)。
	Get(ctx context.Context, userID int64) (*EnterpriseProfile, error)
	Create(ctx context.Context, userID int64, in EnterpriseInput) (*EnterpriseProfile, error)
	Update(ctx context.Context, userID int64, in EnterpriseInput, status string) (*EnterpriseProfile, error)
	CreditCodeExists(ctx context.Context, creditCode string, excludeUserID int64) (bool, error)
	HasActiveMembership(ctx context.Context, userID int64) (bool, error)
	AdminList(ctx context.Context, filter EnterpriseAdminFilter) ([]EnterpriseAdminItem, int64, error)
	Review(ctx context.Context, userID, reviewerID int64, status, reason string) (*EnterpriseProfile, error)
}

type EnterpriseService struct {
	repo EnterpriseRepository
}

func NewEnterpriseService(repo EnterpriseRepository) *EnterpriseService {
	return &EnterpriseService{repo: repo}
}

// ---- 校验 ----

const creditCodeCharset = "0123456789ABCDEFGHJKLMNPQRTUWXY"

var (
	creditCodeWeights  = [17]int{1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28}
	creditCodePattern  = regexp.MustCompile(`^[0-9A-HJ-NP-RT-UW-Y]{18}$`)
	mobilePhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
	// 座机：可选区号（3-4 位，可带连字符）+ 7-8 位号码 + 可选分机。
	landlinePhonePattern = regexp.MustCompile(`^(0[0-9]{2,3}-?)?[0-9]{7,8}(-[0-9]{1,6})?$`)
)

// ValidateCreditCode 校验统一社会信用代码（GB 32100-2015）：18 位、字符集、校验位。
func ValidateCreditCode(code string) bool {
	if !creditCodePattern.MatchString(code) {
		return false
	}
	sum := 0
	for i := 0; i < 17; i++ {
		sum += strings.IndexByte(creditCodeCharset, code[i]) * creditCodeWeights[i]
	}
	check := (31 - sum%31) % 31
	return code[17] == creditCodeCharset[check]
}

func runeLenBetween(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max
}

// NormalizeAndValidateEnterpriseInput 去空白、信用代码转大写，并校验各字段。
func NormalizeAndValidateEnterpriseInput(in EnterpriseInput) (EnterpriseInput, error) {
	out := EnterpriseInput{
		CompanyName:  strings.TrimSpace(in.CompanyName),
		CreditCode:   strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(in.CreditCode), " ", "")),
		Address:      strings.TrimSpace(in.Address),
		ContactName:  strings.TrimSpace(in.ContactName),
		ContactPhone: strings.ReplaceAll(strings.TrimSpace(in.ContactPhone), " ", ""),
	}
	if !runeLenBetween(out.CompanyName, 2, 100) {
		return out, infraerrors.BadRequest("INVALID_ENTERPRISE_COMPANY_NAME", "company name must be 2-100 characters")
	}
	if !ValidateCreditCode(out.CreditCode) {
		return out, infraerrors.BadRequest("INVALID_ENTERPRISE_CREDIT_CODE", "invalid unified social credit code")
	}
	if !runeLenBetween(out.Address, 5, 200) {
		return out, infraerrors.BadRequest("INVALID_ENTERPRISE_ADDRESS", "address must be 5-200 characters")
	}
	if !runeLenBetween(out.ContactName, 2, 50) {
		return out, infraerrors.BadRequest("INVALID_ENTERPRISE_CONTACT_NAME", "contact name must be 2-50 characters")
	}
	if !mobilePhonePattern.MatchString(out.ContactPhone) && !landlinePhonePattern.MatchString(out.ContactPhone) {
		return out, infraerrors.BadRequest("INVALID_ENTERPRISE_CONTACT_PHONE", "invalid contact phone")
	}
	return out, nil
}

// ---- 业务 ----

// ValidateForRegistration 注册前预检：格式 + 信用代码是否已被占用。返回规范化后的输入。
func (s *EnterpriseService) ValidateForRegistration(ctx context.Context, in EnterpriseInput) (EnterpriseInput, error) {
	out, err := NormalizeAndValidateEnterpriseInput(in)
	if err != nil {
		return out, err
	}
	exists, err := s.repo.CreditCodeExists(ctx, out.CreditCode, 0)
	if err != nil {
		return out, err
	}
	if exists {
		return out, ErrEnterpriseCreditCodeExists
	}
	return out, nil
}

// CreateProfile 为账号创建企业档案（注册企业版 / 个人账号升级企业账号），初始状态 pending。
func (s *EnterpriseService) CreateProfile(ctx context.Context, userID int64, in EnterpriseInput) (*EnterpriseProfile, error) {
	out, err := NormalizeAndValidateEnterpriseInput(in)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEnterpriseAlreadyRegistered
	}
	return s.repo.Create(ctx, userID, out)
}

// UpdateProfile 修改企业信息。
//   - approved：公司名称、信用代码锁定，只能改地址、联系人、电话，状态保持。
//   - rejected / incomplete：修改后重新提交，状态回到 pending。
//   - pending：可修改全部，状态保持。
func (s *EnterpriseService) UpdateProfile(ctx context.Context, userID int64, in EnterpriseInput) (*EnterpriseProfile, error) {
	out, err := NormalizeAndValidateEnterpriseInput(in)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrEnterpriseNotFound
	}
	status := existing.Status
	switch existing.Status {
	case EnterpriseStatusApproved:
		if out.CompanyName != existing.CompanyName || out.CreditCode != existing.CreditCode {
			return nil, ErrEnterpriseIdentityLocked
		}
	case EnterpriseStatusRejected, EnterpriseStatusIncomplete:
		status = EnterpriseStatusPending
	}
	if out.CreditCode != existing.CreditCode {
		exists, err := s.repo.CreditCodeExists(ctx, out.CreditCode, userID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEnterpriseCreditCodeExists
		}
	}
	return s.repo.Update(ctx, userID, out, status)
}

// WorkspaceAccess 计算团队空间访问权限。
func (s *EnterpriseService) WorkspaceAccess(ctx context.Context, userID int64) (*WorkspaceAccess, error) {
	profile, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	member, err := s.repo.HasActiveMembership(ctx, userID)
	if err != nil {
		return nil, err
	}
	access := &WorkspaceAccess{
		AccountType:  AccountTypePersonal,
		Enterprise:   profile,
		IsTeamMember: member,
	}
	if profile != nil {
		access.AccountType = AccountTypeEnterprise
		access.CanCreateTeam = true
	}
	access.HasTeamAccess = access.CanCreateTeam || member
	return access, nil
}

// IsEnterprise 当前账号是否为企业账号。
func (s *EnterpriseService) IsEnterprise(ctx context.Context, userID int64) (bool, error) {
	profile, err := s.repo.Get(ctx, userID)
	if err != nil {
		return false, err
	}
	return profile != nil, nil
}

// AdminList 后台审核列表。
func (s *EnterpriseService) AdminList(ctx context.Context, filter EnterpriseAdminFilter) ([]EnterpriseAdminItem, int64, error) {
	return s.repo.AdminList(ctx, filter)
}

// Review 后台审核：approve 通过；reject 驳回（必须填写原因）。
func (s *EnterpriseService) Review(ctx context.Context, userID, reviewerID int64, action, reason string) (*EnterpriseProfile, error) {
	reason = strings.TrimSpace(reason)
	var status string
	switch action {
	case "approve":
		status, reason = EnterpriseStatusApproved, ""
	case "reject":
		if reason == "" || utf8.RuneCountInString(reason) > 200 {
			return nil, ErrEnterpriseInvalidReview
		}
		status = EnterpriseStatusRejected
	default:
		return nil, ErrEnterpriseInvalidReview
	}
	existing, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrEnterpriseNotFound
	}
	if existing.Status == EnterpriseStatusIncomplete && status == EnterpriseStatusApproved {
		return nil, infraerrors.Conflict("ENTERPRISE_INFO_INCOMPLETE", "enterprise information is incomplete and cannot be approved")
	}
	return s.repo.Review(ctx, userID, reviewerID, status, reason)
}
