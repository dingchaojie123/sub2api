package service

import (
	"fmt"
	"strings"
)

// Video model capabilities, exposed to clients through GET /v1/models so they can
// validate a request before sending it. Values are taken from the existing
// validators wherever those can be called (resolutions, ratios, tiers, limits);
// the few values that exist only as literals inside a normalizer (for example the
// duration range) are repeated here and checked against the normalizer by
// pp_video_capabilities_test.go. A field that is not known for certain is left
// out instead of guessed.

const (
	PPVideoProtocolV1 = "v1"
	PPVideoProtocolV2 = "v2"
)

type PPVideoDurationRange struct {
	Min    int   `json:"min"`
	Max    int   `json:"max"`
	Values []int `json:"values,omitempty"`
}

type PPVideoAssetCapabilities struct {
	FirstLastFrame  *bool `json:"first_last_frame,omitempty"`
	ReferenceImages *int  `json:"reference_images,omitempty"`
	ReferenceVideos *int  `json:"reference_videos,omitempty"`
	ReferenceAudios *int  `json:"reference_audios,omitempty"`
}

// PPVideoModelCapabilities describes what a video model accepts. Protocols is
// always present; every other field is omitted when it is not known.
type PPVideoModelCapabilities struct {
	Protocols     []string                  `json:"protocols"`
	Duration      *PPVideoDurationRange     `json:"duration,omitempty"`
	Resolutions   []string                  `json:"resolutions,omitempty"`
	Ratios        []string                  `json:"ratios,omitempty"`
	Assets        *PPVideoAssetCapabilities `json:"assets,omitempty"`
	GenerateAudio *bool                     `json:"generate_audio,omitempty"`
	Callback      *bool                     `json:"callback,omitempty"`
}

func ppVideoCapInt(value int) *int {
	return &value
}

func ppVideoCapBool(value bool) *bool {
	return &value
}

// ppVideoGeneralCapabilityModels lists the general-video-model IDs whose public
// name is the same string the platform's capability functions are written for.
// Other IDs in that group (for example seedance-2.0-fast or kling-3.0) are public
// aliases of differently named upstream models, so deriving capabilities from
// their public name would give wrong answers; they only report their protocols.
var ppVideoGeneralCapabilityModels = []string{
	"grok-imagine-video",
	"grok-imagine-video-1.5",
	"gemini-omni-flash",
	"gemini-omni-flash-1.1",
	"veo-3.1",
	"veo-3.1-fast",
	"wan3.0-video",
}

// PPVideoCapabilitiesForModel returns the capabilities of one model of a video
// platform. Models without detailed data only report which request protocols
// their group accepts.
func PPVideoCapabilitiesForModel(platform string, model string) PPVideoModelCapabilities {
	platform = strings.TrimSpace(platform)
	model = strings.TrimSpace(model)

	capabilities := PPVideoModelCapabilities{Protocols: []string{PPVideoProtocolV1}}
	if IsPPVideoUnifiedPlatform(platform) {
		capabilities.Protocols = append(capabilities.Protocols, PPVideoProtocolV2)
	}

	switch platform {
	case PlatformByteDance:
		fillPPVideoByteDanceCapabilities(&capabilities, model)
	case PlatformWan3:
		if ppVideoIsWan30Model(model) {
			fillPPVideoWanCapabilities(&capabilities)
		}
	case PlatformPixverseV6:
		if ppVideoIsPixverseV6Model(model) {
			fillPPVideoPixverseCapabilities(&capabilities)
		}
	case PlatformMiniMaxH3CompShare:
		if strings.EqualFold(model, MiniMaxH3VideoDefaultModel) || strings.EqualFold(model, CompShareVideoLiteModel) {
			fillPPVideoCompShareCapabilities(&capabilities)
		}
	case Platform88APIVideo:
		fillPPVideoGeneralModelCapabilities(&capabilities, model)
	case PlatformMaiTokenSeedance:
		if _, ok := ppVideoMaiTokenPublicModel(model); ok {
			fillPPVideoMaiTokenCapabilities(&capabilities)
		}
	case PlatformJijiuSeedance:
		if spec, ok := jijiuModelSpecFor(model); ok {
			fillPPVideoJijiuCapabilities(&capabilities, spec)
		}
	case PlatformSeedanceHub:
		if line, ok := SeedanceHubLineByName(model); ok {
			fillSeedanceHubCapabilities(&capabilities, line)
		}
	}
	return capabilities
}

// fillPPVideoJijiuCapabilities reports the limits of one model, read from the same table the
// request normalizer enforces. Frames, the audio switch and callbacks are not offered.
func fillPPVideoJijiuCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	capabilities.Duration = &PPVideoDurationRange{Min: spec.MinSeconds, Max: spec.MaxSeconds}
	capabilities.Resolutions = append([]string(nil), spec.Resolutions...)
	capabilities.Assets = &PPVideoAssetCapabilities{
		FirstLastFrame:  ppVideoCapBool(false),
		ReferenceImages: ppVideoCapInt(spec.MaxImages),
		ReferenceVideos: ppVideoCapInt(spec.MaxVideos),
		ReferenceAudios: ppVideoCapInt(spec.MaxAudios),
	}
	capabilities.GenerateAudio = ppVideoCapBool(false)
	capabilities.Callback = ppVideoCapBool(false)
}

func fillPPVideoMaiTokenCapabilities(capabilities *PPVideoModelCapabilities) {
	capabilities.Duration = &PPVideoDurationRange{Min: maiTokenMinDurationSeconds, Max: maiTokenMaxDurationSeconds}
	capabilities.Resolutions = []string{VideoBillingResolution480P}
	// The ratio is passed through to the upstream unchecked, so it is not listed.
	capabilities.Assets = &PPVideoAssetCapabilities{
		FirstLastFrame:  ppVideoCapBool(true),
		ReferenceImages: ppVideoCapInt(maiTokenMaxReferenceImages),
		ReferenceVideos: ppVideoCapInt(maiTokenMaxReferenceVideos),
		ReferenceAudios: ppVideoCapInt(maiTokenMaxReferenceAudios),
	}
	capabilities.GenerateAudio = ppVideoCapBool(true)
	capabilities.Callback = ppVideoCapBool(false)
}

func fillPPVideoByteDanceCapabilities(capabilities *PPVideoModelCapabilities, model string) {
	upstream := byteDanceUpstreamModel(model)
	if upstream == "" {
		return
	}
	capabilities.Duration = &PPVideoDurationRange{Min: 4, Max: byteDanceMaxDurationSeconds(upstream)}
	for _, resolution := range []string{"480p", "720p", "1080p", "4k"} {
		if byteDanceResolutionForModel(upstream, resolution) != "" {
			capabilities.Resolutions = append(capabilities.Resolutions, resolution)
		}
	}
	// The gateway does not validate the ratio for this platform; this is the set the
	// customer documentation lists.
	capabilities.Ratios = []string{"adaptive", "16:9", "4:3", "1:1", "3:4", "9:16", "21:9"}
	capabilities.Assets = &PPVideoAssetCapabilities{FirstLastFrame: ppVideoCapBool(true)}
	capabilities.GenerateAudio = ppVideoCapBool(true)
	capabilities.Callback = ppVideoCapBool(true)
}

func fillPPVideoWanCapabilities(capabilities *PPVideoModelCapabilities) {
	capabilities.Duration = &PPVideoDurationRange{Min: 2, Max: 30}
	for _, resolution := range []string{"480p", "720p", "1080p"} {
		if wan30Resolution(resolution) != "" {
			capabilities.Resolutions = append(capabilities.Resolutions, resolution)
		}
	}
	for _, ratio := range []string{"adaptive", "16:9", "9:16", "1:1", "4:3", "3:4"} {
		if wan30RatioAllowed(ratio) {
			capabilities.Ratios = append(capabilities.Ratios, ratio)
		}
	}
	capabilities.Assets = &PPVideoAssetCapabilities{
		FirstLastFrame:  ppVideoCapBool(true),
		ReferenceImages: ppVideoCapInt(10),
		ReferenceVideos: ppVideoCapInt(5),
		ReferenceAudios: ppVideoCapInt(5),
	}
	capabilities.GenerateAudio = ppVideoCapBool(true)
	capabilities.Callback = ppVideoCapBool(false)
}

func fillPPVideoPixverseCapabilities(capabilities *PPVideoModelCapabilities) {
	capabilities.Duration = &PPVideoDurationRange{Min: 1, Max: 15}
	for _, resolution := range []string{"360p", "540p", "720p", "1080p"} {
		if pixverseV6Resolution(resolution) != "" {
			capabilities.Resolutions = append(capabilities.Resolutions, resolution)
		}
	}
	// The ratio only applies to text-to-video requests; with any media the model
	// follows the media.
	for _, ratio := range []string{"16:9", "4:3", "1:1", "3:4", "9:16", "2:3", "3:2", "21:9"} {
		body := []byte(fmt.Sprintf(`{"aspect_ratio":%q}`, ratio))
		if _, ok, err := pixverseV6AspectRatioFromRequest(body); err == nil && ok {
			capabilities.Ratios = append(capabilities.Ratios, ratio)
		}
	}
	capabilities.Assets = &PPVideoAssetCapabilities{
		FirstLastFrame:  ppVideoCapBool(true),
		ReferenceImages: ppVideoCapInt(1),
		ReferenceVideos: ppVideoCapInt(1),
		ReferenceAudios: ppVideoCapInt(0),
	}
	capabilities.GenerateAudio = ppVideoCapBool(true)
	capabilities.Callback = ppVideoCapBool(false)
}

func fillPPVideoCompShareCapabilities(capabilities *PPVideoModelCapabilities) {
	capabilities.Duration = &PPVideoDurationRange{Min: 4, Max: 30}
	capabilities.Resolutions = []string{"480p", "768p", "1080p", "2k", "4k"}
	capabilities.Ratios = []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}
	capabilities.Assets = &PPVideoAssetCapabilities{
		FirstLastFrame:  ppVideoCapBool(true),
		ReferenceImages: ppVideoCapInt(9),
		ReferenceVideos: ppVideoCapInt(3),
		ReferenceAudios: ppVideoCapInt(3),
	}
	capabilities.GenerateAudio = ppVideoCapBool(true)
	capabilities.Callback = ppVideoCapBool(true)
}

func fillPPVideoGeneralModelCapabilities(capabilities *PPVideoModelCapabilities, model string) {
	known := false
	for _, candidate := range ppVideoGeneralCapabilityModels {
		if strings.EqualFold(model, candidate) {
			model = candidate
			known = true
			break
		}
	}
	if !known {
		return
	}

	limits := video88APICaps(model)
	duration := &PPVideoDurationRange{Min: limits.minSeconds, Max: limits.maxSeconds}
	if video88APIIsVeo(model) {
		duration.Values = []int{4, 6, 8}
	}
	capabilities.Duration = duration

	if fixed := video88APIFixedTier(model); fixed != "" {
		capabilities.Resolutions = []string{fixed}
	} else {
		for _, tier := range []string{"480p", "720p", "768p", "1080p", "2k", "4k"} {
			if video88APITierAllowed(model, tier) {
				capabilities.Resolutions = append(capabilities.Resolutions, tier)
			}
		}
	}
	for _, ratio := range []string{"adaptive", "1:1", "21:9", "16:9", "9:16", "3:4", "4:3", "3:2", "2:3"} {
		if video88APIValidateAspect(model, ratio) == nil {
			capabilities.Ratios = append(capabilities.Ratios, ratio)
		}
	}

	assets := &PPVideoAssetCapabilities{FirstLastFrame: ppVideoCapBool(limits.frames)}
	if limits.images >= 0 {
		assets.ReferenceImages = ppVideoCapInt(limits.images)
	}
	if limits.videos >= 0 {
		assets.ReferenceVideos = ppVideoCapInt(limits.videos)
	}
	if limits.audios >= 0 {
		assets.ReferenceAudios = ppVideoCapInt(limits.audios)
	}
	capabilities.Assets = assets
	capabilities.GenerateAudio = ppVideoCapBool(limits.audio)
	capabilities.Callback = ppVideoCapBool(false)
}
