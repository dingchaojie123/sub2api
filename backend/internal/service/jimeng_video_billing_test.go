package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettleJimengVideoTaskSucceededCapturesHoldAndAccountsWithoutBalanceDeduction(t *testing.T) {
	groupID := int64(9)
	task := &JimengVideoTask{
		LocalTaskID:          "vidtask_success",
		TaskID:               "task_success",
		UserID:               1,
		APIKeyID:             2,
		GroupID:              &groupID,
		AccountID:            3,
		Status:               JimengTaskStatusSucceeded,
		BillingStatus:        JimengVideoBillingStatusHeld,
		HoldID:               JimengVideoHoldRequestID("vidtask_success"),
		CaptureID:            JimengVideoCaptureRequestID("vidtask_success"),
		ReleaseID:            JimengVideoReleaseRequestID("vidtask_success"),
		EstimatedTotalCost:   2.5,
		HoldAmount:           5,
		VideoDurationSeconds: 5,
		VideoResolution:      "720p",
	}
	billingRepo := &jimengVideoBillingRepoStub{task: task}
	logRepo := &jimengVideoUsageLogRepoStub{inserted: true}
	svc := &OpenAIGatewayService{
		usageBillingRepo: billingRepo,
		usageLogRepo:     logRepo,
	}

	err := svc.SettleJimengVideoTask(context.Background(), &JimengVideoSettlementInput{
		Task:               task,
		FinalStatus:        JimengTaskStatusSucceeded,
		APIKey:             &APIKey{ID: 2, User: &User{ID: 1}, Quota: 100, RateLimit5h: 10},
		User:               &User{ID: 1},
		Account:            &Account{ID: 3, Type: AccountTypeAPIKey},
		RequestPayloadHash: "payload_hash",
		APIKeyService:      &jimengVideoQuotaUpdaterStub{},
		QuotaPlatform:      PlatformJimeng,
	})

	require.NoError(t, err)
	require.Len(t, billingRepo.captures, 1)
	require.Equal(t, task.CaptureID, billingRepo.captures[0].RequestID)
	require.Equal(t, task.HoldID, billingRepo.captures[0].HoldRequestID)
	require.InDelta(t, 5, billingRepo.captures[0].ActualAmount, 1e-12)

	require.Len(t, billingRepo.applyCommands, 1)
	accounting := billingRepo.applyCommands[0]
	require.Equal(t, JimengVideoAccountingRequestID(task.LocalTaskID), accounting.RequestID)
	require.Zero(t, accounting.BalanceCost, "final accounting must not deduct balance again")
	require.InDelta(t, 5, accounting.APIKeyQuotaCost, 1e-12)
	require.InDelta(t, 5, accounting.APIKeyRateLimitCost, 1e-12)

	require.Len(t, billingRepo.settled, 1)
	require.Equal(t, JimengVideoBillingStatusCaptured, billingRepo.settled[0].BillingStatus)
	require.InDelta(t, 5, billingRepo.settled[0].ActualCost, 1e-12)

	require.Equal(t, 1, logRepo.calls)
	require.NotNil(t, logRepo.lastLog)
	require.Equal(t, "task_success", logRepo.lastLog.RequestID)
	require.InDelta(t, 2.5, logRepo.lastLog.OutputCost, 1e-12)
	require.InDelta(t, 2.5, logRepo.lastLog.TotalCost, 1e-12)
	require.InDelta(t, 5, logRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, 2, logRepo.lastLog.RateMultiplier, 1e-12)
	require.Equal(t, BillingTypeBalance, logRepo.lastLog.BillingType)
}

func TestSettleJimengVideoTaskFailedReleasesHoldWithoutUsageAccounting(t *testing.T) {
	task := &JimengVideoTask{
		LocalTaskID:   "vidtask_failed",
		TaskID:        "task_failed",
		UserID:        1,
		APIKeyID:      2,
		AccountID:     3,
		Status:        JimengTaskStatusFailed,
		BillingStatus: JimengVideoBillingStatusHeld,
		HoldID:        JimengVideoHoldRequestID("vidtask_failed"),
		CaptureID:     JimengVideoCaptureRequestID("vidtask_failed"),
		ReleaseID:     JimengVideoReleaseRequestID("vidtask_failed"),
		HoldAmount:    5,
	}
	billingRepo := &jimengVideoBillingRepoStub{task: task}
	logRepo := &jimengVideoUsageLogRepoStub{inserted: true}
	svc := &OpenAIGatewayService{
		usageBillingRepo: billingRepo,
		usageLogRepo:     logRepo,
	}

	err := svc.SettleJimengVideoTask(context.Background(), &JimengVideoSettlementInput{
		Task:               task,
		FinalStatus:        JimengTaskStatusFailed,
		APIKey:             &APIKey{ID: 2, User: &User{ID: 1}},
		User:               &User{ID: 1},
		Account:            &Account{ID: 3, Type: AccountTypeAPIKey},
		RequestPayloadHash: "payload_hash",
	})

	require.NoError(t, err)
	require.Empty(t, billingRepo.captures)
	require.Len(t, billingRepo.releases, 1)
	require.Equal(t, task.ReleaseID, billingRepo.releases[0].RequestID)
	require.Equal(t, task.HoldID, billingRepo.releases[0].HoldRequestID)
	require.Empty(t, billingRepo.applyCommands)
	require.Zero(t, logRepo.calls)
	require.Len(t, billingRepo.settled, 1)
	require.Equal(t, JimengVideoBillingStatusReleased, billingRepo.settled[0].BillingStatus)
}

func TestSettleJimengVideoTaskSkipsDifferentInFlightSettlement(t *testing.T) {
	task := &JimengVideoTask{
		LocalTaskID:   "vidtask_inflight",
		TaskID:        "task_inflight",
		UserID:        1,
		APIKeyID:      2,
		AccountID:     3,
		Status:        JimengTaskStatusSucceeded,
		BillingStatus: JimengVideoBillingStatusSettling,
		HoldID:        JimengVideoHoldRequestID("vidtask_inflight"),
		CaptureID:     JimengVideoCaptureRequestID("vidtask_inflight"),
		ReleaseID:     JimengVideoReleaseRequestID("vidtask_inflight"),
		HoldAmount:    5,
	}
	billingRepo := &jimengVideoBillingRepoStub{task: task}
	svc := &OpenAIGatewayService{usageBillingRepo: billingRepo}

	err := svc.SettleJimengVideoTask(context.Background(), &JimengVideoSettlementInput{
		Task:        task,
		FinalStatus: JimengTaskStatusFailed,
	})

	require.NoError(t, err)
	require.Empty(t, billingRepo.captures)
	require.Empty(t, billingRepo.releases)
	require.Empty(t, billingRepo.settled)
}

type jimengVideoBillingRepoStub struct {
	UsageBillingRepository

	task          *JimengVideoTask
	reserves      []*BatchImageBalanceHoldCommand
	captures      []*BatchImageBalanceHoldCommand
	releases      []*BatchImageBalanceHoldCommand
	applyCommands []*UsageBillingCommand
	settled       []MarkJimengVideoSettledParams
}

func (r *jimengVideoBillingRepoStub) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.applyCommands = append(r.applyCommands, cmd)
	return &UsageBillingApplyResult{Applied: true}, nil
}

func (r *jimengVideoBillingRepoStub) ReserveBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.reserves = append(r.reserves, cmd)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

func (r *jimengVideoBillingRepoStub) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.captures = append(r.captures, cmd)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

func (r *jimengVideoBillingRepoStub) ReleaseBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.releases = append(r.releases, cmd)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

func (r *jimengVideoBillingRepoStub) CreateJimengVideoTask(_ context.Context, params CreateJimengVideoTaskParams) (*JimengVideoTask, error) {
	task := &JimengVideoTask{
		LocalTaskID:          params.LocalTaskID,
		UserID:               params.UserID,
		APIKeyID:             params.APIKeyID,
		GroupID:              params.GroupID,
		AccountID:            params.AccountID,
		Model:                params.Model,
		Status:               params.Status,
		BillingStatus:        params.BillingStatus,
		RequestHash:          params.RequestHash,
		IdempotencyKey:       params.IdempotencyKey,
		HoldID:               params.HoldID,
		CaptureID:            params.CaptureID,
		ReleaseID:            params.ReleaseID,
		EstimatedTotalCost:   params.EstimatedTotalCost,
		HoldAmount:           params.HoldAmount,
		Currency:             params.Currency,
		VideoDurationSeconds: params.VideoDurationSeconds,
		VideoResolution:      params.VideoResolution,
	}
	r.task = task
	return task, nil
}

func (r *jimengVideoBillingRepoStub) GetJimengVideoTaskByIdempotencyKey(_ context.Context, _ int64, _ int64, _ string) (*JimengVideoTask, error) {
	if r.task == nil {
		return nil, ErrJimengVideoTaskNotFound
	}
	return r.task, nil
}

func (r *jimengVideoBillingRepoStub) GetJimengVideoTaskForOwner(_ context.Context, _ int64, _ int64, _ string) (*JimengVideoTask, error) {
	if r.task == nil {
		return nil, ErrJimengVideoTaskNotFound
	}
	return r.task, nil
}

func (r *jimengVideoBillingRepoStub) MarkJimengVideoTaskSubmitted(_ context.Context, params MarkJimengVideoSubmittedParams) (*JimengVideoTask, error) {
	if r.task == nil {
		return nil, ErrJimengVideoTaskNotFound
	}
	r.task.TaskID = params.TaskID
	r.task.Status = params.Status
	r.task.ResponseStatus = params.ResponseStatus
	r.task.ResponseContentType = params.ResponseContentType
	r.task.ResponseBody = params.ResponseBody
	return r.task, nil
}

func (r *jimengVideoBillingRepoStub) MarkJimengVideoTaskStatus(_ context.Context, params MarkJimengVideoStatusParams) (*JimengVideoTask, error) {
	if r.task == nil {
		return nil, ErrJimengVideoTaskNotFound
	}
	r.task.Status = params.Status
	r.task.ResponseStatus = params.ResponseStatus
	r.task.ResponseContentType = params.ResponseContentType
	r.task.ResponseBody = params.ResponseBody
	return r.task, nil
}

func (r *jimengVideoBillingRepoStub) MarkJimengVideoTaskSubmitFailed(_ context.Context, _ string, _ string, _ string) error {
	if r.task == nil {
		return ErrJimengVideoTaskNotFound
	}
	r.task.Status = JimengTaskStatusFailed
	r.task.BillingStatus = JimengVideoBillingStatusReleased
	return nil
}

func (r *jimengVideoBillingRepoStub) ClaimJimengVideoTaskSettlement(_ context.Context, params ClaimJimengVideoSettlementParams) (*JimengVideoTask, bool, error) {
	if r.task == nil {
		return nil, false, ErrJimengVideoTaskNotFound
	}
	if r.task.TaskID != params.TaskID {
		return nil, false, nil
	}
	if r.task.BillingStatus == JimengVideoBillingStatusCaptured || r.task.BillingStatus == JimengVideoBillingStatusReleased {
		return nil, false, nil
	}
	if (r.task.BillingStatus == JimengVideoBillingStatusSettling || r.task.BillingStatus == JimengVideoBillingStatusSettlingNone) && r.task.Status != params.FinalStatus {
		return nil, false, nil
	}
	if r.task.BillingStatus == JimengVideoBillingStatusNone || r.task.BillingStatus == JimengVideoBillingStatusSettlingNone {
		r.task.BillingStatus = JimengVideoBillingStatusSettlingNone
	} else {
		r.task.BillingStatus = JimengVideoBillingStatusSettling
	}
	r.task.Status = params.FinalStatus
	return r.task, true, nil
}

func (r *jimengVideoBillingRepoStub) MarkJimengVideoTaskSettled(_ context.Context, params MarkJimengVideoSettledParams) (*JimengVideoTask, error) {
	if r.task == nil {
		return nil, ErrJimengVideoTaskNotFound
	}
	r.settled = append(r.settled, params)
	r.task.Status = params.Status
	r.task.BillingStatus = params.BillingStatus
	r.task.ActualCost = &params.ActualCost
	return r.task, nil
}

type jimengVideoUsageLogRepoStub struct {
	UsageLogRepository

	inserted bool
	calls    int
	lastLog  *UsageLog
}

func (r *jimengVideoUsageLogRepoStub) Create(_ context.Context, log *UsageLog) (bool, error) {
	r.calls++
	r.lastLog = log
	return r.inserted, nil
}

type jimengVideoQuotaUpdaterStub struct{}

func (s *jimengVideoQuotaUpdaterStub) UpdateQuotaUsed(_ context.Context, _ int64, _ float64) error {
	return nil
}

func (s *jimengVideoQuotaUpdaterStub) UpdateRateLimitUsage(_ context.Context, _ int64, _ float64) error {
	return nil
}
