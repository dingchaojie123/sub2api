package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type organizationRepository struct{ db *sql.DB }

func NewOrganizationRepository(db *sql.DB) service.OrganizationRepository {
	return &organizationRepository{db: db}
}

func (r *organizationRepository) Create(ctx context.Context, ownerUserID int64, in service.CreateOrganizationInput) (_ *service.Organization, err error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 120 {
		return nil, errors.New("organization name is required and must not exceed 120 characters")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var org service.Organization
	err = tx.QueryRowContext(ctx, `INSERT INTO organizations(name, owner_user_id, seat_limit)
		VALUES($1,$2,$3) RETURNING id,name,owner_user_id,status,seat_limit,balance,display_balance,frozen_balance,frozen_display_balance,created_at`,
		name, ownerUserID, in.SeatLimit).Scan(&org.ID, &org.Name, &org.OwnerUserID, &org.Status, &org.SeatLimit, &org.Balance, &org.DisplayBalance, &org.FrozenBalance, &org.FrozenDisplayBalance, &org.CreatedAt)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,'owner')`, org.ID, ownerUserID); err != nil {
		return nil, err
	}
	if err = insertOrganizationAudit(ctx, tx, org.ID, ownerUserID, "organization.create", "organization", fmt.Sprint(org.ID), map[string]any{"name": name, "seat_limit": in.SeatLimit}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	org.Role, org.SeatUsed = service.OrganizationRoleOwner, 1
	return &org, nil
}

func (r *organizationRepository) Delete(ctx context.Context, organizationID, actorUserID int64) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var ownerUserID int64
	var status string
	var balance, displayBalance, frozenBalance, frozenDisplayBalance float64
	err = tx.QueryRowContext(ctx, `SELECT owner_user_id,status,balance,display_balance,frozen_balance,frozen_display_balance
		FROM organizations WHERE id=$1 FOR UPDATE`, organizationID).
		Scan(&ownerUserID, &status, &balance, &displayBalance, &frozenBalance, &frozenDisplayBalance)
	if errors.Is(err, sql.ErrNoRows) || err == nil && status != "active" {
		return nil, service.ErrOrganizationNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerUserID != actorUserID {
		return nil, service.ErrOrganizationOwnerOnly
	}
	if frozenBalance > 0.00000001 || frozenDisplayBalance > 0.00000001 {
		return nil, service.ErrOrganizationHasFrozenUsage
	}

	rows, err := tx.QueryContext(ctx, `SELECT k.key FROM api_keys k
		JOIN organization_api_keys ok ON ok.api_key_id=k.id WHERE ok.organization_id=$1 AND ok.status='active'`, organizationID)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,display_balance=display_balance+$2,updated_at=NOW()
		WHERE id=$3 AND deleted_at IS NULL`, balance, displayBalance, ownerUserID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO organization_quota_ledger
		(organization_id,actor_user_id,entry_type,amount,balance_after,detail)
		VALUES($1,$2,'delete_refund',$3,0,jsonb_build_object('destination','owner_personal_balance','display_amount',$4::numeric))`,
		organizationID, actorUserID, -balance, displayBalance); err != nil {
		return nil, err
	}
	if err = insertOrganizationAudit(ctx, tx, organizationID, actorUserID, "organization.delete", "organization", fmt.Sprint(organizationID), map[string]any{"refunded_amount": balance, "refunded_display_amount": displayBalance}); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE api_keys SET status='inactive',updated_at=NOW()
		WHERE id IN(SELECT api_key_id FROM organization_api_keys WHERE organization_id=$1 AND status='active')`, organizationID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_api_keys SET status='inactive' WHERE organization_id=$1 AND status='active'`, organizationID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_members SET status='disabled',updated_at=NOW() WHERE organization_id=$1`, organizationID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_invitations SET revoked_at=COALESCE(revoked_at,NOW()) WHERE organization_id=$1`, organizationID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organizations SET status='deleted',balance=0,display_balance=0,updated_at=NOW() WHERE id=$1`, organizationID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *organizationRepository) ListForUser(ctx context.Context, userID int64) ([]service.Organization, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT o.id,o.name,o.owner_user_id,o.status,o.seat_limit,
		o.balance,o.display_balance,o.frozen_balance,o.frozen_display_balance,m.role,o.created_at,
		(SELECT COUNT(*) FROM organization_members sm WHERE sm.organization_id=o.id AND sm.status='active')
		FROM organizations o JOIN organization_members m ON m.organization_id=o.id
		WHERE m.user_id=$1 AND m.status='active' AND o.status='active' ORDER BY o.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []service.Organization{}
	for rows.Next() {
		var item service.Organization
		if err := rows.Scan(&item.ID, &item.Name, &item.OwnerUserID, &item.Status, &item.SeatLimit, &item.Balance, &item.DisplayBalance, &item.FrozenBalance, &item.FrozenDisplayBalance, &item.Role, &item.CreatedAt, &item.SeatUsed); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *organizationRepository) GetForUser(ctx context.Context, organizationID, userID int64) (*service.Organization, error) {
	var item service.Organization
	err := r.db.QueryRowContext(ctx, `SELECT o.id,o.name,o.owner_user_id,o.status,o.seat_limit,
		o.balance,o.display_balance,o.frozen_balance,o.frozen_display_balance,m.role,o.created_at,
		(SELECT COUNT(*) FROM organization_members sm WHERE sm.organization_id=o.id AND sm.status='active')
		FROM organizations o JOIN organization_members m ON m.organization_id=o.id
		WHERE o.id=$1 AND m.user_id=$2 AND m.status='active' AND o.status='active'`, organizationID, userID).
		Scan(&item.ID, &item.Name, &item.OwnerUserID, &item.Status, &item.SeatLimit, &item.Balance, &item.DisplayBalance, &item.FrozenBalance, &item.FrozenDisplayBalance, &item.Role, &item.CreatedAt, &item.SeatUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrOrganizationNotFound
	}
	return &item, err
}

func (r *organizationRepository) ListMembers(ctx context.Context, organizationID, actorUserID int64) ([]service.OrganizationMember, error) {
	org, err := r.GetForUser(ctx, organizationID, actorUserID)
	if err != nil {
		return nil, err
	}
	query := `SELECT m.id,m.organization_id,m.user_id,u.email,u.username,m.role,m.status,
		m.monthly_limit,m.monthly_used,m.usage_period_start,m.joined_at
		FROM organization_members m JOIN users u ON u.id=m.user_id
		WHERE m.organization_id=$1`
	args := []any{organizationID}
	if org.Role == service.OrganizationRoleMember {
		query += ` AND m.user_id=$2`
		args = append(args, actorUserID)
	}
	query += ` ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,m.joined_at`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []service.OrganizationMember{}
	for rows.Next() {
		var m service.OrganizationMember
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Email, &m.Username, &m.Role, &m.Status, &m.MonthlyLimit, &m.MonthlyUsed, &m.UsagePeriodStart, &m.JoinedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *organizationRepository) CreateInvitation(ctx context.Context, organizationID, actorUserID int64, tokenHash string, in service.CreateOrganizationInvitationInput) (*service.OrganizationInvitation, error) {
	role := strings.ToLower(strings.TrimSpace(in.Role))
	if role == "" {
		role = service.OrganizationRoleMember
	}
	if role != service.OrganizationRoleMember && role != service.OrganizationRoleAdmin {
		return nil, errors.New("invalid organization role")
	}
	if in.MaxUses <= 0 {
		in.MaxUses = 1
	}
	if in.ExpiresAt.IsZero() {
		in.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	actorRole, err := organizationActorRole(ctx, tx, organizationID, actorUserID)
	if err != nil {
		return nil, err
	}
	if actorRole != service.OrganizationRoleOwner && actorRole != service.OrganizationRoleAdmin {
		return nil, service.ErrOrganizationForbidden
	}
	if role == service.OrganizationRoleAdmin && actorRole != service.OrganizationRoleOwner {
		return nil, service.ErrOrganizationForbidden
	}
	var item service.OrganizationInvitation
	email := strings.ToLower(strings.TrimSpace(in.Email))
	err = tx.QueryRowContext(ctx, `INSERT INTO organization_invitations(organization_id,email,token_hash,role,max_uses,expires_at,created_by)
		VALUES($1,NULLIF($2,''),$3,$4,$5,$6,$7) RETURNING id,organization_id,COALESCE(email,''),role,max_uses,used_count,expires_at,created_at`, organizationID, email, tokenHash, role, in.MaxUses, in.ExpiresAt, actorUserID).
		Scan(&item.ID, &item.OrganizationID, &item.Email, &item.Role, &item.MaxUses, &item.UsedCount, &item.ExpiresAt, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err = insertOrganizationAudit(ctx, tx, organizationID, actorUserID, "invitation.create", "invitation", fmt.Sprint(item.ID), map[string]any{"email": email, "role": role, "max_uses": in.MaxUses}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *organizationRepository) ListInvitations(ctx context.Context, organizationID, actorUserID int64) ([]service.OrganizationInvitation, error) {
	org, err := r.GetForUser(ctx, organizationID, actorUserID)
	if err != nil {
		return nil, err
	}
	if org.Role != service.OrganizationRoleOwner && org.Role != service.OrganizationRoleAdmin {
		return nil, service.ErrOrganizationForbidden
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,organization_id,COALESCE(email,''),role,max_uses,used_count,expires_at,created_at FROM organization_invitations WHERE organization_id=$1 AND revoked_at IS NULL ORDER BY created_at DESC`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []service.OrganizationInvitation{}
	for rows.Next() {
		var i service.OrganizationInvitation
		if err := rows.Scan(&i.ID, &i.OrganizationID, &i.Email, &i.Role, &i.MaxUses, &i.UsedCount, &i.ExpiresAt, &i.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (r *organizationRepository) AcceptInvitation(ctx context.Context, userID int64, userEmail, tokenHash string) (*service.Organization, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var invID, orgID int64
	var inviteEmail, role string
	var maxUses, used int
	var expires time.Time
	err = tx.QueryRowContext(ctx, `SELECT id,organization_id,COALESCE(email,''),role,max_uses,used_count,expires_at FROM organization_invitations WHERE token_hash=$1 AND revoked_at IS NULL FOR UPDATE`, tokenHash).Scan(&invID, &orgID, &inviteEmail, &role, &maxUses, &used, &expires)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (used >= maxUses || !expires.After(time.Now())) {
		return nil, service.ErrOrganizationInviteInvalid
	}
	if err != nil {
		return nil, err
	}
	if inviteEmail != "" && !strings.EqualFold(inviteEmail, strings.TrimSpace(userEmail)) {
		return nil, service.ErrOrganizationEmailMismatch
	}
	var seats, active int
	err = tx.QueryRowContext(ctx, `SELECT o.seat_limit,(SELECT COUNT(*) FROM organization_members m WHERE m.organization_id=o.id AND m.status='active') FROM organizations o WHERE o.id=$1 AND o.status='active' FOR UPDATE`, orgID).Scan(&seats, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrOrganizationNotFound
	}
	if err != nil {
		return nil, err
	}
	var existingStatus string
	lookupErr := tx.QueryRowContext(ctx, `SELECT status FROM organization_members WHERE organization_id=$1 AND user_id=$2`, orgID, userID).Scan(&existingStatus)
	if (errors.Is(lookupErr, sql.ErrNoRows) || existingStatus != "active") && active >= seats {
		return nil, service.ErrOrganizationSeatLimit
	}
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
	}
	if lookupErr == nil && existingStatus == "active" {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetForUser(ctx, orgID, userID)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO organization_members(organization_id,user_id,role,status) VALUES($1,$2,$3,'active') ON CONFLICT(organization_id,user_id) DO UPDATE SET role=EXCLUDED.role,status='active',updated_at=NOW()`, orgID, userID, role)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_invitations SET used_count=used_count+1 WHERE id=$1`, invID); err != nil {
		return nil, err
	}
	if err = insertOrganizationAudit(ctx, tx, orgID, userID, "member.join", "member", fmt.Sprint(userID), map[string]any{"invitation_id": invID}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetForUser(ctx, orgID, userID)
}

func (r *organizationRepository) UpdateMember(ctx context.Context, organizationID, actorUserID, memberUserID int64, role, status *string, monthlyLimit *float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	actorRole, err := organizationActorRole(ctx, tx, organizationID, actorUserID)
	if err != nil {
		return err
	}
	var targetRole, targetStatus string
	err = tx.QueryRowContext(ctx, `SELECT role,status FROM organization_members WHERE organization_id=$1 AND user_id=$2 FOR UPDATE`, organizationID, memberUserID).Scan(&targetRole, &targetStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrOrganizationNotFound
	}
	if err != nil {
		return err
	}
	if targetRole == service.OrganizationRoleOwner {
		return service.ErrOrganizationOwnerImmutable
	}
	if actorRole != service.OrganizationRoleOwner && (actorRole != service.OrganizationRoleAdmin || targetRole != service.OrganizationRoleMember) {
		return service.ErrOrganizationForbidden
	}
	if role != nil {
		v := strings.ToLower(strings.TrimSpace(*role))
		if v != service.OrganizationRoleAdmin && v != service.OrganizationRoleMember {
			return errors.New("invalid organization role")
		}
		if v == service.OrganizationRoleAdmin && actorRole != service.OrganizationRoleOwner {
			return service.ErrOrganizationForbidden
		}
		role = &v
	}
	if status != nil {
		v := strings.ToLower(strings.TrimSpace(*status))
		if v != "active" && v != "disabled" {
			return errors.New("invalid member status")
		}
		status = &v
		if v == "active" && targetStatus != "active" {
			var seats, active int
			if err := tx.QueryRowContext(ctx, `SELECT o.seat_limit,(SELECT COUNT(*) FROM organization_members m WHERE m.organization_id=o.id AND m.status='active') FROM organizations o WHERE o.id=$1 FOR UPDATE`, organizationID).Scan(&seats, &active); err != nil {
				return err
			}
			if active >= seats {
				return service.ErrOrganizationSeatLimit
			}
		}
	}
	if monthlyLimit != nil && *monthlyLimit < 0 {
		return errors.New("monthly limit must not be negative")
	}
	_, err = tx.ExecContext(ctx, `UPDATE organization_members SET role=COALESCE($3,role),status=COALESCE($4,status),monthly_limit=COALESCE($5,monthly_limit),updated_at=NOW() WHERE organization_id=$1 AND user_id=$2`, organizationID, memberUserID, role, status, monthlyLimit)
	if err != nil {
		return err
	}
	if status != nil && *status == "disabled" {
		if _, err = tx.ExecContext(ctx, `UPDATE api_keys SET status='inactive',updated_at=NOW() WHERE id IN(SELECT api_key_id FROM organization_api_keys WHERE organization_id=$1 AND member_user_id=$2 AND status='active')`, organizationID, memberUserID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE organization_api_keys SET status='inactive' WHERE organization_id=$1 AND member_user_id=$2 AND status='active'`, organizationID, memberUserID)
		if err != nil {
			return err
		}
	}
	if err = insertOrganizationAudit(ctx, tx, organizationID, actorUserID, "member.update", "member", fmt.Sprint(memberUserID), map[string]any{"role": role, "status": status, "monthly_limit": monthlyLimit}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *organizationRepository) RemoveMember(ctx context.Context, organizationID, actorUserID, memberUserID int64) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	actorRole, err := organizationActorRole(ctx, tx, organizationID, actorUserID)
	if err != nil {
		return nil, err
	}
	var targetRole string
	err = tx.QueryRowContext(ctx, `SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2 FOR UPDATE`, organizationID, memberUserID).Scan(&targetRole)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrOrganizationNotFound
	}
	if err != nil {
		return nil, err
	}
	if targetRole == service.OrganizationRoleOwner {
		return nil, service.ErrOrganizationOwnerImmutable
	}
	if actorRole != service.OrganizationRoleOwner && (actorRole != service.OrganizationRoleAdmin || targetRole != service.OrganizationRoleMember) {
		return nil, service.ErrOrganizationForbidden
	}
	rows, err := tx.QueryContext(ctx, `SELECT k.key FROM api_keys k JOIN organization_api_keys ok ON ok.api_key_id=k.id WHERE ok.organization_id=$1 AND ok.member_user_id=$2 AND ok.status='active'`, organizationID, memberUserID)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if _, err = tx.ExecContext(ctx, `UPDATE api_keys SET status='inactive',updated_at=NOW() WHERE id IN(SELECT api_key_id FROM organization_api_keys WHERE organization_id=$1 AND member_user_id=$2 AND status='active')`, organizationID, memberUserID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_api_keys SET status='inactive' WHERE organization_id=$1 AND member_user_id=$2 AND status='active'`, organizationID, memberUserID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_members SET status='disabled',updated_at=NOW() WHERE organization_id=$1 AND user_id=$2`, organizationID, memberUserID); err != nil {
		return nil, err
	}
	if err = insertOrganizationAudit(ctx, tx, organizationID, actorUserID, "member.remove", "member", fmt.Sprint(memberUserID), nil); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *organizationRepository) FundFromUser(ctx context.Context, organizationID, actorUserID int64, amount float64) (*service.Organization, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	role, err := organizationActorRole(ctx, tx, organizationID, actorUserID)
	if err != nil {
		return nil, err
	}
	if role != service.OrganizationRoleOwner && role != service.OrganizationRoleAdmin {
		return nil, service.ErrOrganizationForbidden
	}
	var actualAmount float64
	err = tx.QueryRowContext(ctx, `WITH locked AS (
		SELECT id,balance,CASE WHEN COALESCE(display_balance,0)>0 THEN display_balance ELSE balance END AS display_balance
		FROM users WHERE id=$2 AND deleted_at IS NULL FOR UPDATE
	), converted AS (
		SELECT id,balance,display_balance,CASE WHEN balance>0 AND display_balance>0 THEN $1*balance/display_balance ELSE $1 END AS actual_amount FROM locked
	), updated AS (
		UPDATE users u SET balance=u.balance-c.actual_amount,display_balance=GREATEST(0,c.display_balance-$1),updated_at=NOW()
		FROM converted c WHERE u.id=c.id AND c.display_balance >= $1 AND c.balance >= c.actual_amount
		RETURNING c.actual_amount
	) SELECT actual_amount FROM updated`, amount, actorUserID).Scan(&actualAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrOrganizationBalance
	}
	if err != nil {
		return nil, err
	}
	var balance float64
	err = tx.QueryRowContext(ctx, `UPDATE organizations SET balance=balance+$1,display_balance=display_balance+$2,updated_at=NOW() WHERE id=$3 RETURNING balance`, actualAmount, amount, organizationID).Scan(&balance)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO organization_quota_ledger(organization_id,actor_user_id,entry_type,amount,balance_after,detail) VALUES($1,$2,'fund',$3,$4,jsonb_build_object('source','personal_balance','display_amount',$5::numeric))`, organizationID, actorUserID, actualAmount, balance, amount); err != nil {
		return nil, err
	}
	if err = insertOrganizationAudit(ctx, tx, organizationID, actorUserID, "quota.fund", "organization", fmt.Sprint(organizationID), map[string]any{"amount": actualAmount, "display_amount": amount}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetForUser(ctx, organizationID, actorUserID)
}

func (r *organizationRepository) AttachAPIKey(ctx context.Context, organizationID, actorUserID, apiKeyID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = organizationActorRole(ctx, tx, organizationID, actorUserID); err != nil {
		return err
	}
	var ownerID int64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM api_keys WHERE id=$1 AND deleted_at IS NULL`, apiKeyID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("api key not found")
	}
	if err != nil {
		return err
	}
	if ownerID != actorUserID {
		return service.ErrOrganizationForbidden
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO organization_api_keys(api_key_id,organization_id,member_user_id) VALUES($1,$2,$3)`, apiKeyID, organizationID, actorUserID)
	if err != nil {
		return err
	}
	if err = insertOrganizationAudit(ctx, tx, organizationID, actorUserID, "api_key.create", "api_key", fmt.Sprint(apiKeyID), nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *organizationRepository) SetAPIKeyBillingSource(ctx context.Context, apiKeyID, actorUserID int64, billingType string, organizationID *int64) (_ *service.APIKeyBillingSource, credential string, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()

	if billingType == service.APIKeyBillingSourcePersonal {
		var previousOrganizationID sql.NullInt64
		err = tx.QueryRowContext(ctx, `SELECT organization_id FROM organization_api_keys WHERE api_key_id=$1`, apiKeyID).Scan(&previousOrganizationID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, "", err
		}
		if previousOrganizationID.Valid {
			var lockedOrganizationID int64
			if err = tx.QueryRowContext(ctx, `SELECT id FROM organizations WHERE id=$1 FOR UPDATE`, previousOrganizationID.Int64).Scan(&lockedOrganizationID); err != nil {
				return nil, "", err
			}
			if err = insertOrganizationAudit(ctx, tx, previousOrganizationID.Int64, actorUserID, "api_key.billing_source.change", "api_key", fmt.Sprint(apiKeyID), map[string]any{"billing_type": "personal"}); err != nil {
				return nil, "", err
			}
		}
		if err = lockOwnedAPIKey(ctx, tx, apiKeyID, actorUserID, &credential); err != nil {
			return nil, "", err
		}
		if previousOrganizationID.Valid {
			if _, err = tx.ExecContext(ctx, `UPDATE organization_api_keys SET status='detached' WHERE api_key_id=$1`, apiKeyID); err != nil {
				return nil, "", err
			}
		}
		if err = tx.Commit(); err != nil {
			return nil, "", err
		}
		return &service.APIKeyBillingSource{Type: service.APIKeyBillingSourcePersonal, Status: service.APIKeyBillingSourceActive}, credential, nil
	}

	if organizationID == nil || *organizationID <= 0 {
		return nil, "", service.ErrOrganizationForbidden
	}
	if _, err = organizationActorRole(ctx, tx, *organizationID, actorUserID); err != nil {
		return nil, "", err
	}
	var organizationName string
	if err = tx.QueryRowContext(ctx, `SELECT name FROM organizations WHERE id=$1`, *organizationID).Scan(&organizationName); err != nil {
		return nil, "", err
	}
	if err = insertOrganizationAudit(ctx, tx, *organizationID, actorUserID, "api_key.billing_source.change", "api_key", fmt.Sprint(apiKeyID), map[string]any{"billing_type": "organization"}); err != nil {
		return nil, "", err
	}
	if err = lockOwnedAPIKey(ctx, tx, apiKeyID, actorUserID, &credential); err != nil {
		return nil, "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO organization_api_keys(api_key_id,organization_id,member_user_id,status,created_at)
		VALUES($1,$2,$3,'active',NOW())
		ON CONFLICT(api_key_id) DO UPDATE SET organization_id=EXCLUDED.organization_id,member_user_id=EXCLUDED.member_user_id,status='active',created_at=NOW()`, apiKeyID, *organizationID, actorUserID)
	if err != nil {
		return nil, "", err
	}
	if err = tx.Commit(); err != nil {
		return nil, "", err
	}
	organizationIDCopy := *organizationID
	return &service.APIKeyBillingSource{
		Type:             service.APIKeyBillingSourceOrganization,
		OrganizationID:   &organizationIDCopy,
		OrganizationName: organizationName,
		Status:           service.APIKeyBillingSourceActive,
	}, credential, nil
}

func lockOwnedAPIKey(ctx context.Context, tx *sql.Tx, apiKeyID, actorUserID int64, credential *string) error {
	var ownerUserID int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id,key FROM api_keys WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, apiKeyID).Scan(&ownerUserID, credential); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrAPIKeyNotFound
		}
		return err
	}
	if ownerUserID != actorUserID {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func (r *organizationRepository) GetBillingSubjectByAPIKey(ctx context.Context, apiKeyID int64) (*service.OrganizationBillingSubject, error) {
	var s service.OrganizationBillingSubject
	var orgStatus, memberStatus, keyStatus string
	err := r.db.QueryRowContext(ctx, `SELECT ok.organization_id,ok.member_user_id,o.balance,o.frozen_balance,m.monthly_limit,
		CASE WHEN m.usage_period_start < date_trunc('month',NOW()) THEN 0 ELSE m.monthly_used END,
		CASE WHEN m.usage_period_start < date_trunc('month',NOW()) THEN 0 ELSE m.monthly_frozen END,
		o.status,m.status,ok.status FROM organization_api_keys ok JOIN organizations o ON o.id=ok.organization_id JOIN organization_members m ON m.organization_id=ok.organization_id AND m.user_id=ok.member_user_id WHERE ok.api_key_id=$1 AND ok.status <> 'detached'`, apiKeyID).Scan(&s.OrganizationID, &s.MemberUserID, &s.Balance, &s.FrozenBalance, &s.MonthlyLimit, &s.MonthlyUsed, &s.MonthlyFrozen, &orgStatus, &memberStatus, &keyStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Active = orgStatus == "active" && memberStatus == "active" && keyStatus == "active"
	return &s, nil
}

func (r *organizationRepository) GetAPIKeyBillingSources(ctx context.Context, apiKeyIDs []int64) (map[int64]service.APIKeyBillingSource, error) {
	sources := make(map[int64]service.APIKeyBillingSource, len(apiKeyIDs))
	if len(apiKeyIDs) == 0 {
		return sources, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT ok.api_key_id,o.id,o.name,o.status,m.status,ok.status
		FROM organization_api_keys ok
		JOIN organizations o ON o.id=ok.organization_id
		JOIN organization_members m ON m.organization_id=ok.organization_id AND m.user_id=ok.member_user_id
		WHERE ok.api_key_id = ANY($1)`, pq.Array(apiKeyIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var apiKeyID, organizationID int64
		var name, organizationStatus, memberStatus, bindingStatus string
		if err := rows.Scan(&apiKeyID, &organizationID, &name, &organizationStatus, &memberStatus, &bindingStatus); err != nil {
			return nil, err
		}
		if bindingStatus == "detached" {
			continue
		}
		status := service.APIKeyBillingSourceInactive
		if organizationStatus == "active" && memberStatus == "active" && bindingStatus == "active" {
			status = service.APIKeyBillingSourceActive
		}
		organizationIDCopy := organizationID
		sources[apiKeyID] = service.APIKeyBillingSource{
			Type:             service.APIKeyBillingSourceOrganization,
			OrganizationID:   &organizationIDCopy,
			OrganizationName: name,
			Status:           status,
		}
	}
	return sources, rows.Err()
}

func (r *organizationRepository) UsageReport(ctx context.Context, organizationID, actorUserID int64, start, end time.Time) (*service.OrganizationUsageReport, error) {
	org, err := r.GetForUser(ctx, organizationID, actorUserID)
	if err != nil {
		return nil, err
	}
	if org.Role != service.OrganizationRoleOwner && org.Role != service.OrganizationRoleAdmin {
		return nil, service.ErrOrganizationForbidden
	}
	report := &service.OrganizationUsageReport{Members: []service.OrganizationMemberUsage{}, Models: []service.OrganizationModelUsage{}}
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(actual_cost),0),COALESCE(SUM(input_tokens),0),COALESCE(SUM(output_tokens),0),COALESCE(SUM(image_count),0),COALESCE(SUM(video_count),0) FROM usage_logs WHERE organization_id=$1 AND created_at >= $2 AND created_at < $3`, organizationID, start, end).Scan(&report.Summary.TotalRequests, &report.Summary.TotalCost, &report.Summary.InputTokens, &report.Summary.OutputTokens, &report.Summary.ImageCount, &report.Summary.VideoCount)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT u.id,u.email,u.username,COUNT(l.id),COALESCE(SUM(l.actual_cost),0),COALESCE(SUM(l.input_tokens),0),COALESCE(SUM(l.output_tokens),0) FROM organization_members m JOIN users u ON u.id=m.user_id LEFT JOIN usage_logs l ON l.organization_id=m.organization_id AND l.user_id=m.user_id AND l.created_at >= $2 AND l.created_at < $3 WHERE m.organization_id=$1 GROUP BY u.id,u.email,u.username ORDER BY COALESCE(SUM(l.actual_cost),0) DESC`, organizationID, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v service.OrganizationMemberUsage
		if err := rows.Scan(&v.UserID, &v.Email, &v.Username, &v.TotalRequests, &v.TotalCost, &v.InputTokens, &v.OutputTokens); err != nil {
			rows.Close()
			return nil, err
		}
		report.Members = append(report.Members, v)
	}
	rows.Close()
	rows, err = r.db.QueryContext(ctx, `SELECT model,COUNT(*),COALESCE(SUM(actual_cost),0),COALESCE(SUM(input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens),0) FROM usage_logs WHERE organization_id=$1 AND created_at >= $2 AND created_at < $3 GROUP BY model ORDER BY COALESCE(SUM(actual_cost),0) DESC`, organizationID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v service.OrganizationModelUsage
		if err := rows.Scan(&v.Model, &v.TotalRequests, &v.TotalCost, &v.TotalTokens); err != nil {
			return nil, err
		}
		report.Models = append(report.Models, v)
	}
	return report, rows.Err()
}

func (r *organizationRepository) ListAuditLogs(ctx context.Context, organizationID, actorUserID int64, limit int) ([]service.OrganizationAuditLog, error) {
	org, err := r.GetForUser(ctx, organizationID, actorUserID)
	if err != nil {
		return nil, err
	}
	if org.Role != service.OrganizationRoleOwner && org.Role != service.OrganizationRoleAdmin {
		return nil, service.ErrOrganizationForbidden
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,actor_user_id,action,target_type,target_id,detail,created_at FROM organization_audit_logs WHERE organization_id=$1 ORDER BY created_at DESC LIMIT $2`, organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []service.OrganizationAuditLog{}
	for rows.Next() {
		var v service.OrganizationAuditLog
		var raw []byte
		if err := rows.Scan(&v.ID, &v.ActorUserID, &v.Action, &v.TargetType, &v.TargetID, &raw, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &v.Detail)
		items = append(items, v)
	}
	return items, rows.Err()
}

func organizationActorRole(ctx context.Context, tx *sql.Tx, organizationID, userID int64) (string, error) {
	var role, status, orgStatus string
	err := tx.QueryRowContext(ctx, `SELECT m.role,m.status,o.status FROM organization_members m JOIN organizations o ON o.id=m.organization_id WHERE m.organization_id=$1 AND m.user_id=$2 FOR UPDATE OF o`, organizationID, userID).Scan(&role, &status, &orgStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return "", service.ErrOrganizationForbidden
	}
	if err != nil {
		return "", err
	}
	if status != "active" || orgStatus != "active" {
		return "", service.ErrOrganizationForbidden
	}
	return role, nil
}

func insertOrganizationAudit(ctx context.Context, tx *sql.Tx, organizationID, actorUserID int64, action, targetType, targetID string, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO organization_audit_logs(organization_id,actor_user_id,action,target_type,target_id,detail) VALUES($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6)`, organizationID, actorUserID, action, targetType, targetID, raw)
	return err
}
