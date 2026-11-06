package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type ppVideoBillingCacheStub struct {
	service.BillingCache
	invalidated []int64
}

func (s *ppVideoBillingCacheStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.invalidated = append(s.invalidated, userID)
	return nil
}

func TestInvalidatePPVideoBalanceCache(t *testing.T) {
	cache := &ppVideoBillingCacheStub{}
	cacheService := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(cacheService.Stop)
	h := &OpenAIGatewayHandler{billingCacheService: cacheService}

	h.invalidatePPVideoBalanceCache(context.Background(), 42)
	h.invalidatePPVideoBalanceCache(context.Background(), 0)

	require.Equal(t, []int64{42}, cache.invalidated)
}

func TestPPVideoOperationFromRequestTreatsUpdatedImageFieldsAsImageToVideo(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"images":["https://example.com/in.png"]}`,
		`{"start_frame_url":"https://example.com/start.png"}`,
		`{"end_frame_url":"https://example.com/end.png"}`,
	} {
		require.Equal(t, service.PPVideoOperationKlingImageToVideo, ppVideoOperationFromRequest([]byte(body)), "body=%s", body)
	}
}

func TestPPVideoOperationFromRequestDefaultsToTextToVideo(t *testing.T) {
	t.Parallel()

	require.Equal(t, service.PPVideoOperationKlingTextToVideo, ppVideoOperationFromRequest([]byte(`{"prompt":"waves"}`)))
}

func TestShouldReplayStoredPPVideoStatus(t *testing.T) {
	t.Parallel()
	settledAt := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		task *service.PPVideoTask
		want bool
	}{
		{name: "nil task"},
		{
			name: "processing task with response is still live",
			task: &service.PPVideoTask{Status: service.PPVideoTaskStatusProcessing, ResponseBody: `{"status":"processing"}`},
		},
		{
			name: "succeeded task without stored response cannot replay",
			task: &service.PPVideoTask{Status: service.PPVideoTaskStatusSucceeded},
		},
		{
			name: "succeeded task with stored response replays while settlement is pending",
			task: &service.PPVideoTask{Status: service.PPVideoTaskStatusSucceeded, ResponseBody: `{"video_url":"https://cdn.example.com/kling.mp4"}`},
			want: true,
		},
		{
			name: "settled succeeded task with stored response replays",
			task: &service.PPVideoTask{Status: service.PPVideoTaskStatusSucceeded, ResponseBody: `{"video_url":"https://cdn.example.com/kling.mp4"}`, SettledAt: &settledAt},
			want: true,
		},
		{
			name: "settled failed task with stored response replays",
			task: &service.PPVideoTask{Status: service.PPVideoTaskStatusFailed, ResponseBody: `{"status":"failed"}`, SettledAt: &settledAt},
			want: true,
		},
		{
			name: "CTMO premature success is polled again",
			task: &service.PPVideoTask{
				Platform:     service.PlatformCTMOSeedance,
				Status:       service.PPVideoTaskStatusSucceeded,
				ResponseBody: `{"id":"ctmo-1","status":"succeeded","progress":0,"content":"https://video.ctmoai.com/v1/videos/ctmo-1/content"}`,
			},
		},
		{
			name: "CTMO completed task replays",
			task: &service.PPVideoTask{
				Platform:     service.PlatformCTMOSeedance,
				Status:       service.PPVideoTaskStatusSucceeded,
				ResponseBody: `{"id":"ctmo-2","status":"completed","progress":100}`,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, shouldReplayStoredPPVideoStatus(tt.task))
		})
	}
}

func TestCanFallbackToStoredPPVideoStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task *service.PPVideoTask
		want bool
	}{
		{name: "nil task"},
		{name: "task without stored response", task: &service.PPVideoTask{Status: service.PPVideoTaskStatusProcessing}},
		{
			name: "processing task with stored response",
			task: &service.PPVideoTask{Status: service.PPVideoTaskStatusProcessing, ResponseBody: `{"status":"processing"}`},
			want: true,
		},
		{
			name: "succeeded task awaiting settlement",
			task: &service.PPVideoTask{Status: service.PPVideoTaskStatusSucceeded, ResponseBody: `{"video_url":"https://cdn.example.com/kling.mp4"}`},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, canFallbackToStoredPPVideoStatus(tt.task))
		})
	}
}

func TestWriteStoredPPVideoResponseNormalizesProviderVideoURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeStoredPPVideoResponse(c, &service.PPVideoTask{
		Platform:            service.PlatformKling,
		Model:               "kling-v3",
		ResponseStatus:      200,
		ResponseContentType: "application/json",
		ResponseBody: `{
			"data": {
				"task_id": "kling-stored-1",
				"task_status": "succeed",
				"result": {
					"video_url": "https://cdn.example.com/stored-kling.mp4"
				}
			}
		}`,
	})

	body := recorder.Body.Bytes()
	require.Equal(t, 200, recorder.Code)
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "video_url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "result_url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "videos.0.url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "videos.0.video_url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "data.video_url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "data.url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "data.result_url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "data.videos.0.url").String())
	require.Equal(t, "https://cdn.example.com/stored-kling.mp4", gjson.GetBytes(body, "data.videos.0.video_url").String())
	require.Equal(t, service.PPVideoTaskStatusSucceeded, gjson.GetBytes(body, "status").String())
}

func TestWriteStoredOPONECompletedResponseUsesContentProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeStoredPPVideoResponse(c, &service.PPVideoTask{
		Platform:            service.PlatformOPONE,
		Model:               "seedance-2.5-Q",
		TaskID:              "opone-q-1",
		Status:              service.PPVideoTaskStatusSucceeded,
		ResponseStatus:      200,
		ResponseContentType: "application/json",
		ResponseBody:        `{"status":"completed","progress":100}`,
	})

	body := recorder.Body.Bytes()
	require.Equal(t, 200, recorder.Code)
	require.Equal(t, service.PPVideoTaskStatusSucceeded, gjson.GetBytes(body, "status").String())
	require.Equal(t, "/v1/videos/opone-q-1/content", gjson.GetBytes(body, "video_url").String())
	require.Equal(t, "/v1/videos/opone-q-1/content", gjson.GetBytes(body, "videos.0.url").String())
}

func TestWriteStoredCTMOPrematureSuccessReturnsProcessingWithoutURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	writeStoredPPVideoResponse(c, &service.PPVideoTask{
		Platform:            service.PlatformCTMOSeedance,
		Model:               "seedance2.5-one-720p",
		TaskID:              "ctmo-pending-1",
		Status:              service.PPVideoTaskStatusSucceeded,
		ResponseStatus:      200,
		ResponseContentType: "application/json",
		ResponseBody:        `{"id":"ctmo-pending-1","status":"succeeded","progress":0,"content":"https://video.ctmoai.com/v1/videos/ctmo-pending-1/content","video_url":"https://video.ctmoai.com/v1/videos/ctmo-pending-1/content"}`,
	})

	body := recorder.Body.Bytes()
	require.Equal(t, 200, recorder.Code)
	require.Equal(t, service.PPVideoTaskStatusProcessing, gjson.GetBytes(body, "status").String())
	require.False(t, gjson.GetBytes(body, "content").Exists())
	require.False(t, gjson.GetBytes(body, "video_url").Exists())
	require.False(t, gjson.GetBytes(body, "result_url").Exists())
}
