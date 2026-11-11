package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseRepository struct{ db *sql.DB }

func NewEnterpriseRepository(db *sql.DB) service.EnterpriseRepository {
	return &enterpriseRepository{db: db}
}

const enterpriseColumns = `user_id,company_name,credit_code,address,contact_name,contact_phone,verify_status,reject_reason,reviewed_by,reviewed_at,created_at,updated_at`

type enterpriseScanner interface {
	Scan(dest ...any) error
}

func scanEnterpriseProfile(row enterpriseScanner, p *service.EnterpriseProfile) error {
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	if err := row.Scan(&p.UserID, &p.CompanyName, &p.CreditCode, &p.Address, &p.ContactName, &p.ContactPhone,
		&p.Status, &p.RejectReason, &reviewedBy, &reviewedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	if reviewedBy.Valid {
		v := reviewedBy.Int64
		p.ReviewedBy = &v
	}
	if reviewedAt.Valid {
		v := reviewedAt.Time
		p.ReviewedAt = &v
	}
	return nil
}

// mapEnterpriseUniqueViolation 把唯一约束冲突翻译为业务错误。
func mapEnterpriseUniqueViolation(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		if strings.Contains(pqErr.Constraint, "credit_code") {
			return service.ErrEnterpriseCreditCodeExists
		}
		return service.ErrEnterpriseAlreadyRegistered
	}
	return err
}

func (r *enterpriseRepository) Get(ctx context.Context, userID int64) (*service.EnterpriseProfile, error) {
	var p service.EnterpriseProfile
	err := scanEnterpriseProfile(r.db.QueryRowContext(ctx,
		`SELECT `+enterpriseColumns+` FROM enterprise_profiles WHERE user_id=$1`, userID), &p)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *enterpriseRepository) Create(ctx context.Context, userID int64, in service.EnterpriseInput) (_ *service.EnterpriseProfile, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 已被删除账号占用的信用代码不应阻止公司重新注册：先清理其遗留档案。
	if _, err = tx.ExecContext(ctx, `DELETE FROM enterprise_profiles ep USING users u
		WHERE ep.user_id=u.id AND u.deleted_at IS NOT NULL AND ep.credit_code=$1`, in.CreditCode); err != nil {
		return nil, err
	}
	var p service.EnterpriseProfile
	err = scanEnterpriseProfile(tx.QueryRowContext(ctx, `INSERT INTO enterprise_profiles
		(user_id,company_name,credit_code,address,contact_name,contact_phone,verify_status)
		VALUES($1,$2,$3,$4,$5,$6,'pending') RETURNING `+enterpriseColumns,
		userID, in.CompanyName, in.CreditCode, in.Address, in.ContactName, in.ContactPhone), &p)
	if err != nil {
		return nil, mapEnterpriseUniqueViolation(err)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *enterpriseRepository) Update(ctx context.Context, userID int64, in service.EnterpriseInput, status string) (*service.EnterpriseProfile, error) {
	var p service.EnterpriseProfile
	// 重新提交（状态回到 pending）时清空驳回信息与审核人。
	err := scanEnterpriseProfile(r.db.QueryRowContext(ctx, `UPDATE enterprise_profiles SET
		company_name=$2, credit_code=$3, address=$4, contact_name=$5, contact_phone=$6,
		reject_reason=CASE WHEN $7::text='pending' THEN '' ELSE reject_reason END,
		reviewed_by=CASE WHEN $7::text='pending' THEN NULL ELSE reviewed_by END,
		reviewed_at=CASE WHEN $7::text='pending' THEN NULL ELSE reviewed_at END,
		verify_status=$7::text, updated_at=NOW()
		WHERE user_id=$1 RETURNING `+enterpriseColumns,
		userID, in.CompanyName, in.CreditCode, in.Address, in.ContactName, in.ContactPhone, status), &p)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseNotFound
	}
	if err != nil {
		return nil, mapEnterpriseUniqueViolation(err)
	}
	return &p, nil
}

func (r *enterpriseRepository) CreditCodeExists(ctx context.Context, creditCode string, excludeUserID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM enterprise_profiles ep
		JOIN users u ON u.id=ep.user_id AND u.deleted_at IS NULL
		WHERE ep.credit_code=$1 AND ep.user_id<>$2)`, creditCode, excludeUserID).Scan(&exists)
	return exists, err
}

func (r *enterpriseRepository) HasActiveMembership(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM organization_members m
		JOIN organizations o ON o.id=m.organization_id
		WHERE m.user_id=$1 AND m.status='active' AND o.status='active')`, userID).Scan(&exists)
	return exists, err
}

func (r *enterpriseRepository) AdminList(ctx context.Context, filter service.EnterpriseAdminFilter) ([]service.EnterpriseAdminItem, int64, error) {
	where := []string{"u.deleted_at IS NULL"}
	args := []any{}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, "ep.verify_status=$"+itoa(len(args)))
	}
	if s := strings.TrimSpace(filter.Search); s != "" {
		args = append(args, "%"+escapeLike(s)+"%")
		n := "$" + itoa(len(args))
		where = append(where, "(ep.company_name ILIKE "+n+" OR ep.credit_code ILIKE "+n+" OR ep.contact_name ILIKE "+n+" OR u.email ILIKE "+n+")")
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enterprise_profiles ep JOIN users u ON u.id=ep.user_id WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit, offset := filter.Limit, filter.Offset
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, `SELECT ep.user_id,ep.company_name,ep.credit_code,ep.address,ep.contact_name,ep.contact_phone,
		ep.verify_status,ep.reject_reason,ep.reviewed_by,ep.reviewed_at,ep.created_at,ep.updated_at,u.email,u.username
		FROM enterprise_profiles ep JOIN users u ON u.id=ep.user_id
		WHERE `+cond+` ORDER BY
		CASE ep.verify_status WHEN 'pending' THEN 0 WHEN 'incomplete' THEN 1 WHEN 'rejected' THEN 2 ELSE 3 END,
		ep.created_at DESC, ep.user_id DESC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.EnterpriseAdminItem, 0)
	for rows.Next() {
		var it service.EnterpriseAdminItem
		var reviewedBy sql.NullInt64
		var reviewedAt sql.NullTime
		if err := rows.Scan(&it.UserID, &it.CompanyName, &it.CreditCode, &it.Address, &it.ContactName, &it.ContactPhone,
			&it.Status, &it.RejectReason, &reviewedBy, &reviewedAt, &it.CreatedAt, &it.UpdatedAt, &it.Email, &it.Username); err != nil {
			return nil, 0, err
		}
		if reviewedBy.Valid {
			v := reviewedBy.Int64
			it.ReviewedBy = &v
		}
		if reviewedAt.Valid {
			v := reviewedAt.Time
			it.ReviewedAt = &v
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

func (r *enterpriseRepository) Review(ctx context.Context, userID, reviewerID int64, status, reason string) (*service.EnterpriseProfile, error) {
	var p service.EnterpriseProfile
	err := scanEnterpriseProfile(r.db.QueryRowContext(ctx, `UPDATE enterprise_profiles SET
		verify_status=$2, reject_reason=$3, reviewed_by=$4, reviewed_at=NOW(), updated_at=NOW()
		WHERE user_id=$1 RETURNING `+enterpriseColumns, userID, status, reason, reviewerID), &p)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
