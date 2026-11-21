package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// kyy-wan is the Wan3.0 upstream of the Keyiyun / GlobalAiOpc "model center" (docs.globalaiopc.com,
// API host zcbservice.aizfw.cn). One task API serves every model of the center:
//
//	POST {base}/v2/model-center/tasks          body carries `model`, answers {id, object, created, model, status}
//	GET  {base}/v2/model-center/tasks/{id}     answers the same object plus progress, result_url / video_url, ...
//
// Both calls carry the upstream key as a Bearer credential. The status vocabulary is queued /
// processing / completed / failed; result_url and video_url are only present once the task is
// completed, and the address expires, so a client is told to download it promptly. The task
// object also reports `amount` (the credits the task cost the account) and `actualDuration`:
// those are the supplier's accounting, not ours, and are removed from every public response.
//
// The request fields of wan3.0-video-c3 are prompt (required), duration (2-30 whole seconds,
// required), resolution (480P / 720P / 1080P, upstream default 1080P), aspect_ratio (adaptive,
// 21:9, 16:9, 4:3, 1:1, 3:4, 9:16; upstream default adaptive), seed, reference_images (up to 10),
// reference_videos (up to 5, each shorter than 15 seconds, reference + output seconds below 30),
// reference_audios (up to 5), first_image and last_image. The upstream says that a task with
// reference videos is billed for the reference seconds plus the output seconds.
//
// The gateway keeps its own request shape (images / videos / audios / first_frame / last_frame,
// the same as the other video platforms) and maps it onto those fields; the upstream's own names
// (reference_images, first_image, aspect_ratio, ...) are accepted as aliases. Decisions of the
// operator (2026-11-20): everything the document offers is opened; limits the document does not
// state (whether first / last frames can be mixed with reference media, whether a last frame
// needs a first frame, how long a reference video may be) are NOT enforced here, because the
// upstream refuses what it does not support and the hold is released when it does. The same
// document says nothing about the exact JSON shape of first_image / last_image (the type column
// says image[] with at most one item, the sample request writes a plain string); a plain string
// is sent, as in the sample.
//
// Billing: the channel price of the model, per output second, in three tiers (480p / 720p /
// 1080p). The seconds of reference videos that the upstream adds to its own bill are NOT added
// here: the gateway cannot know how long a reference video is, so a request that carries long
// reference videos can cost more upstream than it earns here.
//
// Delivery: the upstream's result address is a direct link. It is passed on to the client like
// the other direct-link platforms (Wan3.0 on ModelVerse, Guangying) do; the task record page
// fetches it through the gateway without any credential.

// KyyWanDefaultBaseURL is the default upstream address (the `multimodalBaseUrl` of the upstream's
// documentation site). An account's own base_url takes precedence.
const KyyWanDefaultBaseURL = "https://zcbservice.aizfw.cn/kyyReactApiServer"

// KyyWanTasksPath is appended to the base address for both submission and status.
const KyyWanTasksPath = "/v2/model-center/tasks"

// KyyWanModelC3 is the only model offered so far.
const KyyWanModelC3 = "wan3.0-video-c3"

type kyyWanModelSpec struct {
	Name              string
	MinSeconds        int
	MaxSeconds        int
	Resolutions       []string // lower case; the upstream gets them upper case (480P)
	DefaultResolution string
	Ratios            []string
	MaxImages         int
	MaxVideos         int
	MaxAudios         int
}

// kyyWanModels lists the models the gateway offers. The upstream has three more Wan3.0 models
// (wan3.0-prime-c2, wan3.0-video-c2, wan3.0-video-r2v-c4) that were not asked for; add a row, and
// a channel price, to open one (check its document first: the limits differ per model).
var kyyWanModels = []kyyWanModelSpec{
	{
		Name: KyyWanModelC3, MinSeconds: 2, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p", "1080p"}, DefaultResolution: "1080p",
		Ratios: kyyWanRatios, MaxImages: 10, MaxVideos: 5, MaxAudios: 5,
	},
}

// kyyWanRatios are the ratios the upstream documents for wan3.0-video-c3.
var kyyWanRatios = []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}

// kyyWanRefusedFields are fields of other protocols that this model does not take. They are
// refused with a message instead of being dropped, because dropping them would mislead the caller
// (for example `audio` and `prompt_extend` exist on the ModelVerse Wan3.0 platform but not here).
var kyyWanRefusedFields = []string{
	"watermark", "callback", "callback_url", "callback_token",
	"generate_audio", "enable_audio", "audio", "prompt_extend",
	"negative_prompt", "quality", "fps", "draft", "return_last_frame",
	"input", "parameters", "media", "content",
	"input_reference", "image_end", "start_frame_url", "end_frame_url", "first_frame_url", "last_frame_url",
	"file_url", "link", "link_url",
}

// kyyWanModelNames lists the names of every model in the table, in table order. Account model
// sync uses it.
func kyyWanModelNames() []string {
	names := make([]string, 0, len(kyyWanModels))
	for _, spec := range kyyWanModels {
		names = append(names, spec.Name)
	}
	return names
}

// kyyWanModelSpecFor finds a model by name, ignoring case.
func kyyWanModelSpecFor(model string) (kyyWanModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return kyyWanModelSpec{}, false
	}
	for _, spec := range kyyWanModels {
		if strings.EqualFold(spec.Name, model) {
			return spec, true
		}
	}
	return kyyWanModelSpec{}, false
}

func ppVideoIsKyyWanModel(model string) bool {
	_, ok := kyyWanModelSpecFor(model)
	return ok
}

// ppVideoKyyWanPublicModels keeps the models of this platform, under their canonical spelling and
// without duplicates. The upstream's model list holds every model of the center, so names that
// are not in the table are dropped.
func ppVideoKyyWanPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		spec, ok := kyyWanModelSpecFor(model)
		if !ok {
			continue
		}
		if _, exists := seen[spec.Name]; exists {
			continue
		}
		seen[spec.Name] = struct{}{}
		filtered = append(filtered, spec.Name)
	}
	return filtered
}

func (s kyyWanModelSpec) allowsResolution(resolution string) bool {
	for _, candidate := range s.Resolutions {
		if candidate == resolution {
			return true
		}
	}
	return false
}

func (s kyyWanModelSpec) allowsRatio(ratio string) bool {
	for _, candidate := range s.Ratios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// kyyWanError rewrites the wording of the shared Seedance helpers (they name the model family in
// their messages) so a client of this model never reads "Seedance".
func kyyWanError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), "Seedance", "Wan3.0"))
}

// normalizeKyyWanPayload validates a native (v1) request and returns the upstream body.
func normalizeKyyWanPayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	result, err := normalizeKyyWanPayloadRaw(body, public)
	return result, kyyWanError(err)
}

func normalizeKyyWanPayloadRaw(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Wan3.0 request body is required")
	}
	if !public.HasExplicitModel {
		return nil, fmt.Errorf("Wan3.0 model is required")
	}
	requestedModel := strings.TrimSpace(public.UpstreamModel)
	if requestedModel == "" {
		requestedModel = strings.TrimSpace(public.Model)
	}
	spec, ok := kyyWanModelSpecFor(requestedModel)
	if !ok {
		return nil, fmt.Errorf("model %q is not available", public.Model)
	}

	for _, path := range kyyWanRefusedFields {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Wan3.0 does not support %s", path)
		}
	}
	if value := gjson.GetBytes(body, "assets"); value.Exists() && value.Type != gjson.Null {
		return nil, fmt.Errorf("Wan3.0 does not accept assets on this endpoint; use images, videos and audios, or the unified v2 endpoint")
	}
	for _, path := range []string{"n", "count", "video_count"} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.Number || value.Float() != 1 {
			return nil, fmt.Errorf("Wan3.0 supports one video per request")
		}
	}

	refs, err := xsnReferencesFromRequest(body)
	if err != nil {
		return nil, err
	}
	if len(refs.images) > spec.MaxImages {
		return nil, fmt.Errorf("Wan3.0 accepts at most %d reference images", spec.MaxImages)
	}
	if len(refs.videos) > spec.MaxVideos {
		return nil, fmt.Errorf("Wan3.0 accepts at most %d reference videos", spec.MaxVideos)
	}
	if len(refs.audios) > spec.MaxAudios {
		return nil, fmt.Errorf("Wan3.0 accepts at most %d reference audios", spec.MaxAudios)
	}

	firstFrame, err := kyyWanFrame(body, "first_frame", "first_image")
	if err != nil {
		return nil, err
	}
	lastFrame, err := kyyWanFrame(body, "last_frame", "last_image")
	if err != nil {
		return nil, err
	}

	prompt, err := jijiuPrompt(body)
	if err != nil {
		return nil, err
	}
	seconds, err := jijiuDurationSeconds(body, jijiuModelSpec{MinSeconds: spec.MinSeconds, MaxSeconds: spec.MaxSeconds}, refs)
	if err != nil {
		return nil, err
	}
	resolution, err := jijiuResolution(body, jijiuModelSpec{Resolutions: spec.Resolutions, DefaultResolution: spec.DefaultResolution})
	if err != nil {
		return nil, err
	}
	if !spec.allowsResolution(resolution) {
		return nil, fmt.Errorf("Wan3.0 resolution must be %s", strings.Join(spec.Resolutions, ", "))
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !spec.allowsRatio(ratio) {
		return nil, fmt.Errorf("Wan3.0 ratio must be one of %s", strings.Join(spec.Ratios, ", "))
	}
	seed, hasSeed, err := wan30SeedFromRequest(body)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"model":      spec.Name,
		"prompt":     prompt,
		"duration":   seconds,
		"resolution": strings.ToUpper(resolution),
	}
	if ratio != "" {
		result["aspect_ratio"] = ratio
	}
	if hasSeed {
		result["seed"] = seed
	}
	if len(refs.images) > 0 {
		result["reference_images"] = jijiuStringItems(refs.images)
	}
	if len(refs.videos) > 0 {
		result["reference_videos"] = jijiuStringItems(refs.videos)
	}
	if len(refs.audios) > 0 {
		result["reference_audios"] = jijiuStringItems(refs.audios)
	}
	if firstFrame != "" {
		result["first_image"] = firstFrame
	}
	if lastFrame != "" {
		result["last_image"] = lastFrame
	}

	public.Model = spec.Name
	public.UpstreamModel = spec.Name
	public.Prompt = prompt
	public.DurationMilliseconds = int64(seconds) * 1000
	public.VideoCount = 1
	public.Resolution = resolution
	public.HasImage = len(refs.images) > 0 || firstFrame != "" || lastFrame != ""
	return result, nil
}

// kyyWanFrame reads one frame image written as a bare URL, {"url": ...} or {"image_url": ...}
// under either of its two names (the gateway's first_frame / last_frame, the upstream's
// first_image / last_image). Naming both is refused so the two can never disagree. Only a public
// HTTPS address is accepted.
func kyyWanFrame(body []byte, keys ...string) (string, error) {
	found := ""
	foundKey := ""
	for _, key := range keys {
		value := gjson.GetBytes(body, key)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if foundKey != "" {
			return "", fmt.Errorf("Wan3.0 accepts only one of %s and %s", foundKey, key)
		}
		rawURL, ok := jijiuMediaURL(value, "image_url")
		if !ok || rawURL == "" {
			return "", fmt.Errorf("Wan3.0 %s must be a URL", key)
		}
		if err := validateJijiuHTTPSURL(key, rawURL); err != nil {
			return "", err
		}
		found, foundKey = rawURL, key
	}
	return found, nil
}

// finalizeKyyWanResponse reads what the generic parser does not. The status field decides: the
// generic parser turns any response that carries a result address into a success, but this
// upstream's task that still says queued / processing is not finished, whatever else it carries.
// (A task that says completed without an address stays unfinished, as the generic parser has it.)
// A failed task carries its reason in `error` (a plain string, or an object with a message), and a
// failed task without any reason still gets a generic one so the client is never left with an
// empty failure.
func finalizeKyyWanResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	if result.Status == PPVideoTaskStatusSucceeded &&
		NormalizePPVideoTaskStatus(extractPPVideoText(body, "status", "state")) == PPVideoTaskStatusProcessing {
		result.Status = PPVideoTaskStatusProcessing
	}
	if result.ErrorMessage == "" {
		if value := gjson.GetBytes(body, "error"); value.Type == gjson.String {
			result.ErrorMessage = strings.TrimSpace(value.String())
		}
	}
	if result.ErrorMessage == "" && result.Status == PPVideoTaskStatusFailed {
		result.ErrorMessage = "video generation failed"
	}
	// Billing uses the seconds that were requested: out-of-range lengths are refused before
	// submission, so they equal the seconds produced, and a length echoed in the response (which
	// may count reference media) must not replace them at settlement.
	result.GeneratedDurationMilliseconds = 0
	result.InputVideoDurationMilliseconds = 0
	return result
}

// ppVideoKyyWanModelAliases lists the canonical spelling of a model whatever the case of the
// request, so an account whitelist saved under the canonical name still matches a request written
// in another case.
func ppVideoKyyWanModelAliases(model string) []string {
	spec, ok := kyyWanModelSpecFor(model)
	if !ok {
		return nil
	}
	return []string{spec.Name}
}

// ppVideoKyyWanRemoveUpstreamFields drops what the upstream's task object says about its own
// accounting: `amount` is the supplier's price to us, `actualDuration` its own time figure, and
// media_url / error_message / task_id repeat other fields under the supplier's names.
func ppVideoKyyWanRemoveUpstreamFields(payload map[string]any) {
	if payload == nil {
		return
	}
	for _, key := range []string{"amount", "actualDuration", "media_url", "error_message"} {
		delete(payload, key)
	}
}

// fillPPVideoKyyWanCapabilities reports the limits of one model, read from the table the
// normalizer enforces. The upstream has no audio switch and no callback.
func fillPPVideoKyyWanCapabilities(capabilities *PPVideoModelCapabilities, spec kyyWanModelSpec) {
	capabilities.Duration = &PPVideoDurationRange{Min: spec.MinSeconds, Max: spec.MaxSeconds}
	capabilities.Resolutions = append([]string(nil), spec.Resolutions...)
	capabilities.Ratios = append([]string(nil), spec.Ratios...)
	capabilities.Assets = &PPVideoAssetCapabilities{
		FirstLastFrame:  ppVideoCapBool(true),
		ReferenceImages: ppVideoCapInt(spec.MaxImages),
		ReferenceVideos: ppVideoCapInt(spec.MaxVideos),
		ReferenceAudios: ppVideoCapInt(spec.MaxAudios),
	}
	capabilities.GenerateAudio = ppVideoCapBool(false)
	capabilities.Callback = ppVideoCapBool(false)
}

// toKyyWan translates a v2 request for the kyy-wan platform. model, prompt and duration are
// required (the upstream requires prompt and duration; model picks the table row). The first /
// last frame assets become first_frame / last_frame, references become images / videos / audios,
// and the native normalizer then enforces the limits and writes the upstream request. Seed is
// accepted; the audio switch, watermark, callbacks, metadata and options are rejected: the
// upstream has none, and accepting more later is compatible whereas tightening is not. Asset
// names are not forwarded: the upstream only knows the position.
func (r *ppVideoUnifiedRequest) toKyyWan() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{seed: true}); err != nil {
		return nil, err
	}
	if r.model == "" {
		return nil, ppVideoUnifiedErr("model", "is required for this model group")
	}
	if len(r.options) > 0 {
		return nil, ppVideoUnifiedUnsupportedOption(r.options[0])
	}
	if r.prompt == "" {
		return nil, ppVideoUnifiedErr("prompt", "is required for this model")
	}
	if r.duration == nil {
		return nil, ppVideoUnifiedErr("duration", "is required for this model")
	}

	media := r.groupAssets()
	payload := map[string]any{"model": r.model, "prompt": r.prompt, "duration": *r.duration}
	if r.resolution != "" {
		payload["resolution"] = r.resolution
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
	}
	if r.seed != nil {
		payload["seed"] = *r.seed
	}
	if media.firstFrame != nil {
		payload["first_frame"] = media.firstFrame.URL
	}
	if media.lastFrame != nil {
		payload["last_frame"] = media.lastFrame.URL
	}
	if len(media.referenceImages) > 0 {
		payload["images"] = ppVideoUnifiedURLs(media.referenceImages)
	}
	if len(media.referenceVideos) > 0 {
		payload["videos"] = ppVideoUnifiedURLs(media.referenceVideos)
	}
	if len(media.referenceAudios) > 0 {
		payload["audios"] = ppVideoUnifiedURLs(media.referenceAudios)
	}
	return payload, nil
}
