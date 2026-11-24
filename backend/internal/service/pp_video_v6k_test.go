package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	// Upstream ids (the exact spelling) and the public names clients see.
	v6kTestPro        = "官方按条SD2.0"
	v6kTestFast       = "官方sd2.0-fast"
	v6kTestMini       = "SD2.0-mini"
	v6kTestProPublic  = "Seedance2.0-V6K"
	v6kTestFastPublic = "Seedance2.0-V6K-fast"
	v6kTestMiniPublic = "Seedance2.0-V6K-mini"
	// 活动 (2026-11-23): the upstream id is spelled exactly as the operator wrote it.
	v6kTestActivity       = "seedanco2.5-720"
	v6kTestActivityPublic = "Seedance2.5-V6K-720"
	v6kTestMedia          = "https://example.com/1.png"
)

func TestV6KSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{v6kTestProPublic, v6kTestFastPublic, v6kTestMiniPublic, v6kTestActivityPublic}, v6kModelNames())
	require.Len(t, v6kSeedanceModels, 4)

	// Seconds and reference limits per model: the three Seedance 2.0 models take 4-15 s, 9 images,
	// no videos and 3 audios; the 活动 model takes 4-30 s, 30 images, 10 videos and 10 audios.
	type limits struct {
		maxSeconds int
		refs       []int
	}
	wantLimits := map[string]limits{
		v6kTestPro:      {15, []int{9, 0, 3}},
		v6kTestFast:     {15, []int{9, 0, 3}},
		v6kTestMini:     {15, []int{9, 0, 3}},
		v6kTestActivity: {30, []int{30, 10, 10}},
	}

	seen := make(map[string]string)
	for _, spec := range v6kSeedanceModels {
		for _, name := range []string{spec.Name, spec.publicName()} {
			key := strings.ToLower(name)
			owner, dup := seen[key]
			require.False(t, dup, "name %q is used by %q and %q", name, owner, spec.Name)
			seen[key] = spec.Name
		}
		require.Contains(t, strings.ToLower(spec.publicName()), "seedance", spec.Name)
		require.NotEqual(t, spec.Name, spec.publicName(), spec.Name)
		require.True(t, spec.ForwardResolution, spec.Name)
		require.False(t, spec.FirstLastFrame, spec.Name)
		require.Equal(t, "720p", spec.defaultResolution(), spec.Name)
		require.Equal(t, "images", spec.imageField(), spec.Name)
		require.Equal(t, "videos", spec.videoField(), spec.Name)
		require.Equal(t, "audios", spec.audioField(), spec.Name)
		require.Equal(t, "ratio", spec.ratioField(), spec.Name)
		require.Equal(t, 4, spec.MinSeconds, spec.Name)
		want, ok := wantLimits[spec.Name]
		require.True(t, ok, "no expected limits for %q", spec.Name)
		require.Equal(t, want.maxSeconds, spec.MaxSeconds, spec.Name)
		require.Equal(t, want.refs, []int{spec.MaxImages, spec.MaxVideos, spec.MaxAudios}, spec.Name)
	}

	activity, ok := v6kModelSpecFor(v6kTestActivityPublic)
	require.True(t, ok)
	require.Equal(t, v6kTestActivity, activity.Name)
	require.Equal(t, []string{"720p"}, activity.Resolutions)

	pro, ok := v6kModelSpecFor(strings.ToLower(v6kTestPro))
	require.True(t, ok)
	require.Equal(t, v6kTestPro, pro.Name)
	require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, pro.Resolutions)
	fast, ok := v6kModelSpecFor(v6kTestFastPublic)
	require.True(t, ok)
	require.Equal(t, v6kTestFast, fast.Name)
	require.Equal(t, []string{"480p", "720p"}, fast.Resolutions)
	mini, ok := v6kModelSpecFor(v6kTestMini)
	require.True(t, ok)
	require.Equal(t, []string{"480p", "720p"}, mini.Resolutions)
}

func TestV6KSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformV6KSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformV6KSeedance))
	require.True(t, IsAllowedQuotaPlatform(PlatformV6KSeedance))
	require.True(t, IsSeedanceHubMemberPlatform(PlatformV6KSeedance))
	require.True(t, PPVideoGroupServesPlatform(PlatformSeedanceHub, PlatformV6KSeedance))

	for _, model := range []string{v6kTestPro, v6kTestFast, v6kTestMini, v6kTestActivity, v6kTestProPublic, v6kTestFastPublic, v6kTestMiniPublic, v6kTestActivityPublic} {
		require.True(t, ppVideoModelBelongsToPlatform(PlatformV6KSeedance, model), model)
		require.False(t, ppVideoModelKnownForeignToPlatform(PlatformV6KSeedance, model), model)
	}
	require.False(t, ppVideoModelBelongsToPlatform(PlatformV6KSeedance, "other"))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformXMSeedance, v6kTestProPublic))

	require.Equal(t, []string{v6kTestProPublic, v6kTestFastPublic},
		filterPPVideoPublicModelsForPlatform(PlatformV6KSeedance, []string{v6kTestPro, v6kTestProPublic, "other", v6kTestFast}))
	require.Equal(t, v6kTestMiniPublic, PPVideoPublicModelName(PlatformV6KSeedance, v6kTestMini))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformV6KSeedance, "other"))
}

func TestV6KSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformV6KSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, v6kModelNames(), models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformV6KSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestV6KSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformV6KSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformV6KSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformV6KSeedance, "v6k-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer v6k-key", headers.Get("Authorization"))
	// The supplier says a lost answer must be reconciled, never re-sent: no key is sent.
	require.Empty(t, headers.Get("Idempotency-Key"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformV6KSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformV6KSeedance))

	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","status":"queued"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"code":"video_generation_failed","message":"Video generation failed"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformV6KSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, "task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}
}

func TestV6KSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: v6kTestPro}
	raw := `{"id":"task_1","model":"` + v6kTestPro + `","status":"completed","progress":100,` +
		`"video_url":"https://www.v6k.top/v1/videos/task_1/content","cost":4.29,"final_cost":4.29,"price":5,"usage":{"total_tokens":1}}`
	parsed, err := ParsePPVideoResponse(PlatformV6KSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformV6KSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, v6kTestProPublic, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	for _, field := range []string{"cost", "final_cost", "price"} {
		require.False(t, gjson.GetBytes(normalized, field).Exists(), field)
	}
	require.NotContains(t, string(normalized), "v6k.top")
	require.NotContains(t, string(normalized), "官方")
}

func TestV6KSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	const (
		a  = "https://example.com/a.png"
		au = "https://example.com/au.mp3"
	)
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "the resolution may be left out: 720p is sent",
			body: `{"model":"` + v6kTestProPublic + `","prompt":"x","duration":6}`,
			want: `{"model":"` + v6kTestPro + `","prompt":"x","seconds":6,"resolution":"720p"}`,
		},
		{
			name: "the upstream id is accepted; seconds as a string; aspect_ratio is the ratio",
			body: `{"model":"` + v6kTestFast + `","prompt":"x","seconds":"6","aspect_ratio":"9:16","resolution":"480p"}`,
			want: `{"model":"` + v6kTestFast + `","prompt":"x","seconds":6,"resolution":"480p","ratio":"9:16"}`,
		},
		{
			name: "4K is sent as 4k on the first model",
			body: `{"model":"` + v6kTestProPublic + `","prompt":"x","duration":15,"resolution":"4K","ratio":"21:9"}`,
			want: `{"model":"` + v6kTestPro + `","prompt":"x","seconds":15,"resolution":"4k","ratio":"21:9"}`,
		},
		{
			name: "omni_reference_task_type reference is accepted and not forwarded; equal ratio aliases agree",
			body: `{"model":"` + v6kTestMiniPublic + `","prompt":"x","duration":4,"omni_reference_task_type":"reference","ratio":"1:1","aspect_ratio":"1:1"}`,
			want: `{"model":"` + v6kTestMini + `","prompt":"x","seconds":4,"resolution":"720p","ratio":"1:1"}`,
		},
		{
			name: "references are sent under the upstream's array names, in order",
			body: `{"model":"` + v6kTestProPublic + `","prompt":"x","duration":5,"images":["` + a + `","` + a + `?2"],"audio_urls":["` + au + `"]}`,
			want: `{"model":"` + v6kTestPro + `","prompt":"x","seconds":5,"resolution":"720p","images":["` + a + `","` + a + `?2"],"audios":["` + au + `"]}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestV6KSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ model, upstream, resolution string }{
		{v6kTestProPublic, v6kTestPro, "1080p"},
		{v6kTestPro, v6kTestPro, "4k"},
		{v6kTestFastPublic, v6kTestFast, "480p"},
		{v6kTestMiniPublic, v6kTestMini, "720p"},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+tc.model+`","prompt":"x","duration":12,"resolution":"`+tc.resolution+`","images":["`+v6kTestMedia+`"]}`))
		require.NoError(t, err, tc.model)
		name, ok := ppVideoV6KPublicModel(tc.upstream)
		require.True(t, ok, tc.upstream)
		require.Equal(t, name, public.Model)
		require.Equal(t, tc.upstream, public.UpstreamModel)
		require.Equal(t, tc.upstream, gjson.GetBytes(prepared, "model").String())
		require.Equal(t, int64(12000), public.DurationMilliseconds)
		require.Equal(t, 1, public.VideoCount)
		require.Equal(t, tc.resolution, public.Resolution)
		require.True(t, public.HasImage)

		meta := PPVideoBillingMetadataFromRequest(PlatformV6KSeedance, prepared)
		require.Equal(t, PlatformV6KSeedance, meta.Platform)
		require.Equal(t, tc.upstream, meta.Model)
		require.Equal(t, 12, meta.RequestedDurationSeconds)
		require.Equal(t, tc.resolution, meta.VideoResolution)
	}
}

func TestV6KSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"` + v6kTestMedia + `"`
	pro := `{"model":"` + v6kTestProPublic + `","prompt":"x",`
	fast := `{"model":"` + v6kTestFastPublic + `","prompt":"x",`
	for _, tc := range []struct{ name, body, want string }{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		{"unknown model", `{"model":"nope","prompt":"x","duration":5}`, "is not available"},
		{"missing prompt", `{"model":"` + v6kTestProPublic + `","duration":5}`, "prompt is required"},
		{"missing duration", pro + `"resolution":"720p"}`, "duration is required"},
		{"below 4 s", pro + `"duration":3}`, "from 4 to 15 seconds"},
		{"above 15 s", pro + `"duration":16}`, "from 4 to 15 seconds"},
		{"automatic duration", pro + `"duration":-1}`, "from 4 to 15 seconds"},
		{"fractional duration", pro + `"duration":5.5}`, "integer"},
		{"ratio outside the set", pro + `"duration":5,"ratio":"5:4"}`, "ratio must be one of"},
		{"ratio that is not a string", pro + `"duration":5,"ratio":1}`, "must be a string"},
		{"ratio aliases disagree", pro + `"duration":5,"ratio":"16:9","aspect_ratio":"9:16"}`, "must have the same value"},
		{"fast refuses 1080p", fast + `"duration":5,"resolution":"1080p"}`, "resolution must be 480p, 720p"},
		{"fast refuses 4k", fast + `"duration":5,"resolution":"4k"}`, "resolution must be 480p, 720p"},
		{"size is not offered", pro + `"duration":5,"size":"1280x720"}`, "does not support size"},
		{"edit needs a reference video", pro + `"duration":5,"omni_reference_task_type":"edit"}`, "omni_reference_task_type"},
		{"extend needs a reference video", pro + `"duration":5,"omni_reference_task_type":"extend"}`, "omni_reference_task_type"},
		{"reference videos are not supported", pro + `"duration":5,"videos":[` + ref + `]}`, "does not support reference videos"},
		{"too many images", pro + `"duration":5,"images":[` + repeatedJijiuItems(ref, 10) + `]}`, "at most 9 reference images"},
		{"too many audios", pro + `"duration":5,"audios":[` + repeatedJijiuItems(ref, 4) + `]}`, "at most 3 reference audios"},
		{"first frame", pro + `"duration":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"seed", pro + `"duration":5,"seed":1}`, "does not support seed"},
		{"generate_audio", pro + `"duration":5,"generate_audio":true}`, "does not support generate_audio"},
		{"assets on v1", pro + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"plain http media", pro + `"duration":5,"images":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The documented upper limits are accepted.
	body := pro + `"duration":15,"resolution":"4k","images":[` + repeatedJijiuItems(ref, 9) + `],"audios":[` + repeatedJijiuItems(ref, 3) + `]}`
	_, _, err := PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, []byte(body))
	require.NoError(t, err)
}

// The 活动 model (seedanco2.5-720): 720p only, 4-30 s, 30 images / 10 videos / 10 audios.
func TestV6KSeedanceActivityModel(t *testing.T) {
	t.Parallel()

	const ref = `"` + v6kTestMedia + `"`
	head := `{"model":"` + v6kTestActivityPublic + `","prompt":"x",`

	// The upstream id is the only name sent upstream; the resolution defaults to 720p.
	prepared, public, err := PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, []byte(head+`"duration":4}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"`+v6kTestActivity+`","prompt":"x","seconds":4,"resolution":"720p"}`, string(prepared))
	require.Equal(t, v6kTestActivityPublic, public.Model)
	require.Equal(t, v6kTestActivity, public.UpstreamModel)

	// Either name is accepted, and the documented upper limits (30 s, 30 / 10 / 10 references) pass.
	for _, model := range []string{v6kTestActivity, v6kTestActivityPublic} {
		body := `{"model":"` + model + `","prompt":"x","duration":30,"resolution":"720p","ratio":"21:9","images":[` + repeatedJijiuItems(ref, 30) +
			`],"videos":[` + repeatedJijiuItems(ref, 10) + `],"audios":[` + repeatedJijiuItems(ref, 10) + `]}`
		prepared, _, err = PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, []byte(body))
		require.NoError(t, err, model)
		require.Equal(t, v6kTestActivity, gjson.GetBytes(prepared, "model").String())
		require.Equal(t, int64(30), gjson.GetBytes(prepared, "seconds").Int())
		require.Len(t, gjson.GetBytes(prepared, "images").Array(), 30)
		require.Len(t, gjson.GetBytes(prepared, "videos").Array(), 10)
		require.Len(t, gjson.GetBytes(prepared, "audios").Array(), 10)
	}

	for _, tc := range []struct{ name, body, want string }{
		{"below 4 s", head + `"duration":3}`, "from 4 to 30 seconds"},
		{"above 30 s", head + `"duration":31}`, "from 4 to 30 seconds"},
		{"720p only: 1080p", head + `"duration":5,"resolution":"1080p"}`, "resolution must be 720p"},
		{"720p only: 480p", head + `"duration":5,"resolution":"480p"}`, "resolution must be 720p"},
		{"too many images", head + `"duration":5,"images":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many videos", head + `"duration":5,"videos":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference videos"},
		{"too many audios", head + `"duration":5,"audios":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference audios"},
		{"ratio outside the set", head + `"duration":5,"ratio":"5:4"}`, "ratio must be one of"},
		{"edit is still refused", head + `"duration":5,"omni_reference_task_type":"edit"}`, "omni_reference_task_type"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

func TestTranslatePPVideoUnifiedRequestToV6K(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformV6KSeedance, []byte(`{
		"model": "`+v6kTestProPublic+`",
		"prompt": "@image1 在街道上与@image2 对话",
		"duration": 5,
		"resolution": "1080p",
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "image", "url": "https://cdn.example.com/other.jpg"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "`+v6kTestPro+`",
		"prompt": "@image1 在街道上与@image2 对话",
		"seconds": 5,
		"resolution": "1080p",
		"ratio": "16:9",
		"images": ["https://cdn.example.com/person.jpg", "https://cdn.example.com/other.jpg"],
		"audios": ["https://cdn.example.com/voice.mp3"]
	}`, string(prepared))

	for _, tc := range []struct{ name, body, want string }{
		{"no frames", `{"model":"` + v6kTestProPublic + `","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
		{"no audio switch", `{"model":"` + v6kTestProPublic + `","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"no seed", `{"model":"` + v6kTestProPublic + `","prompt":"x","duration":5,"seed":7}`, "seed"},
		{"prompt is required", `{"model":"` + v6kTestProPublic + `","duration":5}`, "prompt"},
		{"duration is required", `{"model":"` + v6kTestProPublic + `","prompt":"x"}`, "duration"},
		{"model is required", `{"prompt":"x","duration":5}`, "model"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformV6KSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}

	// Rules that live in the native normalizer apply to v2 requests as well.
	for _, tc := range []struct{ name, body, want string }{
		{"duration out of range", `{"model":"` + v6kTestFastPublic + `","prompt":"x","duration":16}`, "from 4 to 15 seconds"},
		{"resolution outside the model", `{"model":"` + v6kTestMiniPublic + `","prompt":"x","duration":5,"resolution":"1080p"}`, "resolution must be 480p, 720p"},
		{"reference video", `{"model":"` + v6kTestProPublic + `","prompt":"x","duration":5,"assets":[{"type":"video","url":"https://cdn.example.com/a.mp4"}]}`, "does not support reference videos"},
	} {
		translated, err := TranslatePPVideoUnifiedRequest(PlatformV6KSeedance, []byte(tc.body))
		require.NoError(t, err, tc.name)
		_, _, err = PreparePPVideoRequestBody(PlatformV6KSeedance, PPVideoOperationGeneric, translated)
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

func TestUnifiedV6KRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformV6KSeedance,
		`{
			"model": "`+v6kTestFastPublic+`",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "720p",
			"ratio": "9:16",
			"assets": [
				{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
				{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
			]
		}`,
		`{
			"model": "`+v6kTestFastPublic+`",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "720p",
			"ratio": "9:16",
			"images": ["https://cdn.example.com/cat.png"],
			"audios": ["https://cdn.example.com/voice.mp3"]
		}`,
	)
}

func TestV6KSeedanceCapabilitiesQuoteAndHubLines(t *testing.T) {
	t.Parallel()

	pro := PPVideoCapabilitiesForModel(PlatformV6KSeedance, v6kTestProPublic)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 15}, pro.Duration)
	require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, pro.Resolutions)
	require.Equal(t, []string{"16:9", "9:16", "1:1", "3:4", "4:3", "21:9"}, pro.Ratios)
	require.Equal(t, []int{9, 0, 3}, []int{*pro.Assets.ReferenceImages, *pro.Assets.ReferenceVideos, *pro.Assets.ReferenceAudios})
	require.False(t, *pro.Assets.FirstLastFrame)
	require.False(t, *pro.GenerateAudio)

	fast := PPVideoCapabilitiesForModel(PlatformV6KSeedance, v6kTestFast)
	require.Equal(t, []string{"480p", "720p"}, fast.Resolutions)
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformV6KSeedance, "other").Duration)

	activity := PPVideoCapabilitiesForModel(PlatformV6KSeedance, v6kTestActivityPublic)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, activity.Duration)
	require.Equal(t, []string{"720p"}, activity.Resolutions)
	require.Equal(t, []string{"16:9", "9:16", "1:1", "3:4", "4:3", "21:9"}, activity.Ratios)
	require.Equal(t, []int{30, 10, 10}, []int{*activity.Assets.ReferenceImages, *activity.Assets.ReferenceVideos, *activity.Assets.ReferenceAudios})
	require.False(t, *activity.Assets.FirstLastFrame)

	// The price estimate enforces the same limits as the submission.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformV6KSeedance, v6kTestProPublic, "", 10)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformV6KSeedance, v6kTestPro, "4k", 15)
	require.True(t, ok)
	require.Equal(t, "4k", resolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{v6kTestFastPublic, "4k", 10},
		{v6kTestMiniPublic, "1080p", 10},
		{v6kTestProPublic, "720p", 3},
		{v6kTestProPublic, "720p", 16},
		{"other", "720p", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformV6KSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.0--480p/720p/1080p/4k--官方按条--按次--V6K", "1080p", 10)
	require.True(t, ok)
	require.Equal(t, "1080p", resolution)

	// Every model of the table is one visible per-request line of the aggregate group, tag V6K.
	wantNames := map[string]string{
		v6kTestPro:      "Seedance2.0--480p/720p/1080p/4k--官方按条--按次--V6K",
		v6kTestFast:     "Seedance2.0--480p/720p--官方fast--按次--V6K",
		v6kTestMini:     "Seedance2.0--480p/720p--mini--按次--V6K",
		v6kTestActivity: "Seedance2.5--720p--活动--按次--V6K",
	}
	count := 0
	for _, line := range SeedanceHubLines() {
		if line.Platform != PlatformV6KSeedance {
			continue
		}
		count++
		spec, found := v6kModelSpecFor(line.Model)
		require.True(t, found, line.Name)
		require.Equal(t, spec.publicName(), line.Model, line.Name)
		require.Equal(t, wantNames[spec.Name], line.Name)
		require.Equal(t, BillingModePerRequest, line.Billing, line.Name)
		require.False(t, line.Hidden, line.Name)
		for _, name := range []string{line.Name, line.Model, spec.Name} {
			byName, found := SeedanceHubLineByName(name)
			require.True(t, found, name)
			require.Equal(t, line.Name, byName.Name)
		}
		_, found = SeedanceHubLineFor(PlatformXMSeedance, line.Model)
		require.False(t, found, line.Name)
		require.Equal(t, PPVideoCapabilitiesForModel(PlatformV6KSeedance, spec.Name).Duration,
			PPVideoCapabilitiesForModel(PlatformSeedanceHub, line.Name).Duration)

		quoted := fmt.Sprintf("%q", line.Name)
		resolved, body, err := ResolveSeedanceHubRequest([]byte(`{"model":` + quoted + `,"prompt":"x","duration":5}`))
		require.NoError(t, err)
		require.Equal(t, line.Name, resolved.Name)
		require.Equal(t, line.Model, gjson.GetBytes(body, "model").String())
		prepared, _, err := PreparePPVideoRequestBody(resolved.Platform, PPVideoOperationGeneric, body)
		require.NoError(t, err)
		require.Equal(t, spec.Name, gjson.GetBytes(prepared, "model").String())
	}
	require.Equal(t, len(v6kSeedanceModels), count)
}

func v6kTestAccount() *Account {
	return &Account{ID: 9, Platform: PlatformV6KSeedance, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "v6k-key"}}
}

// The result is the file itself: it is streamed as is, with the Range header forwarded and the
// key sent to the supplier's own site only.
func TestV6KContentIsStreamedFromTheDefaultSite(t *testing.T) {
	t.Parallel()

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{xmTestResponse(http.StatusPartialContent, "video/mp4", "video")}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	resp, err := svc.ForwardPPVideoContent(context.Background(), v6kTestAccount(), "task_1", "bytes=0-4", "")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "video", string(body))
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://www.v6k.top/v1/videos/task_1/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer v6k-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "bytes=0-4", upstream.requests[0].Header.Get("Range"))
}
