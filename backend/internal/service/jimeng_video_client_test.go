package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type jimengHTTPUpstreamRecorder struct {
	lastReq      *http.Request
	lastBody     []byte
	lastProxyURL string
	requestURLs  []string
	requests     []*http.Request
	bodies       [][]byte
	responses    []*http.Response
	resp         *http.Response
}

func (u *jimengHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	u.lastProxyURL = proxyURL
	if req != nil {
		u.requestURLs = append(u.requestURLs, req.URL.String())
		u.requests = append(u.requests, req)
	}
	if req != nil && req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.lastBody = body
		u.bodies = append(u.bodies, body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	if len(u.responses) > 0 {
		resp := u.responses[0]
		u.responses = u.responses[1:]
		return resp, nil
	}
	return u.resp, nil
}

func (u *jimengHTTPUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestJimengVideoClientCreateGenerationUsesVideoEndpointAndBearerAuth(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		_, _ = w.Write([]byte(`{"task_id":"task_123","status":"processing"}`))
	}))
	defer server.Close()

	client, err := NewJimengVideoClient(server.URL+"/v1", "jimeng-key", server.Client())
	require.NoError(t, err)

	result, err := client.CreateGeneration(context.Background(), JimengVideoGenerationRequest{
		Model:    "video-v1",
		Prompt:   "make a short video",
		Duration: 5,
		Extra: map[string]any{
			"seed": float64(7),
		},
	})

	require.NoError(t, err)
	require.Equal(t, "/v1/videos/generations", gotPath)
	require.Equal(t, "Bearer jimeng-key", gotAuth)
	require.Equal(t, "video-v1", gotBody["model"])
	require.Equal(t, "make a short video", gotBody["prompt"])
	require.Equal(t, float64(5), gotBody["duration"])
	require.Equal(t, float64(7), gotBody["seed"])
	require.Equal(t, "task_123", result.TaskID)
	require.Equal(t, JimengTaskStatusProcessing, result.Status)
}

func TestJimengVideoClientQueryGenerationExtractsNestedTaskIDAndNormalizesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/videos/task_456", r.URL.Path)
		require.Equal(t, "Bearer jimeng-key", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":{"id":"task_456","status":"completed"}}`))
	}))
	defer server.Close()

	client, err := NewJimengVideoClient(server.URL, "jimeng-key", server.Client())
	require.NoError(t, err)

	result, err := client.GetGeneration(context.Background(), "task_456")

	require.NoError(t, err)
	require.Equal(t, "task_456", result.TaskID)
	require.Equal(t, JimengTaskStatusSucceeded, result.Status)
}

func TestParseJimengGenerationResultExtractsOpenAICompatibleUsage(t *testing.T) {
	result, err := parseJimengGenerationResult([]byte(`{
		"data": {
			"id": "task_usage",
			"status": "done",
			"usage": {
				"prompt_tokens": 17,
				"completion_tokens": 23,
				"cache_read_input_tokens": 5
			}
		}
	}`))

	require.NoError(t, err)
	require.True(t, result.HasUsage)
	require.Equal(t, "task_usage", result.TaskID)
	require.Equal(t, 17, result.Usage.InputTokens)
	require.Equal(t, 23, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.CacheReadInputTokens)
}

func TestForwardJimengVideoGenerationUsesAccountCredentialAndReturnsUsage(t *testing.T) {
	upstream := &jimengHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"req_upstream"}},
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"task_id":"task_forward",
			"status":"processing",
			"usage":{"input_tokens":11,"output_tokens":13}
		}`))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader([]byte(`{"model":"video-v1","prompt":"hello"}`)))

	result, err := svc.ForwardJimengVideo(context.Background(), c, &Account{
		ID:          7,
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "jimeng-upstream-key", "base_url": "https://jimeng.example/v1"},
	}, JimengVideoEndpointGenerations, "", []byte(`{"model":"video-v1","prompt":"hello"}`))

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "task_forward", response["id"])
	require.Equal(t, "task_forward", response["task_id"])
	require.Equal(t, "video.generation.task", response["object"])
	require.Equal(t, JimengTaskStatusProcessing, response["status"])
	require.Equal(t, JimengVideoRoutingModel, response["model"])
	require.Equal(t, "https://jimeng.example/v1/videos/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer jimeng-upstream-key", upstream.lastReq.Header.Get("Authorization"))
	require.JSONEq(t, `{"model":"by-seedance2.0-933","prompt":"hello"}`, string(upstream.lastBody))
	require.Equal(t, "task_forward", result.ResponseID)
	require.False(t, result.HasUsage)
	require.True(t, result.Usage.InputTokens > 0)
	require.Equal(t, JimengVideoRoutingModel, result.Model)
	require.Equal(t, JimengVideoBillingModel, result.BillingModel)
	require.Equal(t, JimengVideoRoutingModel, result.UpstreamModel)
}

func TestForwardJimengVideoGenerationNormalizesLegacyModelForUpstream(t *testing.T) {
	upstream := &jimengHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"task_id":"task_legacy","status":"processing"}`))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"seedance 2.0","prompt":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))

	_, err := svc.ForwardJimengVideo(context.Background(), c, &Account{
		ID:          7,
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "jimeng-upstream-key", "base_url": "https://jimeng.example/v1"},
	}, JimengVideoEndpointGenerations, "", body)

	require.NoError(t, err)
	require.JSONEq(t, `{"model":"by-seedance2.0-933","prompt":"hello"}`, string(upstream.lastBody))
}

func TestForwardJimengVideoGenerationPreservesSelectedSeedanceModel(t *testing.T) {
	upstream := &jimengHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"task_id":"task_selected","status":"processing"}`))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"seedance2.0-431","prompt":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))

	result, err := svc.ForwardJimengVideo(context.Background(), c, &Account{
		ID:          7,
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "jimeng-upstream-key", "base_url": "https://jimeng.example/v1"},
	}, JimengVideoEndpointGenerations, "", body)

	require.NoError(t, err)
	require.JSONEq(t, `{"model":"seedance2.0-431","prompt":"hello"}`, string(upstream.lastBody))
	require.Equal(t, "seedance2.0-431", result.Model)
	require.Equal(t, JimengVideoBillingModel, result.BillingModel)
	require.Equal(t, "seedance2.0-431", result.UpstreamModel)
	require.JSONEq(t, `{
		"id": "task_selected",
		"task_id": "task_selected",
		"object": "video.generation.task",
		"status": "processing",
		"model": "seedance2.0-431"
	}`, string(result.ResponseBody))
}

func TestForwardJimengVideoGenerationFallsBackToLegacyEndpoint(t *testing.T) {
	upstream := &jimengHTTPUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"not found"}}`))),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"task_id":"task_legacy_submit","status":"submitted"}`))),
		},
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"video-v1","prompt":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(body))

	result, err := svc.ForwardJimengVideoBuffered(context.Background(), c, &Account{
		ID:          7,
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "jimeng-upstream-key", "base_url": "https://jimeng.example/v1"},
	}, JimengVideoEndpointGenerations, "", body)

	require.NoError(t, err)
	require.Equal(t, []string{
		"https://jimeng.example/v1/videos/generations",
		"https://jimeng.example/v1/video/generations",
	}, upstream.requestURLs)
	require.Len(t, upstream.bodies, 2)
	require.JSONEq(t, `{"model":"by-seedance2.0-933","prompt":"hello"}`, string(upstream.bodies[0]))
	require.JSONEq(t, `{"model":"by-seedance2.0-933","prompt":"hello"}`, string(upstream.bodies[1]))
	require.Equal(t, JimengTaskStatusProcessing, result.TaskStatus)
	require.Equal(t, "/v1/video/generations", result.UpstreamEndpoint)
}

func TestForwardJimengVideoStatusFallsBackToLegacyQueryEndpoint(t *testing.T) {
	upstream := &jimengHTTPUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"not found"}}`))),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"task_fallback","status":"completed"}`))),
		},
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_fallback", nil)

	result, err := svc.ForwardJimengVideoBuffered(context.Background(), c, &Account{
		ID:          7,
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "jimeng-upstream-key", "base_url": "https://jimeng.example/v1"},
	}, JimengVideoEndpointStatus, "task_fallback", nil)

	require.NoError(t, err)
	require.Equal(t, []string{
		"https://jimeng.example/v1/videos/task_fallback",
		"https://jimeng.example/v1/video/generations/task_fallback",
	}, upstream.requestURLs)
	require.Equal(t, JimengTaskStatusSucceeded, result.TaskStatus)
	require.Equal(t, "/v1/video/generations/{task_id}", result.UpstreamEndpoint)
}

func TestForwardJimengVideoStatusTreatsVideoURLAsCompletedResult(t *testing.T) {
	upstream := &jimengHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"data": {
				"id": "task_completed_by_url",
				"status": "processing",
				"task_result": {
					"videos": [{"url": "https://cdn.example.com/completed.mp4"}]
				}
			}
		}`))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_completed_by_url", nil)

	result, err := svc.ForwardJimengVideoBuffered(context.Background(), c, &Account{
		ID:          7,
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "jimeng-upstream-key", "base_url": "https://jimeng.example/v1"},
	}, JimengVideoEndpointStatus, "task_completed_by_url", nil, "seedance2.0-431")

	require.NoError(t, err)
	require.Equal(t, JimengTaskStatusSucceeded, result.TaskStatus)
	require.Equal(t, "seedance2.0-431", result.Model)
	require.Equal(t, JimengVideoBillingModel, result.BillingModel)
	require.Equal(t, "seedance2.0-431", result.UpstreamModel)
	require.JSONEq(t, `{
		"data": {
			"id": "task_completed_by_url",
			"status": "processing",
			"result_url": "https://cdn.example.com/completed.mp4",
			"url": "https://cdn.example.com/completed.mp4",
			"video_url": "https://cdn.example.com/completed.mp4",
			"videos": [{"url": "https://cdn.example.com/completed.mp4", "video_url": "https://cdn.example.com/completed.mp4"}],
			"task_result": {
				"videos": [{"url": "https://cdn.example.com/completed.mp4"}]
			}
		},
		"id": "task_completed_by_url",
		"task_id": "task_completed_by_url",
		"object": "video.generation.task",
		"status": "succeeded",
		"model": "seedance2.0-431",
		"result_url": "https://cdn.example.com/completed.mp4",
		"url": "https://cdn.example.com/completed.mp4",
		"videos": [{"url": "https://cdn.example.com/completed.mp4", "video_url": "https://cdn.example.com/completed.mp4"}],
		"video_url": "https://cdn.example.com/completed.mp4"
	}`, string(result.ResponseBody))
}

func TestForwardJimengVideoGenerationDoesNotBillAcceptedTask(t *testing.T) {
	upstream := &jimengHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"task_id":"task_video","status":"processing"}`))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"video-v1","prompt":"hello","duration":10,"resolution":"720p"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(body))

	result, err := svc.ForwardJimengVideo(context.Background(), c, &Account{
		ID:          7,
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "jimeng-upstream-key", "base_url": "https://jimeng.example/v1"},
	}, JimengVideoEndpointGenerations, "", body)

	require.NoError(t, err)
	require.False(t, result.HasUsage)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, VideoBillingResolution720P, result.VideoResolution)
	require.Equal(t, 10, result.VideoDurationSeconds)
	require.Equal(t, JimengTaskStatusProcessing, result.TaskStatus)
	require.Equal(t, http.StatusOK, result.ResponseStatusCode)
	require.JSONEq(t, `{
		"id": "task_video",
		"task_id": "task_video",
		"object": "video.generation.task",
		"status": "processing",
		"model": "by-seedance2.0-933"
	}`, string(result.ResponseBody))
}

func TestNormalizeJimengVideoPublicResponseAddsStableFields(t *testing.T) {
	raw := []byte(`{
		"data": {
			"task_id": "jimeng-task-1",
			"status": "SUCCESS",
			"task_result": {
				"videos": [{"url": "https://cdn.example.com/video.mp4"}]
			}
		}
	}`)
	parsed, err := parseJimengGenerationResult(raw)
	require.NoError(t, err)

	body := NormalizeJimengVideoPublicResponse(raw, parsed)

	require.JSONEq(t, `{
		"data": {
			"task_id": "jimeng-task-1",
			"status": "SUCCESS",
			"result_url": "https://cdn.example.com/video.mp4",
			"url": "https://cdn.example.com/video.mp4",
			"video_url": "https://cdn.example.com/video.mp4",
			"videos": [{"url": "https://cdn.example.com/video.mp4", "video_url": "https://cdn.example.com/video.mp4"}],
			"task_result": {
				"videos": [{"url": "https://cdn.example.com/video.mp4"}]
			}
		},
		"id": "jimeng-task-1",
		"task_id": "jimeng-task-1",
		"object": "video.generation.task",
		"status": "succeeded",
		"model": "by-seedance2.0-933",
		"result_url": "https://cdn.example.com/video.mp4",
		"url": "https://cdn.example.com/video.mp4",
		"videos": [{"url": "https://cdn.example.com/video.mp4", "video_url": "https://cdn.example.com/video.mp4"}],
		"video_url": "https://cdn.example.com/video.mp4"
	}`, string(body))
}

func TestParseJimengGenerationResultInfersSucceededWhenVideoURLExists(t *testing.T) {
	raw := []byte(`{
		"data": {
			"id": "jimeng-task-with-video",
			"task_result": {
				"videos": [{"url": "https://cdn.example.com/final.mp4"}]
			}
		}
	}`)

	result, err := parseJimengGenerationResult(raw)

	require.NoError(t, err)
	require.Equal(t, "jimeng-task-with-video", result.TaskID)
	require.Equal(t, JimengTaskStatusSucceeded, result.Status)
}

func TestNormalizeJimengVideoPublicResponseMarksVideoResultSucceeded(t *testing.T) {
	raw := []byte(`{
		"data": {
			"task_id": "jimeng-task-processing-with-video",
			"status": "processing",
			"result": {
				"video_url": "https://cdn.example.com/final-from-result.mp4"
			}
		}
	}`)
	parsed, err := parseJimengGenerationResult(raw)
	require.NoError(t, err)

	body := NormalizeJimengVideoPublicResponse(raw, parsed)

	require.JSONEq(t, `{
		"data": {
			"task_id": "jimeng-task-processing-with-video",
			"status": "processing",
			"result": {
				"video_url": "https://cdn.example.com/final-from-result.mp4"
			},
			"result_url": "https://cdn.example.com/final-from-result.mp4",
			"url": "https://cdn.example.com/final-from-result.mp4",
			"videos": [{"url": "https://cdn.example.com/final-from-result.mp4", "video_url": "https://cdn.example.com/final-from-result.mp4"}],
			"video_url": "https://cdn.example.com/final-from-result.mp4"
		},
		"id": "jimeng-task-processing-with-video",
		"task_id": "jimeng-task-processing-with-video",
		"object": "video.generation.task",
		"status": "succeeded",
		"model": "by-seedance2.0-933",
		"result_url": "https://cdn.example.com/final-from-result.mp4",
		"url": "https://cdn.example.com/final-from-result.mp4",
		"videos": [{"url": "https://cdn.example.com/final-from-result.mp4", "video_url": "https://cdn.example.com/final-from-result.mp4"}],
		"video_url": "https://cdn.example.com/final-from-result.mp4"
	}`, string(body))
}

func TestNormalizeJimengVideoPublicResponseFindsSeedanceNestedContentVideoURL(t *testing.T) {
	raw := []byte(`{
		"data": {
			"data": {
				"id": "jimeng-seedance-431",
				"status": "completed",
				"content": {
					"video_url": "https://cdn.example.com/seedance-431.mp4"
				}
			}
		}
	}`)
	parsed, err := parseJimengGenerationResult(raw)
	require.NoError(t, err)

	body := NormalizeJimengVideoPublicResponse(raw, parsed, "seedance2.0-431")

	require.JSONEq(t, `{
		"data": {
			"data": {
				"id": "jimeng-seedance-431",
				"status": "completed",
				"content": {
					"video_url": "https://cdn.example.com/seedance-431.mp4"
				}
			},
			"result_url": "https://cdn.example.com/seedance-431.mp4",
			"url": "https://cdn.example.com/seedance-431.mp4",
			"video_url": "https://cdn.example.com/seedance-431.mp4",
			"videos": [{"url": "https://cdn.example.com/seedance-431.mp4", "video_url": "https://cdn.example.com/seedance-431.mp4"}]
		},
		"id": "jimeng-seedance-431",
		"task_id": "jimeng-seedance-431",
		"object": "video.generation.task",
		"status": "succeeded",
		"model": "seedance2.0-431",
		"result_url": "https://cdn.example.com/seedance-431.mp4",
		"url": "https://cdn.example.com/seedance-431.mp4",
		"videos": [{"url": "https://cdn.example.com/seedance-431.mp4", "video_url": "https://cdn.example.com/seedance-431.mp4"}],
		"video_url": "https://cdn.example.com/seedance-431.mp4"
	}`, string(body))
}

func TestNormalizeJimengVideoPublicResponseFindsDownloadURLInVideosArray(t *testing.T) {
	raw := []byte(`{
		"data": {
			"task_id": "jimeng-download-url",
			"status": "processing",
			"task_result": {
				"videos": [{"download_url": "https://cdn.example.com/download-final.mp4"}]
			}
		}
	}`)
	parsed, err := parseJimengGenerationResult(raw)
	require.NoError(t, err)

	body := NormalizeJimengVideoPublicResponse(raw, parsed, "seedance2.0-431")

	require.JSONEq(t, `{
		"data": {
			"task_id": "jimeng-download-url",
			"status": "processing",
			"result_url": "https://cdn.example.com/download-final.mp4",
			"url": "https://cdn.example.com/download-final.mp4",
			"video_url": "https://cdn.example.com/download-final.mp4",
			"videos": [{"url": "https://cdn.example.com/download-final.mp4", "video_url": "https://cdn.example.com/download-final.mp4"}],
			"task_result": {
				"videos": [{"download_url": "https://cdn.example.com/download-final.mp4"}]
			}
		},
		"id": "jimeng-download-url",
		"task_id": "jimeng-download-url",
		"object": "video.generation.task",
		"status": "succeeded",
		"model": "seedance2.0-431",
		"result_url": "https://cdn.example.com/download-final.mp4",
		"url": "https://cdn.example.com/download-final.mp4",
		"videos": [{"url": "https://cdn.example.com/download-final.mp4", "video_url": "https://cdn.example.com/download-final.mp4"}],
		"video_url": "https://cdn.example.com/download-final.mp4"
	}`, string(body))
}

func TestJimengVideoModelsUseAliasesAndAccountMapping(t *testing.T) {
	legacyAccount := &Account{
		Platform: PlatformJimeng,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"seedance 2.0": "seedance 2.0"},
		},
	}
	selectedModelAccount := &Account{
		Platform: PlatformJimeng,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"seedance2.0-431": "seedance2.0-431"},
		},
	}

	require.Equal(t, "by-seedance2.0-933", JimengVideoRoutingModel)
	require.True(t, legacyAccount.IsModelSupported(JimengVideoRoutingModel))
	require.False(t, legacyAccount.IsModelSupported("seedance2.0-431"))
	require.True(t, selectedModelAccount.IsModelSupported("seedance2.0-431"))
	require.Equal(t, "video-v1", JimengVideoBillingModel)
	require.Equal(t, JimengVideoBillingModel, JimengVideoDefaultModel)
}

func TestNormalizeJimengTaskStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "pending", want: JimengTaskStatusProcessing},
		{input: "submitted", want: JimengTaskStatusProcessing},
		{input: "in_progress", want: JimengTaskStatusProcessing},
		{input: "processing", want: JimengTaskStatusProcessing},
		{input: "success", want: JimengTaskStatusSucceeded},
		{input: "finished", want: JimengTaskStatusSucceeded},
		{input: "completed", want: JimengTaskStatusSucceeded},
		{input: "done", want: JimengTaskStatusSucceeded},
		{input: "failed", want: JimengTaskStatusFailed},
		{input: "error", want: JimengTaskStatusFailed},
		{input: "cancelled", want: JimengTaskStatusFailed},
		{input: "refunded", want: JimengTaskStatusFailed},
		{input: "timeout", want: JimengTaskStatusFailed},
		{input: "timed_out", want: JimengTaskStatusFailed},
		{input: "expired", want: JimengTaskStatusFailed},
		{input: "custom", want: "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.want, NormalizeJimengTaskStatus(tt.input))
		})
	}
}

func TestProbeJimengAPIKeyModelsStatusHandling(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantValid bool
		wantErr   bool
	}{
		{name: "2xx is valid", status: http.StatusOK, wantValid: true},
		{name: "401 is invalid", status: http.StatusUnauthorized, wantErr: true},
		{name: "403 is invalid", status: http.StatusForbidden, wantErr: true},
		{name: "404 is reachable unknown", status: http.StatusNotFound, wantValid: true},
		{name: "405 is reachable unknown", status: http.StatusMethodNotAllowed, wantValid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "Bearer jimeng-key", r.Header.Get("Authorization"))
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"data":[{"id":"video-v1"}]}`))
			}))
			defer server.Close()

			valid, err := ProbeJimengAPIKey(context.Background(), server.URL+"/v1", "jimeng-key", server.Client())

			require.Equal(t, "/v1/models", gotPath)
			require.Equal(t, tt.wantValid, valid)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
