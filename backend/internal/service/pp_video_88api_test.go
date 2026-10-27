package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func Test88APIDownstreamEnvelopeMatchesFlatRequest(t *testing.T) {
	imageURL := "https://example.com/reference.png?credential=test%2Fscope&signature=keep&expires=3600"
	nested := map[string]any{
		"model":      "grok-imagine-video",
		"input":      map[string]any{"prompt": "ocean", "img_url": imageURL},
		"parameters": map[string]any{"duration": 5, "resolution": "720p", "aspect_ratio": "9:16"},
	}
	flat := map[string]any{
		"model": "grok-imagine-video", "prompt": "ocean", "images": []string{imageURL},
		"duration": 5, "ratio": "9:16", "resolution": "720p",
	}
	nestedBody, err := json.Marshal(nested)
	require.NoError(t, err)
	flatBody, err := json.Marshal(flat)
	require.NoError(t, err)
	prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, nestedBody)
	require.NoError(t, err)
	flatPrepared, flatPublic, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, flatBody)
	require.NoError(t, err)
	require.Equal(t, flatPrepared, prepared)
	require.Equal(t, flatPublic, public)
	require.Equal(t, imageURL, gjson.GetBytes(prepared, "images.0").String())
	require.False(t, gjson.GetBytes(prepared, "input").Exists())
	require.False(t, gjson.GetBytes(prepared, "parameters").Exists())
	meta := PPVideoBillingMetadataFromRequest(Platform88APIVideo, prepared)
	require.NoError(t, meta.ValidationError)
	require.Equal(t, 5, meta.RequestedDurationSeconds)
	require.Equal(t, "720p", meta.VideoResolution)
	account := &Account{ID: 1, Platform: Platform88APIVideo, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"grok-imagine-video": "grok-imagine-video"}}}
	accountPrepared, _, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, prepared)
	require.NoError(t, err)
	require.Equal(t, prepared, accountPrepared)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"task_nested","status":"queued"}`))}}}
	result, err := (&OpenAIGatewayService{httpUpstream: upstream}).ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", nestedBody)
	require.NoError(t, err)
	require.Equal(t, "720p", result.VideoResolution)
	require.Equal(t, 5, result.VideoDurationSeconds)
	forwarded, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.JSONEq(t, string(prepared), string(forwarded))
}

func Test88APITopLevelAspectRatioDoesNotConflictWithDefaultSize(t *testing.T) {
	body := []byte(`{"model":"gemini-omni-flash","prompt":"海边日落","duration":5,"resolution":"720p","aspect_ratio":"9:16"}`)
	prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	require.Equal(t, "720p", public.Resolution)
	require.Equal(t, "9:16", gjson.GetBytes(prepared, "ratio").String())
	require.False(t, gjson.GetBytes(prepared, "aspect_ratio").Exists())
	require.False(t, gjson.GetBytes(prepared, "size").Exists())

	upstream := &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"task_portrait","status":"queued"}`))}}}
	account := &Account{ID: 1, Platform: Platform88APIVideo, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"gemini-omni-flash": "gemini-omni-flash"}}}
	_, err = (&OpenAIGatewayService{httpUpstream: upstream}).ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", body)
	require.NoError(t, err)
	forwarded, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.JSONEq(t, string(prepared), string(forwarded))
}

func Test88APIAspectAliasesCollapseOrRejectConflicts(t *testing.T) {
	for _, body := range []string{
		`{"model":"gemini-omni-flash","prompt":"ocean","duration":5,"resolution":"720p","ratio":"9:16"}`,
		`{"model":"gemini-omni-flash","prompt":"ocean","duration":5,"resolution":"720p","size":"720x1280","aspect_ratio":"9:16"}`,
	} {
		prepared, _, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, []byte(body))
		require.NoError(t, err)
		require.Equal(t, "9:16", gjson.GetBytes(prepared, "ratio").String())
		require.False(t, gjson.GetBytes(prepared, "size").Exists())
		require.False(t, gjson.GetBytes(prepared, "aspect_ratio").Exists())
	}

	_, _, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, []byte(`{"model":"gemini-omni-flash","prompt":"ocean","duration":5,"resolution":"720p","ratio":"16:9","aspect_ratio":"9:16"}`))
	require.ErrorContains(t, err, "aspect ratio fields conflict")
}

func Test88APIDownstreamEnvelopeValidation(t *testing.T) {
	for _, tt := range []struct{ name, extra, wantError string }{
		{"text only", `"input":{"prompt":"ocean"},"parameters":{"duration":"5","resolution":"720p","aspect_ratio":"9:16"}`, ""},
		{"mixed nonoverlapping", `"prompt":"ocean","parameters":{"duration":5,"resolution":"720p"}`, ""},
		{"duplicate prompt", `"prompt":"other","input":{"prompt":"ocean"}`, "use either input.prompt"},
		{"duplicate duration", `"duration":8,"parameters":{"duration":5}`, "use either parameters.duration"},
		{"seconds collision", `"seconds":"8","parameters":{"duration":5}`, "use either duration or seconds"},
		{"duplicate resolution", `"resolution":"1080p","parameters":{"resolution":"720p"}`, "use either parameters.resolution"},
		{"metadata tier collision", `"prompt":"ocean","metadata":{"resolution":"480p"},"parameters":{"duration":5,"resolution":"720p"}`, "conflicts"},
		{"duplicate size", `"size":"16:9","parameters":{"aspect_ratio":"9:16"}`, "use either parameters.aspect_ratio"},
		{"duplicate image", `"prompt":"ocean","duration":5,"resolution":"720p","images":["https://example.com/a.png"],"input":{"img_url":"https://example.com/b.png"}`, "use only one image field"},
		{"invalid input", `"input":[]`, "input must be an object"},
		{"invalid parameters", `"parameters":null`, "parameters must be an object"},
		{"invalid duration", `"prompt":"ocean","parameters":{"duration":5.5,"resolution":"720p"}`, "invalid duration"},
		{"invalid aspect", `"input":{"prompt":"ocean"},"parameters":{"duration":5,"resolution":"720p","aspect_ratio":"21:9"}`, "unsupported size"},
		{"unsafe image", `"input":{"prompt":"ocean","img_url":"https://127.0.0.1/a.png"},"parameters":{"duration":5,"resolution":"720p"}`, "public HTTPS URL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, []byte(`{"model":"grok-imagine-video",`+tt.extra+`}`))
			if tt.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantError)
			}
		})
	}
}

func Test88APINormalizesFamiliesAndNewModels(t *testing.T) {
	for _, tt := range []struct {
		model, resolution string
		duration          int
	}{
		{"SD2.0 480P", "480p", 15}, {"SD2.5 1080P", "1080p", 30},
		{"Seedance-2.5-720p官方版", "720p", 30}, {"seedance-2.0-mini-480p", "480p", 8},
		{"Seedance-2.0-720p官方版", "720p", 5}, {"Seedance-2.0-fast-720p官方版", "720p", 5},
		{"kling-3.0-turbo-2k", "2k", 8}, {"kling-3.0-turbo-4k", "4k", 8},
		{"wan3.0-video-720p", "720p", 30},
		{"grok-imagine-video-1.5", "720p", 8}, {"veo-3.1", "1080p", 8},
		{"gemini-omni-flash", "720p", 6}, {"gemini-omni-flash-1.1", "1080p", 6}, {"future-new-model", "2k", 8},
	} {
		t.Run(tt.model, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"prompt":"ocean","seconds":%q,"resolution":%q,"size":"16:9"}`, tt.model, fmt.Sprint(tt.duration), tt.resolution))
			prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, body)
			require.NoError(t, err)
			require.Equal(t, tt.model, public.Model)
			require.Equal(t, tt.resolution, public.Resolution)
			require.False(t, gjson.GetBytes(prepared, "seconds").Exists())
			meta := PPVideoBillingMetadataFromRequest(Platform88APIVideo, prepared)
			require.NoError(t, meta.ValidationError)
			require.Equal(t, tt.resolution, meta.VideoResolution)
			require.Equal(t, tt.duration, meta.RequestedDurationSeconds)
		})
	}
}

func Test88APICurrentModelMatrix(t *testing.T) {
	for _, tt := range []struct {
		model, request, resolution, ratio string
		duration                          int
	}{
		{"SD2.0", `{"resolution":"4k","duration":15}`, "4k", "1:1", 15},
		{"SD2.5", `{"resolution":"1080p","ratio":"21:9","duration":30,"generate_audio":true}`, "1080p", "21:9", 30},
		{"gemini-omni-flash", `{"resolution":"720p","ratio":"9:16","duration":10}`, "720p", "9:16", 10},
		{"gemini-omni-flash-1.1", `{"resolution":"4k","ratio":"16:9","duration":10}`, "4k", "16:9", 10},
		{"grok-imagine-video", `{"resolution":"480p","ratio":"3:2","duration":15}`, "480p", "3:2", 15},
		{"grok-imagine-video-1.5", `{"resolution":"1080p","ratio":"2:3","duration":15}`, "1080p", "2:3", 15},
		{"kling-3.0-turbo", `{"resolution":"4k","ratio":"1:1","duration":15,"generate_audio":true}`, "4k", "1:1", 15},
		{"seedance-2.0-fast官方版", `{"resolution":"720p","ratio":"16:9","duration":15}`, "720p", "16:9", 15},
		{"seedance-2.0-mini官方版", `{"resolution":"480p","ratio":"3:4","duration":15}`, "480p", "3:4", 15},
		{"seedance-2.0官方版", `{"resolution":"720p","ratio":"4:3","duration":15}`, "720p", "4:3", 15},
		{"seedance-2.5官方版", `{"resolution":"720p","ratio":"21:9","duration":30}`, "720p", "21:9", 30},
		{"veo-3.1", `{"resolution":"4k","ratio":"16:9","duration":8,"generate_audio":true}`, "4k", "16:9", 8},
		{"veo-3.1-fast", `{"resolution":"1080p","ratio":"9:16","duration":6}`, "1080p", "9:16", 6},
		{"wan3.0-video", `{"resolution":"1080p","ratio":"adaptive","duration":2}`, "1080p", "adaptive", 2},
		{"wan3.0-video-prime", `{"resolution":"480p","ratio":"21:9","duration":30}`, "480p", "21:9", 30},
	} {
		t.Run(tt.model, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"prompt":"ocean",%s}`, tt.model, strings.TrimPrefix(strings.TrimSuffix(tt.request, "}"), "{")))
			prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, body)
			require.NoError(t, err)
			require.Equal(t, tt.resolution, public.Resolution)
			require.Equal(t, int64(tt.duration*1000), public.DurationMilliseconds)
			require.Equal(t, tt.resolution, gjson.GetBytes(prepared, "resolution").String())
			require.Equal(t, tt.ratio, gjson.GetBytes(prepared, "ratio").String())
			require.False(t, gjson.GetBytes(prepared, "aspect_ratio").Exists())
			require.False(t, gjson.GetBytes(prepared, "metadata.resolution").Exists())
		})
	}
}

func Test88APICurrentModelMatrixRejectsUnsupportedCombinations(t *testing.T) {
	for _, body := range []string{
		`{"model":"SD2.5","prompt":"x","resolution":"1080p","duration":8}`,
		`{"model":"SD2.5","prompt":"x","resolution":"4k","ratio":"16:9","duration":8}`,
		`{"model":"gemini-omni-flash","prompt":"x","resolution":"1080p","ratio":"16:9","duration":6}`,
		`{"model":"grok-imagine-video","prompt":"x","resolution":"1080p","ratio":"16:9","duration":8}`,
		`{"model":"grok-imagine-video-1.5","prompt":"x","resolution":"720p","ratio":"16:9","duration":8,"generate_audio":false}`,
		`{"model":"grok-imagine-video-1.5","prompt":"x","resolution":"720p","size":"1280x720","duration":8}`,
		`{"model":"kling-3.0-turbo","prompt":"x","resolution":"720p","ratio":"21:9","duration":8}`,
		`{"model":"seedance-2.0官方版","prompt":"x","resolution":"1080p","ratio":"16:9","duration":8}`,
		`{"model":"seedance-2.0-mini官方版","prompt":"x","resolution":"720p","ratio":"16:9","duration":8,"first_frame":"https://example.com/a.jpg"}`,
		`{"model":"seedance-2.5官方版","prompt":"x","resolution":"720p","ratio":"16:9","duration":8,"generate_audio":true}`,
		`{"model":"veo-3.1","prompt":"x","resolution":"720p","ratio":"16:9","duration":5}`,
		`{"model":"veo-3.1","prompt":"x","resolution":"720p","ratio":"16:9","duration":8,"images":["data:image/webp;base64,YQ=="]}`,
		`{"model":"wan3.0-video","prompt":"x","resolution":"720p","ratio":"adaptive","duration":1}`,
	} {
		_, _, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, []byte(body))
		require.Error(t, err, body)
	}
}

func Test88APIWanMediaAndVeoFrames(t *testing.T) {
	wan := []byte(`{"model":"wan3.0-video","duration":5,"media":[{"type":"reference_image","url":"https://example.com/a.jpg"}]}`)
	prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, wan)
	require.NoError(t, err)
	require.True(t, public.HasImage)
	require.Equal(t, "1080p", gjson.GetBytes(prepared, "resolution").String())
	require.Equal(t, "adaptive", gjson.GetBytes(prepared, "ratio").String())

	veo := []byte(`{"model":"veo-3.1","prompt":"x","duration":8,"resolution":"720p","images":["https://example.com/a.jpg"]}`)
	prepared, _, err = PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, veo)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/a.jpg", gjson.GetBytes(prepared, "images.0").String())
	require.Equal(t, "frames", gjson.GetBytes(prepared, "metadata.video_mode").String())
}

func Test88APIGemini11ExtensionOmitsUpstreamConflictsAndKeepsBillingDuration(t *testing.T) {
	body := []byte(`{"model":"gemini-omni-flash-1.1","task":"extend","prompt":"continue","resolution":"1080p","video":"https://example.com/input.mp4"}`)
	prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	require.Equal(t, int64(10_000), public.DurationMilliseconds)
	require.False(t, gjson.GetBytes(prepared, "duration").Exists())
	require.False(t, gjson.GetBytes(prepared, "ratio").Exists())
	require.Equal(t, "https://example.com/input.mp4", gjson.GetBytes(prepared, "video").String())
	meta := PPVideoBillingMetadataFromRequest(Platform88APIVideo, prepared)
	require.NoError(t, meta.ValidationError)
	require.Equal(t, int64(10_000), meta.RequestedDurationMilliseconds)
}

func Test88APIFixedResolutionOverridesDownstream(t *testing.T) {
	for _, tt := range []struct{ model, tier string }{
		{"SD2.0 1080P", "1080p"}, {"SD2.0 480P", "480p"}, {"SD2.0 720P", "720p"},
		{"SD2.5 1080P", "1080p"}, {"SD2.5 480P", "480p"}, {"SD2.5 720P", "720p"},
		{"Seedance-2.0-720p官方版", "720p"}, {"Seedance-2.0-fast-720p官方版", "720p"}, {"Seedance-2.5-720p官方版", "720p"},
		{"grok-imagine-video-1.5-1080p", "1080p"},
		{"kling-3.0-turbo-1080p", "1080p"}, {"kling-3.0-turbo-2k", "2k"},
		{"kling-3.0-turbo-4k", "4k"}, {"kling-3.0-turbo-720p", "720p"},
		{"seedance-2.0-mini-480p", "480p"}, {"seedance-2.0-mini-720p", "720p"},
		{"wan3.0-video-1080p", "1080p"}, {"wan3.0-video-480p", "480p"}, {"wan3.0-video-720p", "720p"},
	} {
		t.Run(tt.model, func(t *testing.T) {
			require.Equal(t, tt.tier, video88APIFixedTier(" "+strings.ToUpper(tt.model)+" "))
			canonical := []byte(fmt.Sprintf(`{"model":%q,"prompt":"ocean","duration":8,"size":"9:16"}`, tt.model))
			want, _, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, canonical)
			require.NoError(t, err)
			for _, fields := range []string{
				`"resolution":"4k","size":"9:16"`,
				`"parameters":{"resolution":"480p","aspect_ratio":"9:16"}`,
				`"metadata":{"resolution":"720p"},"size":"9:16"`,
				`"resolution":"1080p","metadata":{"resolution":"2k"},"parameters":{"resolution":"480p"},"size":"720x1280"`,
			} {
				if strings.HasPrefix(strings.ToLower(tt.model), "grok-imagine-video") && strings.Contains(fields, "720x1280") {
					continue
				}
				body := []byte(fmt.Sprintf(`{"model":%q,"prompt":"ocean","duration":8,%s}`, tt.model, fields))
				prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, body)
				require.NoError(t, err)
				require.Equal(t, want, prepared)
				require.Equal(t, HashUsageRequestPayload(want), HashUsageRequestPayload(prepared))
				require.Equal(t, tt.tier, public.Resolution)
				meta := PPVideoBillingMetadataFromRequest(Platform88APIVideo, prepared)
				require.NoError(t, meta.ValidationError)
				require.Equal(t, tt.tier, meta.VideoResolution)
			}
		})
	}
}

func Test88APIUnlistedModelsUseDownstreamResolution(t *testing.T) {
	for _, model := range []string{"future-model-720p", "SD2.6 480P"} {
		t.Run(model, func(t *testing.T) {
			require.Empty(t, video88APIFixedTier(model))
			for _, tier := range []string{"480p", "720p", "768p", "1080p", "2k", "4k"} {
				body := []byte(fmt.Sprintf(`{"model":%q,"input":{"prompt":"ocean"},"parameters":{"duration":8,"resolution":%q,"aspect_ratio":"9:16"}}`, model, tier))
				prepared, public, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, body)
				require.NoError(t, err)
				require.Equal(t, tier, public.Resolution)
				require.Equal(t, tier, gjson.GetBytes(prepared, "resolution").String())
				meta := PPVideoBillingMetadataFromRequest(Platform88APIVideo, prepared)
				require.NoError(t, meta.ValidationError)
				require.Equal(t, tier, meta.VideoResolution)
			}
			_, _, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, []byte(fmt.Sprintf(`{"model":%q,"prompt":"ocean","duration":8}`, model)))
			require.ErrorContains(t, err, "requires an explicit resolution")
		})
	}
}

func Test88APIFixedResolutionForwardAndPricing(t *testing.T) {
	model := "SD2.0 480P"
	account := &Account{ID: 1, Platform: Platform88APIVideo, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
	body := []byte(`{"model":"SD2.0 480P","input":{"prompt":"ocean"},"parameters":{"duration":5,"resolution":"720p","aspect_ratio":"9:16"}}`)
	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	require.Equal(t, "480p", public.Resolution)
	meta := PPVideoBillingMetadataFromRequest(Platform88APIVideo, prepared)
	require.NoError(t, meta.ValidationError)
	groupID := int64(904)
	p480, p720 := 0.1, 0.4
	svc := &OpenAIGatewayService{resolver: newPPVideoResolverWithChannel(t, groupID, Platform88APIVideo, []ChannelModelPricing{{
		Platform: Platform88APIVideo, Models: []string{model}, BillingMode: BillingModeVideo,
		Intervals: []PricingInterval{{TierLabel: "480p", PerRequestPrice: &p480}, {TierLabel: "720p", PerRequestPrice: &p720}},
	}})}
	quote, err := svc.calculate88APIVideoCost(context.Background(), meta, &APIKey{GroupID: &groupID})
	require.NoError(t, err)
	require.InDelta(t, 0.5, quote.Cost, 1e-9)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"task_fixed","status":"queued"}`))}}}
	svc.httpUpstream = upstream
	result, err := svc.ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", body)
	require.NoError(t, err)
	require.Equal(t, "480p", result.VideoResolution)
	require.Equal(t, "480p", gjson.GetBytes(result.ResponseBody, "usage.resolution").String())
	forwarded, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.JSONEq(t, string(prepared), string(forwarded))
	require.False(t, gjson.GetBytes(forwarded, "metadata.resolution").Exists())
}

func Test88APIRejectsInvalidRequests(t *testing.T) {
	for _, body := range []string{
		`{"prompt":"x","duration":8,"resolution":"720p"}`,
		`{"model":"SD2.0 720P","prompt":"x","duration":30}`,
		`{"model":"SD2.5 720P","prompt":"x","duration":8,"seconds":"8"}`,
		`{"model":"SD2.5 720P","prompt":"x","duration":8.5}`,
		`{"model":"SD2.5 720P","prompt":"x","duration":8,"callback_url":"https://example.com/cb"}`,
		`{"model":"SD2.5 720P","prompt":"x","duration":8,"n":2}`,
		`{"model":"SD2.5 720P","prompt":"x","duration":8,"images":["https://127.0.0.1/a"]}`,
		`{"model":"SD2.5 720P","prompt":"x","duration":8,"metadata":{"lastFrame":"https://example.com/a.jpg"}}`,
		`{"model":"kling-3.0-turbo-720p","prompt":"x","duration":8,"metadata":{"referenceAudios":["https://example.com/a.mp3"]}}`,
		`{"model":"grok-imagine-video","prompt":"x","duration":8,"resolution":"720p","images":["https://example.com/a","https://example.com/b"]}`,
		`{"model":"veo-3.1","prompt":"x","duration":5,"resolution":"720p"}`,
		`{"model":"gemini-omni-flash","prompt":"x","duration":6,"metadata":{"referenceVideos":["https://example.com/a.mp4"]}}`,
		`{"model":"SD2.0 720P","prompt":"x","duration":8,"generate_audio":false}`,
		`{"model":"kling-3.0-turbo-720p","prompt":"x","duration":8,"size":"21:9"}`,
	} {
		_, _, err := PreparePPVideoRequestBody(Platform88APIVideo, PPVideoOperationGeneric, []byte(body))
		require.Error(t, err, body)
	}
}

func Test88APIStateIsAuthoritative(t *testing.T) {
	for _, tt := range []struct{ status, media, want string }{
		{"queued", "https://example.com/preview.mp4", PPVideoTaskStatusProcessing},
		{"unknown", "https://example.com/preview.mp4", PPVideoTaskStatusProcessing},
		{"completed", "", PPVideoTaskStatusProcessing},
		{"completed", "https://example.com/video.mp4?token=keep", PPVideoTaskStatusSucceeded},
		{"failed", "https://example.com/preview.mp4", PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(Platform88APIVideo, []byte(fmt.Sprintf(`{"id":"task_1","status":%q,"url":%q,"progress":100,"error":{"code":"rejected","message":"reason"}}`, tt.status, tt.media)))
		require.NoError(t, err)
		require.Equal(t, tt.want, parsed.Status)
		require.Equal(t, "reason", parsed.ErrorMessage)
	}
}

func Test88APIForwardAndOnlineSync(t *testing.T) {
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"SD2.5 720P"},{"id":"future-new-model"}]}`))},
		{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"task_1","status":"queued","url":"https://example.com/preview"}`))},
		{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"task_1","status":"completed","url":"https://example.com/result?signature=keep"}`))},
	}}
	account := &Account{ID: 1, Platform: Platform88APIVideo, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key", "model_mapping": map[string]any{"SD2.5 720P": "SD2.5 720P"}}}
	models, err := (&AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}).FetchUpstreamSupportedModels(context.Background(), account)
	require.NoError(t, err)
	require.Contains(t, models, "future-new-model")
	require.Equal(t, "https://88api.ai/v1/models", upstream.requests[0].URL.String())
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	created, err := svc.ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", []byte(`{"model":"SD2.5 720P","prompt":"ocean","duration":8,"ratio":"16:9"}`))
	require.NoError(t, err)
	require.Equal(t, "https://88api.ai/v1/videos", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer test-key", upstream.requests[1].Header.Get("Authorization"))
	require.False(t, gjson.GetBytes(created.ResponseBody, "url").Exists())
	require.Equal(t, "720p", gjson.GetBytes(created.ResponseBody, "usage.resolution").String())
	completed, err := svc.ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "task_1", nil, PPVideoPublicRequest{Model: "SD2.5 720P", Resolution: "720p", DurationMilliseconds: 8000, VideoCount: 1})
	require.NoError(t, err)
	require.Equal(t, "https://88api.ai/v1/videos/task_1", upstream.requests[2].URL.String())
	require.Equal(t, http.MethodGet, upstream.requests[2].Method)
	require.Equal(t, "https://example.com/result?signature=keep", gjson.GetBytes(completed.ResponseBody, "url").String())
	require.True(t, isPPVideoAccountEligibleForModel(account, "SD2.5 720P"))
	require.False(t, isPPVideoAccountEligibleForModel(account, "not-selected"))
	_, err = PPVideoUpstreamPath(Platform88APIVideo, PPVideoOperationCancel, "task_1")
	require.Error(t, err)
}

func Test88APIRejectsAmbiguousPricing(t *testing.T) {
	groupID := int64(901)
	price := 0.5
	for _, tt := range []struct {
		name, platform, model, tier string
		models                      []string
		intervals                   []PricingInterval
		wantError                   bool
	}{
		{"fixed 4k price", Platform88APIVideo, "kling-3.0-turbo-4k", "4k", []string{"kling-3.0-turbo-4k"}, nil, false},
		{"fixed 768p tier", Platform88APIVideo, "minimax-h3-768p", "768p", []string{"minimax-h3-768p"}, []PricingInterval{{TierLabel: "768P", PerRequestPrice: &price}}, false},
		{"wildcard rejected", Platform88APIVideo, "grok-imagine-video", "720p", []string{"*"}, []PricingInterval{{TierLabel: "720p", PerRequestPrice: &price}}, true},
		{"other platform rejected", PlatformSeedance, "grok-imagine-video", "720p", []string{"grok-imagine-video"}, []PricingInterval{{TierLabel: "720p", PerRequestPrice: &price}}, true},
		{"variable resolution needs tier", Platform88APIVideo, "grok-imagine-video", "720p", []string{"grok-imagine-video"}, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{resolver: newPPVideoResolverWithChannel(t, groupID, tt.platform, []ChannelModelPricing{{Platform: tt.platform, Models: tt.models, BillingMode: BillingModeVideo, PerRequestPrice: &price, Intervals: tt.intervals}})}
			quote, err := svc.calculate88APIVideoCost(context.Background(), PPVideoBillingMetadata{Platform: Platform88APIVideo, Model: tt.model, VideoResolution: tt.tier, RequestedDurationMilliseconds: 8000}, &APIKey{GroupID: &groupID})
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.InDelta(t, 4, quote.Cost, 1e-9)
			}
		})
	}
}

func Test88APISeedancePricingMatchesChannelCaseRules(t *testing.T) {
	groupID := int64(902)
	price := 0.15
	for _, model := range []string{"Seedance-2.0-720p官方版", "Seedance-2.0-fast-720p官方版"} {
		for _, configured := range []string{model, strings.ToLower(model)} {
			t.Run(configured, func(t *testing.T) {
				svc := &OpenAIGatewayService{resolver: newPPVideoResolverWithChannel(t, groupID, Platform88APIVideo, []ChannelModelPricing{{
					Platform: Platform88APIVideo, Models: []string{configured}, BillingMode: BillingModeVideo, PerRequestPrice: &price,
				}})}
				meta := PPVideoBillingMetadata{Platform: Platform88APIVideo, Model: model, VideoResolution: "720p", RequestedDurationMilliseconds: 5000, VideoCount: 1}
				cost, err := svc.CalculatePPVideoCost(context.Background(), &APIKey{GroupID: &groupID, Group: &Group{Platform: Platform88APIVideo, RateMultiplier: 1}}, nil, &Account{}, meta)
				require.NoError(t, err)
				require.InDelta(t, 0.75, cost.ActualCost, 1e-9)
			})
		}
	}
}

func Test88APIPricingReportsSpecificConfigurationFailure(t *testing.T) {
	groupID := int64(903)
	price := 0.15
	model := "Seedance-2.0-720p官方版"
	for _, tt := range []struct {
		name      string
		models    []string
		mode      BillingMode
		price     *float64
		intervals []PricingInterval
		want      string
	}{
		{"different model", []string{"Seedance-2.0-fast-720p官方版"}, BillingModeVideo, &price, nil, "no channel model price matched"},
		{"wrong mode", []string{model}, BillingModePerRequest, &price, nil, "billing mode must be video"},
		{"wildcard", []string{"Seedance-*"}, BillingModeVideo, &price, nil, "wildcard prices are not supported"},
		{"missing default", []string{model}, BillingModeVideo, nil, nil, "no default per-second price"},
		{"wrong tier", []string{model}, BillingModeVideo, &price, []PricingInterval{{TierLabel: "1080p", PerRequestPrice: &price}}, "no matching resolution tier"},
		{"empty tier price", []string{model}, BillingModeVideo, &price, []PricingInterval{{TierLabel: "720p"}}, "empty or invalid per-second price"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{resolver: newPPVideoResolverWithChannel(t, groupID, Platform88APIVideo, []ChannelModelPricing{{Platform: Platform88APIVideo, Models: tt.models, BillingMode: tt.mode, PerRequestPrice: tt.price, Intervals: tt.intervals}})}
			_, err := svc.calculate88APIVideoCost(context.Background(), PPVideoBillingMetadata{Platform: Platform88APIVideo, Model: model, VideoResolution: "720p", RequestedDurationMilliseconds: 5000}, &APIKey{GroupID: &groupID})
			require.ErrorContains(t, err, tt.want)
			require.ErrorContains(t, err, "group_id=903")
		})
	}
}

func Test88APIPublicResponsePreservesTierAndPreferredURL(t *testing.T) {
	raw := []byte(`{"id":"task_1","status":"completed","url":"https://example.com/preferred?sig=keep","video_url":"https://example.com/other"}`)
	parsed, err := ParsePPVideoResponse(Platform88APIVideo, raw)
	require.NoError(t, err)
	body := NormalizePPVideoPublicResponse(Platform88APIVideo, raw, PPVideoPublicRequest{Model: "kling-3.0-turbo-4k", Resolution: "4k", DurationMilliseconds: 8000, VideoCount: 1}, parsed)
	require.Equal(t, "4k", gjson.GetBytes(body, "usage.resolution").String())
	require.Equal(t, "https://example.com/preferred?sig=keep", gjson.GetBytes(body, "video_url").String())
}

func Test88APILongRunningTaskDoesNotRefundOnLocalDeadline(t *testing.T) {
	now := time.Now()
	task := newPPVideoPollerTask("local-88api", "task_88api", PPVideoTaskStatusProcessing, now.Add(-3*time.Hour))
	task.Platform = Platform88APIVideo
	repo := &ppVideoPollerRepoStub{task: task}
	gateway := &ppVideoPollerGatewayStub{result: &OpenAIForwardResult{TaskStatus: PPVideoTaskStatusProcessing, ResponseStatusCode: 200}}
	poller := newPPVideoPollerTestService(repo, gateway, now)
	poller.Accounts = &jimengVideoPollerAccountStub{account: &Account{ID: 3, Platform: Platform88APIVideo, Type: AccountTypeAPIKey}}
	result, err := poller.ProcessTask(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, PPVideoPollerOutcomeProcessing, result.Outcome)
	require.Empty(t, gateway.releases)
	require.Empty(t, gateway.settlements)
}

func Test88APIPricingUsesExactModelAndTier(t *testing.T) {
	groupID := int64(900)
	p480, p720 := 0.1, 0.4
	svc := &OpenAIGatewayService{resolver: newPPVideoResolverWithChannel(t, groupID, Platform88APIVideo, []ChannelModelPricing{{
		Platform: Platform88APIVideo, Models: []string{"grok-imagine-video"}, BillingMode: BillingModeVideo,
		Intervals: []PricingInterval{{TierLabel: "480p", PerRequestPrice: &p480}, {TierLabel: "720p", PerRequestPrice: &p720}},
	}})}
	key := &APIKey{GroupID: &groupID, Group: &Group{Platform: Platform88APIVideo, VideoPrice720P: &p480, RateMultiplier: 1}}
	for _, tt := range []struct {
		tier  string
		price float64
	}{{"480p", p480}, {"720p", p720}} {
		meta := PPVideoBillingMetadata{Platform: Platform88APIVideo, Model: "grok-imagine-video", VideoResolution: tt.tier, RequestedDurationMilliseconds: 8000, VideoCount: 1}
		cost, err := svc.CalculatePPVideoCost(context.Background(), key, nil, &Account{}, meta)
		require.NoError(t, err)
		require.InDelta(t, 8*tt.price, cost.ActualCost, 1e-9)
	}
	for _, meta := range []PPVideoBillingMetadata{
		{Platform: Platform88APIVideo, Model: "grok-imagine-video", VideoResolution: "1080p", RequestedDurationMilliseconds: 8000, VideoCount: 1},
		{Platform: Platform88APIVideo, Model: "missing", VideoResolution: "720p", RequestedDurationMilliseconds: 8000, VideoCount: 1},
	} {
		_, err := svc.CalculatePPVideoCost(context.Background(), key, nil, &Account{}, meta)
		require.Error(t, err)
	}
	task := &PPVideoTask{Platform: Platform88APIVideo, BillingFormula: PPVideoBillingFormulaPerSecond, BillingUnitPrice: p720, RequestedVideoDurationMilliseconds: 8000, VideoCount: 1, EstimatedTotalCost: 3.2, HoldAmount: 6.4}
	require.InDelta(t, 6.4, ppVideoActualSettlementCost(task), 1e-9)
}
