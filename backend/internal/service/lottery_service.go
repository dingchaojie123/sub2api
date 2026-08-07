package service

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const LotteryDailyDrawLimit = 3

var (
	ErrLotteryInsufficientChances  = infraerrors.Conflict("LOTTERY_INSUFFICIENT_CHANCES", "insufficient lottery chances")
	ErrLotteryDailyDrawLimit       = infraerrors.Conflict("LOTTERY_DAILY_DRAW_LIMIT", "daily lottery draw limit reached")
	ErrLotteryPrizeOutOfStock      = infraerrors.Conflict("LOTTERY_PRIZE_OUT_OF_STOCK", "no eligible lottery prize code is available")
	ErrLotteryPrizeAlreadyAssigned = infraerrors.Conflict("LOTTERY_PRIZE_ALREADY_ASSIGNED", "lottery prize code is already assigned")
	ErrLotteryChanceAlreadyGranted = infraerrors.Conflict("LOTTERY_CHANCE_ALREADY_GRANTED", "lottery chances were already granted for this redeem code")
)

// LotteryPrize defines one weighted campaign prize.
type LotteryPrize struct {
	Name        string
	Value       float64
	Probability int
}

var LotteryPrizes = []LotteryPrize{
	{Name: "first", Value: 30, Probability: 1},
	{Name: "second", Value: 10, Probability: 5},
	{Name: "third", Value: 5, Probability: 20},
	{Name: "fourth", Value: 2, Probability: 74},
}

var lotteryChanceTiersByCents = map[int64]int{
	8800:   1,
	16800:  2,
	25800:  3,
	35800:  5,
	45800:  7,
	68800:  9,
	88800:  12,
	128800: 15,
}

// LotteryDrawRecord is a completed lottery draw with its assigned balance code.
type LotteryDrawRecord struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	PrizeName         string    `json:"prize_name"`
	PrizeValue        float64   `json:"value"`
	PrizePoolCodeID   int64     `json:"prize_pool_code_id"`
	PrizeRedeemCodeID int64     `json:"prize_redeem_code_id"`
	Code              string    `json:"code"`
	CreatedAt         time.Time `json:"created_at"`
}

// LotteryPoolSummaryItem reports inventory for one prize denomination.
type LotteryPoolSummaryItem struct {
	PrizeName string  `json:"prize_name"`
	Value     float64 `json:"value"`
	Available int64   `json:"available"`
	Assigned  int64   `json:"assigned"`
}

// LotteryStatus reports the caller's current ability to draw.
type LotteryStatus struct {
	AvailableChances int `json:"available_chances"`
	DailyUsed        int `json:"daily_used"`
	DailyLimit       int `json:"daily_limit"`
}

// LotteryRepository persists lottery chance, draw and prize-pool state.
type LotteryRepository interface {
	AddChanceGrant(ctx context.Context, userID int64, sourceRedeemCodeID int64, chances int) error
	GetAvailableChances(ctx context.Context, userID int64) (int, error)
	CountDrawsToday(ctx context.Context, userID int64, now time.Time, loc *time.Location) (int, error)
	DrawWithPrizeCode(ctx context.Context, userID int64, prize LotteryPrize, now time.Time) (*LotteryDrawRecord, error)
	ListUserDrawRecords(ctx context.Context, userID int64, limit int) ([]LotteryDrawRecord, error)
	BindPrizeCodes(ctx context.Context, ids []int64) (int64, error)
	UnbindPrizeCodes(ctx context.Context, ids []int64) (int64, error)
	GetPoolSummary(ctx context.Context) ([]LotteryPoolSummaryItem, error)
}

// LotteryService owns campaign rules and delegates persistent, atomic operations
// to LotteryRepository.
type LotteryService struct {
	repo     LotteryRepository
	now      func() time.Time
	location *time.Location
	drawRoll func(max int) (int, error)
}

func NewLotteryService(repo LotteryRepository) *LotteryService {
	return &LotteryService{
		repo:     repo,
		now:      time.Now,
		location: time.Local,
		drawRoll: cryptoLotteryRoll,
	}
}

// LotteryChanceGrantForBalanceValue returns the chance count for one exact
// balance redeem amount. Values are converted to cents before matching so that
// JSON float representation does not break configured currency tiers.
func LotteryChanceGrantForBalanceValue(value float64) int {
	if value <= 0 {
		return 0
	}
	if !hasAtMostTwoDecimalPlaces(value) {
		return 0
	}
	return lotteryChanceTiersByCents[int64(math.Round(value*100))]
}

func hasAtMostTwoDecimalPlaces(value float64) bool {
	formatted := strconv.FormatFloat(value, 'f', 8, 64)
	_, fraction, ok := strings.Cut(formatted, ".")
	if !ok {
		return true
	}
	return len(strings.TrimRight(fraction, "0")) <= 2
}

func (s *LotteryService) GrantChancesForRedeem(ctx context.Context, userID, redeemCodeID int64, value float64) error {
	chances := LotteryChanceGrantForBalanceValue(value)
	if chances == 0 {
		return nil
	}
	if err := s.repo.AddChanceGrant(ctx, userID, redeemCodeID, chances); err != nil {
		if errors.Is(err, ErrLotteryChanceAlreadyGranted) {
			return nil
		}
		return fmt.Errorf("add lottery chances: %w", err)
	}
	return nil
}

func (s *LotteryService) GetStatus(ctx context.Context, userID int64) (*LotteryStatus, error) {
	available, err := s.repo.GetAvailableChances(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get available lottery chances: %w", err)
	}
	dailyUsed, err := s.repo.CountDrawsToday(ctx, userID, s.now(), s.location)
	if err != nil {
		return nil, fmt.Errorf("count lottery draws today: %w", err)
	}
	return &LotteryStatus{
		AvailableChances: available,
		DailyUsed:        dailyUsed,
		DailyLimit:       LotteryDailyDrawLimit,
	}, nil
}

func (s *LotteryService) Draw(ctx context.Context, userID int64) (*LotteryDrawRecord, error) {
	status, err := s.GetStatus(ctx, userID)
	if err != nil {
		return nil, err
	}
	if status.AvailableChances <= 0 {
		return nil, ErrLotteryInsufficientChances
	}
	if status.DailyUsed >= LotteryDailyDrawLimit {
		return nil, ErrLotteryDailyDrawLimit
	}

	prize, err := s.pickPrize()
	if err != nil {
		return nil, err
	}
	draw, err := s.repo.DrawWithPrizeCode(ctx, userID, prize, s.now().In(s.location))
	if err != nil {
		return nil, err
	}
	return draw, nil
}

func (s *LotteryService) ListUserDrawRecords(ctx context.Context, userID int64, limit int) ([]LotteryDrawRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	records, err := s.repo.ListUserDrawRecords(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list lottery draw records: %w", err)
	}
	return records, nil
}

func (s *LotteryService) BindPrizeCodes(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, infraerrors.BadRequest("LOTTERY_PRIZE_CODES_REQUIRED", "prize code ids are required")
	}
	return s.repo.BindPrizeCodes(ctx, ids)
}

func (s *LotteryService) UnbindPrizeCodes(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, infraerrors.BadRequest("LOTTERY_PRIZE_CODES_REQUIRED", "prize code ids are required")
	}
	return s.repo.UnbindPrizeCodes(ctx, ids)
}

func (s *LotteryService) GetPoolSummary(ctx context.Context) ([]LotteryPoolSummaryItem, error) {
	return s.repo.GetPoolSummary(ctx)
}

func (s *LotteryService) pickPrize() (LotteryPrize, error) {
	roll, err := s.drawRoll(100)
	if err != nil {
		return LotteryPrize{}, fmt.Errorf("generate lottery roll: %w", err)
	}
	if roll < 0 || roll >= 100 {
		return LotteryPrize{}, fmt.Errorf("invalid lottery roll: %d", roll)
	}

	threshold := 0
	for _, prize := range LotteryPrizes {
		threshold += prize.Probability
		if roll < threshold {
			return prize, nil
		}
	}
	return LotteryPrize{}, errors.New("lottery prize probabilities must total 100")
}

func cryptoLotteryRoll(max int) (int, error) {
	roll, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(roll.Int64()), nil
}
