package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPPVideoPollerProcessingKeepsTaskPending(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	task := newPPVideoPollerTask("ppvidtask_processing", "provider-processing", PPVideoTaskStatusProcessing, now.Add(-time.Minute))
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{
		result: &OpenAIForwardResult{TaskStatus: PPVideoTaskStatusProcessing, ResponseStatusCode: 200},
	}
	poller := newPPVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, PPVideoPollerOutcomeProcessing, result.Outcome)
	require.Empty(t, gateway.settlements)
	require.Empty(t, gateway.releases)
	require.Equal(t, PPVideoTaskStatusProcessing, repo.task.Status)
}

func TestPPVideoPollerSuccessSettlesWithProviderDuration(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	task := newPPVideoPollerTask("ppvidtask_success", "provider-success", PPVideoTaskStatusProcessing, now.Add(-time.Minute))
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{
		result: &OpenAIForwardResult{
			TaskStatus:                PPVideoTaskStatusSucceeded,
			VideoDurationMilliseconds: 5041,
			ResponseStatusCode:        200,
		},
	}
	poller := newPPVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, PPVideoPollerOutcomeSucceeded, result.Outcome)
	require.Len(t, gateway.settlements, 1)
	require.Equal(t, PPVideoTaskStatusSucceeded, gateway.settlements[0].FinalStatus)
	require.Equal(t, int64(5041), gateway.settlements[0].Task.GeneratedVideoDurationMilliseconds)
}

func TestPPVideoPollerUpstreamErrorLeavesTaskForRetry(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	task := newPPVideoPollerTask("ppvidtask_retry", "provider-retry", PPVideoTaskStatusProcessing, now.Add(-time.Minute))
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{err: errors.New("upstream unavailable")}
	poller := newPPVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.Error(t, err)
	require.Equal(t, PPVideoPollerOutcomeRetry, result.Outcome)
	require.Empty(t, gateway.settlements)
	require.Empty(t, gateway.releases)
}

func TestPPVideoPollerSubmitTimeoutReleasesHold(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	task := newPPVideoPollerTask("ppvidtask_timeout", "", PPVideoTaskStatusSubmitting, now.Add(-11*time.Minute))
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{}
	poller := newPPVideoPollerTestService(repo, gateway, now)

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, PPVideoPollerOutcomeTimedOut, result.Outcome)
	require.Len(t, gateway.releases, 1)
	require.Len(t, repo.submitFailures, 1)
	require.Equal(t, "submit_timeout", repo.submitFailures[0].code)
}

type ppVideoPollerGatewayStub struct {
	result      *OpenAIForwardResult
	err         error
	settlements []*PPVideoSettlementInput
	releases    []*PPVideoTask
}

func (g *ppVideoPollerGatewayStub) ForwardPPVideoBuffered(_ context.Context, _ *gin.Context, _ *Account, _ PPVideoOperation, _ string, _ []byte, _ ...PPVideoPublicRequest) (*OpenAIForwardResult, error) {
	if g.err != nil {
		return nil, g.err
	}
	return g.result, nil
}

func (g *ppVideoPollerGatewayStub) SettlePPVideoTask(_ context.Context, in *PPVideoSettlementInput) error {
	g.settlements = append(g.settlements, in)
	return nil
}

func (g *ppVideoPollerGatewayStub) ReleasePPVideoBalanceHold(_ context.Context, task *PPVideoTask, _ string) error {
	g.releases = append(g.releases, task)
	return nil
}

type ppVideoPollerRepoStub struct {
	PPVideoTaskRepository
	task           *PPVideoTask
	statusUpdates  []MarkPPVideoTaskStatusParams
	submitFailures []struct {
		localTaskID string
		code        string
		message     string
	}
}

func (r *ppVideoPollerRepoStub) MarkPPVideoTaskStatus(_ context.Context, params MarkPPVideoTaskStatusParams) (*PPVideoTask, error) {
	r.statusUpdates = append(r.statusUpdates, params)
	r.task.Status = params.Status
	if params.GeneratedVideoDurationMilliseconds > 0 {
		r.task.GeneratedVideoDurationMilliseconds = params.GeneratedVideoDurationMilliseconds
	}
	return r.task, nil
}

func (r *ppVideoPollerRepoStub) MarkPPVideoTaskSubmitFailed(_ context.Context, localTaskID, code, message string) error {
	r.submitFailures = append(r.submitFailures, struct {
		localTaskID string
		code        string
		message     string
	}{localTaskID: localTaskID, code: code, message: message})
	r.task.Status = PPVideoTaskStatusFailed
	return nil
}

func newPPVideoPollerTestService(repo *ppVideoPollerRepoStub, gateway *ppVideoPollerGatewayStub, now time.Time) *PPVideoPollerService {
	return &PPVideoPollerService{
		Repo:       repo,
		Gateway:    gateway,
		APIKeys:    &jimengVideoPollerAPIKeyStub{apiKey: &APIKey{ID: 2, UserID: 1, User: &User{ID: 1}}},
		Accounts:   &jimengVideoPollerAccountStub{account: &Account{ID: 3, Platform: PlatformKling, Type: AccountTypeAPIKey, Concurrency: 1}},
		Options:    PPVideoPollerOptions{BatchSize: 1, LeaseTTL: time.Minute, UpstreamTimeout: time.Second, SubmitTimeout: 10 * time.Minute, MaxProcessingAge: time.Hour},
		LeaseOwner: "pp-test-worker",
		Now:        func() time.Time { return now },
	}
}

func newPPVideoPollerTask(localTaskID, taskID, status string, createdAt time.Time) *PPVideoTask {
	groupID := int64(9)
	submittedAt := createdAt.Add(time.Second)
	return &PPVideoTask{
		LocalTaskID:                   localTaskID,
		TaskID:                        taskID,
		UserID:                        1,
		APIKeyID:                      2,
		GroupID:                       &groupID,
		AccountID:                     3,
		Platform:                      PlatformKling,
		Operation:                     PPVideoOperationGeneric,
		Model:                         "kling-v3",
		Status:                        status,
		BillingStatus:                 PPVideoBillingStatusHeld,
		RequestHash:                   "payload-hash",
		HoldID:                        PPVideoHoldRequestID(localTaskID),
		CaptureID:                     PPVideoCaptureRequestID(localTaskID),
		ReleaseID:                     PPVideoReleaseRequestID(localTaskID),
		EstimatedTotalCost:            5,
		HoldAmount:                    5,
		RequestedVideoDurationSeconds: 5,
		VideoCount:                    1,
		VideoResolution:               VideoBillingResolution720P,
		CreatedAt:                     createdAt,
		UpdatedAt:                     createdAt,
		SubmittedAt:                   &submittedAt,
	}
}
