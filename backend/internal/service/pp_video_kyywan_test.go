package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func kyyWanPrepare(t *testing.T, body string) ([]byte, PPVideoPublicRequest, error) {
	t.Helper()
	return PreparePPVideoRequestBody(PlatformKyyWan, PPVideoOperationGeneric, []byte(body))
}

func TestKyyWanModelTable(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{KyyWanModelC3}, kyyWanModelNames())
	require.Equal(t, "wan3.0-video-c3", KyyWanModelC3)
	require.Len(t, kyyWanModels, 1)

	spec, ok := kyyWanModelSpecFor("WAN3.0-VIDEO-C3")
	require.True(t, ok)
	require.Equal(t, KyyWanModelC3, spec.Name)
	// The upstream document: 2-30 seconds, three resolutions with 1080P as default, 10 / 5 / 5 references.
	require.Equal(t, 2, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"480p", "720p", "1080p"}, spec.Resolutions)
	require.Equal(t, "1080p", spec.DefaultResolution)
	require.Equal(t, []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}, spec.Ratios)
	require.Equal(t, 10, spec.MaxImages)
	require.Equal(t, 5, spec.MaxVideos)
	require.Equal(t, 5, spec.MaxAudios)
	// Clients pick video models by name: the name says wan.
	require.Contains(t, strings.ToLower(spec.Name), "wan")

	// The other Wan3.0 models of the upstream's center, and the ModelVerse Wan3.0 models, are not offered.
	for _, model := range []string{"", "wan3.0-video", "wan3.0-video-prime", "wan3.0-prime-c2", "wan3.0-video-c2", "wan3.0-video-r2v-c4", "seedance_2.0"} {
		_, ok := kyyWanModelSpecFor(model)
		require.False(t, ok, model)
		require.False(t, ppVideoIsKyyWanModel(model), model)
	}
}

func TestKyyWanPlatformRoutingAndModelList(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformKyyWan))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformKyyWan))
	require.True(t, IsAllowedQuotaPlatform(PlatformKyyWan))
	require.False(t, IsSeedanceHubMemberPlatform(PlatformKyyWan), "a Wan model is not a Seedance line")

	require.True(t, ppVideoModelBelongsToPlatform(PlatformKyyWan, KyyWanModelC3))
	require.False(t, ppVideoModelKnownForeignToPlatform(PlatformKyyWan, KyyWanModelC3))
	// Its model is foreign to the other platforms, and the ModelVerse Wan3.0 model is foreign to it.
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformWan3, KyyWanModelC3))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformXMSeedance, KyyWanModelC3))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformKyyWan, Wan30VideoDefaultModel))

	require.Equal(t, []string{KyyWanModelC3},
		filterPPVideoPublicModelsForPlatform(PlatformKyyWan, []string{"WAN3.0-VIDEO-C3", KyyWanModelC3, "wan3.0-video-c2", "other"}))
	require.Equal(t, []string{KyyWanModelC3},
		filterUpstreamModelsForPlatform(PlatformKyyWan, []string{"wan3.0-video-c2", "Wan3.0-Video-C3"}))
	require.Equal(t, KyyWanModelC3, PPVideoPublicModelName(PlatformKyyWan, KyyWanModelC3))
	// A request written in another case still matches an account whitelist saved under the canonical name.
	require.Equal(t, []string{KyyWanModelC3}, ppVideoKyyWanModelAliases("WAN3.0-VIDEO-C3"))
	require.Nil(t, ppVideoKyyWanModelAliases("wan3.0-video-c2"))
}

// Account sync answers from the local table without any HTTP request.
func TestKyyWanModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformKyyWan, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, kyyWanModelNames(), models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformKyyWan, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestKyyWanUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://zcbservice.aizfw.cn/kyyReactApiServer", KyyWanDefaultBaseURL)
	createPath, err := PPVideoUpstreamPath(PlatformKyyWan, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v2/model-center/tasks", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformKyyWan, PPVideoOperationGeneric, "mcp/one")
	require.NoError(t, err)
	require.Equal(t, "/v2/model-center/tasks/mcp%2Fone", statusPath)
	_, err = PPVideoUpstreamPath(PlatformKyyWan, PPVideoOperationCancel, "x")
	require.Error(t, err)
	_, err = PPVideoStatusUpstreamPath(PlatformKyyWan, PPVideoOperationGeneric, "")
	require.Error(t, err)

	headers, err := PPVideoRequestHeaders(PlatformKyyWan, "kyy-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer kyy-key", headers.Get("Authorization"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))

	// This upstream delivers a direct link; the content proxy with credentials is not used.
	require.False(t, ppVideoTaskUsesAuthenticatedContent(PlatformKyyWan))
	require.True(t, PPVideoTaskCanStreamContent(PlatformKyyWan))

	for _, tc := range []struct{ body, want string }{
		{`{"id":"mcp_1","object":"video","created":1774836724,"model":"wan3.0-video-c3","status":"queued","error":null}`, PPVideoTaskStatusProcessing},
		{`{"id":"mcp_1","status":"processing","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"mcp_1","status":"completed","progress":100,"result_url":"https://example.com/r.mp4","video_url":"https://example.com/r.mp4"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"mcp_1","status":"failed","error":"content rejected"}`, PPVideoTaskStatusFailed},
		// Completed without any address is not a success yet.
		{`{"id":"mcp_1","status":"completed","progress":100}`, PPVideoTaskStatusProcessing},
	} {
		parsed, err := ParsePPVideoResponse(PlatformKyyWan, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "mcp_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	// The sample response of the document also carries a task_id field: `id` wins.
	sample, err := ParsePPVideoResponse(PlatformKyyWan, []byte(`{"id":"mcp_example_123456","status":"completed","video_url":"https://example.com/result.mp4","task_id":"sample_task_id"}`))
	require.NoError(t, err)
	require.Equal(t, "mcp_example_123456", sample.TaskID)

	// A failed task carries its reason in `error`, as a plain string or as an object; a failure without one still reads as a failure.
	failed, err := ParsePPVideoResponse(PlatformKyyWan, []byte(`{"id":"mcp_1","status":"failed","error":"content rejected"}`))
	require.NoError(t, err)
	require.Equal(t, "content rejected", failed.ErrorMessage)
	failedObject, err := ParsePPVideoResponse(PlatformKyyWan, []byte(`{"id":"mcp_1","status":"failed","error":{"message":"boom"}}`))
	require.NoError(t, err)
	require.Equal(t, "boom", failedObject.ErrorMessage)
	failedBare, err := ParsePPVideoResponse(PlatformKyyWan, []byte(`{"id":"mcp_1","status":"failed"}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusFailed, failedBare.Status)
	require.NotEmpty(t, failedBare.ErrorMessage)

	// Billing uses the requested seconds, never a length or time figure the upstream reports.
	reported, err := ParsePPVideoResponse(PlatformKyyWan, []byte(`{"id":"mcp_1","status":"completed","video_url":"https://example.com/r.mp4","amount":0.32,"actualDuration":5,"duration":99,"input_duration":12}`))
	require.NoError(t, err)
	require.Zero(t, reported.GeneratedDurationMilliseconds)
	require.Zero(t, reported.InputVideoDurationMilliseconds)
}

func TestKyyWanPublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: "WAN3.0-VIDEO-C3"}
	raw := `{"id":"mcp_1","object":"video","created":1774836724,"model":"wan3.0-video-c3","status":"completed","progress":100,` +
		`"result_url":"https://example.com/result.mp4","video_url":"https://example.com/result.mp4","amount":0.32,"actualDuration":5,` +
		`"error":null,"error_message":"sample","media_url":"https://example.com/result.mp4","task_id":"sample_task_id"}`
	parsed, err := ParsePPVideoResponse(PlatformKyyWan, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformKyyWan, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, KyyWanModelC3, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "mcp_1", gjson.GetBytes(normalized, "id").String())
	require.Equal(t, "mcp_1", gjson.GetBytes(normalized, "task_id").String(), "the supplier's own task_id is replaced by ours")
	require.Equal(t, "https://example.com/result.mp4", gjson.GetBytes(normalized, "video_url").String())
	// The supplier's accounting never reaches a client.
	for _, field := range []string{"amount", "actualDuration", "media_url", "error_message"} {
		require.False(t, gjson.GetBytes(normalized, field).Exists(), field)
	}

	// A task that is still running never shows a result address, even if the upstream sends one.
	running := `{"id":"mcp_1","status":"processing","progress":30,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformKyyWan, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformKyyWan, ppVideoWithoutPublicVideoURL([]byte(running)), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")
}

func TestKyyWanNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	prepared, public, err := kyyWanPrepare(t, `{
		"model":"wan3.0-video-c3","prompt":"  a cat runs  ","duration":8,"resolution":"720p","ratio":"16:9","seed":7,
		"images":["https://example.com/1.png","https://example.com/2.png"],
		"videos":["https://example.com/1.mp4"],
		"audios":["https://example.com/1.mp3"]}`)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"model":            "wan3.0-video-c3",
		"prompt":           "a cat runs",
		"duration":         float64(8),
		"resolution":       "720P",
		"aspect_ratio":     "16:9",
		"seed":             float64(7),
		"reference_images": []any{"https://example.com/1.png", "https://example.com/2.png"},
		"reference_videos": []any{"https://example.com/1.mp4"},
		"reference_audios": []any{"https://example.com/1.mp3"},
	}, gjson.ParseBytes(prepared).Value())
	require.Equal(t, KyyWanModelC3, public.Model)
	require.Equal(t, KyyWanModelC3, public.UpstreamModel)
	require.Equal(t, int64(8000), public.DurationMilliseconds)
	require.Equal(t, 1, public.VideoCount)
	require.Equal(t, "720p", public.Resolution)
	require.True(t, public.HasImage)

	// The upstream's own spellings are accepted, and so are first / last frames (a bare URL or an object).
	prepared, public, err = kyyWanPrepare(t, `{
		"model":"WAN3.0-VIDEO-C3","prompt":"x","duration":"5","aspect_ratio":"adaptive","resolution":"480P",
		"reference_images":["https://example.com/1.png"],
		"first_image":"https://example.com/f.png","last_frame":{"url":"https://example.com/l.png"}}`)
	require.NoError(t, err)
	require.Equal(t, "wan3.0-video-c3", gjson.GetBytes(prepared, "model").String(), "the name is canonicalised")
	require.Equal(t, int64(5), gjson.GetBytes(prepared, "duration").Int())
	require.Equal(t, "480P", gjson.GetBytes(prepared, "resolution").String())
	require.Equal(t, "adaptive", gjson.GetBytes(prepared, "aspect_ratio").String())
	require.Equal(t, "https://example.com/f.png", gjson.GetBytes(prepared, "first_image").String())
	require.Equal(t, "https://example.com/l.png", gjson.GetBytes(prepared, "last_image").String())
	require.Equal(t, []any{"https://example.com/1.png"}, gjson.GetBytes(prepared, "reference_images").Value())
	require.True(t, public.HasImage)
	// The document does not say that frames cannot be mixed with references, so that is not enforced here.

	// The default resolution is sent, so the task is billed at it; no ratio / seed / media means none is sent.
	prepared, public, err = kyyWanPrepare(t, `{"model":"wan3.0-video-c3","prompt":"x","duration":2}`)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"model": "wan3.0-video-c3", "prompt": "x", "duration": float64(2), "resolution": "1080P",
	}, gjson.ParseBytes(prepared).Value())
	require.Equal(t, "1080p", public.Resolution)
	require.False(t, public.HasImage)
	// Seed 0 is a seed.
	prepared, _, err = kyyWanPrepare(t, `{"model":"wan3.0-video-c3","prompt":"x","duration":30,"seed":0}`)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(prepared, "seed").Exists())
	require.Equal(t, int64(0), gjson.GetBytes(prepared, "seed").Int())
}

func TestKyyWanPublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ resolution, want string }{{"480p", "480p"}, {"720P", "720p"}, {"1080p", "1080p"}, {"", "1080p"}} {
		body := `{"model":"wan3.0-video-c3","prompt":"x","duration":12`
		if tc.resolution != "" {
			body += `,"resolution":"` + tc.resolution + `"`
		}
		prepared, _, err := kyyWanPrepare(t, body+`}`)
		require.NoError(t, err, tc.resolution)
		meta := PPVideoBillingMetadataFromRequest(PlatformKyyWan, prepared)
		require.Equal(t, PlatformKyyWan, meta.Platform)
		require.Equal(t, KyyWanModelC3, meta.Model)
		require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
		require.Equal(t, 12, meta.RequestedDurationSeconds)
		require.Equal(t, 1, meta.VideoCount)
		require.Equal(t, tc.want, meta.VideoResolution, tc.resolution)
	}
}

func TestKyyWanRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	image := func(n int) string {
		items := make([]string, 0, n)
		for i := 0; i < n; i++ {
			items = append(items, `"https://example.com/`+string(rune('a'+i))+`.png"`)
		}
		return `[` + strings.Join(items, ",") + `]`
	}
	video := func(n int) string {
		items := make([]string, 0, n)
		for i := 0; i < n; i++ {
			items = append(items, `"https://example.com/`+string(rune('a'+i))+`.mp4"`)
		}
		return `[` + strings.Join(items, ",") + `]`
	}
	base := `"model":"wan3.0-video-c3","prompt":"x","duration":5`

	for _, tc := range []struct{ name, body, want string }{
		{"no model", `{"prompt":"x","duration":5}`, "model is required"},
		{"other model", `{"model":"wan3.0-video-c2","prompt":"x","duration":5}`, "not"},
		{"no prompt", `{"model":"wan3.0-video-c3","duration":5}`, "prompt is required"},
		{"blank prompt", `{"model":"wan3.0-video-c3","prompt":"  ","duration":5}`, "prompt is required"},
		{"no duration", `{"model":"wan3.0-video-c3","prompt":"x"}`, "duration is required"},
		{"duration 1", `{"model":"wan3.0-video-c3","prompt":"x","duration":1}`, "from 2 to 30"},
		{"duration 31", `{"model":"wan3.0-video-c3","prompt":"x","duration":31}`, "from 2 to 30"},
		{"duration fraction", `{"model":"wan3.0-video-c3","prompt":"x","duration":5.5}`, "from 2 to 30"},
		{"resolution 4k", `{` + base + `,"resolution":"4k"}`, "resolution must be 480p, 720p, 1080p"},
		{"resolution 2k", `{` + base + `,"resolution":"2k"}`, "resolution must be"},
		{"ratio", `{` + base + `,"ratio":"5:4"}`, "ratio must be one of"},
		{"seed negative", `{` + base + `,"seed":-1}`, "seed must be an integer"},
		{"seed fraction", `{` + base + `,"seed":1.5}`, "seed must be an integer"},
		{"images 11", `{` + base + `,"images":` + image(11) + `}`, "at most 10 reference images"},
		{"videos 6", `{` + base + `,"videos":` + video(6) + `}`, "at most 5 reference videos"},
		{"audios 6", `{` + base + `,"audios":["https://example.com/1.mp3","https://example.com/2.mp3","https://example.com/3.mp3","https://example.com/4.mp3","https://example.com/5.mp3","https://example.com/6.mp3"]}`, "at most 5 reference audios"},
		{"http image", `{` + base + `,"images":["http://example.com/1.png"]}`, "HTTPS"},
		{"private image", `{` + base + `,"images":["https://127.0.0.1/1.png"]}`, "HTTPS"},
		{"first frame http", `{` + base + `,"first_frame":"http://example.com/1.png"}`, "HTTPS"},
		{"both first names", `{` + base + `,"first_frame":"https://example.com/1.png","first_image":"https://example.com/2.png"}`, "only one of first_frame and first_image"},
		{"both last names", `{` + base + `,"last_frame":"https://example.com/1.png","last_image":"https://example.com/2.png"}`, "only one of last_frame and last_image"},
		{"watermark", `{` + base + `,"watermark":true}`, "does not support watermark"},
		{"audio switch", `{` + base + `,"generate_audio":true}`, "does not support generate_audio"},
		{"wan3 audio", `{` + base + `,"audio":true}`, "does not support audio"},
		{"prompt_extend", `{` + base + `,"prompt_extend":true}`, "does not support prompt_extend"},
		{"callback", `{` + base + `,"callback_url":"https://example.com/cb"}`, "does not support callback_url"},
		{"input.media", `{` + base + `,"input":{"media":[]}}`, "does not support input"},
		{"parameters", `{` + base + `,"parameters":{"duration":5}}`, "does not support parameters"},
		{"assets", `{` + base + `,"assets":[]}`, "does not accept assets"},
		{"two videos", `{` + base + `,"n":2}`, "one video per request"},
	} {
		_, _, err := kyyWanPrepare(t, tc.body)
		require.Error(t, err, tc.name)
		require.Contains(t, err.Error(), tc.want, tc.name)
		// The shared helpers name another model family in their messages; a client of this model never reads it.
		require.NotContains(t, err.Error(), "Seedance", tc.name)
	}

	// A request without a model is told so; the model of another platform is refused by its owner check.
	_, _, err := kyyWanPrepare(t, `{"model":"wan3.0-video","prompt":"x","duration":5}`)
	require.Error(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformKyyWan, PPVideoOperationKlingTextToVideo, []byte(`{`+base+`}`))
	require.Error(t, err)
}

func TestTranslatePPVideoUnifiedRequestToKyyWan(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformKyyWan, []byte(`{
		"model":"wan3.0-video-c3","prompt":"a cat","duration":8,"resolution":"720p","ratio":"16:9","seed":3,
		"assets":[
			{"type":"image","url":"https://example.com/f.png","role":"first_frame"},
			{"type":"image","url":"https://example.com/l.png","role":"last_frame"},
			{"type":"image","url":"https://example.com/r.png","name":"hero"},
			{"type":"video","url":"https://example.com/v.mp4"},
			{"type":"audio","url":"https://example.com/a.mp3"}]}`))
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"model": "wan3.0-video-c3", "prompt": "a cat", "duration": float64(8), "resolution": "720p", "ratio": "16:9", "seed": float64(3),
		"first_frame": "https://example.com/f.png", "last_frame": "https://example.com/l.png",
		"images": []any{"https://example.com/r.png"}, "videos": []any{"https://example.com/v.mp4"}, "audios": []any{"https://example.com/a.mp3"},
	}, gjson.ParseBytes(translated).Value())

	// The translation is equivalent to the native request: same upstream body.
	native := `{"model":"wan3.0-video-c3","prompt":"a cat","duration":8,"resolution":"720p","ratio":"16:9","seed":3,` +
		`"first_frame":"https://example.com/f.png","last_frame":"https://example.com/l.png",` +
		`"images":["https://example.com/r.png"],"videos":["https://example.com/v.mp4"],"audios":["https://example.com/a.mp3"]}`
	fromV2, _, err := PreparePPVideoRequestBody(PlatformKyyWan, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	fromV1, _, err := kyyWanPrepare(t, native)
	require.NoError(t, err)
	require.JSONEq(t, string(fromV1), string(fromV2))

	for _, tc := range []struct{ name, body, param string }{
		{"no model", `{"prompt":"x","duration":5}`, "model"},
		{"no prompt", `{"model":"wan3.0-video-c3","duration":5}`, "prompt"},
		{"no duration", `{"model":"wan3.0-video-c3","prompt":"x"}`, "duration"},
		{"audio switch", `{"model":"wan3.0-video-c3","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"watermark", `{"model":"wan3.0-video-c3","prompt":"x","duration":5,"watermark":true}`, "watermark"},
		{"callback", `{"model":"wan3.0-video-c3","prompt":"x","duration":5,"callback_url":"https://example.com/cb"}`, "callback_url"},
		{"options", `{"model":"wan3.0-video-c3","prompt":"x","duration":5,"options":{"prompt_extend":true}}`, "options"},
		{"metadata", `{"model":"wan3.0-video-c3","prompt":"x","duration":5,"metadata":{"a":1}}`, "metadata"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformKyyWan, []byte(tc.body))
		require.Error(t, err, tc.name)
		require.Contains(t, err.Error(), tc.param, tc.name)
	}
}

func TestKyyWanCapabilitiesAndQuote(t *testing.T) {
	t.Parallel()

	caps := PPVideoCapabilitiesForModel(PlatformKyyWan, "WAN3.0-VIDEO-C3")
	require.Equal(t, []string{PPVideoProtocolV1, PPVideoProtocolV2}, caps.Protocols)
	require.Equal(t, &PPVideoDurationRange{Min: 2, Max: 30}, caps.Duration)
	require.Equal(t, []string{"480p", "720p", "1080p"}, caps.Resolutions)
	require.Equal(t, []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}, caps.Ratios)
	require.Equal(t, 10, *caps.Assets.ReferenceImages)
	require.Equal(t, 5, *caps.Assets.ReferenceVideos)
	require.Equal(t, 5, *caps.Assets.ReferenceAudios)
	require.True(t, *caps.Assets.FirstLastFrame)
	require.False(t, *caps.GenerateAudio)
	require.False(t, *caps.Callback)
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformKyyWan, "wan3.0-video-c2").Duration)

	// The capabilities are the limits the normalizer enforces.
	for _, seconds := range []int{caps.Duration.Min, caps.Duration.Max} {
		_, _, err := kyyWanPrepare(t, fmt.Sprintf(`{"model":"wan3.0-video-c3","prompt":"x","duration":%d}`, seconds))
		require.NoError(t, err, seconds)
	}

	// The price estimate enforces the same limits as the submission.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformKyyWan, KyyWanModelC3, "", 10)
	require.True(t, ok)
	require.Equal(t, "1080p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformKyyWan, KyyWanModelC3, "480p", 2)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{KyyWanModelC3, "", 1},
		{KyyWanModelC3, "", 31},
		{KyyWanModelC3, "4k", 10},
		{KyyWanModelC3, "2k", 10},
		{"wan3.0-video-c2", "", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformKyyWan, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}
}
