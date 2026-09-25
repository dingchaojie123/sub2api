package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	PPVideoTaskStatusSubmitting    = "submitting"
	PPVideoIdempotencyKeyMaxLength = 255

	PPVideoBillingStatusHeld         = "held"
	PPVideoBillingStatusSettling     = "settling"
	PPVideoBillingStatusSettlingNone = "settling_none"
	PPVideoBillingStatusCaptured     = "captured"
	PPVideoBillingStatusReleased     = "released"
	PPVideoBillingStatusNone         = "none"

	ppVideoHoldRequestPrefix    = "pp_video_hold:"
	ppVideoCaptureRequestPrefix = "pp_video_capture:"
	ppVideoReleaseRequestPrefix = "pp_video_release:"
	ppVideoAccountingPrefix     = "pp_video_accounting:"
)

var (
	ErrPPVideoTaskNotFound          = infraerrors.New(http.StatusNotFound, "PP_VIDEO_TASK_NOT_FOUND", "video request not found")
	ErrPPVideoTaskRepositoryMissing = infraerrors.New(http.StatusServiceUnavailable, "PP_VIDEO_TASK_REPOSITORY_MISSING", "PP video task repository is not configured")
	ErrPPVideoIdempotencyConflict   = infraerrors.New(http.StatusConflict, "PP_VIDEO_IDEMPOTENCY_CONFLICT", "idempotency key reused with different video request")
)

type PPVideoTask struct {
	ID                                 int64
	LocalTaskID                        string
	TaskID                             string
	UserID                             int64
	APIKeyID                           int64
	GroupID                            *int64
	AccountID                          int64
	Platform                           string
	Operation                          PPVideoOperation
	Model                              string
	Status                             string
	BillingStatus                      string
	RequestHash                        string
	IdempotencyKey                     string
	RequestedVideoDurationSeconds      int
	RequestedVideoDurationMilliseconds int64
	InputVideoDurationMilliseconds     int64
	GeneratedVideoDurationMilliseconds int64
	VideoCount                         int
	VideoResolution                    string
	OutputWidth                        int
	OutputHeight                       int
	FrameRate                          float64
	HasAudio                           bool
	KlingMode                          string
	BillingFormula                     string
	BillingUnits                       float64
	BillingUnitPrice                   float64
	BillingFallbackUnitPrice           float64
	HoldID                             string
	CaptureID                          string
	ReleaseID                          string
	EstimatedTotalCost                 float64
	HoldAmount                         float64
	ActualCost                         *float64
	Currency                           string
	ResponseStatus                     int
	ResponseContentType                string
	ResponseBody                       string
	LastErrorCode                      string
	LastErrorMessage                   string
	PollLeaseToken                     string
	PollLeaseOwner                     string
	PollLeaseUntil                     *time.Time
	PollAttempts                       int
	LastPolledAt                       *time.Time
	CreatedAt                          time.Time
	UpdatedAt                          time.Time
	SubmittedAt                        *time.Time
	FinishedAt                         *time.Time
	SettledAt                          *time.Time
}

type CreatePPVideoTaskParams struct {
	LocalTaskID                        string
	UserID                             int64
	APIKeyID                           int64
	GroupID                            *int64
	AccountID                          int64
	Platform                           string
	Operation                          PPVideoOperation
	Model                              string
	Status                             string
	BillingStatus                      string
	RequestHash                        string
	IdempotencyKey                     string
	RequestedVideoDurationSeconds      int
	RequestedVideoDurationMilliseconds int64
	InputVideoDurationMilliseconds     int64
	VideoCount                         int
	VideoResolution                    string
	OutputWidth                        int
	OutputHeight                       int
	FrameRate                          float64
	HasAudio                           bool
	KlingMode                          string
	BillingFormula                     string
	BillingUnits                       float64
	BillingUnitPrice                   float64
	BillingFallbackUnitPrice           float64
	HoldID                             string
	CaptureID                          string
	ReleaseID                          string
	EstimatedTotalCost                 float64
	HoldAmount                         float64
	Currency                           string
}

type MarkPPVideoTaskSubmittedParams struct {
	LocalTaskID         string
	TaskID              string
	Status              string
	ResponseStatus      int
	ResponseContentType string
	ResponseBody        string
}

type MarkPPVideoTaskStatusParams struct {
	TaskID                             string
	Status                             string
	GeneratedVideoDurationMilliseconds int64
	VideoCount                         int
	VideoResolution                    string
	OutputWidth                        int
	OutputHeight                       int
	FrameRate                          float64
	InputVideoDurationMilliseconds     int64
	ResponseStatus                     int
	ResponseContentType                string
	ResponseBody                       string
	LastErrorCode                      string
	LastErrorMessage                   string
}

type ClaimPPVideoTaskSettlementParams struct {
	TaskID      string
	FinalStatus string
}

type MarkPPVideoTaskSettledParams struct {
	TaskID           string
	Status           string
	BillingStatus    string
	ActualCost       float64
	LastErrorCode    string
	LastErrorMessage string
}

type ClaimPPVideoTasksForPollingParams struct {
	LeaseOwner   string
	LeaseToken   string
	LeaseUntil   time.Time
	SubmitCutoff time.Time
	Limit        int
}

type ReleasePPVideoTaskPollLeaseParams struct {
	LocalTaskID string
	LeaseOwner  string
	LeaseToken  string
}

type PPVideoTaskRepository interface {
	CreatePPVideoTask(context.Context, CreatePPVideoTaskParams) (*PPVideoTask, error)
	GetPPVideoTaskByIdempotencyKey(context.Context, int64, int64, string) (*PPVideoTask, error)
	GetPPVideoTaskForOwner(context.Context, int64, int64, string) (*PPVideoTask, error)
	MarkPPVideoTaskSubmitted(context.Context, MarkPPVideoTaskSubmittedParams) (*PPVideoTask, error)
	MarkPPVideoTaskStatus(context.Context, MarkPPVideoTaskStatusParams) (*PPVideoTask, error)
	MarkPPVideoTaskSubmitFailed(context.Context, string, string, string) error
	ClaimPPVideoTaskSettlement(context.Context, ClaimPPVideoTaskSettlementParams) (*PPVideoTask, bool, error)
	MarkPPVideoTaskSettled(context.Context, MarkPPVideoTaskSettledParams) (*PPVideoTask, error)
	ClaimPPVideoTasksForPolling(context.Context, ClaimPPVideoTasksForPollingParams) ([]*PPVideoTask, error)
	ReleasePPVideoTaskPollLease(context.Context, ReleasePPVideoTaskPollLeaseParams) error
}

func NewPPVideoLocalTaskID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "ppvidtask_" + hex.EncodeToString(b[:]), nil
}

func NewPPVideoPollLeaseToken() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "ppvidlease_" + hex.EncodeToString(b[:]), nil
}

func PPVideoHoldRequestID(localTaskID string) string {
	return ppVideoHoldRequestPrefix + strings.TrimSpace(localTaskID)
}

func PPVideoCaptureRequestID(localTaskID string) string {
	return ppVideoCaptureRequestPrefix + strings.TrimSpace(localTaskID)
}

func PPVideoReleaseRequestID(localTaskID string) string {
	return ppVideoReleaseRequestPrefix + strings.TrimSpace(localTaskID)
}

func PPVideoAccountingRequestID(localTaskID string) string {
	return ppVideoAccountingPrefix + strings.TrimSpace(localTaskID)
}

func IsTerminalPPVideoTaskStatus(status string) bool {
	switch NormalizePPVideoTaskStatus(status) {
	case PPVideoTaskStatusSucceeded, PPVideoTaskStatusFailed:
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) ppVideoTaskRepo() (PPVideoTaskRepository, error) {
	if s == nil || s.usageBillingRepo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	repo, ok := s.usageBillingRepo.(PPVideoTaskRepository)
	if !ok || repo == nil {
		return nil, ErrPPVideoTaskRepositoryMissing
	}
	return repo, nil
}

func (s *OpenAIGatewayService) CreatePPVideoTask(ctx context.Context, params CreatePPVideoTaskParams) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.CreatePPVideoTask(ctx, params)
}

func (s *OpenAIGatewayService) GetPPVideoTaskByIdempotencyKey(ctx context.Context, userID int64, apiKeyID int64, key string) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPPVideoTaskByIdempotencyKey(ctx, userID, apiKeyID, key)
}

func (s *OpenAIGatewayService) GetPPVideoTaskForOwner(ctx context.Context, userID int64, apiKeyID int64, taskID string) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPPVideoTaskForOwner(ctx, userID, apiKeyID, taskID)
}

func (s *OpenAIGatewayService) MarkPPVideoTaskSubmitted(ctx context.Context, params MarkPPVideoTaskSubmittedParams) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.MarkPPVideoTaskSubmitted(ctx, params)
}

func (s *OpenAIGatewayService) MarkPPVideoTaskStatus(ctx context.Context, params MarkPPVideoTaskStatusParams) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.MarkPPVideoTaskStatus(ctx, params)
}

func (s *OpenAIGatewayService) MarkPPVideoTaskSubmitFailed(ctx context.Context, localTaskID string, code string, message string) error {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return err
	}
	return repo.MarkPPVideoTaskSubmitFailed(ctx, localTaskID, code, message)
}

func (s *OpenAIGatewayService) ClaimPPVideoTaskSettlement(ctx context.Context, params ClaimPPVideoTaskSettlementParams) (*PPVideoTask, bool, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, false, err
	}
	return repo.ClaimPPVideoTaskSettlement(ctx, params)
}

func (s *OpenAIGatewayService) MarkPPVideoTaskSettled(ctx context.Context, params MarkPPVideoTaskSettledParams) (*PPVideoTask, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.MarkPPVideoTaskSettled(ctx, params)
}

func (s *OpenAIGatewayService) ClaimPPVideoTasksForPolling(ctx context.Context, params ClaimPPVideoTasksForPollingParams) ([]*PPVideoTask, error) {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return nil, err
	}
	return repo.ClaimPPVideoTasksForPolling(ctx, params)
}

func (s *OpenAIGatewayService) ReleasePPVideoTaskPollLease(ctx context.Context, params ReleasePPVideoTaskPollLeaseParams) error {
	repo, err := s.ppVideoTaskRepo()
	if err != nil {
		return err
	}
	return repo.ReleasePPVideoTaskPollLease(ctx, params)
}
