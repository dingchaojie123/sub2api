package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The upstream account of a task that already succeeded may have been deleted (for example when a whole
// channel is taken down). Settlement must still go ahead, using an account built from the task, instead
// of retrying forever.
func TestPPVideoPollerSettlesSucceededTaskWhoseAccountWasDeleted(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	task := newPPVideoPollerTask("ppvidtask_deleted_account", "provider-deleted-account", PPVideoTaskStatusSucceeded, now.Add(-time.Hour))
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{}
	poller := newPPVideoPollerTestService(repo, gateway, now)
	poller.Accounts = &jimengVideoPollerAccountStub{err: ErrAccountNotFound.WithCause(errors.New("ent: account not found"))}

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, PPVideoPollerOutcomeSucceeded, result.Outcome)
	require.Len(t, gateway.settlements, 1)
	settled := gateway.settlements[0]
	require.Equal(t, PPVideoTaskStatusSucceeded, settled.FinalStatus)
	require.NotNil(t, settled.Account)
	require.Equal(t, task.AccountID, settled.Account.ID)
	require.Equal(t, task.Platform, settled.Account.Platform)
	require.Equal(t, 1.0, settled.Account.BillingRateMultiplier(), "the stand-in account must not change the price")
}

// Any other failure while loading the account is still retried.
func TestPPVideoPollerRetriesWhenTheAccountLoadFailsForOtherReasons(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	task := newPPVideoPollerTask("ppvidtask_account_error", "provider-account-error", PPVideoTaskStatusSucceeded, now.Add(-time.Hour))
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{}
	poller := newPPVideoPollerTestService(repo, gateway, now)
	poller.Accounts = &jimengVideoPollerAccountStub{err: errors.New("db down")}

	result, err := poller.ProcessTask(context.Background(), task)

	require.EqualError(t, err, "db down")
	require.Equal(t, PPVideoPollerOutcomeRetry, result.Outcome)
	require.Empty(t, gateway.settlements)
}

// A failed task is released without an account, as before.
func TestPPVideoPollerReleasesFailedTaskWhoseAccountWasDeleted(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	task := newPPVideoPollerTask("ppvidtask_failed_no_account", "provider-failed-no-account", PPVideoTaskStatusFailed, now.Add(-time.Hour))
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{}
	poller := newPPVideoPollerTestService(repo, gateway, now)
	poller.Accounts = &jimengVideoPollerAccountStub{err: ErrAccountNotFound}

	result, err := poller.ProcessTask(context.Background(), task)

	require.NoError(t, err)
	require.Equal(t, PPVideoPollerOutcomeFailed, result.Outcome)
	require.Len(t, gateway.settlements, 1)
	require.Nil(t, gateway.settlements[0].Account)
}
