package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// huahuo-seedance is a Seedance upstream (HUAHUO API) that speaks the OpenAI-style video protocol:
// POST /v1/videos answers a task object with an `id`, GET /v1/videos/{id} reports queued / in_progress /
// processing / submitting / reserved / review_required / completed / failed, and
// GET /v1/videos/{id}/content delivers the file (every call carries the upstream token as a Bearer credential).
// A submission must carry an Idempotency-Key header that is unique per task (the gateway sends
// the local task id, so a retry of the same task reuses it). The request fields are model,
// prompt, seconds, size (the aspect ratio) and resolution, all strings, and the reference media
// as input_reference / video_reference / audio_references, which must be public HTTPS addresses.
// The upstream's own aliases (input_images / input_videos / input_audios), its first and last
// frame fields (start_frame / end_frame) and its direct upload are not offered by this gateway;
// a request that carries one of them is refused instead of having its media dropped.
//
// Which models a key can call is decided on the upstream by the key's own model scope, so each
// upstream key gets its own account and the account's model whitelist holds the models that key
// may use. Billing (per second or per request) lives in the channel, not in code.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload) for the prompt, the length, the
// resolution and the reference limits. The upstream publishes the limits of a model only through
// its own model catalogue (GET /v1/market), which needs a key, so the figures in the
// table below are assumptions (see huahuoSeedanceModels) and must be checked against that
// catalogue. The upstream refuses a combination it does not support with an explicit error and
// without charging, so a row that is too wide fails at the upstream (the hold is released)
// rather than being billed wrongly.
//
// Public names (same reason as fengyue, hongpei and vshi): the upstream names carry a vendor path
// (inmind/video/...), which must not reach clients and which no client that picks video models by
// name would recognise as Seedance. Clients see Seedance2.5-mancan-anmiao / -mancan-anci /
// -mancan-flash (and Seedance2.5-shuliang1, 2026-11-19). The upstream name is the only one ever sent upstream; requests may use either
// name, and the model list, the task record and the response show the public one. Channel prices
// are read under the public name first, then the upstream name.
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which streams
// the upstream's authenticated /content endpoint. The upstream's own address (the `video_url` it
// adds to some responses) is never stored or shown to a client. The upstream keeps an original
// for three days and answers 410 afterwards; a refreshed link does not extend that.

// HuahuoSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). The upstream also offers https://cn.huahuo.ccwu.cc for networks that reach the
// default one slowly; both share accounts, keys and task records, so an account whose network
// needs it sets that address as its own base_url, which takes precedence.
const HuahuoSeedanceDefaultBaseURL = "https://huahuo.ccwu.cc"

// huahuoSeedanceRatios are the ratios the gateway lets through (the upstream's document gives
// no list, only "16:9" in its examples, and refuses what a model does not support).
var huahuoSeedanceRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}

// huahuoRefusedFields are the upstream's own spellings that this gateway does not take. They are
// refused with a message instead of being ignored, because ignoring one would drop the media.
var huahuoRefusedFields = []string{
	"media", "content",
	"input_images", "input_videos", "input_audios",
	"video_reference", "audio_references",
	"start_frame", "end_frame",
}

// huahuoSeedance25 describes a Seedance 2.5 model of the upstream. The upstream document names
// the fields of one request (input_reference / video_reference / audio_references / size) but
// publishes the limits of a model only in its catalogue; 4-30 seconds and 30 images / 10 videos /
// 10 audios are the limits of the other Seedance 2.5 upstreams of this gateway (the widest of
// them, so that a limit that is too wide fails at the upstream and one that is too narrow never
// blocks a valid request).
func huahuoSeedance25(name, publicName string, resolutions ...string) jijiuModelSpec {
	return jijiuModelSpec{
		Name: name, PublicName: publicName, MinSeconds: 4, MaxSeconds: 30,
		Resolutions: resolutions, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
		ImageField: "input_reference", VideoField: "video_reference", AudioField: "audio_references", RatioField: "size",
	}
}

// huahuoSeedanceModels lists the models the gateway offers. Add a row, and a channel price, to
// make another upstream model callable.
//
// ASSUMED, not read from the upstream: the resolutions (720p for the two full-reference models,
// 480p or 720p for flash, as the flash lines of the miaowu and hongpei platforms), the 4-30
// second length and the reference counts of huahuoSeedance25. Check them against the upstream's
// catalogue (GET /v1/market, capabilities.durations / resolutions / references) and
// correct the rows; the hub line names (seedanceHubLines) carry the resolutions too.
var huahuoSeedanceModels = []jijiuModelSpec{
	// 满参 (images and audios, NO reference videos since the operator's final table of 2026-11-19;
	// they were 10 before), stable, billed per second.
	huahuoSeedance25NoVideo("inmind/video/seedance2-5-mancananmiao", "Seedance2.5-mancan-anmiao", "720p"),
	// 满参 (images and audios, no reference videos, as above), stable, billed per request.
	huahuoSeedance25NoVideo("inmind/video/seedance2-5-mancananci", "Seedance2.5-mancan-anci", "720p"),
	// flash 满参, stable, billed per second.
	huahuoSeedance25("inmind/video/seedance2-5-flash", "Seedance2.5-mancan-flash", "480p", "720p"),
	// video-2.5shuliang1 (added 2026-11-19 by the operator), billed per request (19 credits): 720p
	// only, 5-30 seconds, 30 images / 10 videos / 10 audios. The operator's table calls it
	// video-2.5shuliang1, the supplier's own page inmind/video/video-2.5shuliang1 (same vendor path
	// as the other rows), which is what the upstream receives. The name says no "seedance", so
	// clients that pick video models by name would skip it: they see the public name
	// Seedance2.5-shuliang1 (mine). The 5-second minimum is the operator's; the other rows take 4.
	huahuoSeedance25Min("inmind/video/video-2.5shuliang1", "Seedance2.5-shuliang1", 5, "720p"),
}

// huahuoSeedance25NoVideo is huahuoSeedance25 for a model that takes no reference videos.
func huahuoSeedance25NoVideo(name, publicName string, resolutions ...string) jijiuModelSpec {
	spec := huahuoSeedance25(name, publicName, resolutions...)
	spec.MaxVideos = 0
	return spec
}

// huahuoSeedance25Min is huahuoSeedance25 with a shortest length other than 4 seconds.
func huahuoSeedance25Min(name, publicName string, minSeconds int, resolutions ...string) jijiuModelSpec {
	spec := huahuoSeedance25(name, publicName, resolutions...)
	spec.MinSeconds = minSeconds
	return spec
}

// huahuoVendorPath is the prefix every upstream name of this platform carries. The operator's own
// table lists the models without it (seedance2-5-mancananci, video-2.5shuliang1), so a name without
// the prefix is accepted too; only the full upstream name is ever sent upstream.
const huahuoVendorPath = "inmind/video/"

// huahuoShortName is the upstream name without the vendor path.
func huahuoShortName(name string) string {
	return strings.TrimPrefix(name, huahuoVendorPath)
}

// huahuoModelNames lists the public names of every model in the table, in table order. Account
// model sync uses it, so the whitelist an administrator saves holds the names clients see.
func huahuoModelNames() []string {
	names := make([]string, 0, len(huahuoSeedanceModels))
	for _, spec := range huahuoSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// huahuoModelSpecFor finds a model by its upstream name, its upstream name without the vendor
// path, or its public name, ignoring case.
func huahuoModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range huahuoSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(huahuoShortName(spec.Name), model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsHuahuoSeedanceModel(model string) bool {
	_, ok := huahuoModelSpecFor(model)
	return ok
}

// ppVideoHuahuoPublicModel resolves either accepted name of a model to its public name.
func ppVideoHuahuoPublicModel(model string) (string, bool) {
	spec, ok := huahuoModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoHuahuoPublicModels maps a list of model names to the public names of the models that
// belong to this platform, without duplicates, so a whitelist saved under the upstream name is
// listed under the public one. Names that are not a model of this platform are dropped.
func ppVideoHuahuoPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoHuahuoPublicModel(model)
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

// ppVideoHuahuoModelAliases lists every name a model answers to, public name first.
func ppVideoHuahuoModelAliases(model string) []string {
	spec, ok := huahuoModelSpecFor(model)
	if !ok {
		return nil
	}
	aliases := []string{spec.publicName()}
	if spec.Name != spec.publicName() {
		aliases = append(aliases, spec.Name)
	}
	if short := huahuoShortName(spec.Name); short != spec.Name && !strings.EqualFold(short, spec.publicName()) {
		// The name the operator's own table uses, without the vendor path.
		aliases = append(aliases, short)
	}
	return aliases
}

func huahuoRatioAllowed(ratio string) bool {
	for _, candidate := range huahuoSeedanceRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeHuahuoSeedancePayload validates a native (v1) request and returns the upstream body.
// The shared normalizer refuses frames, seed, watermark, callbacks, generate_audio and assets,
// enforces the model's length, resolution and reference limits and writes the upstream's field
// names (model, prompt, seconds, size, resolution, input_reference, video_reference,
// audio_references); the ratio is checked here against the accepted set, and the length is
// written as a string, which is what the upstream's document asks for.
func normalizeHuahuoSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	for _, path := range huahuoRefusedFields {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Seedance does not support %s; use images, videos and audios", path)
		}
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !huahuoRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(huahuoSeedanceRatios, ", "))
	}
	payload, err := normalizeSeedanceSpecPayload(body, public, huahuoModelSpecFor, huahuoReferencesFromRequest)
	if err != nil {
		return nil, err
	}
	if seconds, ok := payload["seconds"].(int); ok {
		payload["seconds"] = strconv.Itoa(seconds)
	}
	return payload, nil
}

// huahuoReferencesFromRequest reads the media of a native (v1) request. It accepts the aliases
// the other Seedance models of this gateway document (images / videos / audios, image_urls /
// video_urls / audio_urls, reference_*, camelCase variants, single image). Two different aliases
// of the same kind in one request are refused so an item is never counted twice.
func huahuoReferencesFromRequest(body []byte) (jijiuReferences, error) {
	return xsnReferencesFromRequest(body)
}

// finalizeHuahuoSeedanceResponse reads the task the same way as the xsn platform: the status
// comes from the explicit status field only (queued / in_progress / processing / completed /
// failed; submitting, reserved and review_required are still being confirmed and count as
// processing), a failed task carries its reason in `error`, and billing uses the requested
// seconds. The upstream's own credit figures and its token usage take no part in billing.
func finalizeHuahuoSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// ppVideoHuahuoRemoveUpstreamFields drops what the upstream's task object says about its own
// accounting: the credits it charged this gateway (credit_state / credit_amount), which are the
// supplier's price and not ours, and its cost and usage figures. The gateway adds its own
// video_url and usage afterwards.
func ppVideoHuahuoRemoveUpstreamFields(payload map[string]any) {
	if payload == nil {
		return
	}
	for _, key := range []string{"credit_state", "credit_amount", "final_cost", "cost", "usage"} {
		delete(payload, key)
	}
}

// fillPPVideoHuahuoCapabilities reports the limits of one model, read from the table the
// normalizer enforces.
func fillPPVideoHuahuoCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), huahuoSeedanceRatios...)
}
