package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func ppVideoCapabilityIntValue(value *int) int {
	if value == nil {
		return -1
	}
	return *value
}

func TestPPVideoCapabilitiesProtocolsFollowUnifiedSupport(t *testing.T) {
	t.Parallel()

	for _, platform := range []string{
		PlatformByteDance,
		PlatformWan3,
		PlatformPixverseV6,
		PlatformMiniMaxH3CompShare,
		Platform88APIVideo,
		PlatformGuangying,
		PlatformMaiTokenSeedance,
		PlatformXingAPISeedance,
		PlatformJijiuSeedance,
		PlatformCTMOSeedance,
		PlatformSeedanceHub,
	} {
		capabilities := PPVideoCapabilitiesForModel(platform, "any-model")
		require.Equal(t, []string{"v1", "v2"}, capabilities.Protocols, "platform=%s", platform)
	}
	for _, platform := range []string{PlatformSeedance, PlatformKling, PlatformMiniMaxH3, PlatformOPONE} {
		capabilities := PPVideoCapabilitiesForModel(platform, "any-model")
		require.Equal(t, []string{"v1"}, capabilities.Protocols, "platform=%s", platform)
	}
}

func TestPPVideoCapabilitiesLeaveOutWhatIsNotKnown(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ platform, model string }{
		{PlatformByteDance, "not-a-doubao-model"},
		{PlatformWan3, "not-a-wan-model"},
		{PlatformPixverseV6, "not-a-pixverse-model"},
		{PlatformMiniMaxH3CompShare, "not-an-h3-model"},
		// Public aliases of differently named upstream models: not derived from the name.
		{Platform88APIVideo, "seedance-2.0-fast"},
		{Platform88APIVideo, "kling-3.0"},
		{Platform88APIVideo, "seedance-2.5"},
		{PlatformGuangying, "Seedance 2.5-全参720P"},
		{PlatformMaiTokenSeedance, "seedance-2.5"},
		{PlatformMaiTokenSeedance, "SDTJ-2.5-480P"},
		{PlatformSeedance, "seedance-933"},
		// Supported through v2, but the upstream documents its limits per model only in prose.
		{PlatformXingAPISeedance, "seedance-933"},
		{PlatformXingAPISeedance, "seedance2.5-V"},
	} {
		capabilities := PPVideoCapabilitiesForModel(tc.platform, tc.model)
		require.Nil(t, capabilities.Duration, "%s %s", tc.platform, tc.model)
		require.Empty(t, capabilities.Resolutions, "%s %s", tc.platform, tc.model)
		require.Empty(t, capabilities.Ratios, "%s %s", tc.platform, tc.model)
		require.Nil(t, capabilities.Assets, "%s %s", tc.platform, tc.model)
		require.Nil(t, capabilities.GenerateAudio, "%s %s", tc.platform, tc.model)
		require.Nil(t, capabilities.Callback, "%s %s", tc.platform, tc.model)
	}
}

func TestPPVideoCapabilitiesForByteDanceModels(t *testing.T) {
	t.Parallel()

	legacy := PPVideoCapabilitiesForModel(PlatformByteDance, ByteDanceVideoDefaultModel)
	require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 15}, legacy.Duration)
	require.Equal(t, []string{"480p", "720p", "1080p", "4k"}, legacy.Resolutions)
	require.Equal(t, []string{"adaptive", "16:9", "4:3", "1:1", "3:4", "9:16", "21:9"}, legacy.Ratios)
	require.True(t, *legacy.GenerateAudio)
	require.True(t, *legacy.Callback)
	require.True(t, *legacy.Assets.FirstLastFrame)

	for _, model := range []string{ByteDanceSeedance25Model, ByteDanceSeedance25GlobalModel} {
		capabilities := PPVideoCapabilitiesForModel(PlatformByteDance, model)
		require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, capabilities.Duration, model)
		require.Equal(t, []string{"480p", "720p", "1080p"}, capabilities.Resolutions, model)
	}
}

func TestPPVideoCapabilitiesForWanPixverseAndH3(t *testing.T) {
	t.Parallel()

	for _, model := range []string{Wan30VideoDefaultModel, Wan30VideoPrimeModel} {
		wan := PPVideoCapabilitiesForModel(PlatformWan3, model)
		require.Equal(t, &PPVideoDurationRange{Min: 2, Max: 30}, wan.Duration, model)
		require.Equal(t, []string{"480p", "720p", "1080p"}, wan.Resolutions, model)
		require.Equal(t, []string{"adaptive", "16:9", "9:16", "1:1", "4:3", "3:4"}, wan.Ratios, model)
		require.Equal(t, 10, ppVideoCapabilityIntValue(wan.Assets.ReferenceImages), model)
		require.Equal(t, 5, ppVideoCapabilityIntValue(wan.Assets.ReferenceVideos), model)
		require.Equal(t, 5, ppVideoCapabilityIntValue(wan.Assets.ReferenceAudios), model)
		require.True(t, *wan.GenerateAudio, model)
		require.False(t, *wan.Callback, model)
	}

	pixverse := PPVideoCapabilitiesForModel(PlatformPixverseV6, PixverseV6VideoDefaultModel)
	require.Equal(t, &PPVideoDurationRange{Min: 1, Max: 15}, pixverse.Duration)
	require.Equal(t, []string{"360p", "540p", "720p", "1080p"}, pixverse.Resolutions)
	require.Equal(t, []string{"16:9", "4:3", "1:1", "3:4", "9:16", "2:3", "3:2", "21:9"}, pixverse.Ratios)
	require.Equal(t, 1, ppVideoCapabilityIntValue(pixverse.Assets.ReferenceImages))
	require.Equal(t, 1, ppVideoCapabilityIntValue(pixverse.Assets.ReferenceVideos))
	require.Equal(t, 0, ppVideoCapabilityIntValue(pixverse.Assets.ReferenceAudios))

	for _, model := range []string{MiniMaxH3VideoDefaultModel, CompShareVideoLiteModel} {
		h3 := PPVideoCapabilitiesForModel(PlatformMiniMaxH3CompShare, model)
		require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 30}, h3.Duration, model)
		require.Equal(t, []string{"480p", "768p", "1080p", "2k", "4k"}, h3.Resolutions, model)
		require.Equal(t, 9, ppVideoCapabilityIntValue(h3.Assets.ReferenceImages), model)
		require.Equal(t, 3, ppVideoCapabilityIntValue(h3.Assets.ReferenceVideos), model)
		require.Equal(t, 3, ppVideoCapabilityIntValue(h3.Assets.ReferenceAudios), model)
		require.True(t, *h3.Callback, model)
	}
}

func TestPPVideoCapabilitiesForGeneralModels(t *testing.T) {
	t.Parallel()

	grok := PPVideoCapabilitiesForModel(Platform88APIVideo, "grok-imagine-video")
	require.Equal(t, &PPVideoDurationRange{Min: 1, Max: 15}, grok.Duration)
	require.Equal(t, []string{"480p", "720p"}, grok.Resolutions)
	require.Equal(t, []string{"1:1", "16:9", "9:16", "3:4", "4:3", "3:2", "2:3"}, grok.Ratios)
	require.False(t, *grok.GenerateAudio)

	grok15 := PPVideoCapabilitiesForModel(Platform88APIVideo, "grok-imagine-video-1.5")
	require.Equal(t, []string{"480p", "720p", "1080p"}, grok15.Resolutions)

	flash := PPVideoCapabilitiesForModel(Platform88APIVideo, "gemini-omni-flash")
	require.Equal(t, &PPVideoDurationRange{Min: 3, Max: 10}, flash.Duration)
	require.Equal(t, []string{"720p"}, flash.Resolutions)
	require.Equal(t, []string{"16:9", "9:16"}, flash.Ratios)

	flash11 := PPVideoCapabilitiesForModel(Platform88APIVideo, "gemini-omni-flash-1.1")
	require.Equal(t, []string{"720p", "1080p", "4k"}, flash11.Resolutions)

	for _, model := range []string{"veo-3.1", "veo-3.1-fast"} {
		veo := PPVideoCapabilitiesForModel(Platform88APIVideo, model)
		require.Equal(t, &PPVideoDurationRange{Min: 4, Max: 8, Values: []int{4, 6, 8}}, veo.Duration, model)
		require.Equal(t, []string{"720p", "1080p", "4k"}, veo.Resolutions, model)
		require.Equal(t, []string{"16:9", "9:16"}, veo.Ratios, model)
		require.True(t, *veo.GenerateAudio, model)
	}

	wan := PPVideoCapabilitiesForModel(Platform88APIVideo, "WAN3.0-VIDEO")
	require.Equal(t, &PPVideoDurationRange{Min: 2, Max: 30}, wan.Duration)
	require.Equal(t, []string{"480p", "720p", "1080p"}, wan.Resolutions)
	require.Equal(t, []string{"adaptive", "1:1", "21:9", "16:9", "9:16", "3:4", "4:3"}, wan.Ratios)
}

func TestPPVideoCapabilitiesJSONShape(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(PPVideoCapabilitiesForModel(PlatformPixverseV6, PixverseV6VideoDefaultModel))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"protocols": ["v1", "v2"],
		"duration": {"min": 1, "max": 15},
		"resolutions": ["360p", "540p", "720p", "1080p"],
		"ratios": ["16:9", "4:3", "1:1", "3:4", "9:16", "2:3", "3:2", "21:9"],
		"assets": {"first_last_frame": true, "reference_images": 1, "reference_videos": 1, "reference_audios": 0},
		"generate_audio": true,
		"callback": false
	}`, string(encoded))

	// A model without detailed data serializes to the protocols alone.
	bare, err := json.Marshal(PPVideoCapabilitiesForModel(PlatformKling, "kling-v3"))
	require.NoError(t, err)
	require.JSONEq(t, `{"protocols": ["v1"]}`, string(bare))
}

func requireCapabilityProbeAccepts(t *testing.T, platform string, body string, message string) {
	t.Helper()
	_, _, err := PreparePPVideoRequestBody(platform, PPVideoOperationGeneric, []byte(body))
	require.NoError(t, err, message)
}

func requireCapabilityProbeRejects(t *testing.T, platform string, body string, message string) {
	t.Helper()
	_, _, err := PreparePPVideoRequestBody(platform, PPVideoOperationGeneric, []byte(body))
	require.Error(t, err, message)
}

// The capability data must agree with the validators that actually process
// requests: what it advertises is accepted, and the first value outside it is not.
func TestPPVideoCapabilitiesAgreeWithByteDanceValidation(t *testing.T) {
	t.Parallel()

	for _, model := range []string{ByteDanceVideoDefaultModel, ByteDanceSeedance25Model, ByteDanceSeedance25GlobalModel} {
		capabilities := PPVideoCapabilitiesForModel(PlatformByteDance, model)
		body := func(duration int, resolution string) string {
			return fmt.Sprintf(`{"model":%q,"prompt":"x","duration":%d,"resolution":%q}`, model, duration, resolution)
		}
		requireCapabilityProbeAccepts(t, PlatformByteDance, body(capabilities.Duration.Min, "720p"), model+" min duration")
		requireCapabilityProbeAccepts(t, PlatformByteDance, body(capabilities.Duration.Max, "720p"), model+" max duration")
		requireCapabilityProbeRejects(t, PlatformByteDance, body(capabilities.Duration.Min-1, "720p"), model+" below min duration")
		requireCapabilityProbeRejects(t, PlatformByteDance, body(capabilities.Duration.Max+1, "720p"), model+" above max duration")
		for _, resolution := range capabilities.Resolutions {
			requireCapabilityProbeAccepts(t, PlatformByteDance, body(5, resolution), model+" resolution "+resolution)
		}
		if !byteDanceIsSeedance25Model(model) {
			continue
		}
		requireCapabilityProbeRejects(t, PlatformByteDance, body(5, "4k"), model+" must not accept 4k")
	}
}

func TestPPVideoCapabilitiesAgreeWithWanValidation(t *testing.T) {
	t.Parallel()

	capabilities := PPVideoCapabilitiesForModel(PlatformWan3, Wan30VideoDefaultModel)
	body := func(duration int, resolution string, ratio string) string {
		return fmt.Sprintf(`{"model":"wan3.0-video","prompt":"x","duration":%d,"resolution":%q,"ratio":%q}`, duration, resolution, ratio)
	}
	requireCapabilityProbeAccepts(t, PlatformWan3, body(capabilities.Duration.Min, "720p", "16:9"), "min duration")
	requireCapabilityProbeAccepts(t, PlatformWan3, body(capabilities.Duration.Max, "720p", "16:9"), "max duration")
	requireCapabilityProbeRejects(t, PlatformWan3, body(capabilities.Duration.Min-1, "720p", "16:9"), "below min duration")
	requireCapabilityProbeRejects(t, PlatformWan3, body(capabilities.Duration.Max+1, "720p", "16:9"), "above max duration")
	for _, resolution := range capabilities.Resolutions {
		requireCapabilityProbeAccepts(t, PlatformWan3, body(5, resolution, "16:9"), "resolution "+resolution)
	}
	requireCapabilityProbeRejects(t, PlatformWan3, body(5, "4k", "16:9"), "4k is not a Wan resolution")
	for _, ratio := range capabilities.Ratios {
		requireCapabilityProbeAccepts(t, PlatformWan3, body(5, "720p", ratio), "ratio "+ratio)
	}
	requireCapabilityProbeRejects(t, PlatformWan3, body(5, "720p", "5:4"), "5:4 is not a Wan ratio")
}

func TestPPVideoCapabilitiesAgreeWithPixverseValidation(t *testing.T) {
	t.Parallel()

	capabilities := PPVideoCapabilitiesForModel(PlatformPixverseV6, PixverseV6VideoDefaultModel)
	body := func(extra string) string {
		return `{"model":"pixverse-v6","prompt":"x"` + extra + `}`
	}
	requireCapabilityProbeAccepts(t, PlatformPixverseV6, body(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Min)), "min duration")
	requireCapabilityProbeAccepts(t, PlatformPixverseV6, body(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Max)), "max duration")
	requireCapabilityProbeRejects(t, PlatformPixverseV6, body(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Max+1)), "above max duration")
	for _, resolution := range capabilities.Resolutions {
		requireCapabilityProbeAccepts(t, PlatformPixverseV6, body(fmt.Sprintf(`,"duration":5,"resolution":%q`, resolution)), "resolution "+resolution)
	}
	requireCapabilityProbeRejects(t, PlatformPixverseV6, body(`,"duration":5,"resolution":"2k"`), "2k is not a Pixverse resolution")
	for _, ratio := range capabilities.Ratios {
		requireCapabilityProbeAccepts(t, PlatformPixverseV6, body(fmt.Sprintf(`,"duration":5,"aspect_ratio":%q`, ratio)), "ratio "+ratio)
	}
	requireCapabilityProbeRejects(t, PlatformPixverseV6, body(`,"duration":5,"aspect_ratio":"5:4"`), "5:4 is not a Pixverse ratio")
}

func TestPPVideoCapabilitiesAgreeWithCompShareValidation(t *testing.T) {
	t.Parallel()

	capabilities := PPVideoCapabilitiesForModel(PlatformMiniMaxH3CompShare, MiniMaxH3VideoDefaultModel)
	native := func(extra string) string {
		return `{"model":"MiniMax-H3","content":[{"type":"text","text":"x"}]` + extra + `}`
	}
	requireCapabilityProbeAccepts(t, PlatformMiniMaxH3CompShare, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Min)), "min duration")
	requireCapabilityProbeAccepts(t, PlatformMiniMaxH3CompShare, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Max)), "max duration")
	requireCapabilityProbeRejects(t, PlatformMiniMaxH3CompShare, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Min-1)), "below min duration")
	requireCapabilityProbeRejects(t, PlatformMiniMaxH3CompShare, native(fmt.Sprintf(`,"duration":%d`, capabilities.Duration.Max+1)), "above max duration")
	for _, ratio := range capabilities.Ratios {
		requireCapabilityProbeAccepts(t, PlatformMiniMaxH3CompShare, native(fmt.Sprintf(`,"ratio":%q`, ratio)), "ratio "+ratio)
	}
	requireCapabilityProbeRejects(t, PlatformMiniMaxH3CompShare, native(`,"ratio":"5:4"`), "5:4 is not an H3 ratio")

	// The native request falls back to 768P silently, so resolutions are probed
	// through the unified translation, which validates them.
	for _, resolution := range capabilities.Resolutions {
		_, err := TranslatePPVideoUnifiedRequest(PlatformMiniMaxH3CompShare, []byte(fmt.Sprintf(`{"prompt":"x","resolution":%q}`, resolution)))
		require.NoError(t, err, "resolution "+resolution)
	}
	_, err := TranslatePPVideoUnifiedRequest(PlatformMiniMaxH3CompShare, []byte(`{"prompt":"x","resolution":"720p"}`))
	require.Error(t, err)
}

// The unified translation enforces the same asset limits the capabilities list.
func TestPPVideoCapabilitiesAgreeWithUnifiedAssetLimits(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		platform string
		model    string
	}{
		{PlatformWan3, Wan30VideoDefaultModel},
		{PlatformPixverseV6, PixverseV6VideoDefaultModel},
		{PlatformMiniMaxH3CompShare, MiniMaxH3VideoDefaultModel},
	} {
		limit := ppVideoCapabilityIntValue(PPVideoCapabilitiesForModel(tc.platform, tc.model).Assets.ReferenceImages)
		accepted := fmt.Sprintf(`{"prompt":"x","assets":%s}`, ppVideoUnifiedTestImageAssets(limit))
		_, err := TranslatePPVideoUnifiedRequest(tc.platform, []byte(accepted))
		require.NoError(t, err, "%s accepts %d reference images", tc.platform, limit)

		rejected := fmt.Sprintf(`{"prompt":"x","assets":%s}`, ppVideoUnifiedTestImageAssets(limit+1))
		_, err = TranslatePPVideoUnifiedRequest(tc.platform, []byte(rejected))
		require.Error(t, err, "%s rejects %d reference images", tc.platform, limit+1)
	}
}

// The overview table in the customer documentation is written by hand. This keeps
// its duration and resolution columns in step with the capability data.
func TestPPVideoCapabilitiesMatchTheDocumentedOverviewTable(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/PP_VIDEO_GENERATION_API.md")
	if err != nil {
		t.Skipf("customer documentation is not available: %v", err)
	}
	rows := strings.Split(string(raw), "\n")
	findRow := func(prefix string) string {
		for _, row := range rows {
			if strings.HasPrefix(row, prefix) {
				return strings.ToLower(row)
			}
		}
		return ""
	}

	for _, tc := range []struct {
		rowPrefix string
		platform  string
		model     string
	}{
		{"| Seedance（doubao 系列） |", PlatformByteDance, ByteDanceVideoDefaultModel},
		{"| Seedance（doubao 系列） |", PlatformByteDance, ByteDanceSeedance25Model},
		{"| Wan3.0 |", PlatformWan3, Wan30VideoDefaultModel},
		{"| MiniMax-H3 |", PlatformMiniMaxH3CompShare, MiniMaxH3VideoDefaultModel},
		{"| Pixverse-V6 |", PlatformPixverseV6, PixverseV6VideoDefaultModel},
		{"| Seedance 2.5 480P |", PlatformMaiTokenSeedance, MaiTokenSeedancePublicModel},
	} {
		row := findRow(tc.rowPrefix)
		require.NotEmpty(t, row, "documentation row %q", tc.rowPrefix)

		capabilities := PPVideoCapabilitiesForModel(tc.platform, tc.model)
		duration := fmt.Sprintf("%d-%d", capabilities.Duration.Min, capabilities.Duration.Max)
		require.Contains(t, row, duration, "%s duration in the documentation", tc.model)
		for _, resolution := range capabilities.Resolutions {
			require.Contains(t, row, "`"+resolution+"`", "%s resolution %s in the documentation", tc.model, resolution)
		}
	}
}
