package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/lotterydrawrecord"
	"github.com/Wei-Shaw/sub2api/ent/lotteryprizepoolcode"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const lotteryDrawAdvisoryLockBase int64 = 613_927_481_000_000_000

type lotteryRepository struct {
	client *dbent.Client
	db     *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewLotteryRepository(client *dbent.Client, db *sql.DB) service.LotteryRepository {
	return &lotteryRepository{client: client, db: db}
}

func (r *lotteryRepository) AddChanceGrant(ctx context.Context, userID, sourceRedeemCodeID int64, chances int) error {
	if chances <= 0 {
		return nil
	}
	execer := sqlExecer(r.db)
	if tx := dbent.TxFromContext(ctx); tx != nil {
		execer = tx
	}
	result, err := execer.ExecContext(ctx, `
INSERT INTO lottery_chance_ledger (user_id, delta, reason, source_redeem_code_id)
VALUES ($1, $2, 'redeem', $3)
ON CONFLICT (source_redeem_code_id)
WHERE source_redeem_code_id IS NOT NULL AND delta > 0
DO NOTHING
`, userID, chances, sourceRedeemCodeID)
	if err != nil {
		return fmt.Errorf("insert lottery chance grant: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read lottery chance grant result: %w", err)
	}
	if affected == 0 {
		return service.ErrLotteryChanceAlreadyGranted
	}
	return nil
}

func (r *lotteryRepository) GetAvailableChances(ctx context.Context, userID int64) (int, error) {
	var chances int64
	if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(delta), 0)
FROM lottery_chance_ledger
WHERE user_id = $1
`, userID).Scan(&chances); err != nil {
		return 0, fmt.Errorf("sum lottery chances: %w", err)
	}
	return int(chances), nil
}

func (r *lotteryRepository) CountDrawsToday(ctx context.Context, userID int64, now time.Time, loc *time.Location) (int, error) {
	start, end := lotteryDayBounds(now, loc)
	var count int
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM lottery_draw_records
WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
`, userID, start, end).Scan(&count); err != nil {
		return 0, fmt.Errorf("count lottery draws: %w", err)
	}
	return count, nil
}

func (r *lotteryRepository) DrawWithPrizeCode(ctx context.Context, userID int64, prize service.LotteryPrize, now time.Time) (_ *service.LotteryDrawRecord, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin lottery draw transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lotteryDrawAdvisoryLockBase+userID); err != nil {
		return nil, fmt.Errorf("lock lottery user draw: %w", err)
	}

	var available int64
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(delta), 0)
FROM lottery_chance_ledger
WHERE user_id = $1
`, userID).Scan(&available); err != nil {
		return nil, fmt.Errorf("sum lottery chances in transaction: %w", err)
	}
	if available <= 0 {
		return nil, service.ErrLotteryInsufficientChances
	}

	start, end := lotteryDayBounds(now, now.Location())
	var dailyUsed int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM lottery_draw_records
WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
`, userID, start, end).Scan(&dailyUsed); err != nil {
		return nil, fmt.Errorf("count lottery draws in transaction: %w", err)
	}
	if dailyUsed >= service.LotteryDailyDrawLimit {
		return nil, service.ErrLotteryDailyDrawLimit
	}

	var (
		poolCodeID   int64
		redeemCodeID int64
		code         string
	)
	err = tx.QueryRowContext(ctx, `
SELECT pool.id, pool.redeem_code_id, redeem.code
FROM lottery_prize_pool_codes AS pool
JOIN redeem_codes AS redeem ON redeem.id = pool.redeem_code_id
WHERE pool.prize_value = $1
  AND pool.status = 'available'
  AND redeem.type = $2
  AND redeem.status = $3
  AND (redeem.expires_at IS NULL OR redeem.expires_at > $4)
ORDER BY pool.id
FOR UPDATE OF pool SKIP LOCKED
LIMIT 1
`, prize.Value, service.RedeemTypeBalance, service.StatusUnused, now).Scan(&poolCodeID, &redeemCodeID, &code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrLotteryPrizeOutOfStock
	}
	if err != nil {
		return nil, fmt.Errorf("select lottery prize code: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE lottery_prize_pool_codes
SET status = 'assigned', assigned_to_user_id = $1, assigned_at = $2
WHERE id = $3 AND status = 'available'
`, userID, now, poolCodeID); err != nil {
		return nil, fmt.Errorf("assign lottery prize code: %w", err)
	}

	record := &service.LotteryDrawRecord{
		UserID:            userID,
		PrizeName:         prize.Name,
		PrizeValue:        prize.Value,
		PrizePoolCodeID:   poolCodeID,
		PrizeRedeemCodeID: redeemCodeID,
		Code:              code,
	}
	if err := tx.QueryRowContext(ctx, `
INSERT INTO lottery_draw_records (user_id, prize_name, prize_value, prize_pool_code_id, prize_redeem_code_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at
`, record.UserID, record.PrizeName, record.PrizeValue, record.PrizePoolCodeID, record.PrizeRedeemCodeID).Scan(&record.ID, &record.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrLotteryPrizeAlreadyAssigned
		}
		return nil, fmt.Errorf("create lottery draw record: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE lottery_prize_pool_codes
SET assigned_draw_record_id = $1
WHERE id = $2
`, record.ID, poolCodeID); err != nil {
		return nil, fmt.Errorf("link lottery prize code to draw: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO lottery_chance_ledger (user_id, delta, reason, draw_record_id)
VALUES ($1, -1, 'draw', $2)
`, userID, record.ID); err != nil {
		return nil, fmt.Errorf("spend lottery chance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lottery draw transaction: %w", err)
	}
	return record, nil
}

func (r *lotteryRepository) ListUserDrawRecords(ctx context.Context, userID int64, limit int) ([]service.LotteryDrawRecord, error) {
	records, err := r.client.LotteryDrawRecord.Query().
		Where(lotterydrawrecord.UserIDEQ(userID)).
		Order(dbent.Desc(lotterydrawrecord.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query lottery draw records: %w", err)
	}
	if len(records) == 0 {
		return []service.LotteryDrawRecord{}, nil
	}

	codeIDs := make([]int64, 0, len(records))
	for _, record := range records {
		codeIDs = append(codeIDs, record.PrizeRedeemCodeID)
	}
	codes, err := r.client.RedeemCode.Query().
		Where(redeemcode.IDIn(codeIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query lottery prize redeem codes: %w", err)
	}
	codeByID := make(map[int64]string, len(codes))
	for _, code := range codes {
		codeByID[code.ID] = code.Code
	}

	output := make([]service.LotteryDrawRecord, 0, len(records))
	for _, record := range records {
		output = append(output, service.LotteryDrawRecord{
			ID:                record.ID,
			UserID:            record.UserID,
			PrizeName:         record.PrizeName,
			PrizeValue:        record.PrizeValue,
			PrizePoolCodeID:   record.PrizePoolCodeID,
			PrizeRedeemCodeID: record.PrizeRedeemCodeID,
			Code:              codeByID[record.PrizeRedeemCodeID],
			CreatedAt:         record.CreatedAt,
		})
	}
	return output, nil
}

func (r *lotteryRepository) BindPrizeCodes(ctx context.Context, ids []int64) (int64, error) {
	ids = uniquePositiveIDs(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	result, err := r.db.ExecContext(ctx, `
INSERT INTO lottery_prize_pool_codes (redeem_code_id, prize_value)
SELECT id, value
FROM redeem_codes
WHERE id = ANY($1)
  AND type = $2
  AND status = $3
  AND value IN (10, 50, 100, 300)
  AND (expires_at IS NULL OR expires_at > NOW())
ON CONFLICT (redeem_code_id) DO NOTHING
`, pq.Array(ids), service.RedeemTypeBalance, service.StatusUnused)
	if err != nil {
		return 0, fmt.Errorf("bind lottery prize codes: %w", err)
	}
	bound, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read bound lottery prize codes: %w", err)
	}
	return bound, nil
}

func (r *lotteryRepository) UnbindPrizeCodes(ctx context.Context, ids []int64) (int64, error) {
	ids = uniquePositiveIDs(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	removed, err := r.client.LotteryPrizePoolCode.Delete().
		Where(
			lotteryprizepoolcode.RedeemCodeIDIn(ids...),
			lotteryprizepoolcode.StatusEQ("available"),
		).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("unbind lottery prize codes: %w", err)
	}
	return int64(removed), nil
}

func (r *lotteryRepository) GetPoolSummary(ctx context.Context) ([]service.LotteryPoolSummaryItem, error) {
	summary := make([]service.LotteryPoolSummaryItem, 0, len(service.LotteryPrizes))
	for _, prize := range service.LotteryPrizes {
		available, assigned, err := r.countPoolSummaryForPrize(ctx, prize.Value)
		if err != nil {
			return nil, err
		}
		summary = append(summary, service.LotteryPoolSummaryItem{
			PrizeName: prize.Name,
			Value:     prize.Value,
			Available: available,
			Assigned:  assigned,
		})
	}
	return summary, nil
}

func (r *lotteryRepository) countPoolSummaryForPrize(ctx context.Context, prizeValue float64) (available, assigned int64, err error) {
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM lottery_prize_pool_codes AS pool
JOIN redeem_codes AS redeem ON redeem.id = pool.redeem_code_id
WHERE pool.prize_value = $1
  AND pool.status = 'available'
  AND redeem.type = $2
  AND redeem.status = $3
  AND (redeem.expires_at IS NULL OR redeem.expires_at > NOW())
`, prizeValue, service.RedeemTypeBalance, service.StatusUnused).Scan(&available); err != nil {
		return 0, 0, fmt.Errorf("count available lottery prize codes: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM lottery_prize_pool_codes AS pool
JOIN redeem_codes AS redeem ON redeem.id = pool.redeem_code_id
WHERE pool.prize_value = $1
  AND pool.status = 'assigned'
  AND redeem.type = $2
  AND redeem.status = $3
  AND (redeem.expires_at IS NULL OR redeem.expires_at > NOW())
`, prizeValue, service.RedeemTypeBalance, service.StatusUnused).Scan(&assigned); err != nil {
		return 0, 0, fmt.Errorf("count assigned lottery prize codes: %w", err)
	}
	return available, assigned, nil
}

func lotteryDayBounds(now time.Time, loc *time.Location) (time.Time, time.Time) {
	if loc == nil {
		loc = time.Local
	}
	localNow := now.In(loc)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1)
}

func uniquePositiveIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

var _ service.LotteryRepository = (*lotteryRepository)(nil)
