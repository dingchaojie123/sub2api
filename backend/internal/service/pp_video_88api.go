package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

const Video88APIDefaultBaseURL = "https://88api.ai/v1"

// Capabilities constrain documented families, never the online model catalog.
// New models use the common protocol and require an explicitly priced tier.
type video88APICapabilities struct {
	minSeconds, maxSeconds   int
	images, videos, audios   int
	frames, exclusive, audio bool
}

func video88APICaps(model string) video88APICapabilities {
	m := strings.ToLower(model)
	c := video88APICapabilities{1, 0, -1, -1, -1, true, false, false}
	switch {
	case strings.HasPrefix(m, "sd2.5 "):
		c = video88APICapabilities{4, 30, 30, 10, 10, true, true, true}
	case strings.HasPrefix(m, "sd2.0 "):
		c = video88APICapabilities{4, 15, 9, 3, 3, true, true, false}
	case strings.HasPrefix(m, "seedance-2.5-"):
		c = video88APICapabilities{4, 30, 30, 10, 10, true, false, false}
	case strings.HasPrefix(m, "seedance-2.0-"):
		c = video88APICapabilities{4, 15, 9, 3, 3, !strings.Contains(m, "-mini-"), true, false}
	case strings.HasPrefix(m, "wan3.0-video-"):
		c = video88APICapabilities{4, 30, 10, 5, 5, true, true, false}
		if strings.HasSuffix(m, "-480p") {
			c.images, c.videos, c.audios, c.exclusive = 30, 10, 10, false
		}
	case strings.HasPrefix(m, "kling-3.0-turbo-"):
		c = video88APICapabilities{4, 15, 30, 10, 0, true, false, true}
	case strings.HasPrefix(m, "grok-imagine-video"):
		c = video88APICapabilities{1, 15, 1, 0, 0, false, false, false}
	case m == "gemini-omni-flash":
		c = video88APICapabilities{3, 10, 10, 1, 0, false, false, false}
	case strings.HasPrefix(m, "veo-3.1"):
		c = video88APICapabilities{4, 8, 3, 0, 0, false, false, false}
	}
	return c
}

func video88APITier(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480p", "720p", "768p", "1080p", "2k", "4k":
		return strings.ToLower(strings.TrimSpace(value))
	case "1280x720", "720x1280":
		return "720p"
	case "1920x1080", "1080x1920":
		return "1080p"
	}
	return ""
}

func video88APIFixedTier(model string) string {
	// Only explicitly configured models are fixed; online model names are not a schema.
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "sd2.0 480p", "sd2.5 480p", "seedance-2.0-mini-480p", "wan3.0-video-480p":
		return "480p"
	case "sd2.0 720p", "sd2.5 720p", "seedance-2.0-720p官方版", "seedance-2.0-fast-720p官方版", "seedance-2.5-720p官方版", "kling-3.0-turbo-720p", "seedance-2.0-mini-720p", "wan3.0-video-720p":
		return "720p"
	case "sd2.0 1080p", "sd2.5 1080p", "grok-imagine-video-1.5-1080p", "kling-3.0-turbo-1080p", "wan3.0-video-1080p":
		return "1080p"
	case "kling-3.0-turbo-2k":
		return "2k"
	case "kling-3.0-turbo-4k":
		return "4k"
	}
	return ""
}

func video88APIResolution(payload map[string]any, model string) (string, error) {
	if tier := video88APIFixedTier(model); tier != "" {
		return tier, nil
	}
	tier := ""
	meta, _ := payload["metadata"].(map[string]any)
	for _, value := range []any{payload["resolution"], meta["resolution"], payload["size"]} {
		if value == nil {
			continue
		}
		s, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("88API resolution and size must be strings")
		}
		if strings.Contains(s, ":") || s == "auto" {
			continue
		}
		candidate := video88APITier(s)
		if candidate == "" {
			return "", fmt.Errorf("unsupported 88API resolution %q", s)
		}
		if tier != "" && candidate != tier {
			return "", fmt.Errorf("resolution %q conflicts with model or requested tier %q", candidate, tier)
		}
		tier = candidate
	}
	if tier == "" {
		return "", fmt.Errorf("88API requires an explicit resolution for model %q", model)
	}
	return tier, nil
}

// Normalize the downstream envelope only for 88API, before validation and pricing.
func normalize88APIDownstreamFields(payload map[string]any) error {
	for _, group := range []struct {
		name   string
		fields [][2]string
	}{
		{"input", [][2]string{{"prompt", "prompt"}, {"img_url", "image"}}},
		{"parameters", [][2]string{{"duration", "duration"}, {"resolution", "resolution"}, {"aspect_ratio", "size"}}},
	} {
		value, exists := payload[group.name]
		if !exists {
			continue
		}
		nested, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("88API %s must be an object", group.name)
		}
		for _, field := range group.fields {
			value, exists := nested[field[0]]
			if !exists {
				continue
			}
			if _, exists := payload[field[1]]; exists {
				return fmt.Errorf("use either %s.%s or %s, not both", group.name, field[0], field[1])
			}
			payload[field[1]] = value
			delete(nested, field[0])
		}
		if len(nested) == 0 {
			delete(payload, group.name)
		}
	}
	return nil
}

func normalize88APIVideoPayload(payload map[string]any, public *PPVideoPublicRequest) (map[string]any, error) {
	model, ok := payload["model"].(string)
	if !ok || strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("88API requires an explicit model from the account model list")
	}
	model = strings.TrimSpace(public.UpstreamModel)
	if model == "" {
		model = strings.TrimSpace(payload["model"].(string))
	}
	payload["model"] = model
	if video88APIFixedTier(model) != "" {
		// Discard resolution overrides before envelope collision checks.
		delete(payload, "resolution")
		if parameters, ok := payload["parameters"].(map[string]any); ok {
			delete(parameters, "resolution")
		}
	}
	if err := normalize88APIDownstreamFields(payload); err != nil {
		return nil, err
	}
	meta := map[string]any{}
	if v, exists := payload["metadata"]; exists {
		var ok bool
		meta, ok = v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("metadata must be an object")
		}
	}
	payload["metadata"] = meta
	if v, exists := payload["callback_url"]; exists && v != "" && v != nil {
		return nil, fmt.Errorf("88API integration uses polling; callback_url is not supported")
	}
	delete(payload, "callback_url")
	if payload["duration"] != nil && payload["seconds"] != nil {
		return nil, fmt.Errorf("use either duration or seconds")
	}
	seconds := payload["duration"]
	if seconds == nil {
		seconds = payload["seconds"]
	}
	var duration float64
	switch v := seconds.(type) {
	case float64:
		duration = v
	case string:
		var err error
		duration, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid video duration")
		}
	default:
		return nil, fmt.Errorf("88API requires duration in integer seconds")
	}
	caps := video88APICaps(model)
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration != math.Trunc(duration) || duration < float64(caps.minSeconds) || duration > 86400 || (caps.maxSeconds > 0 && duration > float64(caps.maxSeconds)) {
		return nil, fmt.Errorf("invalid duration for 88API model %q", model)
	}
	for _, key := range []string{"n", "count", "video_count"} {
		if v, exists := payload[key]; exists && v != float64(1) {
			return nil, fmt.Errorf("88API creates one video per task")
		}
		delete(payload, key)
	}
	payload["duration"] = int(duration)
	delete(payload, "seconds")
	tier, err := video88APIResolution(payload, model)
	if err != nil {
		return nil, err
	}
	delete(payload, "resolution")
	if video88APIFixedTier(model) != "" {
		delete(meta, "resolution")
		// Fixed models encode the output tier in their name; size carries only aspect.
		switch payload["size"] {
		case "1280x720", "1920x1080":
			payload["size"] = "16:9"
		case "720x1280", "1080x1920":
			payload["size"] = "9:16"
		}
	} else {
		meta["resolution"] = tier
	}
	if payload["size"] == nil {
		payload["size"] = "16:9"
	}
	m := strings.ToLower(model)
	if err := video88APIValidateAspect(m, payload["size"]); err != nil {
		return nil, err
	}
	veo := strings.HasPrefix(m, "veo-3.1")
	if veo {
		if duration != 4 && duration != 6 && duration != 8 {
			return nil, fmt.Errorf("Veo duration must be 4, 6 or 8 seconds")
		}
		if payload["size"] == "16:9" && (tier == "720p" || tier == "1080p") {
			if tier == "720p" {
				payload["size"] = "1280x720"
			} else {
				payload["size"] = "1920x1080"
			}
		}
		if payload["size"] == "9:16" && (tier == "720p" || tier == "1080p") {
			if tier == "720p" {
				payload["size"] = "720x1280"
			} else {
				payload["size"] = "1080x1920"
			}
		}
	}
	for _, key := range []string{"image", "input_reference"} {
		if v, exists := payload[key]; exists {
			if payload["images"] != nil {
				return nil, fmt.Errorf("use only one image field")
			}
			payload["images"] = []any{v}
			delete(payload, key)
		}
	}
	for _, key := range []string{"referenceImages", "images"} {
		if meta[key] != nil {
			return nil, fmt.Errorf("use top-level images for 88API image references")
		}
	}
	if payload["videos"] != nil && payload["video"] != nil {
		return nil, fmt.Errorf("use only one video field")
	}
	if payload["video"] != nil {
		payload["videos"] = []any{payload["video"]}
		delete(payload, "video")
	}
	if payload["videos"] != nil && meta["referenceVideos"] != nil {
		return nil, fmt.Errorf("use only one reference video field")
	}
	if m == "gemini-omni-flash" {
		if meta["referenceVideos"] != nil {
			return nil, fmt.Errorf("Gemini Omni uses top-level video or videos")
		}
	} else if payload["videos"] != nil {
		meta["referenceVideos"] = payload["videos"]
		delete(payload, "videos")
	}
	imageCount, err := video88APIValidateMedia(payload["images"], caps.images, veo)
	if err != nil {
		return nil, fmt.Errorf("images: %w", err)
	}
	videos := meta["referenceVideos"]
	if m == "gemini-omni-flash" {
		videos = payload["videos"]
	}
	videoCount, err := video88APIValidateMedia(videos, caps.videos, false)
	if err != nil {
		return nil, fmt.Errorf("reference videos: %w", err)
	}
	audioCount, err := video88APIValidateMedia(meta["referenceAudios"], caps.audios, false)
	if err != nil {
		return nil, fmt.Errorf("reference audios: %w", err)
	}
	if strings.HasPrefix(m, "sd2.0 ") && imageCount+videoCount+audioCount > 12 {
		return nil, fmt.Errorf("SD2.0 supports at most 12 total references")
	}
	first, last := meta["firstFrame"], meta["lastFrame"]
	prompt, promptOK := payload["prompt"].(string)
	if payload["prompt"] != nil && !promptOK {
		return nil, fmt.Errorf("prompt must be a string")
	}
	if strings.TrimSpace(prompt) == "" && !(strings.HasPrefix(m, "wan3.0-video-") && (imageCount+videoCount+audioCount > 0 || first != nil)) {
		return nil, fmt.Errorf("88API requires a prompt for this request")
	}
	if first != nil || last != nil {
		if !caps.frames || first == nil {
			return nil, fmt.Errorf("unsupported first/last frame combination")
		}
		if caps.exclusive && imageCount+videoCount+audioCount > 0 {
			return nil, fmt.Errorf("first/last frames cannot be combined with reference media")
		}
		for _, frame := range []any{first, last} {
			if frame != nil {
				if _, err := video88APIValidateMedia([]any{frame}, 1, false); err != nil {
					return nil, fmt.Errorf("frame: %w", err)
				}
			}
		}
	}
	if veo && imageCount > 0 {
		mode, _ := meta["video_mode"].(string)
		if mode != "frames" && mode != "reference" {
			return nil, fmt.Errorf("Veo images require metadata.video_mode frames or reference")
		}
		if mode == "frames" && imageCount > 2 {
			return nil, fmt.Errorf("Veo frames supports at most two images")
		}
		if mode == "reference" && duration != 8 {
			return nil, fmt.Errorf("Veo reference mode requires 8 seconds")
		}
	}
	if v, exists := payload["generate_audio"]; exists {
		if _, ok := v.(bool); !ok || !caps.audio {
			return nil, fmt.Errorf("generate_audio is not supported or is not boolean for this model")
		}
	}
	public.Model = strings.TrimSpace(public.Model)
	public.Prompt = prompt
	public.UpstreamModel, public.Resolution = model, tier
	public.DurationMilliseconds, public.VideoCount, public.HasImage = int64(duration)*1000, 1, imageCount > 0 || first != nil
	return payload, nil
}

func video88APIValidateAspect(model string, size any) error {
	s, ok := size.(string)
	if !ok {
		return fmt.Errorf("size must be a string")
	}
	ratio := s
	switch s {
	case "1280x720", "1920x1080":
		ratio = "16:9"
	case "720x1280", "1080x1920":
		ratio = "9:16"
	}
	allowed := ""
	switch {
	case strings.HasPrefix(model, "sd2.5 "):
		allowed = "auto 1:1 21:9 16:9 9:16 3:4 4:3"
	case strings.HasPrefix(model, "sd2.0 "), strings.HasPrefix(model, "seedance-2."):
		allowed = "1:1 21:9 16:9 9:16 3:4 4:3"
	case strings.HasPrefix(model, "wan3.0-video-"):
		allowed = "1:1 16:9 9:16 3:4 4:3"
	case strings.HasPrefix(model, "kling-3.0-turbo-"):
		allowed = "1:1 16:9 9:16"
	case strings.HasPrefix(model, "grok-imagine-video"):
		allowed = "1:1 16:9 9:16 3:4 4:3 3:2 2:3"
	case strings.HasPrefix(model, "veo-3.1"), model == "gemini-omni-flash":
		allowed = "16:9 9:16"
	}
	if allowed != "" {
		for _, candidate := range strings.Fields(allowed) {
			if candidate == ratio {
				return nil
			}
		}
		return fmt.Errorf("unsupported size %q for 88API model %q", s, model)
	}
	return nil
}

func video88APIValidateMedia(value any, limit int, base64Images bool) (int, error) {
	if value == nil {
		return 0, nil
	}
	items, ok := value.([]any)
	if !ok {
		return 0, fmt.Errorf("must be an array of strings")
	}
	if limit >= 0 && len(items) > limit {
		return 0, fmt.Errorf("at most %d references allowed", limit)
	}
	for _, item := range items {
		s, ok := item.(string)
		if !ok || s == "" {
			return 0, fmt.Errorf("reference must be a nonempty string")
		}
		if base64Images {
			if strings.HasPrefix(s, "data:") {
				prefix, data, found := strings.Cut(s, ",")
				if !found || (prefix != "data:image/jpeg;base64" && prefix != "data:image/png;base64") {
					return 0, fmt.Errorf("Veo requires PNG/JPEG base64 images")
				}
				s = data
			}
			decoded, err := base64.StdEncoding.DecodeString(s)
			if err != nil || len(decoded) == 0 || len(decoded) > 20<<20 {
				return 0, fmt.Errorf("Veo image must be base64 and at most 20 MiB")
			}
			continue
		}
		u, err := url.Parse(s)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || !miniMaxH3ImageProbeURLAllowed(&http.Request{URL: u}) {
			return 0, fmt.Errorf("reference must be a public HTTPS URL")
		}
	}
	return len(items), nil
}

func video88APIBillingMetadata(body []byte) PPVideoBillingMetadata {
	meta := PPVideoBillingMetadata{Platform: Platform88APIVideo, Model: PPVideoModelFromBody(body), VideoCount: 1}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		meta.ValidationError = err
		return meta
	}
	meta.VideoResolution, meta.ValidationError = video88APIResolution(payload, meta.Model)
	meta.RequestedDurationMilliseconds = int64(gjson.GetBytes(body, "duration").Float() * 1000)
	meta.RequestedDurationSeconds = int(meta.RequestedDurationMilliseconds / 1000)
	meta.HasAudio = gjson.GetBytes(body, "generate_audio").Bool() || gjson.GetBytes(body, "metadata.generateAudio").Bool() || strings.HasPrefix(strings.ToLower(meta.Model), "sd2.0 ")
	return meta
}

func parse88APIVideoResponse(body []byte) (PPVideoResponse, error) {
	r := PPVideoResponse{TaskID: gjson.GetBytes(body, "id").String(), Status: PPVideoTaskStatusProcessing, VideoCount: 1, RawBody: append([]byte(nil), body...)}
	r.ErrorMessage = extractPPVideoText(body, "error.message")
	switch gjson.GetBytes(body, "status").String() {
	case "completed":
		media := extractPPVideoText(body, "url", "video_url", "result_url")
		u, err := url.Parse(media)
		if err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil {
			r.Status = PPVideoTaskStatusSucceeded
		}
	case "failed":
		r.Status = PPVideoTaskStatusFailed
	case "queued", "in_progress", "unknown":
	default:
		return PPVideoResponse{}, fmt.Errorf("88API returned an invalid task status")
	}
	if r.TaskID == "" {
		return PPVideoResponse{}, fmt.Errorf("88API response missing id")
	}
	return r, nil
}

func (s *OpenAIGatewayService) calculate88APIVideoCost(ctx context.Context, meta PPVideoBillingMetadata, key *APIKey) (PPVideoBillingQuote, error) {
	missing := func(reason string) error {
		groupID := int64(0)
		if key != nil && key.GroupID != nil {
			groupID = *key.GroupID
		}
		return fmt.Errorf("configure 88API video pricing for model %q and resolution %q in the group's channel (group_id=%d): %s", meta.Model, meta.VideoResolution, groupID, reason)
	}
	if s == nil || s.resolver == nil {
		return PPVideoBillingQuote{}, missing("pricing service is unavailable")
	}
	if key == nil || key.GroupID == nil {
		return PPVideoBillingQuote{}, missing("API key has no pricing group")
	}
	if meta.RequestedDurationMilliseconds <= 0 {
		return PPVideoBillingQuote{}, missing("requested duration must be positive")
	}
	resolved := s.resolver.Resolve(ctx, PricingInput{Model: meta.Model, GroupID: key.GroupID})
	p := resolved.channelPricing
	if p == nil {
		return PPVideoBillingQuote{}, missing("no channel model price matched; check the active channel, group association, platform and model name")
	}
	if p.Platform != Platform88APIVideo {
		return PPVideoBillingQuote{}, missing("matched price belongs to another platform")
	}
	if p.BillingMode != BillingModeVideo {
		return PPVideoBillingQuote{}, missing("billing mode must be video (per second)")
	}
	exact := false
	for _, model := range p.Models {
		if strings.EqualFold(model, meta.Model) {
			exact = true
		}
	}
	if !exact {
		return PPVideoBillingQuote{}, missing("an exact model price is required; wildcard prices are not supported")
	}
	var price *float64
	matchedTier := false
	for _, tier := range p.Intervals {
		if strings.EqualFold(strings.TrimSpace(tier.TierLabel), meta.VideoResolution) {
			price = tier.PerRequestPrice
			matchedTier = true
			break
		}
	}
	// A fixed-resolution model may use its own flat price. Variable-resolution
	// models require an exact tier so an omitted tier cannot silently underbill.
	if len(p.Intervals) == 0 && video88APIFixedTier(meta.Model) == meta.VideoResolution {
		price = p.PerRequestPrice
		if price == nil {
			return PPVideoBillingQuote{}, missing("fixed-resolution model has no default per-second price")
		}
	} else if !matchedTier {
		return PPVideoBillingQuote{}, missing("no matching resolution tier; add a tier with the requested resolution and a per-second price")
	}
	if price == nil || *price < 0 || math.IsNaN(*price) || math.IsInf(*price, 0) {
		return PPVideoBillingQuote{}, missing("matched resolution tier has an empty or invalid per-second price")
	}
	units := float64(meta.RequestedDurationMilliseconds) / 1000
	return PPVideoBillingQuote{Formula: PPVideoBillingFormulaPerSecond, Units: units, UnitPrice: *price, Cost: units * *price}, nil
}
