package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	zexiTest25Nsfw95 = "doubao-seedance-2.5-nsfw-95"
	zexiTest25Nsfw   = "doubao-seedance-2.5-nsfw"
	zexiTest20Nsfw95 = "doubao-seedance-2.0-nsfw-95"
	zexiTest20Nsfw   = "doubao-seedance-2.0-nsfw"
)

func TestZexiSeedanceModelTable(t *testing.T) {
	t.Parallel()

	// The four models the operator asked for, in the order of the request.
	require.Equal(t, []string{zexiTest25Nsfw95, zexiTest25Nsfw, zexiTest20Nsfw95, zexiTest20Nsfw}, zexiModelNames())

	seen := make(map[string]string)
	for _, spec := range zexiSeedanceModels {
		key := strings.ToLower(spec.Name)
		owner, dup := seen[key]
		require.False(t, dup, "name %q is used by %q and %q", spec.Name, owner, spec.Name)
		seen[key] = spec.Name
		// No separate public name: the upstream name already says seedance.
		require.Equal(t, spec.Name, spec.publicName())
		require.Contains(t, strings.ToLower(spec.Name), "seedance", spec.Name)
		require.NotContains(t, strings.ToLower(spec.Name), "zexi", spec.Name)
		require.True(t, spec.ForwardResolution, spec.Name)
		require.Equal(t, "720p", spec.defaultResolution(), spec.Name)
		require.Contains(t, spec.Resolutions, spec.defaultResolution(), spec.Name)
		require.True(t, spec.FirstLastFrame, spec.Name)

		// The limits of the plain models of the same version (the operator: same capabilities).
		if strings.Contains(spec.Name, "2.0") {
			require.Equal(t, 4, spec.MinSeconds, spec.Name)
			require.Equal(t, 15, spec.MaxSeconds, spec.Name)
			require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, spec.Resolutions, spec.Name)
			require.Equal(t, 9, spec.MaxImages, spec.Name)
			require.Equal(t, 3, spec.MaxVideos, spec.Name)
			require.Equal(t, 3, spec.MaxAudios, spec.Name)
		} else {
			require.Equal(t, 4, spec.MinSeconds, spec.Name)
			require.Equal(t, 30, spec.MaxSeconds, spec.Name)
			require.Equal(t, []string{"480p", "720p", "1080p"}, spec.Resolutions, spec.Name)
			require.Equal(t, 30, spec.MaxImages, spec.Name)
			require.Equal(t, 10, spec.MaxVideos, spec.Name)
			require.Equal(t, 10, spec.MaxAudios, spec.Name)
		}
	}

	spec, ok := zexiModelSpecFor("DOUBAO-Seedance-2.5-NSFW-95")
	require.True(t, ok)
	require.Equal(t, zexiTest25Nsfw95, spec.Name)
	require.Equal(t, []string{zexiTest25Nsfw95}, ppVideoZexiModelAliases("doubao-seedance-2.5-nsfw-95"))
	require.Equal(t, []string{zexiTest20Nsfw}, ppVideoPricingModelAliases(PlatformZexiSeedance, "DOUBAO-SEEDANCE-2.0-NSFW"))

	// Not offered: the plain models of the same site and every neighbouring name.
	for _, model := range []string{
		"", "doubao-seedance-2.5", "doubao-seedance-2.0", "doubao-seedance-2.0-fast", "doubao-seedance-2.0-mini",
		"doubao-seedance-2-0-fast-260128", "doubao-seedance-2.5-nsfw-9", "doubao-seedance-2.5-nsfw-95-x",
		"sd2.5-速抛", "grok-imagine-video", "minimax-h3",
	} {
		_, ok = zexiModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestZexiSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformZexiSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformZexiSeedance))
	require.True(t, IsAllowedQuotaPlatform(PlatformZexiSeedance))
	require.True(t, IsSeedanceHubMemberPlatform(PlatformZexiSeedance))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformZexiSeedance, zexiTest25Nsfw95))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformZexiSeedance, zexiTest20Nsfw))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformZexiSeedance, "doubao-seedance-2.5"))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformZexiSeedance, "other"))
	// The model names look like the ones of the doubao platforms; those platforms keep their own.
	require.False(t, ppVideoModelBelongsToPlatform(PlatformByteDance, zexiTest25Nsfw95))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformXSNSeedance, zexiTest25Nsfw95))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformHuahuoSeedance, zexiTest25Nsfw95))
	require.False(t, ppVideoModelKnownForeignToPlatform(PlatformZexiSeedance, zexiTest25Nsfw95))

	require.Equal(t, []string{zexiTest25Nsfw95, zexiTest20Nsfw},
		filterPPVideoPublicModelsForPlatform(PlatformZexiSeedance, []string{zexiTest25Nsfw95, "other", "doubao-seedance-2.5", zexiTest20Nsfw}))
	require.Equal(t, zexiTest25Nsfw, PPVideoPublicModelName(PlatformZexiSeedance, "DOUBAO-SEEDANCE-2.5-NSFW"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformZexiSeedance, "other"))
}

// Account sync answers from the local table without any HTTP request.
func TestZexiSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformZexiSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, zexiModelNames(), models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformZexiSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestZexiSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformZexiSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformZexiSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformZexiSeedance, "zx-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer zx-key", headers.Get("Authorization"))
	require.Empty(t, headers.Get("Idempotency-Key"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformZexiSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformZexiSeedance))

	// queued / in_progress -> completed / failed / cancelled; the official format says succeeded.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","object":"video","status":"queued","progress":0}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed","progress":100,"video_url":"/v1/videos/task_1/content"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"succeeded"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"x","message":"boom"}}`, PPVideoTaskStatusFailed},
		{`{"id":"task_1","status":"cancelled"}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformZexiSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	failed, err := ParsePPVideoResponse(PlatformZexiSeedance, []byte(`{"id":"task_1","status":"failed","error":{"code":"x","message":"content rejected"}}`))
	require.NoError(t, err)
	require.Equal(t, "content rejected", failed.ErrorMessage)
	rejected, err := ParsePPVideoResponse(PlatformZexiSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", rejected.ErrorMessage)

	// Billing uses the requested seconds, never a length or usage the upstream reports, and a
	// missing usage is not turned into a zero.
	echoed, err := ParsePPVideoResponse(PlatformZexiSeedance, []byte(`{"id":"task_1","status":"completed","seconds":"9","usage":{"total_tokens":123}}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
	require.Zero(t, echoed.InputVideoDurationMilliseconds)
}

func TestZexiSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: zexiTest25Nsfw}

	raw := `{"id":"task_1","object":"video","status":"completed","progress":100,` +
		`"video_url":"https://zexitongxue.com/v1/videos/task_1/content","cost":2.9,"final_cost":2.9,"price":3}`
	parsed, err := ParsePPVideoResponse(PlatformZexiSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformZexiSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, zexiTest25Nsfw, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	for _, field := range []string{"cost", "final_cost", "price"} {
		require.False(t, gjson.GetBytes(normalized, field).Exists(), field)
	}
	require.NotContains(t, string(normalized), "zexitongxue")

	// A task that is still running never shows a result address, even if the upstream sends one.
	running := `{"id":"task_1","status":"in_progress","progress":30,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformZexiSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformZexiSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")
}

func TestZexiSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	const (
		a  = "https://example.com/a.png"
		b  = "https://example.com/b.png"
		v  = "https://example.com/v.mp4"
		au = "https://example.com/au.mp3"
	)
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "minimal request: the default resolution is sent, nothing else",
			body: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5}`,
			want: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"resolution":"720p"}`,
		},
		{
			name: "every field, media as content items with their roles",
			body: `{"model":"doubao-seedance-2.5-nsfw-95","prompt":"x","duration":"8","ratio":"9:16","resolution":"1080P",` +
				`"generate_audio":false,"seed":0,"images":["` + a + `","` + b + `"],"reference_videos":["` + v + `"],"audios":["` + au + `"]}`,
			want: `{"model":"doubao-seedance-2.5-nsfw-95","prompt":"x","duration":8,"resolution":"1080p","ratio":"9:16",` +
				`"generate_audio":false,"seed":0,"content":[` +
				`{"type":"image_url","role":"reference_image","image_url":{"url":"` + a + `"}},` +
				`{"type":"image_url","role":"reference_image","image_url":{"url":"` + b + `"}},` +
				`{"type":"video_url","role":"reference_video","video_url":{"url":"` + v + `"}},` +
				`{"type":"audio_url","role":"reference_audio","audio_url":{"url":"` + au + `"}}]}`,
		},
		{
			name: "the model name is matched ignoring case and answered with the table's spelling",
			body: `{"model":"DOUBAO-SEEDANCE-2.5-NSFW","prompt":"x","duration":30}`,
			want: `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":30,"resolution":"720p"}`,
		},
		{
			name: "4k on a Seedance 2.0 model, aspect_ratio as an alias of ratio, adaptive",
			body: `{"model":"doubao-seedance-2.0-nsfw-95","prompt":"x","duration":15,"resolution":"4k","aspect_ratio":"adaptive"}`,
			want: `{"model":"doubao-seedance-2.0-nsfw-95","prompt":"x","duration":15,"resolution":"4k","ratio":"adaptive"}`,
		},
		{
			name: "the audio switch is a boolean that may be true, and -1 is a valid (random) seed",
			body: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"generate_audio":true,"seed":-1}`,
			want: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"resolution":"720p","generate_audio":true,"seed":-1}`,
		},
		{
			name: "the largest seed",
			body: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"seed":9007199254740991}`,
			want: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"resolution":"720p","seed":9007199254740991}`,
		},
		{
			name: "first and last frame",
			body: `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"first_frame":"` + a + `","last_frame":"` + b + `"}`,
			want: `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"resolution":"720p","content":[` +
				`{"type":"image_url","role":"first_frame","image_url":{"url":"` + a + `"}},` +
				`{"type":"image_url","role":"last_frame","image_url":{"url":"` + b + `"}}]}`,
		},
		{
			name: "a first frame alone, written as an object, and end_frame as an alias of last_frame",
			body: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"first_frame":{"url":"` + a + `"},"end_frame":"` + b + `"}`,
			want: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"resolution":"720p","content":[` +
				`{"type":"image_url","role":"first_frame","image_url":{"url":"` + a + `"}},` +
				`{"type":"image_url","role":"last_frame","image_url":{"url":"` + b + `"}}]}`,
		},
		{
			name: "audio together with an image",
			body: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"image_urls":["` + a + `"],"audio_urls":["` + au + `"]}`,
			want: `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":5,"resolution":"720p","content":[` +
				`{"type":"image_url","role":"reference_image","image_url":{"url":"` + a + `"}},` +
				`{"type":"audio_url","role":"reference_audio","audio_url":{"url":"` + au + `"}}]}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestZexiSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ model, resolution string }{
		{zexiTest25Nsfw95, "720p"},
		{zexiTest25Nsfw, "1080p"},
		{zexiTest20Nsfw95, "4k"},
		{zexiTest20Nsfw, "480p"},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+tc.model+`","prompt":"x","duration":12,"resolution":"`+tc.resolution+`","images":["https://example.com/1.png"]}`))
		require.NoError(t, err, tc.model)
		require.Equal(t, tc.model, public.Model)
		require.Equal(t, tc.model, public.UpstreamModel)
		require.Equal(t, int64(12000), public.DurationMilliseconds)
		require.Equal(t, 1, public.VideoCount)
		require.Equal(t, tc.resolution, public.Resolution)
		require.True(t, public.HasImage)

		meta := PPVideoBillingMetadataFromRequest(PlatformZexiSeedance, prepared)
		require.Equal(t, PlatformZexiSeedance, meta.Platform)
		require.Equal(t, tc.model, meta.Model)
		require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
		require.Equal(t, 12, meta.RequestedDurationSeconds)
		require.Equal(t, tc.resolution, meta.VideoResolution)
	}

	// The default resolution is sent, so the task is billed at it.
	prepared, _, err := PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+zexiTest25Nsfw+`","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, "720p", gjson.GetBytes(prepared, "resolution").String())
	require.Equal(t, "720p", PPVideoBillingMetadataFromRequest(PlatformZexiSeedance, prepared).VideoResolution)

	// Frames count as an image input.
	_, public, err := PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+zexiTest25Nsfw+`","prompt":"x","duration":5,"first_frame":"https://example.com/a.png"}`))
	require.NoError(t, err)
	require.True(t, public.HasImage)
}

func TestZexiSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	v25 := `{"model":"doubao-seedance-2.5-nsfw","prompt":"x",`
	v20 := `{"model":"doubao-seedance-2.0-nsfw-95","prompt":"x",`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		// The generic Seedance platform claims every doubao-seedance-* name, so the request is stopped
		// before the normalizer, with the platform check's wording.
		{"the plain model of the site", `{"model":"doubao-seedance-2.5","prompt":"x","duration":5}`, "is not supported by platform"},
		{"missing prompt", `{"model":"doubao-seedance-2.5-nsfw","duration":5}`, "prompt is required"},
		{"missing duration", v25 + `"ratio":"16:9"}`, "duration is required"},
		{"2.5 below the minimum", v25 + `"duration":3}`, "from 4 to 30 seconds"},
		{"2.5 above the maximum", v25 + `"duration":31}`, "from 4 to 30 seconds"},
		{"2.0 above the maximum", v20 + `"duration":16}`, "from 4 to 15 seconds"},
		{"automatic duration is not offered", v25 + `"duration":-1}`, "from 4 to 30 seconds"},
		{"a fractional duration", v25 + `"duration":5.5}`, "integer"},
		{"4k on 2.5", v25 + `"duration":5,"resolution":"4k"}`, "resolution must be 480p, 720p, 1080p for this model"},
		{"unknown ratio", v25 + `"duration":5,"ratio":"2:1"}`, "ratio must be one of"},
		{"the upstream's own content array", v25 + `"duration":5,"content":[{"type":"image_url","image_url":{"url":` + ref + `}}]}`, "does not support content"},
		{"media", v25 + `"duration":5,"media":[]}`, "does not support media"},
		{"edit mode", v25 + `"duration":5,"omni_reference_task_type":"edit"}`, "does not support omni_reference_task_type"},
		{"output format", v25 + `"duration":5,"out_format":"mov"}`, "does not support out_format"},
		{"video_mode", v25 + `"duration":5,"video_mode":"extend_forward"}`, "does not support video_mode"},
		{"watermark", v25 + `"duration":5,"watermark":false}`, "does not support watermark"},
		{"callback", v25 + `"duration":5,"callback_url":"https://example.com/cb"}`, "does not support callback_url"},
		{"return_last_frame", v25 + `"duration":5,"return_last_frame":true}`, "does not support return_last_frame"},
		{"generate_audio as a string", v25 + `"duration":5,"generate_audio":"true"}`, "generate_audio must be a boolean"},
		{"seed as a string", v25 + `"duration":5,"seed":"1"}`, "seed must be -1 or an integer"},
		{"seed below -1", v25 + `"duration":5,"seed":-2}`, "seed must be -1 or an integer"},
		{"a fractional seed", v25 + `"duration":5,"seed":1.5}`, "seed must be -1 or an integer"},
		{"seed above the safe integer", v25 + `"duration":5,"seed":9007199254740992}`, "seed must be -1 or an integer"},
		{"assets on v1", v25 + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"two videos in one request", v25 + `"duration":5,"n":2}`, "one video per request"},
		{"2.5 too many images", v25 + `"duration":5,"images":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"2.0 too many images", v20 + `"duration":5,"images":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
		{"2.0 too many videos", v20 + `"duration":5,"videos":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference videos"},
		{"2.0 too many audios", v20 + `"duration":5,"images":[` + ref + `],"audios":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference audios"},
		{"audio without an image or video", v25 + `"duration":5,"audios":[` + ref + `]}`, "require at least one reference image or video"},
		{"two names for the same images", v25 + `"duration":5,"images":[` + ref + `],"image_urls":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", v25 + `"duration":5,"images":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"media on a private address", v25 + `"duration":5,"images":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
		{"a last frame alone", v25 + `"duration":5,"last_frame":` + ref + `}`, "last_frame requires first_frame"},
		{"frames with reference images", v25 + `"duration":5,"first_frame":` + ref + `,"images":[` + ref + `]}`, "cannot be combined with reference"},
		{"frames with a reference video", v25 + `"duration":5,"first_frame":` + ref + `,"videos":[` + ref + `]}`, "cannot be combined with reference"},
		{"plain http frame", v25 + `"duration":5,"first_frame":"http://example.com/a.png"}`, "must be a public HTTPS URL"},
		{"two names for the last frame", v25 + `"duration":5,"first_frame":` + ref + `,"end_frame":` + ref + `,"last_frame":` + ref + `}`, "only one of end_frame and last_frame"},
		{"another frame alias", v25 + `"duration":5,"first_frame_url":` + ref + `}`, "does not support first or last frames"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The upper limits themselves are accepted.
	_, _, err := PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, []byte(
		v25+`"duration":30,"images":[`+repeatedJijiuItems(ref, 30)+`],`+
			`"videos":[`+repeatedJijiuItems(ref, 10)+`],"audios":[`+repeatedJijiuItems(ref, 10)+`]}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, []byte(
		v20+`"duration":15,"resolution":"4k","images":[`+repeatedJijiuItems(ref, 9)+`],`+
			`"videos":[`+repeatedJijiuItems(ref, 3)+`],"audios":[`+repeatedJijiuItems(ref, 3)+`]}`))
	require.NoError(t, err)

	// Other platforms still refuse what only this one offers (the same request is fine without it).
	_, _, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-特价满血","prompt":"x","duration":5}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformXSNSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"seedance-2.5-特价满血","prompt":"x","duration":5,"generate_audio":true}`))
	require.ErrorContains(t, err, "does not support generate_audio")
}

func TestTranslatePPVideoUnifiedRequestToZexi(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformZexiSeedance, []byte(`{
		"model": "doubao-seedance-2.5-nsfw",
		"prompt": "@image1 在街道上与@image2 对话",
		"duration": 5,
		"resolution": "1080p",
		"ratio": "16:9",
		"generate_audio": true,
		"seed": 7,
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "image", "url": "https://cdn.example.com/other.jpg"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "doubao-seedance-2.5-nsfw",
		"prompt": "@image1 在街道上与@image2 对话",
		"duration": 5,
		"resolution": "1080p",
		"ratio": "16:9",
		"generate_audio": true,
		"seed": 7,
		"content": [
			{"type": "image_url", "role": "reference_image", "image_url": {"url": "https://cdn.example.com/person.jpg"}},
			{"type": "image_url", "role": "reference_image", "image_url": {"url": "https://cdn.example.com/other.jpg"}},
			{"type": "video_url", "role": "reference_video", "video_url": {"url": "https://cdn.example.com/motion.mp4"}},
			{"type": "audio_url", "role": "reference_audio", "audio_url": {"url": "https://cdn.example.com/voice.mp3"}}
		]
	}`, string(prepared))

	// Frames, with the audio switch turned off (false is different from leaving it out).
	translated, err = TranslatePPVideoUnifiedRequest(PlatformZexiSeedance, []byte(`{
		"model": "doubao-seedance-2.0-nsfw-95",
		"prompt": "花朵从含苞到盛开",
		"duration": 5,
		"generate_audio": false,
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/bud.png"},
			{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/bloom.png"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err = PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "doubao-seedance-2.0-nsfw-95",
		"prompt": "花朵从含苞到盛开",
		"duration": 5,
		"resolution": "720p",
		"generate_audio": false,
		"content": [
			{"type": "image_url", "role": "first_frame", "image_url": {"url": "https://cdn.example.com/bud.png"}},
			{"type": "image_url", "role": "last_frame", "image_url": {"url": "https://cdn.example.com/bloom.png"}}
		]
	}`, string(prepared))

	for _, tc := range []struct{ name, body, want string }{
		{"last frame alone", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"assets":[{"type":"image","role":"last_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
		{"frames with references", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"},{"type":"image","url":"https://cdn.example.com/b.png"}]}`, "cannot be combined"},
		{"no watermark", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"watermark":true}`, "watermark"},
		{"no callback", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"callback_url":"https://example.com/cb"}`, "callback_url"},
		{"no metadata", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"metadata":{"a":"b"}}`, "metadata"},
		{"no options", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"options":{"a":"b"}}`, "options"},
		{"a random seed is left out, not -1", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"seed":-1}`, "seed"},
		{"prompt is required", `{"model":"doubao-seedance-2.5-nsfw","duration":5}`, "prompt"},
		{"duration is required", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x"}`, "duration"},
		{"model is required", `{"prompt":"x","duration":5}`, "model"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformZexiSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}

	// Rules that live in the native normalizer apply to v2 requests as well.
	for _, tc := range []struct{ name, body, want string }{
		{"audio without an image or video", `{"model":"doubao-seedance-2.5-nsfw","prompt":"x","duration":5,"assets":[{"type":"audio","url":"https://cdn.example.com/a.mp3"}]}`, "at least one reference image or video"},
		{"unknown model", `{"model":"doubao-seedance-2.5","prompt":"x","duration":5}`, "is not supported by platform"},
		{"duration out of range", `{"model":"doubao-seedance-2.0-nsfw","prompt":"x","duration":16}`, "from 4 to 15 seconds"},
	} {
		translated, err := TranslatePPVideoUnifiedRequest(PlatformZexiSeedance, []byte(tc.body))
		require.NoError(t, err, tc.name)
		_, _, err = PreparePPVideoRequestBody(PlatformZexiSeedance, PPVideoOperationGeneric, translated)
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

// A unified request must behave exactly like the equivalent native request.
func TestUnifiedZexiRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformZexiSeedance,
		`{
			"model": "doubao-seedance-2.5-nsfw-95",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "480p",
			"ratio": "9:16",
			"generate_audio": true,
			"seed": 42,
			"assets": [
				{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
				{"type": "image", "url": "https://cdn.example.com/cat2.png"},
				{"type": "video", "url": "https://cdn.example.com/run.mp4"},
				{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
			]
		}`,
		`{
			"model": "doubao-seedance-2.5-nsfw-95",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "480p",
			"ratio": "9:16",
			"generate_audio": true,
			"seed": 42,
			"images": ["https://cdn.example.com/cat.png", "https://cdn.example.com/cat2.png"],
			"videos": ["https://cdn.example.com/run.mp4"],
			"audios": ["https://cdn.example.com/voice.mp3"]
		}`,
	)
	requireUnifiedMatchesNative(t, PlatformZexiSeedance,
		`{
			"model": "doubao-seedance-2.0-nsfw",
			"prompt": "花朵从含苞到盛开",
			"duration": 5,
			"assets": [
				{"type": "image", "role": "first_frame", "url": "https://cdn.example.com/bud.png"},
				{"type": "image", "role": "last_frame", "url": "https://cdn.example.com/bloom.png"}
			]
		}`,
		`{
			"model": "doubao-seedance-2.0-nsfw",
			"prompt": "花朵从含苞到盛开",
			"duration": 5,
			"first_frame": "https://cdn.example.com/bud.png",
			"last_frame": "https://cdn.example.com/bloom.png"
		}`,
	)
}

func TestZexiSeedanceCapabilitiesQuoteAndHubLines(t *testing.T) {
	t.Parallel()

	v25 := PPVideoCapabilitiesForModel(PlatformZexiSeedance, zexiTest25Nsfw95)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, v25.Duration)
	require.Equal(t, []string{"480p", "720p", "1080p"}, v25.Resolutions)
	require.Equal(t, 30, *v25.Assets.ReferenceImages)
	require.Equal(t, 10, *v25.Assets.ReferenceVideos)
	require.Equal(t, 10, *v25.Assets.ReferenceAudios)
	require.True(t, *v25.Assets.FirstLastFrame)
	require.True(t, *v25.GenerateAudio)
	require.False(t, *v25.Callback)
	require.Equal(t, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}, v25.Ratios)

	v20 := PPVideoCapabilitiesForModel(PlatformZexiSeedance, zexiTest20Nsfw)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 15}, v20.Duration)
	require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, v20.Resolutions)
	require.Equal(t, 9, *v20.Assets.ReferenceImages)
	require.Equal(t, 3, *v20.Assets.ReferenceVideos)
	require.Equal(t, 3, *v20.Assets.ReferenceAudios)
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformZexiSeedance, "doubao-seedance-2.5").Duration)

	// The price estimate enforces the same limits as the submission.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformZexiSeedance, zexiTest25Nsfw, "", 10)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformZexiSeedance, zexiTest20Nsfw95, "4k", 15)
	require.True(t, ok)
	require.Equal(t, "4k", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformZexiSeedance, zexiTest25Nsfw95, "1080p", 30)
	require.True(t, ok)
	require.Equal(t, "1080p", resolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{zexiTest25Nsfw, "", 3},
		{zexiTest25Nsfw, "", 31},
		{zexiTest25Nsfw, "4k", 10},
		{zexiTest20Nsfw, "", 16},
		{"doubao-seedance-2.5", "", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformZexiSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}

	// Every model of the table is one line of the aggregate group, tag ZX, billed per second,
	// and the line reports the platform's own limits.
	wantNames := map[string]string{
		zexiTest25Nsfw95: "Seedance2.5--480p/720p/1080p--nsfw-95--按秒--ZX",
		zexiTest25Nsfw:   "Seedance2.5--480p/720p/1080p--nsfw--按秒--ZX",
		zexiTest20Nsfw95: "Seedance2.0--480p/720p/1080p/4k--nsfw-95--按秒--ZX",
		zexiTest20Nsfw:   "Seedance2.0--480p/720p/1080p/4k--nsfw--按秒--ZX",
	}
	count := 0
	for _, line := range SeedanceHubLines() {
		if line.Platform != PlatformZexiSeedance {
			continue
		}
		count++
		spec, found := zexiModelSpecFor(line.Model)
		require.True(t, found, line.Name)
		require.Equal(t, spec.Name, line.Model, line.Name)
		require.Equal(t, wantNames[spec.Name], line.Name)
		require.Equal(t, BillingModeVideo, line.Billing, line.Name)
		byUpstream, found := SeedanceHubLineByName(spec.Name)
		require.True(t, found, spec.Name)
		require.Equal(t, line.Name, byUpstream.Name)
		require.Equal(t, PPVideoCapabilitiesForModel(PlatformZexiSeedance, spec.Name).Duration,
			PPVideoCapabilitiesForModel(PlatformSeedanceHub, line.Name).Duration)

		_, rewritten, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + line.Name + `","prompt":"x","duration":5}`))
		require.NoError(t, err)
		require.Equal(t, spec.Name, gjson.GetBytes(rewritten, "model").String())
	}
	require.Equal(t, len(zexiSeedanceModels), count)
}
