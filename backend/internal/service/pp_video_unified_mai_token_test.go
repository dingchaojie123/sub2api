package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func maiTokenUnifiedTestAssets(kind string, count int) string {
	extension := map[string]string{"image": "png", "video": "mp4", "audio": "mp3"}[kind]
	items := make([]string, 0, count)
	for index := 0; index < count; index++ {
		items = append(items, fmt.Sprintf(`{"type":%q,"url":"https://example.com/%s%d.%s"}`, kind, kind, index, extension))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func TestTranslatePPVideoUnifiedRequestToMaiToken(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "references with every optional field",
			body: `{
				"model": "Seedance2.5-480全参",
				"prompt": "保持参考主体，镜头平稳推进",
				"duration": 8,
				"resolution": "480P",
				"ratio": "9:16",
				"generate_audio": false,
				"watermark": false,
				"assets": [
					{"type": "image", "name": "a.jpg", "url": "https://example.com/a.jpg"},
					{"type": "image", "url": "https://example.com/b.jpg"},
					{"type": "video", "url": "https://example.com/a.mp4"},
					{"type": "audio", "url": "https://example.com/a.mp3"}
				]
			}`,
			want: `{
				"model": "Seedance2.5-480全参",
				"prompt": "保持参考主体，镜头平稳推进",
				"duration": 8,
				"resolution": "480p",
				"ratio": "9:16",
				"generate_audio": false,
				"watermark": false,
				"referenceImages": ["https://example.com/a.jpg", "https://example.com/b.jpg"],
				"referenceVideos": ["https://example.com/a.mp4"],
				"referenceAudios": ["https://example.com/a.mp3"]
			}`,
		},
		{
			name: "first frame alone",
			body: `{"model":"Seedance2.5-480全参","prompt":"p","assets":[{"type":"image","role":"first_frame","url":"https://example.com/first.jpg"}]}`,
			want: `{"model":"Seedance2.5-480全参","prompt":"p","first_image":"https://example.com/first.jpg"}`,
		},
		{
			name: "first and last frame",
			body: `{"model":"Seedance2.5-480全参","prompt":"p","assets":[{"type":"image","role":"last_frame","url":"https://example.com/last.jpg"},{"type":"image","role":"first_frame","url":"https://example.com/first.jpg"}]}`,
			want: `{"model":"Seedance2.5-480全参","prompt":"p","first_image":"https://example.com/first.jpg","last_image":"https://example.com/last.jpg"}`,
		},
		{
			name: "nothing but the required fields",
			body: `{"model":"Seedance2.5-480全参","prompt":"p"}`,
			want: `{"model":"Seedance2.5-480全参","prompt":"p"}`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			translated, err := TranslatePPVideoUnifiedRequest(PlatformMaiTokenSeedance, []byte(tc.body))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(translated))
		})
	}
}

func TestUnifiedMaiTokenReferencesMatchNativeRequestThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformMaiTokenSeedance, `{
		"model": "Seedance2.5-480全参",
		"prompt": "保持参考主体，镜头平稳推进",
		"duration": 8,
		"resolution": "480p",
		"ratio": "9:16",
		"generate_audio": false,
		"watermark": false,
		"assets": [
			{"type": "image", "url": "https://example.com/a.jpg"},
			{"type": "video", "url": "https://example.com/a.mp4"},
			{"type": "audio", "url": "https://example.com/a.mp3"}
		]
	}`, `{
		"model": "Seedance2.5-480全参",
		"prompt": "保持参考主体，镜头平稳推进",
		"duration": 8,
		"resolution": "480p",
		"ratio": "9:16",
		"generate_audio": false,
		"watermark": false,
		"referenceImages": ["https://example.com/a.jpg"],
		"referenceVideos": ["https://example.com/a.mp4"],
		"referenceAudios": ["https://example.com/a.mp3"]
	}`)
}

func TestUnifiedMaiTokenFramesMatchNativeRequestThroughExistingPipeline(t *testing.T) {
	t.Parallel()

	requireUnifiedMatchesNative(t, PlatformMaiTokenSeedance, `{
		"model": "Seedance2.5-480全参",
		"prompt": "保持首帧主体和构图",
		"duration": 4,
		"resolution": "480p",
		"ratio": "adaptive",
		"assets": [
			{"type": "image", "role": "first_frame", "url": "https://example.com/first.jpg"},
			{"type": "image", "role": "last_frame", "url": "https://example.com/last.jpg"}
		]
	}`, `{
		"model": "Seedance2.5-480全参",
		"prompt": "保持首帧主体和构图",
		"duration": 4,
		"resolution": "480p",
		"ratio": "adaptive",
		"first_image": "https://example.com/first.jpg",
		"last_image": "https://example.com/last.jpg"
	}`)
}

func TestUnifiedMaiTokenOmittedFieldsMatchNativeDefaults(t *testing.T) {
	t.Parallel()

	// The native request applies its own defaults (4 seconds, 480p); the translated
	// request leaves the fields out so those same defaults apply.
	requireUnifiedMatchesNative(t, PlatformMaiTokenSeedance,
		`{"model":"Seedance2.5-480全参","prompt":"海边日落"}`,
		`{"model":"Seedance2.5-480全参","prompt":"海边日落"}`,
	)
}

func TestTranslatePPVideoUnifiedRequestRejectsInvalidMaiTokenRequests(t *testing.T) {
	t.Parallel()

	const base = `{"model":"Seedance2.5-480全参","prompt":"x"`
	const (
		firstFrame = `{"type":"image","role":"first_frame","url":"https://example.com/first.jpg"}`
		lastFrame  = `{"type":"image","role":"last_frame","url":"https://example.com/last.jpg"}`
		reference  = `{"type":"image","url":"https://example.com/ref.jpg"}`
	)

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"missing model", `{"prompt":"x"}`, "model: is required for this model"},
		{"another model", `{"model":"sdtj-2.5-720p","prompt":"x"}`, "model: must be Seedance2.5-480全参 for this model"},
		{"missing prompt", `{"model":"Seedance2.5-480全参"}`, "prompt: is required for this model"},
		{"duration too short", base + `,"duration":3}`, "duration: must be between 4 and 30 for this model"},
		{"duration too long", base + `,"duration":31}`, "duration: must be between 4 and 30 for this model"},
		{"resolution", base + `,"resolution":"720p"}`, "resolution: must be 480p for this model"},
		{"seed", base + `,"seed":1}`, "seed: is not supported by this model"},
		{"callback_url", base + `,"callback_url":"https://example.com/cb"}`, "callback_url: is not supported by this model"},
		{"callback_token", base + `,"callback_token":"t"}`, "callback_token: is not supported by this model"},
		{"metadata", base + `,"metadata":{"a":1}}`, "metadata: is not supported by this model"},
		{"unknown option", base + `,"options":{"foo":1}}`, "options.foo: is not a supported option for this model"},
		{"private address", base + `,"assets":[{"type":"image","url":"http://127.0.0.1/a.png"}]}`, "assets[0].url: must be a public HTTP(S) URL"},
		{"not an http url", base + `,"assets":[{"type":"image","url":"ftp://example.com/a.png"}]}`, "assets[0].url: must be a public HTTP(S) URL"},
		{"last frame without first frame", base + `,"assets":[` + lastFrame + `]}`, "assets[0].role: last_frame requires a first_frame for this model"},
		{"frames with references", base + `,"assets":[` + firstFrame + `,` + reference + `]}`, "assets: first_frame and last_frame images cannot be combined with reference assets for this model"},
		{"too many reference images", base + `,"assets":` + maiTokenUnifiedTestAssets("image", 31) + `}`, "assets[30]: too many reference images; this model allows at most 30"},
		{"too many reference videos", base + `,"assets":` + maiTokenUnifiedTestAssets("video", 11) + `}`, "assets[10]: too many reference videos; this model allows at most 10"},
		{"too many reference audios", base + `,"assets":` + maiTokenUnifiedTestAssets("audio", 11) + `}`, "assets[10]: too many reference audios; this model allows at most 10"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := TranslatePPVideoUnifiedRequest(PlatformMaiTokenSeedance, []byte(tc.body))
			require.Error(t, err)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestPPVideoCapabilitiesForMaiTokenModel(t *testing.T) {
	t.Parallel()

	capabilities := PPVideoCapabilitiesForModel(PlatformMaiTokenSeedance, MaiTokenSeedancePublicModel)

	require.Equal(t, []string{"v1", "v2"}, capabilities.Protocols)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, capabilities.Duration)
	require.Equal(t, []string{"480p"}, capabilities.Resolutions)
	require.Empty(t, capabilities.Ratios)
	require.True(t, *capabilities.Assets.FirstLastFrame)
	require.Equal(t, 30, ppVideoCapabilityIntValue(capabilities.Assets.ReferenceImages))
	require.Equal(t, 10, ppVideoCapabilityIntValue(capabilities.Assets.ReferenceVideos))
	require.Equal(t, 10, ppVideoCapabilityIntValue(capabilities.Assets.ReferenceAudios))
	require.True(t, *capabilities.GenerateAudio)
	require.False(t, *capabilities.Callback)
}

func TestPPVideoCapabilitiesAgreeWithMaiTokenValidation(t *testing.T) {
	t.Parallel()

	capabilities := PPVideoCapabilitiesForModel(PlatformMaiTokenSeedance, MaiTokenSeedancePublicModel)
	native := func(extra string) string {
		return `{"model":"Seedance2.5-480全参","prompt":"x"` + extra + `}`
	}
	requireCapabilityProbeAccepts(t, PlatformMaiTokenSeedance, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Min)), "min duration")
	requireCapabilityProbeAccepts(t, PlatformMaiTokenSeedance, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Max)), "max duration")
	requireCapabilityProbeRejects(t, PlatformMaiTokenSeedance, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Min-1)), "below min duration")
	requireCapabilityProbeRejects(t, PlatformMaiTokenSeedance, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Max+1)), "above max duration")
	for _, resolution := range capabilities.Resolutions {
		requireCapabilityProbeAccepts(t, PlatformMaiTokenSeedance, native(fmt.Sprintf(`,"resolution":%q`, resolution)), "resolution "+resolution)
	}
	requireCapabilityProbeRejects(t, PlatformMaiTokenSeedance, native(`,"resolution":"720p"`), "720p is not a supported resolution")

	// The advertised reference limits are accepted by the translation and by the
	// native validation behind it, and one more is rejected.
	for _, tc := range []struct {
		kind  string
		limit int
	}{
		{"image", ppVideoCapabilityIntValue(capabilities.Assets.ReferenceImages)},
		{"video", ppVideoCapabilityIntValue(capabilities.Assets.ReferenceVideos)},
		{"audio", ppVideoCapabilityIntValue(capabilities.Assets.ReferenceAudios)},
	} {
		accepted := native(`,"assets":` + maiTokenUnifiedTestAssets(tc.kind, tc.limit))
		translated, err := TranslatePPVideoUnifiedRequest(PlatformMaiTokenSeedance, []byte(accepted))
		require.NoError(t, err, "%s limit %d", tc.kind, tc.limit)
		requireCapabilityProbeAccepts(t, PlatformMaiTokenSeedance, string(translated), tc.kind+" limit through the native validation")

		rejected := native(`,"assets":` + maiTokenUnifiedTestAssets(tc.kind, tc.limit+1))
		_, err = TranslatePPVideoUnifiedRequest(PlatformMaiTokenSeedance, []byte(rejected))
		require.Error(t, err, "%s limit %d plus one", tc.kind, tc.limit)
	}
}
