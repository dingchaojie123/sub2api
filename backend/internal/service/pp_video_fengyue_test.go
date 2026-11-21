package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	// fengyueTestModel is the upstream name (the only one ever sent upstream), fengyueTestPublic
	// the name clients see.
	fengyueTestModel   = "sd2.5pp"
	fengyueTestPublic  = "Seedance2.5pp"
	fengyueTestHubLine = "Seedance 2.5--720p--按次--FY"
)

func TestFengyueSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Len(t, fengyueSeedanceModels, 1)
	spec, ok := fengyueModelSpecFor("SD2.5PP")
	require.True(t, ok)
	require.Equal(t, fengyueTestModel, spec.Name)
	require.Equal(t, fengyueTestPublic, spec.PublicName)
	require.Equal(t, fengyueTestPublic, spec.publicName())
	// 5-30 whole seconds as one model: the upstream's two length bands are not modelled here.
	require.Equal(t, 5, spec.MinSeconds)
	require.Equal(t, 30, spec.MaxSeconds)
	// 720p only: the upstream also takes 1080p, which is not sold here (loosening is compatible).
	require.Equal(t, []string{"720p"}, spec.Resolutions)
	require.Equal(t, "720p", spec.defaultResolution())
	require.True(t, spec.ForwardResolution)
	require.Equal(t, 30, spec.MaxImages)
	require.Equal(t, 10, spec.MaxVideos)
	require.Equal(t, 10, spec.MaxAudios)
	require.Equal(t, 50, spec.MaxReferenceTotal)
	require.False(t, spec.FirstLastFrame)
	// Public name first, so a channel price configured under it wins; the upstream name still works.
	require.Equal(t, []string{fengyueTestPublic, fengyueTestModel}, ppVideoFengyueModelAliases("sd2.5pp"))
	require.Equal(t, []string{fengyueTestPublic, fengyueTestModel}, ppVideoFengyueModelAliases("SEEDANCE2.5PP"))
	require.Equal(t, []string{fengyueTestPublic, fengyueTestModel}, ppVideoPricingModelAliases(PlatformFengyueSeedance, "SD2.5PP"))

	for _, model := range []string{"", "sd2.5", "sd2.5pp-fast", "other"} {
		_, ok = fengyueModelSpecFor(model)
		require.False(t, ok, model)
	}
}

func TestFengyueSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformFengyueSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformFengyueSeedance))

	require.True(t, ppVideoModelBelongsToPlatform(PlatformFengyueSeedance, fengyueTestModel))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformFengyueSeedance, "other"))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformFengyueSeedance, fengyueTestPublic))
	// A whitelist saved under either name is listed once, under the public name.
	require.Equal(t, []string{fengyueTestPublic}, filterPPVideoPublicModelsForPlatform(PlatformFengyueSeedance, []string{fengyueTestModel, fengyueTestPublic, "other"}))
	require.Equal(t, fengyueTestPublic, PPVideoPublicModelName(PlatformFengyueSeedance, "SD2.5PP"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformFengyueSeedance, "other"))
}

// Account sync answers from the local table without any HTTP request.
func TestFengyueSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformFengyueSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	// The synced list holds the names clients see.
	require.Equal(t, []string{fengyueTestPublic}, models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformFengyueSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestFengyueSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformFengyueSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformFengyueSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformFengyueSeedance, "fengyue-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer fengyue-key", headers.Get("Authorization"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformFengyueSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformFengyueSeedance))

	// Documented flow: queued / in_progress -> completed / failed.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","object":"video","status":"queued"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed","progress":100}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"x","message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformFengyueSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}

	failed, err := ParsePPVideoResponse(PlatformFengyueSeedance, []byte(`{"id":"task_1","status":"failed","error":{"code":"x","message":"素材不合规"}}`))
	require.NoError(t, err)
	require.Equal(t, "素材不合规", failed.ErrorMessage)
	rejected, err := ParsePPVideoResponse(PlatformFengyueSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", rejected.ErrorMessage)

	// Billing uses the requested seconds, never the length the upstream echoes.
	echoed, err := ParsePPVideoResponse(PlatformFengyueSeedance, []byte(`{"id":"task_1","status":"completed","seconds":"9"}`))
	require.NoError(t, err)
	require.Zero(t, echoed.GeneratedDurationMilliseconds)
}

func TestFengyueSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: "SD2.5PP"}

	raw := `{"id":"task_1","object":"video","status":"completed","progress":100,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err := ParsePPVideoResponse(PlatformFengyueSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformFengyueSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	// The client sees the public name, never the upstream one.
	require.Equal(t, fengyueTestPublic, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "url").String())
	require.NotContains(t, string(normalized), "cdn.example.com")

	// A completed task without any address still gets this gateway's content address.
	bare := `{"id":"task_1","object":"video","status":"completed","progress":100}`
	parsed, err = ParsePPVideoResponse(PlatformFengyueSeedance, []byte(bare))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformFengyueSeedance, []byte(bare), public, parsed)
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())

	// A running task never shows an address, even when the upstream already carries one.
	running := `{"id":"task_1","status":"in_progress","progress":30,"video_url":"https://cdn.example.com/result.mp4"}`
	parsed, err = ParsePPVideoResponse(PlatformFengyueSeedance, []byte(running))
	require.NoError(t, err)
	normalized = NormalizePPVideoPublicResponse(PlatformFengyueSeedance, []byte(running), public, parsed)
	require.Equal(t, "processing", gjson.GetBytes(normalized, "status").String())
	require.False(t, gjson.GetBytes(normalized, "video_url").Exists())
	require.NotContains(t, string(normalized), "cdn.example.com")
}

func TestFengyueSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "the documented example: the upstream's own references array",
			body: `{"model":"sd2.5pp","prompt":"木桌上的红球缓慢旋转，镜头推近","seconds":30,"resolution":"720p","aspect_ratio":"9:16",` +
				`"mode":"omni_reference","references":[` +
				`{"kind":"image","data":"https://example.com/person.png"},` +
				`{"kind":"image","data":"https://example.com/scene.png"},` +
				`{"kind":"video","data":"https://example.com/motion.mp4"}]}`,
			want: `{"model":"sd2.5pp","prompt":"木桌上的红球缓慢旋转，镜头推近","seconds":30,"resolution":"720p","aspect_ratio":"9:16",` +
				`"mode":"omni_reference","references":[` +
				`{"kind":"image","data":"https://example.com/person.png"},` +
				`{"kind":"image","data":"https://example.com/scene.png"},` +
				`{"kind":"video","data":"https://example.com/motion.mp4"}]}`,
		},
		{
			name: "the resolution is filled in, the mode is added and the ratio is optional",
			body: `{"model":"sd2.5pp","prompt":"x","duration":5,"image_urls":["https://example.com/1.png"]}`,
			want: `{"model":"sd2.5pp","prompt":"x","seconds":5,"resolution":"720p","mode":"omni_reference",` +
				`"references":[{"kind":"image","data":"https://example.com/1.png"}]}`,
		},
		{
			name: "aliases are folded into references: images, then videos, then audios",
			body: `{"model":"sd2.5pp","prompt":"x","seconds":"15","ratio":"21:9",` +
				`"audios":["https://example.com/1.mp3"],"videos":["https://example.com/1.mp4"],"images":["https://example.com/1.png"]}`,
			want: `{"model":"sd2.5pp","prompt":"x","seconds":15,"resolution":"720p","aspect_ratio":"21:9","mode":"omni_reference","references":[` +
				`{"kind":"image","data":"https://example.com/1.png"},` +
				`{"kind":"video","data":"https://example.com/1.mp4"},` +
				`{"kind":"audio","data":"https://example.com/1.mp3"}]}`,
		},
		{
			name: "audio may accompany an image",
			body: `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":[` +
				`{"kind":"audio","data":"https://example.com/1.mp3"},{"kind":"image","data":"https://example.com/1.png"}]}`,
			want: `{"model":"sd2.5pp","prompt":"x","seconds":5,"resolution":"720p","mode":"omni_reference","references":[` +
				`{"kind":"image","data":"https://example.com/1.png"},{"kind":"audio","data":"https://example.com/1.mp3"}]}`,
		},
		{
			name: "the public name is accepted and the upstream always receives its own name",
			body: `{"model":"Seedance2.5pp","prompt":"x","duration":8,"image_urls":["https://example.com/1.png"]}`,
			want: `{"model":"sd2.5pp","prompt":"x","seconds":8,"resolution":"720p","mode":"omni_reference",` +
				`"references":[{"kind":"image","data":"https://example.com/1.png"}]}`,
		},
		{
			name: "the model name is matched ignoring case and answered with the table's spelling",
			body: `{"model":"SD2.5PP","prompt":"x","duration":16,"image_urls":["https://example.com/1.png"]}`,
			want: `{"model":"sd2.5pp","prompt":"x","seconds":16,"resolution":"720p","mode":"omni_reference",` +
				`"references":[{"kind":"image","data":"https://example.com/1.png"}]}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformFengyueSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestFengyueSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	prepared, public, err := PreparePPVideoRequestBody(PlatformFengyueSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"sd2.5pp","prompt":"x","duration":12,"resolution":"720p","image_urls":["https://example.com/1.png"]}`))
	require.NoError(t, err)
	require.Equal(t, fengyueTestPublic, public.Model)
	require.Equal(t, fengyueTestModel, public.UpstreamModel)
	require.Equal(t, int64(12000), public.DurationMilliseconds)
	require.Equal(t, 1, public.VideoCount)
	require.Equal(t, "720p", public.Resolution)
	require.True(t, public.HasImage)

	meta := PPVideoBillingMetadataFromRequest(PlatformFengyueSeedance, prepared)
	require.Equal(t, PlatformFengyueSeedance, meta.Platform)
	require.Equal(t, fengyueTestModel, meta.Model)
	require.Equal(t, int64(12000), meta.RequestedDurationMilliseconds)
	require.Equal(t, "720p", meta.VideoResolution)
}

func TestFengyueSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"https://example.com/1.png"`
	base := `{"model":"sd2.5pp","prompt":"x","image_urls":[` + ref + `],`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x","seconds":5,"image_urls":[` + ref + `]}`, "model is required"},
		// Names that start with "seedance" belong to the legacy Seedance platform, so the shared
		// foreign-model check answers first; any other unknown name reaches the normalizer.
		{"another seedance model", `{"model":"seedance-2.5","prompt":"x","seconds":5}`, "is not supported by platform"},
		{"unknown model", `{"model":"other-model","prompt":"x","seconds":5}`, "is not available"},
		{"missing prompt", `{"model":"sd2.5pp","seconds":5,"image_urls":[` + ref + `]}`, "prompt is required"},
		{"missing duration", base + `"ratio":"16:9"}`, "duration is required"},
		{"below the minimum", base + `"seconds":4}`, "from 5 to 30 seconds"},
		{"above the maximum", base + `"seconds":31}`, "from 5 to 30 seconds"},
		{"a fraction", base + `"seconds":5.5}`, "from 5 to 30 seconds"},
		{"4k", base + `"seconds":5,"resolution":"4k"}`, "resolution must be 720p for this model"},
		{"480p", base + `"seconds":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		// The upstream takes 1080p at the same price, but this gateway sells 720p only.
		{"1080p", base + `"seconds":5,"resolution":"1080p"}`, "resolution must be 720p for this model"},
		{"unknown ratio", base + `"seconds":5,"ratio":"5:4"}`, "ratio must be one of"},
		{"another mode", base + `"seconds":5,"mode":"single_reference"}`, "mode must be omni_reference"},
		{"no reference at all", `{"model":"sd2.5pp","prompt":"x","seconds":5}`, "at least one reference image or video"},
		{"audio only, aliases", `{"model":"sd2.5pp","prompt":"x","seconds":5,"audio_urls":["https://example.com/1.mp3"]}`, "at least one reference image or video"},
		{"audio only, references", `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":[{"kind":"audio","data":"https://example.com/1.mp3"}]}`, "at least one reference image or video"},
		{"empty references", `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":[]}`, "at least one reference image or video"},
		{"references is not an array", `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":"x"}`, "references must be an array"},
		{"reference is not an object", `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":["https://example.com/1.png"]}`, "must be an object with kind and data"},
		{"reference with another key", `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":[{"kind":"image","data":` + ref + `,"name":"a"}]}`, "accepts only kind and data"},
		{"unknown kind", `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":[{"kind":"gif","data":` + ref + `}]}`, "kind must be image, video or audio"},
		{"inline data", `{"model":"sd2.5pp","prompt":"x","seconds":5,"references":[{"kind":"image","data":"data:image/png;base64,AAAA"}]}`, "must be a public HTTPS URL"},
		{"references and aliases together", base + `"seconds":5,"references":[{"kind":"image","data":` + ref + `}]}`, "not both"},
		{"first frame", base + `"seconds":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"seed", base + `"seconds":5,"seed":1}`, "does not support seed"},
		{"generate_audio", base + `"seconds":5,"generate_audio":true}`, "does not support generate_audio"},
		{"watermark", base + `"seconds":5,"watermark":false}`, "does not support watermark"},
		{"assets on v1", base + `"seconds":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"more than one video", base + `"seconds":5,"n":2}`, "one video per request"},
		{"too many images", `{"model":"sd2.5pp","prompt":"x","seconds":5,"image_urls":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many videos", base + `"seconds":5,"video_urls":[` + repeatedJijiuItems(`"https://example.com/1.mp4"`, 11) + `]}`, "at most 10 reference videos"},
		{"too many audios", base + `"seconds":5,"audio_urls":[` + repeatedJijiuItems(`"https://example.com/1.mp3"`, 11) + `]}`, "at most 10 reference audios"},
		{"two names for the same images", `{"model":"sd2.5pp","prompt":"x","seconds":5,"image_urls":[` + ref + `],"images":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", `{"model":"sd2.5pp","prompt":"x","seconds":5,"image_urls":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"media on a private address", `{"model":"sd2.5pp","prompt":"x","seconds":5,"image_urls":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformFengyueSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The upper limits themselves are accepted: 30 images, 10 videos and 10 audios (50 items).
	_, _, err := PreparePPVideoRequestBody(PlatformFengyueSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"sd2.5pp","prompt":"x","seconds":30,"resolution":"720p","ratio":"21:9","image_urls":[`+repeatedJijiuItems(ref, 30)+
			`],"video_urls":[`+repeatedJijiuItems(`"https://example.com/1.mp4"`, 10)+
			`],"audio_urls":[`+repeatedJijiuItems(`"https://example.com/1.mp3"`, 10)+`]}`))
	require.NoError(t, err)
}

func TestTranslatePPVideoUnifiedRequestToFengyue(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformFengyueSeedance, []byte(`{
		"model": "sd2.5pp",
		"prompt": "@Image1 在街道上与@Image2 对话",
		"duration": 5,
		"resolution": "720p",
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "image", "url": "https://cdn.example.com/other.jpg"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformFengyueSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "sd2.5pp",
		"prompt": "@Image1 在街道上与@Image2 对话",
		"seconds": 5,
		"aspect_ratio": "16:9",
		"resolution": "720p",
		"mode": "omni_reference",
		"references": [
			{"kind": "image", "data": "https://cdn.example.com/person.jpg"},
			{"kind": "image", "data": "https://cdn.example.com/other.jpg"},
			{"kind": "video", "data": "https://cdn.example.com/motion.mp4"},
			{"kind": "audio", "data": "https://cdn.example.com/voice.mp3"}
		]
	}`, string(prepared))

	// Audio alone is translated and then refused by the platform's reference rule.
	translated, err = TranslatePPVideoUnifiedRequest(PlatformFengyueSeedance, []byte(`{
		"model": "sd2.5pp", "prompt": "x", "duration": 5,
		"assets": [{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}]
	}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformFengyueSeedance, PPVideoOperationGeneric, translated)
	require.ErrorContains(t, err, "at least one reference image or video")

	// Fields this platform does not take are refused by the translator.
	for _, tc := range []struct{ name, body, want string }{
		{"missing model", `{"prompt":"x","duration":5}`, "model"},
		{"missing prompt", `{"model":"sd2.5pp","duration":5}`, "prompt"},
		{"missing duration", `{"model":"sd2.5pp","prompt":"x"}`, "duration"},
		{"generate_audio", `{"model":"sd2.5pp","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"seed", `{"model":"sd2.5pp","prompt":"x","duration":5,"seed":1}`, "seed"},
		{"first frame", `{"model":"sd2.5pp","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
	} {
		_, err = TranslatePPVideoUnifiedRequest(PlatformFengyueSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

func TestFengyueSeedanceCapabilities(t *testing.T) {
	t.Parallel()

	capabilities := PPVideoCapabilitiesForModel(PlatformFengyueSeedance, fengyueTestModel)
	require.Contains(t, capabilities.Protocols, PPVideoProtocolV1)
	require.Contains(t, capabilities.Protocols, PPVideoProtocolV2)
	require.NotNil(t, capabilities.Duration)
	require.Equal(t, 5, capabilities.Duration.Min)
	require.Equal(t, 30, capabilities.Duration.Max)
	require.Equal(t, []string{"720p"}, capabilities.Resolutions)
	require.Equal(t, fengyueAspectRatios, capabilities.Ratios)
	require.NotNil(t, capabilities.Assets)
	require.Equal(t, 30, *capabilities.Assets.ReferenceImages)
	require.Equal(t, 10, *capabilities.Assets.ReferenceVideos)
	require.Equal(t, 10, *capabilities.Assets.ReferenceAudios)
	require.False(t, *capabilities.Assets.FirstLastFrame)
}

func TestFengyueSeedanceHubLine(t *testing.T) {
	t.Parallel()

	line, ok := SeedanceHubLineByName(fengyueTestHubLine)
	require.True(t, ok)
	require.Equal(t, PlatformFengyueSeedance, line.Platform)
	// The hub rewrites the request to the public name, like the jijiu lines; the platform pipeline
	// then maps it to the upstream name.
	require.Equal(t, fengyueTestPublic, line.Model)
	// One flat per-request channel price for every length (5-15 s and 16-30 s alike).
	require.Equal(t, BillingModePerRequest, line.Billing)
	require.True(t, IsSeedanceHubMemberPlatform(PlatformFengyueSeedance))

	// The platform's own name keeps working inside the hub group.
	byModel, ok := SeedanceHubLineByName(fengyueTestModel)
	require.True(t, ok)
	require.Equal(t, line.Name, byModel.Name)

	resolved, body, err := ResolveSeedanceHubRequest([]byte(`{"model":"` + fengyueTestHubLine + `","prompt":"x","duration":5}`))
	require.NoError(t, err)
	require.Equal(t, line.Name, resolved.Name)
	require.Equal(t, fengyueTestPublic, gjson.GetBytes(body, "model").String())

	// The upstream name keeps working inside the hub group as well.
	upstreamNamed, ok := SeedanceHubLineByName(fengyueTestModel)
	require.True(t, ok)
	require.Equal(t, line.Name, upstreamNamed.Name)

	// The estimate for the line uses the platform's table.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, fengyueTestHubLine, "", 8)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	_, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, fengyueTestHubLine, "", 31)
	require.False(t, ok)
}

func TestFengyueSeedanceQuoteSpec(t *testing.T) {
	t.Parallel()

	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformFengyueSeedance, fengyueTestModel, "", 8)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformFengyueSeedance, fengyueTestModel, "720p", 30)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)

	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{fengyueTestModel, "480p", 8},
		{fengyueTestModel, "1080p", 8},
		{fengyueTestModel, "", 4},
		{fengyueTestModel, "", 31},
		{"other", "", 8},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformFengyueSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%s %s %d", tc.model, tc.resolution, tc.seconds)
	}
}
