package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	OrganizationRoleOwner  = "owner"
	OrganizationRoleAdmin  = "admin"
	OrganizationRoleMember = "member"
)

var (
	ErrOrganizationNotFound       = infraerrors.NotFound("ORGANIZATION_NOT_FOUND", "organization not found")
	ErrOrganizationForbidden      = infraerrors.Forbidden("ORGANIZATION_FORBIDDEN", "organization access denied")
	ErrOrganizationSeatLimit      = infraerrors.Conflict("ORGANIZATION_SEAT_LIMIT", "organization seat limit reached")
	ErrOrganizationInviteInvalid  = infraerrors.Conflict("ORGANIZATION_INVITATION_INVALID", "organization invitation is invalid or expired")
	ErrOrganizationEmailMismatch  = infraerrors.Forbidden("ORGANIZATION_EMAIL_MISMATCH", "organization invitation email does not match current user")
	ErrOrganizationOwnerImmutable = infraerrors.Forbidden("ORGANIZATION_OWNER_IMMUTABLE", "organization owner cannot be removed or disabled")
	ErrOrganizationOwnerOnly      = infraerrors.Forbidden("ORGANIZATION_OWNER_ONLY", "only the organization owner can delete the organization")
	ErrOrganizationHasFrozenUsage = infraerrors.Conflict("ORGANIZATION_HAS_FROZEN_USAGE", "organization has unsettled usage and cannot be deleted")
	ErrOrganizationBalance        = infraerrors.Forbidden("ORGANIZATION_QUOTA_EXHAUSTED", "organization balance is insufficient")
	ErrOrganizationMemberLimit    = infraerrors.TooManyRequests("ORGANIZATION_MEMBER_QUOTA_EXHAUSTED", "organization member quota is exhausted")
)

type Organization struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	OwnerUserID          int64     `json:"owner_user_id"`
	Status               string    `json:"status"`
	SeatLimit            int       `json:"seat_limit"`
	SeatUsed             int       `json:"seat_used"`
	Balance              float64   `json:"balance"`
	DisplayBalance       float64   `json:"display_balance"`
	FrozenBalance        float64   `json:"frozen_balance"`
	FrozenDisplayBalance float64   `json:"frozen_display_balance"`
	Role                 string    `json:"role"`
	CreatedAt            time.Time `json:"created_at"`
}

type OrganizationMember struct {
	ID               int64     `json:"id"`
	OrganizationID   int64     `json:"organization_id"`
	UserID           int64     `json:"user_id"`
	Email            string    `json:"email"`
	Username         string    `json:"username"`
	Role             string    `json:"role"`
	Status           string    `json:"status"`
	MonthlyLimit     float64   `json:"monthly_limit"`
	MonthlyUsed      float64   `json:"monthly_used"`
	UsagePeriodStart time.Time `json:"usage_period_start"`
	JoinedAt         time.Time `json:"joined_at"`
}

type OrganizationInvitation struct {
	ID             int64     `json:"id"`
	OrganizationID int64     `json:"organization_id"`
	Email          string    `json:"email,omitempty"`
	Role           string    `json:"role"`
	MaxUses        int       `json:"max_uses"`
	UsedCount      int       `json:"used_count"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	Token          string    `json:"token,omitempty"`
}

type OrganizationUsageSummary struct {
	TotalRequests int64   `json:"total_requests"`
	TotalCost     float64 `json:"total_cost"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	ImageCount    int64   `json:"image_count"`
	VideoCount    int64   `json:"video_count"`
}

type OrganizationMemberUsage struct {
	UserID        int64   `json:"user_id"`
	Email         string  `json:"email"`
	Username      string  `json:"username"`
	TotalRequests int64   `json:"total_requests"`
	TotalCost     float64 `json:"total_cost"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
}

type OrganizationModelUsage struct {
	Model         string  `json:"model"`
	TotalRequests int64   `json:"total_requests"`
	TotalCost     float64 `json:"total_cost"`
	TotalTokens   int64   `json:"total_tokens"`
}

type OrganizationUsageReport struct {
	Summary OrganizationUsageSummary  `json:"summary"`
	Members []OrganizationMemberUsage `json:"members"`
	Models  []OrganizationModelUsage  `json:"models"`
}

type OrganizationAuditLog struct {
	ID          int64          `json:"id"`
	ActorUserID int64          `json:"actor_user_id"`
	Action      string         `json:"action"`
	TargetType  *string        `json:"target_type,omitempty"`
	TargetID    *string        `json:"target_id,omitempty"`
	Detail      map[string]any `json:"detail"`
	CreatedAt   time.Time      `json:"created_at"`
}

type OrganizationBillingSubject struct {
	OrganizationID int64   `json:"organization_id"`
	MemberUserID   int64   `json:"member_user_id"`
	Balance        float64 `json:"balance"`
	FrozenBalance  float64 `json:"frozen_balance"`
	MonthlyLimit   float64 `json:"monthly_limit"`
	MonthlyUsed    float64 `json:"monthly_used"`
	MonthlyFrozen  float64 `json:"monthly_frozen"`
	Active         bool    `json:"active"`
}

type BillingSourceSnapshot struct {
	Type                 string
	OrganizationID       *int64
	OrganizationMemberID *int64
}

func BillingSourceSnapshotFromAPIKey(apiKey *APIKey) BillingSourceSnapshot {
	snapshot := BillingSourceSnapshot{Type: APIKeyBillingSourcePersonal}
	if apiKey == nil || apiKey.Organization == nil {
		return snapshot
	}
	organizationID := apiKey.Organization.OrganizationID
	memberUserID := apiKey.Organization.MemberUserID
	snapshot.Type = APIKeyBillingSourceOrganization
	snapshot.OrganizationID = &organizationID
	snapshot.OrganizationMemberID = &memberUserID
	return snapshot
}

type CreateOrganizationInput struct {
	Name      string
	SeatLimit int
}

type CreateOrganizationInvitationInput struct {
	Email     string
	Role      string
	MaxUses   int
	ExpiresAt time.Time
}

type OrganizationRepository interface {
	Create(ctx context.Context, ownerUserID int64, in CreateOrganizationInput) (*Organization, error)
	Delete(ctx context.Context, organizationID, actorUserID int64) ([]string, error)
	ListForUser(ctx context.Context, userID int64) ([]Organization, error)
	GetForUser(ctx context.Context, organizationID, userID int64) (*Organization, error)
	ListMembers(ctx context.Context, organizationID, actorUserID int64) ([]OrganizationMember, error)
	CreateInvitation(ctx context.Context, organizationID, actorUserID int64, tokenHash string, in CreateOrganizationInvitationInput) (*OrganizationInvitation, error)
	ListInvitations(ctx context.Context, organizationID, actorUserID int64) ([]OrganizationInvitation, error)
	AcceptInvitation(ctx context.Context, userID int64, userEmail, tokenHash string) (*Organization, error)
	UpdateMember(ctx context.Context, organizationID, actorUserID, memberUserID int64, role, status *string, monthlyLimit *float64) error
	RemoveMember(ctx context.Context, organizationID, actorUserID, memberUserID int64) ([]string, error)
	FundFromUser(ctx context.Context, organizationID, actorUserID int64, amount float64) (*Organization, error)
	AttachAPIKey(ctx context.Context, organizationID, actorUserID, apiKeyID int64) error
	SetAPIKeyBillingSource(ctx context.Context, apiKeyID, actorUserID int64, billingType string, organizationID *int64) (*APIKeyBillingSource, string, error)
	GetBillingSubjectByAPIKey(ctx context.Context, apiKeyID int64) (*OrganizationBillingSubject, error)
	GetAPIKeyBillingSources(ctx context.Context, apiKeyIDs []int64) (map[int64]APIKeyBillingSource, error)
	UsageReport(ctx context.Context, organizationID, actorUserID int64, start, end time.Time) (*OrganizationUsageReport, error)
	ListAuditLogs(ctx context.Context, organizationID, actorUserID int64, limit int) ([]OrganizationAuditLog, error)
}

type OrganizationService struct {
	repo     OrganizationRepository
	userRepo UserRepository
}

func NewOrganizationService(repo OrganizationRepository, userRepo UserRepository) *OrganizationService {
	return &OrganizationService{repo: repo, userRepo: userRepo}
}

func (s *OrganizationService) Create(ctx context.Context, userID int64, in CreateOrganizationInput) (*Organization, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 120 {
		return nil, infraerrors.BadRequest("INVALID_ORGANIZATION_NAME", "organization name is required and must not exceed 120 characters")
	}
	if in.SeatLimit <= 0 {
		in.SeatLimit = 5
	}
	if in.SeatLimit > 10000 {
		return nil, infraerrors.BadRequest("INVALID_ORGANIZATION_SEAT_LIMIT", "organization seat limit must not exceed 10000")
	}
	return s.repo.Create(ctx, userID, in)
}

func (s *OrganizationService) List(ctx context.Context, userID int64) ([]Organization, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *OrganizationService) Delete(ctx context.Context, organizationID, userID int64) ([]string, error) {
	return s.repo.Delete(ctx, organizationID, userID)
}

func (s *OrganizationService) Get(ctx context.Context, organizationID, userID int64) (*Organization, error) {
	return s.repo.GetForUser(ctx, organizationID, userID)
}

func (s *OrganizationService) Members(ctx context.Context, organizationID, userID int64) ([]OrganizationMember, error) {
	return s.repo.ListMembers(ctx, organizationID, userID)
}

func (s *OrganizationService) CreateInvitation(ctx context.Context, organizationID, userID int64, in CreateOrganizationInvitationInput) (*OrganizationInvitation, error) {
	in.Role = strings.ToLower(strings.TrimSpace(in.Role))
	if in.Role == "" {
		in.Role = OrganizationRoleMember
	}
	if in.Role != OrganizationRoleMember && in.Role != OrganizationRoleAdmin {
		return nil, infraerrors.BadRequest("INVALID_ORGANIZATION_ROLE", "invalid organization role")
	}
	if in.MaxUses < 0 || in.MaxUses > 1000 {
		return nil, infraerrors.BadRequest("INVALID_INVITATION_MAX_USES", "invitation max uses must be between 1 and 1000")
	}
	if !in.ExpiresAt.IsZero() && !in.ExpiresAt.After(time.Now()) {
		return nil, infraerrors.BadRequest("INVALID_INVITATION_EXPIRY", "invitation expiry must be in the future")
	}
	token, hash, err := newOrganizationInvitationToken()
	if err != nil {
		return nil, err
	}
	invitation, err := s.repo.CreateInvitation(ctx, organizationID, userID, hash, in)
	if err != nil {
		return nil, err
	}
	invitation.Token = token
	return invitation, nil
}

func (s *OrganizationService) Invitations(ctx context.Context, organizationID, userID int64) ([]OrganizationInvitation, error) {
	return s.repo.ListInvitations(ctx, organizationID, userID)
}

func (s *OrganizationService) AcceptInvitation(ctx context.Context, userID int64, token string) (*Organization, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.AcceptInvitation(ctx, userID, user.Email, organizationInvitationHash(token))
}

func (s *OrganizationService) UpdateMember(ctx context.Context, organizationID, actorID, memberID int64, role, status *string, monthlyLimit *float64) error {
	if role != nil {
		value := strings.ToLower(strings.TrimSpace(*role))
		if value != OrganizationRoleAdmin && value != OrganizationRoleMember {
			return infraerrors.BadRequest("INVALID_ORGANIZATION_ROLE", "invalid organization role")
		}
		role = &value
	}
	if status != nil {
		value := strings.ToLower(strings.TrimSpace(*status))
		if value != "active" && value != "disabled" {
			return infraerrors.BadRequest("INVALID_ORGANIZATION_MEMBER_STATUS", "invalid organization member status")
		}
		status = &value
	}
	if monthlyLimit != nil && *monthlyLimit < 0 {
		return infraerrors.BadRequest("INVALID_ORGANIZATION_MEMBER_LIMIT", "monthly limit must not be negative")
	}
	return s.repo.UpdateMember(ctx, organizationID, actorID, memberID, role, status, monthlyLimit)
}

func (s *OrganizationService) RemoveMember(ctx context.Context, organizationID, actorID, memberID int64) ([]string, error) {
	return s.repo.RemoveMember(ctx, organizationID, actorID, memberID)
}

func (s *OrganizationService) Fund(ctx context.Context, organizationID, actorID int64, amount float64) (*Organization, error) {
	if amount <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ORGANIZATION_FUND_AMOUNT", "fund amount must be positive")
	}
	return s.repo.FundFromUser(ctx, organizationID, actorID, amount)
}

func (s *OrganizationService) AttachAPIKey(ctx context.Context, organizationID, userID, apiKeyID int64) error {
	return s.repo.AttachAPIKey(ctx, organizationID, userID, apiKeyID)
}

func (s *OrganizationService) BillingSubject(ctx context.Context, apiKeyID int64) (*OrganizationBillingSubject, error) {
	return s.repo.GetBillingSubjectByAPIKey(ctx, apiKeyID)
}

func (s *OrganizationService) UsageReport(ctx context.Context, organizationID, userID int64, start, end time.Time) (*OrganizationUsageReport, error) {
	return s.repo.UsageReport(ctx, organizationID, userID, start, end)
}

func (s *OrganizationService) AuditLogs(ctx context.Context, organizationID, userID int64, limit int) ([]OrganizationAuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListAuditLogs(ctx, organizationID, userID, limit)
}
