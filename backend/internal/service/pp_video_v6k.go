package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// v6k-seedance is the Seedance 2.0 line of the supplier site V6K (www.v6k.top, a new-api site; the
// operator's integration document "统一接口 / 视频", saved 2026-11-21). It speaks the OpenAI-style
// video protocol with a JSON body: POST /v1/videos answers a task object with a string `id`,
// GET /v1/videos/{id} reports queued / in_progress / completed / failed (HTTP 200 even for a failed
// task; the reason is in `error.message`), and GET /v1/videos/{id}/content delivers the file
// (Range is supported; 410 once the result is no longer available). Every call carries the upstream
// key as a Bearer credential.
//
// Request fields (the document): model (the exact id copied from the site's model list), prompt
// (required, non-empty), seconds (whole seconds; a numeric string is accepted; -1 is only for the
// edit mode of the models that offer it), ratio (`aspect_ratio` is the same field, and both must
// agree when both are given), resolution (lower case, only when the model supports it) and the
// reference media as public URL arrays: images / videos / audios (the document's own names, which
// the shared normalizer writes). The document also lists `size`, `omni_reference_task_type`
// (reference / edit / extend) and the aliases *_refs; none of them is sent. The three models below
// take no reference video, so edit and extend (which need one) are not offered: a request that
// names one of them, or sends `size`, is refused with a message instead of being dropped.
//
// Three models are offered (the operator's decision, 2026-11-21), described with the same table
// type as the jijiu platform (jijiuModelSpec) and run through the same normalizer
// (normalizeSeedanceSpecPayload) for the prompt, the length, the resolution and the reference
// limits:
//
//   - 官方按条SD2.0 ("官方正版"): 480p / 720p / 1080p / 4k (720p when left out), 4-15 s, 9 images / 0
//     videos / 3 audios, billed per request (channel: 11 credits);
//   - 官方sd2.0-fast ("官方原生不卡脸"): 480p / 720p (720p when left out), 4-15 s, 9 images / 0 videos
//     / 3 audios, billed per request (channel: 5 credits);
//   - SD2.0-mini ("官方原生不卡脸"): 480p / 720p (720p when left out), 4-15 s, 9 images / 0 videos / 3
//     audios, billed per request (channel: 4 credits).
//
// The operator's text gives the length twice, "4-15秒" in the parameter block and "5-15秒" in the
// one-line summary; the parameter block is what the site's model list shows, so 4-15 is used (a
// request the upstream refuses fails there and the hold is released). The default ratio 16:9 and
// the default length 5 s are the upstream's own defaults; this gateway still requires the length
// like every other Seedance platform. The prices live in the channel, not in code (the figures in
// the operator's text are the supplier's credits per request, set as the channel price).
//
// Names: the upstream ids contain Chinese text ("官方按条SD2.0", "官方sd2.0-fast") or differ from
// the others only by case ("SD2.0-mini"), and clients that pick video models by name look for
// "Seedance". Clients see Seedance2.0-V6K / -V6K-fast / -V6K-mini. The upstream id is the only one
// ever sent upstream; requests may use either name, and the model list, the task record and the
// response show the public one. Channel prices are read under the public name first, then the
// upstream id.
//
// ASSUMED, not read from the supplier: that the upstream ids are exactly the names the operator
// wrote (the document has no fixed model list, it is copied from the site's model square), that the
// 4K tier is spelled "4k" in the resolution field, and that no model takes first / last frames
// (the document lists none). Which models a key can call is decided by the key's group on the
// upstream, so an account's model whitelist holds only the models that key can use (the same as
// the jijiu, vshi, zexi and xm platforms).
//
// The document says a submission whose answer is lost must be reconciled, never re-sent, so no
// Idempotency-Key is sent. The /content endpoint answers the file itself, so it is streamed as is.

// V6KSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). An account's own base_url takes precedence.
const V6KSeedanceDefaultBaseURL = "https://www.v6k.top"

// v6kSeedanceRatios are the ratios the operator lists for the three models (16:9 when omitted).
var v6kSeedanceRatios = []string{"16:9", "9:16", "1:1", "3:4", "4:3", "21:9"}

// v6kSeedanceModels lists the models the gateway offers. Add a row, and a channel price, to make
// another upstream model callable.
var v6kSeedanceModels = []jijiuModelSpec{
	{
		// 官方正版, per request. 480p / 720p / 1080p / 4k.
		Name: "官方按条SD2.0", PublicName: "Seedance2.0-V6K", MinSeconds: 4, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p", "1080p", "4k"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 3, RatioField: "ratio",
	},
	{
		// 官方原生不卡脸, per request. 480p / 720p.
		Name: "官方sd2.0-fast", PublicName: "Seedance2.0-V6K-fast", MinSeconds: 4, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 3, RatioField: "ratio",
	},
	{
		// 官方原生不卡脸, per request. 480p / 720p.
		Name: "SD2.0-mini", PublicName: "Seedance2.0-V6K-mini", MinSeconds: 4, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 3, RatioField: "ratio",
	},
}

// v6kModelNames lists the public names of every model in the table, in table order. Account model
// sync uses it, so the whitelist an administrator saves holds the names clients see.
func v6kModelNames() []string {
	names := make([]string, 0, len(v6kSeedanceModels))
	for _, spec := range v6kSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// v6kModelSpecFor finds a model by either its upstream name or its public name, ignoring case
// (the upstream name sent is always the table's exact spelling).
func v6kModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range v6kSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsV6KSeedanceModel(model string) bool {
	_, ok := v6kModelSpecFor(model)
	return ok
}

// ppVideoV6KPublicModel resolves either accepted name of a model to its public name.
func ppVideoV6KPublicModel(model string) (string, bool) {
	spec, ok := v6kModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoV6KPublicModels maps a list of model names to the public names of the models that belong
// to this platform, without duplicates, so a whitelist saved under the upstream name is listed
// under the public one. Names that are not a model of this platform are dropped.
func ppVideoV6KPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoV6KPublicModel(model)
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

// ppVideoV6KModelAliases lists every name a model answers to, public name first.
func ppVideoV6KModelAliases(model string) []string {
	spec, ok := v6kModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func v6kRatioAllowed(ratio string) bool {
	for _, candidate := range v6kSeedanceRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeV6KSeedancePayload validates a native (v1) request and returns the upstream body.
//
// The shared normalizer enforces the model's length, resolution and reference limits and the
// prompt, and refuses frames, seed, watermark, callbacks, generate_audio and assets. On top of it
// this function checks what is specific to this upstream: the ratio must be one of the documented
// set (ratio and aspect_ratio must agree), `size` is not offered, and `omni_reference_task_type`
// may only be "reference" (edit and extend need a reference video, which these models do not take).
// The result keeps the upstream's own names: seconds, ratio, resolution, images, videos, audios.
func normalizeV6KSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Seedance request body is required")
	}
	if value := gjson.GetBytes(body, "size"); value.Exists() && value.Type != gjson.Null {
		return nil, fmt.Errorf("Seedance does not support size; use ratio and resolution")
	}
	if value := gjson.GetBytes(body, "omni_reference_task_type"); value.Exists() && value.Type != gjson.Null {
		if value.Type != gjson.String {
			return nil, fmt.Errorf("Seedance omni_reference_task_type must be a string")
		}
		if kind := strings.ToLower(strings.TrimSpace(value.String())); kind != "" && kind != "reference" {
			return nil, fmt.Errorf("Seedance does not support omni_reference_task_type %q for this model; only reference is offered", strings.TrimSpace(value.String()))
		}
	}

	ratio := ""
	for _, path := range []string{"ratio", "aspect_ratio"} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.String {
			return nil, fmt.Errorf("Seedance %s must be a string", path)
		}
		current := strings.TrimSpace(value.String())
		if current == "" {
			continue
		}
		if ratio != "" && current != ratio {
			return nil, fmt.Errorf("Seedance ratio and aspect_ratio must have the same value when both are given")
		}
		ratio = current
	}
	if ratio != "" && !v6kRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(v6kSeedanceRatios, ", "))
	}

	return normalizeSeedanceSpecPayload(body, public, v6kModelSpecFor, xsnReferencesFromRequest)
}

// finalizeV6KSeedanceResponse reads the task the same way as the xsn, xm and naiwa platforms: the
// status comes from the explicit status field only (queued / in_progress / completed / failed; any
// other value counts as still processing), a failed task carries its reason in `error`, and billing
// uses the requested seconds.
func finalizeV6KSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// ppVideoV6KRemoveUpstreamFields drops what the upstream's task object says about its own
// accounting: the usage, cost and price figures are the supplier's, not ours.
func ppVideoV6KRemoveUpstreamFields(payload map[string]any) {
	ppVideoXMRemoveUpstreamFields(payload)
}

// fillPPVideoV6KCapabilities reports the limits of one model, read from the table the normalizer
// enforces, with the documented ratios.
func fillPPVideoV6KCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), v6kSeedanceRatios...)
}
