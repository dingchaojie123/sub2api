package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPrepareCTMOSeedanceRequestBody(t *testing.T) {
	t.Parallel()
	account := &Account{
		Platform: PlatformCTMOSeedance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"seedance-2.0-930": "seedance-2.0-930",
			},
		},
	}

	body, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, []byte(`{
		"model":"seedance-2.0-930",
		"prompt":"lake at dawn",
		"duration":4,
		"ratio":"9:16",
		"resolution":"480p",
		"image_url":"https://example.com/reference.jpg",
		"videos":["https://example.com/reference.mp4"],
		"n":3,
		"metadata":{"ignored":true}
	}`))

	require.NoError(t, err)
	require.Equal(t, "seedance-2.0-930", public.Model)
	require.Equal(t, "seedance-2.0-930", public.UpstreamModel)
	require.Equal(t, 1, public.VideoCount)
	require.JSONEq(t, `{
		"model":"seedance-2.0-930",
		"prompt":"lake at dawn",
		"seconds":4,
		"aspect_ratio":"9:16",
		"resolution":"480p",
		"images":["https://example.com/reference.jpg"],
		"videos":["https://example.com/reference.mp4"]
	}`, string(body))
}

func TestPrepareCTMOSeedanceRequestRestoresLegacyOneModelMappings(t *testing.T) {
	t.Parallel()

	for _, model := range []string{"seedance2.5-one-720p", "seedance2.5-one-1080p"} {
		model := model
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			account := &Account{
				Platform: PlatformCTMOSeedance,
				Credentials: map[string]any{
					"model_mapping": map[string]any{model: legacyTikTokSeedanceOneModel},
				},
			}

			body, public, err := PreparePPVideoRequestBodyForAccount(
				account,
				PPVideoOperationGeneric,
				[]byte(`{"model":"`+model+`","prompt":"lake at dawn","duration":5}`),
			)

			require.NoError(t, err)
			require.Equal(t, model, public.UpstreamModel)
			require.Equal(t, model, gjson.GetBytes(body, "model").String())
		})
	}
}

func TestPrepareSeedanceRequestRestoresLegacyOneModelMappings(t *testing.T) {
	t.Parallel()

	for _, model := range []string{"seedance2.5-one-720p", "seedance2.5-one-1080p"} {
		model := model
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			account := &Account{
				Platform: PlatformSeedance,
				Credentials: map[string]any{
					"model_mapping": map[string]any{model: legacyTikTokSeedanceOneModel},
				},
			}

			body, public, err := PreparePPVideoRequestBodyForAccount(
				account,
				PPVideoOperationGeneric,
				[]byte(`{"model":"`+model+`","prompt":"lake at dawn","duration":5}`),
			)

			require.NoError(t, err)
			require.Equal(t, model, public.UpstreamModel)
			require.Equal(t, model, gjson.GetBytes(body, "model").String())
		})
	}
}

func TestForwardPPVideoBufferedNeverSendsLegacyOneModel(t *testing.T) {
	t.Parallel()

	for _, platform := range []string{PlatformSeedance, PlatformCTMOSeedance} {
		platform := platform
		for _, model := range []string{"seedance2.5-one-720p", "seedance2.5-one-1080p"} {
			model := model
			t.Run(platform+"/"+model, func(t *testing.T) {
				t.Parallel()
				upstream := &httpUpstreamRecorder{responses: []*http.Response{{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewBufferString(`{"id":"task-1","status":"queued"}`)),
				}}}
				svc := &OpenAIGatewayService{httpUpstream: upstream}
				account := &Account{
					Platform: platform,
					Type:     AccountTypeAPIKey,
					Credentials: map[string]any{
						"api_key":  "test-key",
						"base_url": "https://video.example.com",
					},
				}

				_, err := svc.ForwardPPVideoBuffered(
					context.Background(),
					nil,
					account,
					PPVideoOperationGeneric,
					"",
					[]byte(`{"model":"`+legacyTikTokSeedanceOneModel+`","prompt":"lake at dawn"}`),
					PPVideoPublicRequest{Model: model, UpstreamModel: model},
				)

				require.NoError(t, err)
				require.Len(t, upstream.bodies, 1)
				require.Equal(t, model, gjson.GetBytes(upstream.bodies[0], "model").String())
				require.NotContains(t, string(upstream.bodies[0]), legacyTikTokSeedanceOneModel)
			})
		}
	}
}

func TestCTMOSeedanceOnlyAcceptsConfiguredTikTokModels(t *testing.T) {
	t.Parallel()

	models := append([]string(nil), ctmoSeedanceSupportedModels...)
	models = append(models, "SD2.5-30-10-10", "future-seedance-model")
	require.ElementsMatch(t, ctmoSeedanceSupportedModels, filterUpstreamModelsForPlatform(PlatformCTMOSeedance, models))

	_, _, err := PreparePPVideoRequestBody(
		PlatformCTMOSeedance,
		PPVideoOperationGeneric,
		[]byte(`{"model":"SD2.5-30-10-10","prompt":"lake at dawn","duration":5}`),
	)
	require.EqualError(t, err, `model "SD2.5-30-10-10" is not supported by platform "ctmo-seedance"`)
}

func TestCTMOSeedancePathsAndCompletedResponse(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformCTMOSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformCTMOSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	raw := []byte(`{"id":"task-1","model":"seedance2.5-one-720p","status":"completed","progress":100}`)
	parsed, err := ParsePPVideoResponse(PlatformCTMOSeedance, raw)
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)
	normalized := NormalizePPVideoPublicResponse(PlatformCTMOSeedance, raw, PPVideoPublicRequest{Model: "seedance2.5-one-720p"}, parsed)
	require.Equal(t, PPVideoTaskStatusSucceeded, gjson.GetBytes(normalized, "status").String())
	require.Equal(t, "/v1/videos/task-1/content", gjson.GetBytes(normalized, "video_url").String())
	require.Equal(t, PPVideoTaskStatusProcessing, NormalizePPVideoTaskStatus("unknown"))
}

func TestCTMOSeedancePendingResponseDoesNotExposePlaceholderContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "explicit processing",
			raw:  `{"id":"task-processing","status":"processing","progress":0,"content":"https://video.ctmoai.com/v1/videos/task-processing/content","video_url":"https://video.ctmoai.com/v1/videos/task-processing/content","data":{"result_url":"https://video.ctmoai.com/v1/videos/task-processing/content","videos":[{"url":"https://video.ctmoai.com/v1/videos/task-processing/content"}]}}`,
		},
		{
			name: "premature completed",
			raw:  `{"id":"task-completed","status":"completed","progress":"0","content":"https://video.ctmoai.com/v1/videos/task-completed/content","result_url":"https://video.ctmoai.com/v1/videos/task-completed/content"}`,
		},
		{
			name: "missing status",
			raw:  `{"id":"task-no-status","data":{"url":"https://video.ctmoai.com/v1/videos/task-no-status/content"}}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := []byte(tt.raw)
			parsed, err := ParsePPVideoResponse(PlatformCTMOSeedance, raw)
			require.NoError(t, err)
			require.Equal(t, PPVideoTaskStatusProcessing, parsed.Status)

			normalized := NormalizePPVideoPublicResponse(
				PlatformCTMOSeedance,
				raw,
				PPVideoPublicRequest{Model: "seedance2.5-one-720p"},
				parsed,
			)
			require.Equal(t, PPVideoTaskStatusProcessing, gjson.GetBytes(normalized, "status").String())
			for _, path := range []string{"content", "video_url", "url", "result_url", "videos", "data.video_url", "data.url", "data.result_url", "data.videos"} {
				require.False(t, gjson.GetBytes(normalized, path).Exists(), "unexpected pending result field %s in %s", path, normalized)
			}
		})
	}
}

func TestCTMOSeedanceCompletedResponseWithoutProgressRemainsSucceeded(t *testing.T) {
	t.Parallel()

	parsed, err := ParsePPVideoResponse(
		PlatformCTMOSeedance,
		[]byte(`{"id":"task-complete","status":"completed"}`),
	)
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)
}

func TestForwardCTMOSeedancePendingStatusDoesNotReturnPrematureSuccess(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewBufferString(
			`{"id":"task-pending","status":"succeeded","progress":0,"content":"https://video.ctmoai.com/v1/videos/task-pending/content","video_url":"https://video.ctmoai.com/v1/videos/task-pending/content"}`,
		)),
	}}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          42,
		Platform:    PlatformCTMOSeedance,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "ctmo-key", "base_url": "https://video.ctmoai.com"},
	}

	result, err := svc.ForwardPPVideoBuffered(
		context.Background(),
		nil,
		account,
		PPVideoOperationGeneric,
		"task-pending",
		nil,
		PPVideoPublicRequest{Model: "seedance2.5-one-720p"},
	)

	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusProcessing, result.TaskStatus)
	require.Equal(t, PPVideoTaskStatusProcessing, gjson.GetBytes(result.ResponseBody, "status").String())
	require.False(t, gjson.GetBytes(result.ResponseBody, "content").Exists())
	require.False(t, gjson.GetBytes(result.ResponseBody, "video_url").Exists())
	require.False(t, gjson.GetBytes(result.ResponseBody, "result_url").Exists())
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://video.ctmoai.com/v1/videos/task-pending", upstream.requests[0].URL.String())
}

func TestCTMOSeedanceModelSyncRequestAndFiltering(t *testing.T) {
	t.Parallel()
	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	account := &Account{
		Platform: PlatformCTMOSeedance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "ctmo-key",
		},
	}
	req, err := svc.buildPPVideoUpstreamModelsRequest(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "https://video.ctmoai.com/v1/models", req.URL.String())
	require.Equal(t, "Bearer ctmo-key", req.Header.Get("Authorization"))
	require.Equal(t,
		[]string{"seedance-2.0-930", "seedance2.5-one-720p"},
		filterUpstreamModelsForPlatform(PlatformCTMOSeedance, []string{
			"future-seedance-model", "seedance-2.0-930", "seedance2.5-one-720p",
		}),
	)
}

func TestForwardCTMOSeedanceContent(t *testing.T) {
	t.Parallel()
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"video/mp4"}},
		Body:       io.NopCloser(bytes.NewBufferString("video")),
	}}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          42,
		Platform:    PlatformCTMOSeedance,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "ctmo-key", "base_url": "https://video.ctmoai.com"},
	}

	resp, err := svc.ForwardCTMOSeedanceContent(context.Background(), account, "task-1", "bytes=0-1023")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://video.ctmoai.com/v1/videos/task-1/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer ctmo-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-1023", upstream.requests[0].Header.Get("Range"))
}
