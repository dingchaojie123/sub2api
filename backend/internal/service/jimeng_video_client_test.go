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
	resp         *http.Response
}

func (u *jimengHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	u.lastProxyURL = proxyURL
	if req != nil && req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.lastBody = body
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
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
	require.Equal(t, "/v1/video/generations", gotPath)
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
		require.Equal(t, "/v1/video/generations/task_456", r.URL.Path)
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
	require.JSONEq(t, `{"task_id":"task_forward","status":"processing","usage":{"input_tokens":11,"output_tokens":13}}`, rec.Body.String())
	require.Equal(t, "https://jimeng.example/v1/video/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer jimeng-upstream-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, []byte(`{"model":"video-v1","prompt":"hello"}`), upstream.lastBody)
	require.Equal(t, "task_forward", result.ResponseID)
	require.True(t, result.Usage.InputTokens > 0)
	require.Equal(t, "video-v1", result.Model)
}

func TestForwardJimengVideoGenerationSynthesizesVideoUsageForAcceptedTask(t *testing.T) {
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
	require.True(t, result.HasUsage)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, VideoBillingResolution720P, result.VideoResolution)
	require.Equal(t, 10, result.VideoDurationSeconds)
}

func TestJimengVideoRoutingModelMatchesFixedAccountModel(t *testing.T) {
	account := &Account{
		Platform: PlatformJimeng,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"seedance 2.0": "seedance 2.0"},
		},
	}

	require.True(t, account.IsModelSupported(JimengVideoRoutingModel))
	require.Equal(t, "video-v1", JimengVideoBillingModel)
	require.Equal(t, JimengVideoBillingModel, JimengVideoDefaultModel)
}

func TestNormalizeJimengTaskStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "pending", want: JimengTaskStatusProcessing},
		{input: "processing", want: JimengTaskStatusProcessing},
		{input: "success", want: JimengTaskStatusSucceeded},
		{input: "completed", want: JimengTaskStatusSucceeded},
		{input: "done", want: JimengTaskStatusSucceeded},
		{input: "failed", want: JimengTaskStatusFailed},
		{input: "error", want: JimengTaskStatusFailed},
		{input: "cancelled", want: JimengTaskStatusFailed},
		{input: "refunded", want: JimengTaskStatusFailed},
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
