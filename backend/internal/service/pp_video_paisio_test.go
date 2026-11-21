package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	paisioTest480 = "paisio-seedance-2.5-480p"
	paisioTest720 = "paisio-seedance-2.5-720p"
)

func TestPaisioSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, paisioSeedanceModels, 2)

	full := paisioSeedanceModels[0]
	require.Equal(t, paisioTest480, full.Name)
	require.Equal(t, 4, full.MinSeconds)
	require.Equal(t, 30, full.MaxSeconds)
	require.Equal(t, []string{"480p"}, full.Resolutions)
	require.False(t, full.ForwardResolution, "the resolution is part of the upstream model name")
	require.Equal(t, 30, full.MaxImages)
	require.Equal(t, 10, full.MaxVideos)
	require.Equal(t, 10, full.MaxAudios)

	noVideo := paisioSeedanceModels[1]
	require.Equal(t, paisioTest720, noVideo.Name)
	require.Equal(t, 4, noVideo.MinSeconds)
	require.Equal(t, 30, noVideo.MaxSeconds)
	require.Equal(t, []string{"720p"}, noVideo.Resolutions)
	require.False(t, noVideo.ForwardResolution)
	require.Equal(t, 30, noVideo.MaxImages)
	require.Equal(t, 0, noVideo.MaxVideos)
	require.Equal(t, 10, noVideo.MaxAudios)

	// There is no separate public name: clients see the upstream name.
	for _, spec := range paisioSeedanceModels {
		require.Empty(t, spec.PublicName, spec.Name)
		require.Equal(t, []string{spec.Name}, ppVideoPaisioModelAliases(spec.Name))
	}

	// Lookup ignores case and surrounding spaces; other upstream models are not in the table.
	found, ok := paisioModelSpecFor("  PAISIO-Seedance-2.5-480P ")
	require.True(t, ok)
	require.Equal(t, paisioTest480, found.Name)
	for _, model := range []string{"", "sd2-720p", "seedance-2", "video-fast-480p", "paisio-seedance-2.5-1080p"} {
		_, ok = paisioModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestPaisioSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformPaisioSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformPaisioSeedance))

	// The upstream is a general video site: its other models stay discoverable on the account
	// and in the group's model list, but only the models in the table can be called.
	models := []string{"video-fast-480p", "sd2-720p", paisioTest480, paisioTest720, "grok-imagine-video-1.5"}
	require.Equal(t, models, filterUpstreamModelsForPlatform(PlatformPaisioSeedance, models))
	require.Equal(t, models, filterPPVideoPublicModelsForPlatform(PlatformPaisioSeedance, models))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformPaisioSeedance, paisioTest480))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformPaisioSeedance, paisioTest720))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformPaisioSeedance, "sd2-720p"))
	require.Equal(t, paisioTest480, PPVideoPublicModelName(PlatformPaisioSeedance, "PAISIO-seedance-2.5-480p"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformPaisioSeedance, "other"))
}

func TestPaisioSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformPaisioSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformPaisioSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformPaisioSeedance, "pa-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer pa-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformPaisioSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformPaisioSeedance))

	// The documented status flow is pending -> processing -> completed / failed.
	for _, tc := range []struct{ body, want string }{
		{`{"task_id":"task_123","status":"pending","model":"` + paisioTest480 + `"}`, PPVideoTaskStatusProcessing},
		{`{"task_id":"task_123","status":"processing","progress":45}`, PPVideoTaskStatusProcessing},
		{`{"task_id":"task_123","status":"completed","progress":100}`, PPVideoTaskStatusSucceeded},
		{`{"task_id":"task_123","status":"failed","error":{"message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformPaisioSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_123", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	// A failed task carries error.message; a rejected request may carry a plain string.
	failed, err := ParsePPVideoResponse(PlatformPaisioSeedance, []byte(`{"task_id":"task_123","status":"failed","error":{"message":"content rejected"}}`))
	require.NoError(t, err)
	require.Equal(t, "content rejected", failed.ErrorMessage)
	rejected, err := ParsePPVideoResponse(PlatformPaisioSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", rejected.ErrorMessage)

	// Billing uses the requested seconds, never a length echoed by the upstream.
	echoed, err := ParsePPVideoResponse(PlatformPaisioSeedance, []byte(`{"task_id":"task_123","status":"completed","duration":9}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)

	// An address on a task that is still running proves nothing: only the status decides.
	running, err := ParsePPVideoResponse(PlatformPaisioSeedance, []byte(`{"task_id":"task_123","status":"processing","video_url":"https://cdn.example.com/r.mp4"}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusProcessing, running.Status)
}

func TestPaisioSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: paisioTest480}

	// A completed task carries a CDN address and the supplier's own cost in yuan. Clients get
	// the gateway's content address; neither the CDN address nor the cost is passed on.
	raw := `{"task_id":"task_123","status":"completed","progress":100,` +
		`"video_url":"https://cdn.example.com/result.mp4","cost":2.9}`
	parsed, err := ParsePPVideoResponse(PlatformPaisioSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformPaisioSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, paisioTest480, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_123/content", gjson.GetBytes(normalized, "video_url").String())
	require.Equal(t, "/v1/videos/task_123/content", gjson.GetBytes(normalized, "url").String())
	require.False(t, gjson.GetBytes(normalized, "cost").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")
	require.Empty(t, PPVideoProviderURLFromResponse(normalized), "the upstream address is never kept")

	// A running task shows no address at all.
	running := `{"task_id":"task_123","status":"processing","progress":30,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformPaisioSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformPaisioSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "url").Exists())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")

	// A failed task reports the reason.
	failedRaw := `{"task_id":"task_123","status":"failed","error":{"message":"content rejected"}}`
	parsed, err = ParsePPVideoResponse(PlatformPaisioSeedance, []byte(failedRaw))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformPaisioSeedance, []byte(failedRaw), public, parsed)
	require.Equal(t, "failed", gjson.GetBytes(normalized, "status").String())
}

func TestPaisioSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "minimal request sends no resolution and the upstream field names only",
			body: `{"model":"paisio-seedance-2.5-480p","prompt":"日出时平静海面上的帆船","duration":5,"aspect_ratio":"16:9"}`,
			want: `{"model":"paisio-seedance-2.5-480p","prompt":"日出时平静海面上的帆船","duration":5,"aspect_ratio":"16:9"}`,
		},
		{
			name: "the upstream's own media shape",
			body: `{"model":"paisio-seedance-2.5-480p","prompt":"x","duration":8,` +
				`"image_url":"https://example.com/1.png","extra_images":["https://example.com/2.png","https://example.com/3.png"],` +
				`"extra_videos":["https://example.com/1.mp4"],"extra_audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"paisio-seedance-2.5-480p","prompt":"x","duration":8,` +
				`"image_url":"https://example.com/1.png","extra_images":["https://example.com/2.png","https://example.com/3.png"],` +
				`"extra_videos":["https://example.com/1.mp4"],"extra_audios":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the aliases of the other Seedance models: the first image becomes image_url, the rest extra_images",
			body: `{"model":"PAISIO-SEEDANCE-2.5-480P","prompt":"x","duration":"6","ratio":"9:16",` +
				`"images":[{"image_url":"https://example.com/1.png"},"https://example.com/2.png"],` +
				`"reference_videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"paisio-seedance-2.5-480p","prompt":"x","duration":6,"aspect_ratio":"9:16",` +
				`"image_url":"https://example.com/1.png","extra_images":["https://example.com/2.png"],` +
				`"extra_videos":["https://example.com/1.mp4"],"extra_audios":["https://example.com/1.mp3"]}`,
		},
		{
			name: "a single image has no extra_images",
			body: `{"model":"paisio-seedance-2.5-720p","prompt":"x","duration":6,"image_urls":["https://example.com/1.png"]}`,
			want: `{"model":"paisio-seedance-2.5-720p","prompt":"x","duration":6,"image_url":"https://example.com/1.png"}`,
		},
		{
			name: "the resolution of the model may be repeated but is never sent",
			body: `{"model":"paisio-seedance-2.5-720p","prompt":"x","duration":30,"resolution":"720P"}`,
			want: `{"model":"paisio-seedance-2.5-720p","prompt":"x","duration":30}`,
		},
		{
			name: "the 720p model takes audio",
			body: `{"model":"paisio-seedance-2.5-720p","prompt":"x","duration":4,"audio_urls":["https://example.com/1.mp3"]}`,
			want: `{"model":"paisio-seedance-2.5-720p","prompt":"x","duration":4,"extra_audios":["https://example.com/1.mp3"]}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestPaisioSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ model, resolution string }{
		{paisioTest480, "480p"},
		{paisioTest720, "720p"},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+tc.model+`","prompt":"x","duration":12,"image_urls":["https://example.com/1.png"]}`))
		require.NoError(t, err, tc.model)
		require.Equal(t, tc.model, public.Model)
		require.Equal(t, tc.model, public.UpstreamModel)
		require.Equal(t, int64(12000), public.DurationMilliseconds)
		require.Equal(t, 1, public.VideoCount)
		require.Equal(t, tc.resolution, public.Resolution)
		require.True(t, public.HasImage)

		// The resolution is not in the upstream body, yet the channel price tier still follows it.
		require.False(t, gjson.GetBytes(prepared, "resolution").Exists())
		meta := PPVideoBillingMetadataFromRequest(PlatformPaisioSeedance, prepared)
		require.Equal(t, PlatformPaisioSeedance, meta.Platform)
		require.Equal(t, tc.model, meta.Model)
		require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
		require.Equal(t, 1, meta.VideoCount)
		require.Equal(t, tc.resolution, meta.VideoResolution)
	}
}

func TestPaisioSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	head := `{"model":"paisio-seedance-2.5-480p","prompt":"x",`
	head720 := `{"model":"paisio-seedance-2.5-720p","prompt":"x",`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		{"a non-Seedance model of the upstream", `{"model":"gpt-image-2","prompt":"x","duration":5}`, "is not available"},
		{"missing prompt", `{"model":"paisio-seedance-2.5-480p","duration":5}`, "prompt is required"},
		{"missing duration", head + `"aspect_ratio":"16:9"}`, "duration is required"},
		{"below the minimum", head + `"duration":3}`, "from 4 to 30 seconds"},
		{"above the maximum", head + `"duration":31}`, "from 4 to 30 seconds"},
		{"fractional duration", head + `"duration":5.5}`, "duration for this model must be an integer"},
		{"another resolution", head + `"duration":5,"resolution":"720p"}`, "resolution must be 480p for this model"},
		{"another resolution on 720p", head720 + `"duration":5,"resolution":"1080p"}`, "resolution must be 720p for this model"},
		{"unknown ratio", head + `"duration":5,"aspect_ratio":"5:4"}`, "ratio must be one of"},
		{"start frame", head + `"duration":5,"start_image_url":` + ref + `}`, "does not support first or last frames"},
		{"end frame", head + `"duration":5,"end_image_url":` + ref + `}`, "does not support first or last frames"},
		{"first_frame", head + `"duration":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"seed", head + `"duration":5,"seed":1}`, "does not support seed"},
		{"generate_audio", head + `"duration":5,"generate_audio":true}`, "does not support generate_audio"},
		{"callback", head + `"duration":5,"callback_url":"https://example.com/cb"}`, "does not support callback_url"},
		{"assets on v1", head + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"too many images", head + `"duration":5,"images":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many images in the upstream shape", head + `"duration":5,"image_url":` + ref + `,"extra_images":[` + repeatedJijiuItems(ref, 30) + `]}`, "at most 30 reference images"},
		{"too many videos", head + `"duration":5,"extra_videos":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference videos"},
		{"too many audios", head + `"duration":5,"extra_audios":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
		{"videos on the 720p model", head720 + `"duration":5,"extra_videos":[` + ref + `]}`, "does not support reference videos"},
		{"videos on the 720p model through an alias", head720 + `"duration":5,"videos":[` + ref + `]}`, "does not support reference videos"},
		{"above the maximum on the 720p model", head720 + `"duration":31}`, "from 4 to 30 seconds"},
		{"the upstream image shape mixed with the images list", head + `"duration":5,"image_url":` + ref + `,"images":[` + ref + `]}`, "accepts only one of image_url / extra_images and the images list"},
		{"two names for the same videos", head + `"duration":5,"extra_videos":[` + ref + `],"videos":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", head + `"duration":5,"image_url":"http://example.com/1.png"}`, "must be a public HTTPS URL"},
		{"media on a private address", head + `"duration":5,"extra_images":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// A model of the upstream that is not in the table is refused, whatever its name.
	_, _, err := PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"paisio-seedance-2.5-1080p","prompt":"x","duration":5}`))
	require.Error(t, err)

	// The upper limits themselves are accepted: 480p takes reference videos, 720p does not.
	_, _, err = PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, []byte(
		head+`"duration":30,"image_url":`+ref+`,"extra_images":[`+repeatedJijiuItems(ref, 29)+`],`+
			`"extra_videos":[`+repeatedJijiuItems(ref, 10)+`],"extra_audios":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, []byte(
		head720+`"duration":30,"image_url":`+ref+`,"extra_images":[`+repeatedJijiuItems(ref, 29)+`],`+
			`"extra_audios":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)
}

func TestTranslatePPVideoUnifiedRequestToPaisio(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformPaisioSeedance, []byte(`{
		"model": "paisio-seedance-2.5-480p",
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
	prepared, _, err := PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "paisio-seedance-2.5-480p",
		"prompt": "@Image1 在街道上与@Image2 对话",
		"duration": 5,
		"aspect_ratio": "16:9",
		"image_url": "https://cdn.example.com/person.jpg",
		"extra_images": ["https://cdn.example.com/other.jpg"],
		"extra_videos": ["https://cdn.example.com/motion.mp4"],
		"extra_audios": ["https://cdn.example.com/voice.mp3"]
	}`, string(prepared))

	for _, tc := range []struct{ name, body, want string }{
		{"no frames", `{"model":"paisio-seedance-2.5-480p","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
		{"no generate_audio", `{"model":"paisio-seedance-2.5-480p","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"prompt is required", `{"model":"paisio-seedance-2.5-480p","duration":5}`, "prompt"},
		{"duration is required", `{"model":"paisio-seedance-2.5-480p","prompt":"x"}`, "duration"},
		{"model is required", `{"prompt":"x","duration":5}`, "model"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformPaisioSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}

	// v2: a reference video on the 720p model is refused by the same limits as in v1.
	translated, err = TranslatePPVideoUnifiedRequest(PlatformPaisioSeedance, []byte(
		`{"model":"paisio-seedance-2.5-720p","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformPaisioSeedance, PPVideoOperationGeneric, translated)
	require.ErrorContains(t, err, "does not support reference videos")
}

// A unified request must behave exactly like the equivalent native request.
func TestUnifiedPaisioRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformPaisioSeedance,
		`{
			"model": "paisio-seedance-2.5-480p",
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
			"model": "paisio-seedance-2.5-480p",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"aspect_ratio": "9:16",
			"image_url": "https://cdn.example.com/cat.png",
			"extra_images": ["https://cdn.example.com/cat2.png"],
			"extra_videos": ["https://cdn.example.com/run.mp4"],
			"extra_audios": ["https://cdn.example.com/voice.mp3"]
		}`,
	)
}

func TestPaisioSeedanceCapabilitiesQuoteAndHubLines(t *testing.T) {
	t.Parallel()

	full := PPVideoCapabilitiesForModel(PlatformPaisioSeedance, paisioTest480)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, full.Duration)
	require.Equal(t, []string{"480p"}, full.Resolutions)
	require.Equal(t, 30, *full.Assets.ReferenceImages)
	require.Equal(t, 10, *full.Assets.ReferenceVideos)
	require.Equal(t, 10, *full.Assets.ReferenceAudios)
	require.Equal(t, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "3:2", "2:3"}, full.Ratios)

	noVideo := PPVideoCapabilitiesForModel(PlatformPaisioSeedance, paisioTest720)
	require.Equal(t, []string{"720p"}, noVideo.Resolutions)
	require.Equal(t, 0, *noVideo.Assets.ReferenceVideos)
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformPaisioSeedance, "sd2-720p").Duration)

	// The price estimate enforces the same limits as the submission.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformPaisioSeedance, paisioTest480, "", 10)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformPaisioSeedance, paisioTest720, "720p", 30)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{paisioTest480, "", 3},
		{paisioTest480, "", 31},
		{paisioTest480, "720p", 10},
		{paisioTest720, "480p", 10},
		{"sd2-720p", "", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformPaisioSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}

	// The aggregate group serves both models, answering to the upstream names too.
	for _, tc := range []struct{ lineName, model string }{
		{"Seedance2.5--480p--满血原生过人脸--按秒--P", paisioTest480},
		{"Seedance2.5--720p--原生过人脸--按秒--P", paisioTest720},
	} {
		line, found := SeedanceHubLineByName(tc.lineName)
		require.True(t, found, tc.lineName)
		require.Equal(t, PlatformPaisioSeedance, line.Platform)
		require.Equal(t, tc.model, line.Model)
		require.Equal(t, BillingModeVideo, line.Billing)
		require.Equal(t, seedanceHubVersion25, line.Version)
		byUpstream, found := SeedanceHubLineByName(tc.model)
		require.True(t, found, tc.model)
		require.Equal(t, tc.lineName, byUpstream.Name)
		require.Equal(t, PPVideoCapabilitiesForModel(PlatformPaisioSeedance, tc.model).Duration,
			PPVideoCapabilitiesForModel(PlatformSeedanceHub, tc.lineName).Duration)

		_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + tc.lineName + `","prompt":"x","duration":5}`))
		require.NoError(t, err)
		require.Equal(t, tc.model, gjson.GetBytes(rewritten, "model").String())
	}
	require.Equal(t, 0, *PPVideoCapabilitiesForModel(PlatformSeedanceHub, "Seedance2.5--720p--原生过人脸--按秒--P").Assets.ReferenceVideos)
}
