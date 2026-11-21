package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// miaowu-seedance is a Seedance upstream that speaks the OpenAI-style video protocol:
// POST /v1/videos, GET /v1/videos/{id} and GET /v1/videos/{id}/content. Its request fields
// are image_urls / video_urls / audio_urls, ratio and seconds; any other field is a 400 on the
// upstream, so the normalizer builds a clean payload and only sends these.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload); only the field names differ.
// The upstream lists many more models than are described here. Those can be discovered and
// added to an account, but a request for one that is not in the table is refused locally
// (HTTP 400) until its limits and its channel price are known.

// MiaowuSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the
// gateway normalizes it).
const MiaowuSeedanceDefaultBaseURL = "https://api.miaowuai.store/v1"

// miaowuSeedanceModels lists the models whose limits are known (seedance-2.5-slow / -deal / -flash / -pro and seedance-2.0-fast-deal / -fast / -916 / -deal / -tj / -pro, seedance-2.5-flash-tj / -tj / -wx, doubao-seedance-2.0-fast / -mini, and seedance-2.5-flash-s). Add a row, and a channel
// price, to make another upstream model callable.
var miaowuSeedanceModels = []jijiuModelSpec{
	{
		// 480p or 720p (720p when omitted); 4-30 seconds; 30 images, NO reference videos and 10 audios
		// (the operator's final table of 2026-11-19; it was 10 videos before, and the supplier's own
		// page still says 10). Billing is per second; the channel price has one tier per resolution
		// (480p 0.7 credits, 720p 1 credit).
		Name: "seedance-2.5-slow", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 720p only (1.2 credits per second), and reference videos are not supported (MaxVideos 0).
		Name: "seedance-2.5-deal", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 480p or 720p (720p when omitted); 4-30 seconds; 10 images, no reference videos, 5 audios.
		// Billed per request (17 credits, whatever the resolution and length), so the channel price
		// is one default price without resolution tiers.
		Name: "seedance-2.5-flash", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 10, MaxVideos: 0, MaxAudios: 5,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 720p only; 5-30 seconds (4-30 before the operator's final table of 2026-11-19); 30 images,
		// no reference videos, 10 audios. Billed per request (19 credits, whatever the length), so the
		// channel price is one default price.
		Name: "seedance-2.5-pro", MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Seedance 2.0 (added 2026-11-13). 720p only; 5-15 seconds (the operator listed "5s / 10s /
		// 15s" and confirmed any whole number from 5 to 15 is fine); 9 images, no reference videos,
		// 3 audios. Billed per request (5.6 credits, whatever the length), so the channel price is
		// one default price.
		Name: "seedance-2.0-fast-deal", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 480p or 720p (720p when omitted: my assumption, the operator did not say); 5-15 seconds;
		// 9 images, no reference videos, 3 audios. Billed per request since 2026-11-17 (4.5
		// credits, whatever the resolution and length; it was per second before), so the channel
		// price is one default price.
		Name: "seedance-2.0-fast", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 720p only; 5-15 seconds; 9 images, 3 reference videos, 3 audios. Billed per request
		// (5.6 credits, whatever the length), so the channel price is one default price.
		Name: "seedance-2.0-916", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// 480p or 720p (720p when omitted: my assumption); 5-15 seconds; 9 images, 3 reference
		// videos, 3 audios. Billed per request, with the price depending on the resolution (480p 4
		// credits, 720p 8 credits), so the channel price has one per-request tier per resolution.
		Name: "seedance-2.0-deal", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-17 after customers got `model "seedance-2.0-tj" is not supported by
		// platform "miaowu-seedance"`. Updated 2026-11-21 (the operator's request, together with
		// seedance-2.0-fast-tj and seedance-2.0-mini-tj, which have the same spec): 480p / 720p /
		// 1080p / 4k (720p when omitted), 9 images, 3 reference videos, 3 audios, billed per request
		// (one channel price, 11 credits). It was 720p only with no reference videos before. The
		// operator's text gives the length twice ("4-15秒" and "5-15秒"); 5-15 seconds is kept, as
		// for every other 2.0 model of this platform. The upstream spelling of the 4K tier is
		// ASSUMED to be `4k` (the same as seedance-2.0-pro).
		Name: "seedance-2.0-tj", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p", "1080p", "4k"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-21: the same spec as seedance-2.0-tj, billed per request (11 credits).
		Name: "seedance-2.0-fast-tj", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p", "1080p", "4k"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-21: the same spec as seedance-2.0-tj, billed per request (11 credits).
		Name: "seedance-2.0-mini-tj", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p", "1080p", "4k"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-17 (same incident). 720p, 1080p or 4k (720p when omitted: my assumption);
		// 9 images, 3 reference videos, 3 audios; billed per second, so the channel price has one
		// tier per resolution. Length 5-15 seconds is my assumption, as for seedance-2.0-tj.
		Name: "seedance-2.0-pro", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"720p", "1080p", "4k"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-18. 720p only; 4-30 seconds; 10 images, 5 reference videos, 5 audios. Billed
		// per request (15 credits, whatever the length), so the channel price is one default price.
		Name: "seedance-2.5-flash-tj", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 10, MaxVideos: 5, MaxAudios: 5,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-18. 480p or 720p (720p when omitted: my assumption, the operator did not
		// say); 4-30 seconds; 30 images, 10 reference videos, 10 audios. Billed per request, with the
		// price depending on the resolution (480p 10 credits, 720p 19 credits), so the channel price
		// has one per-request tier per resolution.
		Name: "seedance-2.5-tj", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-18: same limits and prices as seedance-2.5-tj. 480p or 720p (720p when
		// omitted: my assumption); 4-30 seconds; 30 images, 10 reference videos, 10 audios. Billed per
		// request (480p 10 credits, 720p 19 credits), so the channel price has one per-request tier
		// per resolution.
		Name: "seedance-2.5-wx", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-19. 720p only; 5-15 seconds; 9 images, no reference videos, no reference
		// audios. Billed per request (5 credits since 2026-11-20, 4.5 before; whatever the length), so the channel price is one
		// default price. The upstream id carries the doubao- prefix (the operator's spelling).
		Name: "doubao-seedance-2.0-fast", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 0,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-19. Same limits as doubao-seedance-2.0-fast (720p only; 5-15 seconds; 9
		// images, no reference videos, no reference audios). Billed per request (3.5 credits since 2026-11-20, 2.5 before;
		// whatever the length), so the channel price is one default price.
		Name: "doubao-seedance-2.0-mini", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"720p"}, ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 0,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
	{
		// Added 2026-11-19. 480p or 720p (720p when omitted: my assumption, the operator did not
		// say); 4-30 seconds; 10 images, no reference videos, 5 audios. Billed per second (480p 0.6, 720p 1 credit since 2026-11-20; it was 0.4 /
		// 0.7), so the channel price has one per-second tier per resolution.
		Name: "seedance-2.5-flash-s", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 10, MaxVideos: 0, MaxAudios: 5,
		ImageField: "image_urls", VideoField: "video_urls", AudioField: "audio_urls", RatioField: "ratio",
	},
}

// miaowuModelSpecFor finds a model by name, ignoring case.
func miaowuModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range miaowuSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsMiaowuSeedanceModel(model string) bool {
	_, ok := miaowuModelSpecFor(model)
	return ok
}

// ppVideoMiaowuPublicModel resolves a model name to its public name.
func ppVideoMiaowuPublicModel(model string) (string, bool) {
	spec, ok := miaowuModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoMiaowuModelAliases lists every name a model answers to, public name first.
func ppVideoMiaowuModelAliases(model string) []string {
	spec, ok := miaowuModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func normalizeMiaowuSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	return normalizeSeedanceSpecPayload(body, public, miaowuModelSpecFor, miaowuReferencesFromRequest)
}

// miaowuReferencesFromRequest reads the media of a native (v1) request. It accepts the
// upstream's own names (image_urls / video_urls / audio_urls) and the aliases the other
// Seedance models of this gateway document (images / videos / audios, reference_*, camelCase
// variants, single image). Two different aliases in one request are refused so a field is
// never counted twice.
func miaowuReferencesFromRequest(body []byte) (jijiuReferences, error) {
	var result jijiuReferences
	var err error
	if result.images, err = jijiuURLList(body, "image_url",
		[]string{"image_urls", "images", "reference_images", "referenceImages", "reference_image_urls"},
		[]string{"image", "image_url"}); err != nil {
		return result, err
	}
	if result.videos, err = jijiuURLList(body, "video_url",
		[]string{"video_urls", "videos", "reference_videos", "referenceVideos"},
		nil); err != nil {
		return result, err
	}
	if result.audios, err = jijiuURLList(body, "audio_url",
		[]string{"audio_urls", "audios", "reference_audios", "referenceAudios", "reference_audio_urls"},
		nil); err != nil {
		return result, err
	}
	return result, nil
}

// finalizeMiaowuSeedanceResponse takes the status from the explicit status field only (the
// upstream reports queued / in progress / completed / failed) and bills the requested
// seconds, the same way as the jijiu platform. A failed task carries error.message; a
// rejected request carries a plain string in `error`, which is read as well.
func finalizeMiaowuSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	result = finalizeJijiuSeedanceResponse(result, body)
	if result.ErrorMessage == "" {
		if value := gjson.GetBytes(body, "error"); value.Type == gjson.String {
			result.ErrorMessage = strings.TrimSpace(value.String())
		}
	}
	return result
}

// fillPPVideoMiaowuCapabilities reports the limits of one model, read from the table the
// normalizer enforces.
func fillPPVideoMiaowuCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
}
