package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCompShareVideoRequestContract(t *testing.T) {
	for _, resolution := range []string{"480P", "768P", "1080P", "2K", "4K", "4k"} {
		t.Run(resolution, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":"MiniMax-H3","content":[{"type":"text","text":"first"},{"type":"text","text":"second"}],"duration":30,"resolution":%q,"ratio":"adaptive","use_context_ir":true,"skill_id":"skill-1","mute_audio":true}`, resolution))
			prepared, public, err := PreparePPVideoRequestBody(PlatformMiniMaxH3CompShare, PPVideoOperationGeneric, body)
			require.NoError(t, err)
			require.False(t, gjson.GetBytes(prepared, "input").Exists())
			require.Equal(t, int64(30), gjson.GetBytes(prepared, "duration").Int())
			require.Equal(t, int64(2), gjson.GetBytes(prepared, "content.#").Int())
			require.Equal(t, "16:9", gjson.GetBytes(prepared, "ratio").String())
			require.Equal(t, "skill-1", gjson.GetBytes(prepared, "skill_id").String())
			require.True(t, gjson.GetBytes(prepared, "mute_audio").Bool())
			if resolution == "4k" {
				resolution = "768P"
			}
			require.Equal(t, resolution, gjson.GetBytes(prepared, "resolution").String())
			require.Equal(t, strings.ToLower(resolution), public.Resolution)
			meta := PPVideoBillingMetadataFromRequest(PlatformMiniMaxH3CompShare, prepared)
			require.NoError(t, meta.ValidationError)
			require.Equal(t, public.Resolution, meta.VideoResolution)
			require.Equal(t, int64(30000), meta.RequestedDurationMilliseconds)
		})
	}
	prepared, _, err := PreparePPVideoRequestBody(PlatformMiniMaxH3CompShare, PPVideoOperationGeneric, []byte(`{"model":"minimax-h3-lite","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,YQ=="}}]}`))
	require.NoError(t, err)
	require.Equal(t, "minimax-h3-lite", gjson.GetBytes(prepared, "model").String())
	require.Equal(t, "first_frame", gjson.GetBytes(prepared, "content.0.role").String())
	require.Equal(t, int64(5), gjson.GetBytes(prepared, "duration").Int())
	_, _, err = PreparePPVideoRequestBody(PlatformMiniMaxH3, PPVideoOperationGeneric, []byte(`{"model":"MiniMax-H3","prompt":"test","duration":30}`))
	require.Error(t, err, "the old provider must retain its own limits")
}

func TestCompShareVideoRejectsInvalidRequests(t *testing.T) {
	for _, body := range []string{
		`{"prompt":"test","duration":0}`,
		`{"prompt":"test","duration":3}`,
		`{"prompt":"test","duration":31}`,
		`{"prompt":"test","duration":4.5}`,
		`{"prompt":"test","duration":"5"}`,
		`{"prompt":"test","model":"MiniMax-Hailuo-2.3"}`,
		`{"prompt":"test","skill_id":"skill-1"}`,
		`{"prompt":"test","mute_audio":"true"}`,
		`{"prompt":"test","callback_token":"secret"}`,
		`{"prompt":"test","callback_url":"file:///tmp/callback"}`,
		`{"content":[]}`,
		`{"content":[{"type":"audio_url","audio_url":{"url":"https://example.com/audio.mp3"}}]}`,
		`{"content":[{"type":"image_url","image_url":{"url":"https://example.com/frame.png"}},{"type":"image_url","image_url":{"url":"https://example.com/ref.png"},"role":"reference_image"}]}`,
		`{"content":[{"type":"text","text":""}]}`,
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformMiniMaxH3CompShare, PPVideoOperationGeneric, []byte(body))
		require.Error(t, err, body)
	}
}

func TestCompShareVideoForwardLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{}
	for _, body := range []string{
		`{"task_id":"task-1"}`,
		`{"task_id":"task-1","action":"delete","status":"running"}`,
		`{"task":{"id":"task-1","status":"succeeded","resolution":"768P","duration":5,"content":{"url":"https://cdn.example.com/final.mp4"},"usage":{"output_seconds":5,"input_seconds":2}}}`,
	} {
		upstream.responses = append(upstream.responses, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))})
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 9, Platform: PlatformMiniMaxH3CompShare, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-ml-test"}}
	body := []byte(`{"model":"MiniMax-H3","prompt":"test","resolution":"4K"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body))
	result, err := svc.ForwardPPVideoBuffered(context.Background(), c, account, PPVideoOperationGeneric, "", body)
	require.NoError(t, err)
	require.Equal(t, "task-1", result.ResponseID)
	require.Equal(t, PPVideoTaskStatusProcessing, result.TaskStatus)
	require.Equal(t, "https://cp.compshare.cn/minimax/v2/video_generation", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-ml-test", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "4K", gjson.GetBytes(upstream.bodies[0], "resolution").String())
	result, err = svc.ForwardPPVideoBuffered(context.Background(), c, account, PPVideoOperationCancel, "task-1", nil)
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusProcessing, result.TaskStatus)
	require.Equal(t, http.MethodDelete, upstream.requests[1].Method)
	require.Equal(t, "/minimax/v2/video_generation/task-1", upstream.requests[1].URL.Path)
	result, err = svc.ForwardPPVideoBuffered(context.Background(), c, account, PPVideoOperationGeneric, "task-1", nil)
	require.NoError(t, err)
	require.Equal(t, "/minimax/v2/query/video_generation/task-1", upstream.requests[2].URL.Path)
	require.Equal(t, PPVideoTaskStatusSucceeded, result.TaskStatus)
	require.Equal(t, "768p", result.VideoResolution)
	require.Equal(t, int64(5000), result.VideoDurationMilliseconds)
	require.Equal(t, int64(2000), result.VideoInputDurationMilliseconds)
	require.Equal(t, "https://cdn.example.com/final.mp4", gjson.GetBytes(result.ResponseBody, "video_url").String())
}

func TestCompShareVideoResponseFailuresAndPendingOutput(t *testing.T) {
	for _, tt := range []struct{ body, status, message string }{
		{`{"task":{"id":"1","status":"succeeded"}}`, PPVideoTaskStatusProcessing, ""},
		{`{"task":{"id":"1","status":"running","content":{"url":"https://example.com/preview.mp4"}}}`, PPVideoTaskStatusProcessing, ""},
		{`{"task":{"id":"1","status":"failed","error":{"code":"blocked","message":"rejected"}}}`, PPVideoTaskStatusFailed, "rejected"},
		{`{"task_id":"1","status":"cancelled","action":"delete"}`, PPVideoTaskStatusFailed, ""},
	} {
		result, err := ParsePPVideoResponse(PlatformMiniMaxH3CompShare, []byte(tt.body))
		require.NoError(t, err)
		require.Equal(t, tt.status, result.Status)
		require.Equal(t, tt.message, result.ErrorMessage)
	}
	_, err := ParsePPVideoResponse(PlatformMiniMaxH3CompShare, []byte(`{"RetCode":8039,"Message":"job not found"}`))
	require.ErrorContains(t, err, "job not found")
}

func TestCompShareVideoPricingAndDowngrade(t *testing.T) {
	prices := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	group := &Group{RateMultiplier: 1, VideoPrice480P: &prices[0], VideoPrice720P: &prices[1], VideoPrice1080P: &prices[2], VideoPrice2K: &prices[3], VideoPrice4K: &prices[4]}
	svc := &OpenAIGatewayService{}
	for i, resolution := range []string{"480p", "768p", "1080p", "2k", "4k"} {
		meta := PPVideoBillingMetadata{Platform: PlatformMiniMaxH3CompShare, Model: MiniMaxH3VideoDefaultModel, RequestedDurationMilliseconds: 30000, VideoCount: 1, VideoResolution: resolution}
		cost, err := svc.CalculatePPVideoCost(context.Background(), &APIKey{Group: group}, nil, &Account{}, meta)
		require.NoError(t, err)
		require.InDelta(t, 30*prices[i], cost.ActualCost, 1e-9)
		task := &PPVideoTask{Platform: PlatformMiniMaxH3CompShare, BillingFormula: cost.BillingFormula, BillingUnitPrice: cost.BillingUnitPrice, BillingFallbackUnitPrice: cost.BillingFallbackUnitPrice, RequestedVideoDurationMilliseconds: 30000, GeneratedVideoDurationMilliseconds: 30000, VideoCount: 1, VideoResolution: resolution, EstimatedTotalCost: cost.TotalCost, HoldAmount: cost.ActualCost}
		require.InDelta(t, cost.ActualCost, ppVideoActualSettlementCost(task), 1e-9)
		if i >= 2 {
			task.VideoResolution = "768p"
			require.InDelta(t, 6.0, ppVideoActualSettlementCost(task), 1e-9)
			task.BillingFallbackUnitPrice = 0
			require.Zero(t, ppVideoActualSettlementCost(task))
		}
	}
	_, _, err := compShareVideoPrices("4k", &APIKey{Group: &Group{}})
	require.ErrorContains(t, err, "configure CompShare")
	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{Platform: PlatformMiniMaxH3CompShare, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}})
	require.NoError(t, err)
	require.Equal(t, []string{MiniMaxH3VideoDefaultModel, CompShareVideoLiteModel}, models)
}
