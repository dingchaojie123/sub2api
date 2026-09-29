package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageBillingRepository struct {
	db *sql.DB
}

func normalizeTaskBillingSource(source string) string {
	if strings.TrimSpace(source) == service.APIKeyBillingSourceOrganization {
		return service.APIKeyBillingSourceOrganization
	}
	return service.APIKeyBillingSourcePersonal
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	return r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint, cmd.OrganizationID, cmd.OrganizationMemberID)
}

func (r *usageBillingRepository) claimUsageBillingRequest(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64, requestFingerprint string, organizationID, memberUserID *int64) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint, organization_id, member_user_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, requestID, apiKeyID, requestFingerprint, organizationID, memberUserID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, requestID, apiKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) ReserveBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, reserveUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) CaptureBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, captureUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) ReleaseBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, releaseUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) applyBatchImageBalanceHold(
	ctx context.Context,
	cmd *service.BatchImageBalanceHoldCommand,
	apply func(context.Context, *sql.Tx, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error),
) (_ *service.BatchImageBalanceHoldResult, err error) {
	if cmd == nil {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint, cmd.OrganizationID, cmd.OrganizationMemberID)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.BatchImageBalanceHoldResult{Applied: false}, nil
	}

	result, err := apply(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &service.BatchImageBalanceHoldResult{}
	}
	result.Applied = true

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost); err != nil {
			return err
		}
	}

	if cmd.BalanceCost > 0 && cmd.OrganizationID != nil && cmd.OrganizationMemberID != nil {
		newBalance, sufficient, err := deductUsageBillingOrganization(ctx, tx, cmd, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewOrganizationBalance = &newBalance
		result.BalanceOverdrafted = !sufficient
	} else if cmd.BalanceCost > 0 {
		newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		result.BalanceOverdrafted = !sufficient
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	return nil
}

func deductUsageBillingOrganization(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, amount float64) (float64, bool, error) {
	organizationID, memberUserID := *cmd.OrganizationID, *cmd.OrganizationMemberID
	if _, err := tx.ExecContext(ctx, `UPDATE organization_members SET monthly_used=0,usage_period_start=date_trunc('month',NOW()),updated_at=NOW() WHERE organization_id=$1 AND user_id=$2 AND usage_period_start < date_trunc('month',NOW())`, organizationID, memberUserID); err != nil {
		return 0, false, err
	}
	// The organization and member are an immutable request-time billing snapshot.
	// Do not re-check the current key binding or membership status here: either may
	// have changed after the upstream accepted the request.
	var balance float64
	err := tx.QueryRowContext(ctx, `UPDATE organizations SET balance=balance-$1,
		display_balance=CASE WHEN COALESCE(display_balance,0)-($1*CASE WHEN balance>0 THEN COALESCE(display_balance,0)/balance ELSE 1 END)>0
			THEN COALESCE(display_balance,0)-($1*CASE WHEN balance>0 THEN COALESCE(display_balance,0)/balance ELSE 1 END) ELSE 0 END,
		updated_at=NOW() WHERE id=$2 AND balance >= $1 RETURNING balance`, amount, organizationID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `UPDATE organizations SET balance=balance-$1,
			display_balance=CASE WHEN COALESCE(display_balance,0)-($1*CASE WHEN balance>0 THEN COALESCE(display_balance,0)/balance ELSE 1 END)>0
				THEN COALESCE(display_balance,0)-($1*CASE WHEN balance>0 THEN COALESCE(display_balance,0)/balance ELSE 1 END) ELSE 0 END,
			updated_at=NOW() WHERE id=$2 RETURNING balance`, amount, organizationID).Scan(&balance)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, service.ErrOrganizationNotFound
		}
		if err != nil {
			return 0, false, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE organization_members SET monthly_used=monthly_used+$1,updated_at=NOW() WHERE organization_id=$2 AND user_id=$3`, amount, organizationID, memberUserID); err != nil {
			return 0, false, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO organization_quota_ledger(organization_id,member_user_id,api_key_id,request_id,entry_type,amount,balance_after,detail) VALUES($1,$2,$3,$4,'usage',$5,$6,jsonb_build_object('model',$7::text,'overdraft',true))`, organizationID, memberUserID, cmd.APIKeyID, cmd.RequestID, -amount, balance, cmd.Model)
		return balance, false, err
	}
	if err != nil {
		return 0, false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_members SET monthly_used=monthly_used+$1,updated_at=NOW() WHERE organization_id=$2 AND user_id=$3`, amount, organizationID, memberUserID); err != nil {
		return 0, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO organization_quota_ledger(organization_id,member_user_id,api_key_id,request_id,entry_type,amount,balance_after,detail) VALUES($1,$2,$3,$4,'usage',$5,$6,jsonb_build_object('model',$7::text))`, organizationID, memberUserID, cmd.APIKeyID, cmd.RequestID, -amount, balance, cmd.Model)
	return balance, true, err
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, error) {
	var newBalance float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			display_balance = CASE
				WHEN COALESCE(display_balance, 0) - ($1 * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END) > 0
					THEN COALESCE(display_balance, 0) - ($1 * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END)
				ELSE 0
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if err == nil {
		return newBalance, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}

	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			display_balance = CASE
				WHEN COALESCE(display_balance, 0) - ($1 * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END) > 0
					THEN COALESCE(display_balance, 0) - ($1 * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END)
				ELSE 0
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, service.ErrUserNotFound
	}
	if err != nil {
		return 0, false, err
	}
	return newBalance, false, nil
}

func reserveUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if organizationID, memberUserID, ok, err := batchImageOrganizationBillingOwner(cmd); err != nil {
		return nil, err
	} else if ok {
		return reserveOrganizationBalance(ctx, tx, organizationID, memberUserID, cmd)
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			display_balance = CASE
				WHEN COALESCE(display_balance, 0) - ($1 * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END) > 0
					THEN COALESCE(display_balance, 0) - ($1 * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END)
				ELSE 0
			END,
			frozen_balance = COALESCE(frozen_balance, 0) + $1,
			frozen_display_balance = COALESCE(frozen_display_balance, 0) + ($1 * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END),
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, service.ErrBatchImageInsufficientBalance
}

func captureUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 && cmd.ActualAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if cmd.ActualAmount-cmd.HoldAmount > 0.00000001 {
		return nil, service.ErrBatchImageSettlementCostExceedsHold
	}
	if organizationID, memberUserID, ok, err := batchImageOrganizationBillingOwner(cmd); err != nil {
		return nil, err
	} else if ok {
		return captureOrganizationBalance(ctx, tx, organizationID, memberUserID, cmd)
	}
	holdRequestID := strings.TrimSpace(cmd.HoldRequestID)
	if holdRequestID == "" {
		holdRequestID = service.BatchImageHoldRequestID(cmd.BatchID)
	}
	held, heldErr := batchImageHoldClaimExists(ctx, tx, holdRequestID, cmd.APIKeyID)
	if heldErr != nil {
		return nil, heldErr
	}
	if !held {
		logger.LegacyPrintf("repository.usage_billing", "[BatchImage] capture failed, hold was never reserved: batch=%s", cmd.BatchID)
		return nil, service.ErrBatchImageHoldNotReserved
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance
				+ CASE WHEN $1 > $2 THEN $1 - $2 ELSE 0 END
				- CASE WHEN $2 > $1 THEN $2 - $1 ELSE 0 END,
			display_balance = CASE
				WHEN $1 > $2 THEN COALESCE(display_balance, 0)
					+ (CASE WHEN $1 > 0 THEN ($1 - $2) / $1 ELSE 0 END)
						* (CASE WHEN COALESCE(frozen_balance, 0) > 0 THEN COALESCE(frozen_display_balance, 0) * ($1 / frozen_balance) ELSE 0 END)
				WHEN $2 > $1 THEN CASE
					WHEN COALESCE(display_balance, 0) - (($2 - $1) * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END) > 0
						THEN COALESCE(display_balance, 0) - (($2 - $1) * CASE WHEN balance > 0 THEN COALESCE(display_balance, 0) / balance ELSE 1 END)
					ELSE 0
				END
				ELSE COALESCE(display_balance, 0)
			END,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			frozen_display_balance = CASE
				WHEN COALESCE(frozen_display_balance, 0) - (CASE WHEN COALESCE(frozen_balance, 0) > 0 THEN COALESCE(frozen_display_balance, 0) * ($1 / frozen_balance) ELSE 0 END) > 0
					THEN COALESCE(frozen_display_balance, 0) - (CASE WHEN COALESCE(frozen_balance, 0) > 0 THEN COALESCE(frozen_display_balance, 0) * ($1 / frozen_balance) ELSE 0 END)
				ELSE 0
			END,
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.ActualAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func releaseUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if organizationID, memberUserID, ok, err := batchImageOrganizationBillingOwner(cmd); err != nil {
		return nil, err
	} else if ok {
		return releaseOrganizationBalance(ctx, tx, organizationID, memberUserID, cmd)
	}
	// 释放前校验该 job 确实预留过 hold（hold request id 已被 claim），
	// 防止从未成功冻结的 job 触发"幻影释放"，从其他用户的冻结资金池中凭空生成余额。
	holdRequestID := strings.TrimSpace(cmd.HoldRequestID)
	if holdRequestID == "" {
		holdRequestID = service.BatchImageHoldRequestID(cmd.BatchID)
	}
	held, heldErr := batchImageHoldClaimExists(ctx, tx, holdRequestID, cmd.APIKeyID)
	if heldErr != nil {
		return nil, heldErr
	}
	if !held {
		logger.LegacyPrintf("repository.usage_billing", "[BatchImage] release skipped, hold was never reserved: batch=%s", cmd.BatchID)
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1,
			display_balance = COALESCE(display_balance, 0)
				+ (CASE WHEN COALESCE(frozen_balance, 0) > 0 THEN COALESCE(frozen_display_balance, 0) * ($1 / frozen_balance) ELSE $1 END),
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			frozen_display_balance = CASE
				WHEN COALESCE(frozen_display_balance, 0) - (CASE WHEN COALESCE(frozen_balance, 0) > 0 THEN COALESCE(frozen_display_balance, 0) * ($1 / frozen_balance) ELSE 0 END) > 0
					THEN COALESCE(frozen_display_balance, 0) - (CASE WHEN COALESCE(frozen_balance, 0) > 0 THEN COALESCE(frozen_display_balance, 0) * ($1 / frozen_balance) ELSE 0 END)
				ELSE 0
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func batchImageOrganizationBillingOwner(cmd *service.BatchImageBalanceHoldCommand) (int64, int64, bool, error) {
	if cmd == nil || strings.TrimSpace(cmd.BillingSource) != service.APIKeyBillingSourceOrganization {
		return 0, 0, false, nil
	}
	if cmd.OrganizationID == nil || cmd.OrganizationMemberID == nil || *cmd.OrganizationID <= 0 || *cmd.OrganizationMemberID <= 0 {
		return 0, 0, false, service.ErrOrganizationForbidden
	}
	return *cmd.OrganizationID, *cmd.OrganizationMemberID, true, nil
}

func reserveOrganizationBalance(ctx context.Context, tx *sql.Tx, organizationID, memberUserID int64, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE organization_members SET monthly_used=0,monthly_frozen=0,usage_period_start=date_trunc('month',NOW()),updated_at=NOW() WHERE organization_id=$1 AND user_id=$2 AND usage_period_start < date_trunc('month',NOW())`, organizationID, memberUserID); err != nil {
		return nil, err
	}
	var limit, used, frozen float64
	var memberStatus, orgStatus, keyStatus string
	err := tx.QueryRowContext(ctx, `SELECT m.monthly_limit,m.monthly_used,m.monthly_frozen,m.status,o.status,ok.status FROM organization_members m JOIN organizations o ON o.id=m.organization_id JOIN organization_api_keys ok ON ok.organization_id=m.organization_id AND ok.member_user_id=m.user_id WHERE m.organization_id=$1 AND m.user_id=$2 AND ok.api_key_id=$3 FOR UPDATE OF m,o,ok`, organizationID, memberUserID, cmd.APIKeyID).Scan(&limit, &used, &frozen, &memberStatus, &orgStatus, &keyStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrOrganizationForbidden
	}
	if err != nil {
		return nil, err
	}
	if memberStatus != "active" || orgStatus != "active" || keyStatus != "active" {
		return nil, service.ErrOrganizationForbidden
	}
	if limit > 0 && used+frozen+cmd.HoldAmount > limit {
		return nil, service.ErrBatchImageInsufficientBalance
	}
	var balance, orgFrozen float64
	err = tx.QueryRowContext(ctx, `UPDATE organizations SET balance=balance-$1,
		display_balance=CASE WHEN COALESCE(display_balance,0)-($1*CASE WHEN balance>0 THEN COALESCE(display_balance,0)/balance ELSE 1 END)>0
			THEN COALESCE(display_balance,0)-($1*CASE WHEN balance>0 THEN COALESCE(display_balance,0)/balance ELSE 1 END) ELSE 0 END,
		frozen_balance=frozen_balance+$1,
		frozen_display_balance=COALESCE(frozen_display_balance,0)+($1*CASE WHEN balance>0 THEN COALESCE(display_balance,0)/balance ELSE 1 END),
		updated_at=NOW() WHERE id=$2 AND balance >= $1 RETURNING balance,frozen_balance`, cmd.HoldAmount, organizationID).Scan(&balance, &orgFrozen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrBatchImageInsufficientBalance
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_members SET monthly_frozen=monthly_frozen+$1,updated_at=NOW() WHERE organization_id=$2 AND user_id=$3`, cmd.HoldAmount, organizationID, memberUserID); err != nil {
		return nil, err
	}
	return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &orgFrozen}, nil
}

func captureOrganizationBalance(ctx context.Context, tx *sql.Tx, organizationID, memberUserID int64, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.ActualAmount-cmd.HoldAmount > 0.00000001 {
		return nil, service.ErrBatchImageSettlementCostExceedsHold
	}
	holdID := strings.TrimSpace(cmd.HoldRequestID)
	if holdID == "" {
		holdID = service.BatchImageHoldRequestID(cmd.BatchID)
	}
	held, err := batchImageHoldClaimExists(ctx, tx, holdID, cmd.APIKeyID)
	if err != nil {
		return nil, err
	}
	if !held {
		return nil, service.ErrBatchImageHoldNotReserved
	}
	var balance, frozen float64
	err = tx.QueryRowContext(ctx, `UPDATE organizations SET
		balance=balance+($1-$2),
		display_balance=COALESCE(display_balance,0)+(CASE WHEN $1>0 THEN ($1-$2)/$1 ELSE 0 END)*(CASE WHEN frozen_balance>0 THEN COALESCE(frozen_display_balance,0)*($1/frozen_balance) ELSE 0 END),
		frozen_balance=frozen_balance-$1,
		frozen_display_balance=GREATEST(0,COALESCE(frozen_display_balance,0)-(CASE WHEN frozen_balance>0 THEN COALESCE(frozen_display_balance,0)*($1/frozen_balance) ELSE 0 END)),
		updated_at=NOW() WHERE id=$3 AND frozen_balance >= $1 RETURNING balance,frozen_balance`, cmd.HoldAmount, cmd.ActualAmount, organizationID).Scan(&balance, &frozen)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_members SET monthly_frozen=GREATEST(0,monthly_frozen-$1),monthly_used=monthly_used+$2,updated_at=NOW() WHERE organization_id=$3 AND user_id=$4`, cmd.HoldAmount, cmd.ActualAmount, organizationID, memberUserID); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO organization_quota_ledger(organization_id,member_user_id,api_key_id,request_id,entry_type,amount,balance_after,detail) VALUES($1,$2,$3,$4,'usage',$5,$6,jsonb_build_object('media','async'))`, organizationID, memberUserID, cmd.APIKeyID, cmd.RequestID, -cmd.ActualAmount, balance)
	if err != nil {
		return nil, err
	}
	return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
}

func releaseOrganizationBalance(ctx context.Context, tx *sql.Tx, organizationID, memberUserID int64, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	holdID := strings.TrimSpace(cmd.HoldRequestID)
	if holdID == "" {
		holdID = service.BatchImageHoldRequestID(cmd.BatchID)
	}
	held, err := batchImageHoldClaimExists(ctx, tx, holdID, cmd.APIKeyID)
	if err != nil {
		return nil, err
	}
	if !held {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err = tx.QueryRowContext(ctx, `UPDATE organizations SET
		balance=balance+$1,
		display_balance=COALESCE(display_balance,0)+(CASE WHEN frozen_balance>0 THEN COALESCE(frozen_display_balance,0)*($1/frozen_balance) ELSE $1 END),
		frozen_balance=frozen_balance-$1,
		frozen_display_balance=GREATEST(0,COALESCE(frozen_display_balance,0)-(CASE WHEN frozen_balance>0 THEN COALESCE(frozen_display_balance,0)*($1/frozen_balance) ELSE 0 END)),
		updated_at=NOW() WHERE id=$2 AND frozen_balance >= $1 RETURNING balance,frozen_balance`, cmd.HoldAmount, organizationID).Scan(&balance, &frozen)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE organization_members SET monthly_frozen=GREATEST(0,monthly_frozen-$1),updated_at=NOW() WHERE organization_id=$2 AND user_id=$3`, cmd.HoldAmount, organizationID, memberUserID); err != nil {
		return nil, err
	}
	return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
}

// batchImageHoldClaimExists 检查 hold request id 是否已在 dedup（或归档）表中被 claim，
// 即该 batch 的冻结操作确实成功提交过。
func batchImageHoldClaimExists(ctx context.Context, tx *sql.Tx, holdRequestID string, apiKeyID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func userExistsForBilling(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN `+dailyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN `+dailyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_daily_reset_at', `+nextDailyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_weekly_reset_at', `+nextWeeklyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	// 必须在执行下一条 SQL 前显式关闭 rows：pq 驱动在同一连接上
	// 不允许前一条查询的结果集未耗尽时启动新查询，否则会返回
	// "unexpected Parse response" 错误。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 任意维度额度在本次递增中从"未超"跨越到"已超"时，必须刷新调度快照，
	// 否则 Redis 中缓存的 Account 仍显示旧的 used 值，后续请求会继续选中本账号，
	// 最终观察到 daily_used / weekly_used 大幅超过配置的 limit。
	// 对于日/周额度，即使本次触发了周期重置（pre=0、post=amount），
	// 判定式 (post-amount) < limit 同样成立，逻辑与总额度保持一致。
	crossedTotal := state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit
	crossedDaily := state.DailyLimit > 0 && state.DailyUsed >= state.DailyLimit && (state.DailyUsed-amount) < state.DailyLimit
	crossedWeekly := state.WeeklyLimit > 0 && state.WeeklyUsed >= state.WeeklyLimit && (state.WeeklyUsed-amount) < state.WeeklyLimit
	if crossedTotal || crossedDaily || crossedWeekly {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
