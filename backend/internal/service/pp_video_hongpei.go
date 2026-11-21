package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

// hongpei-seedance is a Seedance upstream that speaks an OpenAI-style video protocol:
// POST /v1/videos answers a task object, GET /v1/videos/{id} reports its state, and
// GET /v1/videos/{id}/content delivers the file (every call carries the upstream token as a
// Bearer credential). A submission must carry an Idempotency-Key header (the gateway sends the
// local task id, so a retry of the same task reuses it). The request fields are model, prompt,
// duration (whole seconds), resolution, aspect_ratio and the reference media image_urls /
// video_urls / audio_urls, which must be public HTTPS addresses. The upstream's own custom
// `media` / `content` arrays and its frame fields are not offered by this gateway; a request that
// carries `media` or `content` is refused instead of having its media dropped.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload) for the prompt, the length, the
// resolution and the reference limits; hongpeiSeedanceModel adds what differs per model (the
// ratios, the prompt length, and whether reference audio needs a reference image or video).
// Most models are billed per request whatever their length; vs-sd2.5-nh is billed per second. Either
// way the price (and the billing mode) lives in the channel, not
// in code. The upstream reports no fixed prompt limit for vs-sd2.5-Y and vs-sd2.5-flash (the
// request body limit applies), so none is enforced here for them.
//
// Public names (the operator's request, 2026-11-14, same reason as fengyue): the upstream names
// (vs-sd2.5-L, ...) carry no "seedance", so clients that pick video models by name filtered them out
// of a single-platform group. Clients see Seedance2.5-vs-L / -vs-v / -vs-Y / -vs-flash / -vs-a / -vs-s / -vs-nh / -vs-X. The upstream
// name is the only one ever sent upstream; requests may use either name, and the model list, the task
// record and the response show the public one. Channel prices are read under the public name first,
// then the upstream name.
//
// A request for a model that is not in the table is refused locally (HTTP 400).
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which
// streams the upstream's authenticated /content endpoint. The upstream's own address is never
// stored or shown to a client.

// HongpeiSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). It is the site the upstream's integration document was saved from; an
// account's own base_url takes precedence.
const HongpeiSeedanceDefaultBaseURL = "https://api.hongpei.click"

// hongpeiSeedanceModel is one upstream model: the shared limits plus the per-model rules.
type hongpeiSeedanceModel struct {
	spec   jijiuModelSpec
	ratios []string
	// promptMax is the longest prompt in characters; 0 means the upstream publishes no limit.
	promptMax int
	// audioNeedsVisual: reference audio is only accepted together with a reference image or video.
	audioNeedsVisual bool
	// audioNeedsImage: stricter than audioNeedsVisual, reference audio needs a reference image
	// (a reference video does not count).
	audioNeedsImage bool
}

var (
	hongpeiStandardRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
	hongpeiAdaptiveRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}
	hongpeiYRatios        = []string{"16:9", "9:16", "4:3", "3:4", "1:1"}
)

// hongpeiSeedanceModels lists the models whose limits are known. Add a row, and a channel price,
// to make another upstream model callable.
var hongpeiSeedanceModels = []hongpeiSeedanceModel{
	{
		// 720p only, 4-30 seconds (5 once more than 9 images or any video / audio is sent), 30
		// images, 10 videos and 10 audios but at most 30 items in total; audio needs an image or
		// video.
		spec: jijiuModelSpec{
			Name: "vs-sd2.5-L", PublicName: "Seedance2.5-vs-L", MinSeconds: 4, MinSecondsWithRefs: 5, MaxSeconds: 30,
			Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
			MaxImages: 30, MaxVideos: 10, MaxAudios: 10, MaxReferenceTotal: 30,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiStandardRatios, promptMax: 20000, audioNeedsVisual: true,
	},
	{
		// 720p only, 4-30 seconds, 30 images, 10 videos and 10 audios; also takes adaptive.
		spec: jijiuModelSpec{
			Name: "vs-sd-2.5-v", PublicName: "Seedance2.5-vs-v", MinSeconds: 4, MaxSeconds: 30,
			Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
			MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiAdaptiveRatios, promptMax: 15000,
	},
	{
		// 720p only, 5-30 seconds, 30 images, 10 videos and 10 audios; no 21:9.
		spec: jijiuModelSpec{
			Name: "vs-sd2.5-Y", PublicName: "Seedance2.5-vs-Y", MinSeconds: 5, MaxSeconds: 30,
			Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
			MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiYRatios,
	},
	{
		// 480p or 720p at the same price, 4-30 seconds, 10 images, no reference videos, 5 audios.
		spec: jijiuModelSpec{
			Name: "vs-sd2.5-flash", PublicName: "Seedance2.5-vs-flash", MinSeconds: 4, MaxSeconds: 30,
			Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
			MaxImages: 10, MaxVideos: 0, MaxAudios: 5,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiStandardRatios,
	},
	{
		// vs-sd2.5-a (2026-11-20, from the operator and the upstream's model card): 480p, 720p,
		// 1080p or 4k, 4-30 whole seconds (default 4 s, 480p, 16:9), 30 images, 10 videos and 10
		// audios, prompt at most 15000 characters. Billed per request, one price per resolution
		// (the operator's 11 / 19 / 27 / 35 credits; the prices live in the channel as tiers
		// labelled 480p / 720p / 1080p / 4k). The upstream id carries no "seedance", so clients that
		// keep only Seedance-looking names would drop it from a single-platform group; they see
		// Seedance2.5-vs-a, and only the upstream id is ever sent upstream. The ratio list is the
		// standard one: the model card was cut off after 16:9 / 9:16 / 1:1.
		spec: jijiuModelSpec{
			Name: "vs-sd2.5-a", PublicName: "Seedance2.5-vs-a", MinSeconds: 4, MaxSeconds: 30,
			Resolutions: []string{"480p", "720p", "1080p", "4k"}, DefaultResolution: "480p", ForwardResolution: true,
			MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiStandardRatios, promptMax: 15000,
	},
	{
		// vs-sd2.5-s (2026-11-21, from the upstream's model card): 720p only, 4-30 whole seconds (the
		// upstream defaults to 30; this gateway requires the length), 30 images, 10 videos and 10
		// audios but at most 30 items in total; audio needs an image or video; prompt at most 15000
		// characters. Billed per request at one fixed price whatever the length (the card shows
		// 5.7 yuan; the credit price lives in the channel). Public name Seedance2.5-vs-s (the upstream id
		// has no "seedance").
		spec: jijiuModelSpec{
			Name: "vs-sd2.5-s", PublicName: "Seedance2.5-vs-s", MinSeconds: 4, MaxSeconds: 30,
			Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
			MaxImages: 30, MaxVideos: 10, MaxAudios: 10, MaxReferenceTotal: 30,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiStandardRatios, promptMax: 15000, audioNeedsVisual: true,
	},
	{
		// vs-sd2.5-nh (2026-11-21): 480p, 720p or 1080p (default 720p), 4-30 whole seconds, 30 images,
		// 10 videos and 10 audios, audio needs an image or video, prompt at most 15000 characters.
		// The only hongpei model billed PER SECOND: the upstream multiplies the seconds by the price of
		// the chosen resolution (the card shows 0.22 / 0.25 / 0.5 yuan per second). In the channel use
		// mode "video" with tiers labelled 480p / 720p / 1080p. The card states no combined reference
		// limit, so none is enforced. Public name Seedance2.5-vs-nh.
		spec: jijiuModelSpec{
			Name: "vs-sd2.5-nh", PublicName: "Seedance2.5-vs-nh", MinSeconds: 4, MaxSeconds: 30,
			Resolutions: []string{"480p", "720p", "1080p"}, DefaultResolution: "720p", ForwardResolution: true,
			MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiStandardRatios, promptMax: 15000, audioNeedsVisual: true,
	},
	{
		// vs-sd2.5-X (2026-11-21): 720p only, 4-30 whole seconds (default 4 s, 16:9), 30 images, 10
		// videos and 10 audios, prompt at most 15000 characters, reference audio needs a reference
		// IMAGE (a video is not enough). The card also says the reference videos add up to at most 30
		// seconds and so do the audios; the durations of the media are not known here, the upstream
		// checks them. Billed per request (the card shows 4.9 yuan). Public name Seedance2.5-vs-X.
		spec: jijiuModelSpec{
			Name: "vs-sd2.5-X", PublicName: "Seedance2.5-vs-X", MinSeconds: 4, MaxSeconds: 30,
			Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
			MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
			ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
		},
		ratios: hongpeiStandardRatios, promptMax: 15000, audioNeedsImage: true,
	},
}

// hongpeiModelNames lists the names of every model in the table, in table order. Account model
// sync uses it.
func hongpeiModelNames() []string {
	names := make([]string, 0, len(hongpeiSeedanceModels))
	for _, model := range hongpeiSeedanceModels {
		names = append(names, model.spec.publicName())
	}
	return names
}

// hongpeiModelFor finds a model by name, ignoring case.
func hongpeiModelFor(model string) (hongpeiSeedanceModel, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return hongpeiSeedanceModel{}, false
	}
	for _, candidate := range hongpeiSeedanceModels {
		if strings.EqualFold(candidate.spec.Name, model) || strings.EqualFold(candidate.spec.publicName(), model) {
			return candidate, true
		}
	}
	return hongpeiSeedanceModel{}, false
}

func hongpeiModelSpecFor(model string) (jijiuModelSpec, bool) {
	entry, ok := hongpeiModelFor(model)
	return entry.spec, ok
}

func ppVideoIsHongpeiSeedanceModel(model string) bool {
	_, ok := hongpeiModelFor(model)
	return ok
}

// ppVideoHongpeiPublicModels maps a list of model names to the public names of the models that
// belong to this platform, without duplicates, so a whitelist saved under the upstream name is
// listed under the public one. Names that are not a model of this platform are dropped.
func ppVideoHongpeiPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoHongpeiPublicModel(model)
		if !ok {
			continue
		}
		if _, exists := seen[publicModel]; exists {
			continue
		}
		seen[publicModel] = struct{}{}
		filtered = append(filtered, publicModel)
	}
	return filtered
}

// ppVideoHongpeiPublicModel resolves either accepted name of a model to its public name.
func ppVideoHongpeiPublicModel(model string) (string, bool) {
	entry, ok := hongpeiModelFor(model)
	if !ok {
		return "", false
	}
	return entry.spec.publicName(), true
}

// ppVideoHongpeiModelAliases lists every name a model answers to, public name first.
func ppVideoHongpeiModelAliases(model string) []string {
	entry, ok := hongpeiModelFor(model)
	if !ok {
		return nil
	}
	if entry.spec.publicName() == entry.spec.Name {
		return []string{entry.spec.Name}
	}
	return []string{entry.spec.publicName(), entry.spec.Name}
}

func (m hongpeiSeedanceModel) allowsRatio(ratio string) bool {
	for _, candidate := range m.ratios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeHongpeiSeedancePayload validates a native (v1) request and returns the upstream body.
// The shared normalizer refuses frames, seed, watermark, callbacks, generate_audio and assets, and
// enforces the model's length, resolution and reference limits; the ratio, the prompt length and
// the audio rule are checked here, and the length is written as `duration`.
func normalizeHongpeiSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Seedance request body is required")
	}
	upstreamModel := strings.TrimSpace(public.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(public.Model)
	}
	entry, ok := hongpeiModelFor(upstreamModel)
	if !ok {
		return nil, fmt.Errorf("model %q is not available", public.Model)
	}
	for _, path := range []string{"media", "content"} {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Seedance does not support %s; use images, videos and audios", path)
		}
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !entry.allowsRatio(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s for this model", strings.Join(entry.ratios, ", "))
	}
	references := func(raw []byte) (jijiuReferences, error) {
		refs, err := hongpeiReferencesFromRequest(raw)
		if err != nil {
			return refs, err
		}
		if entry.audioNeedsImage && len(refs.audios) > 0 && len(refs.images) == 0 {
			return refs, fmt.Errorf("Seedance reference audio must be sent with at least one reference image for this model")
		}
		if entry.audioNeedsVisual && len(refs.audios) > 0 && len(refs.images)+len(refs.videos) == 0 {
			return refs, fmt.Errorf("Seedance reference audio must be sent with at least one reference image or video for this model")
		}
		return refs, nil
	}
	generic, err := normalizeSeedanceSpecPayload(body, public, hongpeiModelSpecFor, references)
	if err != nil {
		return nil, err
	}
	if entry.promptMax > 0 {
		if prompt, _ := generic["prompt"].(string); utf8.RuneCountInString(prompt) > entry.promptMax {
			return nil, fmt.Errorf("Seedance prompt must be at most %d characters for this model", entry.promptMax)
		}
	}
	result := map[string]any{
		"model":    generic["model"],
		"prompt":   generic["prompt"],
		"duration": generic["seconds"],
	}
	for _, key := range []string{"aspect_ratio", "resolution", "image_urls", "video_urls", "audio_urls"} {
		if value, ok := generic[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

// hongpeiReferencesFromRequest reads the media of a native (v1) request. It accepts the upstream's
// own names (image_urls / video_urls / audio_urls) and the aliases the other Seedance models of
// this gateway document (images / videos / audios, reference_*, camelCase variants, single
// image). Two different aliases of the same kind in one request are refused so an item is never
// counted twice.
func hongpeiReferencesFromRequest(body []byte) (jijiuReferences, error) {
	return xsnReferencesFromRequest(body)
}

// finalizeHongpeiSeedanceResponse reads the task the same way as the xsn platform: the status
// comes from the explicit status field only, a failed task carries its reason in `error`, and
// billing uses the requested seconds. A task id of the form task_bulk_... is polled like any other.
func finalizeHongpeiSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// fillPPVideoHongpeiCapabilities reports the limits of one model, read from the table the
// normalizer enforces.
func fillPPVideoHongpeiCapabilities(capabilities *PPVideoModelCapabilities, model hongpeiSeedanceModel) {
	fillPPVideoJijiuCapabilities(capabilities, model.spec)
	capabilities.Ratios = append([]string(nil), model.ratios...)
}
