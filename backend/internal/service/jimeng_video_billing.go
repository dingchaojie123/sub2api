package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	JimengVideoTaskStatusSubmitting     = "submitting"
	JimengVideoIdempotencyKeyMaxLength = 255

	JimengVideoBillingStatusHeld     = "held"
	JimengVideoBillingStatusCaptured = "captured"
	JimengVideoBillingStatusReleased = "released"
	JimengVideoBillingStatusNone     = "none"

	jimengVideoHoldRequestPrefix    = "jimeng_video_hold:"
	jimengVideoCaptureRequestPrefix = "jimeng_video_capture:"
	jimengVideoReleaseRequestPrefix = "jimeng_video_release:"
	jimengVideoAccountingPrefix     = "jimeng_video_accounting:"
)

var (
	ErrJimengVideoTaskNotFound           = infraerrors.New(http.StatusNotFound, "JIMENG_VIDEO_TASK_NOT_FOUND", "video request not found")
	ErrJimengVideoTaskRepositoryMissing  = infraerrors.New(http.StatusServiceUnavailable, "JIMENG_VIDEO_TASK_REPOSITORY_MISSING", "video task repository is not configured")
	ErrJimengVideoIdempotencyRequired    = infraerrors.New(http.StatusBadRequest, "JIMENG_VIDEO_IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key is required for video generation")
	ErrJimengVideoIdempotencyConflict    = infraerrors.New(http.StatusConflict, "JIMENG_VIDEO_IDEMPOTENCY_CONFLICT", "idempotency key reused with different video request")
	ErrJimengVideoIdempotencyInProgress  = infraerrors.New(http.StatusConflict, "JIMENG_VIDEO_IDEMPOTENCY_IN_PROGRESS", "idempotent video request is still being submitted")
	ErrJimengVideoBillingHoldFailed      = infraerrors.New(http.StatusBadGateway, "JIMENG_VIDEO_BILLING_HOLD_FAILED", "video balance hold failed")
	ErrJimengVideoSettlementBillingFailed = infraerrors.New(http.StatusBadGateway, "JIMENG_VIDEO_SETTLEMENT_BILLING_FAILED", "video settlement billing failed")
	ErrJimengVideoInsufficientBalance    = infraerrors.New(http.StatusPaymentRequired, "JIMENG_VIDEO_INSUFFICIENT_BALANCE", "insufficient balance for video generation hold")
)

type JimengVideoTask struct {
	ID                   int64
	LocalTaskID          string
	TaskID               string
	UserID               int64
	APIKeyID             int64
	GroupID              *int64
	AccountID            int64
	Model                string
	Status               string
	BillingStatus        string
	RequestHash          string
	IdempotencyKey       string
	HoldID               string
	CaptureID            string
	ReleaseID            string
	EstimatedTotalCost   float64
	HoldAmount           float64
	ActualCost           *float64
	Currency             string
	VideoDurationSeconds int
	VideoResolution      string
	ResponseStatus        int
	ResponseContentType   string
	ResponseBody          string
	LastErrorCode         string
	LastErrorMessage      string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	SubmittedAt          *time.Time
	FinishedAt           *time.Time
	SettledAt            *time.Time
}

type CreateJimengVideoTaskParams struct {
	LocalTaskID          string
	UserID               int64
	APIKeyID             int64
	GroupID              *int64
	AccountID            int64
	Model                string
	Status               string
	BillingStatus        string
	RequestHash          string
	IdempotencyKey       string
	HoldID               string
	CaptureID            string
	ReleaseID            string
	EstimatedTotalCost   float64
	HoldAmount           float64
	Currency             string
	VideoDurationSeconds int
	VideoResolution      string
}

type MarkJimengVideoSubmittedParams struct {
	LocalTaskID        string
	TaskID             string
	Status             string
	ResponseStatus      int
	ResponseContentType string
	ResponseBody        string
}

type MarkJimengVideoStatusParams struct {
	TaskID              string
	Status              string
	ResponseStatus      int
	ResponseContentType string
	ResponseBody        string
	LastErrorCode       string
	LastErrorMessage    string
}

type MarkJimengVideoSettledParams struct {
	TaskID           string
	Status           string
	BillingStatus    string
	ActualCost       float64
	LastErrorCode    string
	LastErrorMessage string
}

type JimengVideoTaskRepository interface {
	CreateJimengVideoTask(ctx context.Context, params CreateJimengVideoTaskParams) (*JimengVideoTask, error)
	GetJimengVideoTaskByIdempotencyKey(ctx context.Context, userID int64, apiKeyID int64, idempotencyKey string) (*JimengVideoTask, error)
	GetJimengVideoTaskForOwner(ctx context.Context, userID int64, apiKeyID int64, taskID string) (*JimengVideoTask, error)
	MarkJimengVideoTaskSubmitted(ctx context.Context, params MarkJimengVideoSubmittedParams) (*JimengVideoTask, error)
	MarkJimengVideoTaskStatus(ctx context.Context, params MarkJimengVideoStatusParams) (*JimengVideoTask, error)
	MarkJimengVideoTaskSubmitFailed(ctx context.Context, localTaskID string, code string, message string) error
	MarkJimengVideoTaskSettled(ctx context.Context, params MarkJimengVideoSettledParams) (*JimengVideoTask, error)
}

type JimengVideoSettlementInput struct {
	Task               *JimengVideoTask
	FinalStatus        string
	APIKey             *APIKey
	User               *User
	Account            *Account
	Subscription       *UserSubscription
	InboundEndpoint    string
	UpstreamEndpoint   string
	UserAgent          string
	IPAddress          string
	RequestPayloadHash string
	APIKeyService      APIKeyQuotaUpdater
	QuotaPlatform      string
}

func NewJimengVideoLocalTaskID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "vidtask_" + hex.EncodeToString(b[:]), nil
}

func JimengVideoHoldRequestID(localTaskID string) string {
	return jimengVideoHoldRequestPrefix + strings.TrimSpace(localTaskID)
}

func JimengVideoCaptureRequestID(localTaskID string) string {
	return jimengVideoCaptureRequestPrefix + strings.TrimSpace(localTaskID)
}

func JimengVideoReleaseRequestID(localTaskID string) string {
	return jimengVideoReleaseRequestPrefix + strings.TrimSpace(localTaskID)
}

func JimengVideoAccountingRequestID(localTaskID string) string {
	return jimengVideoAccountingPrefix + strings.TrimSpace(localTaskID)
}

func IsTerminalJimengVideoTaskStatus(status string) bool {
	switch NormalizeJimengTaskStatus(status) {
	case JimengTaskStatusSucceeded, JimengTaskStatusFailed:
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) jimengVideoTaskRepo() (JimengVideoTaskRepository, error) {
	if s == nil || s.usageBillingRepo == nil {
		return nil, ErrJimengVideoTaskRepositoryMissing
	}
	repo, ok := s.usageBillingRepo.(JimengVideoTaskRepository)
	if !ok || repo == nil {
		return nil, ErrJimengVideoTaskRepositoryMissing
	}
	return repo, nil
}

func (s *OpenAIGatewayService) CreateJimengVideoTask(ctx context.Context, params CreateJimengVideoTaskParams) (*JimengVideoTask, error) {
	repo, err := s.jimengVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.CreateJimengVideoTask(ctx, params)
}

func (s *OpenAIGatewayService) GetJimengVideoTaskByIdempotencyKey(ctx context.Context, userID int64, apiKeyID int64, idempotencyKey string) (*JimengVideoTask, error) {
	repo, err := s.jimengVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetJimengVideoTaskByIdempotencyKey(ctx, userID, apiKeyID, idempotencyKey)
}

func (s *OpenAIGatewayService) GetJimengVideoTaskForOwner(ctx context.Context, userID int64, apiKeyID int64, taskID string) (*JimengVideoTask, error) {
	repo, err := s.jimengVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetJimengVideoTaskForOwner(ctx, userID, apiKeyID, taskID)
}

func (s *OpenAIGatewayService) SelectJimengVideoTaskAccountForStatus(ctx context.Context, accountID int64) (selection *AccountSelectionResult, decision OpenAIAccountScheduleDecision, err error) {
	start := time.Now()
	decision = OpenAIAccountScheduleDecision{
		Layer:             "jimeng_video_task_binding",
		CandidateCount:    1,
		TopK:              1,
		SelectedAccountID: accountID,
	}
	defer func() {
		decision.LatencyMs = time.Since(start).Milliseconds()
	}()
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return nil, decision, ErrNoAvailableAccounts
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, decision, err
	}
	if account == nil {
		return nil, decision, ErrAccountNotFound
	}
	if account.Platform != PlatformJimeng {
		return nil, decision, ErrNoAvailableAccounts
	}
	decision.SelectedAccountType = account.Type
	result, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err == nil && result != nil && result.Acquired {
		return &AccountSelectionResult{Account: account, Acquired: true, ReleaseFunc: result.ReleaseFunc}, decision, nil
	}
	cfg := s.schedulingConfig()
	return &AccountSelectionResult{
		Account: account,
		WaitPlan: &AccountWaitPlan{
			AccountID:      account.ID,
			MaxConcurrency: account.Concurrency,
			Timeout:        cfg.StickySessionWaitTimeout,
			MaxWaiting:     cfg.StickySessionMaxWaiting,
		},
	}, decision, nil
}

func (s *OpenAIGatewayService) MarkJimengVideoTaskSubmitted(ctx context.Context, params MarkJimengVideoSubmittedParams) (*JimengVideoTask, error) {
	repo, err := s.jimengVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.MarkJimengVideoTaskSubmitted(ctx, params)
}

func (s *OpenAIGatewayService) MarkJimengVideoTaskStatus(ctx context.Context, params MarkJimengVideoStatusParams) (*JimengVideoTask, error) {
	repo, err := s.jimengVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.MarkJimengVideoTaskStatus(ctx, params)
}

func (s *OpenAIGatewayService) MarkJimengVideoTaskSubmitFailed(ctx context.Context, localTaskID string, code string, message string) error {
	repo, err := s.jimengVideoTaskRepo()
	if err != nil {
		return err
	}
	return repo.MarkJimengVideoTaskSubmitFailed(ctx, localTaskID, code, message)
}

func (s *OpenAIGatewayService) CalculateJimengVideoCost(ctx context.Context, apiKey *APIKey, user *User, meta JimengVideoBillingMetadata) (*CostBreakdown, error) {
	if apiKey == nil {
		return nil, errors.New("api key is required")
	}
	if user == nil {
		user = apiKey.User
	}
	videoMultiplier := 1.0
	baseMultiplier := 1.0
	if s != nil && s.cfg != nil {
		baseMultiplier = s.cfg.Default.RateMultiplier
	}
	if s != nil && user != nil && apiKey.GroupID != nil && apiKey.Group != nil {
		baseMultiplier = s.ResolveUserGroupRateMultiplier(ctx, user.ID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	}
	videoMultiplier = resolveVideoRateMultiplier(apiKey, baseMultiplier)

	result := &OpenAIForwardResult{
		Model:                JimengVideoDefaultModel,
		BillingModel:         JimengVideoBillingModel,
		UpstreamModel:        JimengVideoDefaultModel,
		ImageCount:           1,
		VideoCount:           1,
		VideoResolution:      meta.VideoResolution,
		VideoDurationSeconds: meta.VideoDurationSeconds,
	}
	if s != nil && s.billingService != nil {
		return s.calculateOpenAIRecordUsageCost(ctx, result, apiKey,
			usageBillingModelCandidates(JimengVideoBillingModel, result.BillingModel, result.Model, result.UpstreamModel),
			baseMultiplier, baseMultiplier, videoMultiplier, baseMultiplier, UsageTokens{}, "", false)
	}
	billing := NewBillingService(nil, nil)
	return billing.CalculateVideoCost(
		JimengVideoBillingModel,
		NormalizeVideoBillingResolutionOrDefault(meta.VideoResolution),
		1,
		NormalizeVideoBillingDurationSecondsOrDefault(meta.VideoDurationSeconds),
		videoPriceConfigFromAPIKey(apiKey),
		videoMultiplier,
	), nil
}

func (s *OpenAIGatewayService) ReserveJimengVideoBalanceHold(ctx context.Context, task *JimengVideoTask, payloadHash string) error {
	if s == nil || s.usageBillingRepo == nil || task == nil {
		return ErrJimengVideoBillingHoldFailed
	}
	if task.HoldAmount <= 0 {
		return nil
	}
	cmd := jimengVideoBalanceHoldCommand(task, task.HoldID, 0, payloadHash)
	if _, err := s.usageBillingRepo.ReserveBatchImageBalance(ctx, cmd); err != nil {
		if errors.Is(err, ErrBatchImageInsufficientBalance) {
			return ErrJimengVideoInsufficientBalance
		}
		return ErrJimengVideoBillingHoldFailed.WithCause(err)
	}
	return nil
}

func (s *OpenAIGatewayService) CaptureJimengVideoBalanceHold(ctx context.Context, task *JimengVideoTask, actualAmount float64, payloadHash string) error {
	if s == nil || s.usageBillingRepo == nil || task == nil {
		return ErrJimengVideoSettlementBillingFailed
	}
	if task.HoldAmount <= 0 && actualAmount <= 0 {
		return nil
	}
	cmd := jimengVideoBalanceHoldCommand(task, task.CaptureID, actualAmount, payloadHash)
	if _, err := s.usageBillingRepo.CaptureBatchImageBalance(ctx, cmd); err != nil {
		return ErrJimengVideoSettlementBillingFailed.WithCause(err)
	}
	return nil
}

func (s *OpenAIGatewayService) ReleaseJimengVideoBalanceHold(ctx context.Context, task *JimengVideoTask, payloadHash string) error {
	if s == nil || s.usageBillingRepo == nil || task == nil {
		return nil
	}
	if task.HoldAmount <= 0 {
		return nil
	}
	cmd := jimengVideoBalanceHoldCommand(task, task.ReleaseID, 0, payloadHash)
	if _, err := s.usageBillingRepo.ReleaseBatchImageBalance(ctx, cmd); err != nil {
		if errors.Is(err, ErrUsageBillingRequestConflict) {
			return nil
		}
		return ErrJimengVideoSettlementBillingFailed.WithCause(err)
	}
	return nil
}

func jimengVideoBalanceHoldCommand(task *JimengVideoTask, requestID string, actualAmount float64, payloadHash string) *BatchImageBalanceHoldCommand {
	if task == nil {
		return nil
	}
	return &BatchImageBalanceHoldCommand{
		RequestID:          strings.TrimSpace(requestID),
		APIKeyID:           task.APIKeyID,
		UserID:             task.UserID,
		BatchID:            strings.TrimSpace(task.LocalTaskID),
		HoldRequestID:      strings.TrimSpace(task.HoldID),
		HoldAmount:         task.HoldAmount,
		ActualAmount:       actualAmount,
		RequestPayloadHash: strings.TrimSpace(payloadHash),
	}
}

func (s *OpenAIGatewayService) SettleJimengVideoTask(ctx context.Context, in *JimengVideoSettlementInput) error {
	if in == nil || in.Task == nil {
		return ErrJimengVideoTaskNotFound
	}
	task := in.Task
	finalStatus := NormalizeJimengTaskStatus(in.FinalStatus)
	if finalStatus == "" {
		finalStatus = NormalizeJimengTaskStatus(task.Status)
	}
	if finalStatus == "" || finalStatus == JimengTaskStatusProcessing || finalStatus == JimengVideoTaskStatusSubmitting {
		return nil
	}
	if finalStatus == JimengTaskStatusSucceeded {
		return s.captureJimengVideoTask(ctx, in)
	}
	if finalStatus == JimengTaskStatusFailed {
		return s.releaseJimengVideoTask(ctx, in)
	}
	return nil
}

func (s *OpenAIGatewayService) captureJimengVideoTask(ctx context.Context, in *JimengVideoSettlementInput) error {
	task := in.Task
	if task.BillingStatus == JimengVideoBillingStatusCaptured || task.BillingStatus == JimengVideoBillingStatusReleased {
		return nil
	}
	actualCost := task.HoldAmount
	if actualCost < 0 {
		actualCost = 0
	}

	isSubscriptionBill := in.Subscription != nil && in.APIKey != nil && in.APIKey.Group != nil && in.APIKey.Group.IsSubscriptionType()
	if isSubscriptionBill {
		if err := s.recordJimengVideoSubscriptionUsage(in); err != nil {
			return ErrJimengVideoSettlementBillingFailed.WithCause(err)
		}
	} else if task.BillingStatus == JimengVideoBillingStatusHeld {
		if err := s.CaptureJimengVideoBalanceHold(ctx, task, actualCost, in.RequestPayloadHash); err != nil {
			return err
		}
		if err := s.recordJimengVideoBalanceUsage(ctx, in, actualCost); err != nil {
			return ErrJimengVideoSettlementBillingFailed.WithCause(err)
		}
	}

	if repo, err := s.jimengVideoTaskRepo(); err == nil {
		_, err = repo.MarkJimengVideoTaskSettled(ctx, MarkJimengVideoSettledParams{
			TaskID:        task.TaskID,
			Status:        JimengTaskStatusSucceeded,
			BillingStatus: JimengVideoBillingStatusCaptured,
			ActualCost:    actualCost,
		})
		return err
	}
	return nil
}

func (s *OpenAIGatewayService) releaseJimengVideoTask(ctx context.Context, in *JimengVideoSettlementInput) error {
	task := in.Task
	if task.BillingStatus == JimengVideoBillingStatusCaptured || task.BillingStatus == JimengVideoBillingStatusReleased {
		return nil
	}
	if task.BillingStatus == JimengVideoBillingStatusHeld {
		if err := s.ReleaseJimengVideoBalanceHold(ctx, task, in.RequestPayloadHash); err != nil {
			return err
		}
	}
	if repo, err := s.jimengVideoTaskRepo(); err == nil {
		_, err = repo.MarkJimengVideoTaskSettled(ctx, MarkJimengVideoSettledParams{
			TaskID:        task.TaskID,
			Status:        JimengTaskStatusFailed,
			BillingStatus: JimengVideoBillingStatusReleased,
		})
		return err
	}
	return nil
}

func (s *OpenAIGatewayService) recordJimengVideoSubscriptionUsage(in *JimengVideoSettlementInput) error {
	if in == nil || in.Task == nil || in.APIKey == nil || in.Account == nil {
		return nil
	}
	task := in.Task
	result := jimengVideoUsageResultFromTask(task)
	billingCtx, cancel := detachedBillingContext(context.Background())
	defer cancel()
	return s.RecordUsage(billingCtx, &OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             in.APIKey,
		User:               firstJimengVideoUser(in.User, in.APIKey),
		Account:            in.Account,
		Subscription:       in.Subscription,
		InboundEndpoint:    in.InboundEndpoint,
		UpstreamEndpoint:   in.UpstreamEndpoint,
		UserAgent:          in.UserAgent,
		IPAddress:          in.IPAddress,
		RequestPayloadHash: in.RequestPayloadHash,
		APIKeyService:      in.APIKeyService,
		QuotaPlatform:      in.QuotaPlatform,
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      JimengVideoBillingModel,
			ChannelMappedModel: JimengVideoBillingModel,
		},
	})
}

func (s *OpenAIGatewayService) recordJimengVideoBalanceUsage(ctx context.Context, in *JimengVideoSettlementInput, actualCost float64) error {
	if s == nil || in == nil || in.Task == nil || in.APIKey == nil || in.Account == nil {
		return nil
	}
	task := in.Task
	requestID := jimengVideoUsageRequestID(task)
	billingMode := string(BillingModeVideo)
	accountRateMultiplier := in.Account.BillingRateMultiplier()
	videoResolution := NormalizeVideoBillingResolutionOrDefault(task.VideoResolution)
	videoDurationSeconds := NormalizeVideoBillingDurationSecondsOrDefault(task.VideoDurationSeconds)
	totalCost := jimengVideoTaskTotalCost(task, actualCost)
	rateMultiplier := 1.0
	if totalCost > 0 {
		rateMultiplier = actualCost / totalCost
	}
	usageLog := &UsageLog{
		UserID:                 task.UserID,
		APIKeyID:               task.APIKeyID,
		AccountID:              task.AccountID,
		RequestID:              requestID,
		Model:                  JimengVideoBillingModel,
		RequestedModel:         JimengVideoBillingModel,
		UpstreamModel:          optionalNonEqualStringPtr(JimengVideoDefaultModel, JimengVideoBillingModel),
		InboundEndpoint:        optionalTrimmedStringPtr(in.InboundEndpoint),
		UpstreamEndpoint:       optionalTrimmedStringPtr(in.UpstreamEndpoint),
		VideoCount:             1,
		VideoResolution:        &videoResolution,
		VideoDurationSeconds:   &videoDurationSeconds,
		TotalCost:              totalCost,
		ActualCost:             actualCost,
		RateMultiplier:         rateMultiplier,
		AccountRateMultiplier:  &accountRateMultiplier,
		BillingType:            BillingTypeBalance,
		RequestType:            RequestTypeSync,
		BillingMode:            &billingMode,
		UserAgent:              optionalTrimmedStringPtr(in.UserAgent),
		IPAddress:              optionalTrimmedStringPtr(in.IPAddress),
		CreatedAt:              time.Now(),
	}
	if task.GroupID != nil {
		usageLog.GroupID = task.GroupID
	}
	if _, err := s.applyJimengVideoBalanceAccounting(ctx, in, usageLog, totalCost, actualCost, accountRateMultiplier); err != nil {
		return err
	}
	if s.usageLogRepo != nil {
		writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.jimeng_video_billing")
	}
	return nil
}

func (s *OpenAIGatewayService) applyJimengVideoBalanceAccounting(
	ctx context.Context,
	in *JimengVideoSettlementInput,
	usageLog *UsageLog,
	totalCost float64,
	actualCost float64,
	accountRateMultiplier float64,
) (bool, error) {
	if s == nil || in == nil || in.Task == nil || in.APIKey == nil || in.Account == nil {
		return false, nil
	}
	repo := s.usageBillingRepo
	if repo == nil {
		return true, nil
	}
	task := in.Task
	apiKey := in.APIKey
	account := in.Account
	user := firstJimengVideoUser(in.User, apiKey)
	if user == nil {
		return false, nil
	}
	cmd := &UsageBillingCommand{
		RequestID:          JimengVideoAccountingRequestID(task.LocalTaskID),
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		AccountID:          account.ID,
		AccountType:        account.Type,
		Model:              JimengVideoBillingModel,
		BillingType:        BillingTypeBalance,
		MediaType:          string(BillingModeVideo),
		RequestPayloadHash: strings.TrimSpace(in.RequestPayloadHash),
	}
	if usageLog != nil {
		cmd.Model = usageLog.Model
		cmd.BillingType = usageLog.BillingType
	}
	if actualCost > 0 && apiKey.Quota > 0 && in.APIKeyService != nil {
		cmd.APIKeyQuotaCost = actualCost
	}
	if actualCost > 0 && apiKey.HasRateLimits() && in.APIKeyService != nil {
		cmd.APIKeyRateLimitCost = actualCost
	}
	if totalCost > 0 && account.IsAPIKeyOrBedrock() && account.HasAnyQuotaLimit() {
		cmd.AccountQuotaCost = totalCost * accountRateMultiplier
	}
	cmd.Normalize()

	billingCtx, cancel := detachedBillingContext(ctx)
	defer cancel()
	result, err := repo.Apply(billingCtx, cmd)
	if err != nil {
		return false, err
	}
	if result == nil || !result.Applied {
		if s.deferredService != nil {
			s.deferredService.ScheduleLastUsedUpdate(account.ID)
		}
		return false, nil
	}
	if result.APIKeyQuotaExhausted {
		if invalidator, ok := in.APIKeyService.(apiKeyAuthCacheInvalidator); ok && apiKey.Key != "" {
			invalidator.InvalidateAuthCacheByKey(billingCtx, apiKey.Key)
		}
	}
	s.finalizeJimengVideoBalanceAccounting(billingCtx, in, totalCost, actualCost, accountRateMultiplier, result)
	return true, nil
}

func (s *OpenAIGatewayService) finalizeJimengVideoBalanceAccounting(
	ctx context.Context,
	in *JimengVideoSettlementInput,
	totalCost float64,
	actualCost float64,
	accountRateMultiplier float64,
	result *UsageBillingApplyResult,
) {
	if s == nil || in == nil || in.APIKey == nil || in.Account == nil {
		return
	}
	deps := s.billingDeps()
	if deps == nil {
		return
	}
	if actualCost > 0 && in.APIKey.HasRateLimits() && deps.billingCacheService != nil {
		deps.billingCacheService.QueueUpdateAPIKeyRateLimitUsage(in.APIKey.ID, actualCost)
	}
	if deps.deferredService != nil {
		deps.deferredService.ScheduleLastUsedUpdate(in.Account.ID)
	}
	user := firstJimengVideoUser(in.User, in.APIKey)
	if !isJimengVideoSubscriptionTask(in) && in.QuotaPlatform != "" && actualCost > 0 && user != nil &&
		deps.billingCacheService != nil && deps.userPlatformQuotaRepo != nil {
		if deps.billingCacheService.HasUserPlatformQuotaLimit(ctx, user.ID, in.QuotaPlatform) {
			deps.billingCacheService.IncrementUserPlatformQuotaUsage(user.ID, in.QuotaPlatform, actualCost)
			if deps.cfg == nil || !deps.cfg.Database.UserPlatformQuotaFlusherEnabled {
				dbCtx, dbCancel := detachUpstreamContext(ctx)
				userID, platform, cost := user.ID, in.QuotaPlatform, actualCost
				go func() {
					defer dbCancel()
					if err := deps.userPlatformQuotaRepo.IncrementUsageWithReset(dbCtx, userID, platform, cost, time.Now().UTC()); err != nil {
						userPlatformQuotaDBIncrErrorTotal.Add(1)
						logger.LegacyPrintf("service.jimeng_video_billing", "ALERT: incr user platform quota DB failed user=%d platform=%s cost=%f: %v", userID, platform, cost, err)
					}
				}()
			}
		}
	}
	if totalCost > 0 && deps.balanceNotifyService != nil {
		go notifyAccountQuota(&postUsageBillingParams{
			Cost:                  &CostBreakdown{TotalCost: totalCost, ActualCost: actualCost, BillingMode: string(BillingModeVideo)},
			User:                  user,
			APIKey:                in.APIKey,
			Account:               in.Account,
			RequestPayloadHash:    strings.TrimSpace(in.RequestPayloadHash),
			AccountRateMultiplier: accountRateMultiplier,
			APIKeyService:         in.APIKeyService,
			Platform:              in.QuotaPlatform,
		}, deps, result)
	}
}

func jimengVideoUsageResultFromTask(task *JimengVideoTask) *OpenAIForwardResult {
	if task == nil {
		return nil
	}
	return &OpenAIForwardResult{
		RequestID:             jimengVideoUsageRequestID(task),
		ResponseID:            strings.TrimSpace(task.TaskID),
		Model:                 JimengVideoBillingModel,
		BillingModel:          JimengVideoBillingModel,
		UpstreamModel:         JimengVideoDefaultModel,
		HasUsage:              true,
		ImageCount:            1,
		VideoCount:            1,
		VideoResolution:       task.VideoResolution,
		VideoDurationSeconds:  task.VideoDurationSeconds,
		UpstreamEndpoint:      jimengVideoUpstreamEndpoint(JimengVideoEndpointStatus),
		TaskStatus:            JimengTaskStatusSucceeded,
	}
}

func jimengVideoUsageRequestID(task *JimengVideoTask) string {
	if task == nil {
		return ""
	}
	if taskID := strings.TrimSpace(task.TaskID); taskID != "" {
		return taskID
	}
	return strings.TrimSpace(task.LocalTaskID)
}

func jimengVideoTaskTotalCost(task *JimengVideoTask, fallback float64) float64 {
	if task != nil && task.EstimatedTotalCost > 0 {
		return task.EstimatedTotalCost
	}
	if fallback > 0 {
		return fallback
	}
	return 0
}

func isJimengVideoSubscriptionTask(in *JimengVideoSettlementInput) bool {
	return in != nil && in.Subscription != nil && in.APIKey != nil && in.APIKey.Group != nil && in.APIKey.Group.IsSubscriptionType()
}

func firstJimengVideoUser(user *User, apiKey *APIKey) *User {
	if user != nil {
		return user
	}
	if apiKey != nil {
		return apiKey.User
	}
	return nil
}
