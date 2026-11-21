package service

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

// naiwa-seedance is the 奶蛙 sd2.5 line ("视频线路14") of the supplier site nailongapi.cc. It speaks
// the OpenAI-style video protocol with a JSON body: POST /v1/videos answers a task object with an
// `id` (the supplier prefixes it; it is stored and used as given, never edited), GET /v1/videos/{id}
// reports the task, and GET /v1/videos/{id}/content delivers the file (the supplier proxies the
// original task's content and its controlled redirects; the key must never be sent to a third-party
// address). Every call carries the upstream key as a Bearer credential.
//
// Request fields (the supplier's document, checked 2026-11-16): model (the exact id, upper / lower
// case matters), prompt (required, non-empty), duration (whole seconds, required; `seconds` and
// `duration_seconds` are the same field), resolution (lower case 480p / 720p), ratio (`width:height`;
// `aspect_ratio` / `aspectRatio` are the same field), the reference media as public HTTPS URL string
// arrays (image_urls / video_urls / audio_urls; images / videos / audios and reference_* are
// aliases that cannot be mixed within one kind) and count / n (only 1). The upstream refuses every
// other field (seed, generate_audio, multipart, remix, ...), so the normalizer builds a clean payload
// and only sends those. There is no first / last frame, no audio switch and no seed.
//
// Three models are offered (the operator's decision, 2026-11-17). They are described with the same
// table type as the jijiu platform (jijiuModelSpec) and run through the same normalizer
// (normalizeSeedanceSpecPayload) for the length, the resolution and the reference limits; the
// prompt length, the required resolution, the duration / ratio aliases and the upstream's field
// names are handled in normalizeNaiwaSeedancePayload:
//
//   - 奶蛙sd2.5-V: 720p only (left out, it defaults to 720p), 5-30 s, 30 images / 10 videos / 10
//     audios, prompt up to 15000 characters, billed per request (channel: 19 credits);
//   - 奶蛙sd2.5-flash-M: 480p / 720p (required), 4-30 s, 10 images / 5 videos / 5 audios, billed per
//     request by resolution (channel: 480p 12, 720p 17 credits; the same price for every length);
//   - 奶蛙sd2.5-M: 480p / 720p (required), 5-30 s, 30 images / 10 videos / 10 audios, native real-face
//     support, billed per request by resolution (channel: 480p 15, 720p 19 credits).
//
// The prices live in the channel, not in code (the supplier's own prices in its document are in its
// own credits and are not ours). The prompt limit of the two M models, the single-file size, the
// image pixels, the length of each reference clip and the combined limit across kinds are not known
// (the supplier lists them as unverified), so none of them is enforced here: a request that breaks
// one fails at the upstream and the hold is released. The ratio is only checked for the
// `width:height` shape because the supplier does not list the allowed set.
//
// The supplier states that the line makes one attempt in total and does not promise that a client
// request id is idempotent across services, so no Idempotency-Key is sent, and a lost or ambiguous
// answer to a submit must be checked against the original request, never re-sent.
//
// Names: the upstream ids contain Chinese text and no "seedance" (and differ only by the letter
// after the last dash), so clients that pick video models by name would not list them. Clients see
// Seedance2.5-NW-V / -NW-flash-M / -NW-M. The upstream id is the only one ever sent upstream;
// requests may use either name, and the model list, the task record and the response show the
// public one. Channel prices are read under the public name first, then the upstream id.
//
// Which models a key can call is decided by the key on the upstream, so an account's model
// whitelist holds only the models that key can use (the same as the jijiu, vshi, zexi and xm
// platforms).
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which reads
// the upstream's authenticated /content endpoint and, when that answers JSON with a download `url`
// instead of the file, follows the address without any credential (followXMContentResponse is
// shared with the xm platform). The upstream's own address is never stored or shown to a client.

// NaiwaSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). An account's own base_url takes precedence.
const NaiwaSeedanceDefaultBaseURL = "https://api.nailongapi.cc"

// naiwaPromptMaxChars is the prompt limit the supplier documents for the V model.
const naiwaPromptMaxChars = 15000

// naiwaRatioPattern is the only check on the ratio: the supplier says `width:height` (16:9, 9:16)
// and does not list the allowed set or the default, so the value is passed through.
var naiwaRatioPattern = regexp.MustCompile(`^[0-9]{1,3}:[0-9]{1,3}$`)

// naiwaSeedanceModels lists the models the gateway offers. Add a row, and a channel price, to make
// another upstream model callable.
var naiwaSeedanceModels = []jijiuModelSpec{
	{
		// 720p only; the resolution may be left out. Billed per request.
		Name: "奶蛙sd2.5-V", PublicName: "Seedance2.5-NW-V", MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10, MaxPromptChars: naiwaPromptMaxChars,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 480p / 720p, both required; 4-30 s. Billed per request by resolution.
		Name: "奶蛙sd2.5-flash-M", PublicName: "Seedance2.5-NW-flash-M", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, ForwardResolution: true, ResolutionRequired: true,
		MaxImages: 10, MaxVideos: 5, MaxAudios: 5,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 480p / 720p, both required; 5-30 s. Native real-face support. Billed per request by
		// resolution.
		Name: "奶蛙sd2.5-M", PublicName: "Seedance2.5-NW-M", MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, ForwardResolution: true, ResolutionRequired: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
}

// naiwaModelNames lists the public names of every model in the table, in table order. Account
// model sync uses it, so the whitelist an administrator saves holds the names clients see.
func naiwaModelNames() []string {
	names := make([]string, 0, len(naiwaSeedanceModels))
	for _, spec := range naiwaSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// naiwaModelSpecFor finds a model by either its upstream name or its public name, ignoring case
// (the upstream name sent is always the table's exact spelling).
func naiwaModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range naiwaSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsNaiwaSeedanceModel(model string) bool {
	_, ok := naiwaModelSpecFor(model)
	return ok
}

// ppVideoNaiwaPublicModel resolves either accepted name of a model to its public name.
func ppVideoNaiwaPublicModel(model string) (string, bool) {
	spec, ok := naiwaModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoNaiwaPublicModels maps a list of model names to the public names of the models that belong
// to this platform, without duplicates, so a whitelist saved under the upstream name is listed
// under the public one. Names that are not a model of this platform are dropped.
func ppVideoNaiwaPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoNaiwaPublicModel(model)
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

// ppVideoNaiwaModelAliases lists every name a model answers to, public name first.
func ppVideoNaiwaModelAliases(model string) []string {
	spec, ok := naiwaModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

// normalizeNaiwaSeedancePayload validates a native (v1) request and returns the upstream body.
//
// The shared normalizer enforces the model's length, resolution and reference limits and the
// prompt, and refuses frames, seed, watermark, callbacks, generate_audio and assets. On top of it
// this function checks what is specific to this upstream: the duration and ratio aliases must
// agree, a model with a required resolution must be given one, the V prompt limit, and the
// `width:height` shape of the ratio. The result is rewritten to the upstream's request, which names
// the length `duration`.
func normalizeNaiwaSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Seedance request body is required")
	}
	if err := naiwaAliasesAgree(body, "duration", "seconds", "duration_seconds"); err != nil {
		return nil, err
	}
	ratio, err := naiwaRatio(body)
	if err != nil {
		return nil, err
	}

	upstreamModel := strings.TrimSpace(public.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(public.Model)
	}
	spec, known := naiwaModelSpecFor(upstreamModel)
	if known && spec.ResolutionRequired {
		value := gjson.GetBytes(body, "resolution")
		if !value.Exists() || value.Type == gjson.Null || (value.Type == gjson.String && strings.TrimSpace(value.String()) == "") {
			return nil, fmt.Errorf("Seedance resolution is required for this model and must be %s", strings.Join(spec.Resolutions, " or "))
		}
	}

	generic, err := normalizeSeedanceSpecPayload(body, public, naiwaModelSpecFor, xsnReferencesFromRequest)
	if err != nil {
		return nil, err
	}
	if known && spec.MaxPromptChars > 0 {
		if prompt, _ := generic["prompt"].(string); utf8.RuneCountInString(prompt) > spec.MaxPromptChars {
			return nil, fmt.Errorf("Seedance prompt for this model must be at most %d characters", spec.MaxPromptChars)
		}
	}

	if seconds, ok := generic["seconds"]; ok {
		generic["duration"] = seconds
		delete(generic, "seconds")
	}
	if ratio != "" {
		generic["ratio"] = ratio
	}
	return generic, nil
}

// naiwaAliasesAgree refuses a request that carries two spellings of one field with different
// values (the supplier accepts the aliases only when they agree). A number and the same number
// written as a string count as equal.
func naiwaAliasesAgree(body []byte, paths ...string) error {
	first := ""
	firstPath := ""
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		current := strings.TrimSpace(value.String())
		if firstPath == "" {
			first, firstPath = current, path
			continue
		}
		if current != first {
			return fmt.Errorf("Seedance %s and %s must have the same value when both are given", firstPath, path)
		}
	}
	return nil
}

// naiwaRatio reads the ratio from any of its three spellings, which must agree, and checks only
// the `width:height` shape.
func naiwaRatio(body []byte) (string, error) {
	paths := []string{"ratio", "aspect_ratio", "aspectRatio"}
	ratio := ""
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.String {
			return "", fmt.Errorf("Seedance %s must be a string", path)
		}
		current := strings.TrimSpace(value.String())
		if ratio != "" && current != "" && current != ratio {
			return "", fmt.Errorf("Seedance ratio, aspect_ratio and aspectRatio must have the same value when more than one is given")
		}
		if current != "" {
			ratio = current
		}
	}
	if ratio != "" && !naiwaRatioPattern.MatchString(ratio) {
		return "", fmt.Errorf("Seedance ratio must be written as width:height, for example 16:9 or 9:16")
	}
	return ratio, nil
}

// finalizeNaiwaSeedanceResponse reads the task the same way as the xsn and xm platforms: the
// status comes from the explicit status field only (queued / in_progress / completed / failed;
// any other value, an unknown one included, counts as still processing because the supplier says an
// unknown result is neither success nor failure), a failed task carries its reason in `error`, and
// billing uses the requested seconds.
func finalizeNaiwaSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// ppVideoNaiwaRemoveUpstreamFields drops what the upstream's task object says about its own
// accounting: the usage, cost and price figures are the supplier's, not ours.
func ppVideoNaiwaRemoveUpstreamFields(payload map[string]any) {
	ppVideoXMRemoveUpstreamFields(payload)
}

// fillPPVideoNaiwaCapabilities reports the limits of one model, read from the table the normalizer
// enforces. The ratio is not listed because the supplier gives no set to promise.
func fillPPVideoNaiwaCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
}
