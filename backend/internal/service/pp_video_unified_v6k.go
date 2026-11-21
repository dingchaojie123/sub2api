package service

// toV6K translates a v2 request for the v6k-seedance platform. toJijiu's output (model, prompt,
// duration, ratio, resolution, images / videos / audios in the order they were sent) is read by
// the native normalizer, so the rules are shared: no frames, audio switch, seed, watermark,
// callback, metadata or options, prompt and duration required. The native normalizer then enforces
// the model's limits (length, resolution, ratio set, reference counts) and writes the upstream's
// fields (seconds, ratio, resolution, images, audios). Asset names are not forwarded: the upstream
// only takes plain URL arrays, so a reference by name has no effect here.
func (r *ppVideoUnifiedRequest) toV6K() (map[string]any, error) {
	return r.toJijiu()
}
