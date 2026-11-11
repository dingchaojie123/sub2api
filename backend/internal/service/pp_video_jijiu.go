package service

import (
	"fmt"
	"math/big"
	"net"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
)

// jijiu-seedance is a Seedance upstream built on a new-api site. It speaks the OpenAI-style
// video protocol: POST /v1/videos, GET /v1/videos/{id}, GET /v1/videos/{id}/content (which
// may answer 302 to an object store, followed by the HTTP client). Every model is selected by
// its full Chinese name, has its own limits, and a few take differently named reference
// fields, so the per-model table below is the single source of truth for the request
// normalizer, the model sync filter, the capabilities list and the price quote.

// jijiuMaxImagesWithoutLongerMinimum is the largest number of images a request may carry
// before the "30-10" line raises its minimum length (the upstream documents more than 9).
const jijiuMaxImagesWithoutLongerMinimum = 9

// jijiuModelSpec describes one upstream model. Resolutions[0] is only used when
// DefaultResolution is empty.
type jijiuModelSpec struct {
	// Name is the name the upstream knows and the only one ever sent to it. PublicName, when
	// set, is the name shown to clients (model list, responses, price lookup, documentation);
	// requests may use either, so accounts and integrations built against the upstream name
	// keep working.
	Name               string
	PublicName         string
	MinSeconds         int
	MaxSeconds         int
	MinSecondsWithRefs int // when > 0: minimum once any video/audio, or more than 9 images, is sent
	Resolutions        []string
	DefaultResolution  string
	ForwardResolution  bool // the upstream documents resolution as a field to send for this model
	MaxImages          int
	MaxVideos          int // 0: reference videos are not supported
	MaxAudios          int
	MaxReferenceTotal  int // 0: no combined limit
	VideoField         string
	AudioField         string
}

func (s jijiuModelSpec) publicName() string {
	if s.PublicName != "" {
		return s.PublicName
	}
	return s.Name
}

func (s jijiuModelSpec) defaultResolution() string {
	if s.DefaultResolution != "" {
		return s.DefaultResolution
	}
	return s.Resolutions[0]
}

func (s jijiuModelSpec) videoField() string {
	if s.VideoField != "" {
		return s.VideoField
	}
	return "videos"
}

func (s jijiuModelSpec) audioField() string {
	if s.AudioField != "" {
		return s.AudioField
	}
	return "audios"
}

func (s jijiuModelSpec) allowsResolution(resolution string) bool {
	for _, candidate := range s.Resolutions {
		if candidate == resolution {
			return true
		}
	}
	return false
}

// jijiuSeedanceModels lists the nine Seedance models of the upstream, taken from its
// integration document. Out-of-range durations are rejected here instead of relying on the
// upstream's silent clamping, so the seconds we bill are the seconds it produces.
var jijiuSeedanceModels = []jijiuModelSpec{
	{
		Name: "满血seedance-2.5全参A", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
	},
	{
		Name: "满血seedance-2.5全参C", MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"1080p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
		VideoField: "video_refs", AudioField: "audio_refs",
	},
	{
		// The upstream accepts up to 10 reference videos here but bills a higher tier ("带参考视频按
		// 含视频档计费") that this platform has no price for. Reference videos are refused until a
		// separate price exists, because charging the plain per-second price would undercharge;
		// allowing them later is compatible, so loosen MaxVideos together with that price.
		Name: "稳定seedance-2.5全参D", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p", "720p", "1080p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
	},
	{
		Name: "稳定seedance-2.5参图参音F", MinSeconds: 4, MaxSeconds: 30,
		Resolutions: []string{"480p"}, ForwardResolution: true, MaxImages: 30, MaxVideos: 0, MaxAudios: 10,
	},
	{
		Name: "SD2.0满血稳定933-线路一", MinSeconds: 4, MaxSeconds: 15,
		Resolutions: []string{"480p", "720p", "1080p"}, DefaultResolution: "720p", ForwardResolution: true,
		MaxImages: 9, MaxVideos: 3, MaxAudios: 3,
	},
	{
		Name: "SD2.0特价稳定903z", MinSeconds: 10, MaxSeconds: 15,
		Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3,
	},
	{
		Name: "SD2.0mini特价稳定903z", MinSeconds: 4, MaxSeconds: 15,
		Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3,
	},
	{
		Name: "SD2.5特价301010线路一", PublicName: "Seedance2.5按次收费J2", MinSeconds: 5, MaxSeconds: 30,
		Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10,
	},
	{
		Name: "SD2.5特价30-10线路二", PublicName: "Seedance2.5 按次收费J1", MinSeconds: 4, MaxSeconds: 30, MinSecondsWithRefs: 5,
		Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10, MaxReferenceTotal: 30,
	},
}

// jijiuModelSpecFor finds a model by either its upstream name or its public name, ignoring case.
func jijiuModelSpecFor(model string) (jijiuModelSpec, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return jijiuModelSpec{}, false
	}
	for _, spec := range jijiuSeedanceModels {
		if strings.EqualFold(spec.Name, model) || strings.EqualFold(spec.publicName(), model) {
			return spec, true
		}
	}
	return jijiuModelSpec{}, false
}

func ppVideoIsJijiuSeedanceModel(model string) bool {
	_, ok := jijiuModelSpecFor(model)
	return ok
}

// ppVideoJijiuPublicModel resolves either accepted name of a model to its public name.
func ppVideoJijiuPublicModel(model string) (string, bool) {
	spec, ok := jijiuModelSpecFor(model)
	if !ok {
		return "", false
	}
	return spec.publicName(), true
}

// ppVideoJijiuModelAliases lists every name a model answers to, public name first.
func ppVideoJijiuModelAliases(model string) []string {
	spec, ok := jijiuModelSpecFor(model)
	if !ok {
		return nil
	}
	if spec.publicName() == spec.Name {
		return []string{spec.Name}
	}
	return []string{spec.publicName(), spec.Name}
}

// ppVideoJijiuPublicModels maps a list of model names to the public names of the models that
// belong to this platform, without duplicates. The upstream lists every model of its site, so
// names that are not one of the Seedance models are dropped.
func ppVideoJijiuPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoJijiuPublicModel(model)
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

type jijiuReferences struct {
	images []string
	videos []string
	audios []string
}

func (r jijiuReferences) count() int {
	return len(r.images) + len(r.videos) + len(r.audios)
}

func normalizeJijiuSeedancePayload(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Seedance request body is required")
	}
	if !public.HasExplicitModel {
		return nil, fmt.Errorf("Seedance model is required")
	}
	upstreamModel := strings.TrimSpace(public.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(public.Model)
	}
	spec, ok := jijiuModelSpecFor(upstreamModel)
	if !ok {
		return nil, fmt.Errorf("model %q is not available", public.Model)
	}
	if err := jijiuRejectUnsupportedFields(body); err != nil {
		return nil, err
	}

	references, err := jijiuReferencesFromRequest(body)
	if err != nil {
		return nil, err
	}
	if err := jijiuCheckReferenceLimits(spec, references); err != nil {
		return nil, err
	}

	prompt, err := jijiuPrompt(body)
	if err != nil {
		return nil, err
	}
	seconds, err := jijiuDurationSeconds(body, spec, references)
	if err != nil {
		return nil, err
	}
	resolution, err := jijiuResolution(body, spec)
	if err != nil {
		return nil, err
	}
	ratio, err := jijiuOptionalString(body, "ratio", "aspect_ratio")
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"model":   spec.Name,
		"prompt":  prompt,
		"seconds": seconds,
	}
	if ratio != "" {
		result["aspect_ratio"] = ratio
	}
	if spec.ForwardResolution {
		result["resolution"] = resolution
	}
	if len(references.images) > 0 {
		result["images"] = jijiuStringItems(references.images)
	}
	if len(references.videos) > 0 {
		result[spec.videoField()] = jijiuStringItems(references.videos)
	}
	if len(references.audios) > 0 {
		result[spec.audioField()] = jijiuStringItems(references.audios)
	}

	if requested, ok := jijiuModelSpecFor(public.Model); ok {
		// What the client asked for is shown under its public name, even when an account
		// mapping serves it from another upstream line.
		public.Model = requested.publicName()
	}
	public.UpstreamModel = spec.Name
	public.Prompt = prompt
	public.DurationMilliseconds = int64(seconds) * 1000
	public.VideoCount = 1
	public.Resolution = resolution
	public.HasImage = len(references.images) > 0
	return result, nil
}

func jijiuStringItems(values []string) []any {
	items := make([]any, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

// jijiuRejectUnsupportedFields refuses fields the upstream does not take. Its document lists
// no first/last-frame field and tells callers not to send negative_prompt, quality, fps, n or
// output_format; dropping them silently would mislead the caller, and accepting more later is
// compatible whereas tightening is not.
func jijiuRejectUnsupportedFields(body []byte) error {
	for _, path := range []string{
		"first_frame", "last_frame", "first_image", "last_image", "first_frame_url", "last_frame_url",
		"start_frame_url", "end_frame_url", "input_reference", "image_end",
	} {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return fmt.Errorf("Seedance does not support first or last frames")
		}
	}
	for _, path := range []string{
		"seed", "draft", "watermark", "generate_audio", "return_last_frame",
		"callback", "callback_url", "callback_token",
		"negative_prompt", "quality", "fps", "output_format",
	} {
		if value := gjson.GetBytes(body, path); value.Exists() && value.Type != gjson.Null {
			return fmt.Errorf("Seedance does not support %s", path)
		}
	}
	if value := gjson.GetBytes(body, "assets"); value.Exists() && value.Type != gjson.Null {
		return fmt.Errorf("Seedance does not accept assets on this endpoint; use images, videos and audios, or the unified v2 endpoint")
	}
	for _, path := range []string{"n", "count", "video_count"} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.Number || value.Float() != 1 {
			return fmt.Errorf("Seedance supports one video per request")
		}
	}
	return nil
}

func jijiuCheckReferenceLimits(spec jijiuModelSpec, references jijiuReferences) error {
	if len(references.images) > spec.MaxImages {
		return fmt.Errorf("Seedance accepts at most %d reference images for this model", spec.MaxImages)
	}
	if spec.MaxVideos == 0 && len(references.videos) > 0 {
		return fmt.Errorf("Seedance does not support reference videos for this model")
	}
	if len(references.videos) > spec.MaxVideos {
		return fmt.Errorf("Seedance accepts at most %d reference videos for this model", spec.MaxVideos)
	}
	if len(references.audios) > spec.MaxAudios {
		return fmt.Errorf("Seedance accepts at most %d reference audios for this model", spec.MaxAudios)
	}
	if spec.MaxReferenceTotal > 0 && references.count() > spec.MaxReferenceTotal {
		return fmt.Errorf("Seedance accepts at most %d reference items in total for this model", spec.MaxReferenceTotal)
	}
	return nil
}

func jijiuPrompt(body []byte) (string, error) {
	value := gjson.GetBytes(body, "prompt")
	if value.Exists() && value.Type != gjson.Null && value.Type != gjson.String {
		return "", fmt.Errorf("Seedance prompt must be a string")
	}
	prompt := strings.TrimSpace(value.String())
	if prompt == "" {
		// The upstream requires a non-empty prompt even when reference media is sent.
		return "", fmt.Errorf("Seedance prompt is required")
	}
	return prompt, nil
}

func jijiuDurationSeconds(body []byte, spec jijiuModelSpec, references jijiuReferences) (int, error) {
	var value gjson.Result
	for _, path := range []string{"duration", "duration_seconds", "seconds"} {
		value = gjson.GetBytes(body, path)
		if value.Exists() && value.Type != gjson.Null {
			break
		}
	}
	if !value.Exists() || value.Type == gjson.Null {
		return 0, fmt.Errorf("Seedance duration is required and must be an integer number of seconds")
	}
	minSeconds := spec.MinSeconds
	if spec.MinSecondsWithRefs > 0 &&
		(len(references.videos) > 0 || len(references.audios) > 0 || len(references.images) > jijiuMaxImagesWithoutLongerMinimum) {
		minSeconds = spec.MinSecondsWithRefs
	}
	message := fmt.Errorf("Seedance duration for this model must be an integer from %d to %d seconds", minSeconds, spec.MaxSeconds)

	// Clients written for the OpenAI-style video API often send the length as a numeric string
	// ("5"); that is accepted, anything that is not a whole number is not.
	raw := value.Raw
	switch value.Type {
	case gjson.Number:
	case gjson.String:
		raw = strings.TrimSpace(value.String())
	default:
		return 0, message
	}
	rational, ok := new(big.Rat).SetString(raw)
	if !ok || !rational.IsInt() || !rational.Num().IsInt64() {
		return 0, message
	}
	seconds := rational.Num().Int64()
	if seconds < int64(minSeconds) || seconds > int64(spec.MaxSeconds) {
		return 0, message
	}
	return int(seconds), nil
}

// jijiuResolution returns the resolution the task is produced and billed at: the one asked
// for when the model has a choice, otherwise its default. Spellings are compared exactly
// (case-insensitively) so "2k" or "768p" are never silently treated as 1080p or 720p.
func jijiuResolution(body []byte, spec jijiuModelSpec) (string, error) {
	value := gjson.GetBytes(body, "resolution")
	if !value.Exists() || value.Type == gjson.Null {
		return spec.defaultResolution(), nil
	}
	if value.Type != gjson.String {
		return "", fmt.Errorf("Seedance resolution must be a string")
	}
	resolution := strings.ToLower(strings.TrimSpace(value.String()))
	if resolution == "" {
		return spec.defaultResolution(), nil
	}
	if !spec.allowsResolution(resolution) {
		return "", fmt.Errorf("Seedance resolution must be %s for this model", strings.Join(spec.Resolutions, ", "))
	}
	return resolution, nil
}

func jijiuOptionalString(body []byte, paths ...string) (string, error) {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.String {
			return "", fmt.Errorf("Seedance %s must be a string", path)
		}
		return strings.TrimSpace(value.String()), nil
	}
	return "", nil
}

// jijiuReferencesFromRequest accepts the aliases the other Seedance models of this site
// already document (images / videos / audios, reference_images, camelCase variants, single
// image / image_url, and the upstream's own video_refs / audio_refs). The result is
// model-independent; the normalizer writes it under the field names the model expects.
func jijiuReferencesFromRequest(body []byte) (jijiuReferences, error) {
	var result jijiuReferences
	var err error
	if result.images, err = jijiuURLList(body, "image_url",
		[]string{"images", "reference_images", "referenceImages", "reference_image_urls"},
		[]string{"image", "image_url"}); err != nil {
		return result, err
	}
	if result.videos, err = jijiuURLList(body, "video_url",
		[]string{"videos", "reference_videos", "video_refs", "referenceVideos"},
		nil); err != nil {
		return result, err
	}
	if result.audios, err = jijiuURLList(body, "audio_url",
		[]string{"audios", "reference_audios", "audio_refs", "referenceAudios", "reference_audio_urls"},
		nil); err != nil {
		return result, err
	}
	return result, nil
}

// jijiuMediaURL reads one media item written as a bare URL, {"<key>": "URL"},
// {"<key>": {"url": "URL"}} or {"url": "URL"}.
func jijiuMediaURL(item gjson.Result, key string) (string, bool) {
	if item.Type == gjson.String {
		return strings.TrimSpace(item.String()), true
	}
	if !item.IsObject() {
		return "", false
	}
	if nested := item.Get(key); nested.Exists() {
		if nested.Type == gjson.String {
			return strings.TrimSpace(nested.String()), true
		}
		if nested.IsObject() && nested.Get("url").Type == gjson.String {
			return strings.TrimSpace(nested.Get("url").String()), true
		}
		return "", false
	}
	if direct := item.Get("url"); direct.Type == gjson.String {
		return strings.TrimSpace(direct.String()), true
	}
	return "", false
}

// jijiuURLList collects media URLs from the one alias that is present. Two different aliases
// in one request are refused so a field is never counted twice.
func jijiuURLList(body []byte, objectKey string, arrayPaths, singlePaths []string) ([]string, error) {
	var urls []string
	usedPath := ""
	for _, path := range arrayPaths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if !value.IsArray() {
			return nil, fmt.Errorf("Seedance %s must be an array", path)
		}
		if len(value.Array()) == 0 {
			continue
		}
		if usedPath != "" {
			return nil, fmt.Errorf("Seedance accepts only one of %s and %s", usedPath, path)
		}
		usedPath = path
		for index, item := range value.Array() {
			rawURL, ok := jijiuMediaURL(item, objectKey)
			if !ok || rawURL == "" {
				return nil, fmt.Errorf("Seedance %s[%d] must be a URL or an object with %s", path, index, objectKey)
			}
			if err := validateJijiuHTTPSURL(fmt.Sprintf("%s[%d]", path, index), rawURL); err != nil {
				return nil, err
			}
			urls = append(urls, rawURL)
		}
	}
	for _, path := range singlePaths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		rawURL, ok := jijiuMediaURL(value, objectKey)
		if !ok || rawURL == "" {
			return nil, fmt.Errorf("Seedance %s must be a URL", path)
		}
		if usedPath != "" {
			return nil, fmt.Errorf("Seedance accepts only one of %s and %s", usedPath, path)
		}
		if err := validateJijiuHTTPSURL(path, rawURL); err != nil {
			return nil, err
		}
		usedPath = path
		urls = append(urls, rawURL)
	}
	return urls, nil
}

// validateJijiuHTTPSURL requires a public HTTPS address: the upstream fetches the media
// itself and documents HTTPS direct links only.
func validateJijiuHTTPSURL(field, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Scheme != "https" {
		return fmt.Errorf("Seedance %s must be a public HTTPS URL", field)
	}
	if isBlockedHostname(parsed.Hostname()) {
		return fmt.Errorf("Seedance %s must be a public HTTPS URL", field)
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && (!ip.IsGlobalUnicast() || isPrivateIP(ip)) {
		return fmt.Errorf("Seedance %s must be a public HTTPS URL", field)
	}
	return nil
}

// finalizeJijiuSeedanceResponse turns a parsed task response into one whose status comes from
// the explicit status field only. The upstream reports SUCCESS / FAILURE in capitals on some
// lines and completed / failed on others (NormalizePPVideoTaskStatus lower-cases them), may
// carry a result address while the task is still running, and does not always carry one when
// it has finished, so the presence of a video address proves nothing either way.
func finalizeJijiuSeedanceResponse(result PPVideoResponse, body []byte) PPVideoResponse {
	switch result.Status {
	case PPVideoTaskStatusSucceeded, PPVideoTaskStatusFailed, PPVideoTaskStatusProcessing:
	default:
		// NOT_START, SUBMITTED, UNKNOWN: the task exists but has not finished.
		if result.Status != "" || result.TaskID != "" {
			result.Status = PPVideoTaskStatusProcessing
		}
	}
	if result.ErrorMessage == "" && (result.Status == PPVideoTaskStatusFailed || (result.Status == "" && result.TaskID == "")) {
		result.ErrorMessage = extractPPVideoText(body, "fail_reason", "data.fail_reason", "message")
	}
	// Billing uses the seconds that were requested: out-of-range lengths are refused before
	// submission, so they equal the seconds produced, and a length echoed in the response (which
	// may count reference media or be rounded) must not replace them at settlement.
	result.GeneratedDurationMilliseconds = 0
	result.InputVideoDurationMilliseconds = 0
	if result.VideoCount <= 0 {
		result.VideoCount = 1
	}
	return result
}
