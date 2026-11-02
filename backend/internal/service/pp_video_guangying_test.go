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

type guangyingRoundTripFunc func(*http.Request) (*http.Response, error)

func (f guangyingRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPrepareGuangyingRequestBody(t *testing.T) {
	t.Parallel()
	publicModel := "Seedance 2.5-全参720P"
	account := &Account{
		Platform: PlatformGuangying,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{publicModel: publicModel},
		},
	}

	body, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, []byte(`{
		"model":"Seedance 2.5-全参720P",
		"prompt":"@image1 lake at dawn",
		"duration":5,
		"aspect_ratio":"9:16",
		"resolution":"720p",
		"image_url":"https://cdn.example.com/reference.jpg",
		"native_face":true,
		"seed":123,
		"metadata":{"trace":"customer-1"}
	}`))

	require.NoError(t, err)
	require.Equal(t, publicModel, public.Model)
	require.Equal(t, "gy-model-31", public.UpstreamModel)
	require.JSONEq(t, `{
		"model":"gy-model-31",
		"prompt":"@image1 lake at dawn",
		"duration":5,
		"ratio":"9:16",
		"resolution":"720p",
		"nativeFace":true,
		"assets":[{"type":"image","name":"image1","url":"https://cdn.example.com/reference.jpg"}],
		"metadata":{"trace":"customer-1"}
	}`, string(body))
	require.False(t, gjson.GetBytes(body, "seed").Exists())
}

func TestGuangyingPublicModelWorksWithLegacyUpstreamIDMapping(t *testing.T) {
	t.Parallel()
	account := &Account{
		Platform: PlatformGuangying,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gy-model-21": "gy-model-21"},
		},
	}

	require.True(t, account.IsModelSupported("Seedance 2.0-无参考视频 按秒计费"))
	body, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, []byte(`{
		"model":"Seedance 2.0-无参考视频 按秒计费",
		"prompt":"lake at dawn","duration":5,"resolution":"720p"
	}`))

	require.NoError(t, err)
	require.Equal(t, "Seedance 2.0-无参考视频 按秒计费", public.Model)
	require.Equal(t, "gy-model-21", public.UpstreamModel)
	require.Equal(t, "gy-model-21", gjson.GetBytes(body, "model").String())
}

func TestGuangyingModelAliasesMapBothDirections(t *testing.T) {
	t.Parallel()
	expected := map[string]string{
		"Seedance 2.5-全参720P":       "gy-model-31",
		"特惠-Seedance 2.5-超分掺水版":     "gy-model-39",
		"特价-Seedance 2.5-图文":        "gy-model-35",
		"Seedance 2.5-全参1080超分":     "gy-model-40",
		"限时限量-Seedance 2.5 满参":      "gy-model-50",
		"竖屏-Seedance 2.0按次计费":       "gy-model-13",
		"竖屏-Seedance 2.0 Mini 按次计费": "gy-model-19",
		"竖屏-Seedance 2.0 Fast 按次计费": "gy-model-20",
		"Seedance 2.0-无参考视频 按秒计费":   "gy-model-21",
		"Seedance 2.5-无参考视频 按秒计费":   "gy-model-22",
		"Seedance 2.0按次计费":          "gy-model-30",
	}

	for publicModel, upstreamModel := range expected {
		actualUpstream, ok := ppVideoGuangyingUpstreamModel(publicModel)
		require.True(t, ok)
		require.Equal(t, upstreamModel, actualUpstream)
		actualPublic, ok := ppVideoGuangyingPublicModel(upstreamModel)
		require.True(t, ok)
		require.Equal(t, publicModel, actualPublic)
	}
}

func TestGuangyingLegacyPublicNamesRemainCompatible(t *testing.T) {
	t.Parallel()
	legacyAliases := map[string]string{
		"Seedance 2.5-全参720P 按秒计费":   "gy-model-31",
		"特惠-Seedance 2.5-超分掺水版 按次计费": "gy-model-39",
		"特价-Seedance 2.5-图文 按次计费":    "gy-model-35",
		"Seedance 2.5-全参1080超分 按秒计费": "gy-model-40",
	}
	canonicalNames := map[string]string{
		"gy-model-31": "Seedance 2.5-全参720P",
		"gy-model-35": "特价-Seedance 2.5-图文",
		"gy-model-39": "特惠-Seedance 2.5-超分掺水版",
		"gy-model-40": "Seedance 2.5-全参1080超分",
	}

	for legacyName, upstreamModel := range legacyAliases {
		actualUpstream, ok := ppVideoGuangyingUpstreamModel(legacyName)
		require.True(t, ok)
		require.Equal(t, upstreamModel, actualUpstream)
		actualPublic, ok := ppVideoGuangyingPublicModel(legacyName)
		require.True(t, ok)
		require.Equal(t, canonicalNames[upstreamModel], actualPublic)
		require.Contains(t, ppVideoGuangyingModelAliases(legacyName), legacyName)
	}
}

func TestPrepareGuangyingRejectsRetiredModels(t *testing.T) {
	t.Parallel()
	for _, model := range []string{"gy-model-24", "gy-model-25", "gy-model-27"} {
		_, _, err := PreparePPVideoRequestBody(PlatformGuangying, PPVideoOperationGeneric, []byte(`{
			"model":"`+model+`","prompt":"lake at dawn","duration":5,"resolution":"720p"
		}`))
		require.ErrorContains(t, err, "is not supported")
	}
}

func TestPrepareGuangyingRejectsCallerSuppliedUpstreamFileID(t *testing.T) {
	t.Parallel()
	_, _, err := PreparePPVideoRequestBody(PlatformGuangying, PPVideoOperationGeneric, []byte(`{
		"model":"gy-model-31","prompt":"@image1 lake at dawn","duration":5,
		"assets":[{"type":"image","name":"image1","fileId":"another-users-file"}]
	}`))
	require.ErrorContains(t, err, "fileId is not accepted")
}

func TestGuangyingPathsResponseAndModelFiltering(t *testing.T) {
	t.Parallel()
	createPath, err := PPVideoUpstreamPath(PlatformGuangying, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/video/generations", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformGuangying, PPVideoOperationGeneric, "job/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/tasks/job%2Fone", statusPath)

	raw := []byte(`{"data":{"id":"job-1","status":"succeeded","downloadUrl":"https://cdn.example.com/result.mp4","results":[{"type":"video","url":"https://cdn.example.com/result.mp4"}]}}`)
	parsed, err := ParsePPVideoResponse(PlatformGuangying, raw)
	require.NoError(t, err)
	require.Equal(t, "job-1", parsed.TaskID)
	require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)
	normalized := NormalizePPVideoPublicResponse(PlatformGuangying, raw, PPVideoPublicRequest{Model: "Seedance 2.5-全参720P"}, parsed)
	require.Equal(t, "https://cdn.example.com/result.mp4", gjson.GetBytes(normalized, "video_url").String())
	require.Equal(t, "Seedance 2.5-全参720P", gjson.GetBytes(normalized, "model").String())
	require.Equal(t, PPVideoTaskStatusRequiresAttention, NormalizePPVideoTaskStatus("requires_attention"))

	require.ElementsMatch(t,
		[]string{"Seedance 2.0-无参考视频 按秒计费", "Seedance 2.5-全参720P", "限时限量-Seedance 2.5 满参"},
		filterUpstreamModelsForPlatform(PlatformGuangying, []string{"seedance-2.0", "gy-model-31", "gy-model-21", "gy-model-50", "gy-model-27", "gy-model-18", "gy-model-999"}),
	)
	require.Equal(t,
		[]string{"Seedance 2.5-全参720P", "限时限量-Seedance 2.5 满参"},
		FilterPPVideoPublicModelsForPlatform(PlatformGuangying, []string{"gy-model-31", "Seedance 2.5-全参720P", "gy-model-50", "gy-model-27", "gy-model-18"}),
	)
}

func TestForwardGuangyingUsesIsolatedProtocolAndIdempotency(t *testing.T) {
	t.Parallel()
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"id":"job-1","status":"queued"},"requestId":"req-1"}`)),
	}}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          42,
		Platform:    PlatformGuangying,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gy-key",
			"base_url": GuangyingDefaultBaseURL,
		},
	}
	body := []byte(`{"model":"Seedance 2.5-无参考视频 按秒计费","prompt":"lake at dawn","duration":5,"ratio":"16:9","resolution":"720p"}`)
	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	public.UpstreamIdempotencyKey = "video-local-1"

	result, err := svc.ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", prepared, public)
	require.NoError(t, err)
	require.Equal(t, "job-1", result.ResponseID)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.guangying.store/v1/video/generations", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer gy-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "video-local-1", upstream.requests[0].Header.Get("Idempotency-Key"))
	require.Equal(t, "gy-model-22", gjson.GetBytes(upstream.bodies[0], "model").String())
}

func TestForwardGuangyingUploadsReferenceURLsBeforeCreatingTask(t *testing.T) {
	originalDownloadClient := guangyingMediaDownloadClient
	guangyingMediaDownloadClient = &http.Client{Transport: guangyingRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "https://cdn.example.com/reference.jpg", req.URL.String())
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:          io.NopCloser(bytes.NewBufferString("jpeg-content")),
			ContentLength: int64(len("jpeg-content")),
		}, nil
	})}
	t.Cleanup(func() { guangyingMediaDownloadClient = originalDownloadClient })

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(`{"data":{"id":"file-1"}}`))},
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(`{"data":{"id":"job-1","status":"queued"}}`))},
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID: 42, Platform: PlatformGuangying, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "gy-key", "base_url": GuangyingDefaultBaseURL},
	}
	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, []byte(`{
		"model":"gy-model-31","prompt":"@image1 lake at dawn","duration":5,
		"ratio":"16:9","resolution":"720p","image_url":"https://cdn.example.com/reference.jpg"
	}`))
	require.NoError(t, err)
	public.UpstreamIdempotencyKey = "video-local-upload"

	_, err = svc.ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", prepared, public)
	require.NoError(t, err)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "/v1/media/uploads", upstream.requests[0].URL.Path)
	require.Contains(t, upstream.requests[0].Header.Get("Content-Type"), "multipart/form-data")
	require.Contains(t, string(upstream.bodies[0]), "jpeg-content")
	require.Equal(t, "/v1/video/generations", upstream.requests[1].URL.Path)
	require.Equal(t, "file-1", gjson.GetBytes(upstream.bodies[1], "assets.0.fileId").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "assets.0.url").Exists())
}

func TestGuangyingModelSyncDefaultsToDocumentedBaseURL(t *testing.T) {
	t.Parallel()
	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	req, err := svc.buildPPVideoUpstreamModelsRequest(context.Background(), &Account{
		Platform:    PlatformGuangying,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "gy-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.guangying.store/v1/models", req.URL.String())
	require.Equal(t, "Bearer gy-key", req.Header.Get("Authorization"))
}
