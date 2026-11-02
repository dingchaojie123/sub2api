package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type oponeUpstreamRecorder struct {
	requests  []*http.Request
	responses []*http.Response
}

func (u *oponeUpstreamRecorder) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	resp := u.responses[0]
	u.responses = u.responses[1:]
	return resp, nil
}

func (u *oponeUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func TestPrepareOPONESeedanceFamilies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		request    string
		model      string
		durationMs int64
		resolution string
	}{
		{"B default", `{"model":"seedance-2.5-b1","prompt":"lake"}`, "seedance-2.5-b1", 30000, "720p"},
		{"S default", `{"model":"seedance-2.5-s1","prompt":"lake","resolution":"480p"}`, "seedance-2.5-s1", 5000, "480p"},
		{"S7 default", `{"model":"seedance-2.5-s7","prompt":"lake","resolution":"720p"}`, "seedance-2.5-s7", 5000, "720p"},
		{"S8 default", `{"model":"seedance-2.5-s8","prompt":"lake","resolution":"720p"}`, "seedance-2.5-s8", 5000, "720p"},
		{"S7 explicit", `{"model":"seedance-2.5-s7","prompt":"lake","duration":30,"resolution":"1080p"}`, "seedance-2.5-s7", 30000, "1080p"},
		{"S8 explicit", `{"model":"seedance-2.5-s8","prompt":"lake","duration":4,"resolution":"480p"}`, "seedance-2.5-s8", 4000, "480p"},
		{"Q explicit", `{"model":"seedance-2.0-Q","prompt":"lake","duration":15,"resolution":"4k"}`, "seedance-2.0-Q", 15000, "4k"},
		{"Qs explicit", `{"model":"seedance-2.5-Qs","prompt":"lake","duration":30,"resolution":"1080p"}`, "seedance-2.5-Qs", 30000, "1080p"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body, public, err := PreparePPVideoRequestBody(PlatformOPONE, PPVideoOperationGeneric, []byte(tt.request))
			require.NoError(t, err)
			require.Equal(t, tt.model, public.Model)
			require.Equal(t, tt.durationMs, public.DurationMilliseconds)
			require.Equal(t, tt.resolution, public.Resolution)
			require.Equal(t, tt.durationMs/1000, gjson.GetBytes(body, "duration").Int())
		})
	}
}

func TestPrepareOPONEOfficialSeedancePayload(t *testing.T) {
	t.Parallel()
	body, public, err := PreparePPVideoRequestBody(PlatformOPONE, PPVideoOperationGeneric, []byte(`{
		"model":"seedance-2.5-ss",
		"prompt":"lake",
		"duration":8,
		"resolution":"1080p",
		"aspect_ratio":"auto",
		"metadata":{"generate_audio":true}
	}`))
	require.NoError(t, err)
	require.Equal(t, "seedance-2.5-SS", public.Model)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, int64(8), gjson.GetBytes(body, "seconds").Int())
	require.Equal(t, "1080p", gjson.GetBytes(body, "metadata.resolution").String())
	require.Equal(t, "auto", gjson.GetBytes(body, "metadata.ratio").String())
	require.True(t, gjson.GetBytes(body, "metadata.generate_audio").Bool())
	require.False(t, gjson.GetBytes(body, "duration").Exists())
}

func TestOPONERejectsUnsupportedModelSpecs(t *testing.T) {
	t.Parallel()
	for _, request := range []string{
		`{"model":"gpt-image-2","prompt":"lake","duration":5}`,
		`{"model":"seedance-2.5-s6","prompt":"lake","resolution":"720p"}`,
		`{"model":"seedance-2.0fast-Q","prompt":"lake","duration":16,"resolution":"720p"}`,
		`{"model":"seedance-2.5-b2","prompt":"lake","duration":30,"resolution":"1080p"}`,
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformOPONE, PPVideoOperationGeneric, []byte(request))
		require.Error(t, err)
	}
}

func TestOPONEModelsPathsAndResponse(t *testing.T) {
	t.Parallel()
	upstreamModels := []string{
		"seedance-2.0-s4",
		"seedance-2.5-SS",
		"seedance-2.5-s6",
		"seedance-2.5-s7",
		"seedance-2.5-s8",
	}
	require.Equal(t, upstreamModels, filterUpstreamModelsForPlatform(PlatformOPONE, upstreamModels))
	require.Equal(t, upstreamModels, filterPPVideoPublicModelsForPlatform(PlatformOPONE, upstreamModels))

	createPath, err := PPVideoUpstreamPath(PlatformOPONE, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformOPONE, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	raw := []byte(`{"id":"task-1","status":"completed","video_url":"https://io.opone.cc/video.mp4?sig=keep"}`)
	parsed, err := ParsePPVideoResponse(PlatformOPONE, raw)
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)
	normalized := NormalizePPVideoPublicResponse(PlatformOPONE, raw, PPVideoPublicRequest{Model: "seedance-2.5-s5"}, parsed)
	require.Equal(t, "/v1/videos/task-1/content", gjson.GetBytes(normalized, "video_url").String())
	require.Equal(t, "/v1/videos/task-1/content", gjson.GetBytes(normalized, "data.result_url").String())
}

func TestOPONECompletedWithoutInlineURLUsesContentProxy(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"id":"task-q","status":"completed","progress":100}`)
	parsed, err := ParsePPVideoResponse(PlatformOPONE, raw)
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)

	normalized := NormalizePPVideoPublicResponse(
		PlatformOPONE,
		raw,
		PPVideoPublicRequest{Model: "seedance-2.5-Q"},
		parsed,
	)
	require.Equal(t, PPVideoTaskStatusSucceeded, gjson.GetBytes(normalized, "status").String())
	require.Equal(t, "/v1/videos/task-q/content", gjson.GetBytes(normalized, "video_url").String())
}

func TestOPONEMediaStorePendingContinuesPolling(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"id":"task-q",
		"status":"completed",
		"progress":99,
		"error":{"code":"media_store_pending","message":"video is being stored"}
	}`)
	parsed, err := ParsePPVideoResponse(PlatformOPONE, raw)
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusProcessing, parsed.Status)

	normalized := NormalizePPVideoPublicResponse(
		PlatformOPONE,
		raw,
		PPVideoPublicRequest{Model: "seedance-2.5-Q"},
		parsed,
	)
	require.Equal(t, PPVideoTaskStatusProcessing, gjson.GetBytes(normalized, "status").String())
	require.Empty(t, gjson.GetBytes(normalized, "video_url").String())
}

func TestOPONES5IncludesReferenceVideoDurationInBilling(t *testing.T) {
	t.Parallel()
	body, _, err := PreparePPVideoRequestBody(PlatformOPONE, PPVideoOperationGeneric, []byte(`{
		"model":"seedance-2.5-s5","prompt":"lake","duration":10,
		"videos":[{"url":"https://example.com/a.mp4","durationSeconds":8}],
		"references":[{"role":"reference_video","url":"https://example.com/b.mp4","durationSeconds":4}]
	}`))
	require.NoError(t, err)
	meta := PPVideoBillingMetadataFromRequest(PlatformOPONE, body)
	require.Equal(t, int64(12000), meta.InputVideoDurationMilliseconds)

	task := &PPVideoTask{
		Platform:                           PlatformOPONE,
		Model:                              "seedance-2.5-s5",
		VideoCount:                         1,
		BillingFormula:                     PPVideoBillingFormulaPerSecond,
		RequestedVideoDurationMilliseconds: 10000,
		InputVideoDurationMilliseconds:     12000,
	}
	require.Equal(t, 22.0, ppVideoTaskBillingUnits(task, task.RequestedVideoDurationMilliseconds))
}

func TestOPONEModelSyncAndContentForwarding(t *testing.T) {
	t.Parallel()
	modelSvc := &AccountTestService{cfg: &config.Config{Security: config.SecurityConfig{
		URLAllowlist: config.URLAllowlistConfig{Enabled: false},
	}}}
	account := &Account{ID: 42, Platform: PlatformOPONE, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "opone-key"}}
	req, err := modelSvc.buildPPVideoUpstreamModelsRequest(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "https://api.opone.cc/v1/models", req.URL.String())
	require.Equal(t, "Bearer opone-key", req.Header.Get("Authorization"))

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusPartialContent,
		Header:     http.Header{"Content-Type": []string{"video/mp4"}},
		Body:       io.NopCloser(bytes.NewBufferString("video")),
	}}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	resp, err := svc.ForwardPPVideoContent(context.Background(), account, "task-1", "bytes=0-4")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, "https://api.opone.cc/v1/videos/task-1/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer opone-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-4", upstream.requests[0].Header.Get("Range"))
}

func TestOPONEContentPrefersAuthenticatedEndpoint(t *testing.T) {
	t.Parallel()
	providerURL := "https://ark-video.example/final.mp4?signature=keep"
	raw := []byte(`{"status":"succeeded","provider_url":"` + providerURL + `"}`)
	require.Equal(t, providerURL, PPVideoProviderURLFromResponse(raw))

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusPartialContent,
		Header: http.Header{
			"Content-Type":  []string{"video/mp4"},
			"Content-Range": []string{"bytes 0-4/5"},
		},
		Body: io.NopCloser(bytes.NewBufferString("video")),
	}}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 42, Platform: PlatformOPONE, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "opone-key"}}

	resp, err := svc.ForwardPPVideoContent(context.Background(), account, "task-1", "bytes=0-4", providerURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.opone.cc/v1/videos/task-1/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer opone-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-4", upstream.requests[0].Header.Get("Range"))
}

func TestOPONEContentFallsBackToStoredProviderURL(t *testing.T) {
	t.Parallel()
	providerURL := "https://ark-video.example/expired.mp4?signature=old"
	upstream := &oponeUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"expired"}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"video/mp4"}},
			Body:       io.NopCloser(bytes.NewBufferString("video")),
		},
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{ID: 42, Platform: PlatformOPONE, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "opone-key"}}

	resp, err := svc.ForwardPPVideoContent(context.Background(), account, "task-1", "bytes=0-", providerURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://api.opone.cc/v1/videos/task-1/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer opone-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, providerURL, upstream.requests[1].URL.String())
	require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-", upstream.requests[1].Header.Get("Range"))
}
