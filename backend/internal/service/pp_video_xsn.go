package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// xsn-seedance is a Seedance upstream that speaks the OpenAI-style video protocol:
// POST /v1/videos answers a task object with an `id`, GET /v1/videos/{id} reports
// queued / in_progress / completed / failed, and GET /v1/videos/{id}/content delivers the file
// (every call carries the upstream token as a Bearer credential). Its request fields are model,
// prompt, seconds, ratio, resolution and the reference media image_urls / video_urls /
// audio_urls, which must be public HTTP(S) addresses. The normalizer builds a clean payload and
// only sends those.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload) for the prompt, the length, the
// resolution and the reference limits; only the field names differ (the same ones the miaowu
// platform uses). Both models are fixed at 720p and take no reference videos. seedance-2.5-特价满血
// is billed per output second; its upstream charges twice the price once a reference video is
// sent while this gateway has one channel price per model, so reference videos are refused
// (MaxVideos 0) until a surcharge is agreed; allowing them later is compatible.
// seedance-2.5-全天打 is billed per video whatever its length (the upstream price is $10 per video;
// the operator's channel price is 19 credits per request, set in the channel, not in code).
// A request for a model that is not in the table is refused locally (HTTP 400).
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which
// streams the upstream's authenticated /content endpoint. The upstream's own address is never
// stored or shown to a client.

// XSNSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it).
const XSNSeedanceDefaultBaseURL = "https://xsn686.cn"

// The two 即梦 models are named by the upstream without "seedance", so clients that pick video
// models by name (downstream tools look for "Seedance") did not list them in a single-platform
// group. They are shown to clients under the public names below (the same fix as the fengyue
// platform); the upstream name is the only one ever sent upstream, and requests may use either.
const (
	XSNJimengUpstreamModel      = "即梦-Sd-2.5"
	XSNJimengPublicModel        = "Seedance-即梦-Sd-2.5"
	XSNJimengFlashUpstreamModel = "即梦-Sd-2.5-flash"
	XSNJimengFlashPublicModel   = "Seedance-即梦-Sd-2.5-flash"
)

// xsnSeedanceModels lists the models whose limits are known. Add a row, and a channel price, to
// make another upstream model callable.
var xsnSeedanceModels = []jijiuModelSpec{
	{
		// 720p only, 4-30 seconds (the upstream falls back to 4 seconds when the length is
		// missing; this gateway requires it), 30 images and 10 audios. No reference videos.
		Name: "seedance-2.5-特价满血", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 720p only, 5-30 seconds, 30 images and 10 audios. No reference videos. Billed per video
		// (per request), independent of the length.
		Name: "seedance-2.5-全天打", MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 即梦-Sd-2.5 (official forward, faces supported), 2026-11-16: 480p or 720p (720p when
		// omitted), billed per second (channel: 480p 1 credit, 720p 1.2 credits per second), 30
		// images, 10 videos and 10 audios. The length range was not given; 4-30 s is assumed, as
		// for the other XSN 2.5 models.
		Name: XSNJimengUpstreamModel, PublicName: XSNJimengPublicModel, MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 即梦-Sd-2.5-flash (official forward, faces supported): 720p only, billed per request
		// (channel: 17 credits per request), 10 images, 5 videos and 5 audios. Length range
		// assumed 4-30 s as above.
		Name: XSNJimengFlashUpstreamModel, PublicName: XSNJimengFlashPublicModel, MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 10, MaxVideos: 5, MaxAudios: 5,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
}

// xsnAspectRatios are the ratios the upstream documents (16:9 when omitted).
var xsnAspectRatios = []string{"1:1", "16:9", "4:3", "3:4", "21:9", "9:16"}

// xsnModelNames lists the public names of every model in the table, in table order. Account
// model sync uses it, so the whitelist an administrator saves holds the names clients see.
func xsnModelNames() []string {
	names := make([]string, 0, len(xsnSeedanceModels))
	for _, spec := range xsnSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// ppVideoXSNPublicModels maps a list of model names to the public names of the models that
// belong to this platform, without duplicates, so a whitelist saved under an upstream name is
// listed under the public one. Names that are not a model of this platform are dropped.
func ppVideoXSNPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoXSNPublicModel(model)
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

// xsnModelSpecFor finds a model by name, ignoring case.
func xsnModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range xsnSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsXSNSeedanceModel(model string) bool {
	_, ok := xsnModelSpecFor(model)
	return ok
}

// ppVideoXSNPublicModel resolves a model name to its public name (the upstream name: this
// platform has no separate public names).
func ppVideoXSNPublicModel(model string) (string, bool) {
	spec, ok := xsnModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoXSNModelAliases lists every name a model answers to, public name first.
func ppVideoXSNModelAliases(model string) []string {
	spec, ok := xsnModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func xsnAspectRatioAllowed(ratio string) bool {
	for _, candidate := range xsnAspectRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeXSNSeedancePayload validates a native (v1) request and returns the upstream body.
// The shared normalizer refuses frames, seed, watermark, callbacks, generate_audio and assets, and
// enforces the model's length, resolution and reference limits; the ratio is checked here
// against the documented set.
func normalizeXSNSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !xsnAspectRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(xsnAspectRatios, ", "))
	}
	return normalizeSeedanceSpecPayload(body, public, xsnModelSpecFor, xsnReferencesFromRequest)
}

// xsnReferencesFromRequest reads the media of a native (v1) request. It accepts the upstream's
// own names (image_urls / video_urls / audio_urls) and the aliases the other Seedance models of
// this gateway document (images / videos / audios, reference_*, camelCase variants, single
// image). Two different aliases of the same kind in one request are refused so an item is never
// counted twice. Reference videos are collected under every alias so that the model limit
// (none) refuses them whichever name they were sent under.
func xsnReferencesFromRequest(body []byte) (jijiuReferences, error) {
	var result jijiuReferences
	var err error
	if result.images, err = jijiuURLList(body, "image_url",
		[]string{"image_urls", "images", "reference_images", "referenceImages", "reference_image_urls"},
		[]string{"image", "image_url"}); err != nil {
		return result, err
	}
	if result.videos, err = jijiuURLList(body, "video_url",
		[]string{"video_urls", "videos", "reference_videos", "referenceVideos", "video_refs"},
		nil); err != nil {
		return result, err
	}
	if result.audios, err = jijiuURLList(body, "audio_url",
		[]string{"audio_urls", "audios", "reference_audios", "referenceAudios", "reference_audio_urls", "audio_refs"},
		nil); err != nil {
		return result, err
	}
	return result, nil
}

// finalizeXSNSeedanceResponse takes the status from the explicit status field only (the upstream
// reports queued / in_progress / completed / failed) and bills the requested seconds, the same
// way as the jijiu platform. A failed task carries error.message; a rejected request may carry a
// plain string in `error`, which is read as well.
func finalizeXSNSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	result = finalizeJijiuSeedanceResponse(result, body)
	if result.ErrorMessage == "" {
		if value := gjson.GetBytes(body, "error"); value.Type == gjson.String {
			result.ErrorMessage = strings.TrimSpace(value.String())
		}
	}
	return result
}

// fillPPVideoXSNCapabilities reports the limits of one model, read from the table the normalizer
// enforces.
func fillPPVideoXSNCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), xsnAspectRatios...)
}
