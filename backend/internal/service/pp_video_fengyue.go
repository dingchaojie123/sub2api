package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// fengyue-seedance is a Seedance upstream that speaks an OpenAI-style video protocol with its own
// request shape: POST /v1/videos answers a task object with an `id`, GET /v1/videos/{id} reports
// queued / in_progress / completed / failed, and GET /v1/videos/{id}/content delivers the file
// (every call carries the upstream token as a Bearer credential, the same way as the xsn
// platform). The request fields are model, prompt, seconds, resolution, aspect_ratio, mode and
// `references`, an array of {"kind": "image" | "video" | "audio", "data": <public https url>}; any
// other key of a reference item is refused by the upstream.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload) for the prompt, the length, the
// resolution and the reference limits. The normalizer writes the three URL arrays under the names
// image_urls / video_urls / audio_urls; normalizeFengyueSeedancePayload then folds them into the
// upstream's `references` array (images, then videos, then audios) and adds mode.
//
// The only model is named sd2.5pp by the upstream and Seedance2.5pp to clients (the operator's
// rename, 2026-11-12: the upstream name carries no "seedance", so clients that pick video models by
// name did not list it). The upstream name is the only one ever sent upstream; requests may use
// either name, and the model list, the task record and the response show the public one. It takes
// 5-30 whole seconds at 720p, only the omni_reference mode, and at
// least one reference image or video (audio alone is refused by the upstream). The upstream also
// offers 1080p at the same price; this gateway sells 720p only (the public name of the aggregate
// line says 720p), and allowing 1080p later is compatible. Reference media is not charged extra.
// The upstream prices a request by length band (5-15 seconds and 16-30 seconds); this gateway
// charges one flat per-request channel price for every length (the operator's decision: 19
// credits), so the model is billed per request, not per second. A request for a model that is not
// in the table is refused locally (HTTP 400).
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which
// streams the upstream's authenticated /content endpoint. The upstream's own address is never
// stored or shown to a client.

// FengyueSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). It is the address of the site the upstream's integration document was saved
// from; an account's own base_url takes precedence.
const FengyueSeedanceDefaultBaseURL = "https://api.nldw.online"

// fengyueModeOmniReference is the only generation mode of the upstream.
const fengyueModeOmniReference = "omni_reference"

// FengyueSeedanceUpstreamModel is the name the upstream knows (the only one sent to it) and
// FengyueSeedancePublicModel the name clients see.
const (
	FengyueSeedanceUpstreamModel = "sd2.5pp"
	FengyueSeedancePublicModel   = "Seedance2.5pp"
)

// fengyueSeedanceModels lists the models whose limits are known. Add a row, and a channel price,
// to make another upstream model callable.
var fengyueSeedanceModels = []jijiuModelSpec{
	{
		// 5-30 seconds, 720p only (the upstream also takes 1080p; not sold here), 30 images, 10
		// videos and 10 audios (50 items in total). Per-file size and per-video / per-audio length
		// limits are enforced by the upstream.
		Name: FengyueSeedanceUpstreamModel, PublicName: FengyueSeedancePublicModel, MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10, MaxReferenceTotal: 50,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "aspect_ratio",
	},
}

// fengyueAspectRatios are the ratios the upstream documents (16:9 when omitted).
var fengyueAspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}

// fengyueModelNames lists the public names of every model in the table, in table order. Account
// model sync uses it, so the whitelist an administrator saves holds the names clients see.
func fengyueModelNames() []string {
	names := make([]string, 0, len(fengyueSeedanceModels))
	for _, spec := range fengyueSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// ppVideoFengyuePublicModels maps a list of model names to the public names of the models that
// belong to this platform, without duplicates, so a whitelist saved under the upstream name is
// listed under the public one. Names that are not a model of this platform are dropped.
func ppVideoFengyuePublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoFengyuePublicModel(model)
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

// fengyueModelSpecFor finds a model by either its upstream name or its public name, ignoring case.
func fengyueModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range fengyueSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsFengyueSeedanceModel(model string) bool {
	_, ok := fengyueModelSpecFor(model)
	return ok
}

// ppVideoFengyuePublicModel resolves either accepted name of a model to its public name.
func ppVideoFengyuePublicModel(model string) (string, bool) {
	spec, ok := fengyueModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoFengyueModelAliases lists every name a model answers to, public name first.
func ppVideoFengyueModelAliases(model string) []string {
	spec, ok := fengyueModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func fengyueAspectRatioAllowed(ratio string) bool {
	for _, candidate := range fengyueAspectRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeFengyueSeedancePayload validates a native (v1) request and returns the upstream body.
// The shared normalizer refuses frames, seed, watermark, callbacks, generate_audio and assets, and
// enforces the model's length, resolution and reference limits; the ratio is checked here against
// the documented set, and the references are rewritten into the upstream's `references` array.
func normalizeFengyueSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !fengyueAspectRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(fengyueAspectRatios, ", "))
	}
	mode, err := jijiuOptionalString(body, "mode")
	if err != nil {
		return nil, err
	}
	if mode != "" && mode != fengyueModeOmniReference {
		return nil, fmt.Errorf("Seedance mode must be %s", fengyueModeOmniReference)
	}
	payload, err := normalizeSeedanceSpecPayload(body, public, fengyueModelSpecFor, fengyueReferencesFromRequest)
	if err != nil {
		return nil, err
	}
	return fengyueFoldReferences(payload), nil
}

// fengyueFoldReferences moves the three URL arrays the shared normalizer wrote into the upstream's
// single `references` array (images, then videos, then audios) and selects the only mode.
func fengyueFoldReferences(payload map[string]any) map[string]any {
	references := make([]any, 0)
	for _, group := range []struct{ field, kind string }{
		{"image_urls", "image"},
		{"video_urls", "video"},
		{"audio_urls", "audio"},
	} {
		items, _ := payload[group.field].([]any)
		for _, item := range items {
			references = append(references, map[string]any{"kind": group.kind, "data": item})
		}
		delete(payload, group.field)
	}
	payload["references"] = references
	payload["mode"] = fengyueModeOmniReference
	return payload
}

// fengyueReferencesFromRequest reads the media of a native (v1) request. It accepts the aliases
// the other Seedance models of this gateway document (images / videos / audios, image_urls / ...,
// reference_*, camelCase variants, single image). Two different aliases of the same kind in one
// request are refused so an item is never counted twice. The upstream needs at least one image or
// one video: a request that carries only audio, or nothing, is refused here instead of after the
// upstream has been called.
func fengyueReferencesFromRequest(body []byte) (jijiuReferences, error) {
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
	fromField, present, err := fengyueReferencesField(body)
	if err != nil {
		return result, err
	}
	if present {
		if result.count() > 0 {
			return result, fmt.Errorf("Seedance accepts either references or images, videos and audios, not both")
		}
		result = fromField
	}
	if len(result.images)+len(result.videos) == 0 {
		return result, fmt.Errorf("Seedance requires at least one reference image or video for this model (audio alone is not accepted)")
	}
	return result, nil
}

// fengyueReferencesField reads the upstream's own `references` array: items of {"kind": image |
// video | audio, "data": <public https url>} and no other key. Inline data: addresses are not
// accepted (the gateway forwards public links only). The second result tells whether the field
// was present at all.
func fengyueReferencesField(body []byte) (jijiuReferences, bool, error) {
	var result jijiuReferences
	value := gjson.GetBytes(body, "references")
	if !value.Exists() || value.Type == gjson.Null {
		return result, false, nil
	}
	if !value.IsArray() {
		return result, true, fmt.Errorf("Seedance references must be an array")
	}
	for index, item := range value.Array() {
		if !item.IsObject() {
			return result, true, fmt.Errorf("Seedance references[%d] must be an object with kind and data", index)
		}
		unknownKey := ""
		item.ForEach(func(key, _ gjson.Result) bool {
			if key.String() != "kind" && key.String() != "data" {
				unknownKey = key.String()
				return false
			}
			return true
		})
		if unknownKey != "" {
			return result, true, fmt.Errorf("Seedance references[%d] accepts only kind and data, not %q", index, unknownKey)
		}
		kind := item.Get("kind")
		data := item.Get("data")
		if kind.Type != gjson.String || data.Type != gjson.String {
			return result, true, fmt.Errorf("Seedance references[%d] must have a string kind and a string data", index)
		}
		address := strings.TrimSpace(data.String())
		if err := validateJijiuHTTPSURL(fmt.Sprintf("references[%d].data", index), address); err != nil {
			return result, true, err
		}
		switch strings.ToLower(strings.TrimSpace(kind.String())) {
		case "image":
			result.images = append(result.images, address)
		case "video":
			result.videos = append(result.videos, address)
		case "audio":
			result.audios = append(result.audios, address)
		default:
			return result, true, fmt.Errorf("Seedance references[%d].kind must be image, video or audio", index)
		}
	}
	return result, true, nil
}

// finalizeFengyueSeedanceResponse reads the task the same way as the xsn platform: the status
// comes from the explicit status field only (queued / in_progress / completed / failed), a failed
// task carries its reason in `error` (a string or an object with a message), and billing uses the
// requested seconds.
func finalizeFengyueSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// fillPPVideoFengyueCapabilities reports the limits of one model, read from the table the
// normalizer enforces.
func fillPPVideoFengyueCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), fengyueAspectRatios...)
}
