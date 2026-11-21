package service

// toNaiwa translates a v2 request for the naiwa-seedance platform. toJijiu's output (model, prompt,
// duration, ratio, resolution, images / videos / audios in the order they were sent) is read by
// the naiwa reader as aliases, so the rules are shared: no frames, audio switch, seed, watermark,
// callback, metadata or options, prompt and duration required. The native normalizer then enforces
// the model's limits (length, resolution - required for the two M models -, reference counts, the V
// prompt limit) and writes the upstream's fields (duration, ratio, resolution, image_urls,
// video_urls, audio_urls). Asset names are not forwarded: the upstream only takes plain URL arrays,
// so a reference by name has no effect here.
func (r *ppVideoUnifiedRequest) toNaiwa() (map[string]any, error) {
	return r.toJijiu()
}
