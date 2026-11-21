package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	xsnTestModel   = "seedance-2.5-特价满血"
	xsnTestHubLine = "Seedance2.5--720p--特价满血--按秒--N"

	xsnTestAllDayModel   = "seedance-2.5-全天打"
	xsnTestAllDayHubLine = "Seedance2.5--720p--全天打--按次--N"

	// The upstream names, and the public names clients see (they carry "Seedance").
	xsnTestJimengModel        = "即梦-Sd-2.5"
	xsnTestJimengFlashModel   = "即梦-Sd-2.5-flash"
	xsnTestJimengPublic       = "Seedance-即梦-Sd-2.5"
	xsnTestJimengFlashPublic  = "Seedance-即梦-Sd-2.5-flash"
	xsnTestJimengHubLine      ="Seedance2.5--480p/720p--即梦-Sd-2.5--按秒--N"
	xsnTestJimengFlashHubLine = "Seedance2.5--720p--即梦-Sd-2.5-flash--按次--N"
)

func TestXSNSeedanceJimengModels(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	const videoRef = `"https://example.com/1.mp4"`
	const audioRef = `"https://example.com/1.mp3"`

	// Table rows (names match ignoring case).
	spec, ok := xsnModelSpecFor("即梦-sd-2.5")
	require.True(t, ok)
	require.Equal(t, xsnTestJimengModel, spec.Name)
	require.Equal(t, []string{"480p", "720p"}, spec.Resolutions)
	require.Equal(t, "720p", spec.defaultResolution())
	require.Equal(t, 30, spec.MaxImages)
	require.Equal(t, 10, spec.MaxVideos)
	require.Equal(t, 10, spec.MaxAudios)
	flash, ok := xsnModelSpecFor("即梦-SD-2.5-FLASH")
	require.True(t, ok)
	require.Equal(t, xsnTestJimengFlashModel, flash.Name)
	require.Equal(t, []string{"720p"}, flash.Resolutions)
	require.Equal(t, 10, flash.MaxImages)
	require.Equal(t, 5, flash.MaxVideos)
	require.Equal(t, 5, flash.MaxAudios)
	// The shorter name is its own model, not a prefix match of the flash one.
	require.NotEqual(t, spec.Name, flash.Name)

	// Public names: clients that pick video models by name look for "Seedance". Either name finds
	// the model, the public one is listed first (so a channel price under it wins), and the
	// upstream name is the only one ever sent upstream.
	require.Equal(t, xsnTestJimengPublic, spec.publicName())
	require.Equal(t, xsnTestJimengFlashPublic, flash.publicName())
	byPublic, ok := xsnModelSpecFor("SEEDANCE-即梦-SD-2.5")
	require.True(t, ok)
	require.Equal(t, xsnTestJimengModel, byPublic.Name)
	byPublic, ok = xsnModelSpecFor(xsnTestJimengFlashPublic)
	require.True(t, ok)
	require.Equal(t, xsnTestJimengFlashModel, byPublic.Name)
	require.Equal(t, []string{xsnTestJimengPublic, xsnTestJimengModel}, ppVideoXSNModelAliases(xsnTestJimengModel))
	require.Equal(t, []string{xsnTestJimengFlashPublic, xsnTestJimengFlashModel}, ppVideoPricingModelAliases(PlatformXSNSeedance, xsnTestJimengFlashPublic))
	// The two models with "seedance" in their name keep a single name.
	require.Equal(t, []string{xsnTestModel}, ppVideoXSNModelAliases(xsnTestModel))
	require.Equal(t, []string{xsnTestAllDayModel}, ppVideoXSNModelAliases(xsnTestAllDayModel))

	// Model list of a single-platform group: a whitelist saved under either name is listed once,
	// under the public name.
	require.True(t, ppVideoModelBelongsToPlatform(PlatformXSNSeedance, xsnTestJimengPublic))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformXSNSeedance, xsnTestJimengFlashModel))
	require.Equal(t, []string{xsnTestJimengPublic, xsnTestJimengFlashPublic, xsnTestModel},
		filterPPVideoPublicModelsForPlatform(PlatformXSNSeedance,
			[]string{xsnTestJimengModel, xsnTestJimengPublic, xsnTestJimengFlashModel, xsnTestModel, "other"}))
	require.Equal(t, xsnTestJimengPublic, PPVideoPublicModelName(PlatformXSNSeedance, "即梦-SD-2.5"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformXSNSeedance, "other"))

	// A request under the public name is sent upstream under the upstream name, and answered
	// under the public name.
	prepared, public, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"Seedance-即梦-Sd-2.5-flash","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, xsnTestJimengFlashModel, gjson.GetBytes(prepared, "model").String())
	require.Equal(t, xsnTestJimengFlashPublic, public.Model)
	require.Equal(t, xsnTestJimengFlashModel, public.UpstreamModel)
	parsed, err := ParsePPVideoResponse(PlatformXSNSeedance, []byte(`{"id":"task_1","status":"completed"}`))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformXSNSeedance, []byte(`{"id":"task_1","status":"completed"}`), public, parsed)
	require.Equal(t, xsnTestJimengFlashPublic, gjson.GetBytes(normalized, "model").String())

	// Native requests: the resolution defaults to 720p, 480p is valid for Sd-2.5 only, every
	// documented ratio is accepted, and reference videos are forwarded as video_urls.
	for _, ratio := range xsnAspectRatios {
		prepared, public, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"即梦-Sd-2.5","prompt":"x","duration":5,"ratio":"`+ratio+`"}`))
		require.NoError(t, err, ratio)
		require.JSONEq(t, `{"model":"即梦-Sd-2.5","prompt":"x","seconds":5,"ratio":"`+ratio+`","resolution":"720p"}`, string(prepared))
		require.Equal(t, xsnTestJimengPublic, public.Model)
		require.Equal(t, xsnTestJimengModel, public.UpstreamModel)
		require.Equal(t, "720p", public.Resolution)
	}
	prepared, public, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"即梦-Sd-2.5","prompt":"x","duration":6,"resolution":"480p","image_urls":[`+ref+`],"video_urls":[`+videoRef+`],"audio_urls":[`+audioRef+`]}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"即梦-Sd-2.5","prompt":"x","seconds":6,"resolution":"480p",`+
		`"image_urls":[`+ref+`],"video_urls":[`+videoRef+`],"audio_urls":[`+audioRef+`]}`, string(prepared))
	require.Equal(t, xsnTestJimengPublic, public.Model)
	require.Equal(t, "480p", public.Resolution)
	require.True(t, public.HasImage)
	// Billing metadata carries the upstream name; the channel price is found under either name
	// (ppVideoPricingModelAliases), the public one first.
	meta := PPVideoBillingMetadataFromRequest(PlatformXSNSeedance, prepared)
	require.Equal(t, xsnTestJimengModel, meta.Model)
	require.Equal(t, "480p", meta.VideoResolution)
	require.Equal(t, int64(6000), meta.RequestedDurationMilliseconds)

	// Limits of each model.
	for _, tc := range []struct{ name, body, want string }{
		{"Sd-2.5 images", `{"model":"即梦-Sd-2.5","prompt":"x","seconds":5,"image_urls":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"Sd-2.5 videos", `{"model":"即梦-Sd-2.5","prompt":"x","seconds":5,"video_urls":[` + repeatedJijiuItems(videoRef, 11) + `]}`, "at most 10 reference videos"},
		{"Sd-2.5 audios", `{"model":"即梦-Sd-2.5","prompt":"x","seconds":5,"audio_urls":[` + repeatedJijiuItems(audioRef, 11) + `]}`, "at most 10 reference audios"},
		{"Sd-2.5 1080p", `{"model":"即梦-Sd-2.5","prompt":"x","seconds":5,"resolution":"1080p"}`, "resolution must be"},
		{"Sd-2.5 ratio", `{"model":"即梦-Sd-2.5","prompt":"x","seconds":5,"ratio":"5:4"}`, "ratio must be one of"},
		{"flash images", `{"model":"即梦-Sd-2.5-flash","prompt":"x","seconds":5,"image_urls":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference images"},
		{"flash videos", `{"model":"即梦-Sd-2.5-flash","prompt":"x","seconds":5,"video_urls":[` + repeatedJijiuItems(videoRef, 6) + `]}`, "at most 5 reference videos"},
		{"flash audios", `{"model":"即梦-Sd-2.5-flash","prompt":"x","seconds":5,"audio_urls":[` + repeatedJijiuItems(audioRef, 6) + `]}`, "at most 5 reference audios"},
		{"flash 480p", `{"model":"即梦-Sd-2.5-flash","prompt":"x","seconds":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"flash ratio", `{"model":"即梦-Sd-2.5-flash","prompt":"x","seconds":5,"ratio":"5:4"}`, "ratio must be one of"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
	// The upper limits are accepted.
	_, _, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"即梦-Sd-2.5","prompt":"x","seconds":30,"image_urls":[`+repeatedJijiuItems(ref, 30)+`],"video_urls":[`+
			repeatedJijiuItems(videoRef, 10)+`],"audio_urls":[`+repeatedJijiuItems(audioRef, 10)+`]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"即梦-Sd-2.5-flash","prompt":"x","seconds":30,"image_urls":[`+repeatedJijiuItems(ref, 10)+`],"video_urls":[`+
			repeatedJijiuItems(videoRef, 5)+`],"audio_urls":[`+repeatedJijiuItems(audioRef, 5)+`]}`))
	require.NoError(t, err)

	// Capabilities.
	capabilities := PPVideoCapabilitiesForModel(PlatformXSNSeedance, xsnTestJimengModel)
	require.Equal(t, []string{"480p", "720p"}, capabilities.Resolutions)
	require.Equal(t, xsnAspectRatios, capabilities.Ratios)
	require.Equal(t, 30, *capabilities.Assets.ReferenceImages)
	require.Equal(t, 10, *capabilities.Assets.ReferenceVideos)
	require.Equal(t, 10, *capabilities.Assets.ReferenceAudios)
	capabilities = PPVideoCapabilitiesForModel(PlatformXSNSeedance, xsnTestJimengFlashModel)
	require.Equal(t, []string{"720p"}, capabilities.Resolutions)
	require.Equal(t, 10, *capabilities.Assets.ReferenceImages)
	require.Equal(t, 5, *capabilities.Assets.ReferenceVideos)
	require.Equal(t, 5, *capabilities.Assets.ReferenceAudios)

	// Hub lines: Sd-2.5 per second, flash per request, each rewritten to the platform's public
	// name; the upstream name and the public name both find the line.
	for _, tc := range []struct {
		hubLine, public, upstream string
		billing                   BillingMode
	}{
		{xsnTestJimengHubLine, xsnTestJimengPublic, xsnTestJimengModel, BillingModeVideo},
		{xsnTestJimengFlashHubLine, xsnTestJimengFlashPublic, xsnTestJimengFlashModel, BillingModePerRequest},
	} {
		line, found := SeedanceHubLineByName(tc.hubLine)
		require.True(t, found, tc.hubLine)
		require.Equal(t, PlatformXSNSeedance, line.Platform)
		require.Equal(t, tc.public, line.Model)
		require.Equal(t, tc.billing, line.Billing)
		for _, name := range []string{tc.public, tc.upstream} {
			byModel, found := SeedanceHubLineByName(name)
			require.True(t, found, name)
			require.Equal(t, line.Name, byModel.Name)
		}
		_, body, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + tc.hubLine + `","prompt":"x","duration":5}`))
		require.NoError(t, err)
		require.Equal(t, tc.public, gjson.GetBytes(body, "model").String())
		// The platform pipeline takes the rewritten body and sends the upstream name.
		prepared, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, body)
		require.NoError(t, err)
		require.Equal(t, tc.upstream, gjson.GetBytes(prepared, "model").String())
	}

	// Quote spec.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, xsnTestJimengModel, "", 8)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, xsnTestJimengModel, "480p", 8)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, xsnTestJimengFlashModel, "480p", 8)
	require.False(t, ok)
}

func TestXSNSeedanceAllDayModel(t *testing.T) {
	t.Parallel()

	spec, ok := xsnModelSpecFor("SEEDANCE-2.5-全天打")
	require.True(t, ok)
	require.Equal(t, xsnTestAllDayModel, spec.Name)
	require.Empty(t, spec.PublicName)
	require.Equal(t, 5, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"720p"}, spec.Resolutions)
	require.Equal(t, 30, spec.MaxImages)
	require.Equal(t, 10, spec.MaxAudios)
	require.Equal(t, 0, spec.MaxVideos)
	require.False(t, spec.FirstLastFrame)
	require.True(t, ppVideoModelBelongsToPlatform(PlatformXSNSeedance, xsnTestAllDayModel))

	// Native request: every documented ratio is accepted, the resolution is filled in.
	for _, ratio := range xsnAspectRatios {
		prepared, public, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"SEEDANCE-2.5-全天打","prompt":"x","duration":5,"ratio":"`+ratio+`"}`))
		require.NoError(t, err, ratio)
		require.JSONEq(t, `{"model":"seedance-2.5-全天打","prompt":"x","seconds":5,"ratio":"`+ratio+`","resolution":"720p"}`, string(prepared))
		require.Equal(t, xsnTestAllDayModel, public.Model)
		require.Equal(t, int64(5000), public.DurationMilliseconds)
	}

	// The limits: 5 to 30 seconds (4 seconds is valid for the other model only), 30 images, 10 audios.
	const ref = `"https://example.com/1.png"`
	base := `{"model":"seedance-2.5-全天打","prompt":"x",`
	for _, tc := range []struct{ name, body, want string }{
		{"below the minimum", base + `"seconds":4}`, "from 5 to 30 seconds"},
		{"above the maximum", base + `"seconds":31}`, "from 5 to 30 seconds"},
		{"another resolution", base + `"seconds":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"unknown ratio", base + `"seconds":5,"ratio":"5:4"}`, "ratio must be one of"},
		{"reference videos", base + `"seconds":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
		{"too many images", base + `"seconds":5,"image_urls":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many audios", base + `"seconds":5,"audio_urls":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
	_, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		base+`"seconds":30,"ratio":"21:9","image_urls":[`+repeatedJijiuItems(ref, 30)+`],"audio_urls":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)

	// The other model still takes 4 seconds.
	_, _, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-特价满血","prompt":"x","seconds":4}`))
	require.NoError(t, err)

	// Capabilities.
	capabilities := PPVideoCapabilitiesForModel(PlatformXSNSeedance, xsnTestAllDayModel)
	require.NotNil(t, capabilities.Duration)
	require.Equal(t, 5, capabilities.Duration.Min)
	require.Equal(t, 30, capabilities.Duration.Max)
	require.Equal(t, []string{"720p"}, capabilities.Resolutions)
	require.Equal(t, xsnAspectRatios, capabilities.Ratios)
	require.Equal(t, 30, *capabilities.Assets.ReferenceImages)
	require.Equal(t, 0, *capabilities.Assets.ReferenceVideos)
	require.Equal(t, 10, *capabilities.Assets.ReferenceAudios)

	// Hub line: billed per request, rewritten to the platform's own name.
	line, found := SeedanceHubLineByName(xsnTestAllDayHubLine)
	require.True(t, found)
	require.Equal(t, PlatformXSNSeedance, line.Platform)
	require.Equal(t, xsnTestAllDayModel, line.Model)
	require.Equal(t, BillingModePerRequest, line.Billing)
	byModel, found := SeedanceHubLineByName(xsnTestAllDayModel)
	require.True(t, found)
	require.Equal(t, line.Name, byModel.Name)
	_, body, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + xsnTestAllDayHubLine + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, xsnTestAllDayModel, gjson.GetBytes(body, "model").String())

	// Quote spec: 5 to 30 seconds, 720p.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, xsnTestAllDayModel, "", 5)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	for _, seconds := range []int{4, 31} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, xsnTestAllDayModel, "", seconds)
		require.False(t, ok, seconds)
	}
}

func TestXSNSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, xsnSeedanceModels, 4)
	spec, ok := xsnModelSpecFor("SEEDANCE-2.5-特价满血")
	require.True(t, ok)
	require.Equal(t, xsnTestModel, spec.Name)
	require.Empty(t, spec.PublicName)
	require.Equal(t, 4, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"720p"}, spec.Resolutions)
	require.True(t, spec.ForwardResolution)
	require.Equal(t, 30, spec.MaxImages)
	require.Equal(t, 10, spec.MaxAudios)
	// The upstream doubles its price for reference videos; they are refused until a surcharge exists.
	require.Equal(t, 0, spec.MaxVideos)
	require.False(t, spec.FirstLastFrame)
	require.Equal(t, []string{xsnTestModel}, ppVideoXSNModelAliases("seedance-2.5-特价满血"))
	require.Equal(t, []string{xsnTestModel}, ppVideoPricingModelAliases(PlatformXSNSeedance, "SEEDANCE-2.5-特价满血"))

	for _, model := range []string{"", "seedance-2.5", "seedance-2.0-特价满血", "other"} {
		_, ok = xsnModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestXSNSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformXSNSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformXSNSeedance))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformXSNSeedance, xsnTestModel))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformXSNSeedance, "other"))
	require.Equal(t, []string{xsnTestModel}, filterPPVideoPublicModelsForPlatform(PlatformXSNSeedance, []string{xsnTestModel, "other"}))
	require.Equal(t, xsnTestModel, PPVideoPublicModelName(PlatformXSNSeedance, "SEEDANCE-2.5-特价满血"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformXSNSeedance, "other"))
}

// Account sync answers from the local table without any HTTP request (the service has no
// upstream client at all here).
func TestXSNSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformXSNSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	// The jimeng models are synced under their public names.
	require.Equal(t, []string{xsnTestModel, xsnTestAllDayModel, xsnTestJimengPublic, xsnTestJimengFlashPublic}, models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformXSNSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestXSNSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformXSNSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformXSNSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformXSNSeedance, "xsn-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer xsn-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformXSNSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformXSNSeedance))

	// Documented flow: queued / in_progress -> completed / failed.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","object":"video","status":"queued"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed","progress":100}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"x","message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformXSNSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	failed, err := ParsePPVideoResponse(PlatformXSNSeedance, []byte(`{"id":"task_1","status":"failed","error":{"code":"x","message":"content rejected"}}`))
	require.NoError(t, err)
	require.Equal(t, "content rejected", failed.ErrorMessage)
	rejected, err := ParsePPVideoResponse(PlatformXSNSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", rejected.ErrorMessage)

	// Billing uses the requested seconds, never the length the upstream echoes.
	echoed, err := ParsePPVideoResponse(PlatformXSNSeedance, []byte(`{"id":"task_1","status":"completed","seconds":"9"}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
}

func TestXSNSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: "SEEDANCE-2.5-特价满血"}

	raw := `{"id":"task_1","object":"video","status":"completed","progress":100,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err := ParsePPVideoResponse(PlatformXSNSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformXSNSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, xsnTestModel, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "url").String())
	require.NotContains(t, string(normalized), "cdn.example.com")

	// A completed task without any address still gets this gateway's content address.
	bare := `{"id":"task_1","object":"video","status":"completed","progress":100}`
	parsed, err = ParsePPVideoResponse(PlatformXSNSeedance, []byte(bare))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformXSNSeedance, []byte(bare), public, parsed)
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())

	// A running task never shows an address, even when the upstream already carries one.
	running := `{"id":"task_1","status":"in_progress","progress":30,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformXSNSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformXSNSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")
}

func TestXSNSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "text to video, the documented example",
			body: `{"model":"seedance-2.5-特价满血","prompt":"海边日出，镜头缓慢推进","seconds":"8","ratio":"16:9","resolution":"720p"}`,
			want: `{"model":"seedance-2.5-特价满血","prompt":"海边日出，镜头缓慢推进","seconds":8,"ratio":"16:9","resolution":"720p"}`,
		},
		{
			name: "the resolution is filled in and the ratio is optional",
			body: `{"model":"seedance-2.5-特价满血","prompt":"x","duration":5}`,
			want: `{"model":"seedance-2.5-特价满血","prompt":"x","seconds":5,"resolution":"720p"}`,
		},
		{
			name: "the upstream's own media fields",
			body: `{"model":"seedance-2.5-特价满血","prompt":"x","seconds":10,"ratio":"9:16",` +
				`"image_urls":["https://example.com/1.png","https://example.com/2.png"],"audio_urls":["https://example.com/1.mp3"]}`,
			want: `{"model":"seedance-2.5-特价满血","prompt":"x","seconds":10,"ratio":"9:16","resolution":"720p",` +
				`"image_urls":["https://example.com/1.png","https://example.com/2.png"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "aliases are renamed to the upstream's fields",
			body: `{"model":"seedance-2.5-特价满血","prompt":"x","duration":4,"images":["https://example.com/1.png"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"seedance-2.5-特价满血","prompt":"x","seconds":4,"resolution":"720p",` +
				`"image_urls":["https://example.com/1.png"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the model name is matched ignoring case and answered with the table's spelling",
			body: `{"model":"SEEDANCE-2.5-特价满血","prompt":"x","duration":30,"ratio":"21:9"}`,
			want: `{"model":"seedance-2.5-特价满血","prompt":"x","seconds":30,"ratio":"21:9","resolution":"720p"}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestXSNSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	prepared, public, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-特价满血","prompt":"x","duration":12,"image_urls":["https://example.com/1.png"]}`))
	require.NoError(t, err)
	require.Equal(t, xsnTestModel, public.Model)
	require.Equal(t, xsnTestModel, public.UpstreamModel)
	require.Equal(t, int64(12000), public.DurationMilliseconds)
	require.Equal(t, 1, public.VideoCount)
	require.Equal(t, "720p", public.Resolution)
	require.True(t, public.HasImage)

	meta := PPVideoBillingMetadataFromRequest(PlatformXSNSeedance, prepared)
	require.Equal(t, PlatformXSNSeedance, meta.Platform)
	require.Equal(t, xsnTestModel, meta.Model)
	require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
	require.Equal(t, "720p", meta.VideoResolution)
}

func TestXSNSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	base := `{"model":"seedance-2.5-特价满血","prompt":"x",`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","seconds":5}`, "model is required"},
		// Names that start with "seedance" belong to the legacy Seedance platform, so the shared
		// foreign-model check answers first; any other unknown name reaches the normalizer.
		{"another seedance model", `{"model":"seedance-2.5","prompt":"x","seconds":5}`, "is not supported by platform"},
		{"unknown model", `{"model":"other-model","prompt":"x","seconds":5}`, "is not available"},
		{"missing prompt", `{"model":"seedance-2.5-特价满血","seconds":5}`, "prompt is required"},
		{"missing duration", base + `"ratio":"16:9"}`, "duration is required"},
		{"below the minimum", base + `"seconds":3}`, "from 4 to 30 seconds"},
		{"above the maximum", base + `"seconds":31}`, "from 4 to 30 seconds"},
		{"another resolution", base + `"seconds":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"unknown ratio", base + `"seconds":5,"ratio":"5:4"}`, "ratio must be one of"},
		{"reference videos, the upstream's name", base + `"seconds":5,"video_urls":[` + ref + `]}`, "does not support reference videos"},
		{"reference videos, an alias", base + `"seconds":5,"videos":[` + ref + `]}`, "does not support reference videos"},
		{"reference videos, another alias", base + `"seconds":5,"reference_videos":[` + ref + `]}`, "does not support reference videos"},
		{"first frame", base + `"seconds":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"seed", base + `"seconds":5,"seed":1}`, "does not support seed"},
		{"generate_audio", base + `"seconds":5,"generate_audio":true}`, "does not support generate_audio"},
		{"watermark", base + `"seconds":5,"watermark":false}`, "does not support watermark"},
		{"assets on v1", base + `"seconds":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"more than one video", base + `"seconds":5,"n":2}`, "one video per request"},
		{"too many images", base + `"seconds":5,"image_urls":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many audios", base + `"seconds":5,"audio_urls":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
		{"two names for the same images", base + `"seconds":5,"image_urls":[` + ref + `],"images":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", base + `"seconds":5,"image_urls":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"media on a private address", base + `"seconds":5,"image_urls":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The upper limits themselves are accepted.
	_, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		base+`"seconds":30,"ratio":"21:9","image_urls":[`+repeatedJijiuItems(ref, 30)+`],"audio_urls":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)
}

func TestTranslatePPVideoUnifiedRequestToXSN(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformXSNSeedance, []byte(`{
		"model": "seedance-2.5-特价满血",
		"prompt": "@Image1 在街道上与@Image2 对话",
		"duration": 5,
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "image", "url": "https://cdn.example.com/other.jpg"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "seedance-2.5-特价满血",
		"prompt": "@Image1 在街道上与@Image2 对话",
		"seconds": 5,
		"ratio": "16:9",
		"resolution": "720p",
		"image_urls": ["https://cdn.example.com/person.jpg", "https://cdn.example.com/other.jpg"],
		"audio_urls": ["https://cdn.example.com/voice.mp3"]
	}`, string(prepared))

	// A reference video is translated and then refused by the model limits.
	translated, err = TranslatePPVideoUnifiedRequest(PlatformXSNSeedance, []byte(`{
		"model": "seedance-2.5-特价满血", "prompt": "x", "duration": 5,
		"assets": [{"type": "video", "url": "https://cdn.example.com/motion.mp4"}]
	}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, translated)
	require.ErrorContains(t, err, "does not support reference videos")

	// Fields this platform does not take are refused by the translator.
	for _, tc := range []struct{ name, body, want string }{
		{"missing model", `{"prompt":"x","duration":5}`, "model"},
		{"missing prompt", `{"model":"seedance-2.5-特价满血","duration":5}`, "prompt"},
		{"missing duration", `{"model":"seedance-2.5-特价满血","prompt":"x"}`, "duration"},
		{"generate_audio", `{"model":"seedance-2.5-特价满血","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"seed", `{"model":"seedance-2.5-特价满血","prompt":"x","duration":5,"seed":1}`, "seed"},
		{"first frame", `{"model":"seedance-2.5-特价满血","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
	} {
		_, err = TranslatePPVideoUnifiedRequest(PlatformXSNSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

func TestXSNSeedanceCapabilities(t *testing.T) {
	t.Parallel()

	capabilities := PPVideoCapabilitiesForModel(PlatformXSNSeedance, xsnTestModel)
	require.Contains(t, capabilities.Protocols, PPVideoProtocolV1)
	require.Contains(t, capabilities.Protocols, PPVideoProtocolV2)
	require.NotNil(t, capabilities.Duration)
	require.Equal(t, 4, capabilities.Duration.Min)
	require.Equal(t, 30, capabilities.Duration.Max)
	require.Equal(t, []string{"720p"}, capabilities.Resolutions)
	require.Equal(t, xsnAspectRatios, capabilities.Ratios)
	require.NotNil(t, capabilities.Assets)
	require.Equal(t, 30, *capabilities.Assets.ReferenceImages)
	require.Equal(t, 0, *capabilities.Assets.ReferenceVideos)
	require.Equal(t, 10, *capabilities.Assets.ReferenceAudios)
	require.False(t, *capabilities.Assets.FirstLastFrame)
}

func TestXSNSeedanceHubLine(t *testing.T) {
	t.Parallel()

	line, ok := SeedanceHubLineByName(xsnTestHubLine)
	require.True(t, ok)
	require.Equal(t, PlatformXSNSeedance, line.Platform)
	require.Equal(t, xsnTestModel, line.Model)
	require.Equal(t, BillingModeVideo, line.Billing)
	require.True(t, IsSeedanceHubMemberPlatform(PlatformXSNSeedance))

	// The platform's own name keeps working inside the hub group.
	byModel, ok := SeedanceHubLineByName(xsnTestModel)
	require.True(t, ok)
	require.Equal(t, line.Name, byModel.Name)

	resolved, body, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + xsnTestHubLine + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, line.Name, resolved.Name)
	require.Equal(t, xsnTestModel, gjson.GetBytes(body, "model").String())

	// The estimate for the line uses the platform's table.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, xsnTestHubLine, "", 8)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, xsnTestHubLine, "", 31)
	require.False(t, ok)
}

func TestXSNSeedanceQuoteSpec(t *testing.T) {
	t.Parallel()

	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, xsnTestModel, "", 8)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, xsnTestModel, "720p", 30)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)

	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{xsnTestModel, "480p", 8},
		{xsnTestModel, "", 3},
		{xsnTestModel, "", 31},
		{"other", "", 8},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformXSNSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%s %s %d", tc.model, tc.resolution, tc.seconds)
	}
}
