package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// paisio-seedance is a Seedance upstream that speaks an OpenAI-style video protocol:
// POST /v1/videos answers {"task_id", "status": "pending"}, GET /v1/videos/{task_id} reports
// pending / processing / completed / failed with progress, video_url (a CDN address that needs
// no credentials and expires after hours) and error.message, and GET /v1/videos/{task_id}/content
// redirects (302) to the same file. The gateway's upstream client follows that redirect, so the
// authenticated /content endpoint is the one delivery path and the CDN address is never kept.
//
// Its request fields are duration, aspect_ratio, image_url (the first image, @Image1) plus
// extra_images (@Image2 ...), extra_videos and extra_audios. The resolution is part of the model
// name, so there is no resolution field to send. Any other field is dropped by the normalizer,
// which builds a clean payload.
//
// Models are described with the same table type as the jijiu platform (jijiuModelSpec) and go
// through the same normalizer (normalizeSeedanceSpecPayload) for prompt, duration, resolution
// and reference limits; normalizePaisioSeedancePayload then renames the result to the upstream's
// field names. The upstream lists more models than are described here. They can be discovered and
// added to an account, but a request for one that is not in the table is refused locally
// (HTTP 400) until its limits and its channel price are known.

// PaisioSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the
// gateway normalizes it).
const PaisioSeedanceDefaultBaseURL = "https://api.paisio.online"

// paisioSeedanceModels lists the models whose limits are known. Both are billed per second and
// the resolution is fixed by the model name. The 720p model does not support reference videos.
// Add a row, and a channel price, to make another upstream model callable.
var paisioSeedanceModels = []jijiuModelSpec{
	{
		Name: "paisio-seedance-2.5-480p", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
	},
	{
		Name: "paisio-seedance-2.5-720p", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
	},
}

// paisioAspectRatios are the ratios the upstream documents.
var paisioAspectRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "3:2", "2:3"}

// paisioModelSpecFor finds a model by name, ignoring case.
func paisioModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range paisioSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsPaisioSeedanceModel(model string) bool {
	_, ok := paisioModelSpecFor(model)
	return ok
}

// ppVideoPaisioPublicModel resolves a model name to its public name (the upstream name: this
// platform has no separate public names).
func ppVideoPaisioPublicModel(model string) (string, bool) {
	spec, ok := paisioModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoPaisioModelAliases lists every name a model answers to, public name first.
func ppVideoPaisioModelAliases(model string) []string {
	spec, ok := paisioModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func paisioAspectRatioAllowed(ratio string) bool {
	for _, candidate := range paisioAspectRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizePaisioSeedancePayload validates a native (v1) request and returns the upstream body.
//
// The first/last frame fields of the upstream (start_image_url, end_image_url) are refused: it
// documents them as supported by "some models" only, and neither model here is known to take
// them. Dropping them silently would mislead the caller, and accepting them later is compatible.
// The images are sent in the order the client gave them, the first as image_url and the rest as
// extra_images, so that @Image1, @Image2 ... in the prompt keep their meaning.
func normalizePaisioSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	for _, path := range []string{"start_image_url", "end_image_url"} {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Seedance does not support first or last frames")
		}
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !paisioAspectRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(paisioAspectRatios, ", "))
	}

	generic, err := normalizeSeedanceSpecPayload(body, public, paisioModelSpecFor, paisioReferencesFromRequest)
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
	if images, ok := generic["images"].([]any); ok && len(images) > 0 {
		result["image_url"] = images[0]
		if len(images) > 1 {
			result["extra_images"] = images[1:]
		}
	}
	if videos, ok := generic["videos"].([]any); ok && len(videos) > 0 {
		result["extra_videos"] = videos
	}
	if audios, ok := generic["audios"].([]any); ok && len(audios) > 0 {
		result["extra_audios"] = audios
	}
	return result, nil
}

// paisioReferencesFromRequest reads the media of a native (v1) request. It accepts the
// upstream's own shape (image_url as the first image, extra_images, extra_videos,
// extra_audios) and the aliases the other Seedance models of this gateway document (images /
// videos / audios, reference_*, camelCase variants, image_urls / video_urls / audio_urls,
// single image). Mixing the upstream's image shape with one of the aliases is refused so an
// image is never counted twice, and two aliases of the same kind are refused by jijiuURLList.
func paisioReferencesFromRequest(body []byte) (jijiuReferences, error) {
	var result jijiuReferences
	var err error

	aliased, err := jijiuURLList(body, "image_url",
		[]string{"image_urls", "images", "reference_images", "referenceImages", "reference_image_urls"},
		[]string{"image"})
	if err != nil {
		return result, err
	}
	first, err := jijiuURLList(body, "image_url", nil, []string{"image_url"})
	if err != nil {
		return result, err
	}
	extra, err := jijiuURLList(body, "image_url", []string{"extra_images"}, nil)
	if err != nil {
		return result, err
	}
	native := append(append([]string(nil), first...), extra...)
	if len(aliased) > 0 && len(native) > 0 {
		return result, fmt.Errorf("Seedance accepts only one of image_url / extra_images and the images list")
	}
	if len(aliased) > 0 {
		result.images = aliased
	} else {
		result.images = native
	}

	if result.videos, err = jijiuURLList(body, "video_url",
		[]string{"extra_videos", "video_urls", "videos", "reference_videos", "referenceVideos", "video_refs"},
		nil); err != nil {
		return result, err
	}
	if result.audios, err = jijiuURLList(body, "audio_url",
		[]string{"extra_audios", "audio_urls", "audios", "reference_audios", "referenceAudios", "reference_audio_urls", "audio_refs"},
		nil); err != nil {
		return result, err
	}
	return result, nil
}

// finalizePaisioSeedanceResponse takes the status from the explicit status field only (the
// upstream reports pending / processing / completed / failed) and bills the requested seconds,
// the same way as the jijiu platform; cost (the upstream's own charge in yuan) plays no part. A
// failed task carries error.message; a rejected request may carry a plain string in `error`,
// which is read as well.
func finalizePaisioSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	result = finalizeJijiuSeedanceResponse(result, body)
	if result.ErrorMessage == "" {
		if value := gjson.GetBytes(body, "error"); value.Type == gjson.String {
			result.ErrorMessage = strings.TrimSpace(value.String())
		}
	}
	return result
}

// fillPPVideoPaisioCapabilities reports the limits of one model, read from the table the
// normalizer enforces.
func fillPPVideoPaisioCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), paisioAspectRatios...)
}
