package service

import (
	"fmt"
	"strings"
)

// toMaiToken translates to the native sdtj-2.5-480p request shape: a fixed model
// and resolution, top-level duration/ratio/generate_audio/watermark, and the
// reference lists referenceImages / referenceVideos / referenceAudios plus
// first_image / last_image. The native shape has no notion of asset names, so
// names are only checked for uniqueness. Its own validators (duration range,
// reference limits, frame rules) run again on the translated body; the checks
// here use neutral wording and point at the v2 field that is wrong.
func (r *ppVideoUnifiedRequest) toMaiToken() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{generateAudio: true, watermark: true}); err != nil {
		return nil, err
	}
	if len(r.options) > 0 {
		return nil, ppVideoUnifiedUnsupportedOption(r.options[0])
	}
	if r.model == "" {
		return nil, ppVideoUnifiedErr("model", "is required for this model")
	}
	if _, ok := ppVideoMaiTokenPublicModel(r.model); !ok {
		return nil, ppVideoUnifiedErr("model", "must be %s for this model", MaiTokenSeedancePublicModel)
	}
	if r.prompt == "" {
		return nil, ppVideoUnifiedErr("prompt", "is required for this model")
	}
	if r.duration != nil && (*r.duration < 4 || *r.duration > 30) {
		return nil, ppVideoUnifiedErr("duration", "must be between 4 and 30 for this model")
	}
	if r.resolution != "" && !strings.EqualFold(r.resolution, VideoBillingResolution480P) {
		return nil, ppVideoUnifiedErr("resolution", "must be 480p for this model")
	}

	for _, asset := range r.assets {
		if err := validateMaiTokenHTTPURL("url", asset.URL); err != nil {
			return nil, ppVideoUnifiedErr(fmt.Sprintf("assets[%d].url", asset.Index), "must be a public HTTP(S) URL")
		}
	}
	media := r.groupAssets()
	if media.lastFrame != nil && media.firstFrame == nil {
		return nil, ppVideoUnifiedErr(fmt.Sprintf("assets[%d].role", media.lastFrame.Index), "last_frame requires a first_frame for this model")
	}
	if media.hasFrames() && media.referenceCount() > 0 {
		return nil, errPPVideoUnifiedFramesWithReferences
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceImages, maiTokenMaxReferenceImages, "reference images"); err != nil {
		return nil, err
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceVideos, maiTokenMaxReferenceVideos, "reference videos"); err != nil {
		return nil, err
	}
	if err := ppVideoUnifiedCheckLimit(media.referenceAudios, maiTokenMaxReferenceAudios, "reference audios"); err != nil {
		return nil, err
	}

	payload := map[string]any{"model": MaiTokenSeedancePublicModel, "prompt": r.prompt}
	if r.duration != nil {
		payload["duration"] = *r.duration
	}
	if r.resolution != "" {
		payload["resolution"] = VideoBillingResolution480P
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
	}
	if len(media.referenceImages) > 0 {
		payload["referenceImages"] = ppVideoUnifiedURLs(media.referenceImages)
	}
	if len(media.referenceVideos) > 0 {
		payload["referenceVideos"] = ppVideoUnifiedURLs(media.referenceVideos)
	}
	if len(media.referenceAudios) > 0 {
		payload["referenceAudios"] = ppVideoUnifiedURLs(media.referenceAudios)
	}
	if media.firstFrame != nil {
		payload["first_image"] = media.firstFrame.URL
	}
	if media.lastFrame != nil {
		payload["last_image"] = media.lastFrame.URL
	}
	if r.generateAudio != nil {
		payload["generate_audio"] = *r.generateAudio
	}
	if r.watermark != nil {
		payload["watermark"] = *r.watermark
	}
	return payload, nil
}
