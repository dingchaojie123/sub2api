package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserVideoTaskFromRecord_SucceededTask(t *testing.T) {
	created := time.Date(2026, 11, 6, 14, 34, 8, 0, time.UTC)
	finished := created.Add(9*time.Minute + 36*time.Second)
	charged := 6.5

	got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
		LocalTaskID:                        "ppvidtask_1",
		TaskID:                             "upstream_1",
		APIKeyName:                         "prod-key",
		Platform:                           service.PlatformCTMOSeedance,
		Operation:                          service.PPVideoOperationGeneric,
		Model:                              "seedance2.0-933-C",
		Status:                             "completed",
		BillingStatus:                      service.PPVideoBillingStatusCaptured,
		Prompt:                             "a cat",
		VideoCount:                         1,
		VideoResolution:                    "720p",
		GeneratedVideoDurationMilliseconds: 15000,
		HoldAmount:                         10,
		ActualCost:                         &charged,
		CreatedAt:                          created,
		FinishedAt:                         &finished,
	})

	require.Equal(t, "ppvidtask_1", got.ID)
	require.Equal(t, "upstream_1", got.RequestID)
	require.Equal(t, service.PPVideoTaskStatusSucceeded, got.Status)
	require.Equal(t, 1, got.OutputCount)
	require.InDelta(t, 15, got.DurationSeconds, 1e-9)
	require.Equal(t, "/v1/videos/generations", got.Endpoint)
	require.NotNil(t, got.CreditsCharged)
	require.InDelta(t, 6.5, *got.CreditsCharged, 1e-9)
	require.NotNil(t, got.CreditsRefunded)
	require.InDelta(t, 3.5, *got.CreditsRefunded, 1e-9)
	require.NotNil(t, got.TotalSeconds)
	require.InDelta(t, 576, *got.TotalSeconds, 1e-9)
	require.True(t, got.CanPlay)
}

func TestUserVideoTaskFromRecord_FailedTaskHasNoOutputAndIsRefunded(t *testing.T) {
	got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
		LocalTaskID:      "ppvidtask_2",
		Platform:         service.PlatformCTMOSeedance,
		Status:           "error",
		BillingStatus:    service.PPVideoBillingStatusReleased,
		VideoCount:       1,
		HoldAmount:       6.5,
		LastErrorCode:    "upstream_error",
		LastErrorMessage: "boom",
		CreatedAt:        time.Now(),
	})

	require.Equal(t, service.PPVideoTaskStatusFailed, got.Status)
	require.Equal(t, 0, got.OutputCount)
	require.Equal(t, "ppvidtask_2", got.RequestID, "falls back to the local id when no upstream id exists")
	require.Nil(t, got.CreditsCharged)
	require.NotNil(t, got.CreditsRefunded)
	require.InDelta(t, 6.5, *got.CreditsRefunded, 1e-9)
	require.False(t, got.CanPlay)
	require.Equal(t, "boom", got.ErrorMessage)
}

func TestUserVideoTaskFromRecord_RecoversTheFailureReasonFromTheStoredResponse(t *testing.T) {
	got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
		LocalTaskID:         "ppvidtask_6",
		Platform:            service.PlatformGuangying,
		Status:              "failed",
		BillingStatus:       service.PPVideoBillingStatusReleased,
		HoldAmount:          2.24,
		FailureResponseBody: `{"data":{"status":"failed","message":"generation failed"}}`,
		CreatedAt:           time.Now(),
	})

	require.Equal(t, service.PPVideoTaskStatusFailed, got.Status)
	require.Equal(t, "generation failed", got.ErrorMessage)
}

func TestUserVideoTaskFromRecord_NoReasonAnywhereLeavesTheMessageEmpty(t *testing.T) {
	got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
		LocalTaskID:         "ppvidtask_7",
		Status:              "failed",
		FailureResponseBody: `{"status":"failed"}`,
		CreatedAt:           time.Now(),
	})

	require.Equal(t, "", got.ErrorMessage)
}

func TestUserVideoTaskFromRecord_SubmittingCountsAsProcessingAndCannotPlay(t *testing.T) {
	got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
		LocalTaskID: "ppvidtask_3",
		Platform:    service.PlatformOPONE,
		Status:      service.PPVideoTaskStatusSubmitting,
		CreatedAt:   time.Now(),
	})

	require.Equal(t, service.PPVideoTaskStatusProcessing, got.Status)
	require.False(t, got.CanPlay)
	require.Nil(t, got.TotalSeconds)
}

func TestUserVideoTaskFromRecord_SucceededTaskCanPlayOnEveryVideoPlatform(t *testing.T) {
	for _, platform := range []string{service.PlatformGuangying, service.PlatformKling, service.PlatformCTMOSeedance} {
		got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
			LocalTaskID: "ppvidtask_4",
			Platform:    platform,
			Status:      "succeeded",
			CreatedAt:   time.Now(),
		})

		require.Equal(t, service.PPVideoTaskStatusSucceeded, got.Status)
		require.True(t, got.CanPlay, platform)
	}

	notVideo := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
		LocalTaskID: "ppvidtask_5", Platform: service.PlatformOpenAI, Status: "succeeded", CreatedAt: time.Now(),
	})
	require.False(t, notVideo.CanPlay)
}

func TestUserVideoTaskFromRecord_MaiTokenCanPlayOnlyAfterCapture(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		billingStatus string
		want          bool
	}{
		{billingStatus: service.PPVideoBillingStatusHeld, want: false},
		{billingStatus: service.PPVideoBillingStatusSettling, want: false},
		{billingStatus: service.PPVideoBillingStatusCaptured, want: true},
	} {
		got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
			LocalTaskID: "ppvidtask_mai", Platform: service.PlatformMaiTokenSeedance,
			Status: service.PPVideoTaskStatusSucceeded, BillingStatus: tt.billingStatus,
			CreatedAt: time.Now(),
		})

		require.Equal(t, tt.want, got.CanPlay, tt.billingStatus)
	}
}

func TestUserVideoTaskFromRecord_TruncatesLongErrorMessage(t *testing.T) {
	got := userVideoTaskFromRecord(&service.PPVideoTaskRecord{
		LocalTaskID:      "ppvidtask_5",
		LastErrorMessage: strings.Repeat("错", userVideoTaskErrorMessageMaxRunes+30),
		CreatedAt:        time.Now(),
	})

	require.Equal(t, userVideoTaskErrorMessageMaxRunes, len([]rune(got.ErrorMessage)))
}

func TestValidUserVideoTaskStatusFilter(t *testing.T) {
	for _, ok := range []string{"", "processing", "succeeded", "failed", "requires_attention"} {
		require.True(t, validUserVideoTaskStatusFilter(ok), ok)
	}
	for _, bad := range []string{"success", "running", "' OR 1=1 --", "FAILED"} {
		require.False(t, validUserVideoTaskStatusFilter(bad), bad)
	}
}

func newUserVideoTaskTestContext(method, target string, authenticated bool) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, nil)
	if authenticated {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
	}
	return c, recorder
}

func TestUserVideoTaskEndpointsRequireAuthentication(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	tests := []struct {
		name string
		call func(*gin.Context)
	}{
		{"list", h.ListUserVideoTasks},
		{"detail", h.GetUserVideoTask},
		{"content", h.GetUserVideoTaskContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newUserVideoTaskTestContext(http.MethodGet, "/api/v1/video-tasks", false)
			tt.call(c)
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestListUserVideoTasksRejectsInvalidFilters(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	for _, target := range []string{
		"/api/v1/video-tasks?status=bogus",
		"/api/v1/video-tasks?start_date=2026-13-45",
		"/api/v1/video-tasks?end_date=yesterday",
	} {
		t.Run(target, func(t *testing.T) {
			c, recorder := newUserVideoTaskTestContext(http.MethodGet, target, true)
			h.ListUserVideoTasks(c)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
