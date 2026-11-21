package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// vshi-seedance is a Seedance upstream that speaks the "unified video" protocol of VSHI API:
// POST /v1/unified/videos answers a task object with an `id`, GET /v1/unified/videos/{id} reports
// queued / in_progress / completed / failed, and GET /v1/unified/videos/{id}/content delivers the
// file (every call carries the upstream token as a Bearer credential). A submission carries an
// Idempotency-Key header (the gateway sends the local task id, so a retry of the same task reuses
// it; the upstream accepts 8-128 letters, digits and _.:-). The request fields are model, prompt,
// seconds (whole seconds), resolution, aspect_ratio and the reference media image_urls /
// video_urls / audio_urls, which must be public HTTPS addresses. The upstream also takes content
// arrays, frame roles, generation_mode and provider_options; this gateway offers none of them, and
// a request that carries `media` or `content` is refused instead of having its media dropped.
//
// Which models a token can call is decided by the token's group on the upstream (the key's group
// picks the line and the price there), so each upstream group gets its own account and the
// account's model whitelist holds only the models that token can use, the same way as the jijiu
// platform.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload) for the prompt, the length, the
// resolution and the reference limits. The upstream publishes the limits per version, not per
// model, so every Seedance 2.0 row uses 4-15 seconds with 9 images / 3 videos / 3 audios and every
// Seedance 2.5 row 4-30 seconds with 30 images / 10 videos / 10 audios (the operator's figures,
// 2026-11-14); the resolutions come from the upstream's list of registered models. The upstream
// refuses a combination it does not support with an explicit error, so a row that is too wide
// fails at the upstream (the hold is released) rather than being billed wrongly. Billing (per
// request or per second) lives in the channel, not in code.
//
// Only four upstream models are offered (the operator's decision, 2026-11-16): NSFW-fast,
// NSFW-mini, NSFW-pro and NSFW-2.5. Every other model the upstream lists (the official
// doubao-* / huoshan-* lines, the other third-party lines, sd2.5-s, sd2.0-110, Wan 3.0,
// MiniMax-H3, HappyHorse) is not in the table, so a request for it is refused locally (HTTP 400).
//
// Public names (same reason as fengyue and hongpei): the upstream names carry no "seedance", so
// clients that pick video models by name filtered them out of a single-platform group. Clients see
// Seedance2.0-NSFW-fast / -mini / -pro and Seedance2.5-NSFW, with no vendor name in them. The
// upstream name is the only one ever sent upstream; requests may use either name, and the model
// list, the task record and the response show the public one. Channel prices are read under the
// public name first, then the upstream name.
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which streams
// the upstream's authenticated /content endpoint. The upstream's own address is never stored or
// shown to a client.

// VshiSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). It is the site the upstream's integration document was saved from; an account's
// own base_url takes precedence.
const VshiSeedanceDefaultBaseURL = "https://api.vshiai.com"

// vshiUnifiedVideosPath is the upstream's unified video collection: POST creates, GET {path}/{id}
// reads the task and GET {path}/{id}/content delivers the file. It differs from the /v1/videos
// that the other OpenAI-style Seedance platforms use.
const vshiUnifiedVideosPath = "/v1/unified/videos"

// Seedance 2.0 and 2.5 limits as the operator gave them for the whole upstream.
const (
	vshiSeedance20MinSeconds = 4
	vshiSeedance20MaxSeconds = 15
	vshiSeedance25MinSeconds = 4
	vshiSeedance25MaxSeconds = 30
)

// vshiAspectRatios are the ratios the gateway lets through (the upstream's own page says "such as"
// and lists no complete set, and refuses what a model does not support). 16:9 when omitted.
var vshiAspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}

func vshiSeedance20(name, publicName string, resolutions ...string) jijiuModelSpec {
	return jijiuModelSpec{
		Name: name, PublicName: publicName, MinSeconds: vshiSeedance20MinSeconds, MaxSeconds: vshiSeedance20MaxSeconds,
		Resolutions: resolutions, DefaultResolution: vshiDefaultResolution(resolutions), ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
	}
}

func vshiSeedance25(name, publicName string, resolutions ...string) jijiuModelSpec {
	return jijiuModelSpec{
		Name: name, PublicName: publicName, MinSeconds: vshiSeedance25MinSeconds, MaxSeconds: vshiSeedance25MaxSeconds,
		Resolutions: resolutions, DefaultResolution: vshiDefaultResolution(resolutions), ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
	}
}

// vshiDefaultResolution is 720p when the model offers it, else the first (lowest) one.
func vshiDefaultResolution(resolutions []string) string {
	for _, resolution := range resolutions {
		if resolution == "720p" {
			return resolution
		}
	}
	return resolutions[0]
}

// vshiSeedanceModels lists the models the gateway offers. Add a row, and a channel price, to make
// another upstream model callable.
var vshiSeedanceModels = []jijiuModelSpec{
	// Seedance 2.0.
	vshiSeedance20("NSFW-pro", "Seedance2.0-NSFW-pro", "480p", "720p", "1080p", "4k"),
	vshiSeedance20("NSFW-fast", "Seedance2.0-NSFW-fast", "480p", "720p"),
	vshiSeedance20("NSFW-mini", "Seedance2.0-NSFW-mini", "480p", "720p"),
	// Seedance 2.5.
	vshiSeedance25("NSFW-2.5", "Seedance2.5-NSFW", "480p", "720p", "1080p"),
}

// vshiModelNames lists the public names of every model in the table, in table order. Account model
// sync uses it, so the whitelist an administrator saves holds the names clients see.
func vshiModelNames() []string {
	names := make([]string, 0, len(vshiSeedanceModels))
	for _, spec := range vshiSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// vshiModelSpecFor finds a model by either its upstream name or its public name, ignoring case.
func vshiModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range vshiSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsVshiSeedanceModel(model string) bool {
	_, ok := vshiModelSpecFor(model)
	return ok
}

// ppVideoVshiPublicModel resolves either accepted name of a model to its public name.
func ppVideoVshiPublicModel(model string) (string, bool) {
	spec, ok := vshiModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoVshiPublicModels maps a list of model names to the public names of the models that belong
// to this platform, without duplicates, so a whitelist saved under the upstream name is listed
// under the public one. Names that are not a model of this platform are dropped.
func ppVideoVshiPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoVshiPublicModel(model)
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

// ppVideoVshiModelAliases lists every name a model answers to, public name first.
func ppVideoVshiModelAliases(model string) []string {
	spec, ok := vshiModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func vshiAspectRatioAllowed(ratio string) bool {
	for _, candidate := range vshiAspectRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeVshiSeedancePayload validates a native (v1) request and returns the upstream body. The
// shared normalizer refuses frames, seed, watermark, callbacks, generate_audio and assets, and
// enforces the model's length, resolution and reference limits and writes the upstream's own
// field names (seconds, aspect_ratio, image_urls, video_urls, audio_urls); the ratio is checked
// here against the accepted set.
func normalizeVshiSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	for _, path := range []string{"media", "content", "provider_options", "generation_mode"} {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Seedance does not support %s; use images, videos and audios", path)
		}
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !vshiAspectRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(vshiAspectRatios, ", "))
	}
	return normalizeSeedanceSpecPayload(body, public, vshiModelSpecFor, vshiReferencesFromRequest)
}

// vshiReferencesFromRequest reads the media of a native (v1) request. It accepts the upstream's
// own names (image_urls / video_urls / audio_urls, and its reference_images / reference_videos /
// reference_audios aliases) and the aliases the other Seedance models of this gateway document
// (images / videos / audios, camelCase variants, single image). Two different aliases of the same
// kind in one request are refused so an item is never counted twice.
func vshiReferencesFromRequest(body []byte) (jijiuReferences, error) {
	return xsnReferencesFromRequest(body)
}

// finalizeVshiSeedanceResponse reads the task the same way as the xsn platform: the status comes
// from the explicit status field only (queued / in_progress / completed / failed), a failed task
// carries its reason in `error` (an object with a message, or a string) and may come back with HTTP
// 200, and billing uses the requested seconds. final_cost and usage in the response are the
// upstream's own figures and take no part in billing.
func finalizeVshiSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// ppVideoVshiRemoveUpstreamFields drops what the upstream's task object says about itself: the
// download addresses in `result` (they name the upstream host) and the cost figures
// (`final_cost`, `usage`). The gateway adds its own video_url and usage afterwards.
func ppVideoVshiRemoveUpstreamFields(payload map[string]any) {
	if payload == nil {
		return
	}
	delete(payload, "final_cost")
	delete(payload, "usage")
	if result, ok := payload["result"].(map[string]any); ok {
		for _, key := range []string{"preview_url", "download_url", "url"} {
			delete(result, key)
		}
		if len(result) == 0 {
			delete(payload, "result")
		}
	}
}

// fillPPVideoVshiCapabilities reports the limits of one model, read from the table the normalizer
// enforces.
func fillPPVideoVshiCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), vshiAspectRatios...)
}
