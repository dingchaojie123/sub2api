package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHongpeiSeedanceModelTable(t *testing.T) {
	t.Parallel()

	// Clients see the public names (they carry "seedance"); the upstream names are the only ones sent upstream.
	require.Equal(t, []string{"Seedance2.5-vs-L", "Seedance2.5-vs-v", "Seedance2.5-vs-Y", "Seedance2.5-vs-flash", "Seedance2.5-vs-a", "Seedance2.5-vs-s", "Seedance2.5-vs-nh", "Seedance2.5-vs-X"}, hongpeiModelNames())
	for _, tc := range [][2]string{
		{"vs-sd2.5-L", "Seedance2.5-vs-L"}, {"vs-sd-2.5-v", "Seedance2.5-vs-v"},
		{"vs-sd2.5-Y", "Seedance2.5-vs-Y"}, {"vs-sd2.5-flash", "Seedance2.5-vs-flash"},
		{"vs-sd2.5-a", "Seedance2.5-vs-a"}, {"vs-sd2.5-s", "Seedance2.5-vs-s"},
		{"vs-sd2.5-nh", "Seedance2.5-vs-nh"}, {"vs-sd2.5-X", "Seedance2.5-vs-X"},
	} {
		for _, name := range []string{tc[0], strings.ToUpper(tc[0]), tc[1], strings.ToLower(tc[1])} {
			entry, ok := hongpeiModelFor(name)
			require.True(t, ok, name)
			require.Equal(t, tc[0], entry.spec.Name, name)
			require.Equal(t, tc[1], entry.spec.publicName(), name)
		}
	}

	l, ok := hongpeiModelFor("VS-SD2.5-L")
	require.True(t, ok)
	require.Equal(t, 4, l.spec.MinSeconds)
	require.Equal(t, 5, l.spec.MinSecondsWithRefs)
	require.Equal(t, 30, l.spec.MaxSeconds)
	require.Equal(t, []string{"720p"}, l.spec.Resolutions)
	require.Equal(t, 30, l.spec.MaxReferenceTotal)
	require.Equal(t, 20000, l.promptMax)
	require.True(t, l.audioNeedsVisual)
	require.False(t, l.allowsRatio("adaptive"))

	v, ok := hongpeiModelFor("vs-sd-2.5-v")
	require.True(t, ok)
	require.Equal(t, 15000, v.promptMax)
	require.True(t, v.allowsRatio("adaptive"))
	require.False(t, v.audioNeedsVisual)

	y, ok := hongpeiModelFor("vs-sd2.5-Y")
	require.True(t, ok)
	require.Equal(t, 5, y.spec.MinSeconds)
	require.Zero(t, y.promptMax)
	require.False(t, y.allowsRatio("21:9"))

	flash, ok := hongpeiModelFor("vs-sd2.5-flash")
	require.True(t, ok)
	require.Equal(t, []string{"480p", "720p"}, flash.spec.Resolutions)
	require.Equal(t, "720p", flash.spec.defaultResolution())
	require.Equal(t, 10, flash.spec.MaxImages)
	require.Zero(t, flash.spec.MaxVideos)
	require.Equal(t, 5, flash.spec.MaxAudios)

	// vs-sd2.5-a: four resolutions (default 480p), 4-30 seconds, 30 / 10 / 10 references, prompt 15000.
	a, ok := hongpeiModelFor("VS-SD2.5-A")
	require.True(t, ok)
	require.Equal(t, "vs-sd2.5-a", a.spec.Name)
	require.Equal(t, "Seedance2.5-vs-a", a.spec.publicName())
	require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, a.spec.Resolutions)
	require.Equal(t, "480p", a.spec.defaultResolution())
	require.Equal(t, 4, a.spec.MinSeconds)
	require.Zero(t, a.spec.MinSecondsWithRefs)
	require.Equal(t, 30, a.spec.MaxSeconds)
	require.Equal(t, 30, a.spec.MaxImages)
	require.Equal(t, 10, a.spec.MaxVideos)
	require.Equal(t, 10, a.spec.MaxAudios)
	require.Equal(t, 15000, a.promptMax)
	require.False(t, a.audioNeedsVisual)
	// The public name carries "seedance", so a client that keeps only Seedance-looking names still sees it.
	require.Contains(t, strings.ToLower(a.spec.publicName()), "seedance")
	require.Equal(t, []string{"Seedance2.5-vs-a", "vs-sd2.5-a"}, ppVideoHongpeiModelAliases("vs-sd2.5-a"))
	require.Equal(t, []string{"Seedance2.5-vs-a", "vs-sd2.5-a"}, ppVideoPricingModelAliases(PlatformHongpeiSeedance, "SEEDANCE2.5-VS-A"))

	// vs-sd2.5-s: 720p only, 4-30 s, 30 / 10 / 10 but at most 30 in total, audio needs an image or video.
	sModel, ok := hongpeiModelFor("vs-sd2.5-s")
	require.True(t, ok)
	require.Equal(t, []string{"720p"}, sModel.spec.Resolutions)
	require.Equal(t, 4, sModel.spec.MinSeconds)
	require.Equal(t, 30, sModel.spec.MaxSeconds)
	require.Equal(t, 30, sModel.spec.MaxReferenceTotal)
	require.Equal(t, 15000, sModel.promptMax)
	require.True(t, sModel.audioNeedsVisual)
	require.False(t, sModel.audioNeedsImage)
	require.False(t, sModel.allowsRatio("adaptive"))

	// vs-sd2.5-nh: 480p / 720p / 1080p (default 720p), no combined limit, audio needs an image or video.
	nh, ok := hongpeiModelFor("vs-sd2.5-nh")
	require.True(t, ok)
	require.Equal(t, []string{"480p", "720p", "1080p"}, nh.spec.Resolutions)
	require.Equal(t, "720p", nh.spec.defaultResolution())
	require.Zero(t, nh.spec.MaxReferenceTotal)
	require.Equal(t, 30, nh.spec.MaxImages)
	require.Equal(t, 10, nh.spec.MaxVideos)
	require.Equal(t, 10, nh.spec.MaxAudios)
	require.Equal(t, 15000, nh.promptMax)
	require.True(t, nh.audioNeedsVisual)

	// vs-sd2.5-X: 720p only, audio needs an IMAGE (a video is not enough).
	x, ok := hongpeiModelFor("vs-sd2.5-x")
	require.True(t, ok)
	require.Equal(t, "vs-sd2.5-X", x.spec.Name)
	require.Equal(t, []string{"720p"}, x.spec.Resolutions)
	require.Equal(t, 15000, x.promptMax)
	require.True(t, x.audioNeedsImage)
	require.False(t, x.audioNeedsVisual)

	// The public names carry "seedance", so a client that keeps only Seedance-looking names still sees them.
	for _, name := range []string{"vs-sd2.5-s", "vs-sd2.5-nh", "vs-sd2.5-X", "vs-sd-2.5-v"} {
		public, ok := ppVideoHongpeiPublicModel(name)
		require.True(t, ok, name)
		require.Contains(t, strings.ToLower(public), "seedance", name)
	}

	// vs-sd2.5-Z, -vs and the others of the upstream document are not served.
	for _, model := range []string{"", "vs-sd2.5-Z", "vs-sd2.5-m", "vs-sd2.5-vs", "vs-sd2.5-flash-a", "vs-sd2.0-a"} {
		_, ok = hongpeiModelFor(model)
		require.False(t, ok, model)
	}
	// Public name first, so a channel price configured under it wins; the upstream name still works.
	require.Equal(t, []string{"Seedance2.5-vs-L", "vs-sd2.5-L"}, ppVideoHongpeiModelAliases("VS-SD2.5-L"))
	require.Equal(t, []string{"Seedance2.5-vs-L", "vs-sd2.5-L"}, ppVideoPricingModelAliases(PlatformHongpeiSeedance, "vs-sd2.5-l"))
	require.Equal(t, []string{"Seedance2.5-vs-L", "vs-sd2.5-L"}, ppVideoPricingModelAliases(PlatformHongpeiSeedance, "SEEDANCE2.5-VS-L"))
}

func TestHongpeiSeedanceRouting(t *testing.T) {
	t.Parallel()

	require.True(t, IsPPVideoPlatform(PlatformHongpeiSeedance))
	require.True(t, IsPPVideoUnifiedPlatform(PlatformHongpeiSeedance))
	require.True(t, IsSeedanceHubMemberPlatform(PlatformHongpeiSeedance))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformHongpeiSeedance, "vs-sd2.5-Y"))
	require.False(t, ppVideoModelBelongsToPlatform(PlatformHongpeiSeedance, "vs-sd2.5-Z"))
	for _, name := range []string{"vs-sd2.5-X", "vs-sd2.5-s", "vs-sd2.5-nh", "Seedance2.5-vs-X", "Seedance2.5-vs-s", "Seedance2.5-vs-nh"} {
		require.True(t, ppVideoModelBelongsToPlatform(PlatformHongpeiSeedance, name), name)
	}
	require.True(t, ppVideoTaskUsesAuthenticatedContent(PlatformHongpeiSeedance))
	require.True(t, PPVideoTaskCanStreamContent(PlatformHongpeiSeedance))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformHongpeiSeedance, "Seedance2.5-vs-Y"))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformHongpeiSeedance, "vs-sd2.5-a"))
	require.True(t, ppVideoModelBelongsToPlatform(PlatformHongpeiSeedance, "Seedance2.5-vs-a"))
	require.Equal(t, "Seedance2.5-vs-a", PPVideoPublicModelName(PlatformHongpeiSeedance, "vs-sd2.5-a"))
	require.Equal(t, "Seedance2.5-vs-L", PPVideoPublicModelName(PlatformHongpeiSeedance, "VS-SD2.5-L"))
	require.Equal(t, "other", PPVideoPublicModelName(PlatformHongpeiSeedance, "other"))
	// A whitelist saved under either name is listed once, under the public name.
	require.Equal(t, []string{"Seedance2.5-vs-L", "Seedance2.5-vs-Y"},
		filterPPVideoPublicModelsForPlatform(PlatformHongpeiSeedance, []string{"vs-sd2.5-L", "Seedance2.5-vs-L", "vs-sd2.5-Y", "vs-sd2.5-Z", "other"}))
	// The upstream id of vs-sd2.5-a has no "seedance"; a whitelist that holds it is listed under the public name.
	require.Equal(t, []string{"Seedance2.5-vs-a"},
		filterPPVideoPublicModelsForPlatform(PlatformHongpeiSeedance, []string{"vs-sd2.5-a", "Seedance2.5-vs-a"}))
}

func TestHongpeiSeedanceModelSyncUsesLocalTable(t *testing.T) {
	t.Parallel()

	models, err := (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformHongpeiSeedance, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"},
	})
	require.NoError(t, err)
	require.Equal(t, hongpeiModelNames(), models)

	_, err = (&AccountTestService{}).FetchUpstreamSupportedModels(context.Background(), &Account{
		Platform: PlatformHongpeiSeedance, Type: AccountTypeAPIKey,
	})
	require.Error(t, err)
}

func TestHongpeiSeedanceUpstreamPathsHeadersAndResponses(t *testing.T) {
	t.Parallel()

	createPath, err := PPVideoUpstreamPath(PlatformHongpeiSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", createPath)
	headers, err := PPVideoRequestHeaders(PlatformHongpeiSeedance, "hongpei-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer hongpei-key", headers.Get("Authorization"))

	for _, tc := range []struct{ body, want string }{
		{`{"id":"task_1","status":"queued"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"in_progress"}`, PPVideoTaskStatusProcessing},
		// A bulk-intake id is polled like any other task.
		{`{"id":"task_bulk_9","status":"reconciling"}`, PPVideoTaskStatusProcessing},
		{`{"id":"task_1","status":"completed"}`, PPVideoTaskStatusSucceeded},
		{`{"id":"task_1","status":"failed","error":{"message":"boom"}}`, PPVideoTaskStatusFailed},
	} {
		parsed, err := ParsePPVideoResponse(PlatformHongpeiSeedance, []byte(tc.body))
		require.NoError(t, err, tc.body)
		require.Equal(t, tc.want, parsed.Status, tc.body)
	}
	failed, err := ParsePPVideoResponse(PlatformHongpeiSeedance, []byte(`{"error":"insufficient balance"}`))
	require.NoError(t, err)
	require.Equal(t, "insufficient balance", failed.ErrorMessage)

	// The upstream's address is never shown; the gateway's own content address is.
	public := PPVideoPublicRequest{Model: "vs-sd2.5-L"}
	raw := `{"id":"task_1","status":"completed","video_url":"https://cdn.example.com/result.mp4","actual_task_id":"up_1"}`
	parsed, err := ParsePPVideoResponse(PlatformHongpeiSeedance, []byte(raw))
	require.NoError(t, err)
	normalized := NormalizePPVideoPublicResponse(PlatformHongpeiSeedance, []byte(raw), public, parsed)
	// The client sees the public name, never the upstream one.
	require.Equal(t, "Seedance2.5-vs-L", gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "/v1/videos/task_1/content", gjson.GetBytes(normalized, "video_url").String())
	require.NotContains(t, string(normalized), "cdn.example.com")
}

func TestHongpeiSeedanceNormalizesNativeRequest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			// The public name is accepted and the upstream name is what goes upstream.
			name: "public name: sent upstream under the upstream name",
			body: `{"model":"Seedance2.5-vs-Y","prompt":"x","duration":5}`,
			want: `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"resolution":"720p"}`,
		},
		{
			name: "L: defaults to 720p, duration is sent as duration",
			body: `{"model":"vs-sd2.5-L","prompt":"x","duration":4}`,
			want: `{"model":"vs-sd2.5-L","prompt":"x","duration":4,"resolution":"720p"}`,
		},
		{
			name: "L: audio with an image, 5 seconds once media is sent",
			body: `{"model":"vs-sd2.5-L","prompt":"x","duration":5,"aspect_ratio":"21:9","images":["https://example.com/1.png"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"vs-sd2.5-L","prompt":"x","duration":5,"resolution":"720p","aspect_ratio":"21:9",` +
				`"image_urls":["https://example.com/1.png"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "v: adaptive ratio and a string duration",
			body: `{"model":"vs-sd-2.5-v","prompt":"x","seconds":"30","ratio":"adaptive","audio_urls":["https://example.com/1.mp3"]}`,
			want: `{"model":"vs-sd-2.5-v","prompt":"x","duration":30,"resolution":"720p","aspect_ratio":"adaptive","audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "Y: 5 seconds, videos allowed",
			body: `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"videos":["https://example.com/1.mp4"]}`,
			want: `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"resolution":"720p","video_urls":["https://example.com/1.mp4"]}`,
		},
		{
			name: "a: public name, defaults to 480p",
			body: `{"model":"Seedance2.5-vs-a","prompt":"x","duration":4}`,
			want: `{"model":"vs-sd2.5-a","prompt":"x","duration":4,"resolution":"480p"}`,
		},
		{
			name: "a: 4k with all three kinds of references, 4 seconds",
			body: `{"model":"vs-sd2.5-a","prompt":"x","duration":4,"resolution":"4k","ratio":"21:9","images":["https://example.com/1.png"],"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"vs-sd2.5-a","prompt":"x","duration":4,"resolution":"4k","aspect_ratio":"21:9",` +
				`"image_urls":["https://example.com/1.png"],"video_urls":["https://example.com/1.mp4"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "a: 1080p, 30 seconds, audio without an image or video",
			body: `{"model":"Seedance2.5-vs-A","prompt":"x","seconds":30,"resolution":"1080p","audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"vs-sd2.5-a","prompt":"x","duration":30,"resolution":"1080p","audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "s: public name, 720p, 30 seconds, audio with a video",
			body: `{"model":"Seedance2.5-vs-s","prompt":"x","duration":30,"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"vs-sd2.5-s","prompt":"x","duration":30,"resolution":"720p","video_urls":["https://example.com/1.mp4"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "nh: 1080p, 4 seconds",
			body: `{"model":"vs-sd2.5-nh","prompt":"x","duration":4,"resolution":"1080p"}`,
			want: `{"model":"vs-sd2.5-nh","prompt":"x","duration":4,"resolution":"1080p"}`,
		},
		{
			name: "nh: defaults to 720p, 30 images + 10 videos + 10 audios (no combined limit)",
			body: `{"model":"Seedance2.5-vs-nh","prompt":"x","duration":5,"images":[` + strings.TrimSuffix(strings.Repeat(`"https://example.com/i.png",`, 30), ",") + `],"videos":[` + strings.TrimSuffix(strings.Repeat(`"https://example.com/v.mp4",`, 10), ",") + `],"audios":[` + strings.TrimSuffix(strings.Repeat(`"https://example.com/a.mp3",`, 10), ",") + `]}`,
			want: `{"model":"vs-sd2.5-nh","prompt":"x","duration":5,"resolution":"720p","image_urls":[` + strings.TrimSuffix(strings.Repeat(`"https://example.com/i.png",`, 30), ",") + `],"video_urls":[` + strings.TrimSuffix(strings.Repeat(`"https://example.com/v.mp4",`, 10), ",") + `],"audio_urls":[` + strings.TrimSuffix(strings.Repeat(`"https://example.com/a.mp3",`, 10), ",") + `]}`,
		},
		{
			name: "X: audio with an image",
			body: `{"model":"Seedance2.5-vs-X","prompt":"x","duration":4,"images":["https://example.com/1.png"],"audios":["https://example.com/1.mp3"]}`,
			want: `{"model":"vs-sd2.5-X","prompt":"x","duration":4,"resolution":"720p","image_urls":["https://example.com/1.png"],"audio_urls":["https://example.com/1.mp3"]}`,
		},
		{
			name: "flash: 480p is accepted",
			body: `{"model":"vs-sd2.5-flash","prompt":"x","duration":4,"resolution":"480p","image_urls":["https://example.com/1.png"]}`,
			want: `{"model":"vs-sd2.5-flash","prompt":"x","duration":4,"resolution":"480p","image_urls":["https://example.com/1.png"]}`,
		},
	} {
		prepared, _, err := PreparePPVideoRequestBody(PlatformHongpeiSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.NoError(t, err, tc.name)
		require.JSONEq(t, tc.want, string(prepared), tc.name)
	}
}

func TestHongpeiSeedanceRefusesWhatTheModelDoesNotTake(t *testing.T) {
	t.Parallel()

	manyImages := `"` + strings.TrimSuffix(strings.Repeat(`https://example.com/i.png","`, 20), `,"`)
	manyVideos := `"` + strings.TrimSuffix(strings.Repeat(`https://example.com/v.mp4","`, 10), `,"`)

	for _, tc := range []struct{ name, body, contains string }{
		{"unknown model", `{"model":"vs-sd2.5-Z","prompt":"x","duration":4}`, "is not available"},
		{"L: audio alone", `{"model":"vs-sd2.5-L","prompt":"x","duration":5,"audios":["https://example.com/1.mp3"]}`, "reference audio must be sent with"},
		{"L: 4 seconds once a video is sent", `{"model":"vs-sd2.5-L","prompt":"x","duration":4,"videos":["https://example.com/1.mp4"]}`, "from 5 to 30"},
		{"L: 31 items in total", `{"model":"vs-sd2.5-L","prompt":"x","duration":5,"images":[` + manyImages + `],"videos":[` + manyVideos + `],"audios":["https://example.com/1.mp3"]}`, "at most 30 reference items"},
		{"L: adaptive ratio", `{"model":"vs-sd2.5-L","prompt":"x","duration":4,"ratio":"adaptive"}`, "ratio must be one of"},
		{"L: prompt over 20000", `{"model":"vs-sd2.5-L","prompt":"` + strings.Repeat("a", 20001) + `","duration":4}`, "at most 20000 characters"},
		{"v: prompt over 15000", `{"model":"vs-sd-2.5-v","prompt":"` + strings.Repeat("a", 15001) + `","duration":5}`, "at most 15000 characters"},
		{"Y: 4 seconds", `{"model":"vs-sd2.5-Y","prompt":"x","duration":4}`, "from 5 to 30"},
		{"Y: 21:9", `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"ratio":"21:9"}`, "ratio must be one of"},
		{"flash: reference video", `{"model":"vs-sd2.5-flash","prompt":"x","duration":4,"videos":["https://example.com/1.mp4"]}`, "does not support reference videos"},
		{"flash: 1080p", `{"model":"vs-sd2.5-flash","prompt":"x","duration":4,"resolution":"1080p"}`, "resolution must be"},
		{"flash: 31 seconds", `{"model":"vs-sd2.5-flash","prompt":"x","duration":31}`, "from 4 to 30"},
		{"a: 3 seconds", `{"model":"vs-sd2.5-a","prompt":"x","duration":3}`, "from 4 to 30"},
		{"a: 31 seconds", `{"model":"Seedance2.5-vs-a","prompt":"x","duration":31}`, "from 4 to 30"},
		{"a: 2k", `{"model":"vs-sd2.5-a","prompt":"x","duration":4,"resolution":"2k"}`, "resolution must be"},
		{"a: prompt over 15000", `{"model":"vs-sd2.5-a","prompt":"` + strings.Repeat("a", 15001) + `","duration":4}`, "at most 15000 characters"},
		{"a: 11 reference videos", `{"model":"vs-sd2.5-a","prompt":"x","duration":4,"videos":[` + manyVideos + `,"https://example.com/v2.mp4"]}`, "at most 10"},
		{"a: adaptive ratio", `{"model":"vs-sd2.5-a","prompt":"x","duration":4,"ratio":"adaptive"}`, "ratio must be one of"},
		{"s: 1080p", `{"model":"vs-sd2.5-s","prompt":"x","duration":4,"resolution":"1080p"}`, "resolution must be"},
		{"s: 3 seconds", `{"model":"Seedance2.5-vs-s","prompt":"x","duration":3}`, "from 4 to 30"},
		{"s: audio alone", `{"model":"vs-sd2.5-s","prompt":"x","duration":4,"audios":["https://example.com/1.mp3"]}`, "reference audio must be sent with"},
		{"s: 31 items in total", `{"model":"vs-sd2.5-s","prompt":"x","duration":5,"images":[` + manyImages + `],"videos":[` + manyVideos + `],"audios":["https://example.com/1.mp3"]}`, "at most 30 reference items"},
		{"s: prompt over 15000", `{"model":"vs-sd2.5-s","prompt":"` + strings.Repeat("a", 15001) + `","duration":4}`, "at most 15000 characters"},
		{"nh: 4k", `{"model":"vs-sd2.5-nh","prompt":"x","duration":4,"resolution":"4k"}`, "resolution must be"},
		{"nh: 31 seconds", `{"model":"vs-sd2.5-nh","prompt":"x","duration":31}`, "from 4 to 30"},
		{"nh: audio alone", `{"model":"Seedance2.5-vs-nh","prompt":"x","duration":4,"audios":["https://example.com/1.mp3"]}`, "reference audio must be sent with"},
		{"nh: prompt over 15000", `{"model":"vs-sd2.5-nh","prompt":"` + strings.Repeat("a", 15001) + `","duration":4}`, "at most 15000 characters"},
		{"X: 480p", `{"model":"vs-sd2.5-X","prompt":"x","duration":4,"resolution":"480p"}`, "resolution must be"},
		{"X: audio with a video only", `{"model":"vs-sd2.5-X","prompt":"x","duration":4,"videos":["https://example.com/1.mp4"],"audios":["https://example.com/1.mp3"]}`, "at least one reference image for this model"},
		{"X: audio alone", `{"model":"Seedance2.5-vs-X","prompt":"x","duration":4,"audios":["https://example.com/1.mp3"]}`, "at least one reference image for this model"},
		{"X: prompt over 15000", `{"model":"vs-sd2.5-X","prompt":"` + strings.Repeat("a", 15001) + `","duration":4}`, "at most 15000 characters"},
		{"media array", `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"media":[{"type":"reference_image","url":"https://example.com/1.png"}]}`, "does not support media"},
		{"content array", `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"content":[]}`, "does not support content"},
		{"frames", `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"first_frame":"https://example.com/1.png"}`, "first or last frames"},
		{"audio switch", `{"model":"vs-sd2.5-Y","prompt":"x","duration":5,"generate_audio":true}`, "does not support generate_audio"},
	} {
		_, _, err := PreparePPVideoRequestBody(PlatformHongpeiSeedance, PPVideoOperationGeneric, []byte(tc.body))
		require.Error(t, err, tc.name)
		require.ErrorContains(t, err, tc.contains, tc.name)
	}
}

func TestHongpeiSeedancePricesAreFoundUnderEitherName(t *testing.T) {
	t.Parallel()

	const upstream, public = "vs-sd2.5-Y", "Seedance2.5-vs-Y"
	groupID := int64(94)
	publicPrice, upstreamPrice := 6.0, 5.0
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{
		Platform:             PlatformHongpeiSeedance,
		RateMultiplier:       1,
		VideoRateIndependent: true,
		VideoRateMultiplier:  1,
	}}
	cost := func(svc *OpenAIGatewayService, model string) (*CostBreakdown, error) {
		return svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
			Platform:                      PlatformHongpeiSeedance,
			Model:                         model, // the prepared request carries the upstream name
			RequestedDurationMilliseconds: 5000,
			VideoCount:                    1,
		})
	}
	service := func(prices ...ChannelModelPricing) *OpenAIGatewayService {
		return &OpenAIGatewayService{
			billingService: newTestBillingService(),
			resolver:       newPPVideoResolverWithChannel(t, groupID, PlatformHongpeiSeedance, prices),
		}
	}
	priced := func(model string, price *float64) ChannelModelPricing {
		return ChannelModelPricing{
			Platform: PlatformHongpeiSeedance, Models: []string{model}, BillingMode: BillingModePerRequest, PerRequestPrice: price,
		}
	}

	// A price configured under the public name is used for a request that carries the upstream name
	// (the reported failure: "no channel model price matched" for "vs-sd2.5-Y").
	got, err := cost(service(priced(public, &publicPrice)), upstream)
	require.NoError(t, err)
	require.InDelta(t, 6, got.TotalCost, 1e-9)

	// A price configured under the upstream name also applies.
	got, err = cost(service(priced(upstream, &upstreamPrice)), upstream)
	require.NoError(t, err)
	require.InDelta(t, 5, got.TotalCost, 1e-9)

	// When both exist the public name wins.
	got, err = cost(service(priced(upstream, &upstreamPrice), priced(public, &publicPrice)), upstream)
	require.NoError(t, err)
	require.InDelta(t, 6, got.TotalCost, 1e-9)

	// A price under another model's name never applies.
	_, err = cost(service(priced("Seedance2.5-vs-L", &publicPrice)), upstream)
	require.ErrorContains(t, err, "Hongpei Seedance")
}

// vs-sd2.5-a is billed per request with one price per resolution (the operator's 11 / 19 / 27 / 35
// credits): the channel holds four tiers labelled like the resolution the request is billed as, the
// length does not matter, and a request that names no resolution is billed as the default (480p).
func TestHongpeiSeedanceVsAIsPricedPerResolutionTier(t *testing.T) {
	t.Parallel()

	const upstream, public = "vs-sd2.5-a", "Seedance2.5-vs-a"
	groupID := int64(95)
	p480, p720, p1080, p4k := 11.0, 19.0, 27.0, 35.0
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
		resolver: newPPVideoResolverWithChannel(t, groupID, PlatformHongpeiSeedance, []ChannelModelPricing{{
			Platform: PlatformHongpeiSeedance, Models: []string{public}, BillingMode: BillingModePerRequest,
			Intervals: []PricingInterval{
				{TierLabel: "480p", PerRequestPrice: &p480},
				{TierLabel: "720p", PerRequestPrice: &p720},
				{TierLabel: "1080p", PerRequestPrice: &p1080},
				{TierLabel: "4k", PerRequestPrice: &p4k},
			},
		}}),
	}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{
		Platform:             PlatformHongpeiSeedance,
		RateMultiplier:       1,
		VideoRateIndependent: true,
		VideoRateMultiplier:  1,
	}}
	cost := func(resolution string, milliseconds int64) float64 {
		got, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
			Platform:                      PlatformHongpeiSeedance,
			Model:                         upstream, // the prepared request carries the upstream name
			VideoResolution:               resolution,
			RequestedDurationMilliseconds: milliseconds,
			VideoCount:                    1,
		})
		require.NoError(t, err, resolution)
		return got.TotalCost
	}

	require.InDelta(t, 11, cost("480p", 4000), 1e-9)
	require.InDelta(t, 19, cost("720p", 4000), 1e-9)
	require.InDelta(t, 27, cost("1080p", 4000), 1e-9)
	require.InDelta(t, 35, cost("4k", 4000), 1e-9)
	require.InDelta(t, 35, cost("4K", 30000), 1e-9, "tier labels are case-insensitive and the length is not billed")
	require.InDelta(t, 11, cost("480p", 30000), 1e-9)

	// The request that names no resolution is billed as the model's default.
	spec, ok := hongpeiModelSpecFor(upstream)
	require.True(t, ok)
	require.InDelta(t, 11, cost(spec.defaultResolution(), 4000), 1e-9)
}

// vs-sd2.5-nh is the one hongpei model billed per second: the channel is in "video" mode with one
// tier per resolution, and the cost is seconds times the tier's price.
func TestHongpeiSeedanceNhIsBilledPerSecondByResolutionTier(t *testing.T) {
	t.Parallel()

	const upstream, public = "vs-sd2.5-nh", "Seedance2.5-vs-nh"
	groupID := int64(96)
	p480, p720, p1080 := 0.5, 0.8, 1.5
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
		resolver: newPPVideoResolverWithChannel(t, groupID, PlatformHongpeiSeedance, []ChannelModelPricing{{
			Platform: PlatformHongpeiSeedance, Models: []string{public}, BillingMode: BillingModeVideo,
			Intervals: []PricingInterval{
				{TierLabel: "480p", PerRequestPrice: &p480},
				{TierLabel: "720p", PerRequestPrice: &p720},
				{TierLabel: "1080p", PerRequestPrice: &p1080},
			},
		}}),
	}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{
		Platform:             PlatformHongpeiSeedance,
		RateMultiplier:       1,
		VideoRateIndependent: true,
		VideoRateMultiplier:  1,
	}}
	cost := func(resolution string, seconds int64) float64 {
		got, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, PPVideoBillingMetadata{
			Platform:                      PlatformHongpeiSeedance,
			Model:                         upstream, // the prepared request carries the upstream name
			VideoResolution:               resolution,
			RequestedDurationMilliseconds: seconds * 1000,
			VideoCount:                    1,
		})
		require.NoError(t, err, resolution)
		return got.TotalCost
	}
	require.InDelta(t, 4*0.5, cost("480p", 4), 1e-9)
	require.InDelta(t, 10*0.8, cost("720p", 10), 1e-9)
	require.InDelta(t, 30*1.5, cost("1080p", 30), 1e-9)

	// A request that names no resolution is billed as the default (720p).
	spec, ok := hongpeiModelSpecFor(upstream)
	require.True(t, ok)
	require.InDelta(t, 5*0.8, cost(spec.defaultResolution(), 5), 1e-9)
}

func TestHongpeiSeedanceCapabilities(t *testing.T) {
	t.Parallel()

	for _, model := range hongpeiSeedanceModels {
		var capabilities PPVideoModelCapabilities
		fillPPVideoHongpeiCapabilities(&capabilities, model)
		require.Equal(t, &PPVideoDurationRange{Min: model.spec.MinSeconds, Max: model.spec.MaxSeconds}, capabilities.Duration, model.spec.Name)
		require.Equal(t, model.spec.Resolutions, capabilities.Resolutions, model.spec.Name)
		require.Equal(t, model.ratios, capabilities.Ratios, model.spec.Name)
		require.NotNil(t, capabilities.Assets, model.spec.Name)
	}
}
