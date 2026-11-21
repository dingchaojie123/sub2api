package service

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// zexi-seedance is a Seedance upstream (泽西同学, zexitongxue.com) that speaks the OpenAI-style video
// protocol: POST /v1/videos answers a task object with an `id`, GET /v1/videos/{id} reports
// queued / in_progress / completed / failed / cancelled, and GET /v1/videos/{id}/content delivers
// the file (every call carries the upstream key as a Bearer credential; the upstream answers the
// file itself, supports a single byte Range and does not redirect to an outside address). The
// request fields are model, prompt, duration (whole seconds), resolution, ratio, generate_audio
// (boolean), seed (-1 or a non-negative integer) and `content`, an array of reference media with a
// role each: reference_image, first_frame, last_frame (all image_url items), reference_video
// (video_url) and reference_audio (audio_url). Media must be public HTTPS addresses.
//
// The gateway keeps its own request shape (images / videos / audios / first_frame / last_frame,
// the same as the other Seedance platforms) and builds the upstream's `content` array from it. The
// upstream's own `content` array, its edit / extend task types (omni_reference_task_type), the
// automatic length (duration -1) and its output format are not offered; a request that carries one
// of them is refused instead of having it dropped.
//
// Four models are offered (the operator's decision, 2026-11-16): doubao-seedance-2.5-nsfw-95,
// doubao-seedance-2.5-nsfw, doubao-seedance-2.0-nsfw-95 and doubao-seedance-2.0-nsfw. The upstream
// document lists neither of them: it lists the plain doubao-seedance-2.5 and doubao-seedance-2.0
// models, and the operator says the four have the same capabilities and differ only in the
// upstream group (and therefore the price). So the limits below are the ones the document gives for
// the plain models of the same version (2.5: 4-30 seconds, 480p / 720p / 1080p, 30 images / 10
// videos / 10 audios; 2.0: 4-15 seconds, 480p / 720p / 1080p / 4k, 9 / 3 / 3). The upstream refuses
// a combination it does not support with an explicit error, so a row that is too wide fails at
// the upstream (the hold is released) rather than being billed wrongly. ASSUMED, not read from
// the upstream: that the four models take the same limits as the plain ones; check them with one
// request per model and correct the rows.
//
// Which models a key can call is decided by the key's group on the upstream (the document says to
// pick the model and a callable group first and to submit with that group's key), so each upstream
// group gets its own account and the account's model whitelist holds only the models that key
// can use, the same way as the jijiu and vshi platforms. Billing (per second, the operator's
// decision) lives in the channel, not in code.
//
// The public name of a model is its upstream name (all four carry "seedance"), so there is no
// rename and no alias.
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which streams
// the upstream's authenticated /content endpoint. The upstream's own address is never stored or
// shown to a client. The upstream does not extend the life of a source file: once it expires or is
// deleted the download fails although the task still reads "completed".

// ZexiSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). It is the site the upstream's integration document was saved from; an
// account's own base_url takes precedence.
const ZexiSeedanceDefaultBaseURL = "https://zexitongxue.com"

// zexiMaxSeed is the largest seed the upstream takes (the largest safe JSON integer).
const zexiMaxSeed = 9007199254740991

// zexiSeedanceRatios are the ratios the upstream documents for its Seedance models. adaptive lets
// the model follow the reference media.
var zexiSeedanceRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}

// zexiRefusedFields are the upstream's own spellings and features this gateway does not offer.
// They are refused with a message instead of being ignored: ignoring `content` would drop the
// media, and ignoring omni_reference_task_type would turn an edit into a new video.
var zexiRefusedFields = []string{
	"media", "content",
	"omni_reference_task_type", "out_format", "format",
	"video_mode", "video_durations", "web_search",
}

func zexiSeedance25(name string) jijiuModelSpec {
	return jijiuModelSpec{
		Name: name, MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p", "1080p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10, FirstLastFrame: true,
	}
}

func zexiSeedance20(name string) jijiuModelSpec {
	return jijiuModelSpec{
		Name: name, MinSeconds: 4, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p", "1080p", "4k"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3, FirstLastFrame: true,
	}
}

// zexiSeedanceModels lists the models the gateway offers. Add a row, and a channel price, to make
// another upstream model callable.
var zexiSeedanceModels = []jijiuModelSpec{
	// Seedance 2.5.
	zexiSeedance25("doubao-seedance-2.5-nsfw-95"),
	zexiSeedance25("doubao-seedance-2.5-nsfw"),
	// Seedance 2.0.
	zexiSeedance20("doubao-seedance-2.0-nsfw-95"),
	zexiSeedance20("doubao-seedance-2.0-nsfw"),
}

// zexiModelNames lists the names of every model in the table, in table order. Account model sync
// uses it.
func zexiModelNames() []string {
	names := make([]string, 0, len(zexiSeedanceModels))
	for _, spec := range zexiSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// zexiModelSpecFor finds a model by name, ignoring case.
func zexiModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range zexiSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsZexiSeedanceModel(model string) bool {
	_, ok := zexiModelSpecFor(model)
	return ok
}

// ppVideoZexiPublicModel resolves a model name to its public name (the upstream name: this
// platform has no separate public names, so only the letter case is normalized).
func ppVideoZexiPublicModel(model string) (string, bool) {
	spec, ok := zexiModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoZexiModelAliases lists every name a model answers to, public name first.
func ppVideoZexiModelAliases(model string) []string {
	spec, ok := zexiModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func zexiRatioAllowed(ratio string) bool {
	for _, candidate := range zexiSeedanceRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeZexiSeedancePayload validates a native (v1) request and returns the upstream body.
//
// The generate_audio switch, the seed and the first / last frame are read here and removed from
// the body the shared normalizer sees, because it refuses them outright (most Seedance platforms
// have none of them). The shared normalizer then enforces the model's length, resolution and
// reference limits and the prompt; the result is rewritten to the upstream's request: duration,
// resolution, ratio, generate_audio, seed and a `content` array holding the media with their
// roles. The rules on the media follow the upstream's document: audio references need at least one
// image or video, a last frame needs a first frame, and frames cannot be combined with reference
// images, videos or audios.
func normalizeZexiSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Seedance request body is required")
	}
	for _, path := range zexiRefusedFields {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Seedance does not support %s; use images, videos, audios, first_frame and last_frame", path)
		}
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !zexiRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(zexiSeedanceRatios, ", "))
	}

	generateAudio, hasAudioSwitch, stripped, err := zexiGenerateAudio(body)
	if err != nil {
		return nil, err
	}
	seed, hasSeed, stripped, err := zexiSeed(stripped)
	if err != nil {
		return nil, err
	}
	first, last, stripped, err := apophisFrames(stripped)
	if err != nil {
		return nil, err
	}
	hasFrames := first != "" || last != ""
	if hasFrames {
		upstreamModel := strings.TrimSpace(public.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = strings.TrimSpace(public.Model)
		}
		if spec, ok := zexiModelSpecFor(upstreamModel); ok && !spec.FirstLastFrame {
			return nil, fmt.Errorf("Seedance does not support first or last frames for this model")
		}
		if first == "" {
			return nil, fmt.Errorf("Seedance last_frame requires first_frame")
		}
	}

	generic, err := normalizeSeedanceSpecPayload(stripped, public, zexiModelSpecFor, zexiReferencesFromRequest)
	if err != nil {
		return nil, err
	}
	images, _ := generic["images"].([]any)
	videos, _ := generic["videos"].([]any)
	audios, _ := generic["audios"].([]any)
	if hasFrames && len(images)+len(videos)+len(audios) > 0 {
		return nil, fmt.Errorf("Seedance first_frame and last_frame cannot be combined with reference images, videos or audios")
	}
	if len(audios) > 0 && len(images) == 0 && len(videos) == 0 {
		return nil, fmt.Errorf("Seedance reference audios require at least one reference image or video")
	}

	result := map[string]any{
		"model":    generic["model"],
		"prompt":   generic["prompt"],
		"duration": generic["seconds"],
	}
	if value, ok := generic["resolution"]; ok {
		result["resolution"] = value
	}
	if ratio != "" {
		result["ratio"] = ratio
	}
	if hasAudioSwitch {
		result["generate_audio"] = generateAudio
	}
	if hasSeed {
		result["seed"] = seed
	}

	content := make([]any, 0, len(images)+len(videos)+len(audios)+2)
	if hasFrames {
		content = append(content, zexiContentItem("image_url", "first_frame", first))
		if last != "" {
			content = append(content, zexiContentItem("image_url", "last_frame", last))
		}
		public.HasImage = true
	}
	for _, item := range images {
		content = append(content, zexiContentItem("image_url", "reference_image", fmt.Sprint(item)))
	}
	for _, item := range videos {
		content = append(content, zexiContentItem("video_url", "reference_video", fmt.Sprint(item)))
	}
	for _, item := range audios {
		content = append(content, zexiContentItem("audio_url", "reference_audio", fmt.Sprint(item)))
	}
	if len(content) > 0 {
		result["content"] = content
	}
	return result, nil
}

// zexiContentItem is one entry of the upstream's `content` array: {"type": "image_url", "role":
// "reference_image", "image_url": {"url": "..."}}.
func zexiContentItem(kind, role, mediaURL string) map[string]any {
	return map[string]any{
		"type": kind,
		"role": role,
		kind:   map[string]any{"url": mediaURL},
	}
}

// zexiGenerateAudio reads the audio switch (a JSON boolean; a string is refused, `false` is
// different from leaving it out) and returns the body without that key.
func zexiGenerateAudio(body []byte) (value bool, present bool, stripped []byte, err error) {
	stripped = body
	raw := gjson.GetBytes(body, "generate_audio")
	if !raw.Exists() || raw.Type == gjson.Null {
		return false, false, stripped, nil
	}
	if raw.Type != gjson.True && raw.Type != gjson.False {
		return false, false, nil, fmt.Errorf("Seedance generate_audio must be a boolean")
	}
	if stripped, err = sjson.DeleteBytes(stripped, "generate_audio"); err != nil {
		return false, false, nil, fmt.Errorf("Seedance generate_audio is invalid")
	}
	return raw.Type == gjson.True, true, stripped, nil
}

// zexiSeed reads the seed (-1 for random, or a whole number from 0 to 9007199254740991; 0 is a
// valid seed) and returns the body without that key.
func zexiSeed(body []byte) (value int64, present bool, stripped []byte, err error) {
	stripped = body
	raw := gjson.GetBytes(body, "seed")
	if !raw.Exists() || raw.Type == gjson.Null {
		return 0, false, stripped, nil
	}
	message := fmt.Errorf("Seedance seed must be -1 or an integer from 0 to %d", int64(zexiMaxSeed))
	if raw.Type != gjson.Number {
		return 0, false, nil, message
	}
	rational, ok := new(big.Rat).SetString(raw.Raw)
	if !ok || !rational.IsInt() || !rational.Num().IsInt64() {
		return 0, false, nil, message
	}
	seed := rational.Num().Int64()
	if seed < -1 || seed > zexiMaxSeed {
		return 0, false, nil, message
	}
	if stripped, err = sjson.DeleteBytes(stripped, "seed"); err != nil {
		return 0, false, nil, fmt.Errorf("Seedance seed is invalid")
	}
	return seed, true, stripped, nil
}

// zexiReferencesFromRequest reads the media of a native (v1) request. It accepts the aliases the
// other Seedance models of this gateway document (images / videos / audios, image_urls /
// video_urls / audio_urls, reference_*, camelCase variants, single image). Two different aliases
// of the same kind in one request are refused so an item is never counted twice.
func zexiReferencesFromRequest(body []byte) (jijiuReferences, error) {
	return xsnReferencesFromRequest(body)
}

// finalizeZexiSeedanceResponse reads the task the same way as the xsn platform: the status comes
// from the explicit status field only (queued / in_progress / completed / failed; cancelled counts
// as failed), a failed task carries its reason in `error`, and billing uses the requested seconds.
// The usage the upstream reports (its own token count, which may also be missing) takes no part in
// billing.
func finalizeZexiSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// ppVideoZexiRemoveUpstreamFields drops what the upstream's task object says about its own
// accounting: the usage, cost and price figures are the supplier's, not ours. The gateway adds
// its own video_url and usage afterwards.
func ppVideoZexiRemoveUpstreamFields(payload map[string]any) {
	if payload == nil {
		return
	}
	for _, key := range []string{"usage", "final_cost", "cost", "price"} {
		delete(payload, key)
	}
}

// fillPPVideoZexiCapabilities reports the limits of one model, read from the table the
// normalizer enforces. Frames, the audio switch and the seed are offered here.
func fillPPVideoZexiCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), zexiSeedanceRatios...)
	if capabilities.Assets != nil {
		capabilities.Assets.FirstLastFrame = ppVideoCapBool(spec.FirstLastFrame)
	}
	capabilities.GenerateAudio = ppVideoCapBool(true)
}
