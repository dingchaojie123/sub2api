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
	// Upstream ids (the exact spelling, with the case that matters) and the public names clients see.
	naiwaTestV            = "奶蛙sd2.5-V"
	naiwaTestFlash        = "奶蛙sd2.5-flash-M"
	naiwaTestM            = "奶蛙sd2.5-M"
	naiwaTestVPublic      = "Seedance2.5-NW-V"
	naiwaTestFlashPublic  = "Seedance2.5-NW-flash-M"
	naiwaTestMPublic      = "Seedance2.5-NW-M"
	naiwaTestMediaExample = "https://example.com/1.png"
)

func TestNaiwaSeedanceModelTable(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{naiwaTestVPublic, naiwaTestFlashPublic, naiwaTestMPublic}, naiwaModelNames())
	require.Len(t, naiwaSeedanceModels, 3)

	seen := make(map[string]string)
	for _, spec := range naiwaSeedanceModels {
		for _, name := range []string{spec.Name, spec.publicName()} {
			key := strings.ToLower(name)
			owner, dup := seen[key]
			require.False(t, dup, "name %q is used by %q and %q", name, owner, spec.Name)
			seen[key] = spec.Name
		}
		require.Contains(t, strings.ToLower(spec.publicName()), "seedance", spec.Name)
		require.NotContains(t, strings.ToLower(spec.publicName()), "nailong", spec.Name)
		require.NotEqual(t, spec.Name, spec.publicName(), spec.Name)
		require.True(t, spec.ForwardResolution, spec.Name)
		require.False(t, spec.FirstLastFrame, spec.Name)
		require.Equal(t, "image_urls", spec.imageField(), spec.Name)
		require.Equal(t, "video_urls", spec.videoField(), spec.Name)
		require.Equal(t, "audio_urls", spec.audioField(), spec.Name)
		require.Equal(t, "ratio", spec.ratioField(), spec.Name)
		require.Equal(t, 30, spec.MaxSeconds, spec.Name)
	}

	// The operator's figures per model.
	v, ok := naiwaModelSpecFor(strings.ToLower(naiwaTestV))
	require.True(t, ok)
	require.Equal(t, naiwaTestV, v.Name)
	require.Equal(t, 5, v.MinSeconds)
	require.Equal(t, []string{"720p"}, v.Resolutions)
	require.False(t, v.ResolutionRequired)
	require.Equal(t, 15000, v.MaxPromptChars)
	require.Equal(t, []int{30, 10, 10}, []int{v.MaxImages, v.MaxVideos, v.MaxAudios})

	flash, ok := naiwaModelSpecFor(naiwaTestFlashPublic)
	require.True(t, ok)
	require.Equal(t, naiwaTestFlash, flash.Name)
	require.Equal(t, 4, flash.MinSeconds)
	require.Equal(t, []string{"480p", "720p"}, flash.Resolutions)
	require.True(t, flash.ResolutionRequired)
	require.Equal(t, []int{10, 5, 5}, []int{flash.MaxImages, flash.MaxVideos, flash.MaxAudios})

	m, ok := naiwaModelSpecFor(naiwaTestM)
	require.True(t, ok)
	require.Equal(t, 5, m.MinSeconds)
	require.Equal(t, []string{"480p", "720p"}, m.Resolutions)
	require.True(t, m.ResolutionRequired)
	require.Equal(t, []int{30, 10, 10}, []int{m.MaxImages, m.MaxVideos, m.MaxAudios})
}

func TestNaiwaSeedanceModelListAndRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformNaiwaSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformNaiwaSeedance))
	require.True(t, IsAllowedQuotaPlatform(PlatformNaiwaSeedance))
	require.True(t, IsSeedanceHubMemberPlatform(PlatformNaiwaSeedance))
	require.True(t, PPVideoGroupServesPlatform(PlatformSeedanceHub, PlatformNaiwaSeedance))

	for _, model := range []string{naiwaTestV, naiwaTestFlash, naiwaTestM, naiwaTestVPublic, naiwaTestFlashPublic, naiwaTestMPublic} {
		require.True(t, ppVideoModelBelongsToPlatform(PlatformNaiwaSeedance, model), model)
		require.False(t, ppVideoModelKnownForeignToPlatform(PlatformNaiwaSeedance, model), model)
	}
	require.False(t, ppVideoModelBelongsToPlatform(PlatformNaiwaSeedance, "other"))
	require.True(t, ppVideoModelKnownForeignToPlatform(PlatformXMSeedance, naiwaTestVPublic))

	require.Equal(t, []string{naiwaTestVPublic, naiwaTestMPublic},
		filterPPVideoPublicModelsForPlatform(PlatformNaiwaSeedance, []string{naiwaTestV, naiwaTestVPublic, "other", naiwaTestM}))
	require.Equal(t, naiwaTestFlashPublic, PPVideoPublicModelName(PlatformNaiwaSeedance, naiwaTestFlash))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformNaiwaSeedance, "other"))
}

func TestNaiwaSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformNaiwaSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, naiwaModelNames(), models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformNaiwaSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestNaiwaSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformNaiwaSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	statusPath, err := PPVideoStatusUpstreamPath(PlatformNaiwaSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", statusPath)

	headers, err := PPVideoRequestHeaders(PlatformNaiwaSeedance, "nw-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer nw-key", headers.Get("Authorization"))
	// The supplier makes one attempt in total and promises no idempotency: no key is sent.
	require.Empty(t, headers.Get("Idempotency-Key"))

	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformNaiwaSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformNaiwaSeedance))

	// An unknown or waiting status is never a failure.
	for _, tc := range []struct{ body, want string }{
		{`{"id":"nw_task_1","status":"queued"}`, PPVideoTaskStatusProcessing},
		{`{"id":"nw_task_1","status":"in_progress","progress":40}`, PPVideoTaskStatusProcessing},
		{`{"id":"nw_task_1","status":"unknown"}`, PPVideoTaskStatusProcessing},
		{`{"id":"nw_task_1","status":"completed"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"nw_task_1","status":"failed","error":{"message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformNaiwaSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		// The prefix of the id is kept as it is.
		require.Equal(t, "nw_task_1", parsed.TaskID, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}
	rejected, err := ParsePPVideoResponse(PlatformNaiwaSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", rejected.ErrorMessage)
}

func TestNaiwaSeedancePublicResponse(t *testing.T) {
	t.Parallel()

	public := PPVideoPublicRequest{Model: naiwaTestM}
	raw := `{"id":"nw_task_1","model":"` + naiwaTestM + `","status":"completed","progress":100,` +
		`"video_url":"https://api.nailongapi.cc/v1/videos/nw_task_1/content","cost":4.29,"final_cost":4.29,"price":5,"usage":{"total_tokens":1}}`
	parsed, err := ParsePPVideoResponse(PlatformNaiwaSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformNaiwaSeedance, []byte(raw), public, parsed)
	require.Equal(t, "succeeded", gjson.GetBytes(normalized, "status").String())
	require.Equal(t, naiwaTestMPublic, gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/nw_task_1/content", gjson.GetBytes(normalized, "video_url").String())
	for _, field := range []string{"cost", "final_cost", "price"} {
		require.False(t, gjson.GetBytes(normalized, field).Exists(), field)
	}
	require.NotContains(t, string(normalized), "nailongapi")
	require.NotContains(t, string(normalized), "奶蛙")
}

func TestNaiwaSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	const (
		a  = "https://example.com/a.png"
		v  = "https://example.com/v.mp4"
		au = "https://example.com/au.mp3"
	)
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "V may leave the resolution out: 720p is sent",
			body: `{"model":"` + naiwaTestVPublic + `","prompt":"x","duration":6}`,
			want: `{"model":"` + naiwaTestV + `","prompt":"x","duration":6,"resolution":"720p"}`,
		},
		{
			name: "the upstream id is accepted as the model name; seconds and aspect_ratio are aliases",
			body: `{"model":"` + naiwaTestV + `","prompt":"x","seconds":"6","aspect_ratio":"16:9","resolution":"720p"}`,
			want: `{"model":"` + naiwaTestV + `","prompt":"x","duration":6,"resolution":"720p","ratio":"16:9"}`,
		},
		{
			name: "aspectRatio and duration_seconds are read too",
			body: `{"model":"` + naiwaTestM + `","prompt":"x","duration_seconds":5,"aspectRatio":"9:16","resolution":"480p"}`,
			want: `{"model":"` + naiwaTestM + `","prompt":"x","duration":5,"resolution":"480p","ratio":"9:16"}`,
		},
		{
			name: "the aliases that agree are accepted together",
			body: `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5,"seconds":"5","ratio":"16:9","aspect_ratio":"16:9","resolution":"720P"}`,
			want: `{"model":"` + naiwaTestM + `","prompt":"x","duration":5,"resolution":"720p","ratio":"16:9"}`,
		},
		{
			name: "references are sent under the upstream's own array names, in order",
			body: `{"model":"` + naiwaTestFlashPublic + `","prompt":"x","duration":4,"resolution":"480p","images":["` + a + `","` + a + `?2"],"reference_videos":["` + v + `"],"audio_urls":["` + au + `"]}`,
			want: `{"model":"` + naiwaTestFlash + `","prompt":"x","duration":4,"resolution":"480p",` +
				`"image_urls":["` + a + `","` + a + `?2"],"video_urls":["` + v + `"],"audio_urls":["` + au + `"]}`,
		},
		{
			name: "the longest length at the highest resolution",
			body: `{"model":"` + naiwaTestFlashPublic + `","prompt":"x","duration":30,"resolution":"720p"}`,
			want: `{"model":"` + naiwaTestFlash + `","prompt":"x","duration":30,"resolution":"720p"}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prepared, _, err := PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(prepared))
		})
	}
}

func TestNaiwaSeedancePublicRequestAndBillingMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ model, upstream, resolution string }{
		{naiwaTestVPublic, naiwaTestV, "720p"},
		{naiwaTestFlashPublic, naiwaTestFlash, "480p"},
		{naiwaTestFlashPublic, naiwaTestFlash, "720p"},
		{naiwaTestMPublic, naiwaTestM, "480p"},
		{naiwaTestM, naiwaTestM, "720p"},
	} {
		prepared, public, err := PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, []byte(
			`{"model":"`+tc.model+`","prompt":"x","duration":12,"resolution":"`+tc.resolution+`","images":["`+naiwaTestMediaExample+`"]}`))
		require.NoError(t, err, tc.model)
		name, ok := ppVideoNaiwaPublicModel(tc.upstream)
		require.True(t, ok, tc.upstream)
		require.Equal(t, name, public.Model)
		require.Equal(t, tc.upstream, public.UpstreamModel)
		require.Equal(t, tc.upstream, gjson.GetBytes(prepared, "model").String())
		require.Equal(t, int64(12000), public.DurationMilliseconds)
		require.Equal(t, 1, public.VideoCount)
		require.Equal(t, tc.resolution, public.Resolution)
		require.True(t, public.HasImage)

		meta := PPVideoBillingMetadataFromRequest(PlatformNaiwaSeedance, prepared)
		require.Equal(t, PlatformNaiwaSeedance, meta.Platform)
		require.Equal(t, tc.upstream, meta.Model)
		require.Equal(t, 12, meta.RequestedDurationSeconds)
		// The resolution is what picks the per-request price tier.
		require.Equal(t, tc.resolution, meta.VideoResolution)
	}
}

func TestNaiwaSeedanceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	const ref = `"` + naiwaTestMediaExample + `"`
	v := `{"model":"` + naiwaTestVPublic + `","prompt":"x",`
	flash := `{"model":"` + naiwaTestFlashPublic + `","prompt":"x","resolution":"480p",`
	m := `{"model":"` + naiwaTestMPublic + `","prompt":"x","resolution":"720p",`
	for _, tc := range []struct{ name, body, want string }{
		{"missing model", `{"prompt":"x","duration":5}`, "model is required"},
		{"the old M model is gone", `{"model":"奶蛙sd2.5-480P720P-M","prompt":"x","duration":5}`, "is not available"},
		{"the other retired names", `{"model":"奶蛙sd2.5-M-S","prompt":"x","duration":5}`, "is not available"},
		{"missing prompt", `{"model":"` + naiwaTestVPublic + `","duration":5,"resolution":"720p"}`, "prompt is required"},
		{"missing duration", v + `"resolution":"720p"}`, "duration is required"},
		{"V below 5 s", v + `"duration":4}`, "from 5 to 30 seconds"},
		{"M below 5 s", m + `"duration":4}`, "from 5 to 30 seconds"},
		{"flash-M below 4 s", flash + `"duration":3}`, "from 4 to 30 seconds"},
		{"above 30 s", m + `"duration":31}`, "from 5 to 30 seconds"},
		{"automatic duration", m + `"duration":-1}`, "from 5 to 30 seconds"},
		{"fractional duration", m + `"duration":5.5}`, "integer"},
		{"duration and seconds disagree", m + `"duration":5,"seconds":"6"}`, "must have the same value"},
		{"duration and duration_seconds disagree", m + `"duration":5,"duration_seconds":6}`, "must have the same value"},
		{"ratio aliases disagree", m + `"duration":5,"ratio":"16:9","aspect_ratio":"9:16"}`, "must have the same value"},
		{"ratio that is not width:height", m + `"duration":5,"ratio":"wide"}`, "width:height"},
		{"ratio that is not a string", m + `"duration":5,"ratio":1}`, "must be a string"},
		{"V refuses 480p", v + `"duration":5,"resolution":"480p"}`, "resolution must be 720p for this model"},
		{"M refuses 1080p", m[:len(m)-len(`"resolution":"720p",`)] + `"resolution":"1080p","duration":5}`, "resolution must be 480p, 720p"},
		{"M needs the resolution", `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5}`, "resolution is required"},
		{"flash-M needs the resolution", `{"model":"` + naiwaTestFlashPublic + `","prompt":"x","duration":5,"resolution":""}`, "resolution is required"},
		{"seed", m + `"duration":5,"seed":1}`, "does not support seed"},
		{"generate_audio", m + `"duration":5,"generate_audio":true}`, "does not support generate_audio"},
		{"first frame", m + `"duration":5,"first_frame":` + ref + `}`, "does not support first or last frames"},
		{"assets on v1", m + `"duration":5,"assets":[{"type":"image","url":` + ref + `}]}`, "unified v2 endpoint"},
		{"two videos in one request", m + `"duration":5,"count":2}`, "one video per request"},
		{"too many images for M", m + `"duration":5,"images":[` + repeatedJijiuItems(ref, 31) + `]}`, "at most 30 reference images"},
		{"too many images for flash-M", flash + `"duration":5,"images":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference images"},
		{"too many videos for flash-M", flash + `"duration":5,"videos":[` + repeatedJijiuItems(ref, 6) + `]}`, "at most 5 reference videos"},
		{"too many audios for flash-M", flash + `"duration":5,"audios":[` + repeatedJijiuItems(ref, 6) + `]}`, "at most 5 reference audios"},
		{"too many videos for V", v + `"duration":5,"videos":[` + repeatedJijiuItems(ref, 11) + `]}`, "at most 10 reference videos"},
		{"two names for the same images", m + `"duration":5,"images":[` + ref + `],"image_urls":[` + ref + `]}`, "accepts only one of"},
		{"plain http media", m + `"duration":5,"images":["http://example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"media on an IP address", m + `"duration":5,"images":["https://127.0.0.1/1.png"]}`, "must be a public HTTPS URL"},
		{"media with credentials", m + `"duration":5,"images":["https://user:pw@example.com/1.png"]}`, "must be a public HTTPS URL"},
		{"V prompt above 15000 characters", `{"model":"` + naiwaTestVPublic + `","prompt":"` + strings.Repeat("字", 15001) + `","duration":5}`, "at most 15000 characters"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, []byte(tc.body))
			require.ErrorContains(t, err, tc.want)
		})
	}

	// The documented upper limits are accepted, and the 15000-character prompt is the V limit
	// only (the supplier gives none for the M models).
	for _, tc := range []struct {
		head                   string
		images, videos, audios int
	}{
		{m, 30, 10, 10},
		{flash, 10, 5, 5},
		{v, 30, 10, 10},
	} {
		body := tc.head + `"duration":30,"images":[` + repeatedJijiuItems(ref, tc.images) + `],"videos":[` + repeatedJijiuItems(ref, tc.videos) + `],"audios":[` + repeatedJijiuItems(ref, tc.audios) + `]}`
		_, _, err := PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, []byte(body))
		require.NoError(t, err, tc.head)
	}
	_, _, err := PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+naiwaTestVPublic+`","prompt":"`+strings.Repeat("字", 15000)+`","duration":5}`))
	require.NoError(t, err)
	_, _, err = PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, []byte(
		`{"model":"`+naiwaTestMPublic+`","prompt":"`+strings.Repeat("字", 15001)+`","duration":5,"resolution":"720p"}`))
	require.NoError(t, err)
}

func TestTranslatePPVideoUnifiedRequestToNaiwa(t *testing.T) {
	t.Parallel()

	translated, err := TranslatePPVideoUnifiedRequest(PlatformNaiwaSeedance, []byte(`{
		"model": "`+naiwaTestMPublic+`",
		"prompt": "@image1 在街道上与@image2 对话",
		"duration": 5,
		"resolution": "480p",
		"ratio": "16:9",
		"assets": [
			{"type": "image", "name": "person.jpg", "url": "https://cdn.example.com/person.jpg"},
			{"type": "image", "url": "https://cdn.example.com/other.jpg"},
			{"type": "video", "url": "https://cdn.example.com/motion.mp4"},
			{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
		]
	}`))
	require.NoError(t, err)
	prepared, _, err := PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, translated)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model": "`+naiwaTestM+`",
		"prompt": "@image1 在街道上与@image2 对话",
		"duration": 5,
		"resolution": "480p",
		"ratio": "16:9",
		"image_urls": ["https://cdn.example.com/person.jpg", "https://cdn.example.com/other.jpg"],
		"video_urls": ["https://cdn.example.com/motion.mp4"],
		"audio_urls": ["https://cdn.example.com/voice.mp3"]
	}`, string(prepared))

	for _, tc := range []struct{ name, body, want string }{
		{"no frames", `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5,"assets":[{"type":"image","role":"first_frame","url":"https://cdn.example.com/a.png"}]}`, "first_frame"},
		{"no audio switch", `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5,"generate_audio":true}`, "generate_audio"},
		{"no seed", `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5,"seed":7}`, "seed"},
		{"no callback", `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5,"callback_url":"https://example.com/cb"}`, "callback_url"},
		{"no options", `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5,"options":{"a":"b"}}`, "options"},
		{"prompt is required", `{"model":"` + naiwaTestMPublic + `","duration":5}`, "prompt"},
		{"duration is required", `{"model":"` + naiwaTestMPublic + `","prompt":"x"}`, "duration"},
		{"model is required", `{"prompt":"x","duration":5}`, "model"},
	} {
		_, err := TranslatePPVideoUnifiedRequest(PlatformNaiwaSeedance, []byte(tc.body))
		require.ErrorContains(t, err, tc.want, tc.name)
	}

	// Rules that live in the native normalizer apply to v2 requests as well.
	for _, tc := range []struct{ name, body, want string }{
		{"M needs the resolution", `{"model":"` + naiwaTestMPublic + `","prompt":"x","duration":5}`, "resolution is required"},
		{"duration out of range", `{"model":"` + naiwaTestVPublic + `","prompt":"x","duration":4}`, "from 5 to 30 seconds"},
		{"too many videos", `{"model":"` + naiwaTestFlashPublic + `","prompt":"x","duration":5,"resolution":"480p","assets":[` +
			strings.TrimSuffix(strings.Repeat(`{"type":"video","url":"https://cdn.example.com/a.mp4"},`, 6), ",") + `]}`, "at most 5 reference videos"},
	} {
		translated, err := TranslatePPVideoUnifiedRequest(PlatformNaiwaSeedance, []byte(tc.body))
		require.NoError(t, err, tc.name)
		_, _, err = PreparePPVideoRequestBody(PlatformNaiwaSeedance, PPVideoOperationGeneric, translated)
		require.ErrorContains(t, err, tc.want, tc.name)
	}
}

func TestUnifiedNaiwaRequestMatchesNativeRequest(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformNaiwaSeedance,
		`{
			"model": "`+naiwaTestFlashPublic+`",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "720p",
			"ratio": "9:16",
			"assets": [
				{"type": "image", "name": "cat.png", "url": "https://cdn.example.com/cat.png"},
				{"type": "video", "url": "https://cdn.example.com/run.mp4"},
				{"type": "audio", "url": "https://cdn.example.com/voice.mp3"}
			]
		}`,
		`{
			"model": "`+naiwaTestFlashPublic+`",
			"prompt": "夕阳下的海边，一只猫沿沙滩奔跑",
			"duration": 8,
			"resolution": "720p",
			"ratio": "9:16",
			"images": ["https://cdn.example.com/cat.png"],
			"videos": ["https://cdn.example.com/run.mp4"],
			"audios": ["https://cdn.example.com/voice.mp3"]
		}`,
	)
}

func TestNaiwaSeedanceCapabilitiesQuoteAndHubLines(t *testing.T) {
	t.Parallel()

	v := PPVideoCapabilitiesForModel(PlatformNaiwaSeedance, naiwaTestVPublic)
	require.Equal(t, &PPVideoDurationRange{Min: 5, Max: 30}, v.Duration)
	require.Equal(t, []string{"720p"}, v.Resolutions)
	require.Equal(t, 30, *v.Assets.ReferenceImages)
	require.Equal(t, 10, *v.Assets.ReferenceVideos)
	require.Equal(t, 10, *v.Assets.ReferenceAudios)
	require.False(t, *v.Assets.FirstLastFrame)
	require.False(t, *v.GenerateAudio)
	// The supplier gives no set of ratios, so none is promised.
	require.Empty(t, v.Ratios)

	flash := PPVideoCapabilitiesForModel(PlatformNaiwaSeedance, naiwaTestFlash)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, flash.Duration)
	require.Equal(t, []string{"480p", "720p"}, flash.Resolutions)
	require.Equal(t, []int{10, 5, 5}, []int{*flash.Assets.ReferenceImages, *flash.Assets.ReferenceVideos, *flash.Assets.ReferenceAudios})
	require.Nil(t, PPVideoCapabilitiesForModel(PlatformNaiwaSeedance, "other").Duration)

	// The price estimate enforces the same limits as the submission, including the required resolution.
	resolution, ok := normalizeMediaQuoteVideoSpec(PlatformNaiwaSeedance, naiwaTestVPublic, "", 10)
	require.True(t, ok)
	require.Equal(t, "720p", resolution)
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformNaiwaSeedance, naiwaTestFlash, "480p", 4)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
	}{
		{naiwaTestMPublic, "", 10},
		{naiwaTestFlashPublic, "", 10},
		{naiwaTestMPublic, "480p", 4},
		{naiwaTestFlashPublic, "720p", 3},
		{naiwaTestVPublic, "480p", 10},
		{naiwaTestMPublic, "1080p", 10},
		{naiwaTestMPublic, "720p", 31},
		{"other", "720p", 10},
	} {
		_, ok = normalizeMediaQuoteVideoSpec(PlatformNaiwaSeedance, tc.model, tc.resolution, tc.seconds)
		require.False(t, ok, "%+v", tc)
	}
	resolution, ok = normalizeMediaQuoteVideoSpec(PlatformSeedanceHub, "Seedance2.5--480p/720p--M--按次--NW", "480p", 10)
	require.True(t, ok)
	require.Equal(t, "480p", resolution)

	// Every model of the table is one per-request line of the aggregate group, tag NW.
	wantNames := map[string]string{
		naiwaTestV:     "Seedance2.5--720p--V--按次--NW",
		naiwaTestFlash: "Seedance2.5--480p/720p--flash-M--按次--NW",
		naiwaTestM:     "Seedance2.5--480p/720p--M--按次--NW",
	}
	count := 0
	for _, line := range SeedanceHubLines() {
		if line.Platform != PlatformNaiwaSeedance {
			continue
		}
		count++
		spec, found := naiwaModelSpecFor(line.Model)
		require.True(t, found, line.Name)
		require.Equal(t, spec.publicName(), line.Model, line.Name)
		require.Equal(t, wantNames[spec.Name], line.Name)
		require.Equal(t, BillingModePerRequest, line.Billing, line.Name)
		for _, name := range []string{line.Name, line.Model, spec.Name} {
			byName, found := SeedanceHubLineByName(name)
			require.True(t, found, name)
			require.Equal(t, line.Name, byName.Name)
		}
		_, found = SeedanceHubLineFor(PlatformXMSeedance, line.Model)
		require.False(t, found, line.Name)
		require.Equal(t, PPVideoCapabilitiesForModel(PlatformNaiwaSeedance, spec.Name).Duration,
			PPVideoCapabilitiesForModel(PlatformSeedanceHub, line.Name).Duration)

		// A hub request is rewritten to the platform's public name, then goes through the platform.
		// The M models need the resolution, so one is named for every line.
		quoted := fmt.Sprintf("%q", line.Name)
		resolved, body, err := ResolveSeedanceHubRequest([]byte(`{"model":` + quoted + `,"prompt":"x","duration":5,"resolution":"720p"}`))
		require.NoError(t, err)
		require.Equal(t, line.Name, resolved.Name)
		require.Equal(t, line.Model, gjson.GetBytes(body, "model").String())
		prepared, _, err := PreparePPVideoRequestBody(resolved.Platform, PPVideoOperationGeneric, body)
		require.NoError(t, err)
		require.Equal(t, spec.Name, gjson.GetBytes(prepared, "model").String())
	}
	require.Equal(t, len(naiwaSeedanceModels), count)
}

func naiwaTestAccount() *Account {
	return &Account{ID: 8, Platform: PlatformNaiwaSeedance, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "nw-key"}}
}

// The result is read through the same content follower as the xm platform: a file answer is
// returned unchanged, a JSON answer with an address is followed without any credential.
func TestNaiwaContentFileAndJSONAnswers(t *testing.T) {
	t.Parallel()

	upstream := &oponeUpstreamRecorder{responses: []*http.Response{xmTestResponse(http.StatusOK, "video/mp4", "video")}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	resp, err := svc.ForwardPPVideoContent(context.Background(), naiwaTestAccount(), "nw_task_1", "", "")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "video", string(body))
	require.Len(t, upstream.requests, 1)
	// The default address is the supplier's site, and the task id is used as the supplier gave it.
	require.Equal(t, "https://api.nailongapi.cc/v1/videos/nw_task_1/content", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer nw-key", upstream.requests[0].Header.Get("Authorization"))

	upstream = &oponeUpstreamRecorder{responses: []*http.Response{
		xmTestResponse(http.StatusOK, "application/json", `{"url":"https://cdn.example.com/v.mp4?sig=1"}`),
		xmTestResponse(http.StatusPartialContent, "video/mp4", "video"),
	}}
	svc = &OpenAIGatewayService{httpUpstream: upstream}
	resp2, err := svc.ForwardPPVideoContent(context.Background(), naiwaTestAccount(), "nw_task_1", "bytes=0-4", "")
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusPartialContent, resp2.StatusCode)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://cdn.example.com/v.mp4?sig=1", upstream.requests[1].URL.String())
	require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
}
