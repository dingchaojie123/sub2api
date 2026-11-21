package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHuahuoSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, huahuoSeedanceModels, 4)
	seen := make(map[string]string)
	for _, spec := range huahuoSeedanceModels {
		// A row answers to its upstream name, that name without the vendor path (the operator's own
		// spelling) and its public name; no name may belong to two rows.
		for _, name := range []string{spec.Name, huahuoShortName(spec.Name), spec.publicName()} {
			key := strings.ToLower(name)
			owner, dup := seen[key]
			require.False(t, dup, "name %q is used by %q and %q", name, owner, spec.Name)
			seen[key] = spec.Name
		}
		// Clients pick video models by name: every public name has to say seedance ...
		require.Contains(t, strings.ToLower(spec.publicName()), "seedance", spec.Name)
		// ... and none of them carries the upstream's vendor path or name.
		require.NotContains(t, strings.ToLower(spec.publicName()), "huahuo", spec.Name)
		require.NotContains(t, strings.ToLower(spec.publicName()), "inmind", spec.Name)
		// Every upstream name carries the vendor path (the supplier's own page shows
		// inmind/video/video-2.5shuliang1 too).
		require.True(t, strings.HasPrefix(spec.Name, huahuoVendorPath), spec.Name)
		require.NotEqual(t, spec.Name, huahuoShortName(spec.Name), spec.Name)
		require.NotEmpty(t, spec.Resolutions, spec.Name)
		require.Contains(t, spec.Resolutions, spec.defaultResolution(), spec.Name)
		// size carries a ratio, so the resolution must always be sent: with it left out, the billing
		// metadata would read the ratio as the resolution.
		require.True(t, spec.ForwardResolution, spec.Name)
		require.False(t, spec.FirstLastFrame, spec.Name)

		// The upstream's own field names.
		require.Equal(t, "input_reference", spec.imageField(), spec.Name)
		require.Equal(t, "video_reference", spec.videoField(), spec.Name)
		require.Equal(t, "audio_references", spec.audioField(), spec.Name)
		require.Equal(t, "size", spec.ratioField(), spec.Name)

		// The assumed limits (see huahuoSeedanceModels). The two 满参稳定 models take no reference
		// videos (the operator's final table of 2026-11-19); the other two take 10.
		wantMin := 4
		if spec.Name == huahuoTestShuliang {
			wantMin = 5
		}
		wantVideos := 10
		if strings.HasSuffix(spec.Name, "mancananmiao") || strings.HasSuffix(spec.Name, "mancananci") {
			wantVideos = 0
		}
		require.Equal(t, wantMin, spec.MinSeconds, spec.Name)
		require.Equal(t, 30, spec.MaxSeconds, spec.Name)
		require.Equal(t, 30, spec.MaxImages, spec.Name)
		require.Equal(t, wantVideos, spec.MaxVideos, spec.Name)
		require.Equal(t, 10, spec.MaxAudios, spec.Name)
	}

	flash, ok := huahuoModelSpecFor("INMIND/video/seedance2-5-flash")
	require.True(t, ok)
	require.Equal(t, "inmind/video/seedance2-5-flash", flash.Name)
	require.Equal(t, "Seedance2.5-mancan-flash", flash.publicName())
	require.Equal(t, []string{"480p", "720p"}, flash.Resolutions)

	// Public name first, so a channel price configured under it wins; the upstream name and its
	// short form (the operator's table) still work.
	require.Equal(t, []string{"Seedance2.5-mancan-flash", "inmind/video/seedance2-5-flash", "seedance2-5-flash"}, ppVideoHuahuoModelAliases("inmind/video/seedance2-5-flash"))
	require.Equal(t, []string{"Seedance2.5-mancan-anci", "inmind/video/seedance2-5-mancananci", "seedance2-5-mancananci"}, ppVideoPricingModelAliases(PlatformHuahuoSeedance, "seedance2.5-MANCAN-anci"))
	for _, short := range []string{"seedance2-5-mancananmiao", "SEEDANCE2-5-MANCANANCI", "seedance2-5-flash", "video-2.5shuliang1"} {
		spec, found := huahuoModelSpecFor(short)
		require.True(t, found, short)
		require.Equal(t, huahuoVendorPath+strings.ToLower(short), strings.ToLower(spec.Name), short)
	}

	// Not offered: the upstream's other models and other model families.
	for _, model := range []string{"", "inmind/video/seedance-2-5-3", "inmind/video/tesusd", "seedance-2.0-930", "wan3.0-video"} {
		_, ok = huahuoModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestHuahuoSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformHuahuoSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformHuahuoSeedance))
	require.True(t, IsAllowedQuotaPlatform(PlatformHuahuoSeedance))
	require.True(t, IsSeedanceHubMemberPlatform(PlatformHuahuoSeedance))
	require.True(t, PPVideoGroupServesPlatform(PlatformSeedanceHub, PlatformHuahuoSeedance))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformHuahuoSeedance, "inmind/video/seedance2-5-mancananmiao"))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformHuahuoSeedance, "Seedance2.5-mancan-flash"))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformHuahuoSeedance, "other"))
	// Another platform's model is foreign to this one, and this platform's model to the others.
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformJijiuSeedance, "Seedance2.5-mancan-anci"))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformHuahuoSeedance, "Seedance2.5-vs-L"))

	// A whitelist saved under either name is listed once, under the public name.
	require.Equal(t, []string{"Seedance2.5-mancan-anmiao", "Seedance2.5-mancan-flash"},
		filterPPVideoPublicModelsForPlatform(PlatformHuahuoSeedance, []string{
			"inmind/video/seedance2-5-mancananmiao", "Seedance2.5-mancan-anmiao", "inmind/video/seedance2-5-flash", "other",
		}))
	require.Equal(t, "Seedance2.5-mancan-flash", PPVideoPublicModelName(PlatformHuahuoSeedance, "inmind/video/seedance2-5-flash"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformHuahuoSeedance, "other"))
}

// Account sync answers from the local table without any HTTP request.
func TestHuahuoSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformHuahuoSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, huahuoModelNames(), models)
	require.Len(t, models, 4)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformHuahuoSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestHuahuoSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	// The OpenAI-style collection, the same as the other Seedance platforms of this kind.
	createPath, err := PPVideoUpstreamPath(PlatformHuahuoSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformHuahuoSeedance, PPVideoOperationGeneric, "task_1")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task_1", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformHuahuoSeedance, "huahuo-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer huahuo-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformHuahuoSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformHuahuoSeedance))

	// Waiting states, including the three the upstream calls "still being confirmed", never count as
	// a failure; only completed and failed end a task.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","object":"video","status":"queued","progress":0}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"processing"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"submitting"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"reserved"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"review_required"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed","progress":100}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"x","message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformHuahuoSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	failed, err := ParsePPVideoResponse(PlatformHuahuoSeedance, []byte(`{"id":"task_1","status":"failed","error":{"code":"content_policy","message":"素材不合规"}}`))
	require.NoError(t, err)
	require.Equal(t, "素材不合规", failed.ErrorMessage)

	// Billing uses the requested seconds, never the length the upstream echoes.
	echoed, err := ParsePPVideoResponse(PlatformHuahuoSeedance, []byte(`{"id":"task_1","status":"completed","seconds":"9","credit_amount":1.5}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
}

func TestHuahuoSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: "inmind/video/seedance2-5-mancananmiao"}

	raw := `{"id":"task_1","task_id":"task_1","object":"video","model":"inmind/video/seedance2-5-mancananmiao","status":"completed","progress":100,` +
		`"progress_source":"confirmed","stage":"completed","credit_state":"settled","credit_amount":12.5,` +
		`"video_url":"https://huahuo.ccwu.cc/v1/videos/task_1/content#task_id=abc","usage":{"total_tokens":1}}`
	parsed, err := ParsePPVideoResponse(PlatformHuahuoSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformHuahuoSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	// The client sees the public name, this gateway's own address, and nothing of the upstream's.
	require.Equal(t, "Seedance2.5-mancan-anmiao", gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	require.NotContains(t, string(normalized), "huahuo")
	require.NotContains(t, string(normalized), "inmind")
	// The credits the upstream charged this gateway are the supplier's price, not the client's.
	require.False(t, gjson.GetBytes(normalized, "credit_amount").Exists())
	require.False(t, gjson.GetBytes(normalized, "credit_state").Exists())

	// A running task never shows an address, even when the upstream already carries one.
	running := `{"id":"task_1","status":"in_progress","progress":30,"video_url":"https://huahuo.ccwu.cc/x.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformHuahuoSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformHuahuoSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "huahuo")
}

func TestHuahuoSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	prepare := func(body string) (map[string]any, error) {
		prepared, _, err := PreparePPVideoRequestBody(PlatformHuahuoSeedance, PPVideoOperationGeneric, []byte(body))
		if err != nil {
			return nil, err
		}
		return gjson.ParseBytes(prepared).Value().(map[string]any), nil
	}

	// The upstream receives its own name, whichever name was used, and its own field names; the
	// length, the ratio and the resolution go out as strings.
	payload, err := prepare(`{"model":"Seedance2.5-mancan-flash","prompt":"x","duration":15,"resolution":"720p","ratio":"9:16",` +
		`"images":["https://example.com/1.png"],"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3","https://example.com/2.mp3"]}`)
	require.NoError(t, err)
	require.Equal(t, "inmind/video/seedance2-5-flash", payload["model"])
	require.Equal(t, "15", payload["seconds"])
	require.Equal(t, "720p", payload["resolution"])
	require.Equal(t, "9:16", payload["size"])
	require.Equal(t, []any{"https://example.com/1.png"}, payload["input_reference"])
	require.Equal(t, []any{"https://example.com/1.mp4"}, payload["video_reference"])
	require.Equal(t, []any{"https://example.com/1.mp3", "https://example.com/2.mp3"}, payload["audio_references"])
	// Nothing of this gateway's own spellings reaches the upstream.
	for _, key := range []string{"duration", "ratio", "aspect_ratio", "images", "videos", "audios", "image_urls"} {
		require.NotContains(t, payload, key)
	}

	// Either name is accepted; the upstream name is the one sent; seconds may be a numeric string.
	payload, err = prepare(`{"model":"inmind/video/seedance2-5-mancananci","prompt":"x","seconds":"30"}`)
	require.NoError(t, err)
	require.Equal(t, "inmind/video/seedance2-5-mancananci", payload["model"])
	require.Equal(t, "30", payload["seconds"])
	// The resolution is filled in when omitted, and no ratio is invented.
	require.Equal(t, "720p", payload["resolution"])
	require.NotContains(t, payload, "size")

	// flash takes 480p as well.
	payload, err = prepare(`{"model":"Seedance2.5-mancan-flash","prompt":"x","seconds":4,"resolution":"480p"}`)
	require.NoError(t, err)
	require.Equal(t, "480p", payload["resolution"])
	require.Equal(t, "4", payload["seconds"])

	// 30 images, 10 videos and 10 audios are the limits of flash (the two 满参稳定 models take no
	// reference videos since the final table of 2026-11-19).
	items := func(n int, kind, ext string) string {
		parts := make([]string, 0, n)
		for i := 0; i < n; i++ {
			parts = append(parts, fmt.Sprintf(`"https://example.com/%s%d.%s"`, kind, i, ext))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	_, err = prepare(`{"model":"Seedance2.5-mancan-flash","prompt":"x","seconds":30,"images":` + items(30, "i", "png") +
		`,"videos":` + items(10, "v", "mp4") + `,"audios":` + items(10, "a", "mp3") + `}`)
	require.NoError(t, err)
	_, err = prepare(`{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":30,"images":` + items(30, "i", "png") +
		`,"audios":` + items(10, "a", "mp3") + `}`)
	require.NoError(t, err)

	for name, body := range map[string]string{
		"shorter than 4 seconds":         `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":3}`,
		"longer than 30 seconds":         `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":31}`,
		"length is required":             `{"model":"Seedance2.5-mancan-anmiao","prompt":"x"}`,
		"fractional length":              `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":"5.5"}`,
		"prompt is required":             `{"model":"Seedance2.5-mancan-anmiao","seconds":5}`,
		"1080p is not offered":           `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"resolution":"1080p"}`,
		"the 720p models refuse 480p":    `{"model":"Seedance2.5-mancan-anci","prompt":"x","seconds":5,"resolution":"480p"}`,
		"flash has no 1080p":             `{"model":"Seedance2.5-mancan-flash","prompt":"x","seconds":5,"resolution":"1080p"}`,
		"ratio outside the accepted set": `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"ratio":"5:4"}`,
		"adaptive is not offered":        `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"ratio":"adaptive"}`,
		"31 images":                      `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"images":` + items(31, "i", "png") + `}`,
		"11 videos":                      `{"model":"Seedance2.5-mancan-flash","prompt":"x","seconds":5,"videos":` + items(11, "v", "mp4") + `}`,
		"a video on 满参稳定 (per second)": `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"videos":` + items(1, "v", "mp4") + `}`,
		"a video on 满参稳定 (per request)": `{"model":"Seedance2.5-mancan-anci","prompt":"x","seconds":5,"videos":` + items(1, "v", "mp4") + `}`,
		"11 audios":                      `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"audios":` + items(11, "a", "mp3") + `}`,
		"reference over http":            `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"images":["http://example.com/1.png"]}`,
		"first frame":                    `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"first_frame":"https://example.com/1.png"}`,
		"seed is not passed":             `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"seed":1}`,
		"audio switch is not passed":     `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"generate_audio":true}`,
		"more than one video":            `{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,"n":2}`,
		"unknown model":                  `{"model":"other","prompt":"x","seconds":5}`,
		"another model of the upstream":  `{"model":"inmind/video/seedance-2-5-3","prompt":"x","seconds":5}`,
	} {
		_, err := prepare(body)
		require.Error(t, err, name)
	}

	// The upstream's own spellings are refused rather than ignored, so media is never dropped.
	for _, field := range []string{
		`"input_reference":["https://example.com/1.png"]`,
		`"input_images":["https://example.com/1.png"]`,
		`"input_videos":["https://example.com/1.mp4"]`,
		`"input_audios":["https://example.com/1.mp3"]`,
		`"video_reference":["https://example.com/1.mp4"]`,
		`"audio_references":["https://example.com/1.mp3"]`,
		`"start_frame":"https://example.com/1.png"`,
		`"end_frame":"https://example.com/2.png"`,
		`"media":[]`,
		`"content":[{"type":"text","text":"x"}]`,
	} {
		_, err := prepare(`{"model":"Seedance2.5-mancan-anmiao","prompt":"x","seconds":5,` + field + `}`)
		require.Error(t, err, field)
	}
}

func TestHuahuoSeedanceUnifiedRequestMatchesNative(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformHuahuoSeedance,
		`{"model":"Seedance2.5-mancan-flash","prompt":"p","duration":5,"resolution":"720p","ratio":"9:16",
		  "assets":[{"type":"image","url":"https://example.com/1.png"},{"type":"video","url":"https://example.com/1.mp4"},{"type":"audio","url":"https://example.com/1.mp3"}]}`,
		`{"model":"Seedance2.5-mancan-flash","prompt":"p","duration":5,"resolution":"720p","ratio":"9:16",
		  "images":["https://example.com/1.png"],"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`)

	translated, err := TranslatePPVideoUnifiedRequest(PlatformHuahuoSeedance, []byte(
		`{"model":"Seedance2.5-mancan-anci","prompt":"p","duration":5,"assets":[{"type":"audio","url":"https://example.com/1.mp3"}]}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformHuahuoSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/1.mp3", gjson.GetBytes(prepared, "audio_references.0").String())
	require.Equal(t, "5", gjson.GetBytes(prepared, "seconds").String())

	// First and last frames are refused, as everywhere else.
	_, err = TranslatePPVideoUnifiedRequest(PlatformHuahuoSeedance, []byte(
		`{"model":"Seedance2.5-mancan-anci","prompt":"p","duration":5,"assets":[`+
			`{"type":"image","role":"first_frame","url":"https://example.com/1.png"},{"type":"image","role":"last_frame","url":"https://example.com/2.png"}]}`))
	if err == nil {
		// The translator may pass frames on; the native normalizer must then refuse them.
		_, _, err = PreparePPVideoRequestBody(PlatformHuahuoSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"Seedance2.5-mancan-anci","prompt":"p","duration":5,"first_frame":"https://example.com/1.png"}`))
	}
	require.Error(t, err)
}

func TestHuahuoSeedanceCapabilities(t *testing.T) {
	t.Parallel()

	c := PPVideoCapabilitiesForModel(PlatformHuahuoSeedance, "inmind/video/seedance2-5-flash")
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, c.Duration)
	require.Equal(t, []string{"480p", "720p"}, c.Resolutions)
	require.Equal(t, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, c.Ratios)
	require.Equal(t, 30, *c.Assets.ReferenceImages)
	require.Equal(t, 10, *c.Assets.ReferenceVideos)
	require.Equal(t, 10, *c.Assets.ReferenceAudios)
	// The two 满参稳定 models take no reference videos (the final table of 2026-11-19).
	for _, model := range []string{"Seedance2.5-mancan-anmiao", "Seedance2.5-mancan-anci"} {
		require.Equal(t, 0, *PPVideoCapabilitiesForModel(PlatformHuahuoSeedance, model).Assets.ReferenceVideos, model)
	}
	// The hub reports the same limits for a line of this platform.
	require.Equal(t,
		PPVideoCapabilitiesForModel(PlatformHuahuoSeedance, "Seedance2.5-mancan-anmiao"),
		PPVideoCapabilitiesForModel(PlatformSeedanceHub, "Seedance2.5--720p--满参稳定--按秒--HH"))
}

func TestHuahuoSeedanceMediaQuoteSpec(t *testing.T) {
	t.Parallel()

	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformHuahuoSeedance, "Seedance2.5-mancan-flash", "", 30)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformHuahuoSeedance, "inmind/video/seedance2-5-flash", "480p", 4)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformHuahuoSeedance, "Seedance2.5-mancan-anci", "480p", 5)
	require.False(t, ok, "resolution the model does not offer")
	_, ok = normalizeMediaQuoteVideoSpec(PlatformHuahuoSeedance, "Seedance2.5-mancan-anci", "720p", 31)
	require.False(t, ok, "above the longest length")

	// The hub line is estimated with the platform's table.
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.5--480p/720p--flash满参稳定--按秒--HH", "480p", 10)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)
}

func TestHuahuoSeedanceHubLines(t *testing.T) {
	t.Parallel()

	var lines []SeedanceHubLine
	for _, line := range SeedanceHubLines() {
		if line.Platform == PlatformHuahuoSeedance {
			lines = append(lines, line)
		}
	}
	require.Len(t, lines, 4)

	billing := map[string]BillingMode{}
	for _, line := range lines {
		billing[line.Name] = line.Billing
		// Every name a line answers to reaches the line: the hub name, the platform's public name
		// and the upstream name.
		for _, name := range []string{line.Name, line.Model} {
			found, ok := SeedanceHubLineByName(name)
			require.True(t, ok, name)
			require.Equal(t, line.Name, found.Name)
		}
		spec, ok := huahuoModelSpecFor(line.Model)
		require.True(t, ok, line.Name)
		found, ok := SeedanceHubLineByName(spec.Name)
		require.True(t, ok, spec.Name)
		require.Equal(t, line.Name, found.Name)
		// Only this platform serves it.
		_, ok = SeedanceHubLineFor(PlatformHongpeiSeedance, line.Model)
		require.False(t, ok, line.Name)

		// A hub request is rewritten to the platform's public name, and then goes through the platform.
		quoted := fmt.Sprintf("%q", line.Name)
		resolved, body, err := ResolveSeedanceHubRequest([]byte(`{"model":` + quoted + `,"prompt":"p","duration":5}`))
		require.NoError(t, err)
		require.Equal(t, line.Name, resolved.Name)
		require.Equal(t, line.Model, gjson.GetBytes(body, "model").String())
		prepared, _, err := PreparePPVideoRequestBody(resolved.Platform, PPVideoOperationGeneric, body)
		require.NoError(t, err)
		require.Equal(t, spec.Name, gjson.GetBytes(prepared, "model").String())
	}
	require.Equal(t, map[string]BillingMode{
		"Seedance2.5--720p--满参稳定--按秒--HH":           BillingModeVideo,
		"Seedance2.5--720p--满参稳定--按次--HH":           BillingModePerRequest,
		"Seedance2.5--480p/720p--flash满参稳定--按秒--HH": BillingModeVideo,
		"Seedance2.5--720p--shuliang1--按次--HH":        BillingModePerRequest,
	}, billing)
}

// video-2.5shuliang1 (2026-11-19): the upstream name says no "seedance", so clients that pick video
// models by name would skip it; it is offered as Seedance2.5-shuliang1. 720p only, 5-30 seconds,
// 30 images / 10 videos / 10 audios, billed per request.
func TestHuahuoSeedanceShuliang1PublicName(t *testing.T) {
	t.Parallel()

	const publicName, upstreamName, shortName = "Seedance2.5-shuliang1", huahuoTestShuliang, "video-2.5shuliang1"
	require.Contains(t, strings.ToLower(publicName), "seedance")
	require.Equal(t, shortName, huahuoShortName(upstreamName))

	spec, ok := huahuoModelSpecFor("SEEDANCE2.5-SHULIANG1")
	require.True(t, ok)
	require.Equal(t, upstreamName, spec.Name)
	require.Equal(t, publicName, spec.publicName())
	require.Equal(t, 5, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	require.Equal(t, []string{"720p"}, spec.Resolutions)
	require.Equal(t, 30, spec.MaxImages)
	require.Equal(t, 10, spec.MaxVideos)
	require.Equal(t, 10, spec.MaxAudios)

	require.True(t, ppVideoModelBelongsToPlatform(PlatformHuahuoSeedance, upstreamName))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformHuahuoSeedance, publicName))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformJijiuSeedance, publicName))
	require.Equal(t, []string{publicName, upstreamName, shortName}, ppVideoHuahuoModelAliases(upstreamName))
	require.Equal(t, []string{publicName, upstreamName, shortName}, ppVideoPricingModelAliases(PlatformHuahuoSeedance, publicName))
	require.Equal(t, publicName, PPVideoPublicModelName(PlatformHuahuoSeedance, upstreamName))
	require.Equal(t, publicName, PPVideoPublicModelName(PlatformHuahuoSeedance, shortName))
	require.Equal(t, []string{publicName},
		filterPPVideoPublicModelsForPlatform(PlatformHuahuoSeedance, []string{upstreamName, shortName, publicName, "other"}))

	// Any of the three names in the request: the upstream gets its own (full) name, the task shows the public one.
	for _, model := range []string{publicName, upstreamName, shortName} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformHuahuoSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+model+`","prompt":"x","seconds":5,"images":["https://example.com/1.png"]}`))
		require.NoError(t, err, model)
		require.Equal(t, upstreamName, gjson.GetBytes(prepared, "model").String(), model)
		require.Equal(t, "5", gjson.GetBytes(prepared, "seconds").String(), model)
		require.Equal(t, "720p", gjson.GetBytes(prepared, "resolution").String(), model)
		require.Equal(t, publicName, public.Model, model)
		require.Equal(t, upstreamName, public.UpstreamModel, model)
		meta := PPVideoBillingMetadataFromRequest(PlatformHuahuoSeedance, prepared)
		require.Equal(t, upstreamName, meta.Model, model)
		require.Equal(t, "720p", meta.VideoResolution, model)
	}

	// Outside the limits: 4 and 31 seconds, 480p, 1080p.
	for _, body := range []string{
		`{"model":"` + publicName + `","prompt":"x","seconds":4}`,
		`{"model":"` + publicName + `","prompt":"x","seconds":31}`,
		`{"model":"` + publicName + `","prompt":"x","seconds":5,"resolution":"480p"}`,
		`{"model":"` + publicName + `","prompt":"x","seconds":5,"resolution":"1080p"}`,
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformHuahuoSeedance, PPVideoOperationGeneric, []byte(body))
		require.Error(t, err, body)
	}
	_, _, err := PreparePPVideoRequestBody(PlatformHuahuoSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+publicName+`","prompt":"x","seconds":30}`))
	require.NoError(t, err)

	// The aggregate group answers to the line name, the public name and the upstream name.
	const line = "Seedance2.5--720p--shuliang1--按次--HH"
	for _, name := range []string{line, publicName, upstreamName, shortName} {
		found, ok := SeedanceHubLineByName(name)
		require.True(t, ok, name)
		require.Equal(t, line, found.Name, name)
		require.False(t, found.Hidden)
		require.Equal(t, BillingModePerRequest, found.Billing)
	}

	// The response and the model list never show anything but the public name.
	raw := `{"id":"task_1","object":"video","model":"` + upstreamName + `","status":"completed","progress":100}`
	parsed, err := ParsePPVideoResponse(PlatformHuahuoSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformHuahuoSeedance, []byte(raw), PPVideoPublicRequest{Model: upstreamName}, parsed)
	require.Equal(t, publicName, gjson.GetBytes(normalized, "model").String())
}

const huahuoTestShuliang = "inmind/video/video-2.5shuliang1"
