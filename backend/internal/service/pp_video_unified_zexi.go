package service

// toZexi translates a v2 request for the zexi-seedance platform. It is toJijiu's shape plus what
// the upstream takes beyond it: the first / last frame (as first_frame / last_frame, a last frame
// needs a first one, and frames cannot be combined with reference assets), the audio switch
// (generate_audio) and the seed (a whole number from 0; "random" is simply leaving it out, because
// the v2 protocol reads seed as a non-negative integer). The native normalizer then enforces the
// model's limits and writes the upstream's request, with the media as a `content` array that
// carries each item's role. Watermark, callbacks, metadata and options are rejected: the upstream
// has none, and accepting more later is compatible whereas tightening is not. Asset names are not
// forwarded: the upstream only knows the position (@image1, @video1, @audio1), so a reference by
// name has no effect here. prompt and duration are required because every model has its own length
// range and billing is per second.
func (r *ppVideoUnifiedRequest) toZexi() (map[string]any, error) {
	if err := r.rejectUnsupported(ppVideoUnifiedSupport{generateAudio: true, seed: true}); err != nil {
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
		return nil, ppVideoUnifiedErr("duration", "is required for this model group")
	}

	media := r.groupAssets()
	if media.lastFrame != nil && media.firstFrame == nil {
		return nil, ppVideoUnifiedErr("assets", "last_frame requires a first_frame for this model")
	}
	if media.hasFrames() && media.referenceCount() > 0 {
		return nil, errPPVideoUnifiedFramesWithReferences
	}

	payload := map[string]any{"model": r.model, "prompt": r.prompt, "duration": *r.duration}
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
