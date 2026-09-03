package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrPPVideoBillingHoldFailed       = infraerrors.New(502, "PP_VIDEO_BILLING_HOLD_FAILED", "PP video balance hold failed")
	ErrPPVideoSettlementBillingFailed = infraerrors.New(502, "PP_VIDEO_SETTLEMENT_BILLING_FAILED", "PP video settlement billing failed")
	ErrPPVideoInsufficientBalance     = infraerrors.New(402, "PP_VIDEO_INSUFFICIENT_BALANCE", "insufficient balance for PP video generation")
)

type PPVideoBillingQuote struct {
	Formula   string
	Units     float64
	UnitPrice float64
	Cost      float64
}

func (s *OpenAIGatewayService) calculatePPVideoGroupVideoCost(
	meta PPVideoBillingMetadata,
	apiKey *APIKey,
) (PPVideoBillingQuote, error) {
	billingService := (*BillingService)(nil)
	if s != nil {
		billingService = s.billingService
	}
	if billingService == nil {
		billingService = NewBillingService(nil, nil)
	}

	durationMs := meta.RequestedDurationMilliseconds
	if durationMs <= 0 {
		durationMs = int64(meta.RequestedDurationSeconds) * 1000
	}
	if durationMs <= 0 || meta.VideoCount <= 0 {
		return PPVideoBillingQuote{}, fmt.Errorf("PP video duration and count must be positive")
	}

	resolution := ppVideoGroupBillingResolution(meta)
	groupConfig := videoPriceConfigFromAPIKey(apiKey)
	unitPrice := billingService.getVideoUnitPrice(meta.Model, resolution, groupConfig)
	units := float64(durationMs) / 1000 * float64(meta.VideoCount)
	return PPVideoBillingQuote{
		Formula:   PPVideoBillingFormulaPerSecond,
		Units:     units,
		UnitPrice: unitPrice,
		Cost:      units * unitPrice,
	}, nil
}

func ppVideoGroupBillingResolution(meta PPVideoBillingMetadata) string {
	rawResolution := strings.ToLower(strings.TrimSpace(meta.VideoResolution))
	resolution := normalizePPVideoResolution(rawResolution)
	if meta.Platform == PlatformSeedance && rawResolution == "4k" {
		resolution = VideoBillingResolution1080P
	}
	if meta.Platform == PlatformKling {
		switch strings.ToLower(strings.TrimSpace(meta.KlingMode)) {
		case "", "std", "2x":
			resolution = VideoBillingResolution720P
		case "pro", "2x_pro", "2x-pro":
			resolution = VideoBillingResolution1080P
		case "4k":
			// The existing group rate card has no 4K tier. Use its highest
			// configured video tier instead of introducing a second PP-only rate card.
			resolution = VideoBillingResolution1080P
		}
	}
	if resolution == "" {
		resolution = SiteVideoDefaultResolution
	}
	return NormalizeVideoBillingResolutionOrDefault(resolution)
}

type PPVideoSettlementInput struct {
	Task               *PPVideoTask
	FinalStatus        string
	APIKey             *APIKey
	User               *User
	Account            *Account
	Subscription       *UserSubscription
	InboundEndpoint    string
	UpstreamEndpoint   string
	RequestPayloadHash string
	APIKeyService      APIKeyQuotaUpdater
	QuotaPlatform      string
}

func (s *OpenAIGatewayService) CalculatePPVideoCost(
	ctx context.Context,
	apiKey *APIKey,
	user *User,
	account *Account,
	meta PPVideoBillingMetadata,
) (*CostBreakdown, error) {
	if apiKey == nil || apiKey.Group == nil || account == nil {
		return nil, fmt.Errorf("PP video billing requires API key, group and account")
	}
	if meta.ValidationError != nil {
		return nil, meta.ValidationError
	}
	billingAPIKey := apiKey
	if s != nil {
		billingAPIKey = s.apiKeyWithFreshGroupMediaPricing(ctx, apiKey)
	}
	quote, err := s.calculatePPVideoGroupVideoCost(meta, billingAPIKey)
	if err != nil {
		return nil, err
	}
	baseMultiplier := 1.0
	if s != nil && user != nil && apiKey.GroupID != nil {
		baseMultiplier = s.ResolveUserGroupRateMultiplier(ctx, user.ID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	} else if apiKey.Group.RateMultiplier > 0 {
		baseMultiplier = apiKey.Group.RateMultiplier
	}
	videoMultiplier := resolveVideoRateMultiplier(apiKey, baseMultiplier)
	accountMultiplier := account.BillingRateMultiplier()
	return &CostBreakdown{
		TotalCost:        quote.Cost,
		OutputCost:       quote.Cost,
		ActualCost:       quote.Cost * videoMultiplier * accountMultiplier,
		BillingMode:      string(BillingModeVideo),
		BillingFormula:   quote.Formula,
		BillingUnits:     quote.Units,
		BillingUnitPrice: quote.UnitPrice,
	}, nil
}

func (s *OpenAIGatewayService) ReservePPVideoBalanceHold(ctx context.Context, task *PPVideoTask, payloadHash string) error {
	if s == nil || s.usageBillingRepo == nil || task == nil || task.HoldAmount <= 0 {
		return nil
	}
	_, err := s.usageBillingRepo.ReserveBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          task.HoldID,
		APIKeyID:           task.APIKeyID,
		UserID:             task.UserID,
		BatchID:            task.LocalTaskID,
		HoldRequestID:      task.HoldID,
		HoldAmount:         task.HoldAmount,
		RequestPayloadHash: strings.TrimSpace(payloadHash),
	})
	if err != nil {
		if errors.Is(err, ErrBatchImageInsufficientBalance) {
			return ErrPPVideoInsufficientBalance
		}
		return ErrPPVideoBillingHoldFailed.WithCause(err)
	}
	return nil
}

func (s *OpenAIGatewayService) CapturePPVideoBalanceHold(ctx context.Context, task *PPVideoTask, actualAmount float64, payloadHash string) error {
	if s == nil || s.usageBillingRepo == nil || task == nil || (task.HoldAmount <= 0 && actualAmount <= 0) {
		return nil
	}
	_, err := s.usageBillingRepo.CaptureBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          task.CaptureID,
		APIKeyID:           task.APIKeyID,
		UserID:             task.UserID,
		BatchID:            task.LocalTaskID,
		HoldRequestID:      task.HoldID,
		HoldAmount:         task.HoldAmount,
		ActualAmount:       actualAmount,
		RequestPayloadHash: strings.TrimSpace(payloadHash),
	})
	if err != nil {
		return ErrPPVideoSettlementBillingFailed.WithCause(err)
	}
	return nil
}

func (s *OpenAIGatewayService) ReleasePPVideoBalanceHold(ctx context.Context, task *PPVideoTask, payloadHash string) error {
	if s == nil || s.usageBillingRepo == nil || task == nil || task.HoldAmount <= 0 {
		return nil
	}
	_, err := s.usageBillingRepo.ReleaseBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          task.ReleaseID,
		APIKeyID:           task.APIKeyID,
		UserID:             task.UserID,
		BatchID:            task.LocalTaskID,
		HoldRequestID:      task.HoldID,
		HoldAmount:         task.HoldAmount,
		RequestPayloadHash: strings.TrimSpace(payloadHash),
	})
	if errors.Is(err, ErrUsageBillingRequestConflict) {
		return nil
	}
	if err != nil {
		return ErrPPVideoSettlementBillingFailed.WithCause(err)
	}
	return nil
}

func (s *OpenAIGatewayService) SettlePPVideoTask(ctx context.Context, in *PPVideoSettlementInput) error {
	if in == nil || in.Task == nil {
		return ErrPPVideoTaskNotFound
	}
	finalStatus := NormalizePPVideoTaskStatus(in.FinalStatus)
	if finalStatus != PPVideoTaskStatusSucceeded && finalStatus != PPVideoTaskStatusFailed {
		return nil
	}
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return err
	}
	task, claimed, err := repo.ClaimPPVideoTaskSettlement(ctx, ClaimPPVideoTaskSettlementParams{
		TaskID:      in.Task.TaskID,
		FinalStatus: finalStatus,
	})
	if err != nil {
		return err
	}
	if !claimed {
		// A previous worker may have completed the compare-and-set transition
		// and failed during the downstream accounting step. The polling query
		// intentionally returns settling rows so the idempotent capture/apply
		// operations can be retried.
		switch strings.TrimSpace(in.Task.BillingStatus) {
		case PPVideoBillingStatusSettling, PPVideoBillingStatusSettlingNone:
			task = in.Task
		default:
			return nil
		}
	}
	if task == nil {
		return nil
	}
	settlement := *in
	settlement.Task = task
	settlement.FinalStatus = NormalizePPVideoTaskStatus(task.Status)
	if settlement.FinalStatus == "" {
		settlement.FinalStatus = finalStatus
	}
	if settlement.FinalStatus != PPVideoTaskStatusSucceeded && settlement.FinalStatus != PPVideoTaskStatusFailed {
		return nil
	}
	if settlement.FinalStatus == PPVideoTaskStatusSucceeded {
		if err := validatePPVideoSuccessfulSettlement(&settlement); err != nil {
			return err
		}
		return s.captureAndRecordPPVideoTask(ctx, &settlement)
	}
	return s.releaseAndMarkPPVideoTask(ctx, &settlement)
}

func validatePPVideoSuccessfulSettlement(in *PPVideoSettlementInput) error {
	if in == nil || in.Task == nil {
		return ErrPPVideoTaskNotFound
	}
	if in.APIKey == nil || in.Account == nil {
		return ErrPPVideoSettlementBillingFailed.WithCause(errors.New("PP video settlement missing api key or account"))
	}
	if isPPVideoSubscriptionTask(in.Task) && in.Subscription == nil {
		return ErrPPVideoSettlementBillingFailed.WithCause(errors.New("PP video subscription settlement missing subscription"))
	}
	return nil
}

func (s *OpenAIGatewayService) captureAndRecordPPVideoTask(ctx context.Context, in *PPVideoSettlementInput) error {
	task := in.Task
	if task.BillingStatus == PPVideoBillingStatusCaptured || task.BillingStatus == PPVideoBillingStatusReleased {
		return nil
	}
	actualCost := ppVideoActualSettlementCost(task)
	isSubscription := isPPVideoSubscriptionTask(task)
	if !isSubscription && task.HoldAmount > 0 &&
		(task.BillingStatus == PPVideoBillingStatusHeld || task.BillingStatus == PPVideoBillingStatusSettling) {
		if err := s.CapturePPVideoBalanceHold(ctx, task, ppVideoBalanceCaptureAmount(task, actualCost), in.RequestPayloadHash); err != nil {
			return err
		}
	}
	if err := s.recordPPVideoUsage(ctx, in, actualCost, isSubscription); err != nil {
		return ErrPPVideoSettlementBillingFailed.WithCause(err)
	}
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return err
	}
	_, err = repo.MarkPPVideoTaskSettled(ctx, MarkPPVideoTaskSettledParams{
		TaskID: task.TaskID, Status: PPVideoTaskStatusSucceeded,
		BillingStatus: PPVideoBillingStatusCaptured, ActualCost: actualCost,
	})
	return err
}

func (s *OpenAIGatewayService) releaseAndMarkPPVideoTask(ctx context.Context, in *PPVideoSettlementInput) error {
	task := in.Task
	if task.BillingStatus == PPVideoBillingStatusCaptured || task.BillingStatus == PPVideoBillingStatusReleased {
		return nil
	}
	if task.BillingStatus == PPVideoBillingStatusHeld || task.BillingStatus == PPVideoBillingStatusSettling {
		if err := s.ReleasePPVideoBalanceHold(ctx, task, in.RequestPayloadHash); err != nil {
			return err
		}
	}
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return err
	}
	_, err = repo.MarkPPVideoTaskSettled(ctx, MarkPPVideoTaskSettledParams{
		TaskID: task.TaskID, Status: PPVideoTaskStatusFailed,
		BillingStatus: PPVideoBillingStatusReleased,
	})
	return err
}

func ppVideoActualSettlementCost(task *PPVideoTask) float64 {
	rawCost := ppVideoRawSettlementCost(task)
	if rawCost <= 0 {
		return 0
	}
	multiplier := 1.0
	if task != nil && task.EstimatedTotalCost > 0 && task.HoldAmount > 0 {
		multiplier = task.HoldAmount / task.EstimatedTotalCost
	}
	return rawCost * multiplier
}

func ppVideoRawSettlementCost(task *PPVideoTask) float64 {
	if task == nil {
		return 0
	}
	if task.BillingFormula != "" && task.BillingUnitPrice > 0 {
		durationMs := task.GeneratedVideoDurationMilliseconds
		if durationMs <= 0 {
			durationMs = task.RequestedVideoDurationMilliseconds
		}
		units := ppVideoTaskBillingUnits(task, durationMs)
		rawCost := units * task.BillingUnitPrice
		if rawCost <= 0 {
			return 0
		}
		return rawCost
	}
	base := task.EstimatedTotalCost
	if base <= 0 {
		base = task.HoldAmount
	}
	if base <= 0 {
		return 0
	}
	requestedMs := task.RequestedVideoDurationMilliseconds
	if requestedMs <= 0 {
		requestedMs = int64(task.RequestedVideoDurationSeconds) * 1000
	}
	if task.GeneratedVideoDurationMilliseconds <= 0 || requestedMs <= 0 {
		return base
	}
	actual := base * float64(task.GeneratedVideoDurationMilliseconds) / float64(requestedMs)
	if actual < 0 {
		return 0
	}
	return actual
}

func ppVideoBalanceCaptureAmount(task *PPVideoTask, actualCost float64) float64 {
	if task == nil || actualCost <= 0 {
		return 0
	}
	if task.HoldAmount <= 0 {
		return 0
	}
	if task.HoldAmount > 0 && actualCost > task.HoldAmount {
		return task.HoldAmount
	}
	return actualCost
}

func ppVideoTaskBillingUnits(task *PPVideoTask, outputDurationMilliseconds int64) float64 {
	if task == nil || outputDurationMilliseconds <= 0 || task.VideoCount <= 0 {
		return 0
	}
	count := float64(task.VideoCount)
	switch task.BillingFormula {
	case PPVideoBillingFormulaPerSecond:
		return float64(outputDurationMilliseconds) / 1000 * count
	default:
		return 0
	}
}

func (s *OpenAIGatewayService) recordPPVideoUsage(
	ctx context.Context,
	in *PPVideoSettlementInput,
	actualCost float64,
	isSubscription bool,
) error {
	if s == nil || in == nil || in.Task == nil || in.APIKey == nil || in.Account == nil {
		return nil
	}
	task := in.Task
	totalCost := task.EstimatedTotalCost
	if totalCost <= 0 {
		totalCost = actualCost
	}
	outputCost := ppVideoRawSettlementCost(task)
	if outputCost <= 0 {
		outputCost = totalCost
	}
	rateMultiplier := 1.0
	if outputCost > 0 {
		rateMultiplier = actualCost / outputCost
	}
	videoCount := task.VideoCount
	if videoCount <= 0 {
		videoCount = 1
	}
	resolution := normalizePPVideoResolution(task.VideoResolution)
	if resolution == "" {
		resolution = VideoBillingResolution720P
	}
	durationSeconds := task.RequestedVideoDurationSeconds
	if task.GeneratedVideoDurationMilliseconds > 0 {
		durationSeconds = int((task.GeneratedVideoDurationMilliseconds + 999) / 1000)
	}
	if durationSeconds <= 0 {
		durationSeconds = 1
	}
	billingType := BillingTypeBalance
	if isSubscription {
		billingType = BillingTypeSubscription
	}
	billingMode := string(BillingModeVideo)
	accountRateMultiplier := in.Account.BillingRateMultiplier()
	model := strings.TrimSpace(task.Model)
	if model == "" {
		model = task.Platform
	}
	usageLog := &UsageLog{
		UserID:                task.UserID,
		APIKeyID:              task.APIKeyID,
		AccountID:             task.AccountID,
		RequestID:             ppVideoUsageRequestID(task),
		Model:                 model,
		RequestedModel:        model,
		InboundEndpoint:       optionalTrimmedStringPtr(in.InboundEndpoint),
		UpstreamEndpoint:      optionalTrimmedStringPtr(in.UpstreamEndpoint),
		GroupID:               task.GroupID,
		VideoCount:            videoCount,
		VideoResolution:       &resolution,
		VideoDurationSeconds:  &durationSeconds,
		OutputCost:            outputCost,
		TotalCost:             outputCost,
		ActualCost:            actualCost,
		RateMultiplier:        rateMultiplier,
		AccountRateMultiplier: &accountRateMultiplier,
		BillingType:           billingType,
		RequestType:           RequestTypeSync,
		BillingMode:           &billingMode,
		CreatedAt:             time.Now().UTC(),
	}
	if outputCost <= 0 {
		usageLog.TotalCost = totalCost
	}
	if isSubscription && in.Subscription != nil {
		usageLog.SubscriptionID = &in.Subscription.ID
	}
	if err := s.applyPPVideoUsageAccounting(ctx, in, usageLog, usageLog.TotalCost, actualCost, accountRateMultiplier, isSubscription); err != nil {
		return err
	}
	if s.usageLogRepo != nil {
		writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.pp_video_billing")
	}
	return nil
}

func (s *OpenAIGatewayService) applyPPVideoUsageAccounting(
	ctx context.Context,
	in *PPVideoSettlementInput,
	usageLog *UsageLog,
	totalCost float64,
	actualCost float64,
	accountRateMultiplier float64,
	isSubscription bool,
) error {
	if s == nil || in == nil || in.Task == nil || in.APIKey == nil || in.Account == nil {
		return nil
	}
	repo := s.usageBillingRepo
	if repo == nil {
		return nil
	}
	user := in.User
	if user == nil {
		user = in.APIKey.User
	}
	if user == nil {
		user = &User{ID: in.Task.UserID}
	}
	cmd := &UsageBillingCommand{
		RequestID:          PPVideoAccountingRequestID(in.Task.LocalTaskID),
		APIKeyID:           in.APIKey.ID,
		UserID:             user.ID,
		AccountID:          in.Account.ID,
		AccountType:        in.Account.Type,
		Model:              usageLog.Model,
		BillingType:        usageLog.BillingType,
		MediaType:          string(BillingModeVideo),
		RequestPayloadHash: strings.TrimSpace(in.RequestPayloadHash),
	}
	if isSubscription && in.Subscription != nil {
		cmd.SubscriptionID = &in.Subscription.ID
		cmd.SubscriptionCost = actualCost
	}
	if !isSubscription && actualCost > 0 {
		capturedAmount := ppVideoBalanceCaptureAmount(in.Task, actualCost)
		if extraBalanceCost := actualCost - capturedAmount; extraBalanceCost > 0 {
			cmd.BalanceCost = extraBalanceCost
		}
	}
	if actualCost > 0 && in.APIKey.Quota > 0 && in.APIKeyService != nil {
		cmd.APIKeyQuotaCost = actualCost
	}
	if actualCost > 0 && in.APIKey.HasRateLimits() && in.APIKeyService != nil {
		cmd.APIKeyRateLimitCost = actualCost
	}
	if totalCost > 0 && in.Account.IsAPIKeyOrBedrock() && in.Account.HasAnyQuotaLimit() {
		cmd.AccountQuotaCost = totalCost * accountRateMultiplier
	}
	cmd.Normalize()
	billingCtx, cancel := detachedBillingContext(ctx)
	defer cancel()
	result, err := repo.Apply(billingCtx, cmd)
	if err != nil {
		return err
	}
	if result == nil || !result.Applied {
		if s.deferredService != nil {
			s.deferredService.ScheduleLastUsedUpdate(in.Account.ID)
		}
		return nil
	}
	if result.APIKeyQuotaExhausted {
		if invalidator, ok := in.APIKeyService.(apiKeyAuthCacheInvalidator); ok && in.APIKey.Key != "" {
			invalidator.InvalidateAuthCacheByKey(billingCtx, in.APIKey.Key)
		}
	}
	s.finalizePPVideoUsageAccounting(billingCtx, in, totalCost, actualCost, accountRateMultiplier, isSubscription, result)
	return nil
}

func (s *OpenAIGatewayService) finalizePPVideoUsageAccounting(
	ctx context.Context,
	in *PPVideoSettlementInput,
	totalCost float64,
	actualCost float64,
	accountRateMultiplier float64,
	isSubscription bool,
	result *UsageBillingApplyResult,
) {
	if s == nil || in == nil || in.Task == nil || in.APIKey == nil || in.Account == nil {
		return
	}
	deps := s.billingDeps()
	if deps == nil {
		return
	}
	if actualCost > 0 && isSubscription && deps.billingCacheService != nil && in.APIKey.GroupID != nil {
		userID := in.Task.UserID
		if in.User != nil {
			userID = in.User.ID
		}
		deps.billingCacheService.QueueUpdateSubscriptionUsage(userID, *in.APIKey.GroupID, actualCost)
	}
	if actualCost > 0 && in.APIKey.HasRateLimits() && deps.billingCacheService != nil {
		deps.billingCacheService.QueueUpdateAPIKeyRateLimitUsage(in.APIKey.ID, actualCost)
	}
	if s.deferredService != nil {
		s.deferredService.ScheduleLastUsedUpdate(in.Account.ID)
	}
	user := in.User
	if user == nil {
		user = in.APIKey.User
	}
	if user == nil {
		user = &User{ID: in.Task.UserID}
	}
	if !isSubscription && in.QuotaPlatform != "" && actualCost > 0 &&
		deps.billingCacheService != nil && deps.userPlatformQuotaRepo != nil &&
		deps.billingCacheService.HasUserPlatformQuotaLimit(ctx, user.ID, in.QuotaPlatform) {
		deps.billingCacheService.IncrementUserPlatformQuotaUsage(user.ID, in.QuotaPlatform, actualCost)
		if deps.cfg == nil || !deps.cfg.Database.UserPlatformQuotaFlusherEnabled {
			dbCtx, dbCancel := detachUpstreamContext(ctx)
			userID, platform, cost := user.ID, in.QuotaPlatform, actualCost
			go func() {
				defer dbCancel()
				if err := deps.userPlatformQuotaRepo.IncrementUsageWithReset(dbCtx, userID, platform, cost, time.Now().UTC()); err != nil {
					userPlatformQuotaDBIncrErrorTotal.Add(1)
					logger.L().Warn("pp_video.user_platform_quota_increment_failed",
						zap.Int64("user_id", userID),
						zap.String("platform", platform),
						zap.Float64("cost", cost),
						zap.Error(err),
					)
				}
			}()
		}
	}
	if totalCost > 0 && deps.balanceNotifyService != nil {
		notifyAccountQuota(&postUsageBillingParams{
			Cost:                  &CostBreakdown{TotalCost: totalCost, ActualCost: actualCost, BillingMode: string(BillingModeVideo)},
			User:                  user,
			APIKey:                in.APIKey,
			Account:               in.Account,
			RequestPayloadHash:    strings.TrimSpace(in.RequestPayloadHash),
			AccountRateMultiplier: accountRateMultiplier,
			APIKeyService:         in.APIKeyService,
			Platform:              in.QuotaPlatform,
			IsSubscriptionBill:    isSubscription,
		}, deps, result)
	}
}

func ppVideoUsageRequestID(task *PPVideoTask) string {
	if task == nil {
		return ""
	}
	if taskID := strings.TrimSpace(task.TaskID); taskID != "" {
		return taskID
	}
	return strings.TrimSpace(task.LocalTaskID)
}
