package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var errJimengVideoPollerAccountBusy = errors.New("jimeng video poller account busy")

type JimengVideoPollerOutcome string

const (
	JimengVideoPollerOutcomeSkipped    JimengVideoPollerOutcome = "skipped"
	JimengVideoPollerOutcomeProcessing JimengVideoPollerOutcome = "processing"
	JimengVideoPollerOutcomeSucceeded  JimengVideoPollerOutcome = "succeeded"
	JimengVideoPollerOutcomeFailed     JimengVideoPollerOutcome = "failed"
	JimengVideoPollerOutcomeTimedOut   JimengVideoPollerOutcome = "timed_out"
	JimengVideoPollerOutcomeRetry      JimengVideoPollerOutcome = "retry"
)

type JimengVideoPollerOptions struct {
	BatchSize        int
	PollInterval     time.Duration
	LeaseTTL         time.Duration
	UpstreamTimeout  time.Duration
	SubmitTimeout    time.Duration
	MaxProcessingAge time.Duration
}

type JimengVideoPollerRunStats struct {
	Claimed    int
	Processing int
	Succeeded  int
	Failed     int
	TimedOut   int
	Skipped    int
	Errors     int
}

type JimengVideoPollerTaskResult struct {
	Outcome JimengVideoPollerOutcome
	Status  string
}

type ClaimJimengVideoTasksForPollingParams struct {
	LeaseOwner   string
	LeaseUntil   time.Time
	SubmitCutoff time.Time
	Limit        int
}

type JimengVideoTaskPollRepository interface {
	ClaimJimengVideoTasksForPolling(ctx context.Context, params ClaimJimengVideoTasksForPollingParams) ([]*JimengVideoTask, error)
	ReleaseJimengVideoTaskPollLease(ctx context.Context, localTaskID string, leaseOwner string) error
	MarkJimengVideoTaskStatus(ctx context.Context, params MarkJimengVideoStatusParams) (*JimengVideoTask, error)
	MarkJimengVideoTaskSubmitFailed(ctx context.Context, localTaskID string, code string, message string) error
}

type JimengVideoPollerGateway interface {
	ForwardJimengVideoBuffered(ctx context.Context, c *gin.Context, account *Account, endpoint JimengVideoEndpoint, taskID string, body []byte) (*OpenAIForwardResult, error)
	SettleJimengVideoTask(ctx context.Context, in *JimengVideoSettlementInput) error
	ReleaseJimengVideoBalanceHold(ctx context.Context, task *JimengVideoTask, payloadHash string) error
}

type JimengVideoAPIKeyLoader interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
	UpdateQuotaUsed(ctx context.Context, apiKeyID int64, cost float64) error
	UpdateRateLimitUsage(ctx context.Context, apiKeyID int64, cost float64) error
}

type JimengVideoAccountLoader interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type JimengVideoSubscriptionResolver interface {
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
}

type JimengVideoBalanceCacheInvalidator interface {
	InvalidateUserBalance(ctx context.Context, userID int64) error
}

type JimengVideoAccountSelector interface {
	SelectJimengVideoTaskAccountForStatus(ctx context.Context, accountID int64) (selection *AccountSelectionResult, decision OpenAIAccountScheduleDecision, err error)
}

type JimengVideoPollerService struct {
	Repo           JimengVideoTaskPollRepository
	Gateway        JimengVideoPollerGateway
	APIKeys        JimengVideoAPIKeyLoader
	Accounts       JimengVideoAccountLoader
	Subscriptions  JimengVideoSubscriptionResolver
	BalanceCache   JimengVideoBalanceCacheInvalidator
	AccountSelector JimengVideoAccountSelector
	Options        JimengVideoPollerOptions
	LeaseOwner     string
	Now            func() time.Time
}

func NewJimengVideoPollerService(
	repo JimengVideoTaskPollRepository,
	gateway JimengVideoPollerGateway,
	apiKeys JimengVideoAPIKeyLoader,
	accounts JimengVideoAccountLoader,
	subscriptions JimengVideoSubscriptionResolver,
	balanceCache JimengVideoBalanceCacheInvalidator,
	opts JimengVideoPollerOptions,
) *JimengVideoPollerService {
	return &JimengVideoPollerService{
		Repo:          repo,
		Gateway:       gateway,
		APIKeys:       apiKeys,
		Accounts:      accounts,
		Subscriptions: subscriptions,
		BalanceCache:  balanceCache,
		Options:       opts,
		LeaseOwner:    defaultJimengVideoPollerLeaseOwner(),
		Now:           time.Now,
	}
}

func ProvideJimengVideoPollerRuntime(
	gateway *OpenAIGatewayService,
	repo UsageBillingRepository,
	apiKeyService *APIKeyService,
	accountRepo AccountRepository,
	subscriptionService *SubscriptionService,
	billingCache *BillingCacheService,
	cfg *config.Config,
) *JimengVideoPollerRuntime {
	var taskRepo JimengVideoTaskPollRepository
	if typed, ok := repo.(JimengVideoTaskPollRepository); ok {
		taskRepo = typed
	}
	poller := NewJimengVideoPollerService(
		taskRepo,
		gateway,
		apiKeyService,
		accountRepo,
		subscriptionService,
		billingCache,
		jimengVideoPollerOptionsFromConfig(cfg),
	)
	poller.AccountSelector = gateway
	runtime := NewJimengVideoPollerRuntime(poller, cfg)
	runtime.Start()
	return runtime
}

func jimengVideoPollerOptionsFromConfig(cfg *config.Config) JimengVideoPollerOptions {
	opts := JimengVideoPollerOptions{
		BatchSize:        25,
		PollInterval:     30 * time.Second,
		LeaseTTL:         2 * time.Minute,
		UpstreamTimeout:  30 * time.Second,
		SubmitTimeout:    10 * time.Minute,
		MaxProcessingAge: 2 * time.Hour,
	}
	if cfg == nil {
		return opts
	}
	if cfg.JimengVideo.PollBatchSize > 0 {
		opts.BatchSize = cfg.JimengVideo.PollBatchSize
	}
	if cfg.JimengVideo.PollIntervalSeconds > 0 {
		opts.PollInterval = time.Duration(cfg.JimengVideo.PollIntervalSeconds) * time.Second
	}
	if cfg.JimengVideo.PollLeaseTTLSeconds > 0 {
		opts.LeaseTTL = time.Duration(cfg.JimengVideo.PollLeaseTTLSeconds) * time.Second
	}
	if cfg.JimengVideo.PollUpstreamTimeoutSeconds > 0 {
		opts.UpstreamTimeout = time.Duration(cfg.JimengVideo.PollUpstreamTimeoutSeconds) * time.Second
	}
	if cfg.JimengVideo.SubmitTimeoutSeconds > 0 {
		opts.SubmitTimeout = time.Duration(cfg.JimengVideo.SubmitTimeoutSeconds) * time.Second
	}
	if cfg.JimengVideo.MaxProcessingSeconds > 0 {
		opts.MaxProcessingAge = time.Duration(cfg.JimengVideo.MaxProcessingSeconds) * time.Second
	}
	return opts
}

func defaultJimengVideoPollerLeaseOwner() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

func (s *JimengVideoPollerService) RunOnce(ctx context.Context) (JimengVideoPollerRunStats, error) {
	var stats JimengVideoPollerRunStats
	if s == nil || s.Repo == nil || s.Gateway == nil {
		return stats, ErrJimengVideoTaskRepositoryMissing
	}
	opts := s.options()
	if opts.BatchSize <= 0 {
		return stats, nil
	}
	now := s.now()
	tasks, err := s.Repo.ClaimJimengVideoTasksForPolling(ctx, ClaimJimengVideoTasksForPollingParams{
		LeaseOwner:   s.leaseOwner(),
		LeaseUntil:   now.Add(opts.LeaseTTL),
		SubmitCutoff: now.Add(-opts.SubmitTimeout),
		Limit:        opts.BatchSize,
	})
	if err != nil {
		stats.Errors++
		return stats, err
	}
	stats.Claimed = len(tasks)
	var lastErr error
	for _, task := range tasks {
		if task == nil {
			stats.Skipped++
			continue
		}
		result, procErr := s.ProcessTask(ctx, task)
		switch result.Outcome {
		case JimengVideoPollerOutcomeProcessing:
			stats.Processing++
		case JimengVideoPollerOutcomeSucceeded:
			stats.Succeeded++
		case JimengVideoPollerOutcomeFailed:
			stats.Failed++
		case JimengVideoPollerOutcomeTimedOut:
			stats.TimedOut++
		case JimengVideoPollerOutcomeSkipped:
			stats.Skipped++
		}
		if procErr != nil {
			stats.Errors++
			lastErr = procErr
			logger.L().Warn("jimeng_video.poller.task_failed",
				zap.String("local_task_id", task.LocalTaskID),
				zap.String("task_id", task.TaskID),
				zap.Error(procErr),
			)
		}
		if releaseErr := s.Repo.ReleaseJimengVideoTaskPollLease(ctx, task.LocalTaskID, s.leaseOwner()); releaseErr != nil {
			stats.Errors++
			lastErr = releaseErr
			logger.L().Warn("jimeng_video.poller.release_lease_failed",
				zap.String("local_task_id", task.LocalTaskID),
				zap.String("lease_owner", s.leaseOwner()),
				zap.Error(releaseErr),
			)
		}
	}
	return stats, lastErr
}

func (s *JimengVideoPollerService) ProcessTask(ctx context.Context, task *JimengVideoTask) (JimengVideoPollerTaskResult, error) {
	if s == nil || task == nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeSkipped}, nil
	}
	if s.Repo == nil || s.Gateway == nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, ErrJimengVideoTaskRepositoryMissing
	}
	if NormalizeJimengTaskStatus(task.BillingStatus) != JimengVideoBillingStatusHeld {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeSkipped}, nil
	}
	opts := s.options()
	now := s.now()
	status := NormalizeJimengTaskStatus(task.Status)
	if status == "" {
		status = JimengTaskStatusProcessing
	}
	if IsTerminalJimengVideoTaskStatus(status) {
		return s.settleFromTask(ctx, task, status, nil)
	}
	if strings.TrimSpace(task.TaskID) == "" {
		if taskAge(task, now) >= opts.SubmitTimeout {
			return s.releaseAndFail(ctx, task, "submit_timeout", "video task submission timed out")
		}
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeSkipped, Status: status}, nil
	}
	if taskAge(task, now) >= opts.MaxProcessingAge {
		return s.releaseAndFail(ctx, task, "processing_timeout", "video task processing timed out")
	}
	account, releaseFunc, err := s.resolveAccount(ctx, task.AccountID)
	if err != nil {
		if errors.Is(err, errJimengVideoPollerAccountBusy) {
			return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeSkipped, Status: status}, nil
		}
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, err
	}
	if releaseFunc != nil {
		defer releaseFunc()
	}
	requestCtx := ctx
	cancel := func() {}
	if opts.UpstreamTimeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, opts.UpstreamTimeout)
	}
	defer cancel()
	result, err := s.Gateway.ForwardJimengVideoBuffered(requestCtx, nil, account, JimengVideoEndpointStatus, task.TaskID, nil)
	if err != nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, err
	}
	if result == nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, errors.New("jimeng video poller got empty upstream result")
	}
	upstreamStatus := NormalizeJimengTaskStatus(result.TaskStatus)
	if upstreamStatus == "" {
		upstreamStatus = JimengTaskStatusProcessing
	}
	updated, err := s.Repo.MarkJimengVideoTaskStatus(ctx, MarkJimengVideoStatusParams{
		TaskID:              task.TaskID,
		Status:              upstreamStatus,
		ResponseStatus:      result.ResponseStatusCode,
		ResponseContentType: result.ResponseContentType,
		ResponseBody:        string(result.ResponseBody),
	})
	if err != nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, err
	}
	task = updated
	if IsTerminalJimengVideoTaskStatus(upstreamStatus) {
		return s.settleFromTask(ctx, task, upstreamStatus, result)
	}
	return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeProcessing, Status: upstreamStatus}, nil
}

func (s *JimengVideoPollerService) settleFromTask(ctx context.Context, task *JimengVideoTask, finalStatus string, upstream *OpenAIForwardResult) (JimengVideoPollerTaskResult, error) {
	finalStatus = NormalizeJimengTaskStatus(finalStatus)
	if finalStatus == "" {
		finalStatus = NormalizeJimengTaskStatus(task.Status)
	}
	if finalStatus == "" {
		finalStatus = JimengTaskStatusFailed
	}
	if task == nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeSkipped}, nil
	}
	if strings.TrimSpace(task.TaskID) == "" {
		if finalStatus == JimengTaskStatusFailed {
			return s.releaseAndFail(ctx, task, "submit_failed", "video task submission failed")
		}
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeSkipped}, nil
	}
	apiKey, err := s.loadAPIKey(ctx, task.APIKeyID)
	if err != nil {
		apiKey = nil
	}
	var user *User
	if apiKey != nil && apiKey.User != nil {
		user = apiKey.User
	} else {
		user = &User{ID: task.UserID}
	}
	if apiKey != nil && apiKey.User == nil {
		apiKeyCopy := *apiKey
		apiKeyCopy.User = user
		apiKey = &apiKeyCopy
	}
	account, err := s.loadAccount(ctx, task.AccountID)
	if err != nil {
		account = nil
	}
	subscription, err := s.loadSubscription(ctx, task, apiKey)
	if err != nil {
		subscription = nil
	}
	upstreamEndpoint := jimengVideoUpstreamEndpoint(JimengVideoEndpointStatus)
	if upstream != nil && strings.TrimSpace(upstream.UpstreamEndpoint) != "" {
		upstreamEndpoint = upstream.UpstreamEndpoint
	}
	settlement := &JimengVideoSettlementInput{
		Task:               task,
		FinalStatus:        finalStatus,
		APIKey:             apiKey,
		User:               user,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    "/v1/video/generations",
		UpstreamEndpoint:   upstreamEndpoint,
		RequestPayloadHash: task.RequestHash,
		APIKeyService:      s.APIKeys,
		QuotaPlatform:      PlatformJimeng,
	}
	if err := s.Gateway.SettleJimengVideoTask(ctx, settlement); err != nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, err
	}
	if s.BalanceCache != nil {
		if err := s.BalanceCache.InvalidateUserBalance(ctx, user.ID); err != nil {
			logger.L().Warn("jimeng_video.poller.balance_cache_invalidate_failed",
				zap.Int64("user_id", user.ID),
				zap.Error(err),
			)
		}
	}
	outcome := JimengVideoPollerOutcomeFailed
	if finalStatus == JimengTaskStatusSucceeded {
		outcome = JimengVideoPollerOutcomeSucceeded
	}
	return JimengVideoPollerTaskResult{Outcome: outcome, Status: finalStatus}, nil
}

func (s *JimengVideoPollerService) releaseAndFail(ctx context.Context, task *JimengVideoTask, code string, message string) (JimengVideoPollerTaskResult, error) {
	if task == nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeSkipped}, nil
	}
	if err := s.Gateway.ReleaseJimengVideoBalanceHold(ctx, task, task.RequestHash); err != nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, err
	}
	if err := s.Repo.MarkJimengVideoTaskSubmitFailed(ctx, task.LocalTaskID, code, message); err != nil {
		return JimengVideoPollerTaskResult{Outcome: JimengVideoPollerOutcomeRetry}, err
	}
	if s.BalanceCache != nil {
		_ = s.BalanceCache.InvalidateUserBalance(ctx, task.UserID)
	}
	outcome := JimengVideoPollerOutcomeFailed
	if strings.Contains(code, "timeout") {
		outcome = JimengVideoPollerOutcomeTimedOut
	}
	return JimengVideoPollerTaskResult{Outcome: outcome, Status: JimengTaskStatusFailed}, nil
}

func (s *JimengVideoPollerService) resolveAccount(ctx context.Context, accountID int64) (*Account, func(), error) {
	if s == nil || accountID <= 0 {
		return nil, nil, ErrAccountNotFound
	}
	if s.AccountSelector != nil {
		selection, _, err := s.AccountSelector.SelectJimengVideoTaskAccountForStatus(ctx, accountID)
		if err != nil {
			return nil, nil, err
		}
		if selection != nil {
			if selection.Account == nil {
				return nil, nil, ErrAccountNotFound
			}
			if !selection.Acquired {
				return nil, nil, errJimengVideoPollerAccountBusy
			}
			return selection.Account, selection.ReleaseFunc, nil
		}
	}
	account, err := s.loadAccount(ctx, accountID)
	return account, nil, err
}

func (s *JimengVideoPollerService) loadAPIKey(ctx context.Context, apiKeyID int64) (*APIKey, error) {
	if s == nil || s.APIKeys == nil {
		return nil, infraerrors.New(500, "JIMENG_VIDEO_API_KEY_SERVICE_MISSING", "api key service is not configured")
	}
	apiKey, err := s.APIKeys.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, err
	}
	if apiKey == nil {
		return nil, ErrAccountNotFound
	}
	return apiKey, nil
}

func (s *JimengVideoPollerService) loadAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.Accounts == nil {
		return nil, ErrAccountNotFound
	}
	account, err := s.Accounts.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrAccountNotFound
	}
	return account, nil
}

func (s *JimengVideoPollerService) loadSubscription(ctx context.Context, task *JimengVideoTask, apiKey *APIKey) (*UserSubscription, error) {
	if s == nil || s.Subscriptions == nil || task == nil || apiKey == nil {
		return nil, nil
	}
	groupID := int64(0)
	if apiKey.GroupID != nil {
		groupID = *apiKey.GroupID
	} else if task.GroupID != nil {
		groupID = *task.GroupID
	}
	if groupID <= 0 {
		return nil, nil
	}
	sub, err := s.Subscriptions.GetActiveSubscription(ctx, task.UserID, groupID)
	if err != nil {
		if errors.Is(err, ErrSubscriptionNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return sub, nil
}

func (s *JimengVideoPollerService) options() JimengVideoPollerOptions {
	if s == nil {
		return JimengVideoPollerOptions{}
	}
	opts := s.Options
	if opts.BatchSize <= 0 {
		opts.BatchSize = 25
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 30 * time.Second
	}
	if opts.LeaseTTL <= 0 {
		opts.LeaseTTL = 2 * time.Minute
	}
	if opts.UpstreamTimeout <= 0 {
		opts.UpstreamTimeout = 30 * time.Second
	}
	if opts.SubmitTimeout <= 0 {
		opts.SubmitTimeout = 10 * time.Minute
	}
	if opts.MaxProcessingAge <= 0 {
		opts.MaxProcessingAge = 2 * time.Hour
	}
	return opts
}

func (s *JimengVideoPollerService) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *JimengVideoPollerService) leaseOwner() string {
	if s != nil && strings.TrimSpace(s.LeaseOwner) != "" {
		return strings.TrimSpace(s.LeaseOwner)
	}
	return defaultJimengVideoPollerLeaseOwner()
}

func taskAge(task *JimengVideoTask, now time.Time) time.Duration {
	if task == nil {
		return 0
	}
	start := task.CreatedAt
	if task.SubmittedAt != nil {
		start = *task.SubmittedAt
	}
	if start.IsZero() {
		return 0
	}
	return now.Sub(start)
}
