package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	apophisTestPerSecond  = "Seedance2.5-720p-按秒-Z"
	apophisTestPerRequest = "Seedance2.5-720p-按次-301010"
	apophisTestFrames     = "Seedance2.5-720p-按秒-9图"
	apophisTestNoVideo    = "Seedance2.0-720p-按次-903"
)

func TestApophisSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, apophisSeedanceModels, 15)
	seen := map[string]struct{}{}
	for _, spec := range apophisSeedanceModels {
		require.NotEmpty(t, spec.Name)
		require.Empty(t, spec.PublicName, "no separate public name: %s", spec.Name)
		require.False(t, spec.ForwardResolution, "the resolution is part of the upstream model name: %s", spec.Name)
		require.Len(t, spec.Resolutions, 1, spec.Name)
		require.Contains(t, spec.Name, spec.Resolutions[0], spec.Name)
		require.LessOrEqual(t, spec.MinSeconds, spec.MaxSeconds, spec.Name)
		require.Equal(t, []string{spec.Name}, ppVideoApophisModelAliases(spec.Name))
		_, dup := seen[spec.Name]
		require.False(t, dup, spec.Name)
		seen[spec.Name] = struct{}{}
	}

	// Rows copied from the upstream capability table.
	spec, ok := apophisModelSpecFor("seedance2.5-720p-按次-301010")
	require.True(t, ok)
	require.Equal(t, 30, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	spec, ok = apophisModelSpecFor(apophisTestNoVideo)
	require.True(t, ok)
	require.Equal(t, 0, spec.MaxVideos)
	require.Equal(t, 9, spec.MaxImages)
	require.True(t, spec.FirstLastFrame)
	spec, ok = apophisModelSpecFor("Seedance2.5-1080p-按次-Y")
	require.True(t, ok)
	require.Equal(t, []string{"1080p"}, spec.Resolutions)
	require.False(t, spec.FirstLastFrame)

	// Only these two models take frames.
	frames := 0
	for _, spec := range apophisSeedanceModels {
		if spec.FirstLastFrame {
			frames++
		}
	}
	require.Equal(t, 2, frames)

	// Official-forward lines, MiniMax and unknown names are not in the table.
	for _, model := range []string{"", "官转-Seedance2.5-特价", "国内官转-Seedance2.0", "MiniMax-H3-按秒", "Seedance2.5-720p-按秒-X"} {
		_, ok = apophisModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestApophisSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformApophisSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformApophisSeedance))

	models := []string{"官转-Seedance2.5-Full", apophisTestPerSecond, "MiniMax-H3-按秒"}
	require.Equal(t, models, filterUpstreamModelsForPlatform(PlatformApophisSeedance, models))
	require.Equal(t, models, filterPPVideoPublicModelsForPlatform(PlatformApophisSeedance, models))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformApophisSeedance, apophisTestPerSecond))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformApophisSeedance, "官转-Seedance2.5-Full"))
	require.Equal(t, apophisTestPerSecond, PPVideoPublicModelName(PlatformApophisSeedance, "SEEDANCE2.5-720P-按秒-z"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformApophisSeedance, "other"))
}

// The upstream has no usable model-list endpoint, so account sync answers from the local table
// without any HTTP request (the service has no upstream client at all here).
func TestApophisSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformApophisSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Len(t, models, len(apophisSeedanceModels))
	require.Contains(t, models, apophisTestPerSecond)
	for _, model := range models {
		require.True(t, ppVideoIsApophisSeedanceModel(model), model)
	}

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformApophisSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestApophisSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformApophisSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformApophisSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformApophisSeedance, "ap-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer ap-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformApophisSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformApophisSeedance))

	// Documented flow: queued -> in_progress -> completed / failed.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","object":"video","status":"queued","progress":0}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed","progress":100,"seconds":"8"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"x","message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformApophisSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	failed, err := ParsePPVideoResponse(PlatformApophisSeedance, []byte(`{"id":"task_1","status":"failed","error":{"code":"x","message":"content rejected"}}`))
	require.NoError(t, err)
	require.Equal(t, "content rejected", failed.ErrorMessage)
	rejected, err := ParsePPVideoResponse(PlatformApophisSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", rejected.ErrorMessage)

	// Billing uses the requested seconds, never the length the upstream echoes.
	echoed, err := ParsePPVideoResponse(PlatformApophisSeedance, []byte(`{"id":"task_1","status":"completed","seconds":"9"}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
}

func TestApophisSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: apophisTestPerSecond}

	raw := `{"id":"task_1","object":"video","status":"completed","progress":100,` +
		`"video_url":"https://cdn.example.com/result.mp4","cost":2.9}`
	parsed, err := ParsePPVideoResponse(PlatformApophisSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformApophisSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, apophisTestPerSecond, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	require.False(t, gjson.GetBytes(normalized, "cost").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")

	// The upstream's own docs now show video_url on a completed task (its own /content address,
	// which needs the account key); older responses had none. Either way the client gets this
	// gateway's content address and never the upstream's.
	for _, raw := range []string{
		`{"id":"task_1","object":"video","status":"completed","progress":100,"seconds":"8","completed_at":1794372420,` +
			`"video_url":"https://www.apophiscode.com/v1/videos/task_1/content"}`,
		`{"id":"task_1","object":"video","status":"completed","progress":100,"seconds":"8","completed_at":1794372420}`,
	} {
		parsed, err = ParsePPVideoResponse(PlatformApophisSeedance, []byte(raw))
		require.NoError(t, err)
		require.Equal(t, PPVideoTaskStatusSucceeded, parsed.Status)
		normalized = NormalizePPVideoPublicResponse(PlatformApophisSeedance, []byte(raw), public, parsed)
		require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
		require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
		require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "url").String())
		require.NotContains(t, string(normalized), "apophiscode.com")
	}

	running := `{"id":"task_1","status":"in_progress","progress":30,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformApophisSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformApophisSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")
}

func TestApophisSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "minimal request: no resolution, the upstream field names only",
			body: `{"model":"Seedance2.5-720p-按秒-Z","prompt":"日出时平静海面上的帆船","duration":5,"aspect_ratio":"16:9"}`,
			want: `{"model":"Seedance2.5-720p-按秒-Z","prompt":"日出时平静海面上的帆船","duration":5,"aspect_ratio":"16:9"}`,
		},
		{
			name: "media: images, videos and audio",
			body: `{"model":"Seedance2.5-720p-按秒-Z","prompt":"x","duration":"8","ratio":"9:16",` +
				`"images":["https://example.com/1.png","https://example.com/2.png"],` +
				`"reference_videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"Seedance2.5-720p-按秒-Z","prompt":"x","duration":8,"aspect_ratio":"9:16",` +
				`"images":["https://example.com/1.png","https://example.com/2.png"],` +
				`"videos":["https://example.com/1.mp4"],"audio":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the upstream's own audio field",
			body: `{"model":"Seedance2.0-720p-按次-903","prompt":"x","duration":5,"audio":["https://example.com/1.mp3"]}`,
			want: `{"model":"Seedance2.0-720p-按次-903","prompt":"x","duration":5,"audio":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the resolution of the model may be repeated but is never sent",
			body: `{"model":"Seedance2.5-480p-按秒-Z","prompt":"x","duration":5,"resolution":"480P"}`,
			want: `{"model":"Seedance2.5-480p-按秒-Z","prompt":"x","duration":5}`,
		},
		{
			name: "the model name is matched ignoring case and answered with the table's spelling",
			body: `{"model":"seedance2.5-720p-按次-301010","prompt":"x","duration":30}`,
			want: `{"model":"Seedance2.5-720p-按次-301010","prompt":"x","duration":30}`,
		},
		{
			name: "first and last frame",
			body: `{"model":"Seedance2.5-720p-按秒-9图","prompt":"x","duration":5,` +
				`"first_frame":"https://example.com/a.png","end_frame":"https://example.com/b.png"}`,
			want: `{"model":"Seedance2.5-720p-按秒-9图","prompt":"x","duration":5,` +
				`"first_frame":"https://example.com/a.png","end_frame":"https://example.com/b.png"}`,
		},
		{
			name: "last_frame is an alias of end_frame and a first frame may be sent alone",
			body: `{"model":"Seedance2.0-720p-按次-903","prompt":"x","duration":5,"first_frame":{"url":"https://example.com/a.png"}}`,
			want: `{"model":"Seedance2.0-720p-按次-903","prompt":"x","duration":5,"first_frame":"https://example.com/a.png"}`,
		},
		{
			name: "last_frame alias",
			body: `{"model":"Seedance2.0-720p-按次-903","prompt":"x","duration":5,` +
				`"first_frame":"https://example.com/a.png","last_frame":"https://example.com/b.png"}`,
			want: `{"model":"Seedance2.0-720p-按次-903","prompt":"x","duration":5,` +
				`"first_frame":"https://example.com/a.png","end_frame":"https://example.com/b.png"}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestApophisSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ model, resolution string }{
		{apophisTestPerSecond, "720p"},
		{"Seedance2.5-480p-按秒-Z", "480p"},
		{"Seedance2.5-1080p-按次-Y", "1080p"},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+tc.model+`","prompt":"x","duration":12,"images":["https://example.com/1.png"]}`))
		require.NoError(t, err, tc.model)
		require.Equal(t, tc.model, public.Model)
		require.Equal(t, tc.model, public.UpstreamModel)
		require.Equal(t, int64(12000), public.DurationMilliseconds)
		require.Equal(t, 1, public.VideoCount)
		require.Equal(t, tc.resolution, public.Resolution)
		require.True(t, public.HasImage)

		require.False(t, gjson.GetBytes(prepared, "resolution").Exists())
		meta := PPVideoBillingMetadataFromRequest(PlatformApophisSeedance, prepared)
		require.Equal(t, PlatformApophisSeedance, meta.Platform)
		require.Equal(t, tc.model, meta.Model)
		require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
		require.Equal(t, tc.resolution, meta.VideoResolution)
	}

	// Frames count as an image input.
	_, public, err := PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+apophisTestFrames+`","prompt":"x","duration":5,"first_frame":"https://example.com/a.png"}`))
	require.NoError(t, err)
	require.True(t, public.HasImage)
}

func TestApophisSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	z := `{"model":"Seedance2.5-720p-按秒-Z","prompt":"x",`
	noVideo := `{"model":"Seedance2.0-720p-按次-903","prompt":"x",`
	frames := `{"model":"Seedance2.5-720p-按秒-9图","prompt":"x",`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		{"an official-forward model", `{"model":"官转-Seedance2.5-特价","prompt":"x","duration":5}`, "is not available"},
		{"missing prompt", `{"model":"Seedance2.5-720p-按秒-Z","duration":5}`, "prompt is required"},
		{"missing duration", z + `"aspect_ratio":"16:9"}`, "duration is required"},
		{"below the minimum", z + `"duration":4}`, "from 5 to 30 seconds"},
		{"above the maximum", z + `"duration":31}`, "from 5 to 30 seconds"},
		{"automatic duration is for official-forward lines", z + `"duration":-1}`, "from 5 to 30 seconds"},
		{"the single length model", `{"model":"Seedance2.5-720p-按次-301010","prompt":"x","duration":29}`, "from 30 to 30 seconds"},
		{"another resolution", z + `"duration":5,"resolution":"1080p"}`, "resolution must be 720p for this model"},
		{"unknown ratio", z + `"duration":5,"aspect_ratio":"adaptive"}`, "ratio must be one of"},
		{"video_mode", z + `"duration":5,"video_mode":"extend_forward"}`, "does not support video_mode"},
		{"video_durations", z + `"duration":5,"video_durations":[8]}`, "does not support video_durations"},
		{"web_search", z + `"duration":5,"web_search":true}`, "does not support web_search"},
		{"seed", z + `"duration":5,"seed":1}`, "does not support seed"},
		{"generate_audio", z + `"duration":5,"generate_audio":true}`, "does not support generate_audio"},
		{"watermark", z + `"duration":5,"watermark":false}`, "does not support watermark"},
		{"return_last_frame", z + `"duration":5,"return_last_frame":true}`, "does not support return_last_frame"},
		{"assets on v1", z + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"too many images", z + `"duration":5,"images":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many videos", z + `"duration":5,"videos":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference videos"},
		{"too many audios", z + `"duration":5,"audio":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
		{"nine images on the 9-image line", frames + `"duration":5,"images":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
		{"videos on a model without them", noVideo + `"duration":5,"videos":[` + ref + `]}`, "does not support reference videos"},
		{"two names for the same audio", z + `"duration":5,"audio":[` + ref + `],"audios":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", z + `"duration":5,"images":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"media on a private address", z + `"duration":5,"images":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
		{"frames on a model that has none", z + `"duration":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"an end frame alone", frames + `"duration":5,"end_frame":` + ref + `}`, "end_frame requires first_frame"},
		{"frames with reference images", frames + `"duration":5,"first_frame":` + ref + `,"images":[` + ref + `]}`, "cannot be combined with reference"},
		{"frames with audio", frames + `"duration":5,"first_frame":` + ref + `,"audio":[` + ref + `]}`, "cannot be combined with reference"},
		{"plain http frame", frames + `"duration":5,"first_frame":"http://example.com/a.png"}`, "must be a public HTTPS URL"},
		{"two names for the end frame", frames + `"duration":5,"first_frame":` + ref + `,"end_frame":` + ref + `,"last_frame":` + ref + `}`, "only one of end_frame and last_frame"},
		{"another frame alias", frames + `"duration":5,"first_frame_url":` + ref + `}`, "does not support first or last frames"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The upper limits themselves are accepted.
	_, _, err := PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, []byte(
		z+`"duration":30,"images":[`+repeatedJijiuItems(ref, 30)+`],`+
			`"videos":[`+repeatedJijiuItems(ref, 10)+`],"audio":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)
}

func TestTranslatePPVideoUnifiedRequestToApophis(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformApophisSeedance, []byte(`{
		"model": "Seedance2.5-720p-按秒-Z",
		"prompt": "@Image1 在街道上与@Image2 对话",
		"duration": 5,
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "image", "url": "https://cdn.example.com/other.jpg"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "Seedance2.5-720p-按秒-Z",
		"prompt": "@Image1 在街道上与@Image2 对话",
		"duration": 5,
		"aspect_ratio": "16:9",
		"images": ["https://cdn.example.com/person.jpg", "https://cdn.example.com/other.jpg"],
		"videos": ["https://cdn.example.com/motion.mp4"],
		"audio": ["https://cdn.example.com/voice.mp3"]
	}`, string(prepared))

	// Frames: both, or a first frame alone, on a model that takes them.
	translated, err = TranslatePPVideoUnifiedRequest(PlatformApophisSeedance, []byte(`{
		"model": "Seedance2.5-720p-按秒-9图",
		"prompt": "花朵从含苞到盛开",
		"duration": 5,
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/bud.png"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/bloom.png"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err = PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "Seedance2.5-720p-按秒-9图",
		"prompt": "花朵从含苞到盛开",
		"duration": 5,
		"first_frame": "https://cdn.example.com/bud.png",
		"end_frame": "https://cdn.example.com/bloom.png"
	}`, string(prepared))

	for _, tc := range []struct{ name, body, want string }{
		{"last frame alone", `{"model":"Seedance2.5-720p-按秒-9图","prompt":"x","duration":5,"assets":[{"type":"image","role":"last_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
		{"frames with references", `{"model":"Seedance2.5-720p-按秒-9图","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"},{"type":"image","url":"https://cdn.example.com/b.png"}]}`, "cannot be combined"},
		{"no generate_audio", `{"model":"Seedance2.5-720p-按秒-Z","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"prompt is required", `{"model":"Seedance2.5-720p-按秒-Z","duration":5}`, "prompt"},
		{"duration is required", `{"model":"Seedance2.5-720p-按秒-Z","prompt":"x"}`, "duration"},
		{"model is required", `{"prompt":"x","duration":5}`, "model"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformApophisSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}

	// Frames on a model without them are refused by the same rule as in v1.
	translated, err = TranslatePPVideoUnifiedRequest(PlatformApophisSeedance, []byte(
		`{"model":"Seedance2.5-720p-按秒-Z","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformApophisSeedance, PPVideoOperationGeneric, translated)
	require.ErrorContains(t, err, "does not support first or last frames")

	// The jijiu-shaped platforms still refuse frames outright.
	_, err = TranslatePPVideoUnifiedRequest(PlatformJijiuSeedance, []byte(
		`{"model":"稳定seedance-2.5全参D","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`))
	require.ErrorContains(t, err, "first_frame")
}

// A unified request must behave exactly like the equivalent native request.
func TestUnifiedApophisRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformApophisSeedance,
		`{
			"model": "Seedance2.5-720p-按秒-Z",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"ratio": "9:16",
			"assets": [
				{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
				{"type": "image", "url": "https://cdn.example.com/cat2.png"},
				{"type": "video", "url": "https://cdn.example.com/run.mp4"},
				{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
			]
		}`,
		`{
			"model": "Seedance2.5-720p-按秒-Z",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"aspect_ratio": "9:16",
			"images": ["https://cdn.example.com/cat.png", "https://cdn.example.com/cat2.png"],
			"videos": ["https://cdn.example.com/run.mp4"],
			"audio": ["https://cdn.example.com/voice.mp3"]
		}`,
	)
	requireUnifiedMatchesNative(t, PlatformApophisSeedance,
		`{
			"model": "Seedance2.5-720p-按秒-9图",
			"prompt": "花朵从含苞到盛开",
			"duration": 5,
			"assets": [
				{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/bud.png"},
				{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/bloom.png"}
			]
		}`,
		`{
			"model": "Seedance2.5-720p-按秒-9图",
			"prompt": "花朵从含苞到盛开",
			"duration": 5,
			"first_frame": "https://cdn.example.com/bud.png",
			"end_frame": "https://cdn.example.com/bloom.png"
		}`,
	)
}

func TestApophisSeedanceCapabilitiesQuoteAndHubLines(t *testing.T) {
	t.Parallel()

	z := PPVideoCapabilitiesForModel(PlatformApophisSeedance, apophisTestPerSecond)
	require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 30}, z.Duration)
	require.Equal(t, []string{"720p"}, z.Resolutions)
	require.Equal(t, 30, *z.Assets.ReferenceImages)
	require.Equal(t, 10, *z.Assets.ReferenceVideos)
	require.Equal(t, 10, *z.Assets.ReferenceAudios)
	require.False(t, *z.Assets.FirstLastFrame)
	require.Equal(t, []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"}, z.Ratios)

	frames := PPVideoCapabilitiesForModel(PlatformApophisSeedance, apophisTestFrames)
	require.True(t, *frames.Assets.FirstLastFrame)
	require.Equal(t, 9, *frames.Assets.ReferenceImages)
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformApophisSeedance, "官转-Seedance2.5-Full").Duration)

	// The price estimate enforces the same limits as the submission.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformApophisSeedance, apophisTestPerSecond, "", 10)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformApophisSeedance, apophisTestPerRequest, "720p", 30)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{apophisTestPerSecond, "", 4},
		{apophisTestPerSecond, "", 31},
		{apophisTestPerSecond, "480p", 10},
		{apophisTestPerRequest, "", 29},
		{"官转-Seedance2.5-Full", "", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformApophisSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}

	// Every model of the table is one line of the aggregate group, letter A, and the line's
	// billing follows the model's name.
	count := 0
	for _, line := range SeedanceHubLines() {
		if line.Platform != PlatformApophisSeedance {
			continue
		}
		count++
		spec, found := apophisModelSpecFor(line.Model)
		require.True(t, found, line.Name)
		require.Equal(t, spec.Name, line.Model, line.Name)
		wantBilling := BillingModeVideo
		if strings.Contains(spec.Name, "按次") {
			wantBilling = BillingModePerRequest
		}
		require.Equal(t, wantBilling, line.Billing, line.Name)
		require.True(t, strings.HasSuffix(line.Name, "--A"), line.Name)
		byUpstream, found := SeedanceHubLineByName(spec.Name)
		require.True(t, found, spec.Name)
		require.Equal(t, line.Name, byUpstream.Name)
		require.Equal(t, PPVideoCapabilitiesForModel(PlatformApophisSeedance, spec.Name).Duration,
			PPVideoCapabilitiesForModel(PlatformSeedanceHub, line.Name).Duration)

		_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + line.Name + `","prompt":"x","duration":5}`))
		require.NoError(t, err)
		require.Equal(t, spec.Name, gjson.GetBytes(rewritten, "model").String())
	}
	require.Equal(t, len(apophisSeedanceModels), count)
}
