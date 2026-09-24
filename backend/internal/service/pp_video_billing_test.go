package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestCalculatePPVideoCostUsesGroupVideoPriceForSeedance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		resolution      string
		durationMs      int64
		videoCount      int
		inputDurationMs int64
		outputWidth     int
		outputHeight    int
		frameRate       float64
		wantRate        float64
	}{
		{
			name:            "480p",
			resolution:      VideoBillingResolution480P,
			durationMs:      4000,
			videoCount:      2,
			inputDurationMs: 2000,
			outputWidth:     640,
			outputHeight:    480,
			frameRate:       30,
			wantRate:        0.1,
		},
		{
			name:            "720p",
			resolution:      VideoBillingResolution720P,
			durationMs:      5000,
			videoCount:      3,
			inputDurationMs: 8000,
			outputWidth:     1280,
			outputHeight:    720,
			frameRate:       60,
			wantRate:        0.2,
		},
		{
			name:            "1080p",
			resolution:      VideoBillingResolution1080P,
			durationMs:      6000,
			videoCount:      1,
			inputDurationMs: 1000,
			outputWidth:     1920,
			outputHeight:    1080,
			frameRate:       24,
			wantRate:        0.4,
		},
		{
			name:            "4k maps to 1080p",
			resolution:      "4k",
			durationMs:      7000,
			videoCount:      2,
			inputDurationMs: 99000,
			outputWidth:     4096,
			outputHeight:    2160,
			frameRate:       120,
			wantRate:        0.4,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			groupID := int64(100)
			svc := &OpenAIGatewayService{
				billingService: newTestBillingService(),
			}
			apiKey := &APIKey{
				GroupID: &groupID,
				Group: &Group{
					ID:              groupID,
					Platform:        PlatformSeedance,
					RateMultiplier:  1,
					VideoPrice480P:  func() *float64 { v := 0.1; return &v }(),
					VideoPrice720P:  func() *float64 { v := 0.2; return &v }(),
					VideoPrice1080P: func() *float64 { v := 0.4; return &v }(),
				},
			}
			meta := PPVideoBillingMetadata{
				Platform:                       PlatformSeedance,
				Model:                          "doubao-seedance-2-0-mini-260615",
				RequestedDurationMilliseconds:  tt.durationMs,
				InputVideoDurationMilliseconds: tt.inputDurationMs,
				VideoCount:                     tt.videoCount,
				VideoResolution:                tt.resolution,
				OutputWidth:                    tt.outputWidth,
				OutputHeight:                   tt.outputHeight,
				FrameRate:                      tt.frameRate,
			}

			cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, meta)

			require.NoError(t, err)
			require.Equal(t, PPVideoBillingFormulaPerSecond, cost.BillingFormula)
			require.InDelta(t, float64(tt.durationMs)/1000*float64(tt.videoCount), cost.BillingUnits, 1e-12)
			require.InDelta(t, tt.wantRate, cost.BillingUnitPrice, 1e-12)
			require.InDelta(t, float64(tt.durationMs)/1000*float64(tt.videoCount)*tt.wantRate, cost.TotalCost, 1e-12)
			require.InDelta(t, cost.TotalCost, cost.OutputCost, 1e-12)
		})
	}
}

func TestCalculatePPVideoCostValidatesByteDanceDurationsByModel(t *testing.T) {
	t.Parallel()

	groupID := int64(102)
	videoPrice := 0.2
	apiKey := &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:             groupID,
			Platform:       PlatformByteDance,
			RateMultiplier: 1,
			VideoPrice720P: &videoPrice,
		},
	}
	svc := &OpenAIGatewayService{billingService: newTestBillingService()}

	tests := []struct {
		name      string
		model     string
		duration  int64
		wantRate  float64
		wantErr   string
	}{
		{
			name:      "Seedance 2.0 rejects 30 seconds",
			model:     ByteDanceVideoDefaultModel,
			duration:  30000,
			wantErr:   "from 4 to 15",
		},
		{
			name:     "Seedance 2.0 keeps the configured unit price",
			model:    ByteDanceVideoDefaultModel,
			duration: 15000,
			wantRate: 0.2,
		},
		{
			name:     "Seedance 2.5 accepts 30 seconds with three times unit price",
			model:    ByteDanceSeedance25Model,
			duration: 30000,
			wantRate: 0.6,
		},
		{
			name:     "Seedance 2.5 Global accepts 30 seconds with three times unit price",
			model:    ByteDanceSeedance25GlobalModel,
			duration: 30000,
			wantRate: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
				Platform:                      PlatformByteDance,
				Model:                         tt.model,
				RequestedDurationMilliseconds: tt.duration,
				VideoCount:                    1,
				VideoResolution:               VideoBillingResolution720P,
			})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			expectedUnits := float64(tt.duration) / 1000
			require.InDelta(t, expectedUnits, cost.BillingUnits, 1e-12)
			require.InDelta(t, tt.wantRate, cost.BillingUnitPrice, 1e-12)
			require.InDelta(t, expectedUnits*tt.wantRate, cost.TotalCost, 1e-12)
		})
	}
}

func TestCalculatePPVideoCostUsesGroupVideoPriceForKling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mode       string
		hasAudio   bool
		resolution string
		wantRate   float64
	}{
		{name: "2x ignores upstream audio rate", mode: "std", hasAudio: true, resolution: VideoBillingResolution720P, wantRate: 0.2},
		{name: "2x pro uses 1080p group rate", mode: "pro", hasAudio: false, resolution: VideoBillingResolution1080P, wantRate: 0.4},
		{name: "4k uses highest existing group tier", mode: "4k", hasAudio: true, resolution: "4k", wantRate: 0.4},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			groupID := int64(101)
			svc := &OpenAIGatewayService{
				billingService: newTestBillingService(),
			}
			apiKey := &APIKey{
				GroupID: &groupID,
				Group: &Group{
					ID:              groupID,
					RateMultiplier:  1,
					VideoPrice720P:  func() *float64 { v := 0.2; return &v }(),
					VideoPrice1080P: func() *float64 { v := 0.4; return &v }(),
				},
			}
			meta := PPVideoBillingMetadata{
				Platform:                      PlatformKling,
				KlingMode:                     tt.mode,
				HasAudio:                      tt.hasAudio,
				VideoResolution:               tt.resolution,
				RequestedDurationMilliseconds: 10000,
				VideoCount:                    1,
			}

			cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, meta)

			require.NoError(t, err)
			require.Equal(t, PPVideoBillingFormulaPerSecond, cost.BillingFormula)
			require.InDelta(t, 10, cost.BillingUnits, 1e-12)
			require.InDelta(t, tt.wantRate, cost.BillingUnitPrice, 1e-12)
			require.InDelta(t, 10*tt.wantRate, cost.TotalCost, 1e-12)
			require.InDelta(t, cost.TotalCost, cost.OutputCost, 1e-12)
		})
	}
}

func TestPPVideoGroupBillingResolutionMapsMiniMaxH3Tiers(t *testing.T) {
	t.Parallel()

	require.Equal(t, VideoBillingResolution720P, ppVideoGroupBillingResolution(PPVideoBillingMetadata{
		Platform:        PlatformMiniMaxH3,
		VideoResolution: "768P",
	}))
	require.Equal(t, VideoBillingResolution1080P, ppVideoGroupBillingResolution(PPVideoBillingMetadata{
		Platform:        PlatformMiniMaxH3,
		VideoResolution: "2K",
	}))
}

func TestCalculatePPVideoCostValidatesMiniMaxHailuo23Durations(t *testing.T) {
	t.Parallel()

	groupID := int64(103)
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
	}
	apiKey := &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:             groupID,
			RateMultiplier: 1,
			VideoPrice720P: func() *float64 { v := 0.2; return &v }(),
		},
	}

	cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
		Platform:                      PlatformMiniMaxH3,
		Model:                         MiniMaxHailuo23VideoModel,
		RequestedDurationMilliseconds: 6000,
		VideoCount:                    1,
		VideoResolution:               "768P",
	})
	require.NoError(t, err)
	require.InDelta(t, 6, cost.BillingUnits, 1e-12)

	_, err = svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
		Platform:                      PlatformMiniMaxH3,
		Model:                         MiniMaxHailuo23VideoModel,
		RequestedDurationMilliseconds: 5000,
		VideoCount:                    1,
		VideoResolution:               "768P",
	})
	require.ErrorContains(t, err, "MiniMax-Hailuo-2.3 duration must be 6 or 10 seconds")
}

func TestPPVideoGroupBillingResolutionMapsPixverseV6Tiers(t *testing.T) {
	t.Parallel()

	require.Equal(t, VideoBillingResolution480P, ppVideoGroupBillingResolution(PPVideoBillingMetadata{
		Platform:        PlatformPixverseV6,
		VideoResolution: "360p",
	}))
	require.Equal(t, VideoBillingResolution720P, ppVideoGroupBillingResolution(PPVideoBillingMetadata{
		Platform:        PlatformPixverseV6,
		VideoResolution: "540p",
	}))
	require.Equal(t, VideoBillingResolution1080P, ppVideoGroupBillingResolution(PPVideoBillingMetadata{
		Platform:        PlatformPixverseV6,
		VideoResolution: "1080p",
	}))
}

func TestCalculatePPVideoCostUsesGroupVideoPriceForHappyHorse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		resolution string
		wantRate   float64
	}{
		{resolution: VideoBillingResolution720P, wantRate: 0.2},
		{resolution: VideoBillingResolution1080P, wantRate: 0.4},
	}

	for _, tt := range tests {
		groupID := int64(102)
		svc := &OpenAIGatewayService{
			billingService: newTestBillingService(),
		}
		apiKey := &APIKey{
			GroupID: &groupID,
			Group: &Group{
				ID:              groupID,
				RateMultiplier:  1,
				VideoPrice720P:  func() *float64 { v := 0.2; return &v }(),
				VideoPrice1080P: func() *float64 { v := 0.4; return &v }(),
			},
		}
		meta := PPVideoBillingMetadata{
			Platform:                      PlatformHappyHourse,
			VideoResolution:               tt.resolution,
			RequestedDurationMilliseconds: 5000,
			VideoCount:                    1,
		}

		cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, meta)

		require.NoError(t, err)
		require.Equal(t, PPVideoBillingFormulaPerSecond, cost.BillingFormula)
		require.InDelta(t, 5, cost.BillingUnits, 1e-12)
		require.InDelta(t, tt.wantRate, cost.BillingUnitPrice, 1e-12)
		require.InDelta(t, 5*tt.wantRate, cost.TotalCost, 1e-12)
	}
}

func TestCalculatePPVideoCostIgnoresChannelVideoPricing(t *testing.T) {
	t.Parallel()

	groupID := int64(103)
	groupVideoPrice := 0.2
	apiKey := &APIKey{
		GroupID: &groupID,
		Group: &Group{
			RateMultiplier:       2,
			VideoRateIndependent: true,
			VideoRateMultiplier:  3,
			VideoPrice720P:       &groupVideoPrice,
		},
	}
	meta := PPVideoBillingMetadata{
		Platform:                      PlatformKling,
		KlingMode:                     "std",
		VideoResolution:               VideoBillingResolution720P,
		RequestedDurationMilliseconds: 1000,
		VideoCount:                    1,
	}

	cost, err := (&OpenAIGatewayService{
		billingService: newTestBillingService(),
		resolver: newPPVideoResolverWithChannel(t, groupID, PlatformKling, []ChannelModelPricing{{
			Platform:        PlatformKling,
			Models:          []string{"kling-v3"},
			BillingMode:     BillingModeVideo,
			PerRequestPrice: func() *float64 { v := 99.0; return &v }(),
		}}),
	}).CalculatePPVideoCost(
		context.Background(),
		apiKey,
		nil,
		&Account{},
		meta,
	)

	require.NoError(t, err)
	require.InDelta(t, groupVideoPrice, cost.TotalCost, 1e-12)
	require.InDelta(t, groupVideoPrice*3, cost.ActualCost, 1e-12)
}

func TestCalculateJimengVideoCostUsesGroupVideoPrice(t *testing.T) {
	t.Parallel()

	groupID := int64(104)
	groupVideoPrice := 0.2
	apiKey := &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:             groupID,
			Platform:       PlatformJimeng,
			RateMultiplier: 1,
			VideoPrice720P: &groupVideoPrice,
		},
	}
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
		resolver: newPPVideoResolverWithChannel(t, groupID, PlatformJimeng, []ChannelModelPricing{{
			Platform:        PlatformJimeng,
			Models:          []string{JimengVideoBillingModel},
			BillingMode:     BillingModeVideo,
			PerRequestPrice: func() *float64 { v := 99.0; return &v }(),
		}}),
	}

	cost, err := svc.CalculateJimengVideoCost(context.Background(), apiKey, nil, JimengVideoBillingMetadata{
		VideoResolution:      VideoBillingResolution720P,
		VideoDurationSeconds: 5,
	})

	require.NoError(t, err)
	require.InDelta(t, 1, cost.TotalCost, 1e-12)
	require.InDelta(t, 1, cost.ActualCost, 1e-12)
}

func TestSettlePPVideoTaskUsesActualMillisecondsAndRecordsUsage(t *testing.T) {
	groupID := int64(9)
	task := &PPVideoTask{
		LocalTaskID:                        "ppvidtask_success",
		TaskID:                             "pp-task-success",
		UserID:                             1,
		APIKeyID:                           2,
		GroupID:                            &groupID,
		AccountID:                          3,
		Platform:                           PlatformKling,
		Model:                              "kling-v3",
		Status:                             PPVideoTaskStatusSucceeded,
		BillingStatus:                      PPVideoBillingStatusHeld,
		RequestHash:                        "payload-hash",
		HoldID:                             PPVideoHoldRequestID("ppvidtask_success"),
		CaptureID:                          PPVideoCaptureRequestID("ppvidtask_success"),
		ReleaseID:                          PPVideoReleaseRequestID("ppvidtask_success"),
		EstimatedTotalCost:                 5,
		HoldAmount:                         5,
		RequestedVideoDurationSeconds:      5,
		RequestedVideoDurationMilliseconds: 5000,
		GeneratedVideoDurationMilliseconds: 5041,
		VideoCount:                         1,
		VideoResolution:                    VideoBillingResolution720P,
		ResponseBody:                       `{"status":"succeeded","video_url":"https://cdn.example.com/kling.mp4"}`,
	}
	repo := &ppVideoBillingRepoStub{task: task, claim: true}
	logRepo := &ppVideoUsageLogRepoStub{}
	svc := &OpenAIGatewayService{
		usageBillingRepo: repo,
		usageLogRepo:     logRepo,
	}

	err := svc.SettlePPVideoTask(context.Background(), &PPVideoSettlementInput{
		Task:               task,
		FinalStatus:        PPVideoTaskStatusSucceeded,
		APIKey:             &APIKey{ID: 2, User: &User{ID: 1}, Quota: 100, RateLimit5h: 10},
		User:               &User{ID: 1},
		Account:            &Account{ID: 3, Type: AccountTypeAPIKey},
		RequestPayloadHash: task.RequestHash,
		APIKeyService:      &ppVideoQuotaUpdaterStub{},
		QuotaPlatform:      PlatformKling,
	})

	require.NoError(t, err)
	require.Len(t, repo.captures, 1)
	require.InDelta(t, 5, repo.captures[0].ActualAmount, 1e-12)
	require.Len(t, repo.applyCommands, 1)
	require.InDelta(t, 0.041, repo.applyCommands[0].BalanceCost, 1e-12)
	require.InDelta(t, 5.041, repo.applyCommands[0].APIKeyQuotaCost, 1e-12)
	require.InDelta(t, 5.041, repo.applyCommands[0].APIKeyRateLimitCost, 1e-12)
	require.Len(t, repo.settled, 1)
	require.Equal(t, PPVideoBillingStatusCaptured, repo.settled[0].BillingStatus)
	require.InDelta(t, 5.041, repo.settled[0].ActualCost, 1e-12)
	require.Equal(t, 1, logRepo.calls)
	require.NotNil(t, logRepo.lastLog)
	require.InDelta(t, 5.041, logRepo.lastLog.OutputCost, 1e-12)
	require.InDelta(t, 5.041, logRepo.lastLog.TotalCost, 1e-12)
	require.InDelta(t, 5.041, logRepo.lastLog.ActualCost, 1e-12)
	require.Equal(t, 6, *logRepo.lastLog.VideoDurationSeconds)
}

func TestSettlePPVideoTaskSucceededWithoutVideoURLDoesNotCapture(t *testing.T) {
	task := &PPVideoTask{
		LocalTaskID:                        "ppvidtask_no_video",
		TaskID:                             "pp-task-no-video",
		UserID:                             1,
		APIKeyID:                           2,
		AccountID:                          3,
		Platform:                           PlatformPixverseV6,
		Model:                              PixverseV6VideoDefaultModel,
		Status:                             PPVideoTaskStatusSucceeded,
		BillingStatus:                      PPVideoBillingStatusHeld,
		RequestHash:                        "payload-hash",
		HoldID:                             PPVideoHoldRequestID("ppvidtask_no_video"),
		CaptureID:                          PPVideoCaptureRequestID("ppvidtask_no_video"),
		ReleaseID:                          PPVideoReleaseRequestID("ppvidtask_no_video"),
		EstimatedTotalCost:                 5,
		HoldAmount:                         5,
		RequestedVideoDurationSeconds:      5,
		RequestedVideoDurationMilliseconds: 5000,
		GeneratedVideoDurationMilliseconds: 5000,
		VideoCount:                         1,
		VideoResolution:                    VideoBillingResolution720P,
		ResponseBody:                       `{"status":"succeeded"}`,
	}
	repo := &ppVideoBillingRepoStub{task: task, claim: true}
	logRepo := &ppVideoUsageLogRepoStub{}
	svc := &OpenAIGatewayService{
		usageBillingRepo: repo,
		usageLogRepo:     logRepo,
	}

	err := svc.SettlePPVideoTask(context.Background(), &PPVideoSettlementInput{
		Task:               task,
		FinalStatus:        PPVideoTaskStatusSucceeded,
		APIKey:             &APIKey{ID: 2, User: &User{ID: 1}, Quota: 100, RateLimit5h: 10},
		User:               &User{ID: 1},
		Account:            &Account{ID: 3, Type: AccountTypeAPIKey},
		RequestPayloadHash: task.RequestHash,
		APIKeyService:      &ppVideoQuotaUpdaterStub{},
		QuotaPlatform:      PlatformPixverseV6,
	})

	require.ErrorIs(t, err, ErrPPVideoSettlementBillingFailed)
	require.Empty(t, repo.captures)
	require.Empty(t, repo.applyCommands)
	require.Empty(t, repo.settled)
	require.Zero(t, logRepo.calls)
}

func TestSettlePPVideoTaskRetriesSettlingState(t *testing.T) {
	task := &PPVideoTask{
		LocalTaskID:                        "ppvidtask_retry_settling",
		TaskID:                             "pp-task-retry-settling",
		UserID:                             1,
		APIKeyID:                           2,
		AccountID:                          3,
		Platform:                           PlatformSeedance,
		Model:                              "seedance-2.0",
		Status:                             PPVideoTaskStatusSucceeded,
		BillingStatus:                      PPVideoBillingStatusSettling,
		EstimatedTotalCost:                 2,
		HoldAmount:                         2,
		RequestedVideoDurationSeconds:      4,
		RequestedVideoDurationMilliseconds: 4000,
		GeneratedVideoDurationMilliseconds: 4000,
		VideoCount:                         1,
		VideoResolution:                    VideoBillingResolution720P,
		ResponseBody:                       `{"status":"succeeded","video_url":"https://cdn.example.com/seedance.mp4"}`,
	}
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
	require.Len(t, repo.settled, 1)
}

func TestPPVideoActualSettlementCostUsesRequestedMilliseconds(t *testing.T) {
	task := &PPVideoTask{
		HoldAmount:                         10,
		RequestedVideoDurationMilliseconds: 5500,
		GeneratedVideoDurationMilliseconds: 5041,
	}

	require.InDelta(t, 10*5041.0/5500.0, ppVideoActualSettlementCost(task), 1e-12)
}

func TestPPVideoActualSettlementCostUsesSeedancePerSecondFormula(t *testing.T) {
	t.Parallel()

	const unitPrice = 0.4
	task := &PPVideoTask{
		Platform:                           PlatformSeedance,
		BillingFormula:                     PPVideoBillingFormulaPerSecond,
		BillingUnitPrice:                   unitPrice,
		EstimatedTotalCost:                 5 * unitPrice,
		HoldAmount:                         5 * unitPrice,
		InputVideoDurationMilliseconds:     2000,
		RequestedVideoDurationMilliseconds: 5000,
		GeneratedVideoDurationMilliseconds: 6000,
		VideoCount:                         1,
	}

	require.InDelta(t, 6*unitPrice, ppVideoActualSettlementCost(task), 1e-12)
}

func newPPVideoResolverWithChannel(t *testing.T, groupID int64, platform string, pricing []ChannelModelPricing) *ModelPricingResolver {
	t.Helper()
	repo := &ppVideoChannelRepositoryStub{
		channels: []Channel{{
			ID:           1,
			Name:         "pp-video-channel",
			Status:       StatusActive,
			GroupIDs:     []int64{groupID},
			ModelPricing: pricing,
		}},
		groupPlatforms: map[int64]string{groupID: platform},
	}
	return NewModelPricingResolver(NewChannelService(repo, nil, nil, nil), NewBillingService(&config.Config{}, nil))
}

type ppVideoChannelRepositoryStub struct {
	channels       []Channel
	groupPlatforms map[int64]string
}

func (r *ppVideoChannelRepositoryStub) Create(context.Context, *Channel) error { return nil }
func (r *ppVideoChannelRepositoryStub) GetByID(context.Context, int64) (*Channel, error) {
	return nil, ErrChannelNotFound
}
func (r *ppVideoChannelRepositoryStub) Update(context.Context, *Channel) error { return nil }
func (r *ppVideoChannelRepositoryStub) Delete(context.Context, int64) error    { return nil }
func (r *ppVideoChannelRepositoryStub) List(context.Context, pagination.PaginationParams, string, string) ([]Channel, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *ppVideoChannelRepositoryStub) ListAll(context.Context) ([]Channel, error) {
	return r.channels, nil
}
func (r *ppVideoChannelRepositoryStub) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}
func (r *ppVideoChannelRepositoryStub) ExistsByNameExcluding(context.Context, string, int64) (bool, error) {
	return false, nil
}
func (r *ppVideoChannelRepositoryStub) GetGroupIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}
func (r *ppVideoChannelRepositoryStub) SetGroupIDs(context.Context, int64, []int64) error {
	return nil
}
func (r *ppVideoChannelRepositoryStub) GetChannelIDByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *ppVideoChannelRepositoryStub) GetGroupsInOtherChannels(context.Context, int64, []int64) ([]int64, error) {
	return nil, nil
}
func (r *ppVideoChannelRepositoryStub) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return r.groupPlatforms, nil
}
func (r *ppVideoChannelRepositoryStub) ListModelPricing(context.Context, int64) ([]ChannelModelPricing, error) {
	return nil, nil
}
func (r *ppVideoChannelRepositoryStub) CreateModelPricing(context.Context, *ChannelModelPricing) error {
	return nil
}
func (r *ppVideoChannelRepositoryStub) UpdateModelPricing(context.Context, *ChannelModelPricing) error {
	return nil
}
func (r *ppVideoChannelRepositoryStub) DeleteModelPricing(context.Context, int64) error {
	return nil
}
func (r *ppVideoChannelRepositoryStub) ReplaceModelPricing(context.Context, int64, []ChannelModelPricing) error {
	return nil
}

type ppVideoBillingRepoStub struct {
	UsageBillingRepository
	PPVideoTaskRepository
	task          *PPVideoTask
	claim         bool
	captures      []*BatchImageBalanceHoldCommand
	releases      []*BatchImageBalanceHoldCommand
	applyCommands []*UsageBillingCommand
	settled       []MarkPPVideoTaskSettledParams
}

func (r *ppVideoBillingRepoStub) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.applyCommands = append(r.applyCommands, cmd)
	return &UsageBillingApplyResult{Applied: true}, nil
}

func (r *ppVideoBillingRepoStub) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.captures = append(r.captures, cmd)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

func (r *ppVideoBillingRepoStub) ReleaseBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	r.releases = append(r.releases, cmd)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

func (r *ppVideoBillingRepoStub) ClaimPPVideoTaskSettlement(_ context.Context, params ClaimPPVideoTaskSettlementParams) (*PPVideoTask, bool, error) {
	if r.task == nil || r.task.TaskID != params.TaskID {
		return nil, false, nil
	}
	if r.claim {
		r.task.Status = params.FinalStatus
		if r.task.BillingStatus == PPVideoBillingStatusNone {
			r.task.BillingStatus = PPVideoBillingStatusSettlingNone
		} else {
			r.task.BillingStatus = PPVideoBillingStatusSettling
		}
	}
	return r.task, r.claim, nil
}

func (r *ppVideoBillingRepoStub) MarkPPVideoTaskSettled(_ context.Context, params MarkPPVideoTaskSettledParams) (*PPVideoTask, error) {
	r.settled = append(r.settled, params)
	r.task.Status = params.Status
	r.task.BillingStatus = params.BillingStatus
	return r.task, nil
}

type ppVideoUsageLogRepoStub struct {
	UsageLogRepository
	calls   int
	lastLog *UsageLog
}

func (r *ppVideoUsageLogRepoStub) Create(_ context.Context, log *UsageLog) (bool, error) {
	r.calls++
	r.lastLog = log
	return true, nil
}

type ppVideoQuotaUpdaterStub struct{}

func (s *ppVideoQuotaUpdaterStub) UpdateQuotaUsed(_ context.Context, _ int64, _ float64) error {
	return nil
}

func (s *ppVideoQuotaUpdaterStub) UpdateRateLimitUsage(_ context.Context, _ int64, _ float64) error {
	return nil
}
