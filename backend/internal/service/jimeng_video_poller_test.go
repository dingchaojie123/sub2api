package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestJimengVideoPollerProcessingKeepsHold(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	task := newJimengVideoPollerTestTask("vidtask_processing", "task_processing", JimengTaskStatusProcessing, now.Add(-time.Minute))
	repo := &jimengVideoPollerRepoStub{claimTasks: []*JimengVideoTask{task}}
	gateway := &jimengVideoPollerGatewayStub{
		forwardResult: &OpenAIForwardResult{
			TaskStatus:          JimengTaskStatusProcessing,
			ResponseStatusCode:  200,
			ResponseContentType: "application/json",
			ResponseBody:        []byte(`{"status":"processing"}`),
			UpstreamEndpoint:    "/v1/video/generations/task_processing",
		},
	}
	poller := newJimengVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, JimengVideoPollerOutcomeProcessing, result.Outcome)
	require.Len(t, repo.statusUpdates, 1)
	require.Equal(t, JimengTaskStatusProcessing, repo.statusUpdates[0].Status)
	require.Empty(t, gateway.settlements)
	require.Empty(t, gateway.releaseHolds)
}

func TestJimengVideoPollerSucceededSettlesHeldTask(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	task := newJimengVideoPollerTestTask("vidtask_success", "task_success", JimengTaskStatusProcessing, now.Add(-time.Minute))
	repo := &jimengVideoPollerRepoStub{claimTasks: []*JimengVideoTask{task}}
	gateway := &jimengVideoPollerGatewayStub{
		forwardResult: &OpenAIForwardResult{
			TaskStatus:          JimengTaskStatusSucceeded,
			ResponseStatusCode:  200,
			ResponseContentType: "application/json",
			ResponseBody:        []byte(`{"status":"succeeded"}`),
			UpstreamEndpoint:    "/v1/video/generations/task_success",
		},
	}
	poller := newJimengVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, JimengVideoPollerOutcomeSucceeded, result.Outcome)
	require.Len(t, repo.statusUpdates, 1)
	require.Equal(t, JimengTaskStatusSucceeded, repo.statusUpdates[0].Status)
	require.Len(t, gateway.settlements, 1)
	require.Equal(t, JimengTaskStatusSucceeded, gateway.settlements[0].FinalStatus)
	require.Equal(t, task.RequestHash, gateway.settlements[0].RequestPayloadHash)
	require.Equal(t, PlatformJimeng, gateway.settlements[0].QuotaPlatform)
	require.Len(t, poller.BalanceCache.(*jimengVideoPollerBalanceCacheStub).invalidatedUserIDs, 1)
	require.Equal(t, task.UserID, poller.BalanceCache.(*jimengVideoPollerBalanceCacheStub).invalidatedUserIDs[0])
}

func TestJimengVideoPollerSucceededSettlesSubscriptionTask(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	task := newJimengVideoPollerTestTask("vidtask_subscription", "task_subscription", JimengTaskStatusProcessing, now.Add(-time.Minute))
	task.BillingStatus = JimengVideoBillingStatusNone
	repo := &jimengVideoPollerRepoStub{claimTasks: []*JimengVideoTask{task}}
	gateway := &jimengVideoPollerGatewayStub{
		forwardResult: &OpenAIForwardResult{
			TaskStatus:          JimengTaskStatusSucceeded,
			ResponseStatusCode:  200,
			ResponseContentType: "application/json",
			ResponseBody:        []byte(`{"status":"succeeded"}`),
			UpstreamEndpoint:    "/v1/video/generations/task_subscription",
		},
	}
	poller := newJimengVideoPollerTestService(repo, gateway, now)
	poller.Subscriptions = &jimengVideoPollerSubscriptionStub{sub: &UserSubscription{ID: 42}}

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, JimengVideoPollerOutcomeSucceeded, result.Outcome)
	require.Len(t, gateway.settlements, 1)
	require.Equal(t, JimengVideoBillingStatusNone, gateway.settlements[0].Task.BillingStatus)
}

func TestJimengVideoPollerFailedSettlesByReleasingHold(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	task := newJimengVideoPollerTestTask("vidtask_failed", "task_failed", JimengTaskStatusProcessing, now.Add(-time.Minute))
	repo := &jimengVideoPollerRepoStub{claimTasks: []*JimengVideoTask{task}}
	gateway := &jimengVideoPollerGatewayStub{
		forwardResult: &OpenAIForwardResult{
			TaskStatus:          "refunded",
			ResponseStatusCode:  200,
			ResponseContentType: "application/json",
			ResponseBody:        []byte(`{"status":"refunded"}`),
			UpstreamEndpoint:    "/v1/video/generations/task_failed",
		},
	}
	poller := newJimengVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, JimengVideoPollerOutcomeFailed, result.Outcome)
	require.Len(t, repo.statusUpdates, 1)
	require.Equal(t, JimengTaskStatusFailed, repo.statusUpdates[0].Status)
	require.Len(t, gateway.settlements, 1)
	require.Equal(t, JimengTaskStatusFailed, gateway.settlements[0].FinalStatus)
}

func TestJimengVideoPollerUpstreamErrorKeepsHeldForRetry(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	task := newJimengVideoPollerTestTask("vidtask_retry", "task_retry", JimengTaskStatusProcessing, now.Add(-time.Minute))
	repo := &jimengVideoPollerRepoStub{claimTasks: []*JimengVideoTask{task}}
	gateway := &jimengVideoPollerGatewayStub{forwardErr: errors.New("upstream unavailable")}
	poller := newJimengVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.Error(t, err)
	require.Equal(t, JimengVideoPollerOutcomeRetry, result.Outcome)
	require.Empty(t, repo.statusUpdates)
	require.Empty(t, repo.submitFailures)
	require.Empty(t, gateway.settlements)
	require.Empty(t, gateway.releaseHolds)
}

func TestJimengVideoPollerSubmittingWithoutTaskIDTimesOutAndReleasesHold(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	task := newJimengVideoPollerTestTask("vidtask_submit_timeout", "", JimengVideoTaskStatusSubmitting, now.Add(-11*time.Minute))
	repo := &jimengVideoPollerRepoStub{claimTasks: []*JimengVideoTask{task}}
	gateway := &jimengVideoPollerGatewayStub{}
	poller := newJimengVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, JimengVideoPollerOutcomeTimedOut, result.Outcome)
	require.Len(t, gateway.releaseHolds, 1)
	require.Equal(t, task.LocalTaskID, gateway.releaseHolds[0].LocalTaskID)
	require.Len(t, repo.submitFailures, 1)
	require.Equal(t, task.LocalTaskID, repo.submitFailures[0].localTaskID)
	require.Equal(t, "submit_timeout", repo.submitFailures[0].code)
	require.Empty(t, gateway.forwards)
}

func TestJimengVideoPollerRunOnceClaimsAndReleasesLease(t *testing.T) {
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	task := newJimengVideoPollerTestTask("vidtask_run_once", "task_run_once", JimengTaskStatusProcessing, now.Add(-time.Minute))
	repo := &jimengVideoPollerRepoStub{claimTasks: []*JimengVideoTask{task}}
	gateway := &jimengVideoPollerGatewayStub{
		forwardResult: &OpenAIForwardResult{
			TaskStatus:          JimengTaskStatusProcessing,
			ResponseStatusCode:  200,
			ResponseContentType: "application/json",
			ResponseBody:        []byte(`{"status":"processing"}`),
		},
	}
	poller := newJimengVideoPollerTestService(repo, gateway, now)
	poller.LeaseOwner = "worker-a"

	stats, err := poller.RunOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, stats.Claimed)
	require.Equal(t, 1, stats.Processing)
	require.Len(t, repo.claimParams, 1)
	require.Equal(t, "worker-a", repo.claimParams[0].LeaseOwner)
	require.Equal(t, 1, repo.claimParams[0].Limit)
	require.Len(t, repo.releaseLeases, 1)
	require.Equal(t, task.LocalTaskID, repo.releaseLeases[0].localTaskID)
	require.Equal(t, "worker-a", repo.releaseLeases[0].leaseOwner)
}

func newJimengVideoPollerTestService(repo *jimengVideoPollerRepoStub, gateway *jimengVideoPollerGatewayStub, now time.Time) *JimengVideoPollerService {
	return &JimengVideoPollerService{
		Repo:           repo,
		Gateway:        gateway,
		APIKeys:        &jimengVideoPollerAPIKeyStub{apiKey: &APIKey{ID: 2, UserID: 1, User: &User{ID: 1}}},
		Accounts:       &jimengVideoPollerAccountStub{account: &Account{ID: 3, Platform: PlatformJimeng, Type: AccountTypeAPIKey, Concurrency: 1}},
		Subscriptions:  &jimengVideoPollerSubscriptionStub{},
		BalanceCache:   &jimengVideoPollerBalanceCacheStub{},
		Options:        JimengVideoPollerOptions{BatchSize: 1, LeaseTTL: time.Minute, UpstreamTimeout: time.Second, SubmitTimeout: 10 * time.Minute, MaxProcessingAge: time.Hour},
		LeaseOwner:     "test-worker",
		Now:            func() time.Time { return now },
	}
}

func newJimengVideoPollerTestTask(localTaskID string, upstreamTaskID string, status string, createdAt time.Time) *JimengVideoTask {
	groupID := int64(9)
	task := &JimengVideoTask{
		LocalTaskID:          localTaskID,
		TaskID:               upstreamTaskID,
		UserID:               1,
		APIKeyID:             2,
		GroupID:              &groupID,
		AccountID:            3,
		Model:                JimengVideoBillingModel,
		Status:               status,
		BillingStatus:        JimengVideoBillingStatusHeld,
		RequestHash:          "payload_hash",
		HoldID:               JimengVideoHoldRequestID(localTaskID),
		CaptureID:            JimengVideoCaptureRequestID(localTaskID),
		ReleaseID:            JimengVideoReleaseRequestID(localTaskID),
		EstimatedTotalCost:   5,
		HoldAmount:           5,
		Currency:             "USD",
		VideoDurationSeconds: 5,
		VideoResolution:      "720p",
		CreatedAt:            createdAt,
		UpdatedAt:            createdAt,
	}
	if upstreamTaskID != "" {
		submittedAt := createdAt.Add(time.Second)
		task.SubmittedAt = &submittedAt
	}
	return task
}

type jimengVideoPollerRepoStub struct {
	claimTasks      []*JimengVideoTask
	claimErr        error
	markStatusErr   error
	submitFailedErr error
	releaseLeaseErr error
	claimParams     []ClaimJimengVideoTasksForPollingParams
	statusUpdates   []MarkJimengVideoStatusParams
	submitFailures  []struct {
		localTaskID string
		code        string
		message     string
	}
	releaseLeases []struct {
		localTaskID string
		leaseOwner  string
	}
}

func (r *jimengVideoPollerRepoStub) ClaimJimengVideoTasksForPolling(_ context.Context, params ClaimJimengVideoTasksForPollingParams) ([]*JimengVideoTask, error) {
	r.claimParams = append(r.claimParams, params)
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	return r.claimTasks, nil
}

func (r *jimengVideoPollerRepoStub) ReleaseJimengVideoTaskPollLease(_ context.Context, localTaskID string, leaseOwner string) error {
	r.releaseLeases = append(r.releaseLeases, struct {
		localTaskID string
		leaseOwner  string
	}{localTaskID: localTaskID, leaseOwner: leaseOwner})
	return r.releaseLeaseErr
}

func (r *jimengVideoPollerRepoStub) MarkJimengVideoTaskStatus(_ context.Context, params MarkJimengVideoStatusParams) (*JimengVideoTask, error) {
	r.statusUpdates = append(r.statusUpdates, params)
	if r.markStatusErr != nil {
		return nil, r.markStatusErr
	}
	for _, task := range r.claimTasks {
		if task.TaskID == params.TaskID {
			task.Status = params.Status
			task.ResponseStatus = params.ResponseStatus
			task.ResponseContentType = params.ResponseContentType
			task.ResponseBody = params.ResponseBody
			task.LastErrorCode = params.LastErrorCode
			task.LastErrorMessage = params.LastErrorMessage
			return task, nil
		}
	}
	return nil, ErrJimengVideoTaskNotFound
}

func (r *jimengVideoPollerRepoStub) MarkJimengVideoTaskSubmitFailed(_ context.Context, localTaskID string, code string, message string) error {
	r.submitFailures = append(r.submitFailures, struct {
		localTaskID string
		code        string
		message     string
	}{localTaskID: localTaskID, code: code, message: message})
	if r.submitFailedErr != nil {
		return r.submitFailedErr
	}
	for _, task := range r.claimTasks {
		if task.LocalTaskID == localTaskID {
			task.Status = JimengTaskStatusFailed
			task.BillingStatus = JimengVideoBillingStatusReleased
			return nil
		}
	}
	return ErrJimengVideoTaskNotFound
}

type jimengVideoPollerGatewayStub struct {
	forwardResult *OpenAIForwardResult
	forwardErr    error
	settlementErr error
	releaseErr    error
	forwards      []struct {
		accountID int64
		taskID    string
	}
	settlements  []*JimengVideoSettlementInput
	releaseHolds []*JimengVideoTask
}

func (g *jimengVideoPollerGatewayStub) ForwardJimengVideoBuffered(_ context.Context, _ *gin.Context, account *Account, _ JimengVideoEndpoint, taskID string, _ []byte) (*OpenAIForwardResult, error) {
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	g.forwards = append(g.forwards, struct {
		accountID int64
		taskID    string
	}{accountID: accountID, taskID: taskID})
	return g.forwardResult, g.forwardErr
}

func (g *jimengVideoPollerGatewayStub) SettleJimengVideoTask(_ context.Context, in *JimengVideoSettlementInput) error {
	g.settlements = append(g.settlements, in)
	return g.settlementErr
}

func (g *jimengVideoPollerGatewayStub) ReleaseJimengVideoBalanceHold(_ context.Context, task *JimengVideoTask, _ string) error {
	g.releaseHolds = append(g.releaseHolds, task)
	return g.releaseErr
}

type jimengVideoPollerAPIKeyStub struct {
	apiKey *APIKey
	err    error
}

func (s *jimengVideoPollerAPIKeyStub) GetByID(_ context.Context, _ int64) (*APIKey, error) {
	return s.apiKey, s.err
}

func (s *jimengVideoPollerAPIKeyStub) UpdateQuotaUsed(_ context.Context, _ int64, _ float64) error {
	return nil
}

func (s *jimengVideoPollerAPIKeyStub) UpdateRateLimitUsage(_ context.Context, _ int64, _ float64) error {
	return nil
}

type jimengVideoPollerAccountStub struct {
	account *Account
	err     error
}

func (s *jimengVideoPollerAccountStub) GetByID(_ context.Context, _ int64) (*Account, error) {
	return s.account, s.err
}

type jimengVideoPollerSubscriptionStub struct {
	sub *UserSubscription
	err error
}

func (s *jimengVideoPollerSubscriptionStub) GetActiveSubscription(_ context.Context, _ int64, _ int64) (*UserSubscription, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	return s.sub, nil
}

type jimengVideoPollerBalanceCacheStub struct {
	invalidatedUserIDs []int64
	err                error
}

func (s *jimengVideoPollerBalanceCacheStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.invalidatedUserIDs = append(s.invalidatedUserIDs, userID)
	return s.err
}
