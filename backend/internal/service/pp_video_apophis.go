package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// apophis-seedance is a Seedance upstream that is itself a gateway site (ApophisCode) speaking
// the OpenAI-style video protocol: POST /v1/videos answers a task object
// {"id", "object": "video", "status": "queued", ...}, GET /v1/videos/{id} reports
// queued / in_progress / completed / failed with progress and error {code, message}, and
// GET /v1/videos/{id}/content delivers the file, answering 302 to the upstream's own link on some
// lines (the gateway's HTTP client follows it). The site sells three families of lines:
//
//   - third-party lines without a name prefix (Seedance2.0-720p-按秒-gf, Seedance2.5-720p-按次-Z满血,
//     ...): fixed resolution, billed per second or per request. These are the ones described here.
//   - 官转- / 国内官转- lines billed by the real video tokens, which the site settles after the
//     video is done (billing_pending, video_mode edit / extend, duration -1, seed, generate_audio).
//     Not supported: they need the token settlement of the mai-token platform and were left out
//     on purpose.
//   - MiniMax-H3 lines: served by the minimax-h3 (CompShare) platform instead.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload) for prompt, duration, resolution
// and reference limits; normalizeApophisSeedancePayload then reads the first / last frame, and
// renames the result to the upstream's field names. The upstream rejects fields it does not
// know ("unsupported field"), so a clean payload is built and only documented fields are sent.
// The upstream lists more models than are described here. They can be discovered and added to an
// account, but a request for one that is not in the table is refused locally (HTTP 400).

// ApophisSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the
// gateway normalizes it).
const ApophisSeedanceDefaultBaseURL = "https://www.apophiscode.com"

// apophisSeedanceModels lists the third-party lines of the upstream's capability table. The
// resolution is fixed by the model name, so it is never sent. The billing mode (按秒 / 按次) is in
// the name and is decided by the channel price, not by this table. Where the table lists no
// reference videos the model refuses them locally.
var apophisSeedanceModels = []jijiuModelSpec{
	// Seedance 2.0
	{Name: "Seedance2.0-720p-按次-903", MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3, FirstLastFrame: true},
	{Name: "Seedance2.0-720p-按秒-gf", MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3},
	{Name: "Seedance2.0-720p-按次-X", MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3},
	{Name: "Seedance2.0-720p-按秒-Xs", MinSeconds: 5, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3},
	{Name: "Seedance2.0-720p-按次-Xs", MinSeconds: 5, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3},
	{Name: "Seedance2.0-720p-按秒-max", MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3},
	// Seedance 2.5
	{Name: "Seedance2.5-720p-按秒-9图", MinSeconds: 4, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3, FirstLastFrame: true},
	{Name: "Seedance2.5-1080p-按次-Y", MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"1080p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3},
	{Name: "Seedance2.5-480p-按秒-Z", MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"480p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	{Name: "Seedance2.5-720p-按秒-Z", MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	{Name: "Seedance2.5-720p-按次-Z满血", MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	{Name: "Seedance2.5-720p-按秒-301010", MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	// The only model with a single allowed length (the capability table lists "30 秒").
	{Name: "Seedance2.5-720p-按次-301010", MinSeconds: 30, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	{Name: "Seedance2.5-720p-按次-gz", MinSeconds: 4, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	{Name: "Seedance2.5-720p-按次-满血全参", MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
}

// apophisAspectRatios are the ratios the upstream documents for these lines (adaptive is for
// the official-forward lines only).
var apophisAspectRatios = []string{"16:9", "9:16", "4:3", "3:4", "1:1", "21:9"}

// apophisFrameKeys are the names a first / last frame may be sent under: the upstream's own
// (first_frame, end_frame) and last_frame as an alias of end_frame.
var apophisFrameKeys = []string{"first_frame", "end_frame", "last_frame"}

// apophisModelNames lists the upstream names of every model in the table, in table order.
func apophisModelNames() []string {
	names := make([]string, 0, len(apophisSeedanceModels))
	for _, spec := range apophisSeedanceModels {
		names = append(names, spec.Name)
	}
	return names
}

// apophisModelSpecFor finds a model by name, ignoring case.
func apophisModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range apophisSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsApophisSeedanceModel(model string) bool {
	_, ok := apophisModelSpecFor(model)
	return ok
}

// ppVideoApophisPublicModel resolves a model name to its public name (the upstream name: this
// platform has no separate public names).
func ppVideoApophisPublicModel(model string) (string, bool) {
	spec, ok := apophisModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoApophisModelAliases lists every name a model answers to, public name first.
func ppVideoApophisModelAliases(model string) []string {
	spec, ok := apophisModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func apophisAspectRatioAllowed(ratio string) bool {
	for _, candidate := range apophisAspectRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeApophisSeedancePayload validates a native (v1) request and returns the upstream body.
//
// Fields that only the token-billed official-forward lines support (video_mode, video_durations,
// web_search, generate_audio, seed, watermark, return_last_frame, duration -1) are refused rather
// than dropped: dropping video_mode=extend_forward, for example, would silently turn an extension
// into a new video. Accepting more later is compatible. A first / last frame is accepted only for
// the models whose capability row says so, an end frame needs a first frame (a first frame alone
// is fine), and frames cannot be combined with reference images, videos or audios (the same rule as the other Seedance
// platforms of this gateway).
func normalizeApophisSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Seedance request body is required")
	}
	for _, path := range []string{"video_mode", "video_durations", "web_search"} {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Seedance does not support %s for this model", path)
		}
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !apophisAspectRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(apophisAspectRatios, ", "))
	}

	// The frames are read here and removed from the body the shared normalizer sees, because it
	// refuses frames outright (most Seedance platforms have none).
	first, last, stripped, err := apophisFrames(body)
	if err != nil {
		return nil, err
	}
	hasFrames := first != "" || last != ""
	if hasFrames {
		upstreamModel := strings.TrimSpace(public.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = strings.TrimSpace(public.Model)
		}
		if spec, ok := apophisModelSpecFor(upstreamModel); ok && !spec.FirstLastFrame {
			return nil, fmt.Errorf("Seedance does not support first or last frames for this model")
		}
		// The upstream takes a first frame alone, but an end frame only together with a first one.
		if first == "" {
			return nil, fmt.Errorf("Seedance end_frame requires first_frame")
		}
	}

	generic, err := normalizeSeedanceSpecPayload(stripped, public, apophisModelSpecFor, apophisReferencesFromRequest)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"model":    generic["model"],
		"prompt":   generic["prompt"],
		"duration": generic["seconds"],
	}
	if value, ok := generic["aspect_ratio"]; ok {
		result["aspect_ratio"] = value
	}
	references := 0
	for _, key := range []string{"images", "videos"} {
		if items, ok := generic[key].([]any); ok && len(items) > 0 {
			result[key] = items
			references += len(items)
		}
	}
	if audios, ok := generic["audios"].([]any); ok && len(audios) > 0 {
		result["audio"] = audios
		references += len(audios)
	}
	if hasFrames {
		if references > 0 {
			return nil, fmt.Errorf("Seedance first_frame and end_frame cannot be combined with reference images, videos or audios")
		}
		result["first_frame"] = first
		if last != "" {
			result["end_frame"] = last
		}
		public.HasImage = true
	}
	return result, nil
}

// apophisFrames reads the first and last frame of a request (bare URL, {"url": ...} or
// {"image_url": ...}; HTTPS only) and returns the body without those keys.
func apophisFrames(body []byte) (first string, last string, stripped []byte, err error) {
	stripped = body
	values := map[string]string{}
	for _, key := range apophisFrameKeys {
		value := gjson.GetBytes(body, key)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		rawURL, ok := jijiuMediaURL(value, "image_url")
		if !ok || rawURL == "" {
			return "", "", nil, fmt.Errorf("Seedance %s must be a URL", key)
		}
		if err := validateJijiuHTTPSURL(key, rawURL); err != nil {
			return "", "", nil, err
		}
		values[key] = rawURL
		if stripped, err = sjson.DeleteBytes(stripped, key); err != nil {
			return "", "", nil, fmt.Errorf("Seedance %s is invalid", key)
		}
	}
	if values["end_frame"] != "" && values["last_frame"] != "" {
		return "", "", nil, fmt.Errorf("Seedance accepts only one of end_frame and last_frame")
	}
	last = values["end_frame"]
	if last == "" {
		last = values["last_frame"]
	}
	return values["first_frame"], last, stripped, nil
}

// apophisReferencesFromRequest reads the media of a native (v1) request: the upstream's own
// shape (images, videos, audio) and the aliases the other Seedance models of this gateway
// document (audios, reference_*, camelCase variants, single image). Two aliases of the same
// kind in one request are refused by jijiuURLList so an item is never counted twice.
func apophisReferencesFromRequest(body []byte) (jijiuReferences, error) {
	var result jijiuReferences
	var err error
	if result.images, err = jijiuURLList(body, "image_url",
		[]string{"images", "image_urls", "reference_images", "referenceImages", "reference_image_urls"},
		[]string{"image", "image_url"}); err != nil {
		return result, err
	}
	if result.videos, err = jijiuURLList(body, "video_url",
		[]string{"videos", "video_urls", "reference_videos", "referenceVideos", "video_refs"},
		nil); err != nil {
		return result, err
	}
	if result.audios, err = jijiuURLList(body, "audio_url",
		[]string{"audio", "audios", "audio_urls", "reference_audios", "referenceAudios", "reference_audio_urls", "audio_refs"},
		nil); err != nil {
		return result, err
	}
	return result, nil
}

// finalizeApophisSeedanceResponse takes the status from the explicit status field only
// (queued / in_progress / completed / failed) and bills the requested seconds, the same way as
// the jijiu platform: the `seconds` the upstream echoes plays no part. A failed task carries
// error.message; a rejected request may carry a plain string in `error`, which is read as well.
func finalizeApophisSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	result = finalizeJijiuSeedanceResponse(result, body)
	if result.ErrorMessage == "" {
		if value := gjson.GetBytes(body, "error"); value.Type == gjson.String {
			result.ErrorMessage = strings.TrimSpace(value.String())
		}
	}
	return result
}

// fillPPVideoApophisCapabilities reports the limits of one model, read from the table the
// normalizer enforces.
func fillPPVideoApophisCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), apophisAspectRatios...)
	if capabilities.Assets != nil {
		capabilities.Assets.FirstLastFrame = ppVideoCapBool(spec.FirstLastFrame)
	}
}
