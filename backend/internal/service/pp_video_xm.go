package service

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// xm-seedance is a Seedance upstream (xm API, ai.987xm.com) that speaks the OpenAI-style video
// protocol with one unified request shape for all of its video models: POST /v1/videos answers a
// task object with an `id`, GET /v1/videos/{id} reports queued / in_progress / completed / failed /
// unknown (unknown is "result not confirmed yet", neither success nor failure), and
// GET /v1/videos/{id}/content delivers the result: it answers either JSON with a download `url` or
// the video file itself, and 410 once the link has expired. Every call carries the upstream key as
// a Bearer credential; the download address of the JSON answer must never receive it. The request
// fields are model, prompt, duration (whole seconds), resolution (lower case), ratio, enable_audio
// (boolean, default true) and `media`, an array of {"type": reference_image | reference_video |
// reference_audio | first_frame | last_frame, "url": <public https address>}. A first / last frame
// pair cannot be mixed with reference media and a last frame needs a first frame. The upstream
// refuses its older aliases when they are mixed with the unified fields, so only the unified ones
// are ever sent.
//
// The gateway keeps its own request shape (images / videos / audios / first_frame / last_frame,
// the same as the other Seedance platforms) and builds the upstream's `media` array from it. The
// upstream's own `media` / `content` arrays are not offered; a request that carries one of them is
// refused instead of having it dropped. The upstream has no seed.
//
// Three models are offered (the operator's decision, 2026-11-17), described with the same table
// type as the jijiu platform (jijiuModelSpec) and run through the same normalizer
// (normalizeSeedanceSpecPayload):
//
//   - seedance_2.5 ("稳定渠道-满参-sd2.5 ·真人脸"): 720p, 30 images / 0 videos / 10 audios, billed
//     per request (19 credits; it was per second with three resolutions until 2026-11-18);
//   - bd-VIP-SD2.5满参真人脸-720P-1 ("sd2.5-01-720"): 720p, 20 images / 5 videos / 5 audios, billed
//     per request (19 credits);
//   - SD2.5-M-满参支持人脸 ("SD2.5-M-满参"): 720p, 30 images / 10 videos / 10 audios, billed per
//     request (19 credits).
//
// A fourth model was added on 2026-11-19 (the operator's request): seedance_2.0, public name
// Seedance2.0-XM, billed per second (480p 0.5 / 720p 0.8 / 1080p 1.2 credits since 2026-11-20; it was 0.3 / 0.4 / 0.7), 5-15 seconds,
// 9 images / 0 videos / 3 audios. A fifth model, seedance_2.5-flash (public name
// Seedance2.5-XM-flash), followed the same day: per second (480p 0.6 / 720p 1 credit since 2026-11-20; it was 0.4 / 0.7), 480p or
// 720p, 4-30 seconds, 10 images / 0 videos / 5 audios.
// A sixth model, SD2.5-X-满参支持人脸 (public name Seedance2.5-XM-X-full), was added on 2026-11-24:
// per request (19 credits), 720p, 5-30 seconds, 30 images / 10 videos / 10 audios.
// The mapping of the operator's descriptions to the upstream ids follows the order in which the
// operator gave them (the three ids were given in the same order as the three descriptions, and
// the 720P in the second id agrees with the 720p-only second description); correct a row here if
// it is wrong. The prices live in the channel, not in code. The 4-30 second length is the one the
// upstream's document gives for Seedance 2.5 (the operator did not give a length per model).
// ASSUMED, not read from the upstream: that all three models take that length, and that all three
// take first / last frames (the document says so for its Seedance models in general). The upstream
// refuses a combination it does not support with an explicit error (400, error.param names the
// field), so a row that is too wide fails at the upstream and the hold is released.
//
// The upstream's document says reference videos are charged by their length on top of the
// generated seconds for the models it describes there. This gateway bills the per-request models
// at one flat price whatever the references are (the operator's decision), so a clip that carries
// long reference videos may cost more upstream than it earns here.
//
// Which models a key can call is decided by the key's group on the upstream, so each upstream
// group gets its own account and the account's model whitelist holds only the models that key can
// use, the same way as the jijiu, vshi and zexi platforms.
//
// Public names (same reason as fengyue, hongpei and huahuo): two of the upstream ids carry no
// "seedance" and contain Chinese text, so clients that pick video models by name would not list
// them. Clients see Seedance2.5-XM-stable-face / -720p-vip / -M-full (and Seedance2.0-XM). The upstream id is the only
// one ever sent upstream; requests may use either name, and the model list, the task record and
// the response show the public one. Channel prices are read under the public name first, then
// the upstream id.
//
// Delivery: the public result address is the gateway's own /v1/videos/{id}/content, which reads
// the upstream's authenticated /content endpoint and, when that answers JSON, follows the `url`
// in it without any credential (followXMContentResponse). The upstream's own address is never
// stored or shown to a client.

// XMSeedanceDefaultBaseURL is the default upstream address (with or without /v1 the gateway
// normalizes it). It is the site the upstream's integration document was saved from; an
// account's own base_url takes precedence.
const XMSeedanceDefaultBaseURL = "https://ai.987xm.com"

// xmContentJSONLimit bounds the JSON answer of the upstream's /content endpoint that is read to
// find the download address.
const xmContentJSONLimit = 64 * 1024

// xmSeedanceRatios are the ratios the upstream documents for these models.
var xmSeedanceRatios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}

// xmRefusedFields are the upstream's own spellings this gateway does not offer. They are refused
// with a message instead of being ignored: ignoring `media` would drop the media.
var xmRefusedFields = []string{"media", "content", "input_reference", "input_images", "input_videos", "input_audios"}

// xmSeedanceModels lists the models the gateway offers. Add a row, and a channel price, to make
// another upstream model callable.
var xmSeedanceModels = []jijiuModelSpec{
	{
		// 稳定渠道 满参 真人脸, per request (changed from per second on 2026-11-18, and from 480p /
		// 720p / 1080p to 720p only, by the operator). No reference videos.
		Name: "seedance_2.5", PublicName: "Seedance2.5-XM-stable-face", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 0, MaxAudios: 10, FirstLastFrame: true,
	},
	{
		// sd2.5-01-720, per request.
		Name: "bd-VIP-SD2.5满参真人脸-720P-1", PublicName: "Seedance2.5-XM-720p-vip", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 20, MaxVideos: 5, MaxAudios: 5, FirstLastFrame: true, NoAudioSwitch: true,
	},
	{
		// SD2.5-M-满参, per request.
		Name: "SD2.5-M-满参支持人脸", PublicName: "Seedance2.5-XM-M-full", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10, FirstLastFrame: true, NoAudioSwitch: true,
	},
	{
		// seedance_2.0, per second (480p 0.5 / 720p 0.8 / 1080p 1.2 credits since 2026-11-20, 0.3 / 0.4 / 0.7 before; added 2026-11-19 by the
		// operator): 5-15 seconds, 9 images / no videos / 3 audios. The default resolution (720p when
		// none is named) is MY assumption. First / last frames and the audio switch are ASSUMED to be
		// offered as on the other rows (the operator did not say); the upstream refuses what it does
		// not support, so a wrong guess fails there and the hold is released.
		Name: "seedance_2.0", PublicName: "Seedance2.0-XM", MinSeconds: 5, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p", "1080p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 0, MaxAudios: 3, FirstLastFrame: true,
	},
	{
		// seedance_2.5-flash, per second (480p 0.6 / 720p 1 credit since 2026-11-20, 0.4 / 0.7 before; added 2026-11-19 by the
		// operator): 480p or 720p, 4-30 seconds, 10 images / 0 videos / 5 audios. The default
		// resolution (720p when none is named) is MY assumption, and so are first / last frames and
		// the audio switch (offered as on seedance_2.5; add NoAudioSwitch if the upstream refuses
		// enable_audio for this model). The public name is mine too.
		Name: "seedance_2.5-flash", PublicName: "Seedance2.5-XM-flash", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 10, MaxVideos: 0, MaxAudios: 5, FirstLastFrame: true,
	},
	{
		// SD2.5-X-满参支持人脸 (added 2026-11-24 by the operator), per request (19 credits): 720p,
		// 5-30 seconds (this row starts at 5, the other 2.5 rows at 4), 30 images / 10 videos / 10
		// audios. The public name and the NoAudioSwitch flag are MY choices: the id carries Chinese text
		// and no "seedance", so clients get Seedance2.5-XM-X-full; and the sibling SD2.5-M-满参支持人脸
		// (same description) answers HTTP 400 to enable_audio, so the switch is left out here too
		// (true -> omitted, false -> local 400). Drop NoAudioSwitch if the upstream turns out to accept
		// it. First / last frames are ASSUMED to be offered as on the other rows.
		Name: "SD2.5-X-满参支持人脸", PublicName: "Seedance2.5-XM-X-full", MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"720p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 10, MaxAudios: 10, FirstLastFrame: true, NoAudioSwitch: true,
	},
}

// xmModelNames lists the public names of every model in the table, in table order. Account model
// sync uses it, so the whitelist an administrator saves holds the names clients see.
func xmModelNames() []string {
	names := make([]string, 0, len(xmSeedanceModels))
	for _, spec := range xmSeedanceModels {
		names = append(names, spec.publicName())
	}
	return names
}

// xmModelSpecFor finds a model by either its upstream name or its public name, ignoring case.
func xmModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range xmSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsXMSeedanceModel(model string) bool {
	_, ok := xmModelSpecFor(model)
	return ok
}

// ppVideoXMPublicModel resolves either accepted name of a model to its public name.
func ppVideoXMPublicModel(model string) (string, bool) {
	spec, ok := xmModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoXMPublicModels maps a list of model names to the public names of the models that belong
// to this platform, without duplicates, so a whitelist saved under the upstream name is listed
// under the public one. Names that are not a model of this platform are dropped.
func ppVideoXMPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoXMPublicModel(model)
		if !ok {
			continue
		}
		if _, exists := seen[publicModel]; exists {
			continue
		}
		seen[publicModel] = struct{}{}
		filtered = append(filtered, publicModel)
	}
	return filtered
}

// ppVideoXMModelAliases lists every name a model answers to, public name first.
func ppVideoXMModelAliases(model string) []string {
	spec, ok := xmModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

func xmRatioAllowed(ratio string) bool {
	for _, candidate := range xmSeedanceRatios {
		if candidate == ratio {
			return true
		}
	}
	return false
}

// normalizeXMSeedancePayload validates a native (v1) request and returns the upstream body.
//
// The audio switch and the first / last frame are read here and removed from the body the shared
// normalizer sees, because it refuses them outright (most Seedance platforms have neither). The
// shared normalizer then enforces the model's length, resolution and reference limits and the
// prompt; the result is rewritten to the upstream's request: duration, resolution, ratio,
// enable_audio and a `media` array holding the media with their types. The rules on the media
// follow the upstream's document: a last frame needs a first frame, and frames cannot be combined
// with reference images, videos or audios.
func normalizeXMSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Seedance request body is required")
	}
	for _, path := range xmRefusedFields {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return nil, fmt.Errorf("Seedance does not support %s; use images, videos, audios, first_frame and last_frame", path)
		}
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}
	if ratio != "" && !xmRatioAllowed(ratio) {
		return nil, fmt.Errorf("Seedance ratio must be one of %s", strings.Join(xmSeedanceRatios, ", "))
	}

	enableAudio, hasAudioSwitch, stripped, err := xmAudioSwitch(body)
	if err != nil {
		return nil, err
	}
	upstreamModel := strings.TrimSpace(public.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(public.Model)
	}
	modelSpec, knownModel := xmModelSpecFor(upstreamModel)
	if hasAudioSwitch && knownModel && modelSpec.NoAudioSwitch {
		// The upstream answers HTTP 400 ("Remove this unsupported field") when enable_audio is sent
		// to this model. The upstream's default is audio on, which is what `true` asks for, so it is
		// left out; `false` cannot be honored and is refused here.
		if !enableAudio {
			return nil, fmt.Errorf("Seedance does not support turning the audio off for this model")
		}
		hasAudioSwitch = false
	}
	first, last, stripped, err := apophisFrames(stripped)
	if err != nil {
		return nil, err
	}
	hasFrames := first != "" || last != ""
	if hasFrames {
		if knownModel && !modelSpec.FirstLastFrame {
			return nil, fmt.Errorf("Seedance does not support first or last frames for this model")
		}
		if first == "" {
			return nil, fmt.Errorf("Seedance last_frame requires first_frame")
		}
	}

	generic, err := normalizeSeedanceSpecPayload(stripped, public, xmModelSpecFor, xmReferencesFromRequest)
	if err != nil {
		return nil, err
	}
	images, _ := generic["images"].([]any)
	videos, _ := generic["videos"].([]any)
	audios, _ := generic["audios"].([]any)
	if hasFrames && len(images)+len(videos)+len(audios) > 0 {
		return nil, fmt.Errorf("Seedance first_frame and last_frame cannot be combined with reference images, videos or audios")
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
		result["enable_audio"] = enableAudio
	}

	media := make([]any, 0, len(images)+len(videos)+len(audios)+2)
	if hasFrames {
		media = append(media, xmMediaItem("first_frame", first))
		if last != "" {
			media = append(media, xmMediaItem("last_frame", last))
		}
		public.HasImage = true
	}
	for _, item := range images {
		media = append(media, xmMediaItem("reference_image", fmt.Sprint(item)))
	}
	for _, item := range videos {
		media = append(media, xmMediaItem("reference_video", fmt.Sprint(item)))
	}
	for _, item := range audios {
		media = append(media, xmMediaItem("reference_audio", fmt.Sprint(item)))
	}
	if len(media) > 0 {
		result["media"] = media
	}
	return result, nil
}

// xmMediaItem is one entry of the upstream's `media` array: {"type": "reference_image", "url": "..."}.
func xmMediaItem(kind, mediaURL string) map[string]any {
	return map[string]any{"type": kind, "url": mediaURL}
}

// xmAudioSwitch reads the audio switch and returns the body without it. The gateway's own name is
// generate_audio; the upstream's name enable_audio is accepted as well. Both must be JSON booleans
// (a string is refused), `false` is different from leaving it out, and giving both is refused so
// the two can never disagree.
func xmAudioSwitch(body []byte) (value bool, present bool, stripped []byte, err error) {
	stripped = body
	var found gjson.Result
	foundKey := ""
	for _, key := range []string{"generate_audio", "enable_audio"} {
		raw := gjson.GetBytes(body, key)
		if !raw.Exists() || raw.Type == gjson.Null {
			continue
		}
		if foundKey != "" {
			return false, false, nil, fmt.Errorf("Seedance accepts either generate_audio or enable_audio, not both")
		}
		if raw.Type != gjson.True && raw.Type != gjson.False {
			return false, false, nil, fmt.Errorf("Seedance %s must be a boolean", key)
		}
		found, foundKey = raw, key
	}
	if foundKey == "" {
		return false, false, stripped, nil
	}
	for _, key := range []string{"generate_audio", "enable_audio"} {
		if stripped, err = sjson.DeleteBytes(stripped, key); err != nil {
			return false, false, nil, fmt.Errorf("Seedance %s is invalid", key)
		}
	}
	return found.Type == gjson.True, true, stripped, nil
}

// xmReferencesFromRequest reads the media of a native (v1) request. It accepts the aliases the
// other Seedance models of this gateway document (images / videos / audios, image_urls /
// video_urls / audio_urls, reference_*, camelCase variants, single image). Two different aliases
// of the same kind in one request are refused so an item is never counted twice.
func xmReferencesFromRequest(body []byte) (jijiuReferences, error) {
	return xsnReferencesFromRequest(body)
}

// finalizeXMSeedanceResponse reads the task the same way as the xsn platform: the status comes
// from the explicit status field only (queued / in_progress / completed / failed; unknown and any
// other value count as still processing, because the upstream says an unknown result is neither
// success nor failure), a failed task carries its reason in `error`, and billing uses the
// requested seconds.
func finalizeXMSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	return finalizeXSNSeedanceResponse(result, body)
}

// ppVideoXMRemoveUpstreamFields drops what the upstream's task object says about its own
// accounting: the usage, cost and price figures are the supplier's, not ours. The gateway adds
// its own video_url and usage afterwards.
func ppVideoXMRemoveUpstreamFields(payload map[string]any) {
	if payload == nil {
		return
	}
	for _, key := range []string{"usage", "final_cost", "cost", "price", "credit_state", "credit_amount"} {
		delete(payload, key)
	}
}

// fillPPVideoXMCapabilities reports the limits of one model, read from the table the normalizer
// enforces. Frames and the audio switch are offered here; the seed is not.
func fillPPVideoXMCapabilities(capabilities *PPVideoModelCapabilities, spec jijiuModelSpec) {
	fillPPVideoJijiuCapabilities(capabilities, spec)
	capabilities.Ratios = append([]string(nil), xmSeedanceRatios...)
	if capabilities.Assets != nil {
		capabilities.Assets.FirstLastFrame = ppVideoCapBool(spec.FirstLastFrame)
	}
	capabilities.GenerateAudio = ppVideoCapBool(!spec.NoAudioSwitch)
}

// followXMContentResponse turns the answer of the upstream's authenticated /content endpoint into
// the video stream the gateway returns. The upstream answers either the file itself (video/* or
// application/octet-stream, returned unchanged) or JSON with a download `url`; that address is
// then fetched without any credential, through the account's proxy first and directly as the last
// resort (the same order as the other delivery fallbacks). A non-success status (410 for an
// expired link included) is returned unchanged so the caller can report it.
func (s *OpenAIGatewayService) followXMContentResponse(ctx context.Context, account *Account, proxyURL string, resp *http.Response, requestErr error, rangeHeader, ifRangeHeader string) (*http.Response, error) {
	if requestErr != nil {
		return nil, requestErr
	}
	if resp == nil {
		return nil, fmt.Errorf("PP video content request returned no response")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp, nil
	}
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if !strings.EqualFold(mediaType, "application/json") {
		return resp, nil
	}
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, xmContentJSONLimit))
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read PP video content answer: %w", readErr)
	}
	downloadURL := xmContentDownloadURL(raw)
	if downloadURL == "" {
		return nil, fmt.Errorf("video url is not available for this task")
	}
	var lastResp *http.Response
	var lastErr error
	for _, deliveryProxy := range ppVideoDeliveryProxies(proxyURL) {
		external, err := s.forwardPPVideoExternalContent(ctx, account, deliveryProxy, downloadURL, rangeHeader, ifRangeHeader)
		if err == nil && external != nil && external.StatusCode >= http.StatusOK && external.StatusCode < http.StatusMultipleChoices {
			if lastResp != nil && lastResp.Body != nil {
				_ = lastResp.Body.Close()
			}
			return external, nil
		}
		if lastResp != nil && lastResp.Body != nil {
			_ = lastResp.Body.Close()
		}
		lastResp, lastErr = external, err
	}
	if lastResp != nil {
		return lastResp, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("video download failed")
}

// xmContentDownloadURL reads the download address of the upstream's JSON /content answer. Only a
// plain https address without credentials in it is accepted.
func xmContentDownloadURL(raw []byte) string {
	address := strings.TrimSpace(gjson.GetBytes(raw, "url").String())
	if address == "" {
		return ""
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return ""
	}
	return address
}
