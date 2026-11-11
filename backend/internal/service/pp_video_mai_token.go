package service

import (
	"fmt"
	"math/big"
	"net"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	maiTokenMaxReferenceImages = 30
	maiTokenMaxReferenceVideos = 10
	maiTokenMaxReferenceAudios = 10
	maiTokenMaxReferences      = 50

	// The generation length the platform accepts, in whole seconds. Shared by request
	// validation, the capabilities list and the price quote.
	maiTokenMinDurationSeconds = 4
	maiTokenMaxDurationSeconds = 30
)

type maiTokenReferences struct {
	images []string
	videos []string
	audios []string
	first  string
	last   string
	texts  []string
}

func normalizeMaiTokenSeedancePayload(_ map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if public == nil {
		return nil, fmt.Errorf("Mai Token Seedance request body is required")
	}
	if !public.HasExplicitModel {
		return nil, fmt.Errorf("model must be %q", MaiTokenSeedancePublicModel)
	}
	publicModel, knownModel := ppVideoMaiTokenPublicModel(public.Model)
	if !knownModel {
		return nil, fmt.Errorf("model must be %q", MaiTokenSeedancePublicModel)
	}
	if err := maiTokenRejectUnsupportedFields(body); err != nil {
		return nil, err
	}

	references, err := maiTokenReferencesFromRequest(body)
	if err != nil {
		return nil, err
	}
	prompt, err := maiTokenPrompt(body, references.texts)
	if err != nil {
		return nil, err
	}
	duration, err := maiTokenDurationSeconds(body)
	if err != nil {
		return nil, err
	}
	resolution, err := maiTokenResolution(body)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"model":      MaiTokenSeedanceModel,
		"prompt":     prompt,
		"duration":   duration,
		"resolution": resolution,
	}
	if ratio, ok, ratioErr := maiTokenOptionalString(body, "ratio", "aspect_ratio"); ratioErr != nil {
		return nil, ratioErr
	} else if ok && ratio != "" {
		result["ratio"] = ratio
	}
	if len(references.images) > 0 {
		result["referenceImages"] = references.images
	}
	if len(references.videos) > 0 {
		result["referenceVideos"] = references.videos
	}
	if len(references.audios) > 0 {
		result["referenceAudios"] = references.audios
	}
	if references.first != "" {
		result["first_image"] = references.first
	}
	if references.last != "" {
		result["last_image"] = references.last
	}
	if value, ok, boolErr := maiTokenOptionalBool(body, "generate_audio", "generateAudio", "options.generate_audio"); boolErr != nil {
		return nil, boolErr
	} else if ok {
		result["generate_audio"] = value
	}
	if value, ok, boolErr := maiTokenOptionalBool(body, "watermark"); boolErr != nil {
		return nil, boolErr
	} else if ok {
		result["watermark"] = value
	}

	public.Model = publicModel
	public.UpstreamModel = MaiTokenSeedanceModel
	public.Prompt = prompt
	public.DurationMilliseconds = int64(duration) * 1000
	public.VideoCount = 1
	public.Resolution = resolution
	public.HasImage = references.first != "" || len(references.images) > 0
	return result, nil
}

func maiTokenRejectUnsupportedFields(body []byte) error {
	for _, path := range []string{
		"seed", "draft", "callback", "callback_url", "callback_token",
		"return_last_frame", "n", "count",
	} {
		value := gjson.GetBytes(body, path)
		if value.Exists() && value.Type != gjson.Null {
			return fmt.Errorf("Mai Token Seedance does not support %s", path)
		}
	}
	return nil
}

func maiTokenPrompt(body []byte, contentTexts []string) (string, error) {
	value := gjson.GetBytes(body, "prompt")
	if value.Exists() && value.Type != gjson.Null && value.Type != gjson.String {
		return "", fmt.Errorf("Mai Token Seedance prompt must be a string")
	}
	prompt := strings.TrimSpace(value.String())
	if prompt == "" {
		prompt = strings.TrimSpace(strings.Join(contentTexts, "\n"))
	}
	if prompt == "" {
		return "", fmt.Errorf("Mai Token Seedance prompt is required")
	}
	return prompt, nil
}

func maiTokenDurationSeconds(body []byte) (int, error) {
	value := gjson.GetBytes(body, "duration")
	if !value.Exists() {
		value = gjson.GetBytes(body, "seconds")
	}
	if !value.Exists() || value.Type == gjson.Null {
		return maiTokenMinDurationSeconds, nil
	}
	if value.Type != gjson.Number {
		return 0, fmt.Errorf("Mai Token Seedance duration must be an integer from 4 to 30 seconds")
	}
	rational, ok := new(big.Rat).SetString(value.Raw)
	if !ok || !rational.IsInt() || !rational.Num().IsInt64() {
		return 0, fmt.Errorf("Mai Token Seedance duration must be an integer from 4 to 30 seconds")
	}
	duration := rational.Num().Int64()
	if duration < maiTokenMinDurationSeconds || duration > maiTokenMaxDurationSeconds {
		return 0, fmt.Errorf("Mai Token Seedance duration must be an integer from 4 to 30 seconds")
	}
	return int(duration), nil
}

func maiTokenResolution(body []byte) (string, error) {
	value := gjson.GetBytes(body, "resolution")
	if !value.Exists() || value.Type == gjson.Null {
		return VideoBillingResolution480P, nil
	}
	if value.Type != gjson.String || strings.TrimSpace(value.String()) != VideoBillingResolution480P {
		return "", fmt.Errorf("Mai Token Seedance resolution must be 480p")
	}
	return VideoBillingResolution480P, nil
}

func maiTokenOptionalString(body []byte, paths ...string) (string, bool, error) {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.String {
			return "", false, fmt.Errorf("Mai Token Seedance %s must be a string", path)
		}
		return strings.TrimSpace(value.String()), true, nil
	}
	return "", false, nil
}

func maiTokenOptionalBool(body []byte, paths ...string) (bool, bool, error) {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		switch value.Type {
		case gjson.True:
			return true, true, nil
		case gjson.False:
			return false, true, nil
		default:
			return false, false, fmt.Errorf("Mai Token Seedance %s must be a boolean", path)
		}
	}
	return false, false, nil
}

func maiTokenReferencesFromRequest(body []byte) (maiTokenReferences, error) {
	var result maiTokenReferences
	referenceValue := gjson.GetBytes(body, "references")
	if referenceValue.Exists() && referenceValue.Type != gjson.Null && !referenceValue.IsArray() {
		return result, fmt.Errorf("Mai Token Seedance references must be an array")
	}
	for index, item := range referenceValue.Array() {
		mediaType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
		role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
		source := strings.TrimSpace(item.Get("source").String())
		if source == "" {
			return result, fmt.Errorf("Mai Token Seedance references[%d].source is required", index)
		}
		if err := validateMaiTokenHTTPURL(fmt.Sprintf("references[%d].source", index), source); err != nil {
			return result, err
		}
		switch role {
		case "reference_image":
			if mediaType != "image" {
				return result, fmt.Errorf("Mai Token Seedance references[%d] image role requires type image", index)
			}
			result.images = append(result.images, source)
		case "reference_video":
			if mediaType != "video" {
				return result, fmt.Errorf("Mai Token Seedance references[%d] video role requires type video", index)
			}
			result.videos = append(result.videos, source)
		case "reference_audio":
			if mediaType != "audio" {
				return result, fmt.Errorf("Mai Token Seedance references[%d] audio role requires type audio", index)
			}
			result.audios = append(result.audios, source)
		case "first_frame":
			if mediaType != "image" || result.first != "" {
				return result, fmt.Errorf("Mai Token Seedance accepts one image first frame")
			}
			result.first = source
		case "last_frame":
			if mediaType != "image" || result.last != "" {
				return result, fmt.Errorf("Mai Token Seedance accepts one image last frame")
			}
			result.last = source
		default:
			return result, fmt.Errorf("Mai Token Seedance references[%d].role is not supported", index)
		}
	}

	var err error
	if result.images, err = maiTokenAppendURLField(body, result.images,
		[]string{"referenceImages", "reference_images", "images", "imageUrls", "image_urls"},
		[]string{"image_url", "input_reference"}); err != nil {
		return result, err
	}
	if result.videos, err = maiTokenAppendURLField(body, result.videos,
		[]string{"referenceVideos", "reference_videos", "videos", "videoUrls", "video_urls"},
		[]string{"video_url"}); err != nil {
		return result, err
	}
	if result.audios, err = maiTokenAppendURLField(body, result.audios,
		[]string{"referenceAudios", "reference_audios", "audios", "audioUrls", "audio_urls"},
		[]string{"audio_url"}); err != nil {
		return result, err
	}

	first, err := maiTokenFrameString(body, "first frame", "first_image", "first_frame", "first_frame_url", "start_frame", "start_image_url")
	if err != nil {
		return result, err
	}
	last, err := maiTokenFrameString(body, "last frame", "last_image", "last_frame", "last_frame_url", "end_frame", "end_image_url")
	if err != nil {
		return result, err
	}
	if first != "" {
		if result.first != "" && result.first != first {
			return result, fmt.Errorf("Mai Token Seedance accepts only one first frame")
		}
		result.first = first
	}
	if last != "" {
		if result.last != "" && result.last != last {
			return result, fmt.Errorf("Mai Token Seedance accepts only one last frame")
		}
		result.last = last
	}

	if err := maiTokenApplyLegacyContent(body, &result); err != nil {
		return result, err
	}
	if result.last != "" && result.first == "" {
		return result, fmt.Errorf("Mai Token Seedance last_image requires first_image")
	}
	if result.first != "" && (len(result.images) > 0 || len(result.videos) > 0 || len(result.audios) > 0) {
		return result, fmt.Errorf("Mai Token Seedance cannot combine first/last frames with reference media")
	}
	if len(result.images) > maiTokenMaxReferenceImages {
		return result, fmt.Errorf("Mai Token Seedance accepts at most %d reference images", maiTokenMaxReferenceImages)
	}
	if len(result.videos) > maiTokenMaxReferenceVideos {
		return result, fmt.Errorf("Mai Token Seedance accepts at most %d reference videos", maiTokenMaxReferenceVideos)
	}
	if len(result.audios) > maiTokenMaxReferenceAudios {
		return result, fmt.Errorf("Mai Token Seedance accepts at most %d reference audios", maiTokenMaxReferenceAudios)
	}
	if len(result.images)+len(result.videos)+len(result.audios) > maiTokenMaxReferences {
		return result, fmt.Errorf("Mai Token Seedance accepts at most %d reference items", maiTokenMaxReferences)
	}
	return result, nil
}

func maiTokenAppendURLField(body []byte, dst []string, arrayPaths, singlePaths []string) ([]string, error) {
	for _, path := range arrayPaths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if !value.IsArray() {
			return nil, fmt.Errorf("Mai Token Seedance %s must be an array", path)
		}
		for index, item := range value.Array() {
			if item.Type != gjson.String || strings.TrimSpace(item.String()) == "" {
				return nil, fmt.Errorf("Mai Token Seedance %s[%d] must be a URL string", path, index)
			}
			rawURL := strings.TrimSpace(item.String())
			if err := validateMaiTokenHTTPURL(fmt.Sprintf("%s[%d]", path, index), rawURL); err != nil {
				return nil, err
			}
			dst = append(dst, rawURL)
		}
		break
	}
	for _, path := range singlePaths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.String || strings.TrimSpace(value.String()) == "" {
			return nil, fmt.Errorf("Mai Token Seedance %s must be a URL string", path)
		}
		rawURL := strings.TrimSpace(value.String())
		if err := validateMaiTokenHTTPURL(path, rawURL); err != nil {
			return nil, err
		}
		dst = append(dst, rawURL)
		break
	}
	return dst, nil
}

func maiTokenFrameString(body []byte, label string, paths ...string) (string, error) {
	var result string
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.String || strings.TrimSpace(value.String()) == "" {
			return "", fmt.Errorf("Mai Token Seedance %s must be a URL string", path)
		}
		rawURL := strings.TrimSpace(value.String())
		if err := validateMaiTokenHTTPURL(path, rawURL); err != nil {
			return "", err
		}
		if result != "" && result != rawURL {
			return "", fmt.Errorf("Mai Token Seedance accepts only one %s", label)
		}
		result = rawURL
	}
	return result, nil
}

func maiTokenApplyLegacyContent(body []byte, result *maiTokenReferences) error {
	content := gjson.GetBytes(body, "content")
	if !content.Exists() {
		return nil
	}
	if !content.IsArray() {
		return fmt.Errorf("Mai Token Seedance content must be an array")
	}
	for index, item := range content.Array() {
		itemType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
		role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
		switch itemType {
		case "text":
			text := strings.TrimSpace(item.Get("text").String())
			if text != "" {
				result.texts = append(result.texts, text)
			}
		case "image_url", "video_url", "audio_url":
			rawURL := strings.TrimSpace(item.Get(itemType).Get("url").String())
			if rawURL == "" {
				rawURL = strings.TrimSpace(item.Get(itemType).String())
			}
			if err := validateMaiTokenHTTPURL(fmt.Sprintf("content[%d].%s", index, itemType), rawURL); err != nil {
				return err
			}
			switch itemType {
			case "image_url":
				switch role {
				case "first_frame":
					if result.first != "" && result.first != rawURL {
						return fmt.Errorf("Mai Token Seedance accepts only one first frame")
					}
					result.first = rawURL
				case "last_frame":
					if result.last != "" && result.last != rawURL {
						return fmt.Errorf("Mai Token Seedance accepts only one last frame")
					}
					result.last = rawURL
				case "", "reference_image":
					result.images = append(result.images, rawURL)
				default:
					return fmt.Errorf("Mai Token Seedance content[%d].role is not supported", index)
				}
			case "video_url":
				if role != "" && role != "reference_video" {
					return fmt.Errorf("Mai Token Seedance content[%d].role is not supported", index)
				}
				result.videos = append(result.videos, rawURL)
			case "audio_url":
				if role != "" && role != "reference_audio" {
					return fmt.Errorf("Mai Token Seedance content[%d].role is not supported", index)
				}
				result.audios = append(result.audios, rawURL)
			}
		default:
			return fmt.Errorf("Mai Token Seedance content[%d].type is not supported", index)
		}
	}
	return nil
}

func validateMaiTokenHTTPURL(field, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("Mai Token Seedance %s must be a public HTTP(S) URL", field)
	}
	if isBlockedHostname(parsed.Hostname()) {
		return fmt.Errorf("Mai Token Seedance %s must be a public HTTP(S) URL", field)
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && (!ip.IsGlobalUnicast() || isPrivateIP(ip)) {
		return fmt.Errorf("Mai Token Seedance %s must be a public HTTP(S) URL", field)
	}
	return nil
}

func parseMaiTokenSeedanceResponse(body []byte) (PPVideoResponse, error) {
	status := strings.ToLower(strings.TrimSpace(extractPPVideoText(body, "status", "state")))
	result := PPVideoResponse{
		TaskID:                 extractPPVideoText(body, "id", "task_id"),
		ProviderBillingSeconds: maiTokenProviderBillingSeconds(body),
		GeneratedDurationMilliseconds: extractPPVideoDurationMilliseconds(body,
			"usage.output_video_duration", "usage.seconds"),
		InputVideoDurationMilliseconds: extractPPVideoDurationMilliseconds(body,
			"usage.reference_video_duration"),
		VideoCount: extractPPVideoPositiveInt(body, "usage.video_count"),
		VideoResolution: normalizePPVideoResolution(extractPPVideoText(body,
			"usage.resolution", "resolution")),
		ErrorMessage: extractPPVideoText(body, "error.message", "error_message", "message"),
		RawBody:      append([]byte(nil), body...),
	}
	switch status {
	case "queued", "in_progress":
		result.Status = PPVideoTaskStatusProcessing
	case "completed":
		if gjson.GetBytes(body, "billing_pending").Bool() || result.ProviderBillingSeconds <= 0 {
			result.Status = PPVideoTaskStatusProcessing
			logMaiTokenCompletedHeld(result.TaskID, body)
		} else {
			result.Status = PPVideoTaskStatusSucceeded
		}
	case "failed":
		result.Status = PPVideoTaskStatusFailed
	default:
		result.Status = NormalizePPVideoTaskStatus(status)
	}
	if result.VideoCount <= 0 {
		result.VideoCount = 1
	}
	return result, nil
}

// Reasons a finished upstream task is still reported to the customer as processing.
const (
	maiTokenHeldUpstreamBillingPending = "upstream_billing_pending"
	maiTokenHeldBillingSecondsMissing  = "billing_seconds_missing"
	maiTokenHeldBillingSecondsNotNum   = "billing_seconds_not_a_number"
	maiTokenHeldBillingSecondsNotInt   = "billing_seconds_not_an_integer"
	maiTokenHeldBillingSecondsNotPos   = "billing_seconds_not_positive"
)

// maiTokenCompletedHeldReason explains why a response whose status is "completed"
// is kept as processing. It mirrors the checks in parseMaiTokenSeedanceResponse
// and returns "" when the response is final.
func maiTokenCompletedHeldReason(body []byte) string {
	if gjson.GetBytes(body, "billing_pending").Bool() {
		return maiTokenHeldUpstreamBillingPending
	}
	value := gjson.GetBytes(body, "usage.billing_seconds")
	switch {
	case !value.Exists():
		return maiTokenHeldBillingSecondsMissing
	case value.Type != gjson.Number:
		return maiTokenHeldBillingSecondsNotNum
	}
	if maiTokenProviderBillingSeconds(body) > 0 {
		return ""
	}
	integer, ok := new(big.Int).SetString(strings.TrimSpace(value.Raw), 10)
	switch {
	case !ok:
		// 11.5, 11.0 and 1e1 are all numbers, but not the whole seconds the upstream documents.
		return maiTokenHeldBillingSecondsNotInt
	case integer.Sign() <= 0:
		return maiTokenHeldBillingSecondsNotPos
	default:
		// A positive integer that does not fit an int64.
		return maiTokenHeldBillingSecondsNotInt
	}
}

// logMaiTokenCompletedHeld records, for support, that the upstream reports the task
// as completed while the local task stays unsettled. Only the status flags and the
// small usage object are logged, never the response body, which can hold signed URLs.
func logMaiTokenCompletedHeld(taskID string, body []byte) {
	usage := gjson.GetBytes(body, "usage").Raw
	if len(usage) > 300 {
		usage = usage[:300]
	}
	logger.L().Warn("pp_video.mai_token_completed_but_held",
		zap.String("upstream_task_id", taskID),
		zap.String("reason", maiTokenCompletedHeldReason(body)),
		zap.Bool("upstream_billing_pending", gjson.GetBytes(body, "billing_pending").Bool()),
		zap.String("usage", usage),
	)
}

func maiTokenProviderBillingSeconds(body []byte) int64 {
	value := gjson.GetBytes(body, "usage.billing_seconds")
	if !value.Exists() || value.Type != gjson.Number {
		return 0
	}
	integer, ok := new(big.Int).SetString(value.Raw, 10)
	if !ok || integer.Sign() < 0 || !integer.IsInt64() {
		return 0
	}
	return integer.Int64()
}
