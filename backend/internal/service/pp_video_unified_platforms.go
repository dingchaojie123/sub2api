package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// Phase 2 of the unified video protocol (v2): translations for Wan3.0,
// Pixverse-V6, MiniMax-H3 (CompShare implementation) and the general video model
// group. Shared helpers live here; the Guangying and doubao translations are in
// pp_video_unified.go. See docs/internal/PP_VIDEO_UNIFIED_PROTOCOL_DESIGN.md.

// ppVideoUnifiedSupport lists the optional unified fields a platform can honor.
// Anything a platform does not list is rejected with a 400 instead of being
// silently dropped.
type ppVideoUnifiedSupport struct {
	generateAudio bool
	seed          bool
	watermark     bool
	callbackURL   bool
	callbackToken bool
	metadata      bool
}

func (r *ppVideoUnifiedRequest) rejectUnsupported(support ppVideoUnifiedSupport) error {
	if r.generateAudio != nil && !support.generateAudio {
		return ppVideoUnifiedErr("generate_audio", "is not supported by this model")
	}
	if r.seed != nil && !support.seed {
		return ppVideoUnifiedErr("seed", "is not supported by this model")
	}
	if r.watermark != nil && !support.watermark {
		return ppVideoUnifiedErr("watermark", "is not supported by this model")
	}
	if r.callbackURL != "" && !support.callbackURL {
		return ppVideoUnifiedErr("callback_url", "is not supported by this model")
	}
	if r.callbackToken != "" && !support.callbackToken {
		return ppVideoUnifiedErr("callback_token", "is not supported by this model")
	}
	if len(r.metadata) > 0 && !support.metadata {
		return ppVideoUnifiedErr("metadata", "is not supported by this model")
	}
	return nil
}

func ppVideoUnifiedUnsupportedOption(option ppVideoUnifiedOption) error {
	return ppVideoUnifiedErr("options."+option.Key, "is not a supported option for this model")
}

func ppVideoUnifiedOptionBool(option ppVideoUnifiedOption) (bool, error) {
	if option.Value.Type != gjson.True && option.Value.Type != gjson.False {
		return false, ppVideoUnifiedErr("options."+option.Key, "must be a boolean")
	}
	return option.Value.Type == gjson.True, nil
}

func ppVideoUnifiedOptionString(option ppVideoUnifiedOption) (string, error) {
	if option.Value.Type != gjson.String {
		return "", ppVideoUnifiedErr("options."+option.Key, "must be a string")
	}
	return strings.TrimSpace(option.Value.String()), nil
}

// ppVideoUnifiedMedia groups assets by how a platform consumes them.
type ppVideoUnifiedMedia struct {
	firstFrame      *ppVideoUnifiedAsset
	lastFrame       *ppVideoUnifiedAsset
	referenceImages []ppVideoUnifiedAsset
	referenceVideos []ppVideoUnifiedAsset
	referenceAudios []ppVideoUnifiedAsset
}

func (m ppVideoUnifiedMedia) hasFrames() bool {
	return m.firstFrame != nil || m.lastFrame != nil
}

func (m ppVideoUnifiedMedia) referenceCount() int {
	return len(m.referenceImages) + len(m.referenceVideos) + len(m.referenceAudios)
}

func (r *ppVideoUnifiedRequest) groupAssets() ppVideoUnifiedMedia {
	var media ppVideoUnifiedMedia
	for _, asset := range r.assets {
		switch asset.Type {
		case "image":
			switch asset.Role {
			case ppVideoUnifiedAssetRoleFirstFrame:
				media.firstFrame = &asset
			case ppVideoUnifiedAssetRoleLastFrame:
				media.lastFrame = &asset
			default:
				media.referenceImages = append(media.referenceImages, asset)
			}
		case "video":
			media.referenceVideos = append(media.referenceVideos, asset)
		case "audio":
			media.referenceAudios = append(media.referenceAudios, asset)
		}
	}
	return media
}

// ppVideoUnifiedCheckLimit reports the first asset that exceeds the limit.
func ppVideoUnifiedCheckLimit(items []ppVideoUnifiedAsset, limit int, label string) error {
	if len(items) > limit {
		return ppVideoUnifiedErr(fmt.Sprintf("assets[%d]", items[limit].Index), "too many %s; this model allows at most %d", label, limit)
	}
	return nil
}

func ppVideoUnifiedURLs(items []ppVideoUnifiedAsset) []any {
	urls := make([]any, 0, len(items))
	for _, item := range items {
		urls = append(urls, item.URL)
	}
	return urls
}

// ppVideoUnifiedContentItem converts an asset into the image_url / video_url /
// audio_url content item used by the doubao and MiniMax-H3 request shapes.
func ppVideoUnifiedContentItem(asset ppVideoUnifiedAsset) map[string]any {
	switch asset.Type {
	case "video":
		return map[string]any{
			"type":      "video_url",
			"video_url": map[string]any{"url": asset.URL},
			"role":      "reference_video",
		}
	case "audio":
		return map[string]any{
			"type":      "audio_url",
			"audio_url": map[string]any{"url": asset.URL},
			"role":      "reference_audio",
		}
	}
	role := "reference_image"
	switch asset.Role {
	case ppVideoUnifiedAssetRoleFirstFrame:
		role = "first_frame"
	case ppVideoUnifiedAssetRoleLastFrame:
		role = "last_frame"
	}
	return map[string]any{
		"type":      "image_url",
		"image_url": map[string]any{"url": asset.URL},
		"role":      role,
	}
}

var errPPVideoUnifiedFramesWithReferences = ppVideoUnifiedErr("assets", "first_frame and last_frame images cannot be combined with reference assets for this model")

// toWan translates to the native Wan3.0 shape: input.prompt, input.media items
// of {type, url}, and a parameters object. Wan's file and link references are
// not part of the unified protocol; keep using the native request for them.
func (r *ppVideoUnifiedRequest) toWan() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{generateAudio: true, seed: true, watermark: true}); err != nil {
		return nil, err
	}
	if r.prompt == "" {
		return nil, ppVideoUnifiedErr("prompt", "is required for this model")
	}

	promptExtend := false
	hasPromptExtend := false
	for _, option := range r.options {
		if option.Key != "prompt_extend" {
			return nil, ppVideoUnifiedUnsupportedOption(option)
		}
		value, err := ppVideoUnifiedOptionBool(option)
		if err != nil {
			return nil, err
		}
		promptExtend, hasPromptExtend = value, true
	}

	media := r.groupAssets()
	if media.hasFrames() && media.referenceCount() > 0 {
		return nil, errPPVideoUnifiedFramesWithReferences
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceImages, 10, "reference images"); err != nil {
		return nil, err
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceVideos, 5, "reference videos"); err != nil {
		return nil, err
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceAudios, 5, "reference audios"); err != nil {
		return nil, err
	}

	items := make([]any, 0, len(r.assets))
	for _, asset := range r.assets {
		mediaType := "reference_image"
		switch asset.Type {
		case "video":
			mediaType = "reference_video"
		case "audio":
			mediaType = "reference_audio"
		default:
			switch asset.Role {
			case ppVideoUnifiedAssetRoleFirstFrame:
				mediaType = "first_frame"
			case ppVideoUnifiedAssetRoleLastFrame:
				mediaType = "last_frame"
			}
		}
		items = append(items, map[string]any{"type": mediaType, "url": asset.URL})
	}

	parameters := make(map[string]any)
	if r.duration != nil {
		parameters["duration"] = *r.duration
	}
	if r.resolution != "" {
		parameters["resolution"] = r.resolution
	}
	if r.ratio != "" {
		parameters["ratio"] = r.ratio
	}
	if r.generateAudio != nil {
		parameters["audio"] = *r.generateAudio
	}
	if r.watermark != nil {
		parameters["watermark"] = *r.watermark
	}
	if r.seed != nil {
		parameters["seed"] = *r.seed
	}
	if hasPromptExtend {
		parameters["prompt_extend"] = promptExtend
	}

	input := map[string]any{"prompt": r.prompt}
	if len(items) > 0 {
		input["media"] = items
	}
	payload := map[string]any{"input": input, "parameters": parameters}
	if r.hasModel {
		payload["model"] = r.model
	}
	return payload, nil
}

// toPixverse translates to the native Pixverse-V6 shape. The model takes a single
// reference image (img_url), a single video (video_url, a video-extension input),
// or a first/last frame pair; it has no audio input. The ratio only applies to
// text-to-video requests, so it is left out whenever media is present, which is
// what the model does with it anyway.
func (r *ppVideoUnifiedRequest) toPixverse() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{generateAudio: true, seed: true}); err != nil {
		return nil, err
	}
	if r.prompt == "" {
		return nil, ppVideoUnifiedErr("prompt", "is required for this model")
	}
	if len(r.options) > 0 {
		return nil, ppVideoUnifiedUnsupportedOption(r.options[0])
	}

	media := r.groupAssets()
	if len(media.referenceAudios) > 0 {
		return nil, ppVideoUnifiedErr(fmt.Sprintf("assets[%d].type", media.referenceAudios[0].Index), "audio assets are not supported by this model")
	}
	if (media.firstFrame == nil) != (media.lastFrame == nil) {
		return nil, ppVideoUnifiedErr("assets", "first_frame and last_frame must be provided together for this model")
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceImages, 1, "reference images"); err != nil {
		return nil, err
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceVideos, 1, "reference videos"); err != nil {
		return nil, err
	}

	input := map[string]any{"prompt": r.prompt}
	hasMedia := false
	if media.firstFrame != nil {
		input["first_frame_url"] = media.firstFrame.URL
		input["last_frame_url"] = media.lastFrame.URL
		hasMedia = true
	}
	if len(media.referenceImages) > 0 {
		input["img_url"] = media.referenceImages[0].URL
		hasMedia = true
	}
	if len(media.referenceVideos) > 0 {
		input["video_url"] = media.referenceVideos[0].URL
		hasMedia = true
	}

	parameters := make(map[string]any)
	if r.duration != nil {
		parameters["duration"] = *r.duration
	}
	if r.resolution != "" {
		parameters["resolution"] = r.resolution
	}
	if r.ratio != "" && !hasMedia {
		parameters["aspect_ratio"] = r.ratio
	}
	if r.generateAudio != nil {
		parameters["generate_audio"] = *r.generateAudio
	}
	if r.seed != nil {
		parameters["seed"] = *r.seed
	}

	payload := map[string]any{"input": input, "parameters": parameters}
	if r.hasModel {
		payload["model"] = r.model
	}
	return payload, nil
}

var ppVideoUnifiedCompShareResolutions = map[string]string{
	"480p": "480P", "768p": "768P", "1080p": "1080P", "2k": "2K", "4k": "4K",
}

// toCompShare translates to the top-level content shape of the MiniMax-H3
// (CompShare) implementation. Its native resolution is case-sensitive and
// silently falls back to 768P for unknown spellings, so the value is validated
// here and converted to the native spelling. mute_audio is the inverse of
// generate_audio.
func (r *ppVideoUnifiedRequest) toCompShare() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{generateAudio: true, watermark: true, callbackURL: true, callbackToken: true}); err != nil {
		return nil, err
	}
	if r.callbackToken != "" && r.callbackURL == "" {
		return nil, ppVideoUnifiedErr("callback_token", "requires callback_url")
	}
	if r.duration != nil && (*r.duration < 4 || *r.duration > 30) {
		return nil, ppVideoUnifiedErr("duration", "must be between 4 and 30 for this model")
	}

	resolution := ""
	if r.resolution != "" {
		native, ok := ppVideoUnifiedCompShareResolutions[strings.ToLower(r.resolution)]
		if !ok {
			return nil, ppVideoUnifiedErr("resolution", "must be one of 480p, 768p, 1080p, 2k, 4k for this model")
		}
		resolution = native
	}
	switch r.ratio {
	case "", "adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16":
	default:
		return nil, ppVideoUnifiedErr("ratio", "must be one of adaptive, 21:9, 16:9, 4:3, 1:1, 3:4, 9:16 for this model")
	}

	useContextIR := false
	hasUseContextIR := false
	skillID := ""
	for _, option := range r.options {
		switch option.Key {
		case "use_context_ir":
			value, err := ppVideoUnifiedOptionBool(option)
			if err != nil {
				return nil, err
			}
			useContextIR, hasUseContextIR = value, true
		case "skill_id":
			value, err := ppVideoUnifiedOptionString(option)
			if err != nil {
				return nil, err
			}
			skillID = value
		default:
			return nil, ppVideoUnifiedUnsupportedOption(option)
		}
	}
	if skillID != "" && !useContextIR {
		return nil, ppVideoUnifiedErr("options.skill_id", "requires options.use_context_ir to be true")
	}

	media := r.groupAssets()
	if media.hasFrames() && media.referenceCount() > 0 {
		return nil, errPPVideoUnifiedFramesWithReferences
	}
	if len(media.referenceAudios) > 0 && len(media.referenceImages)+len(media.referenceVideos) == 0 {
		return nil, ppVideoUnifiedErr(fmt.Sprintf("assets[%d]", media.referenceAudios[0].Index), "reference audio requires a reference image or reference video for this model")
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceImages, 9, "reference images"); err != nil {
		return nil, err
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceVideos, 3, "reference videos"); err != nil {
		return nil, err
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceAudios, 3, "reference audios"); err != nil {
		return nil, err
	}
	if media.referenceCount() > 12 {
		return nil, ppVideoUnifiedErr("assets", "at most 12 reference assets are allowed for this model")
	}

	content := make([]any, 0, len(r.assets)+1)
	if r.prompt != "" {
		content = append(content, map[string]any{"type": "text", "text": r.prompt})
	}
	for _, asset := range r.assets {
		content = append(content, ppVideoUnifiedContentItem(asset))
	}
	if len(content) == 0 {
		return nil, ppVideoUnifiedErr("prompt", "prompt or at least one asset is required")
	}

	payload := map[string]any{"content": content}
	if r.hasModel {
		payload["model"] = r.model
	}
	if r.duration != nil {
		payload["duration"] = *r.duration
	}
	if resolution != "" {
		payload["resolution"] = resolution
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
	}
	if r.generateAudio != nil {
		payload["mute_audio"] = !*r.generateAudio
	}
	if r.watermark != nil {
		payload["aigc_watermark"] = *r.watermark
	}
	if hasUseContextIR {
		payload["use_context_ir"] = useContextIR
	}
	if skillID != "" {
		payload["skill_id"] = skillID
	}
	if r.callbackURL != "" {
		payload["callback_url"] = r.callbackURL
	}
	if r.callbackToken != "" {
		payload["callback_token"] = r.callbackToken
	}
	return payload, nil
}

// toGeneralModels translates to the flat shape of the general video model
// group: a required model, top-level images and videos, reference audios under
// metadata.referenceAudios, and first/last frames under metadata. The platform
// applies its own per-model limits to the media counts and combinations.
func (r *ppVideoUnifiedRequest) toGeneralModels() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{generateAudio: true}); err != nil {
		return nil, err
	}
	if r.model == "" {
		return nil, ppVideoUnifiedErr("model", "is required for this model group")
	}
	if len(r.options) > 0 {
		return nil, ppVideoUnifiedUnsupportedOption(r.options[0])
	}

	media := r.groupAssets()
	payload := map[string]any{"model": r.model}
	if r.prompt != "" {
		payload["prompt"] = r.prompt
	}
	if r.duration != nil {
		payload["duration"] = *r.duration
	}
	if r.resolution != "" {
		payload["resolution"] = r.resolution
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
	}
	if r.generateAudio != nil {
		payload["generate_audio"] = *r.generateAudio
	}
	if len(media.referenceImages) > 0 {
		payload["images"] = ppVideoUnifiedURLs(media.referenceImages)
	}
	if len(media.referenceVideos) > 0 {
		payload["videos"] = ppVideoUnifiedURLs(media.referenceVideos)
	}

	metadata := make(map[string]any)
	if len(media.referenceAudios) > 0 {
		metadata["referenceAudios"] = ppVideoUnifiedURLs(media.referenceAudios)
	}
	if media.firstFrame != nil {
		metadata["firstFrame"] = media.firstFrame.URL
	}
	if media.lastFrame != nil {
		metadata["lastFrame"] = media.lastFrame.URL
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}
	return payload, nil
}

// toXingAPI translates to the flat shape of the XingAPI Seedance upstream: model,
// prompt, duration, ratio, resolution, and the reference media as three URL arrays
// (images, videos, audios) in the order they were sent. The upstream documents no
// frame, watermark, callback or named-asset fields, so those are rejected here
// instead of being passed along or dropped; accepting more later is compatible,
// tightening would not be. Asset names are accepted but not forwarded because the
// upstream has no notion of named references. The per-model limits on the number
// and mix of references are left to the upstream, which documents them per model.
func (r *ppVideoUnifiedRequest) toXingAPI() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{generateAudio: true, seed: true}); err != nil {
		return nil, err
	}
	if r.model == "" {
		return nil, ppVideoUnifiedErr("model", "is required for this model group")
	}
	if len(r.options) > 0 {
		return nil, ppVideoUnifiedUnsupportedOption(r.options[0])
	}
	for _, asset := range r.assets {
		if asset.Role == ppVideoUnifiedAssetRoleFirstFrame || asset.Role == ppVideoUnifiedAssetRoleLastFrame {
			return nil, ppVideoUnifiedErr(fmt.Sprintf("assets[%d].role", asset.Index), "%s is not supported by this model", asset.Role)
		}
	}
	if r.prompt == "" && len(r.assets) == 0 {
		return nil, ppVideoUnifiedErr("prompt", "prompt or at least one asset is required")
	}
	// Billing is per second, so the length has to be stated; without it the request would
	// only fail later with an error that does not name the field.
	if r.duration == nil {
		return nil, ppVideoUnifiedErr("duration", "is required for this model group")
	}

	media := r.groupAssets()
	payload := map[string]any{"model": r.model}
	if r.prompt != "" {
		payload["prompt"] = r.prompt
	}
	payload["duration"] = *r.duration
	if r.resolution != "" {
		payload["resolution"] = r.resolution
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
	}
	if r.generateAudio != nil {
		payload["generate_audio"] = *r.generateAudio
	}
	if r.seed != nil {
		payload["seed"] = *r.seed
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

// toCTMO translates to the generic shape the ctmo-seedance normalizer accepts: model, prompt,
// duration, ratio, resolution, the reference media as three URL arrays (images, videos, audios)
// and, for first/last-frame mode, start_frame_url and end_frame_url (the normalizer turns them
// into the upstream's input_reference and image_end). The upstream has no audio switch, seed,
// watermark or callback, so those are rejected here instead of being passed along or dropped;
// accepting more later is compatible, tightening would not. Asset names are accepted but not
// forwarded because the upstream has no notion of named references. The per-model asset limits
// (for example no reference videos on one model) are enforced by the upstream.
func (r *ppVideoUnifiedRequest) toCTMO() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{}); err != nil {
		return nil, err
	}
	if r.model == "" {
		return nil, ppVideoUnifiedErr("model", "is required for this model group")
	}
	if len(r.options) > 0 {
		return nil, ppVideoUnifiedUnsupportedOption(r.options[0])
	}
	if r.prompt == "" && len(r.assets) == 0 {
		return nil, ppVideoUnifiedErr("prompt", "prompt or at least one asset is required")
	}

	media := r.groupAssets()
	if (media.firstFrame == nil) != (media.lastFrame == nil) {
		return nil, ppVideoUnifiedErr("assets", "first_frame and last_frame must be sent together for this model")
	}
	if media.hasFrames() && media.referenceCount() > 0 {
		return nil, errPPVideoUnifiedFramesWithReferences
	}

	payload := map[string]any{"model": r.model}
	if r.prompt != "" {
		payload["prompt"] = r.prompt
	}
	if r.duration != nil {
		payload["duration"] = *r.duration
	}
	if r.resolution != "" {
		payload["resolution"] = r.resolution
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
	}
	if media.hasFrames() {
		payload["start_frame_url"] = media.firstFrame.URL
		payload["end_frame_url"] = media.lastFrame.URL
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

// toJijiu translates to the generic shape the jijiu-seedance normalizer accepts: model, prompt,
// duration, ratio, resolution and the reference media as three URL arrays (images, videos,
// audios) in the order they were sent. That normalizer then applies the per-model limits and
// writes the fields under the names each model expects. The upstream has no first/last-frame
// field, no audio switch, seed, watermark or callback, so those are rejected here instead of
// being passed along or dropped; accepting more later is compatible, tightening would not.
// Asset names are accepted but not forwarded because the upstream has no notion of named
// references. The prompt is required even when reference media is sent.
func (r *ppVideoUnifiedRequest) toJijiu() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{}); err != nil {
		return nil, err
	}
	if r.model == "" {
		return nil, ppVideoUnifiedErr("model", "is required for this model group")
	}
	if len(r.options) > 0 {
		return nil, ppVideoUnifiedUnsupportedOption(r.options[0])
	}
	for _, asset := range r.assets {
		if asset.Role == ppVideoUnifiedAssetRoleFirstFrame || asset.Role == ppVideoUnifiedAssetRoleLastFrame {
			return nil, ppVideoUnifiedErr(fmt.Sprintf("assets[%d].role", asset.Index), "%s is not supported by this model", asset.Role)
		}
	}
	if r.prompt == "" {
		return nil, ppVideoUnifiedErr("prompt", "is required for this model")
	}
	// Billing is per second or per request depending on the model, and every model has its own
	// length range, so the length has to be stated.
	if r.duration == nil {
		return nil, ppVideoUnifiedErr("duration", "is required for this model group")
	}

	media := r.groupAssets()
	payload := map[string]any{"model": r.model, "prompt": r.prompt, "duration": *r.duration}
	if r.resolution != "" {
		payload["resolution"] = r.resolution
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
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
