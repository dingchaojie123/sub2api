package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"reflect"
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

func video88APIModel(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

func video88APIIsSD25(model string) bool {
	m := video88APIModel(model)
	return m == "sd2.5" || strings.HasPrefix(m, "sd2.5 ")
}

func video88APIIsSD20(model string) bool {
	m := video88APIModel(model)
	return m == "sd2.0" || strings.HasPrefix(m, "sd2.0 ")
}

func video88APIIsSeedanceOfficial25(model string) bool {
	m := video88APIModel(model)
	return m == "seedance-2.5官方版" || strings.HasPrefix(m, "seedance-2.5-")
}

func video88APIIsSeedanceOfficial20(model string) bool {
	m := video88APIModel(model)
	return m == "seedance-2.0官方版" || m == "seedance-2.0-fast官方版" || m == "seedance-2.0-mini官方版" || strings.HasPrefix(m, "seedance-2.0-")
}

func video88APIIsSeedanceMini(model string) bool {
	return strings.Contains(video88APIModel(model), "seedance-2.0-mini")
}

func video88APIIsSeedanceMulti(model string) bool {
	m := video88APIModel(model)
	return strings.HasPrefix(m, "doubao-seedance-2.0") || strings.HasPrefix(m, "doubao-seedance-2-0") || strings.HasPrefix(m, "doubao-seedance-2.5") || strings.HasPrefix(m, "doubao-seedance-2-5")
}

func video88APIIsSeedanceMulti25(model string) bool {
	m := video88APIModel(model)
	return strings.HasPrefix(m, "doubao-seedance-2.5") || strings.HasPrefix(m, "doubao-seedance-2-5")
}

func video88APIIsWan(model string) bool {
	m := video88APIModel(model)
	return m == "wan3.0-video" || m == "wan3.0-video-prime" || strings.HasPrefix(m, "wan3.0-video-")
}

func video88APIIsKling(model string) bool {
	m := video88APIModel(model)
	return m == "kling-3.0-turbo" || strings.HasPrefix(m, "kling-3.0-turbo-")
}

func video88APIIsGrok(model string) bool {
	return strings.HasPrefix(video88APIModel(model), "grok-imagine-video")
}

func video88APIIsVeo(model string) bool {
	return strings.HasPrefix(video88APIModel(model), "veo-3.1")
}

func video88APIIsGemini(model string) bool {
	m := video88APIModel(model)
	return m == "gemini-omni-flash" || m == "gemini-omni-flash-1.1"
}

func video88APICaps(model string) video88APICapabilities {
	m := video88APIModel(model)
	c := video88APICapabilities{1, 0, -1, -1, -1, true, false, false}
	switch {
	case video88APIIsSD25(m):
		c = video88APICapabilities{4, 30, 30, 10, 10, true, true, true}
	case video88APIIsSD20(m):
		c = video88APICapabilities{4, 15, 9, 3, 3, true, true, false}
	case video88APIIsSeedanceOfficial25(m):
		c = video88APICapabilities{4, 30, 30, 10, 10, true, true, false}
	case video88APIIsSeedanceOfficial20(m):
		c = video88APICapabilities{4, 15, 9, 3, 3, !video88APIIsSeedanceMini(m), true, false}
	case video88APIIsSeedanceMulti25(m):
		c = video88APICapabilities{4, 30, 30, 10, 10, true, true, true}
	case video88APIIsSeedanceMulti(m):
		c = video88APICapabilities{4, 15, 9, 3, 3, true, true, true}
	case video88APIIsWan(m):
		c = video88APICapabilities{4, 30, 10, 5, 5, true, true, false}
		if strings.HasSuffix(m, "-480p") {
			c.images, c.videos, c.audios, c.exclusive = 30, 10, 10, false
		}
		if m == "wan3.0-video" || m == "wan3.0-video-prime" {
			c.minSeconds = 2
		}
	case video88APIIsKling(m):
		c = video88APICapabilities{4, 15, 30, 10, 0, true, false, true}
	case video88APIIsGrok(m):
		c = video88APICapabilities{1, 15, 1, 0, 0, false, false, false}
	case m == "gemini-omni-flash":
		c = video88APICapabilities{3, 10, 10, 1, 0, true, false, false}
	case m == "gemini-omni-flash-1.1":
		c = video88APICapabilities{3, 10, 10, 3, 0, true, false, false}
	case video88APIIsVeo(m):
		c = video88APICapabilities{4, 8, 3, 0, 0, true, false, true}
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
	case "3840x2160", "2160x3840":
		return "4k"
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

func video88APIDefaultTier(model string) string {
	m := video88APIModel(model)
	switch {
	case video88APIIsSD25(m), video88APIIsSD20(m):
		return "480p"
	case m == "seedance-2.0-mini官方版":
		return "480p"
	case video88APIIsSeedanceOfficial25(m), video88APIIsSeedanceOfficial20(m), video88APIIsSeedanceMulti(m), video88APIIsKling(m), video88APIIsGrok(m), video88APIIsVeo(m), video88APIIsGemini(m):
		return "720p"
	case video88APIIsWan(m):
		return "1080p"
	}
	return ""
}

func video88APITierAllowed(model, tier string) bool {
	m := video88APIModel(model)
	allowed := ""
	switch {
	case video88APIIsSD25(m):
		allowed = "480p 720p 1080p"
	case video88APIIsSD20(m):
		allowed = "480p 720p 1080p 4k"
	case m == "seedance-2.0-mini官方版":
		allowed = "480p 720p"
	case video88APIIsSeedanceOfficial25(m), video88APIIsSeedanceOfficial20(m):
		allowed = "720p"
	case video88APIIsSeedanceMulti25(m):
		allowed = "480p 720p 1080p"
	case video88APIIsSeedanceMulti(m):
		if strings.Contains(m, "mini") || strings.Contains(m, "fast") {
			allowed = "480p 720p"
		} else {
			allowed = "480p 720p 1080p 4k"
		}
	case m == "gemini-omni-flash":
		allowed = "720p"
	case m == "gemini-omni-flash-1.1", video88APIIsVeo(m):
		allowed = "720p 1080p 4k"
	case video88APIIsKling(m):
		allowed = "720p 1080p 2k 4k"
	case video88APIIsWan(m):
		allowed = "480p 720p 1080p"
	case m == "grok-imagine-video":
		allowed = "480p 720p"
	case strings.HasPrefix(m, "grok-imagine-video-1.5"):
		allowed = "480p 720p 1080p"
	default:
		return true
	}
	for _, candidate := range strings.Fields(allowed) {
		if candidate == tier {
			return true
		}
	}
	return false
}

func video88APIDefaultDuration(model string) int {
	m := video88APIModel(model)
	switch {
	case video88APIIsSD25(m), video88APIIsSD20(m), video88APIIsKling(m), video88APIIsWan(m), video88APIIsSeedanceMulti(m):
		return 5
	case m == "seedance-2.0-mini官方版":
		return 5
	case video88APIIsSeedanceOfficial25(m), video88APIIsSeedanceOfficial20(m):
		return 4
	case video88APIIsVeo(m), video88APIIsGrok(m):
		return 8
	case video88APIIsGemini(m):
		return 3
	}
	return 0
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
		if strings.Contains(s, ":") || s == "auto" || s == "adaptive" {
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
		tier = video88APIDefaultTier(model)
	}
	if tier == "" {
		return "", fmt.Errorf("88API requires an explicit resolution for model %q", model)
	}
	if !video88APITierAllowed(model, tier) {
		return "", fmt.Errorf("unsupported 88API resolution %q for model %q", tier, model)
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

func normalize88APIAspectFields(payload map[string]any, model string) error {
	ratio, pixelSize := "", ""
	meta, _ := payload["metadata"].(map[string]any)
	fields := []struct {
		name  string
		value any
	}{
		{"size", payload["size"]},
		{"ratio", payload["ratio"]},
		{"aspect_ratio", payload["aspect_ratio"]},
		{"metadata.ratio", meta["ratio"]},
		{"metadata.aspect_ratio", meta["aspect_ratio"]},
	}
	for _, field := range fields {
		value := field.value
		if value == nil {
			continue
		}
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("88API %s must be a string", field.name)
		}
		s = strings.ToLower(strings.TrimSpace(s))
		if field.name == "size" {
			switch s {
			case "1280x720", "1920x1080", "3840x2160":
				pixelSize, s = s, "16:9"
			case "720x1280", "1080x1920", "2160x3840":
				pixelSize, s = s, "9:16"
			default:
				if video88APITier(s) != "" {
					continue
				}
			}
		}
		if ratio != "" && s != ratio {
			return fmt.Errorf("88API aspect ratio fields conflict: %q and %q", ratio, s)
		}
		ratio = s
	}
	delete(payload, "size")
	delete(payload, "ratio")
	delete(payload, "aspect_ratio")
	delete(meta, "ratio")
	delete(meta, "aspect_ratio")
	if ratio == "" {
		switch {
		case video88APIIsSD25(model):
			return fmt.Errorf("88API model %q requires an explicit aspect ratio", model)
		case video88APIIsSD20(model):
			ratio = "1:1"
		case video88APIIsSeedanceMulti(model), video88APIIsWan(model):
			ratio = "adaptive"
		default:
			ratio = "16:9"
		}
	}
	if pixelSize != "" && video88APIIsGrok(model) {
		return fmt.Errorf("Grok size does not accept pixel dimensions")
	}
	payload["ratio"] = ratio
	return nil
}

func normalize88APIFrameAliases(payload, meta map[string]any) error {
	for _, aliases := range []struct {
		canonical string
		keys      []string
	}{
		{"firstFrame", []string{"firstFrame", "first_frame"}},
		{"lastFrame", []string{"lastFrame", "last_frame"}},
	} {
		var value any
		for _, key := range aliases.keys {
			for _, source := range []map[string]any{payload, meta} {
				candidate, exists := source[key]
				if !exists || candidate == nil {
					continue
				}
				if value != nil && !reflect.DeepEqual(candidate, value) {
					return fmt.Errorf("conflicting 88API %s fields", aliases.canonical)
				}
				value = candidate
			}
		}
		for _, key := range aliases.keys {
			delete(payload, key)
			delete(meta, key)
		}
		if value != nil {
			meta[aliases.canonical] = value
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
	if err := normalize88APIFrameAliases(payload, meta); err != nil {
		return nil, err
	}
	task, _ := payload["task"].(string)
	task = strings.ToLower(strings.TrimSpace(task))
	geminiExtend := video88APIModel(model) == "gemini-omni-flash-1.1" && task == "extend"
	hadDuration := payload["duration"] != nil || payload["seconds"] != nil
	hadAspect := payload["size"] != nil || payload["ratio"] != nil || payload["aspect_ratio"] != nil || meta["ratio"] != nil || meta["aspect_ratio"] != nil
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
	case nil:
		if geminiExtend {
			duration = 10
		} else {
			duration = float64(video88APIDefaultDuration(model))
		}
		if duration == 0 {
			return nil, fmt.Errorf("88API requires duration in integer seconds")
		}
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
	if geminiExtend && hadDuration {
		return nil, fmt.Errorf("Gemini Omni extension must not include duration")
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
	if geminiExtend {
		delete(payload, "duration")
	} else {
		payload["duration"] = int(duration)
	}
	delete(payload, "seconds")
	tier, err := video88APIResolution(payload, model)
	if err != nil {
		return nil, err
	}
	if err := normalize88APIAspectFields(payload, model); err != nil {
		return nil, err
	}
	if video88APIFixedTier(model) != "" {
		delete(payload, "resolution")
		delete(meta, "resolution")
	} else {
		payload["resolution"] = tier
		delete(meta, "resolution")
	}
	if geminiExtend {
		if tier == "4k" {
			return nil, fmt.Errorf("Gemini Omni extension supports only 720p or 1080p")
		}
		if hadAspect {
			return nil, fmt.Errorf("Gemini Omni extension must not include an aspect ratio")
		}
		delete(payload, "ratio")
		delete(payload, "size")
	}
	aspect := payload["ratio"]
	if payload["size"] != nil {
		aspect = payload["size"]
	}
	m := strings.ToLower(model)
	if !geminiExtend {
		if err := video88APIValidateAspect(m, aspect); err != nil {
			return nil, err
		}
	}
	veo := video88APIIsVeo(m)
	if veo {
		if duration != 4 && duration != 6 && duration != 8 {
			return nil, fmt.Errorf("Veo duration must be 4, 6 or 8 seconds")
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
	if meta["images"] != nil {
		return nil, fmt.Errorf("metadata.images is not a supported 88API image field")
	}
	allowMetadataImages := video88APIIsWan(m)
	if meta["referenceImages"] != nil && !allowMetadataImages {
		return nil, fmt.Errorf("use top-level images for 88API image references")
	}
	if payload["videos"] != nil && payload["video"] != nil {
		return nil, fmt.Errorf("use only one video field")
	}
	topVideos := payload["videos"]
	if payload["video"] != nil {
		topVideos = []any{payload["video"]}
	}
	if topVideos != nil && meta["referenceVideos"] != nil {
		return nil, fmt.Errorf("use only one reference video field")
	}
	googleVideo := video88APIIsGemini(m) || video88APIIsVeo(m)
	if googleVideo {
		if meta["referenceVideos"] != nil {
			return nil, fmt.Errorf("Google video models use top-level video or videos")
		}
	} else if topVideos != nil {
		meta["referenceVideos"] = topVideos
		delete(payload, "video")
		delete(payload, "videos")
	}
	inlineImages := googleVideo || video88APIIsSeedanceMulti(m)
	imageCount, err := video88APIValidateMedia(payload["images"], caps.images, inlineImages)
	if err != nil {
		return nil, fmt.Errorf("images: %w", err)
	}
	if veo {
		if err := video88APIRejectInlineWebP(payload["images"]); err != nil {
			return nil, err
		}
	}
	metadataImageCount, err := video88APIValidateMedia(meta["referenceImages"], caps.images, false)
	if err != nil {
		return nil, fmt.Errorf("reference images: %w", err)
	}
	if imageCount > 0 && metadataImageCount > 0 {
		return nil, fmt.Errorf("use only one reference image field")
	}
	imageCount += metadataImageCount
	videos := meta["referenceVideos"]
	if googleVideo {
		videos = topVideos
	}
	videoCount, err := video88APIValidateMedia(videos, caps.videos, false)
	if err != nil {
		return nil, fmt.Errorf("reference videos: %w", err)
	}
	audioCount, err := video88APIValidateMedia(meta["referenceAudios"], caps.audios, false)
	if err != nil {
		return nil, fmt.Errorf("reference audios: %w", err)
	}
	wanImages, wanVideos, wanAudios, wanFirst, wanLast, err := video88APIValidateWanMedia(payload["media"])
	if err != nil {
		return nil, err
	}
	if payload["media"] != nil {
		if !video88APIIsWan(m) {
			return nil, fmt.Errorf("media is supported only by Wan video models")
		}
		if imageCount+videoCount+audioCount > 0 || meta["firstFrame"] != nil || meta["lastFrame"] != nil {
			return nil, fmt.Errorf("Wan media cannot be combined with other reference fields")
		}
	}
	imageCount += wanImages
	videoCount += wanVideos
	audioCount += wanAudios
	if video88APIIsSD20(m) && imageCount+videoCount+audioCount > 12 {
		return nil, fmt.Errorf("SD2.0 supports at most 12 total references")
	}
	if video88APIIsSeedanceMulti(m) && !video88APIIsSeedanceMulti25(m) && audioCount > 0 && imageCount+videoCount == 0 {
		return nil, fmt.Errorf("Seedance 2.0 reference audio requires an image or video")
	}
	first, last := meta["firstFrame"], meta["lastFrame"]
	if wanFirst {
		first = "media:first_frame"
	}
	if wanLast {
		last = "media:last_frame"
	}
	geminiEdit := m == "gemini-omni-flash-1.1" && videoCount == 1 && imageCount == 0 && first == nil && (task == "" || task == "edit" || task == "extend")
	if geminiEdit {
		if hadAspect {
			return nil, fmt.Errorf("Gemini Omni video editing must not include an aspect ratio")
		}
		delete(payload, "ratio")
		delete(payload, "size")
	}
	if m == "gemini-omni-flash" && videoCount > 0 && imageCount == 0 {
		return nil, fmt.Errorf("Gemini Omni Flash reference video requires a reference image")
	}
	prompt, promptOK := payload["prompt"].(string)
	if payload["prompt"] != nil && !promptOK {
		return nil, fmt.Errorf("prompt must be a string")
	}
	promptOptional := (video88APIIsWan(m) || video88APIIsSD20(m) || video88APIIsSeedanceMulti(m) || video88APIIsGrok(m)) && (imageCount+videoCount+audioCount > 0 || first != nil)
	if strings.TrimSpace(prompt) == "" && !promptOptional {
		return nil, fmt.Errorf("88API requires a prompt for this request")
	}
	if video88APIIsKling(m) && len([]rune(prompt)) > 2000 {
		return nil, fmt.Errorf("Kling prompt must not exceed 2000 characters")
	}
	if first != nil || last != nil {
		if !caps.frames || (last != nil && first == nil) {
			return nil, fmt.Errorf("unsupported first/last frame combination")
		}
		if caps.exclusive && imageCount+videoCount+audioCount > 0 {
			return nil, fmt.Errorf("first/last frames cannot be combined with reference media")
		}
		for _, frame := range []any{meta["firstFrame"], meta["lastFrame"]} {
			if frame != nil {
				if _, err := video88APIValidateMedia([]any{frame}, 1, inlineImages); err != nil {
					return nil, fmt.Errorf("frame: %w", err)
				}
			}
		}
	}
	if video88APIIsKling(m) {
		frameCount := 0
		if first != nil {
			frameCount++
		}
		if last != nil {
			frameCount++
		}
		if imageCount+frameCount > caps.images {
			return nil, fmt.Errorf("Kling supports at most %d total images and frames", caps.images)
		}
	}
	if video88APIIsSeedanceMulti25(m) && (first != nil || last != nil) {
		aspect, _ := payload["ratio"].(string)
		if aspect != "adaptive" {
			return nil, fmt.Errorf("Seedance 2.5 first/last frames require adaptive ratio")
		}
	}
	if geminiExtend && videoCount != 1 {
		return nil, fmt.Errorf("Gemini Omni extension requires one video")
	}
	if veo && imageCount > 0 {
		mode, _ := meta["video_mode"].(string)
		if mode == "" {
			if imageCount <= 2 {
				mode = "frames"
			} else {
				mode = "reference"
			}
			meta["video_mode"] = mode
		}
		if mode != "frames" && mode != "reference" {
			return nil, fmt.Errorf("Veo metadata.video_mode must be frames or reference")
		}
		if mode == "frames" && imageCount > 2 {
			return nil, fmt.Errorf("Veo frames supports at most two images")
		}
		if mode == "reference" && duration != 8 {
			return nil, fmt.Errorf("Veo reference mode requires 8 seconds")
		}
	}
	generateAudio := payload["generate_audio"]
	if generateAudio == nil {
		generateAudio = payload["generateAudio"]
	}
	if generateAudio == nil {
		generateAudio = meta["generateAudio"]
	}
	if generateAudio != nil {
		if _, ok := generateAudio.(bool); !ok || !caps.audio {
			return nil, fmt.Errorf("generate_audio is not supported or is not boolean for this model")
		}
		payload["generate_audio"] = generateAudio
		delete(payload, "generateAudio")
		delete(meta, "generateAudio")
	}
	if video88APIIsGrok(m) {
		for _, key := range []string{"negative_prompt", "seed"} {
			if payload[key] != nil || meta[key] != nil {
				return nil, fmt.Errorf("Grok does not support %s", key)
			}
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
	case "1280x720", "1920x1080", "3840x2160":
		ratio = "16:9"
	case "720x1280", "1080x1920", "2160x3840":
		ratio = "9:16"
	}
	allowed := ""
	switch {
	case video88APIIsSD25(model), video88APIIsSD20(model), video88APIIsSeedanceOfficial25(model), video88APIIsSeedanceOfficial20(model):
		allowed = "1:1 21:9 16:9 9:16 3:4 4:3"
	case video88APIIsSeedanceMulti(model), video88APIIsWan(model):
		allowed = "adaptive 1:1 21:9 16:9 9:16 3:4 4:3"
	case video88APIIsKling(model):
		allowed = "1:1 16:9 9:16"
	case video88APIIsGrok(model):
		allowed = "1:1 16:9 9:16 3:4 4:3 3:2 2:3"
	case video88APIIsVeo(model), video88APIIsGemini(model):
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
		if !ok && base64Images {
			if object, objectOK := item.(map[string]any); objectOK {
				s, ok = object["url"].(string)
			}
		}
		if !ok || s == "" {
			return 0, fmt.Errorf("reference must be a nonempty string")
		}
		u, urlErr := url.Parse(s)
		if urlErr == nil && u.Scheme == "https" && u.Hostname() != "" && miniMaxH3ImageProbeURLAllowed(&http.Request{URL: u}) {
			continue
		}
		if base64Images {
			if strings.HasPrefix(s, "data:") {
				prefix, data, found := strings.Cut(s, ",")
				if !found || (prefix != "data:image/jpeg;base64" && prefix != "data:image/png;base64" && prefix != "data:image/webp;base64") {
					return 0, fmt.Errorf("inline image must be PNG, JPEG or WebP base64")
				}
				s = data
			}
			decoded, err := base64.StdEncoding.DecodeString(s)
			if err != nil || len(decoded) == 0 || len(decoded) > 20<<20 {
				return 0, fmt.Errorf("inline image must be valid base64 and at most 20 MiB")
			}
			continue
		}
		return 0, fmt.Errorf("reference must be a public HTTPS URL")
	}
	return len(items), nil
}

func video88APIRejectInlineWebP(value any) error {
	items, _ := value.([]any)
	for _, item := range items {
		s, _ := item.(string)
		if object, ok := item.(map[string]any); ok {
			s, _ = object["url"].(string)
		}
		if strings.HasPrefix(strings.ToLower(s), "data:image/webp;") {
			return fmt.Errorf("Veo inline images must be PNG or JPEG")
		}
	}
	return nil
}

func video88APIValidateWanMedia(value any) (images, videos, audios int, first, last bool, err error) {
	if value == nil {
		return 0, 0, 0, false, false, nil
	}
	items, ok := value.([]any)
	if !ok {
		return 0, 0, 0, false, false, fmt.Errorf("media must be an array")
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return 0, 0, 0, false, false, fmt.Errorf("media entries must be objects")
		}
		typeName, typeOK := item["type"].(string)
		mediaURL, urlOK := item["url"].(string)
		if !typeOK || !urlOK {
			return 0, 0, 0, false, false, fmt.Errorf("media entries require string type and url")
		}
		if _, validateErr := video88APIValidateMedia([]any{mediaURL}, 1, false); validateErr != nil {
			return 0, 0, 0, false, false, validateErr
		}
		switch typeName {
		case "first_frame":
			if first {
				return 0, 0, 0, false, false, fmt.Errorf("media accepts one first_frame")
			}
			first = true
		case "last_frame":
			if last {
				return 0, 0, 0, false, false, fmt.Errorf("media accepts one last_frame")
			}
			last = true
		case "reference_image":
			images++
		case "reference_video":
			videos++
		case "reference_audio":
			audios++
		default:
			return 0, 0, 0, false, false, fmt.Errorf("unsupported media type %q", typeName)
		}
	}
	if last && !first {
		return 0, 0, 0, false, false, fmt.Errorf("last_frame requires first_frame")
	}
	if (first || last) && images+videos+audios > 0 {
		return 0, 0, 0, false, false, fmt.Errorf("first/last frames cannot be combined with reference media")
	}
	if images > 10 || videos > 5 || audios > 5 {
		return 0, 0, 0, false, false, fmt.Errorf("media exceeds Wan reference limits")
	}
	return images, videos, audios, first, last, nil
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
	if meta.RequestedDurationMilliseconds == 0 && video88APIModel(meta.Model) == "gemini-omni-flash-1.1" && strings.EqualFold(gjson.GetBytes(body, "task").String(), "extend") {
		meta.RequestedDurationMilliseconds = 10_000
	}
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
