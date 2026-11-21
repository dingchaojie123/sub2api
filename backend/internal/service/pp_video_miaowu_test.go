package service

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const miaowuTestModel = "seedance-2.5-slow"

func TestMiaowuSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, miaowuSeedanceModels, 18)
	spec := miaowuSeedanceModels[0]
	require.Equal(t, miaowuTestModel, spec.Name)
	require.Equal(t, 4, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"480p", "720p"}, spec.Resolutions)
	require.Equal(t, "720p", spec.defaultResolution())
	require.Equal(t, 30, spec.MaxImages)
	require.Equal(t, 0, spec.MaxVideos, "no reference videos since the final table of 2026-11-19")
	require.Equal(t, 10, spec.MaxAudios)

	// Lookup ignores case and surrounding spaces; other upstream models are not in the table.
	found, ok := miaowuModelSpecFor("  Seedance-2.5-SLOW ")
	require.True(t, ok)
	require.Equal(t, miaowuTestModel, found.Name)
	for _, model := range []string{"", "seedance-2.5-cheap", "seedance-2.5", "gpt-image-2"} {
		_, ok = miaowuModelSpecFor(model)
		require.False(t, ok, model)
	}
	require.Equal(t, []string{miaowuTestModel}, ppVideoMiaowuModelAliases(miaowuTestModel))

	// seedance-2.5-deal has the same limits except that it takes no reference videos.
	deal, ok := miaowuModelSpecFor("seedance-2.5-deal")
	require.True(t, ok)
	require.Equal(t, 4, deal.MinSeconds)
	require.Equal(t, 30, deal.MaxSeconds)
	require.Equal(t, []string{"720p"}, deal.Resolutions)
	require.True(t, deal.ForwardResolution)
	require.Equal(t, 30, deal.MaxImages)
	require.Equal(t, 0, deal.MaxVideos)
	require.Equal(t, 10, deal.MaxAudios)
}

func TestMiaowuSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformMiaowuSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformMiaowuSeedance))

	// The upstream lists many models. They stay discoverable on the account and in the group's
	// model list; only the models in the table can be called.
	models := []string{"gpt-image-2", "seedance-2.5-cheap", miaowuTestModel, "seedance-2.5-deal", "wan3.0-video"}
	require.Equal(t, models, filterUpstreamModelsForPlatform(PlatformMiaowuSeedance, models))
	require.Equal(t, models, filterPPVideoPublicModelsForPlatform(PlatformMiaowuSeedance, models))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, miaowuTestModel))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, "seedance-2.5-deal"))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, "seedance-2.5-cheap"))
	require.False(t, ppVideoModelKnownForeignToPlatform(PlatformMiaowuSeedance, miaowuTestModel))

	account := &Account{Platform: PlatformMiaowuSeedance, Credentials: map[string]any{
		"model_mapping": map[string]any{miaowuTestModel: miaowuTestModel},
	}}
	require.True(t, account.IsModelSupported("SEEDANCE-2.5-SLOW"))
}

func TestMiaowuSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformMiaowuSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformMiaowuSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformMiaowuSeedance, "mw-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer mw-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformMiaowuSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformMiaowuSeedance))

	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_123","object":"video","status":"queued","progress":0}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_123","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_123","status":"completed","progress":100}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_123","status":"failed","progress":100}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_123", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	// A failed task carries an error object; a rejected request carries a plain string.
	failed, err := ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(
		`{"id":"task_123","object":"video","status":"failed","progress":100,"error":{"code":"task_failed","message":"generation failed"}}`))
	require.NoError(t, err)
	require.Equal(t, "generation failed", failed.ErrorMessage)
	rejected, err := ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(`{"error":"unknown field \"size\""}`))
	require.NoError(t, err)
	require.Equal(t, `unknown field "size"`, rejected.ErrorMessage)

	// Billing uses the requested seconds, never a length echoed by the upstream.
	echoed, err := ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(`{"id":"task_123","status":"completed","duration":9,"seconds":9}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)

	// An address on a task that is still running proves nothing: only the status decides.
	running, err := ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(`{"id":"task_123","status":"in_progress","url":"https://cdn.example.com/r.mp4"}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusProcessing, running.Status)
	finished, err := ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(`{"id":"task_123","status":"completed"}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, finished.Status)
}

func TestMiaowuSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: miaowuTestModel}

	// The documented completed response carries a direct url. Clients get the gateway's own
	// content address; the direct address is only kept as the proxy's fallback.
	raw := `{"id":"task_123","object":"video","model":"seedance-2.5-slow","status":"completed","progress":100,` +
		`"created_at":1730000000,"completed_at":1730000060,"url":"https://cdn.example.com/result.mp4"}`
	parsed, err := ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformMiaowuSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, miaowuTestModel, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_123/content", gjson.GetBytes(normalized, "video_url").String())
	require.Equal(t, "/v1/videos/task_123/content", gjson.GetBytes(normalized, "url").String())
	require.Equal(t, "https://cdn.example.com/result.mp4", PPVideoProviderURLFromResponse(normalized))

	// The url may be absent when the result needs authentication: the content endpoint is used.
	bare := `{"id":"task_123","object":"video","status":"completed","progress":100}`
	parsed, err = ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(bare))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformMiaowuSeedance, []byte(bare), public, parsed)
	require.Equal(t, "/v1/videos/task_123/content", gjson.GetBytes(normalized, "video_url").String())
	require.Empty(t, PPVideoProviderURLFromResponse(normalized))

	// A running task shows no address at all.
	running := `{"id":"task_123","status":"in_progress","progress":30,"url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformMiaowuSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformMiaowuSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "url").Exists())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.Empty(t, PPVideoProviderURLFromResponse(normalized))
}

func TestMiaowuSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "minimal request sends the fixed resolution and the upstream field names only",
			body: `{"model":"seedance-2.5-slow","prompt":"日出时平静海面上的帆船","duration":5,"ratio":"16:9"}`,
			want: `{"model":"seedance-2.5-slow","prompt":"日出时平静海面上的帆船","seconds":5,"ratio":"16:9","resolution":"720p"}`,
		},
		{
			// The media field names are shared by the models of this platform; tj takes videos.
			name: "the upstream's own media names",
			body: `{"model":"seedance-2.5-tj","prompt":"x","seconds":8,"resolution":"720p",` +
				`"image_urls":["https://example.com/1.png"],"video_urls":["https://example.com/1.mp4"],"audio_urls":["https://example.com/1.mp3"]}`,
			want: `{"model":"seedance-2.5-tj","prompt":"x","seconds":8,"resolution":"720p",` +
				`"image_urls":["https://example.com/1.png"],"video_urls":["https://example.com/1.mp4"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the aliases of the other Seedance models are written under the upstream names",
			body: `{"model":"SEEDANCE-2.5-TJ","prompt":"x","duration":"6","aspect_ratio":"9:16",` +
				`"images":[{"image_url":"https://example.com/1.png"}],"reference_videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"seedance-2.5-tj","prompt":"x","seconds":6,"ratio":"9:16","resolution":"720p",` +
				`"image_urls":["https://example.com/1.png"],"video_urls":["https://example.com/1.mp4"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the limits are inclusive",
			body: `{"model":"seedance-2.5-slow","prompt":"x","duration":30}`,
			want: `{"model":"seedance-2.5-slow","prompt":"x","seconds":30,"resolution":"720p"}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestMiaowuSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-slow","prompt":"x","duration":12,"image_urls":["https://example.com/1.png"]}`))
	require.NoError(t, err)
	require.Equal(t, miaowuTestModel, public.Model)
	require.Equal(t, miaowuTestModel, public.UpstreamModel)
	require.Equal(t, int64(12000), public.DurationMilliseconds)
	require.Equal(t, 1, public.VideoCount)
	require.Equal(t, "720p", public.Resolution)
	require.True(t, public.HasImage)

	meta := PPVideoBillingMetadataFromRequest(PlatformMiaowuSeedance, prepared)
	require.Equal(t, PlatformMiaowuSeedance, meta.Platform)
	require.Equal(t, miaowuTestModel, meta.Model)
	require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
	require.Equal(t, 1, meta.VideoCount)
	require.Equal(t, "720p", meta.VideoResolution)
}

func TestMiaowuSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	head := `{"model":"seedance-2.5-slow","prompt":"x",`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		{"a non-Seedance model of the upstream", `{"model":"gpt-image-2","prompt":"x","duration":5}`, "is not available"},
		// Other Seedance models are caught earlier by the cross-platform check.
		{"a Seedance model that is not in the table", `{"model":"seedance-2.5-cheap","prompt":"x","duration":5}`, "is not supported by platform"},
		{"missing prompt", `{"model":"seedance-2.5-slow","duration":5}`, "prompt is required"},
		{"missing duration", head + `"ratio":"16:9"}`, "duration is required"},
		{"below the minimum", head + `"duration":3}`, "from 4 to 30 seconds"},
		{"above the maximum", head + `"duration":31}`, "from 4 to 30 seconds"},
		{"fractional duration", head + `"duration":5.5}`, "duration for this model must be an integer"},
		{"another resolution", head + `"duration":5,"resolution":"1080p"}`, "resolution must be 480p, 720p for this model"},
		{"first frame", head + `"duration":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"input_reference", head + `"duration":5,"input_reference":` + ref + `}`, "does not support first or last frames"},
		{"seed", head + `"duration":5,"seed":1}`, "does not support seed"},
		{"generate_audio", head + `"duration":5,"generate_audio":true}`, "does not support generate_audio"},
		{"callback", head + `"duration":5,"callback_url":"https://example.com/cb"}`, "does not support callback_url"},
		{"assets on v1", head + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"too many images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"a reference video", head + `"duration":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
		{"videos on the deal model", `{"model":"seedance-2.5-deal","prompt":"x","duration":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
		{"too many images for the deal model", `{"model":"seedance-2.5-deal","prompt":"x","duration":5,"image_urls":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"above the maximum for the deal model", `{"model":"seedance-2.5-deal","prompt":"x","duration":31}`, "from 4 to 30 seconds"},
		{"too many audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
		{"two names for the same media", head + `"duration":5,"image_urls":[` + ref + `],"images":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", head + `"duration":5,"image_urls":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"media on a private address", head + `"duration":5,"image_urls":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The upper limits themselves are accepted (slow takes no reference videos since 2026-11-19).
	_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		head+`"duration":5,"image_urls":[`+repeatedJijiuItems(ref, 30)+`],"audio_urls":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)
}

func TestMiaowuSeedanceDealModel(t *testing.T) {
	t.Parallel()

	prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-deal","prompt":"x","duration":6,"image_urls":["https://example.com/1.png"],"audio_urls":["https://example.com/1.mp3"]}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"seedance-2.5-deal","prompt":"x","seconds":6,"resolution":"720p",`+
		`"image_urls":["https://example.com/1.png"],"audio_urls":["https://example.com/1.mp3"]}`, string(prepared))
	require.Equal(t, "seedance-2.5-deal", public.Model)
	meta := PPVideoBillingMetadataFromRequest(PlatformMiaowuSeedance, prepared)
	require.Equal(t, "seedance-2.5-deal", meta.Model)
	require.Equal(t, int64(6000), meta.RequestedDurationMilliseconds)
	require.Equal(t, "720p", meta.VideoResolution)

	// v2: a reference video is refused by the same limits.
	translated, err := TranslatePPVideoUnifiedRequest(PlatformMiaowuSeedance, []byte(
		`{"model":"seedance-2.5-deal","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, translated)
	require.ErrorContains(t, err, "does not support reference videos")

	c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, "seedance-2.5-deal")
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, c.Duration)
	require.Equal(t, 30, *c.Assets.ReferenceImages)
	require.Equal(t, 0, *c.Assets.ReferenceVideos)
	require.Equal(t, 10, *c.Assets.ReferenceAudios)

	// deal is 720p only: 480p is refused here, by the quote, and in the capabilities.
	_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-deal","prompt":"x","duration":6,"resolution":"480p"}`))
	require.ErrorContains(t, err, "resolution must be 720p for this model")
	require.Equal(t, []string{"720p"}, c.Resolutions)
	_, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, "seedance-2.5-deal", "480p", 10)
	require.False(t, ok)

	const lineName = "Seedance2.5--720p--deal--按秒--W"
	line, found := SeedanceHubLineByName(lineName)
	require.True(t, found)
	require.Equal(t, "seedance-2.5-deal", line.Model)
	require.Equal(t, BillingModeVideo, line.Billing)
	_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + lineName + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, "seedance-2.5-deal", gjson.GetBytes(rewritten, "model").String())
}

func TestMiaowuSeedanceFlashModel(t *testing.T) {
	t.Parallel()

	spec, ok := miaowuModelSpecFor("seedance-2.5-flash")
	require.True(t, ok)
	require.Equal(t, 4, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"480p", "720p"}, spec.Resolutions)
	require.Equal(t, "720p", spec.defaultResolution())
	require.Equal(t, 10, spec.MaxImages)
	require.Equal(t, 0, spec.MaxVideos)
	require.Equal(t, 5, spec.MaxAudios)
	require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, "seedance-2.5-flash"))

	// 720p is the default and both resolutions are sent upstream as asked.
	for _, tc := range []struct{ body, want string }{
		{`{"model":"seedance-2.5-flash","prompt":"x","duration":5}`, `{"model":"seedance-2.5-flash","prompt":"x","seconds":5,"resolution":"720p"}`},
		{`{"model":"seedance-2.5-flash","prompt":"x","duration":5,"resolution":"480p"}`, `{"model":"seedance-2.5-flash","prompt":"x","seconds":5,"resolution":"480p"}`},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.JSONEq(t, tc.want, string(prepared), tc.body)
		require.Equal(t, "seedance-2.5-flash", public.Model)
	}

	const ref = `"https://example.com/1.png"`
	head := `{"model":"seedance-2.5-flash","prompt":"x",`
	for _, tc := range []struct{ name, body, want string }{
		{"1080p", head + `"duration":5,"resolution":"1080p"}`, "resolution must be 480p, 720p for this model"},
		{"below the minimum", head + `"duration":3}`, "from 4 to 30 seconds"},
		{"above the maximum", head + `"duration":31}`, "from 4 to 30 seconds"},
		{"eleven images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference images"},
		{"a reference video", head + `"duration":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
		{"six audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, 6) + `]}`, "at most 5 reference audios"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
	_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		head+`"duration":30,"image_urls":[`+repeatedJijiuItems(ref, 10)+`],"audio_urls":[`+repeatedJijiuItems(ref, 5)+`]}`))
	require.NoError(t, err)

	// v2 reaches the same limits.
	translated, err := TranslatePPVideoUnifiedRequest(PlatformMiaowuSeedance, []byte(
		`{"model":"seedance-2.5-flash","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, translated)
	require.ErrorContains(t, err, "does not support reference videos")

	c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, "seedance-2.5-flash")
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, c.Duration)
	require.Equal(t, []string{"480p", "720p"}, c.Resolutions)
	require.Equal(t, 10, *c.Assets.ReferenceImages)
	require.Equal(t, 0, *c.Assets.ReferenceVideos)
	require.Equal(t, 5, *c.Assets.ReferenceAudios)

	// The price estimate accepts both resolutions within 4-30 seconds and nothing else.
	for _, resolution := range []string{"", "480p", "720p"} {
		_, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, "seedance-2.5-flash", resolution, 10)
		require.True(t, ok, resolution)
	}
	for _, tc := range []struct {
		resolution string
		seconds    int
	}{{"1080p", 10}, {"", 3}, {"", 31}} {
		_, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, "seedance-2.5-flash", tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}

	// The aggregate group serves it as a per-request line, and the line name carries both resolutions.
	const lineName = "Seedance2.5--480p/720p--flash--按次--W"
	line, found := SeedanceHubLineByName(lineName)
	require.True(t, found)
	require.Equal(t, "seedance-2.5-flash", line.Model)
	require.Equal(t, BillingModePerRequest, line.Billing)
	_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + lineName + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, "seedance-2.5-flash", gjson.GetBytes(rewritten, "model").String())
}

func TestMiaowuSeedanceProModel(t *testing.T) {
	t.Parallel()

	spec, ok := miaowuModelSpecFor("seedance-2.5-pro")
	require.True(t, ok)
	require.Equal(t, 5, spec.MinSeconds, "5-30 seconds since the final table of 2026-11-19")
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"720p"}, spec.Resolutions)
	require.Equal(t, 30, spec.MaxImages)
	require.Equal(t, 0, spec.MaxVideos)
	require.Equal(t, 10, spec.MaxAudios)
	require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, "seedance-2.5-pro"))

	prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-pro","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"seedance-2.5-pro","prompt":"x","seconds":5,"resolution":"720p"}`, string(prepared))
	require.Equal(t, "seedance-2.5-pro", public.Model)
	require.Equal(t, "720p", PPVideoBillingMetadataFromRequest(PlatformMiaowuSeedance, prepared).VideoResolution)

	const ref = `"https://example.com/1.png"`
	head := `{"model":"seedance-2.5-pro","prompt":"x",`
	for _, tc := range []struct{ name, body, want string }{
		{"480p", head + `"duration":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"below the minimum", head + `"duration":4}`, "from 5 to 30 seconds"},
		{"above the maximum", head + `"duration":31}`, "from 5 to 30 seconds"},
		{"31 images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"a reference video", head + `"duration":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
		{"11 audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
	_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		head+`"duration":30,"image_urls":[`+repeatedJijiuItems(ref, 30)+`],"audio_urls":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)

	translated, err := TranslatePPVideoUnifiedRequest(PlatformMiaowuSeedance, []byte(
		`{"model":"seedance-2.5-pro","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, translated)
	require.ErrorContains(t, err, "does not support reference videos")

	c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, "seedance-2.5-pro")
	require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 30}, c.Duration)
	require.Equal(t, []string{"720p"}, c.Resolutions)
	require.Equal(t, 30, *c.Assets.ReferenceImages)
	require.Equal(t, 0, *c.Assets.ReferenceVideos)
	require.Equal(t, 10, *c.Assets.ReferenceAudios)

	_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, "seedance-2.5-pro", "", 10)
	require.True(t, ok)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, "seedance-2.5-pro", "", 4)
	require.False(t, ok, "below the shortest length")
	_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, "seedance-2.5-pro", "480p", 10)
	require.False(t, ok)

	const lineName = "Seedance2.5--720p--pro--按次--W"
	line, found := SeedanceHubLineByName(lineName)
	require.True(t, found)
	require.Equal(t, "seedance-2.5-pro", line.Model)
	require.Equal(t, BillingModePerRequest, line.Billing)
	_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + lineName + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, "seedance-2.5-pro", gjson.GetBytes(rewritten, "model").String())
}

// The five Seedance 2.0 models added 2026-11-13 share 5-15 seconds and differ in resolutions,
// reference limits and hub line.
func TestMiaowuSeedance20Models(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	for _, tc := range []struct {
		model       string
		line        string
		billing     BillingMode
		resolutions []string
		videos      int
	}{
		{"seedance-2.0-fast-deal", "Seedance2.0--720p--fast-deal--按次--W", BillingModePerRequest, []string{"720p"}, 0},
		{"seedance-2.0-fast", "Seedance2.0--480p/720p--fast--按次--W", BillingModePerRequest, []string{"480p", "720p"}, 0},
		{"seedance-2.0-916", "Seedance2.0--720p--916--按次--W", BillingModePerRequest, []string{"720p"}, 3},
		{"seedance-2.0-deal", "Seedance2.0--480p/720p--deal--按次--W", BillingModePerRequest, []string{"480p", "720p"}, 3},
	} {
		tc := tc
		t.Run(tc.model, func(t *testing.T) {
			t.Parallel()

			spec, ok := miaowuModelSpecFor(tc.model)
			require.True(t, ok)
			require.Equal(t, 5, spec.MinSeconds)
			require.Equal(t, 15, spec.MaxSeconds)
			require.Equal(t, tc.resolutions, spec.Resolutions)
			require.Equal(t, "720p", spec.defaultResolution())
			require.Equal(t, 9, spec.MaxImages)
			require.Equal(t, tc.videos, spec.MaxVideos)
			require.Equal(t, 3, spec.MaxAudios)
			require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, tc.model))

			// 720p is the default; the upstream gets the model, the length and the resolution.
			prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
				`{"model":"`+tc.model+`","prompt":"x","duration":10}`))
			require.NoError(t, err)
			require.JSONEq(t, `{"model":"`+tc.model+`","prompt":"x","seconds":10,"resolution":"720p"}`, string(prepared))
			require.Equal(t, tc.model, public.Model)

			head := `{"model":"` + tc.model + `","prompt":"x",`
			cases := []struct{ name, body, want string }{
				{"1080p", head + `"duration":5,"resolution":"1080p"}`, "resolution must be " + strings.Join(tc.resolutions, ", ") + " for this model"},
				{"below the minimum", head + `"duration":4}`, "from 5 to 15 seconds"},
				{"above the maximum", head + `"duration":16}`, "from 5 to 15 seconds"},
				{"ten images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
				{"four audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference audios"},
			}
			if tc.videos == 0 {
				cases = append(cases, struct{ name, body, want string }{
					"a reference video", head + `"duration":5,"video_urls":[` + ref + `]}`, "does not support reference videos"})
			} else {
				cases = append(cases, struct{ name, body, want string }{
					"four reference videos", head + `"duration":5,"video_urls":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference videos"})
			}
			for _, c := range cases {
				_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(c.body))
				require.ErrorContains(t, err, c.want, c.name)
			}

			// Whole numbers from 5 to 15 are accepted, not only 5, 10 and 15.
			for _, seconds := range []string{"5", "7", "15"} {
				_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
					head+`"duration":`+seconds+`}`))
				require.NoError(t, err, seconds)
			}
			// The largest allowed reference set goes through.
			full := head + `"duration":15,"image_urls":[` + repeatedJijiuItems(ref, 9) + `],"audio_urls":[` + repeatedJijiuItems(ref, 3) + `]`
			if tc.videos > 0 {
				full += `,"video_urls":[` + repeatedJijiuItems(ref, tc.videos) + `]`
			}
			_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(full+`}`))
			require.NoError(t, err)

			// v2 reaches the same limits.
			if tc.videos == 0 {
				translated, err := TranslatePPVideoUnifiedRequest(PlatformMiaowuSeedance, []byte(
					`{"model":"`+tc.model+`","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`))
				require.NoError(t, err)
				_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, translated)
				require.ErrorContains(t, err, "does not support reference videos")
			}

			c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, tc.model)
			require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 15}, c.Duration)
			require.Equal(t, tc.resolutions, c.Resolutions)
			require.Equal(t, 9, *c.Assets.ReferenceImages)
			require.Equal(t, tc.videos, *c.Assets.ReferenceVideos)
			require.Equal(t, 3, *c.Assets.ReferenceAudios)

			// The price estimate follows the same table.
			for _, resolution := range append([]string{""}, tc.resolutions...) {
				_, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, resolution, 10)
				require.True(t, ok, resolution)
			}
			_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, "1080p", 10)
			require.False(t, ok)
			_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, "", 4)
			require.False(t, ok)
			_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, "", 16)
			require.False(t, ok)

			// The aggregate group serves it as its own line, and the model name of the line is the
			// platform's own name (a request under the platform name reaches the same line).
			line, found := SeedanceHubLineByName(tc.line)
			require.True(t, found)
			require.Equal(t, tc.model, line.Model)
			require.Equal(t, PlatformMiaowuSeedance, line.Platform)
			require.Equal(t, seedanceHubVersion20, line.Version)
			require.Equal(t, tc.billing, line.Billing)
			_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + tc.line + `","prompt":"x","duration":5}`))
			require.NoError(t, err)
			require.Equal(t, tc.model, gjson.GetBytes(rewritten, "model").String())
			byModel, found := SeedanceHubLineByName(tc.model)
			require.True(t, found)
			require.Equal(t, tc.line, byModel.Name)
		})
	}
}

// seedance-2.5-flash-tj and seedance-2.5-tj (2026-11-18): 4-30 seconds, per request, and unlike
// flash / pro they take reference videos.
func TestMiaowuSeedance25TjModels(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	for _, tc := range []struct {
		model       string
		line        string
		resolutions []string
		images      int
		videos      int
		audios      int
	}{
		{"seedance-2.5-flash-tj", "Seedance2.5--720p--flash-tj--按次--W", []string{"720p"}, 10, 5, 5},
		{"seedance-2.5-tj", "Seedance2.5--480p/720p--tj--按次--W", []string{"480p", "720p"}, 30, 10, 10},
		{"seedance-2.5-wx", "Seedance2.5--480p/720p--wx--按次--W", []string{"480p", "720p"}, 30, 10, 10},
	} {
		tc := tc
		t.Run(tc.model, func(t *testing.T) {
			t.Parallel()

			spec, ok := miaowuModelSpecFor(tc.model)
			require.True(t, ok)
			require.Equal(t, 4, spec.MinSeconds)
			require.Equal(t, 30, spec.MaxSeconds)
			require.Equal(t, tc.resolutions, spec.Resolutions)
			require.Equal(t, "720p", spec.defaultResolution())
			require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, tc.model))

			// 720p is the default; the upstream gets the model, the length and the resolution.
			prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
				`{"model":"`+tc.model+`","prompt":"x","duration":30}`))
			require.NoError(t, err)
			require.JSONEq(t, `{"model":"`+tc.model+`","prompt":"x","seconds":30,"resolution":"720p"}`, string(prepared))
			require.Equal(t, tc.model, public.Model)

			head := `{"model":"` + tc.model + `","prompt":"x",`
			for _, c := range []struct{ name, body, want string }{
				{"1080p", head + `"duration":5,"resolution":"1080p"}`, "resolution must be " + strings.Join(tc.resolutions, ", ") + " for this model"},
				{"below the minimum", head + `"duration":3}`, "from 4 to 30 seconds"},
				{"above the maximum", head + `"duration":31}`, "from 4 to 30 seconds"},
				{"too many images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, tc.images+1) + `]}`, "at most " + strconv.Itoa(tc.images) + " reference images"},
				{"too many videos", head + `"duration":5,"video_urls":[` + repeatedJijiuItems(ref, tc.videos+1) + `]}`, "at most " + strconv.Itoa(tc.videos) + " reference videos"},
				{"too many audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, tc.audios+1) + `]}`, "at most " + strconv.Itoa(tc.audios) + " reference audios"},
			} {
				_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(c.body))
				require.ErrorContains(t, err, c.want, c.name)
			}
			full := head + `"duration":30,"image_urls":[` + repeatedJijiuItems(ref, tc.images) + `],"video_urls":[` +
				repeatedJijiuItems(ref, tc.videos) + `],"audio_urls":[` + repeatedJijiuItems(ref, tc.audios) + `]}`
			_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(full))
			require.NoError(t, err)

			c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, tc.model)
			require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, c.Duration)
			require.Equal(t, tc.resolutions, c.Resolutions)
			require.Equal(t, tc.images, *c.Assets.ReferenceImages)
			require.Equal(t, tc.videos, *c.Assets.ReferenceVideos)
			require.Equal(t, tc.audios, *c.Assets.ReferenceAudios)

			for _, resolution := range append([]string{""}, tc.resolutions...) {
				_, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, resolution, 10)
				require.True(t, ok, resolution)
			}
			_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, "1080p", 10)
			require.False(t, ok)

			// Served by the aggregate group as a per-request line. Hidden since the final table of
			// 2026-11-19 (these three are not in it), but still answered by name.
			line, found := SeedanceHubLineByName(tc.line)
			require.True(t, found)
			require.Equal(t, tc.model, line.Model)
			require.Equal(t, PlatformMiaowuSeedance, line.Platform)
			require.Equal(t, seedanceHubVersion25, line.Version)
			require.Equal(t, BillingModePerRequest, line.Billing)
			require.True(t, line.Hidden)
			_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + tc.line + `","prompt":"x","duration":5}`))
			require.NoError(t, err)
			require.Equal(t, tc.model, gjson.GetBytes(rewritten, "model").String())
		})
	}
}

// seedance-2.5-flash-s (2026-11-19): like flash, but billed per second (480p 0.4 / 720p 0.7 credits).
func TestMiaowuSeedanceFlashSModel(t *testing.T) {
	t.Parallel()

	const (
		model = "seedance-2.5-flash-s"
		line  = "Seedance2.5--480p/720p--flash-s--按秒--W"
		ref   = `"https://example.com/1.png"`
	)
	spec, ok := miaowuModelSpecFor(strings.ToUpper(model))
	require.True(t, ok)
	require.Equal(t, model, spec.Name)
	require.Equal(t, 4, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"480p", "720p"}, spec.Resolutions)
	require.Equal(t, "720p", spec.defaultResolution())
	require.Equal(t, 10, spec.MaxImages)
	require.Equal(t, 0, spec.MaxVideos)
	require.Equal(t, 5, spec.MaxAudios)
	// It is its own model: flash and flash-tj keep their own limits and lines.
	flash, ok := miaowuModelSpecFor("seedance-2.5-flash")
	require.True(t, ok)
	require.NotEqual(t, flash.Name, spec.Name)

	// 480p reaches the upstream and is billed as its own tier; 720p when none is named.
	prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+model+`","prompt":"x","duration":30,"resolution":"480p","images":[`+ref+`]}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"`+model+`","prompt":"x","seconds":30,"resolution":"480p","image_urls":[`+ref+`]}`, string(prepared))
	require.Equal(t, model, public.Model)
	meta := PPVideoBillingMetadataFromRequest(PlatformMiaowuSeedance, prepared)
	require.Equal(t, "480p", meta.VideoResolution)
	require.Equal(t, 30, meta.RequestedDurationSeconds)
	prepared, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+model+`","prompt":"x","duration":4}`))
	require.NoError(t, err)
	require.Equal(t, "720p", gjson.GetBytes(prepared, "resolution").String())

	head := `{"model":"` + model + `","prompt":"x",`
	for _, c := range []struct{ name, body, want string }{
		{"1080p", head + `"duration":5,"resolution":"1080p"}`, "resolution must be 480p, 720p for this model"},
		{"below the minimum", head + `"duration":3}`, "from 4 to 30 seconds"},
		{"above the maximum", head + `"duration":31}`, "from 4 to 30 seconds"},
		{"too many images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference images"},
		{"a reference video", head + `"duration":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
		{"too many audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, 6) + `]}`, "at most 5 reference audios"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(c.body))
		require.ErrorContains(t, err, c.want, c.name)
	}

	// Served by the aggregate group as a visible per-second line, under its own name.
	hubLine, found := SeedanceHubLineByName(line)
	require.True(t, found)
	require.Equal(t, model, hubLine.Model)
	require.Equal(t, PlatformMiaowuSeedance, hubLine.Platform)
	require.Equal(t, seedanceHubVersion25, hubLine.Version)
	require.Equal(t, BillingModeVideo, hubLine.Billing)
	require.False(t, hubLine.Hidden)
	byModel, found := SeedanceHubLineFor(PlatformMiaowuSeedance, model)
	require.True(t, found)
	require.Equal(t, line, byModel.Name)
	_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + line + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, model, gjson.GetBytes(rewritten, "model").String())
}

// doubao-seedance-2.0-fast and doubao-seedance-2.0-mini (2026-11-19): 720p only, 5-15 seconds, per
// request, 9 images and no reference videos or audios.
func TestMiaowuSeedanceDoubao20Models(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	for _, tc := range []struct{ model, line string }{
		{"doubao-seedance-2.0-fast", "Seedance2.0--720p--doubao-fast--按次--W"},
		{"doubao-seedance-2.0-mini", "Seedance2.0--720p--doubao-mini--按次--W"},
	} {
		tc := tc
		t.Run(tc.model, func(t *testing.T) {
			t.Parallel()

			spec, ok := miaowuModelSpecFor(strings.ToUpper(tc.model))
			require.True(t, ok)
			require.Equal(t, tc.model, spec.Name)
			require.Equal(t, 5, spec.MinSeconds)
			require.Equal(t, 15, spec.MaxSeconds)
			require.Equal(t, []string{"720p"}, spec.Resolutions)
			require.Equal(t, 9, spec.MaxImages)
			require.Equal(t, 0, spec.MaxVideos)
			require.Equal(t, 0, spec.MaxAudios)
			require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, tc.model))
			require.False(t, ppVideoModelKnownForeignToPlatform(PlatformMiaowuSeedance, tc.model))

			// The upstream gets the model, the length and the resolution (720p when none is named).
			prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
				`{"model":"`+tc.model+`","prompt":"x","duration":15,"images":[`+ref+`]}`))
			require.NoError(t, err)
			require.JSONEq(t, `{"model":"`+tc.model+`","prompt":"x","seconds":15,"resolution":"720p","image_urls":[`+ref+`]}`, string(prepared))
			require.Equal(t, tc.model, public.Model)
			require.Equal(t, "720p", PPVideoBillingMetadataFromRequest(PlatformMiaowuSeedance, prepared).VideoResolution)

			head := `{"model":"` + tc.model + `","prompt":"x",`
			for _, c := range []struct{ name, body, want string }{
				{"480p", head + `"duration":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
				{"1080p", head + `"duration":5,"resolution":"1080p"}`, "resolution must be 720p for this model"},
				{"below the minimum", head + `"duration":4}`, "from 5 to 15 seconds"},
				{"above the maximum", head + `"duration":16}`, "from 5 to 15 seconds"},
				{"too many images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
				{"a reference video", head + `"duration":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
				{"a reference audio", head + `"duration":5,"audio_urls":[` + ref + `]}`, "at most 0 reference audios"},
			} {
				_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(c.body))
				require.ErrorContains(t, err, c.want, c.name)
			}
			nine := head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 9) + `]}`
			_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(nine))
			require.NoError(t, err)

			c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, tc.model)
			require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 15}, c.Duration)
			require.Equal(t, []string{"720p"}, c.Resolutions)
			require.Equal(t, 9, *c.Assets.ReferenceImages)
			require.Equal(t, 0, *c.Assets.ReferenceVideos)
			require.Equal(t, 0, *c.Assets.ReferenceAudios)

			// Served by the aggregate group as a visible per-request line, under its own name.
			line, found := SeedanceHubLineByName(tc.line)
			require.True(t, found)
			require.Equal(t, tc.model, line.Model)
			require.Equal(t, PlatformMiaowuSeedance, line.Platform)
			require.Equal(t, seedanceHubVersion20, line.Version)
			require.Equal(t, BillingModePerRequest, line.Billing)
			require.False(t, line.Hidden)
			byModel, found := SeedanceHubLineFor(PlatformMiaowuSeedance, tc.model)
			require.True(t, found)
			require.Equal(t, tc.line, byModel.Name)
			// Only this platform serves the doubao- name.
			_, found = SeedanceHubLineFor(PlatformZexiSeedance, tc.model)
			require.False(t, found)
			_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + tc.line + `","prompt":"x","duration":5}`))
			require.NoError(t, err)
			require.Equal(t, tc.model, gjson.GetBytes(rewritten, "model").String())
		})
	}

	// The two are distinct from the older fast model, which keeps its own limits (480p / 720p, 3 audios).
	old, ok := miaowuModelSpecFor("seedance-2.0-fast")
	require.True(t, ok)
	require.Equal(t, []string{"480p", "720p"}, old.Resolutions)
	require.Equal(t, 3, old.MaxAudios)
}

// seedance-2.0-pro (2026-11-17) is the only miaowu model with 1080p and 4k, so it has its own test.
func TestMiaowuSeedance20ProModel(t *testing.T) {
	t.Parallel()

	const (
		model = "seedance-2.0-pro"
		line  = "Seedance2.0--720p/1080p/4k--pro--按秒--W"
		ref   = `"https://example.com/1.png"`
	)
	spec, ok := miaowuModelSpecFor(model)
	require.True(t, ok)
	require.Equal(t, []string{"720p", "1080p", "4k"}, spec.Resolutions)
	require.Equal(t, "720p", spec.defaultResolution())
	require.Equal(t, 9, spec.MaxImages)
	require.Equal(t, 3, spec.MaxVideos)
	require.Equal(t, 3, spec.MaxAudios)
	require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, model))

	head := `{"model":"` + model + `","prompt":"x",`

	// The resolution is forwarded as asked; 720p when omitted.
	for resolution, want := range map[string]string{"": "720p", "720p": "720p", "1080p": "1080p", "4k": "4k", "4K": "4k"} {
		body := head + `"duration":10`
		if resolution != "" {
			body += `,"resolution":"` + resolution + `"`
		}
		prepared, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(body+`}`))
		require.NoError(t, err, resolution)
		require.JSONEq(t, `{"model":"`+model+`","prompt":"x","seconds":10,"resolution":"`+want+`"}`, string(prepared), resolution)
	}

	// 480p and 2k are not offered; limits are the documented ones.
	for _, tc := range []struct{ name, body, want string }{
		{"480p", head + `"duration":5,"resolution":"480p"}`, "resolution must be 720p, 1080p, 4k for this model"},
		{"2k", head + `"duration":5,"resolution":"2k"}`, "resolution must be 720p, 1080p, 4k for this model"},
		{"ten images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
		{"four reference videos", head + `"duration":5,"video_urls":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference videos"},
		{"four audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference audios"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
	full := head + `"duration":15,"resolution":"4k","image_urls":[` + repeatedJijiuItems(ref, 9) + `],"video_urls":[` +
		repeatedJijiuItems(ref, 3) + `],"audio_urls":[` + repeatedJijiuItems(ref, 3) + `]}`
	_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(full))
	require.NoError(t, err)

	c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, model)
	require.Equal(t, []string{"720p", "1080p", "4k"}, c.Resolutions)
	require.Equal(t, 3, *c.Assets.ReferenceVideos)

	// The price estimate resolves each resolution to its own tier.
	for _, resolution := range []string{"", "720p", "1080p", "4k"} {
		_, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, model, resolution, 10)
		require.True(t, ok, resolution)
	}
	resolved, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, model, "4k", 10)
	require.True(t, ok)
	require.Equal(t, "4k", resolved)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, model, "480p", 10)
	require.False(t, ok)

	// The aggregate group serves it as a per-second line.
	hubLine, found := SeedanceHubLineByName(line)
	require.True(t, found)
	require.Equal(t, model, hubLine.Model)
	require.Equal(t, PlatformMiaowuSeedance, hubLine.Platform)
	require.Equal(t, BillingModeVideo, hubLine.Billing)
	_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + line + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, model, gjson.GetBytes(rewritten, "model").String())
}

func TestTranslatePPVideoUnifiedRequestToMiaowu(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformMiaowuSeedance, []byte(`{
		"model": "seedance-2.5-tj",
		"prompt": "以第一张图片为人物参考",
		"duration": 5,
		"resolution": "720p",
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "seedance-2.5-tj",
		"prompt": "以第一张图片为人物参考",
		"seconds": 5,
		"resolution": "720p",
		"ratio": "16:9",
		"image_urls": ["https://cdn.example.com/person.jpg"],
		"video_urls": ["https://cdn.example.com/motion.mp4"],
		"audio_urls": ["https://cdn.example.com/voice.mp3"]
	}`, string(prepared))

	for _, tc := range []struct{ name, body, want string }{
		{"no frames", `{"model":"seedance-2.5-slow","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
		{"no generate_audio", `{"model":"seedance-2.5-slow","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"prompt is required", `{"model":"seedance-2.5-slow","duration":5}`, "prompt"},
		{"duration is required", `{"model":"seedance-2.5-slow","prompt":"x"}`, "duration"},
		{"model is required", `{"prompt":"x","duration":5}`, "model"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformMiaowuSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

// A unified request must behave exactly like the equivalent native request.
func TestUnifiedMiaowuRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformMiaowuSeedance,
		`{
			"model": "seedance-2.5-tj",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"ratio": "9:16",
			"assets": [
				{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
				{"type": "video", "url": "https://cdn.example.com/run.mp4"},
				{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
			]
		}`,
		`{
			"model": "seedance-2.5-tj",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"ratio": "9:16",
			"images": ["https://cdn.example.com/cat.png"],
			"videos": ["https://cdn.example.com/run.mp4"],
			"audios": ["https://cdn.example.com/voice.mp3"]
		}`,
	)
}

func TestMiaowuSeedanceCapabilitiesQuoteAndHubLine(t *testing.T) {
	t.Parallel()

	c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, miaowuTestModel)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, c.Duration)
	require.Equal(t, []string{"480p", "720p"}, c.Resolutions)
	require.Equal(t, 30, *c.Assets.ReferenceImages)
	require.Equal(t, 0, *c.Assets.ReferenceVideos, "no reference videos since the final table of 2026-11-19")
	require.Equal(t, 10, *c.Assets.ReferenceAudios)
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, "seedance-2.5-cheap").Duration)

	// The price estimate enforces the same limits as the submission.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, miaowuTestModel, "", 10)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, miaowuTestModel, "480p", 10)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)

	// 480p is accepted, sent upstream and priced as its own tier; 720p stays the default.
	prepared480, public480, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-slow","prompt":"x","duration":6,"resolution":"480P"}`))
	require.NoError(t, err)
	require.Equal(t, "480p", gjson.GetBytes(prepared480, "resolution").String())
	require.Equal(t, "480p", public480.Resolution)
	require.Equal(t, "480p", PPVideoBillingMetadataFromRequest(PlatformMiaowuSeedance, prepared480).VideoResolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{miaowuTestModel, "", 3},
		{miaowuTestModel, "", 31},
		{miaowuTestModel, "1080p", 10},
		{"seedance-2.5-cheap", "", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}

	// The aggregate group serves the model as one line, answering to the upstream name too.
	const lineName = "Seedance2.5--480p/720p--特价满血--按秒--W"
	line, found := SeedanceHubLineByName(lineName)
	require.True(t, found)
	require.Equal(t, PlatformMiaowuSeedance, line.Platform)
	require.Equal(t, miaowuTestModel, line.Model)
	require.Equal(t, BillingModeVideo, line.Billing)
	byUpstream, found := SeedanceHubLineByName(miaowuTestModel)
	require.True(t, found)
	require.Equal(t, lineName, byUpstream.Name)
	require.Equal(t, PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, miaowuTestModel).Duration,
		PPVideoCapabilitiesForModel(PlatformSeedanceHub, lineName).Duration)

	_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + lineName + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, miaowuTestModel, gjson.GetBytes(rewritten, "model").String())

	// The earlier name keeps working and finds the same line.
	legacy, found := SeedanceHubLineByName("Seedance2.5--720p--slow--按秒--W")
	require.True(t, found)
	require.Equal(t, lineName, legacy.Name)
	_, legacyRewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"Seedance2.5--720p--slow--按秒--W","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, miaowuTestModel, gjson.GetBytes(legacyRewritten, "model").String())
	require.Equal(t, lineName, PPVideoPublicModelName(PlatformSeedanceHub, "Seedance2.5--720p--slow--按秒--W"))

	// Renamed 2026-11-18: the name used before that is not a line any more.
	_, found = SeedanceHubLineByName("Seedance2.5--480p/720p--slow--按秒--W")
	require.False(t, found)
}

// seedance-2.0-tj (updated 2026-11-21) and seedance-2.0-fast-tj / seedance-2.0-mini-tj (added the same
// day) share one spec: 480p / 720p / 1080p / 4k (720p when omitted), 5-15 seconds, 9 images / 3 reference
// videos / 3 audios, billed per request.
func TestMiaowuSeedance20TjModels(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	for _, tc := range []struct{ model, line string }{
		{"seedance-2.0-tj", "Seedance2.0--480p/720p/1080p/4k--tj--按次--W"},
		{"seedance-2.0-fast-tj", "Seedance2.0--480p/720p/1080p/4k--fast-tj--按次--W"},
		{"seedance-2.0-mini-tj", "Seedance2.0--480p/720p/1080p/4k--mini-tj--按次--W"},
	} {
		tc := tc
		t.Run(tc.model, func(t *testing.T) {
			t.Parallel()

			spec, ok := miaowuModelSpecFor(tc.model)
			require.True(t, ok)
			require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, spec.Resolutions)
			require.Equal(t, "720p", spec.defaultResolution())
			require.Equal(t, []int{5, 15}, []int{spec.MinSeconds, spec.MaxSeconds})
			require.Equal(t, []int{9, 3, 3}, []int{spec.MaxImages, spec.MaxVideos, spec.MaxAudios})
			require.True(t, ppVideoModelBelongsToPlatform(PlatformMiaowuSeedance, tc.model))

			head := `{"model":"` + tc.model + `","prompt":"x",`
			// 720p is the default; every listed resolution is forwarded as written (4K in lower case).
			prepared, public, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(head+`"duration":10}`))
			require.NoError(t, err)
			require.JSONEq(t, `{"model":"`+tc.model+`","prompt":"x","seconds":10,"resolution":"720p"}`, string(prepared))
			require.Equal(t, tc.model, public.Model)
			for _, resolution := range []string{"480p", "720p", "1080p", "4k"} {
				prepared, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(
					head+`"duration":5,"resolution":"`+strings.ToUpper(resolution)+`"}`))
				require.NoError(t, err, resolution)
				require.Equal(t, resolution, gjson.GetBytes(prepared, "resolution").String())
			}

			for _, c := range []struct{ name, body, want string }{
				{"2k", head + `"duration":5,"resolution":"2k"}`, "resolution must be 480p, 720p, 1080p, 4k for this model"},
				{"below the minimum", head + `"duration":4}`, "from 5 to 15 seconds"},
				{"above the maximum", head + `"duration":16}`, "from 5 to 15 seconds"},
				{"ten images", head + `"duration":5,"image_urls":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
				{"four reference videos", head + `"duration":5,"video_urls":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference videos"},
				{"four audios", head + `"duration":5,"audio_urls":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference audios"},
			} {
				_, _, err := PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(c.body))
				require.ErrorContains(t, err, c.want, c.name)
			}
			// The largest allowed reference set goes through.
			full := head + `"duration":15,"resolution":"4k","image_urls":[` + repeatedJijiuItems(ref, 9) + `],"video_urls":[` + repeatedJijiuItems(ref, 3) + `],"audio_urls":[` + repeatedJijiuItems(ref, 3) + `]}`
			_, _, err = PreparePPVideoRequestBody(PlatformMiaowuSeedance, PPVideoOperationGeneric, []byte(full))
			require.NoError(t, err)

			c := PPVideoCapabilitiesForModel(PlatformMiaowuSeedance, tc.model)
			require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 15}, c.Duration)
			require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, c.Resolutions)
			require.Equal(t, []int{9, 3, 3}, []int{*c.Assets.ReferenceImages, *c.Assets.ReferenceVideos, *c.Assets.ReferenceAudios})

			// The price estimate accepts every listed resolution and refuses the others.
			for _, resolution := range []string{"", "480p", "720p", "1080p", "4k"} {
				_, ok := normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, resolution, 10)
				require.True(t, ok, resolution)
			}
			_, ok = normalizeMediaQuoteVideoSpec(PlatformMiaowuSeedance, tc.model, "2k", 10)
			require.False(t, ok)

			// The aggregate line: visible, per request, and every name finds it.
			line, found := SeedanceHubLineByName(tc.line)
			require.True(t, found)
			require.Equal(t, tc.model, line.Model)
			require.Equal(t, BillingModePerRequest, line.Billing)
			require.False(t, line.Hidden)
			byModel, found := SeedanceHubLineByName(tc.model)
			require.True(t, found)
			require.Equal(t, tc.line, byModel.Name)
		})
	}

	// The tj line used to be named for 720p only; the old name still finds it.
	line, found := SeedanceHubLineByName("Seedance2.0--720p--tj--按次--W")
	require.True(t, found)
	require.Equal(t, "Seedance2.0--480p/720p/1080p/4k--tj--按次--W", line.Name)
}
