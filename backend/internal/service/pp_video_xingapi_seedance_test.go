package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestXingAPISeedanceModelsPathsAndHeaders(t *testing.T) {
	t.Parallel()

	models := []string{
		"seedance-933",
		"Doubao-Seedance-Pro",
		"vendor-seedance2.5-special",
		"gpt-image-2",
		"wan3.0-video",
	}
	require.Equal(t, models[:3], filterUpstreamModelsForPlatform(PlatformXingAPISeedance, models))
	require.Equal(t, models[:3], filterPPVideoPublicModelsForPlatform(PlatformXingAPISeedance, models))

	createPath, err := PPVideoUpstreamPath(PlatformXingAPISeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/generations", createPath)

	statusPath, err := PPVideoStatusUpstreamPath(PlatformXingAPISeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformXingAPISeedance, "xing-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer xing-key", headers.Get("Authorization"))

	parsed, err := ParsePPVideoResponse(PlatformXingAPISeedance, []byte(`{"id":"task-x","status":"completed"}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)
	normalized := NormalizePPVideoPublicResponse(
		PlatformXingAPISeedance,
		[]byte(`{"id":"task-x","status":"completed"}`),
		PPVideoPublicRequest{Model: "seedance-933"},
		parsed,
	)
	require.JSONEq(t, `{
		"id":"task-x",
		"task_id":"task-x",
		"object":"video.generation.task",
		"status":"succeeded",
		"model":"seedance-933",
		"video_url":"/v1/videos/task-x/content",
		"url":"/v1/videos/task-x/content",
		"result_url":"/v1/videos/task-x/content",
		"videos":[{"url":"/v1/videos/task-x/content","video_url":"/v1/videos/task-x/content"}],
		"data":{
			"result_url":"/v1/videos/task-x/content",
			"url":"/v1/videos/task-x/content",
			"video_url":"/v1/videos/task-x/content",
			"videos":[{"url":"/v1/videos/task-x/content","video_url":"/v1/videos/task-x/content"}]
		}
	}`, string(normalized))
}

func TestXingAPISeedancePreservesNativePayloadExceptMappedModel(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformXingAPISeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"public-seedance-model": "vendor-seedance-model",
			},
		},
	}
	body := []byte(`{
		"model":"public-seedance-model",
		"prompt":"lake",
		"seconds":8,
		"aspect_ratio":"9:16",
		"images":["https://example.com/a.png"],
		"vendor_options":{"motion":"large"}
	}`)

	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	require.Equal(t, "public-seedance-model", public.Model)
	require.Equal(t, "vendor-seedance-model", public.UpstreamModel)
	require.JSONEq(t, `{
		"model":"vendor-seedance-model",
		"prompt":"lake",
		"seconds":8,
		"aspect_ratio":"9:16",
		"images":["https://example.com/a.png"],
		"vendor_options":{"motion":"large"}
	}`, string(prepared))
}

func TestXingAPISeedanceUsesDefaultModelsAndVideoEndpoints(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          42,
		Platform:    PlatformXingAPISeedance,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "xing-key"},
	}
	modelSvc := &AccountTestService{cfg: &config.Config{Security: config.SecurityConfig{
		URLAllowlist: config.URLAllowlistConfig{Enabled: false},
	}}}
	modelReq, err := modelSvc.buildPPVideoUpstreamModelsRequest(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "https://xingapi.top/v1/models", modelReq.URL.String())
	require.Equal(t, "Bearer xing-key", modelReq.Header.Get("Authorization"))

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(bytes.NewBufferString(`{"data":[
				{"id":"seedance-933"},
				{"id":"Doubao-Seedance-Pro"},
				{"id":"vendor-seedance2.5-special"},
				{"id":"gpt-image-2"}
			]}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"id":"task-x","status":"queued"}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"video/mp4"}},
			Body:       io.NopCloser(bytes.NewBufferString("video")),
		},
	}}
	modelSvc.httpUpstream = upstream
	models, err := modelSvc.FetchUpstreamSupportedModels(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []string{"Doubao-Seedance-Pro", "seedance-933", "vendor-seedance2.5-special"}, models)
	require.Equal(t, "https://xingapi.top/v1/models", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer xing-key", upstream.requests[0].Header.Get("Authorization"))

	gateway := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	result, err := gateway.ForwardPPVideoBuffered(
		context.Background(),
		nil,
		account,
		PPVideoOperationGeneric,
		"",
		[]byte(`{"model":"seedance-933","prompt":"lake","duration":5}`),
	)
	require.NoError(t, err)
	require.Equal(t, "task-x", result.ResponseID)
	require.Equal(t, "https://xingapi.top/v1/videos/generations", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer xing-key", upstream.requests[1].Header.Get("Authorization"))

	contentResp, err := gateway.ForwardPPVideoContent(context.Background(), account, "task-x", "", "")
	require.NoError(t, err)
	defer contentResp.Body.Close()
	require.Equal(t, "https://xingapi.top/v1/videos/task-x/content", upstream.requests[2].URL.String())
	require.Equal(t, "Bearer xing-key", upstream.requests[2].Header.Get("Authorization"))
}

func TestXingAPISeedanceUsesExactPerModelChannelPricing(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		mode        BillingMode
		price       float64
		wantFormula string
		wantUnits   float64
	}{
		{name: "per request", mode: BillingModePerRequest, price: 3.5, wantFormula: PPVideoBillingFormulaPerRequest, wantUnits: 2},
		{name: "per second", mode: BillingModeVideo, price: 0.8, wantFormula: PPVideoBillingFormulaPerSecond, wantUnits: 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			groupID := int64(209)
			price := tt.price
			svc := &OpenAIGatewayService{
				billingService: newTestBillingService(),
				resolver: newPPVideoResolverWithChannel(t, groupID, PlatformXingAPISeedance, []ChannelModelPricing{{
					Platform:        PlatformXingAPISeedance,
					Models:          []string{"seedance-933"},
					BillingMode:     tt.mode,
					PerRequestPrice: &price,
				}}),
			}
			apiKey := &APIKey{GroupID: &groupID, Group: &Group{
				Platform:             PlatformXingAPISeedance,
				RateMultiplier:       1,
				VideoRateIndependent: true,
				VideoRateMultiplier:  1,
			}}

			cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
				Platform:                      PlatformXingAPISeedance,
				Model:                         "seedance-933",
				RequestedDurationMilliseconds: 5000,
				VideoCount:                    2,
			})

			require.NoError(t, err)
			require.Equal(t, tt.wantFormula, cost.BillingFormula)
			require.InDelta(t, tt.wantUnits, cost.BillingUnits, 1e-12)
			require.InDelta(t, tt.wantUnits*tt.price, cost.TotalCost, 1e-12)
		})
	}
}

func TestXingAPISeedanceMatchingDoesNotRejectOtherPlatformAliases(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformKling,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"custom-seedance-alias": "kling-v3"},
		},
	}
	_, _, err := PreparePPVideoRequestBodyForAccount(
		account,
		PPVideoOperationKlingTextToVideo,
		[]byte(`{"model":"custom-seedance-alias","prompt":"lake","duration":5,"mode":"std"}`),
	)
	require.NoError(t, err)
}
