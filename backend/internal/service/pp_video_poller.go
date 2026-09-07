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
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var errPPVideoPollerAccountBusy = errors.New("PP video poller account busy")

type PPVideoPollerOutcome string

const (
	PPVideoPollerOutcomeSkipped    PPVideoPollerOutcome = "skipped"
	PPVideoPollerOutcomeProcessing PPVideoPollerOutcome = "processing"
	PPVideoPollerOutcomeSucceeded  PPVideoPollerOutcome = "succeeded"
	PPVideoPollerOutcomeFailed     PPVideoPollerOutcome = "failed"
	PPVideoPollerOutcomeTimedOut   PPVideoPollerOutcome = "timed_out"
	PPVideoPollerOutcomeRetry      PPVideoPollerOutcome = "retry"
)

type PPVideoPollerOptions struct {
	BatchSize        int
	PollInterval     time.Duration
	LeaseTTL         time.Duration
	UpstreamTimeout  time.Duration
	SubmitTimeout    time.Duration
	MaxProcessingAge time.Duration
}

type PPVideoPollerRunStats struct {
	Claimed    int
	Processing int
	Succeeded  int
	Failed     int
	TimedOut   int
	Skipped    int
	Errors     int
}

type PPVideoPollerTaskResult struct {
	Outcome PPVideoPollerOutcome
	Status  string
}

type PPVideoPollerGateway interface {
	ForwardPPVideoBuffered(ctx context.Context, c *gin.Context, account *Account, operation PPVideoOperation, taskID string, body []byte, publicRequests ...PPVideoPublicRequest) (*OpenAIForwardResult, error)
	SettlePPVideoTask(ctx context.Context, in *PPVideoSettlementInput) error
	ReleasePPVideoBalanceHold(ctx context.Context, task *PPVideoTask, payloadHash string) error
}

type PPVideoAPIKeyLoader interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

type PPVideoAccountLoader interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type PPVideoSubscriptionResolver interface {
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
}

type PPVideoBalanceCacheInvalidator interface {
	InvalidateUserBalance(ctx context.Context, userID int64) error
}

type PPVideoAccountSelector interface {
	SelectPPVideoTaskAccountForStatus(ctx context.Context, accountID int64, platform string) (*AccountSelectionResult, error)
}

type PPVideoPollerService struct {
	Repo            PPVideoTaskRepository
	Gateway         PPVideoPollerGateway
	APIKeys         PPVideoAPIKeyLoader
	Accounts        PPVideoAccountLoader
	Subscriptions   PPVideoSubscriptionResolver
	BalanceCache    PPVideoBalanceCacheInvalidator
	AccountSelector PPVideoAccountSelector
	Options         PPVideoPollerOptions
	LeaseOwner      string
	Now             func() time.Time
}

func NewPPVideoPollerService(
	repo PPVideoTaskRepository,
	gateway PPVideoPollerGateway,
	apiKeys PPVideoAPIKeyLoader,
	accounts PPVideoAccountLoader,
	subscriptions PPVideoSubscriptionResolver,
	balanceCache PPVideoBalanceCacheInvalidator,
	opts PPVideoPollerOptions,
) *PPVideoPollerService {
	return &PPVideoPollerService{
		Repo:          repo,
		Gateway:       gateway,
		APIKeys:       apiKeys,
		Accounts:      accounts,
		Subscriptions: subscriptions,
		BalanceCache:  balanceCache,
		Options:       opts,
		LeaseOwner:    defaultPPVideoPollerLeaseOwner(),
		Now:           time.Now,
	}
}

func ppVideoPollerOptionsFromConfig(cfg *config.Config) PPVideoPollerOptions {
	opts := PPVideoPollerOptions{
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

func defaultPPVideoPollerLeaseOwner() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s-pp-%d", host, os.Getpid())
}

func (s *PPVideoPollerService) RunOnce(ctx context.Context) (PPVideoPollerRunStats, error) {
	var stats PPVideoPollerRunStats
	if s == nil || s.Repo == nil || s.Gateway == nil {
		return stats, ErrPPVideoTaskRepositoryMissing
	}
	opts := s.options()
	if opts.BatchSize <= 0 {
		return stats, nil
	}
	now := s.now()
	leaseToken, err := NewPPVideoPollLeaseToken()
	if err != nil {
		return stats, err
	}
	tasks, err := s.Repo.ClaimPPVideoTasksForPolling(ctx, ClaimPPVideoTasksForPollingParams{
		LeaseOwner:   s.leaseOwner(),
		LeaseToken:   leaseToken,
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
		result, processErr := s.ProcessTask(ctx, task)
		switch result.Outcome {
		case PPVideoPollerOutcomeProcessing:
			stats.Processing++
		case PPVideoPollerOutcomeSucceeded:
			stats.Succeeded++
		case PPVideoPollerOutcomeFailed:
			stats.Failed++
		case PPVideoPollerOutcomeTimedOut:
			stats.TimedOut++
		case PPVideoPollerOutcomeSkipped:
			stats.Skipped++
		}
		if processErr != nil {
			stats.Errors++
			lastErr = processErr
			logger.L().Warn("pp_video.poller.task_failed",
				zap.String("local_task_id", task.LocalTaskID),
				zap.String("task_id", task.TaskID),
				zap.String("platform", task.Platform),
				zap.Error(processErr),
			)
		}
		if releaseErr := s.Repo.ReleasePPVideoTaskPollLease(ctx, ReleasePPVideoTaskPollLeaseParams{
			LocalTaskID: task.LocalTaskID,
			LeaseOwner:  s.leaseOwner(),
			LeaseToken:  task.PollLeaseToken,
		}); releaseErr != nil {
			stats.Errors++
			lastErr = releaseErr
			logger.L().Warn("pp_video.poller.release_lease_failed",
				zap.String("local_task_id", task.LocalTaskID),
				zap.String("lease_owner", s.leaseOwner()),
				zap.Error(releaseErr),
			)
		}
	}
	return stats, lastErr
}

func (s *PPVideoPollerService) ProcessTask(ctx context.Context, task *PPVideoTask) (PPVideoPollerTaskResult, error) {
	if s == nil || task == nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeSkipped}, nil
	}
	if s.Repo == nil || s.Gateway == nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, ErrPPVideoTaskRepositoryMissing
	}
	if !isPendingPPVideoBillingStatus(task.BillingStatus) {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeSkipped}, nil
	}
	opts := s.options()
	now := s.now()
	status := NormalizePPVideoTaskStatus(task.Status)
	if status == "" {
		status = PPVideoTaskStatusProcessing
	}
	if IsTerminalPPVideoTaskStatus(status) {
		return s.settleFromTask(ctx, task, status, nil)
	}
	if strings.TrimSpace(task.TaskID) == "" {
		if ppVideoTaskAge(task, now) >= opts.SubmitTimeout {
			return s.releaseAndFail(ctx, task, "submit_timeout", "video task submission timed out")
		}
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeSkipped, Status: status}, nil
	}
	if ppVideoTaskAge(task, now) >= opts.MaxProcessingAge {
		return s.releaseAndFail(ctx, task, "processing_timeout", "video task processing timed out")
	}
	account, releaseFunc, err := s.resolveAccount(ctx, task.AccountID, task.Platform)
	if err != nil {
		if errors.Is(err, errPPVideoPollerAccountBusy) {
			return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeSkipped, Status: status}, nil
		}
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
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
	durationMilliseconds := task.GeneratedVideoDurationMilliseconds
	if durationMilliseconds <= 0 {
		durationMilliseconds = task.RequestedVideoDurationMilliseconds
	}
	publicRequest := PPVideoPublicRequest{
		Model:                task.Model,
		DurationMilliseconds: durationMilliseconds,
		VideoCount:           task.VideoCount,
		Resolution:           task.VideoResolution,
	}
	result, err := s.Gateway.ForwardPPVideoBuffered(requestCtx, nil, account, task.Operation, task.TaskID, nil, publicRequest)
	if err != nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
	}
	if result == nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, errors.New("PP video poller got empty upstream result")
	}
	upstreamStatus := NormalizePPVideoTaskStatus(result.TaskStatus)
	if upstreamStatus == "" {
		upstreamStatus = PPVideoTaskStatusProcessing
	}
	updated, err := s.Repo.MarkPPVideoTaskStatus(ctx, MarkPPVideoTaskStatusParams{
		TaskID:                             task.TaskID,
		Status:                             upstreamStatus,
		GeneratedVideoDurationMilliseconds: result.VideoDurationMilliseconds,
		VideoCount:                         result.VideoCount,
		VideoResolution:                    result.VideoResolution,
		OutputWidth:                        result.VideoOutputWidth,
		OutputHeight:                       result.VideoOutputHeight,
		FrameRate:                          result.VideoFrameRate,
		InputVideoDurationMilliseconds:     result.VideoInputDurationMilliseconds,
		ResponseStatus:                     result.ResponseStatusCode,
		ResponseContentType:                result.ResponseContentType,
		ResponseBody:                       string(result.ResponseBody),
		LastErrorCode: func() string {
			if upstreamStatus == PPVideoTaskStatusFailed && strings.TrimSpace(result.ErrorMessage) != "" {
				return "upstream_task_failed"
			}
			return ""
		}(),
		LastErrorMessage: result.ErrorMessage,
	})
	if err != nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
	}
	if updated == nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, errors.New("PP video poller persisted empty task")
	}
	persistedStatus := NormalizePPVideoTaskStatus(updated.Status)
	if persistedStatus == "" {
		persistedStatus = upstreamStatus
	}
	if IsTerminalPPVideoTaskStatus(persistedStatus) {
		return s.settleFromTask(ctx, updated, persistedStatus, result)
	}
	return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeProcessing, Status: persistedStatus}, nil
}

func (s *PPVideoPollerService) settleFromTask(ctx context.Context, task *PPVideoTask, finalStatus string, upstream *OpenAIForwardResult) (PPVideoPollerTaskResult, error) {
	finalStatus = NormalizePPVideoTaskStatus(finalStatus)
	if task == nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeSkipped}, nil
	}
	if strings.TrimSpace(task.TaskID) == "" {
		if finalStatus == PPVideoTaskStatusFailed {
			return s.releaseAndFail(ctx, task, "submit_failed", "video task submission failed")
		}
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeSkipped}, nil
	}
	apiKey, err := s.loadAPIKey(ctx, task.APIKeyID)
	if err != nil {
		if finalStatus == PPVideoTaskStatusSucceeded {
			return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
		}
		apiKey = nil
	}
	user := &User{ID: task.UserID}
	if apiKey != nil && apiKey.User != nil {
		user = apiKey.User
	} else if apiKey != nil {
		apiKeyCopy := *apiKey
		apiKeyCopy.User = user
		apiKey = &apiKeyCopy
	}
	account, err := s.loadAccount(ctx, task.AccountID)
	if err != nil {
		if finalStatus == PPVideoTaskStatusSucceeded {
			return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
		}
		account = nil
	}
	subscription, err := s.loadSubscription(ctx, task, apiKey)
	if err != nil {
		if finalStatus == PPVideoTaskStatusSucceeded {
			return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
		}
		subscription = nil
	}
	if finalStatus == PPVideoTaskStatusSucceeded && isPPVideoSubscriptionTask(task) && subscription == nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, ErrSubscriptionNotFound
	}
	upstreamEndpoint := ppVideoUpstreamEndpoint(task)
	if upstream != nil && strings.TrimSpace(upstream.UpstreamEndpoint) != "" {
		upstreamEndpoint = upstream.UpstreamEndpoint
	}
	if err := s.Gateway.SettlePPVideoTask(ctx, &PPVideoSettlementInput{
		Task:               task,
		FinalStatus:        finalStatus,
		APIKey:             apiKey,
		User:               user,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    ppVideoInboundEndpoint(task),
		UpstreamEndpoint:   upstreamEndpoint,
		RequestPayloadHash: task.RequestHash,
		APIKeyService:      ppVideoAPIKeyQuotaUpdater(s.APIKeys),
		QuotaPlatform:      task.Platform,
	}); err != nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
	}
	if s.BalanceCache != nil {
		if err := s.BalanceCache.InvalidateUserBalance(ctx, user.ID); err != nil {
			logger.L().Warn("pp_video.poller.balance_cache_invalidate_failed",
				zap.Int64("user_id", user.ID),
				zap.Error(err),
			)
		}
	}
	outcome := PPVideoPollerOutcomeFailed
	if finalStatus == PPVideoTaskStatusSucceeded {
		outcome = PPVideoPollerOutcomeSucceeded
	}
	return PPVideoPollerTaskResult{Outcome: outcome, Status: finalStatus}, nil
}

func ppVideoAPIKeyQuotaUpdater(loader PPVideoAPIKeyLoader) APIKeyQuotaUpdater {
	updater, _ := loader.(APIKeyQuotaUpdater)
	return updater
}

func (s *PPVideoPollerService) releaseAndFail(ctx context.Context, task *PPVideoTask, code string, message string) (PPVideoPollerTaskResult, error) {
	if task == nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeSkipped}, nil
	}
	if err := s.Gateway.ReleasePPVideoBalanceHold(ctx, task, task.RequestHash); err != nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
	}
	if err := s.Repo.MarkPPVideoTaskSubmitFailed(ctx, task.LocalTaskID, code, message); err != nil {
		return PPVideoPollerTaskResult{Outcome: PPVideoPollerOutcomeRetry}, err
	}
	if s.BalanceCache != nil {
		_ = s.BalanceCache.InvalidateUserBalance(ctx, task.UserID)
	}
	outcome := PPVideoPollerOutcomeFailed
	if strings.Contains(code, "timeout") {
		outcome = PPVideoPollerOutcomeTimedOut
	}
	return PPVideoPollerTaskResult{Outcome: outcome, Status: PPVideoTaskStatusFailed}, nil
}

func (s *PPVideoPollerService) resolveAccount(ctx context.Context, accountID int64, platform string) (*Account, func(), error) {
	if s == nil || accountID <= 0 || !IsPPVideoPlatform(platform) {
		return nil, nil, ErrAccountNotFound
	}
	if s.AccountSelector != nil {
		selection, err := s.AccountSelector.SelectPPVideoTaskAccountForStatus(ctx, accountID, platform)
		if err != nil {
			if errors.Is(err, ErrNoAvailableAccounts) {
				return nil, nil, errPPVideoPollerAccountBusy
			}
			return nil, nil, err
		}
		if selection != nil {
			if selection.Account == nil {
				return nil, nil, ErrAccountNotFound
			}
			if !selection.Acquired {
				return nil, nil, errPPVideoPollerAccountBusy
			}
			return selection.Account, selection.ReleaseFunc, nil
		}
	}
	account, err := s.loadAccount(ctx, accountID)
	return account, nil, err
}

func (s *PPVideoPollerService) loadAPIKey(ctx context.Context, apiKeyID int64) (*APIKey, error) {
	if s == nil || s.APIKeys == nil {
		return nil, errors.New("PP video api key service is not configured")
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

func (s *PPVideoPollerService) loadAccount(ctx context.Context, accountID int64) (*Account, error) {
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

func (s *PPVideoPollerService) loadSubscription(ctx context.Context, task *PPVideoTask, apiKey *APIKey) (*UserSubscription, error) {
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

func (s *PPVideoPollerService) options() PPVideoPollerOptions {
	if s == nil {
		return PPVideoPollerOptions{}
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

func (s *PPVideoPollerService) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *PPVideoPollerService) leaseOwner() string {
	if s != nil && strings.TrimSpace(s.LeaseOwner) != "" {
		return strings.TrimSpace(s.LeaseOwner)
	}
	return defaultPPVideoPollerLeaseOwner()
}

func isPendingPPVideoBillingStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PPVideoBillingStatusHeld,
		PPVideoBillingStatusNone,
		PPVideoBillingStatusSettling,
		PPVideoBillingStatusSettlingNone:
		return true
	default:
		return false
	}
}

func isPPVideoSubscriptionTask(task *PPVideoTask) bool {
	if task == nil {
		return false
	}
	switch strings.TrimSpace(task.BillingStatus) {
	case PPVideoBillingStatusNone, PPVideoBillingStatusSettlingNone:
		return true
	default:
		return false
	}
}

func ppVideoTaskAge(task *PPVideoTask, now time.Time) time.Duration {
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

func ppVideoInboundEndpoint(task *PPVideoTask) string {
	if task == nil {
		return "/v1/video/generations"
	}
	switch task.Operation {
	case PPVideoOperationKlingTextToVideo:
		return "/v1/videos/text2video"
	case PPVideoOperationKlingImageToVideo:
		return "/v1/videos/image2video"
	default:
		return "/v1/video/generations"
	}
}

func ppVideoUpstreamEndpoint(task *PPVideoTask) string {
	if task == nil {
		return "/v1/video/generations/{task_id}"
	}
	switch task.Operation {
	case PPVideoOperationKlingTextToVideo:
		return "/v1/videos/text2video/{task_id}"
	case PPVideoOperationKlingImageToVideo:
		return "/v1/videos/image2video/{task_id}"
	default:
		return "/v1/video/generations/{task_id}"
	}
}
