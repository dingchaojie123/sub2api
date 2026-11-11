package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	jijiuTestModelA   = "满血seedance-2.5全参A"
	jijiuTestModelC   = "满血seedance-2.5全参C"
	jijiuTestModelD   = "稳定seedance-2.5全参D"
	jijiuTestModelF   = "稳定seedance-2.5参图参音F"
	jijiuTestModel933 = "SD2.0满血稳定933-线路一"
	jijiuTestModel903 = "SD2.0特价稳定903z"
	jijiuTestModelMin = "SD2.0mini特价稳定903z"
	jijiuTestModel301 = "SD2.5特价301010线路一"
	jijiuTestModel30  = "SD2.5特价30-10线路二"

	// Public names of the two renamed lines. The J1 name has a space before 按次 and the J2
	// name does not, exactly as they were specified.
	jijiuPublic301 = "Seedance2.5按次收费J2"
	jijiuPublic30  = "Seedance2.5 按次收费J1"
)

func TestJijiuSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, jijiuSeedanceModels, 9)
	seen := map[string]struct{}{}
	for _, spec := range jijiuSeedanceModels {
		_, duplicate := seen[spec.Name]
		require.False(t, duplicate, spec.Name)
		seen[spec.Name] = struct{}{}
		require.NotEmpty(t, spec.Resolutions, spec.Name)
		require.True(t, spec.allowsResolution(spec.defaultResolution()), spec.Name)
		require.LessOrEqual(t, spec.MinSeconds, spec.MaxSeconds, spec.Name)
		require.Positive(t, spec.MaxImages, spec.Name)
	}
	// Upstream and public names never collide with each other, so a name resolves to one model.
	names := map[string]string{}
	for _, spec := range jijiuSeedanceModels {
		for _, name := range []string{spec.Name, spec.publicName()} {
			if owner, exists := names[name]; exists {
				require.Equal(t, spec.Name, owner, name)
			}
			names[name] = spec.Name
		}
	}
	require.Len(t, names, 11, "nine upstream names plus the two public names")

	// Lookup is by the full name, ignoring case, and returns the canonical spelling.
	spec, ok := jijiuModelSpecFor("  sd2.5特价301010线路一 ")
	require.True(t, ok)
	require.Equal(t, jijiuTestModel301, spec.Name)
	require.Equal(t, jijiuPublic301, spec.publicName())
	// Either name finds the model, and models without a public name keep their own.
	byPublic, ok := jijiuModelSpecFor(" seedance2.5按次收费j2 ")
	require.True(t, ok)
	require.Equal(t, jijiuTestModel301, byPublic.Name)
	require.Equal(t, jijiuTestModelD, mustJijiuSpec(t, jijiuTestModelD).publicName())
	_, ok = jijiuModelSpecFor("SD2.5")
	require.False(t, ok)
	_, ok = jijiuModelSpecFor("")
	require.False(t, ok)
}

func mustJijiuSpec(t *testing.T, model string) jijiuModelSpec {
	t.Helper()
	spec, ok := jijiuModelSpecFor(model)
	require.True(t, ok, model)
	return spec
}

func TestJijiuSeedanceModelSyncFilterAndRouting(t *testing.T) {
	t.Parallel()

	// The upstream lists every model of the site; only the nine Seedance models are offered.
	models := []string{
		"gpt-image-2.6", "gpt-6.1-sol", "suno-v6-wild", "wan3.0-video", "Minimax-h3-933特价",
		jijiuTestModelD, jijiuTestModel301, jijiuTestModel933,
	}
	// Synced models are offered under their public names.
	require.Equal(t, []string{jijiuTestModelD, jijiuPublic301, jijiuTestModel933}, filterUpstreamModelsForPlatform(PlatformJijiuSeedance, models))

	require.True(t, IsPPVideoPlatform(PlatformJijiuSeedance))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformJijiuSeedance, jijiuTestModelA))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformJijiuSeedance, "gpt-image-2.6"))
	require.False(t, ppVideoModelKnownForeignToPlatform(PlatformJijiuSeedance, jijiuTestModelA))
}

func TestJijiuSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformJijiuSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)

	statusPath, err := PPVideoStatusUpstreamPath(PlatformJijiuSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformJijiuSeedance, "jj-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer jj-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformJijiuSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformJijiuSeedance))

	// Some lines report capitals (SUCCESS / FAILURE), others completed / failed.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"v1","status":"queued","progress":0}`, PPVideoTaskStatusProcessing},
		{`{"id":"v1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"v1","status":"IN_PROGRESS"}`, PPVideoTaskStatusProcessing},
		{`{"id":"v1","status":"NOT_START"}`, PPVideoTaskStatusProcessing},
		{`{"id":"v1","status":"SUBMITTED"}`, PPVideoTaskStatusProcessing},
		{`{"id":"v1","status":"completed","progress":100}`, PPVideoTaskStatusSucceeded},
		{`{"id":"v1","status":"SUCCESS"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"v1","status":"failed"}`, PPVideoTaskStatusFailed},
		{`{"id":"v1","status":"FAILURE"}`, PPVideoTaskStatusFailed},
		{`{"id":"v1","status":"FAILED","fail_reason":"boom"}`, PPVideoTaskStatusFailed},
	} {
		body, want := tc.body, tc.want
		parsed, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(body))
		require.NoError(t, err, body)
		require.Equal(t, "v1", parsed.TaskID, body)
		require.Equal(t, want, parsed.Status, body)
	}

	failed, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(`{"id":"v1","status":"FAILURE","fail_reason":"model overloaded"}`))
	require.NoError(t, err)
	require.Equal(t, "model overloaded", failed.ErrorMessage)
	failedWithEnvelope, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(`{"id":"v1","status":"failed","error":{"message":"bad media"}}`))
	require.NoError(t, err)
	require.Equal(t, "bad media", failedWithEnvelope.ErrorMessage)

	// A length echoed by the upstream never replaces the requested seconds at settlement.
	echoed, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(`{"id":"v1","status":"completed","duration":9,"usage":{"duration":9,"input_video_duration":4},"resolution":"1080p"}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
	require.Zero(t, echoed.InputVideoDurationMilliseconds)

	// A finished task needs no video address in the response, and an address on a task that
	// is still running proves nothing: only the status decides.
	running, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(`{"id":"v1","status":"in_progress","progress":10,"url":"https://newapi.jijiucanvas.com/v1/videos/v1/content"}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusProcessing, running.Status)
	finished, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(`{"id":"v1","status":"completed"}`))
	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, finished.Status)
}

func TestJijiuSeedancePublicResponseNeverExposesTheUpstreamAddress(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: jijiuTestModelD}

	// Three shapes the upstream documents for a finished task's address.
	for _, raw := range []string{
		`{"id":"vid_9","status":"completed"}`,
		`{"id":"vid_9","status":"SUCCESS","url":"https://cdn.upstream.example/a.mp4"}`,
		`{"id":"vid_9","status":"completed","video_url":"/v1/videos/vid_9/content","download_url":"https://newapi.jijiucanvas.com/v1/videos/vid_9/content?access=abc"}`,
	} {
		parsed, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(raw))
		require.NoError(t, err, raw)
		normalized := NormalizePPVideoPublicResponse(PlatformJijiuSeedance, []byte(raw), public, parsed)
		require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String(), raw)
		require.Equal(t, jijiuTestModelD, gjson.GetBytes(normalized, "model").String(), raw)
		require.Equal(t, "/v1/videos/vid_9/content", gjson.GetBytes(normalized, "video_url").String(), raw)
		require.Equal(t, "/v1/videos/vid_9/content", gjson.GetBytes(normalized, "result_url").String(), raw)
		require.False(t, gjson.GetBytes(normalized, "download_url").Exists(), raw)
		require.NotContains(t, string(normalized), "jijiucanvas", raw)
		require.NotContains(t, string(normalized), "cdn.upstream.example", raw)
	}

	// A running task shows no address at all.
	running := `{"id":"vid_9","status":"in_progress","progress":30,"url":"https://newapi.jijiucanvas.com/v1/videos/vid_9/content"}`
	parsed, err := ParsePPVideoResponse(PlatformJijiuSeedance, []byte(running))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformJijiuSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "url").Exists())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
}

func TestJijiuSeedanceNormalizesNativeRequestPerModel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "model without a resolution choice takes none and sends the plain field names",
			body: `{"model":"` + jijiuTestModel301 + `","prompt":"晴天的小院","duration":5,"ratio":"16:9",` +
				`"images":["https://example.com/1.png"],"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"` + jijiuTestModel301 + `","prompt":"晴天的小院","seconds":5,"aspect_ratio":"16:9",` +
				`"images":["https://example.com/1.png"],"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the full-power C line names its video and audio fields differently",
			body: `{"model":"` + jijiuTestModelC + `","prompt":"x","duration":6,` +
				`"images":["https://example.com/1.png"],"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"` + jijiuTestModelC + `","prompt":"x","seconds":6,` +
				`"images":["https://example.com/1.png"],"video_refs":["https://example.com/1.mp4"],"audio_refs":["https://example.com/1.mp3"]}`,
		},
		{
			name: "a model with a resolution choice sends it, defaulting to 720p",
			body: `{"model":"` + jijiuTestModelD + `","prompt":"x","duration":5}`,
			want: `{"model":"` + jijiuTestModelD + `","prompt":"x","seconds":5,"resolution":"720p"}`,
		},
		{
			name: "an explicit resolution is sent in lower case",
			body: `{"model":"` + jijiuTestModelD + `","prompt":"x","duration":5,"resolution":"1080P"}`,
			want: `{"model":"` + jijiuTestModelD + `","prompt":"x","seconds":5,"resolution":"1080p"}`,
		},
		{
			name: "the only resolution of a fixed model is accepted and not sent",
			body: `{"model":"` + jijiuTestModelA + `","prompt":"x","duration":4,"resolution":"720p"}`,
			want: `{"model":"` + jijiuTestModelA + `","prompt":"x","seconds":4}`,
		},
		{
			name: "the 480p-only F line sends its resolution",
			body: `{"model":"` + jijiuTestModelF + `","prompt":"x","duration":4}`,
			want: `{"model":"` + jijiuTestModelF + `","prompt":"x","seconds":4,"resolution":"480p"}`,
		},
		{
			name: "the model name is written in its canonical spelling",
			body: `{"model":"sd2.0mini特价稳定903z","prompt":"x","duration":4}`,
			want: `{"model":"` + jijiuTestModelMin + `","prompt":"x","seconds":4}`,
		},
		{
			name: "aliases of the other Seedance models are accepted: aspect_ratio, seconds, single image",
			body: `{"model":"` + jijiuTestModel301 + `","prompt":"x","seconds":5,"aspect_ratio":"9:16","image_url":"https://example.com/1.png"}`,
			want: `{"model":"` + jijiuTestModel301 + `","prompt":"x","seconds":5,"aspect_ratio":"9:16","images":["https://example.com/1.png"]}`,
		},
		{
			name: "object items, reference_* and camelCase names",
			body: `{"model":"` + jijiuTestModel301 + `","prompt":"x","duration":5,` +
				`"reference_images":[{"image_url":"https://example.com/1.png"},"https://example.com/2.png"],` +
				`"referenceVideos":[{"video_url":"https://example.com/1.mp4"}],"reference_audios":[{"url":"https://example.com/1.mp3"}]}`,
			want: `{"model":"` + jijiuTestModel301 + `","prompt":"x","seconds":5,` +
				`"images":["https://example.com/1.png","https://example.com/2.png"],"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
		},
		{
			name: "the upstream's own names for the C line are accepted",
			body: `{"model":"` + jijiuTestModelC + `","prompt":"x","duration":5,"video_refs":["https://example.com/1.mp4"],"audio_refs":["https://example.com/1.mp3"]}`,
			want: `{"model":"` + jijiuTestModelC + `","prompt":"x","seconds":5,"video_refs":["https://example.com/1.mp4"],"audio_refs":["https://example.com/1.mp3"]}`,
		},
		{
			name: "numeric string duration, empty resolution and duration_seconds",
			body: `{"model":"` + jijiuTestModelD + `","prompt":"x","duration_seconds":"7","resolution":""}`,
			want: `{"model":"` + jijiuTestModelD + `","prompt":"x","seconds":7,"resolution":"720p"}`,
		},
		{
			name: "the longest length of the 933 line",
			body: `{"model":"` + jijiuTestModel933 + `","prompt":"x","duration":15,"resolution":"480p"}`,
			want: `{"model":"` + jijiuTestModel933 + `","prompt":"x","seconds":15,"resolution":"480p"}`,
		},
		{
			name: "the 30-10 line accepts 4 seconds without references",
			body: `{"model":"` + jijiuTestModel30 + `","prompt":"x","duration":4}`,
			want: `{"model":"` + jijiuTestModel30 + `","prompt":"x","seconds":4}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestJijiuSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	prepared, public, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+jijiuTestModel933+`","prompt":"x","duration":12,"resolution":"1080p","images":["https://example.com/1.png"]}`))
	require.NoError(t, err)
	require.Equal(t, jijiuTestModel933, public.Model)
	require.Equal(t, jijiuTestModel933, public.UpstreamModel)
	require.Equal(t, int64(12000), public.DurationMilliseconds)
	require.Equal(t, 1, public.VideoCount)
	require.Equal(t, "1080p", public.Resolution)
	require.True(t, public.HasImage)

	// Billing reads the prepared body: seconds, the model and (where sent) the resolution.
	meta := PPVideoBillingMetadataFromRequest(PlatformJijiuSeedance, prepared)
	require.Equal(t, PlatformJijiuSeedance, meta.Platform)
	require.Equal(t, jijiuTestModel933, meta.Model)
	require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
	require.Equal(t, 1, meta.VideoCount)
	require.Equal(t, "1080p", meta.VideoResolution)
}

func TestJijiuSeedanceBillingResolutionOfModelsThatDoNotSendOne(t *testing.T) {
	t.Parallel()

	// These models take no resolution field, yet the task is priced and recorded at their fixed one.
	for model, want := range map[string]string{
		jijiuTestModelA:   "720p",
		jijiuTestModelC:   "1080p",
		jijiuTestModel903: "720p",
		jijiuTestModelMin: "720p",
		jijiuTestModel301: "720p",
		jijiuTestModel30:  "720p",
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+model+`","prompt":"x","duration":10}`))
		require.NoError(t, err, model)
		require.False(t, gjson.GetBytes(prepared, "resolution").Exists(), model)
		require.Equal(t, want, public.Resolution, model)
		require.Equal(t, want, PPVideoBillingMetadataFromRequest(PlatformJijiuSeedance, prepared).VideoResolution, model)
	}

	// A model with a choice that was left out is priced at its default, which is also sent.
	prepared, _, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+jijiuTestModelD+`","prompt":"x","duration":10}`))
	require.NoError(t, err)
	require.Equal(t, "720p", PPVideoBillingMetadataFromRequest(PlatformJijiuSeedance, prepared).VideoResolution)
}

func TestJijiuSeedanceSendsTheMappedUpstreamModel(t *testing.T) {
	t.Parallel()

	// Both the public name and the upstream name must be models of the platform: the request is
	// first checked without an account, which refuses any other name. A mapping is therefore a
	// way to serve one upstream line under another model's name, not to invent new names.
	account := &Account{
		Platform: PlatformJijiuSeedance,
		Credentials: map[string]any{
			"model_mapping": map[string]any{jijiuTestModel301: jijiuTestModel30},
		},
	}
	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric,
		[]byte(`{"model":"`+jijiuTestModel301+`","prompt":"lake","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, jijiuPublic301, public.Model)
	require.Equal(t, jijiuTestModel30, public.UpstreamModel)
	require.Equal(t, jijiuTestModel30, gjson.GetBytes(prepared, "model").String())

	// The limits of the upstream line apply, not those of the public name: the 30-10 line
	// allows 4 seconds, the 301010 line does not.
	_, _, err = PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric,
		[]byte(`{"model":"`+jijiuTestModel301+`","prompt":"lake","duration":4}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric,
		[]byte(`{"model":"`+jijiuTestModel301+`","prompt":"lake","duration":4}`))
	require.ErrorContains(t, err, "from 5 to 30 seconds")

	// A mapping to a name the platform does not know is refused before anything is sent.
	bad := &Account{
		Platform:    PlatformJijiuSeedance,
		Credentials: map[string]any{"model_mapping": map[string]any{jijiuTestModel301: "gpt-image-2.6"}},
	}
	_, _, err = PreparePPVideoRequestBodyForAccount(bad, PPVideoOperationGeneric,
		[]byte(`{"model":"`+jijiuTestModel301+`","prompt":"lake","duration":5}`))
	require.ErrorContains(t, err, "is not available")

	// A public name that is not a model of the platform is refused outright.
	_, _, err = PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric,
		[]byte(`{"model":"my-seedance","prompt":"lake","duration":5}`))
	require.ErrorContains(t, err, "is not available")
}

func TestJijiuSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	head := func(model string) string { return `{"model":"` + model + `","prompt":"x",` }
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		{"unknown model", head("gpt-image-2.6") + `"duration":5}`, "is not available"},
		{"missing prompt", `{"model":"` + jijiuTestModelD + `","duration":5}`, "prompt is required"},
		{"empty prompt with media", `{"model":"` + jijiuTestModelD + `","prompt":"  ","duration":5,"images":[` + ref + `]}`, "prompt is required"},
		{"prompt not a string", `{"model":"` + jijiuTestModelD + `","prompt":5,"duration":5}`, "prompt must be a string"},
		{"missing duration", head(jijiuTestModelD) + `"ratio":"16:9"}`, "duration is required"},
		{"non-numeric string duration", head(jijiuTestModelD) + `"duration":"abc"}`, "duration for this model must be an integer"},
		{"fractional duration", head(jijiuTestModelD) + `"duration":5.5}`, "duration for this model must be an integer"},
		{"boolean duration", head(jijiuTestModelD) + `"duration":true}`, "duration for this model must be an integer"},
		{"below the minimum of the 933 line", head(jijiuTestModel933) + `"duration":3}`, "from 4 to 15 seconds"},
		{"above the maximum of the 933 line", head(jijiuTestModel933) + `"duration":16}`, "from 4 to 15 seconds"},
		{"below the clamp of the 903z line", head(jijiuTestModel903) + `"duration":9}`, "from 10 to 15 seconds"},
		{"below the minimum of the C line", head(jijiuTestModelC) + `"duration":4}`, "from 5 to 30 seconds"},
		{"4 seconds with an audio on the 30-10 line", head(jijiuTestModel30) + `"duration":4,"audios":[` + ref + `]}`, "from 5 to 30 seconds"},
		{"4 seconds with ten images on the 30-10 line", head(jijiuTestModel30) + `"duration":4,"images":[` + repeatedJijiuItems(ref, 10) + `]}`, "from 5 to 30 seconds"},
		{"another resolution on a fixed model", head(jijiuTestModelA) + `"duration":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"1080p on the 480p-only line", head(jijiuTestModelF) + `"duration":5,"resolution":"1080p"}`, "resolution must be 480p for this model"},
		{"2k is not 1080p", head(jijiuTestModelD) + `"duration":5,"resolution":"2k"}`, "resolution must be 480p, 720p, 1080p for this model"},
		{"resolution not a string", head(jijiuTestModelD) + `"duration":5,"resolution":720}`, "resolution must be a string"},
		{"first frame", head(jijiuTestModelD) + `"duration":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"start frame url", head(jijiuTestModelD) + `"duration":5,"start_frame_url":` + ref + `}`, "does not support first or last frames"},
		{"generate_audio", head(jijiuTestModelD) + `"duration":5,"generate_audio":true}`, "does not support generate_audio"},
		{"seed", head(jijiuTestModelD) + `"duration":5,"seed":1}`, "does not support seed"},
		{"watermark", head(jijiuTestModelD) + `"duration":5,"watermark":false}`, "does not support watermark"},
		{"callback", head(jijiuTestModelD) + `"duration":5,"callback_url":"https://example.com/cb"}`, "does not support callback_url"},
		{"negative prompt", head(jijiuTestModelD) + `"duration":5,"negative_prompt":"blur"}`, "does not support negative_prompt"},
		{"quality", head(jijiuTestModelD) + `"duration":5,"quality":"high"}`, "does not support quality"},
		{"assets on v1", head(jijiuTestModelD) + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"several videos", head(jijiuTestModelD) + `"duration":5,"n":2}`, "one video per request"},
		{"videos on a model without reference videos", head(jijiuTestModelA) + `"duration":5,"videos":[` + ref + `]}`, "does not support reference videos"},
		{"videos on the F line", head(jijiuTestModelF) + `"duration":5,"videos":[` + ref + `]}`, "does not support reference videos"},
		{"too many images for the 933 line", head(jijiuTestModel933) + `"duration":5,"images":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
		{"too many images", head(jijiuTestModelD) + `"duration":5,"images":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many videos", head(jijiuTestModel301) + `"duration":5,"videos":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference videos"},
		{"videos on the D line (its video tier has no price)", head(jijiuTestModelD) + `"duration":5,"videos":[` + ref + `]}`, "does not support reference videos"},
		{"too many audios for the 933 line", head(jijiuTestModel933) + `"duration":5,"audios":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference audios"},
		{"more than 30 items in total on the 30-10 line", head(jijiuTestModel30) + `"duration":5,"images":[` + repeatedJijiuItems(ref, 20) + `],"videos":[` + repeatedJijiuItems(ref, 10) + `],"audios":[` + repeatedJijiuItems(ref, 1) + `]}`, "at most 30 reference items in total"},
		{"two aliases for the images", head(jijiuTestModelD) + `"duration":5,"images":[` + ref + `],"reference_images":[` + ref + `]}`, "only one of"},
		{"not an array", head(jijiuTestModelD) + `"duration":5,"images":` + ref + `}`, "must be an array"},
		{"empty item", head(jijiuTestModelD) + `"duration":5,"videos":[""]}`, "must be a URL"},
		{"plain http is refused", head(jijiuTestModelD) + `"duration":5,"images":["http://example.com/1.png"]}`, "public HTTPS URL"},
		{"private address", head(jijiuTestModelD) + `"duration":5,"images":["https://10.0.0.1/1.png"]}`, "public HTTPS URL"},
		{"ratio not a string", head(jijiuTestModelD) + `"duration":5,"ratio":169}`, "ratio must be a string"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.Error(t, err)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func repeatedJijiuItems(item string, count int) string {
	var buffer bytes.Buffer
	for index := 0; index < count; index++ {
		if index > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(item)
	}
	return buffer.String()
}

func TestJijiuSeedanceLimitsAtTheBoundary(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	for _, tc := range []struct {
		name string
		body string
	}{
		{"30 images", `{"model":"` + jijiuTestModelD + `","prompt":"x","duration":5,"images":[` + repeatedJijiuItems(ref, 30) + `]}`},
		{"10 videos and 10 audios", `{"model":"` + jijiuTestModel301 + `","prompt":"x","duration":5,"videos":[` + repeatedJijiuItems(ref, 10) + `],"audios":[` + repeatedJijiuItems(ref, 10) + `]}`},
		{"30 items in total on the 30-10 line", `{"model":"` + jijiuTestModel30 + `","prompt":"x","duration":5,"images":[` + repeatedJijiuItems(ref, 10) + `],"videos":[` + repeatedJijiuItems(ref, 10) + `],"audios":[` + repeatedJijiuItems(ref, 10) + `]}`},
		{"9 images and 4 seconds on the 30-10 line", `{"model":"` + jijiuTestModel30 + `","prompt":"x","duration":4,"images":[` + repeatedJijiuItems(ref, 9) + `]}`},
		{"10 images and 5 seconds on the 30-10 line", `{"model":"` + jijiuTestModel30 + `","prompt":"x","duration":5,"images":[` + repeatedJijiuItems(ref, 10) + `]}`},
		{"9 images, 3 videos and 3 audios on the 933 line", `{"model":"` + jijiuTestModel933 + `","prompt":"x","duration":4,"images":[` + repeatedJijiuItems(ref, 9) + `],"videos":[` + repeatedJijiuItems(ref, 3) + `],"audios":[` + repeatedJijiuItems(ref, 3) + `]}`},
		{"longest and shortest of the 903z line", `{"model":"` + jijiuTestModel903 + `","prompt":"x","duration":10}`},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
		})
	}
}

func TestJijiuSeedanceDefaultsAndEndpoints(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          42,
		Platform:    PlatformJijiuSeedance,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "jj-key"},
	}
	modelSvc := &AccountTestService{cfg: &config.Config{Security: config.SecurityConfig{
		URLAllowlist: config.URLAllowlistConfig{Enabled: false},
	}}}
	modelReq, err := modelSvc.buildPPVideoUpstreamModelsRequest(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "https://newapi.jijiucanvas.com/v1/models", modelReq.URL.String())
	require.Equal(t, "Bearer jj-key", modelReq.Header.Get("Authorization"))

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"id":"vid_1","object":"video","status":"queued","progress":0}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"id":"vid_1","status":"SUCCESS"}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"video/mp4"}},
			Body:       io.NopCloser(bytes.NewBufferString("video")),
		},
	}}
	gateway := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}

	prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric,
		[]byte(`{"model":"`+jijiuTestModelD+`","prompt":"lake","duration":5}`))
	require.NoError(t, err)
	result, err := gateway.ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "", prepared, public)
	require.NoError(t, err)
	require.Equal(t, "vid_1", result.ResponseID)
	require.Equal(t, "https://newapi.jijiucanvas.com/v1/videos", upstream.requests[0].URL.String())
	require.Equal(t, http.MethodPost, upstream.requests[0].Method)
	require.Equal(t, "Bearer jj-key", upstream.requests[0].Header.Get("Authorization"))

	_, err = gateway.ForwardPPVideoBuffered(context.Background(), nil, account, PPVideoOperationGeneric, "vid_1", nil, public)
	require.NoError(t, err)
	require.Equal(t, "https://newapi.jijiucanvas.com/v1/videos/vid_1", upstream.requests[1].URL.String())
	require.Equal(t, http.MethodGet, upstream.requests[1].Method)

	contentResp, err := gateway.ForwardPPVideoContent(context.Background(), account, "vid_1", "", "")
	require.NoError(t, err)
	defer contentResp.Body.Close()
	require.Equal(t, "https://newapi.jijiucanvas.com/v1/videos/vid_1/content", upstream.requests[2].URL.String())
	require.Equal(t, "Bearer jj-key", upstream.requests[2].Header.Get("Authorization"))
}

func TestJijiuSeedanceUsesExactPerModelChannelPricingWithResolutionTiers(t *testing.T) {
	t.Parallel()

	groupID := int64(410)
	base, p480, p1080 := 0.3, 0.2, 1.0
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
		resolver: newPPVideoResolverWithChannel(t, groupID, PlatformJijiuSeedance, []ChannelModelPricing{{
			Platform:        PlatformJijiuSeedance,
			Models:          []string{jijiuTestModel933},
			BillingMode:     BillingModeVideo,
			PerRequestPrice: &base,
			Intervals: []PricingInterval{
				{TierLabel: "480p", PerRequestPrice: &p480},
				{TierLabel: "1080p", PerRequestPrice: &p1080},
			},
		}}),
	}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{
		Platform:             PlatformJijiuSeedance,
		RateMultiplier:       1,
		VideoRateIndependent: true,
		VideoRateMultiplier:  1,
	}}
	cost := func(model, resolution string) (*CostBreakdown, error) {
		return svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
			Platform:                      PlatformJijiuSeedance,
			Model:                         model,
			RequestedDurationMilliseconds: 10000,
			VideoCount:                    1,
			VideoResolution:               resolution,
		})
	}

	// Each resolution is charged at its own tier; one with no tier uses the default price.
	for resolution, wantPerSecond := range map[string]float64{"480p": 0.2, "720p": 0.3, "1080p": 1.0, "": 0.3} {
		got, err := cost(jijiuTestModel933, resolution)
		require.NoError(t, err, resolution)
		require.Equal(t, PPVideoBillingFormulaPerSecond, got.BillingFormula, resolution)
		require.InDelta(t, 10, got.BillingUnits, 1e-12, resolution)
		require.InDelta(t, 10*wantPerSecond, got.TotalCost, 1e-9, resolution)
	}

	// A model without its own channel price is refused, and the message names the platform.
	_, err := cost(jijiuTestModelD, "720p")
	require.ErrorContains(t, err, "Jijiu Seedance")
}

func TestJijiuSeedancePublicNames(t *testing.T) {
	t.Parallel()

	// Either name is accepted; the upstream always receives its own name, the client always
	// sees the public one.
	for _, requested := range []string{jijiuTestModel301, jijiuPublic301, "SEEDANCE2.5按次收费J2"} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric,
			[]byte(`{"model":"`+requested+`","prompt":"x","duration":5}`))
		require.NoError(t, err, requested)
		require.Equal(t, jijiuTestModel301, gjson.GetBytes(prepared, "model").String(), requested)
		require.Equal(t, jijiuPublic301, public.Model, requested)
		require.Equal(t, jijiuTestModel301, public.UpstreamModel, requested)
	}
	for _, requested := range []string{jijiuTestModel30, jijiuPublic30} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric,
			[]byte(`{"model":"`+requested+`","prompt":"x","duration":4}`))
		require.NoError(t, err, requested)
		require.Equal(t, jijiuTestModel30, gjson.GetBytes(prepared, "model").String(), requested)
		require.Equal(t, jijiuPublic30, public.Model, requested)
	}
	// The two public names are different models: J2 is the 301010 line (5 s minimum), J1 the 30-10 line.
	_, _, err := PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric,
		[]byte(`{"model":"`+jijiuPublic301+`","prompt":"x","duration":4}`))
	require.ErrorContains(t, err, "from 5 to 30 seconds")
	_, _, err = PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric,
		[]byte(`{"model":"`+jijiuPublic30+`","prompt":"x","duration":4}`))
	require.NoError(t, err)
	// Without the space J1 is not a model.
	_, _, err = PreparePPVideoRequestBody(PlatformJijiuSeedance, PPVideoOperationGeneric,
		[]byte(`{"model":"Seedance2.5按次收费J1","prompt":"x","duration":4}`))
	require.ErrorContains(t, err, "is not available")

	// Names and aliases.
	require.Equal(t, []string{jijiuPublic301, jijiuTestModel301}, ppVideoJijiuModelAliases(jijiuTestModel301))
	require.Equal(t, []string{jijiuPublic301, jijiuTestModel301}, ppVideoJijiuModelAliases(jijiuPublic301))
	require.Equal(t, []string{jijiuTestModelD}, ppVideoJijiuModelAliases(jijiuTestModelD))
	require.Nil(t, ppVideoJijiuModelAliases("gpt-image-2.6"))
	require.Equal(t, jijiuPublic30, PPVideoPublicModelName(PlatformJijiuSeedance, jijiuTestModel30))
	require.Equal(t, jijiuTestModelD, PPVideoPublicModelName(PlatformJijiuSeedance, jijiuTestModelD))
	require.Equal(t, "gpt-image-2.6", PPVideoPublicModelName(PlatformJijiuSeedance, "gpt-image-2.6"))
	require.Equal(t, jijiuTestModel30, PPVideoPublicModelName(PlatformXingAPISeedance, jijiuTestModel30), "other platforms are untouched")

	// The public model list shows public names once, whichever spelling the accounts hold.
	require.Equal(t,
		[]string{jijiuPublic301, jijiuPublic30, jijiuTestModelD},
		filterPPVideoPublicModelsForPlatform(PlatformJijiuSeedance, []string{jijiuTestModel301, jijiuPublic301, jijiuTestModel30, "gpt-image-2.6", jijiuTestModelD}))

	// A task recorded under the upstream name still shows the public name.
	for _, model := range []string{jijiuTestModel301, jijiuPublic301} {
		raw := []byte(`{"id":"vid_1","status":"completed"}`)
		parsed, err := ParsePPVideoResponse(PlatformJijiuSeedance, raw)
		require.NoError(t, err)
		normalized := NormalizePPVideoPublicResponse(PlatformJijiuSeedance, raw, PPVideoPublicRequest{Model: model}, parsed)
		require.Equal(t, jijiuPublic301, gjson.GetBytes(normalized, "model").String(), model)
		require.NotContains(t, string(normalized), "SD2.5", model)
	}

	// An account that holds either name serves requests under either name.
	for _, configured := range []string{jijiuTestModel301, jijiuPublic301} {
		account := &Account{
			Platform:    PlatformJijiuSeedance,
			Credentials: map[string]any{"model_mapping": map[string]any{configured: configured}},
		}
		require.True(t, account.IsModelSupported(jijiuTestModel301), configured)
		require.True(t, account.IsModelSupported(jijiuPublic301), configured)
		require.False(t, account.IsModelSupported(jijiuPublic30), configured)
		for _, requested := range []string{jijiuTestModel301, jijiuPublic301} {
			prepared, public, err := PreparePPVideoRequestBodyForAccount(account, PPVideoOperationGeneric,
				[]byte(`{"model":"`+requested+`","prompt":"x","duration":5}`))
			require.NoError(t, err, configured+" / "+requested)
			require.Equal(t, jijiuTestModel301, gjson.GetBytes(prepared, "model").String(), configured+" / "+requested)
			require.Equal(t, jijiuPublic301, public.Model, configured+" / "+requested)
		}
	}
}

func TestJijiuSeedancePricesAreFoundUnderEitherName(t *testing.T) {
	t.Parallel()

	groupID := int64(411)
	publicPrice, upstreamPrice := 6.0, 5.0
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{
		Platform:             PlatformJijiuSeedance,
		RateMultiplier:       1,
		VideoRateIndependent: true,
		VideoRateMultiplier:  1,
	}}
	cost := func(svc *OpenAIGatewayService, model string) (*CostBreakdown, error) {
		return svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
			Platform:                      PlatformJijiuSeedance,
			Model:                         model, // the prepared request carries the upstream name
			RequestedDurationMilliseconds: 5000,
			VideoCount:                    1,
		})
	}
	service := func(prices ...ChannelModelPricing) *OpenAIGatewayService {
		return &OpenAIGatewayService{
			billingService: newTestBillingService(),
			resolver:       newPPVideoResolverWithChannel(t, groupID, PlatformJijiuSeedance, prices),
		}
	}
	priced := func(model string, price *float64) ChannelModelPricing {
		return ChannelModelPricing{
			Platform: PlatformJijiuSeedance, Models: []string{model}, BillingMode: BillingModePerRequest, PerRequestPrice: price,
		}
	}

	// A price configured under the public name is used for a request that carries the upstream name.
	got, err := cost(service(priced(jijiuPublic301, &publicPrice)), jijiuTestModel301)
	require.NoError(t, err)
	require.InDelta(t, 6, got.TotalCost, 1e-9)

	// A price configured before the rename, under the upstream name, still applies.
	got, err = cost(service(priced(jijiuTestModel301, &upstreamPrice)), jijiuTestModel301)
	require.NoError(t, err)
	require.InDelta(t, 5, got.TotalCost, 1e-9)

	// When both exist the public name wins.
	got, err = cost(service(priced(jijiuTestModel301, &upstreamPrice), priced(jijiuPublic301, &publicPrice)), jijiuTestModel301)
	require.NoError(t, err)
	require.InDelta(t, 6, got.TotalCost, 1e-9)

	// A price under the other renamed line's name never applies.
	_, err = cost(service(priced(jijiuPublic30, &publicPrice)), jijiuTestModel301)
	require.ErrorContains(t, err, "Jijiu Seedance")
}

func TestJijiuSeedanceCapabilitiesAndQuoteUnderPublicNames(t *testing.T) {
	t.Parallel()

	for _, model := range []string{jijiuTestModel30, jijiuPublic30} {
		capabilities := PPVideoCapabilitiesForModel(PlatformJijiuSeedance, model)
		require.NotNil(t, capabilities.Duration, model)
		require.Equal(t, 4, capabilities.Duration.Min, model)
		resolution, valid := normalizeMediaQuoteVideoSpec(PlatformJijiuSeedance, model, "", 4)
		require.True(t, valid, model)
		require.Equal(t, "720p", resolution, model)
	}
	_, valid := normalizeMediaQuoteVideoSpec(PlatformJijiuSeedance, jijiuPublic301, "", 4)
	require.False(t, valid, "the J2 line starts at 5 seconds")
}

func TestJijiuSeedanceCapabilities(t *testing.T) {
	t.Parallel()

	d := PPVideoCapabilitiesForModel(PlatformJijiuSeedance, jijiuTestModelD)
	require.Equal(t, []string{"v1", "v2"}, d.Protocols)
	require.NotNil(t, d.Duration)
	require.Equal(t, 4, d.Duration.Min)
	require.Equal(t, 30, d.Duration.Max)
	require.Equal(t, []string{"480p", "720p", "1080p"}, d.Resolutions)
	require.NotNil(t, d.Assets)
	require.Equal(t, false, *d.Assets.FirstLastFrame)
	require.Equal(t, 30, *d.Assets.ReferenceImages)
	require.Equal(t, 0, *d.Assets.ReferenceVideos)
	require.Equal(t, 10, *d.Assets.ReferenceAudios)
	require.Equal(t, false, *d.GenerateAudio)
	require.Equal(t, false, *d.Callback)

	// A model that takes no reference videos reports zero, and the table drives every limit.
	a := PPVideoCapabilitiesForModel(PlatformJijiuSeedance, jijiuTestModelA)
	require.Equal(t, 0, *a.Assets.ReferenceVideos)
	plain := PPVideoCapabilitiesForModel(PlatformJijiuSeedance, jijiuTestModel301)
	require.Equal(t, 10, *plain.Assets.ReferenceVideos)
	require.Equal(t, []string{"720p"}, a.Resolutions)
	nine := PPVideoCapabilitiesForModel(PlatformJijiuSeedance, jijiuTestModel933)
	require.Equal(t, 15, nine.Duration.Max)
	require.Equal(t, 9, *nine.Assets.ReferenceImages)

	// Unknown models only report their protocols.
	unknown := PPVideoCapabilitiesForModel(PlatformJijiuSeedance, "wan3.0-video")
	require.Equal(t, []string{"v1", "v2"}, unknown.Protocols)
	require.Nil(t, unknown.Duration)
	require.Nil(t, unknown.Assets)
}

func TestJijiuSeedanceMediaQuoteFollowsTheModelTable(t *testing.T) {
	t.Parallel()

	resolution, valid := normalizeMediaQuoteVideoSpec(PlatformJijiuSeedance, jijiuTestModelD, "", 15)
	require.True(t, valid)
	require.Equal(t, "720p", resolution)
	resolution, valid = normalizeMediaQuoteVideoSpec(PlatformJijiuSeedance, jijiuTestModelD, "1080P", 30)
	require.True(t, valid)
	require.Equal(t, "1080p", resolution)

	for _, tc := range []struct {
		name       string
		model      string
		resolution string
		seconds    int
	}{
		{"unknown model", "gpt-image-2.6", "720p", 5},
		{"resolution the model does not have", jijiuTestModelA, "480p", 5},
		{"a loose spelling of a resolution", jijiuTestModelD, "2k", 5},
		{"below the minimum", jijiuTestModel903, "720p", 9},
		{"above the maximum", jijiuTestModel933, "720p", 16},
	} {
		_, valid := normalizeMediaQuoteVideoSpec(PlatformJijiuSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, valid, tc.name)
	}
}
