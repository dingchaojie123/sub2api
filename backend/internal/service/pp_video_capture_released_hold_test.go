package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// ppVideoReleasedHoldRepoStub behaves like the real repository for a task whose hold was already
// released before the task succeeded: a capture is refused with ErrBatchImageHoldAlreadyReleased.
type ppVideoReleasedHoldRepoStub struct {
	*ppVideoBillingRepoStub
}

func (r *ppVideoReleasedHoldRepoStub) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.captures = append(r.captures, cmd)
	return nil, ErrBatchImageHoldAlreadyReleased
}

func newPPVideoSettlingSucceededTask() *PPVideoTask {
	return &PPVideoTask{
		LocalTaskID:                        "ppvidtask_released_before_success",
		TaskID:                             "pp-task-released-before-success",
		UserID:                             1,
		APIKeyID:                           2,
		AccountID:                          3,
		Platform:                           PlatformSeedance,
		Model:                              "seedance-2.0",
		Status:                             PPVideoTaskStatusSucceeded,
		BillingStatus:                      PPVideoBillingStatusSettling,
		EstimatedTotalCost:                 2,
		HoldAmount:                         2,
		ReleaseID:                          "pp_video_release:ppvidtask_released_before_success",
		RequestedVideoDurationSeconds:      4,
		RequestedVideoDurationMilliseconds: 4000,
		GeneratedVideoDurationMilliseconds: 4000,
		VideoCount:                         1,
		VideoResolution:                    VideoBillingResolution720P,
		ResponseBody:                       `{"status":"succeeded","video_url":"https://cdn.example.com/seedance.mp4"}`,
	}
}

// A task whose hold was already released (the money is back with the user) must not be captured
// again, and must not record usage. It is closed as released so the poller stops retrying it.
func TestSettlePPVideoTaskClosesTaskWhoseHoldWasAlreadyReleased(t *testing.T) {
	task := newPPVideoSettlingSucceededTask()
	base := &ppVideoBillingRepoStub{task: task, claim: false}
	repo := &ppVideoReleasedHoldRepoStub{ppVideoBillingRepoStub: base}
	svc := &OpenAIGatewayService{usageBillingRepo: repo}

	err := svc.SettlePPVideoTask(context.Background(), &PPVideoSettlementInput{
		Task:        task,
		FinalStatus: PPVideoTaskStatusSucceeded,
		APIKey:      &APIKey{ID: 2, User: &User{ID: 1}},
		User:        &User{ID: 1},
		Account:     &Account{ID: 3, Type: AccountTypeAPIKey},
	})

	require.NoError(t, err)
	require.Len(t, base.captures, 1)
	require.Equal(t, task.ReleaseID, base.captures[0].ReleaseRequestID, "capture must carry the release id so the repository can refuse it")
	require.Empty(t, base.applyCommands, "no usage may be recorded for a task that was refunded")
	require.Len(t, base.settled, 1)
	require.Equal(t, PPVideoBillingStatusReleased, base.settled[0].BillingStatus)
	require.Equal(t, PPVideoTaskStatusSucceeded, base.settled[0].Status)
	require.Zero(t, base.settled[0].ActualCost)
}

// Any other capture failure must keep the task in the settling state so it is retried.
func TestSettlePPVideoTaskKeepsRetryingOtherCaptureFailures(t *testing.T) {
	task := newPPVideoSettlingSucceededTask()
	base := &ppVideoBillingRepoStub{task: task, claim: false}
	repo := &ppVideoCaptureFailsRepoStub{ppVideoBillingRepoStub: base, err: errors.New("db down")}
	svc := &OpenAIGatewayService{usageBillingRepo: repo}

	err := svc.SettlePPVideoTask(context.Background(), &PPVideoSettlementInput{
		Task:        task,
		FinalStatus: PPVideoTaskStatusSucceeded,
		APIKey:      &APIKey{ID: 2, User: &User{ID: 1}},
		User:        &User{ID: 1},
		Account:     &Account{ID: 3, Type: AccountTypeAPIKey},
	})

	require.Error(t, err)
	require.Empty(t, base.settled, "the task must not be closed on an ordinary failure")
	require.Equal(t, PPVideoBillingStatusSettling, task.BillingStatus)
}

type ppVideoCaptureFailsRepoStub struct {
	*ppVideoBillingRepoStub
	err error
}

func (r *ppVideoCaptureFailsRepoStub) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.captures = append(r.captures, cmd)
	return nil, r.err
}

// An ordinary (not yet released) task still captures normally and carries its release id.
func TestSettlePPVideoTaskCaptureCarriesReleaseID(t *testing.T) {
	task := newPPVideoSettlingSucceededTask()
	repo := &ppVideoBillingRepoStub{task: task, claim: false}
	svc := &OpenAIGatewayService{usageBillingRepo: repo}

	err := svc.SettlePPVideoTask(context.Background(), &PPVideoSettlementInput{
		Task:        task,
		FinalStatus: PPVideoTaskStatusSucceeded,
		APIKey:      &APIKey{ID: 2, User: &User{ID: 1}},
		User:        &User{ID: 1},
		Account:     &Account{ID: 3, Type: AccountTypeAPIKey},
	})

	require.NoError(t, err)
	require.Len(t, repo.captures, 1)
	require.Equal(t, task.ReleaseID, repo.captures[0].ReleaseRequestID)
	require.Len(t, repo.settled, 1)
	require.Equal(t, PPVideoBillingStatusCaptured, repo.settled[0].BillingStatus)
}

// ---- API key loading for settlement ----

type ppVideoAPIKeyOnlyStub struct {
	key *APIKey
	err error
}

func (s *ppVideoAPIKeyOnlyStub) GetByID(context.Context, int64) (*APIKey, error) {
	return s.key, s.err
}

type ppVideoAPIKeyWithDeletedStub struct {
	ppVideoAPIKeyOnlyStub
	deleted      *APIKey
	deletedErr   error
	deletedCalls int
}

func (s *ppVideoAPIKeyWithDeletedStub) GetByIDIncludingDeleted(context.Context, int64) (*APIKey, error) {
	s.deletedCalls++
	return s.deleted, s.deletedErr
}

func TestPPVideoPollerLoadAPIKey(t *testing.T) {
	notFound := fmt.Errorf("get api key: %w", ErrAPIKeyNotFound)

	t.Run("active key is used as is", func(t *testing.T) {
		stub := &ppVideoAPIKeyWithDeletedStub{ppVideoAPIKeyOnlyStub: ppVideoAPIKeyOnlyStub{key: &APIKey{ID: 7}}}
		got, err := (&PPVideoPollerService{APIKeys: stub}).loadAPIKey(context.Background(), 7)
		require.NoError(t, err)
		require.Equal(t, int64(7), got.ID)
		require.Zero(t, stub.deletedCalls)
	})

	t.Run("a deleted key is loaded for settlement", func(t *testing.T) {
		stub := &ppVideoAPIKeyWithDeletedStub{
			ppVideoAPIKeyOnlyStub: ppVideoAPIKeyOnlyStub{err: notFound},
			deleted:               &APIKey{ID: 7},
		}
		got, err := (&PPVideoPollerService{APIKeys: stub}).loadAPIKey(context.Background(), 7)
		require.NoError(t, err)
		require.Equal(t, int64(7), got.ID)
		require.Equal(t, 1, stub.deletedCalls)
	})

	t.Run("a key that cannot be found even among deleted ones still fails", func(t *testing.T) {
		stub := &ppVideoAPIKeyWithDeletedStub{
			ppVideoAPIKeyOnlyStub: ppVideoAPIKeyOnlyStub{err: notFound},
			deletedErr:            ErrAPIKeyNotFound,
		}
		_, err := (&PPVideoPollerService{APIKeys: stub}).loadAPIKey(context.Background(), 7)
		require.ErrorIs(t, err, ErrAPIKeyNotFound)
	})

	t.Run("other errors do not fall back", func(t *testing.T) {
		stub := &ppVideoAPIKeyWithDeletedStub{
			ppVideoAPIKeyOnlyStub: ppVideoAPIKeyOnlyStub{err: errors.New("db down")},
			deleted:               &APIKey{ID: 7},
		}
		_, err := (&PPVideoPollerService{APIKeys: stub}).loadAPIKey(context.Background(), 7)
		require.EqualError(t, err, "db down")
		require.Zero(t, stub.deletedCalls)
	})

	t.Run("a loader without the deleted capability keeps the old behaviour", func(t *testing.T) {
		stub := &ppVideoAPIKeyOnlyStub{err: notFound}
		_, err := (&PPVideoPollerService{APIKeys: stub}).loadAPIKey(context.Background(), 7)
		require.ErrorIs(t, err, ErrAPIKeyNotFound)
	})
}
