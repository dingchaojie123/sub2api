package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardPPVideoBufferedUsesSubmissionAndStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	submissionBody := []byte(`{"model_name":"kling-v3","prompt":"a cat by the sea","duration":"5","mode":"pro"}`)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"provider-task-1","status":"processing"}`))),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{"id":"provider-task-1","status":"succeeded","usage":{"duration":5.041,"video_count":1,"resolution":"1080p"}}`,
				))),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          7,
		Platform:    PlatformKling,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-pp-token", "base_url": "https://pp.example/v1"},
	}

	submissionRecorder := httptest.NewRecorder()
	submissionContext, _ := gin.CreateTestContext(submissionRecorder)
	submissionContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/text2video", bytes.NewReader(submissionBody))
	submission, err := svc.ForwardPPVideoBuffered(
		context.Background(),
		submissionContext,
		account,
		PPVideoOperationKlingTextToVideo,
		"",
		submissionBody,
	)

	require.NoError(t, err)
	require.Equal(t, "provider-task-1", submission.ResponseID)
	require.Equal(t, PPVideoTaskStatusProcessing, submission.TaskStatus)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.MethodPost, upstream.requests[0].Method)
	require.Equal(t, "https://pp.example/v1/videos/text2video", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer test-pp-token", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.requests[0].Header.Get("Content-Type"))
	require.JSONEq(t, string(submissionBody), string(upstream.bodies[0]))
	require.JSONEq(t, `{
		"id": "provider-task-1",
		"task_id": "provider-task-1",
		"object": "video.generation.task",
		"status": "processing",
		"model": "kling-v3"
	}`, string(submission.ResponseBody))

	statusRecorder := httptest.NewRecorder()
	statusContext, _ := gin.CreateTestContext(statusRecorder)
	statusContext.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/text2video/provider-task-1", nil)
	status, err := svc.ForwardPPVideoBuffered(
		context.Background(),
		statusContext,
		account,
		PPVideoOperationKlingTextToVideo,
		"provider-task-1",
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, status.TaskStatus)
	require.Equal(t, int64(5041), status.VideoDurationMilliseconds)
	require.Equal(t, VideoBillingResolution1080P, status.VideoResolution)
	require.Equal(t, "application/json", status.ResponseContentType)
	require.Contains(t, string(status.ResponseBody), `"task_id":"provider-task-1"`)
	require.Contains(t, string(status.ResponseBody), `"duration_ms":5041`)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, http.MethodGet, upstream.requests[1].Method)
	require.Equal(t, "https://pp.example/v1/videos/text2video/provider-task-1", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer test-pp-token", upstream.requests[1].Header.Get("Authorization"))
}

func TestForwardPPVideoBufferedMarksUpstreamErrorResponseCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"code":"model_not_found"}}`))),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          9,
		Platform:    PlatformKling,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-pp-token", "base_url": "https://pp.example/v1"},
	}
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"kling-v3","prompt":"waves","duration":5}`)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(body))

	_, err := svc.ForwardPPVideoBuffered(
		context.Background(),
		ginContext,
		account,
		PPVideoOperationKlingTextToVideo,
		"",
		body,
	)

	require.Error(t, err)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.JSONEq(t, `{"error":{"code":"model_not_found"}}`, recorder.Body.String())
	require.True(t, IsResponseCommitted(ginContext))
}

func TestForwardPPVideoBufferedBuffersRetryableStatusError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"temporarily unavailable"}}`))),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          10,
		Platform:    PlatformKling,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-pp-token", "base_url": "https://pp.example/v1"},
	}
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task-1", nil)

	_, err := svc.ForwardPPVideoBuffered(
		context.Background(),
		ginContext,
		account,
		PPVideoOperationKlingTextToVideo,
		"task-1",
		nil,
	)

	require.ErrorContains(t, err, "HTTP 503")
	require.Empty(t, recorder.Body.String())
	require.False(t, IsResponseCommitted(ginContext))
}

func TestForwardPPVideoBufferedHandlesUpstreamErrorWithoutContext(t *testing.T) {
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"code":"model_not_found"}}`))),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          10,
		Platform:    PlatformKling,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-pp-token", "base_url": "https://pp.example/v1"},
	}
	body := []byte(`{"model":"kling-v3","prompt":"waves","duration":5}`)
	var forwardErr error

	require.NotPanics(t, func() {
		_, forwardErr = svc.ForwardPPVideoBuffered(
			context.Background(),
			nil,
			account,
			PPVideoOperationKlingTextToVideo,
			"",
			body,
		)
	})
	require.Error(t, forwardErr)
}

func TestPPVideoAccountEligibilityRespectsModelWhitelist(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"seedance-a": "upstream-a"},
		},
	}

	require.True(t, isPPVideoAccountEligibleForModel(account, "seedance-a"))
	require.False(t, isPPVideoAccountEligibleForModel(account, "seedance-b"))
	require.True(t, isPPVideoAccountEligibleForModel(account, ""))
}

func TestPPVideoAccountEligibilityAllowsSeedanceMappedUpstreamModel(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				SiteVideoDefaultModel: "doubao-seedance-2-0-mini-260615",
			},
		},
	}

	require.False(t, isPPVideoAccountEligibleForModel(account, SiteVideoDefaultModel))
	require.True(t, isPPVideoAccountEligibleForModel(account, "doubao-seedance-2-0-mini-260615"))
	require.False(t, isPPVideoAccountEligibleForModel(account, "doubao-seedance-2-0-pro-260615"))
}

func TestPPVideoAccountEligibilityAllowsMappedUpstreamModelsForAllPPPlatforms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		platform      string
		mapping       map[string]any
		upstreamModel string
		unsupported   string
	}{
		{
			name:     "Kling",
			platform: PlatformKling,
			mapping: map[string]any{
				"kling-public": "kling-v3",
			},
			upstreamModel: "kling-v3",
			unsupported:   "kling-v4",
		},
		{
			name:     "Happy Horse",
			platform: PlatformHappyHourse,
			mapping: map[string]any{
				"happy-public": "happyhorse-1.0-t2v",
			},
			upstreamModel: "happyhorse-1.0-t2v",
			unsupported:   "happyhorse-1.0-i2v",
		},
		{
			name:     "Seedance",
			platform: PlatformSeedance,
			mapping: map[string]any{
				"seedance-public": "doubao-seedance-2-0-mini-260615",
			},
			upstreamModel: "doubao-seedance-2-0-mini-260615",
			unsupported:   "doubao-seedance-2-0-pro-260615",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			account := &Account{
				Platform:    tt.platform,
				Credentials: map[string]any{"model_mapping": tt.mapping},
			}

			require.True(t, isPPVideoAccountEligibleForModel(account, tt.upstreamModel))
			require.False(t, isPPVideoAccountEligibleForModel(account, tt.unsupported))
		})
	}
}

func TestPPVideoAccountEligibilityUsesMappedUpstreamForExplicitPublicModel(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"doubao-seedance-2-0-260128": "doubao-seedance-2-0-260128",
			},
		},
	}

	require.True(t, isPPVideoAccountEligibleForRequest(account, PPVideoPublicRequest{
		Model:            "doubao-seedance-2-0-260128",
		HasExplicitModel: true,
	}))
}

func TestPPVideoAccountEligibilityRejectsJimengModelOnSeedanceAccount(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"doubao-seedance-2-0-260128": "doubao-seedance-2-0-260128",
			},
		},
	}

	require.False(t, isPPVideoAccountEligibleForRequest(account, PPVideoPublicRequest{
		Model:            "by-seedance2.0-933",
		HasExplicitModel: true,
	}))
}

func TestPPVideoAccountEligibilityRejectsForeignPlatformModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		account *Account
		model   string
	}{
		{
			name: "Kling rejects Seedance model without whitelist",
			account: &Account{
				Platform: PlatformKling,
			},
			model: "doubao-seedance-2-0-mini-260615",
		},
		{
			name: "Kling rejects Seedance alias even when mapped",
			account: &Account{
				Platform: PlatformKling,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"seedance2.0-900": "kling-v3",
					},
				},
			},
			model: "seedance2.0-900",
		},
		{
			name: "Seedance rejects Kling model",
			account: &Account{
				Platform: PlatformSeedance,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"doubao-seedance-2-0-260128": "doubao-seedance-2-0-260128",
					},
				},
			},
			model: "kling-v3",
		},
		{
			name: "Happy Horse rejects Jimeng model",
			account: &Account{
				Platform: PlatformHappyHourse,
			},
			model: "by-seedance2.0-933",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.False(t, isPPVideoAccountEligibleForRequest(tt.account, PPVideoPublicRequest{
				Model:            tt.model,
				HasExplicitModel: true,
			}))
			require.False(t, isPPVideoAccountEligibleForModel(tt.account, tt.model))
		})
	}
}

func TestPPVideoAccountEligibilityUsesPlatformDefaultForOmittedModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		account     *Account
		public      PPVideoPublicRequest
		unsupported PPVideoPublicRequest
	}{
		{
			name: "Happy Horse text to video",
			account: &Account{
				Platform: PlatformHappyHourse,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"happyhorse-1.0-t2v": "happyhorse-1.0-t2v",
					},
				},
			},
			public: PPVideoPublicRequest{
				Model: "",
			},
			unsupported: PPVideoPublicRequest{
				Model:            "happyhorse-1.0-i2v",
				HasExplicitModel: true,
			},
		},
		{
			name: "Happy Horse image to video",
			account: &Account{
				Platform: PlatformHappyHourse,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"happyhorse-1.0-i2v": "happyhorse-1.0-i2v",
					},
				},
			},
			public: PPVideoPublicRequest{
				Model:    "",
				HasImage: true,
			},
			unsupported: PPVideoPublicRequest{
				Model:            "happyhorse-1.0-t2v",
				HasImage:         true,
				HasExplicitModel: true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.True(t, isPPVideoAccountEligibleForRequest(tt.account, tt.public))
			require.False(t, isPPVideoAccountEligibleForRequest(tt.account, tt.unsupported))
		})
	}
}

func TestForwardPPVideoBufferedAdaptsPublicRequestToSeedance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	submissionBody := []byte(`{"model":"video-v1","prompt":"waves"}`)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"data":{"task_id":"seedance-task-1","status":"queued"}}`))),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          8,
		Platform:    PlatformSeedance,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-pp-token", "base_url": "https://pp.example/v1"},
	}
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(submissionBody))

	result, err := svc.ForwardPPVideoBuffered(
		context.Background(),
		ginContext,
		account,
		PPVideoOperationGeneric,
		"",
		submissionBody,
	)

	require.NoError(t, err)
	require.Equal(t, "seedance-task-1", result.ResponseID)
	require.Equal(t, "video-v1", result.Model)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://pp.example/v1/video/generations", upstream.requests[0].URL.String())
	require.JSONEq(t, `{
		"model": "doubao-seedance-2-0-260128",
		"prompt": "waves",
		"duration": 5,
		"resolution": "720p",
		"n": 1
	}`, string(upstream.bodies[0]))
	require.JSONEq(t, `{
		"data": {
			"task_id": "seedance-task-1",
			"status": "queued"
		},
		"id": "seedance-task-1",
		"task_id": "seedance-task-1",
		"object": "video.generation.task",
		"status": "processing",
		"model": "video-v1"
	}`, string(result.ResponseBody))
}
