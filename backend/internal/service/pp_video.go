package service

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	PPVideoTaskStatusProcessing = "processing"
	PPVideoTaskStatusSucceeded  = "succeeded"
	PPVideoTaskStatusFailed     = "failed"

	PPVideoBillingFormulaPerSecond       = "per_second"
	PPVideoDefaultFrameRate              = 24.0
	SiteVideoDefaultModel                = "by-seedance2.0-933"
	SiteVideoDefaultDurationSeconds      = 5
	SiteVideoDefaultDurationMilliseconds = int64(SiteVideoDefaultDurationSeconds * 1000)
	SiteVideoDefaultResolution           = VideoBillingResolution720P
	ModelVerseVideoDefaultBaseURL        = "https://api.modelverse.cn/v1"
	ByteDanceVideoDefaultBaseURL         = ModelVerseVideoDefaultBaseURL
	ByteDanceVideoDefaultModel           = "doubao-seedance-2-0-260128"
	ByteDanceSeedance25Model             = "doubao-seedance-2-5-260628"
	ByteDanceSeedance25GlobalModel       = "doubao-seedance-2-5-260628-global"
	Wan30VideoDefaultModel               = "wan3.0-video"
	Wan30VideoPrimeModel                 = "wan3.0-video-prime"
	MiniMaxH3VideoDefaultModel           = "MiniMax-H3"
	MiniMaxHailuo23VideoModel            = "MiniMax-Hailuo-2.3"
	PixverseV6VideoDefaultModel          = "pixverse-v6"
	GrokImagineVideoDefaultModel         = "grok-imagine-video"
	KuaishouVideoDefaultModel            = "kling-v3"

	ppVideoLegacySiteModel             = "video-v1"
	ppVideoSeedanceDefaultModel        = ByteDanceVideoDefaultModel
	ppVideoHappyHorseTextDefaultModel  = "happyhorse-1.0-t2v"
	ppVideoHappyHorseImageDefaultModel = "happyhorse-1.0-i2v"
	ppVideoKlingDefaultModel           = "kling-v3"
	ppVideoPublicResponseObject        = "video.generation.task"
	ppVideoKlingPromptMaxRunes         = 2500
	ppVideoSeedancePromptMaxRunes      = 2000
)

type PPVideoOperation string

const (
	PPVideoOperationGeneric           PPVideoOperation = "generic"
	PPVideoOperationKlingTextToVideo  PPVideoOperation = "kling_text_to_video"
	PPVideoOperationKlingImageToVideo PPVideoOperation = "kling_image_to_video"
	PPVideoOperationCancel            PPVideoOperation = "cancel"
)

type PPVideoResponse struct {
	TaskID                         string
	Status                         string
	GeneratedDurationMilliseconds  int64
	VideoCount                     int
	VideoResolution                string
	OutputWidth                    int
	OutputHeight                   int
	FrameRate                      float64
	InputVideoDurationMilliseconds int64
	ErrorMessage                   string
	RawBody                        []byte
}

type PPVideoBillingMetadata struct {
	Platform                       string
	Model                          string
	RequestedDurationMilliseconds  int64
	RequestedDurationSeconds       int
	InputVideoDurationMilliseconds int64
	VideoCount                     int
	VideoResolution                string
	OutputWidth                    int
	OutputHeight                   int
	FrameRate                      float64
	HasAudio                       bool
	KlingMode                      string
	ValidationError                error
}

type PPVideoPublicRequest struct {
	Model                string
	UpstreamModel        string
	Prompt               string
	DurationMilliseconds int64
	VideoCount           int
	Resolution           string
	HasImage             bool
	HasExplicitModel     bool
}

func IsPPVideoPlatform(platform string) bool {
	switch strings.TrimSpace(platform) {
	case PlatformKling, PlatformHappyHourse, PlatformSeedance, PlatformByteDance, PlatformWan3, PlatformMiniMaxH3, PlatformMiniMaxH3CompShare, PlatformPixverseV6, PlatformGrokImagineVideo, PlatformKuaishou:
		return true
	default:
		return false
	}
}

func PreparePPVideoRequestBody(platform string, operation PPVideoOperation, body []byte) ([]byte, PPVideoPublicRequest, error) {
	return preparePPVideoRequestBody(platform, operation, body, nil)
}

func PreparePPVideoRequestBodyForAccount(account *Account, operation PPVideoOperation, body []byte) ([]byte, PPVideoPublicRequest, error) {
	if account == nil {
		return nil, PPVideoPublicRequest{}, fmt.Errorf("PP video account is required")
	}
	return preparePPVideoRequestBody(account.Platform, operation, body, account)
}

func preparePPVideoRequestBody(platform string, operation PPVideoOperation, body []byte, account *Account) ([]byte, PPVideoPublicRequest, error) {
	if !IsPPVideoPlatform(platform) {
		return nil, PPVideoPublicRequest{}, fmt.Errorf("unsupported PP video platform %q", platform)
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, PPVideoPublicRequest{}, fmt.Errorf("PP video request body must be valid JSON")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil || payload == nil {
		return nil, PPVideoPublicRequest{}, fmt.Errorf("PP video request body must be a JSON object")
	}

	requestedModel := PPVideoModelFromBody(body)
	public := PPVideoPublicRequest{
		Model:                normalizePPVideoPublicModel(requestedModel),
		Prompt:               extractPPVideoText(body, "prompt", "data.prompt", "parameters.prompt", "input.prompt"),
		DurationMilliseconds: normalizePPVideoPublicDurationMilliseconds(body),
		VideoCount:           normalizePPVideoPublicCount(body),
		Resolution:           normalizePPVideoPublicResolution(body),
		HasImage:             ppVideoRequestHasImage(body),
		HasExplicitModel:     strings.TrimSpace(requestedModel) != "",
	}
	if public.HasExplicitModel && ppVideoModelKnownForeignToPlatform(platform, public.Model) {
		return nil, PPVideoPublicRequest{}, fmt.Errorf(
			"model %q is not supported by platform %q",
			public.Model,
			platform,
		)
	}
	public.UpstreamModel = ppVideoUpstreamModelForAccount(platform, public, account)

	var err error
	switch platform {
	case PlatformKling:
		if err := normalizePPVideoKlingPayload(payload, operation, body, &public); err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformSeedance:
		if err := validateVideoPromptFieldLength("Seedance", "prompt", public.Prompt, ppVideoSeedancePromptMaxRunes); err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
		if err := normalizePPVideoSeedanceMediaFields(payload, body); err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
		payload["model"] = public.UpstreamModel
		payload["duration"] = ppVideoDurationJSONValue(public.DurationMilliseconds)
		payload["resolution"] = public.Resolution
		if public.VideoCount > 1 || !gjson.GetBytes(body, "n").Exists() {
			payload["n"] = public.VideoCount
		}
	case PlatformHappyHourse:
		payload["model"] = public.UpstreamModel
		payload["duration"] = ppVideoDurationJSONValue(public.DurationMilliseconds)
		payload["resolution"] = public.Resolution
		if public.VideoCount > 1 || !gjson.GetBytes(body, "n").Exists() {
			payload["n"] = public.VideoCount
		}
	case PlatformByteDance:
		if operation != PPVideoOperationGeneric {
			return nil, PPVideoPublicRequest{}, fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		if public.HasExplicitModel && !ppVideoByteDanceModelAllowed(public.Model) {
			return nil, PPVideoPublicRequest{}, fmt.Errorf(
				"model %q is not supported by platform %q",
				public.Model,
				platform,
			)
		}
		payload, err = normalizePPVideoByteDancePayload(payload, body, &public)
		if err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformWan3:
		if operation != PPVideoOperationGeneric {
			return nil, PPVideoPublicRequest{}, fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		if public.HasExplicitModel && !ppVideoWan30ModelAllowed(public.Model) {
			return nil, PPVideoPublicRequest{}, fmt.Errorf(
				"model %q is not supported by platform %q",
				public.Model,
				platform,
			)
		}
		payload, err = normalizePPVideoWan30Payload(payload, body, &public)
		if err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformMiniMaxH3CompShare:
		if operation != PPVideoOperationGeneric {
			return nil, PPVideoPublicRequest{}, fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		payload, err = normalizePPVideoCompSharePayload(body, &public)
		if err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformMiniMaxH3:
		if operation != PPVideoOperationGeneric {
			return nil, PPVideoPublicRequest{}, fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		if public.HasExplicitModel && !ppVideoMiniMaxH3ModelAllowedForAccount(public.Model, account) {
			return nil, PPVideoPublicRequest{}, fmt.Errorf(
				"model %q is not supported by platform %q",
				public.Model,
				platform,
			)
		}
		if ppVideoIsMiniMaxHailuo23Model(public.UpstreamModel) {
			payload, err = normalizePPVideoMiniMaxHailuo23Payload(payload, body, &public)
		} else {
			payload, err = normalizePPVideoMiniMaxH3Payload(payload, body, &public)
		}
		if err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformPixverseV6:
		if operation != PPVideoOperationGeneric {
			return nil, PPVideoPublicRequest{}, fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		if public.HasExplicitModel && !ppVideoPixverseV6ModelAllowed(public.Model) {
			return nil, PPVideoPublicRequest{}, fmt.Errorf(
				"model %q is not supported by platform %q",
				public.Model,
				platform,
			)
		}
		payload, err = normalizePPVideoPixverseV6Payload(payload, body, &public)
		if err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformGrokImagineVideo:
		if operation != PPVideoOperationGeneric {
			return nil, PPVideoPublicRequest{}, fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		if public.HasExplicitModel && !ppVideoGrokImagineVideoModelAllowed(public.Model) {
			return nil, PPVideoPublicRequest{}, fmt.Errorf(
				"model %q is not supported by platform %q",
				public.Model,
				platform,
			)
		}
		payload, err = normalizePPVideoGrokImaginePayload(payload, body, &public)
		if err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformKuaishou:
		if operation != PPVideoOperationGeneric {
			return nil, PPVideoPublicRequest{}, fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		if public.HasExplicitModel && !ppVideoKuaishouModelAllowed(public.Model) {
			return nil, PPVideoPublicRequest{}, fmt.Errorf(
				"model %q is not supported by platform %q",
				public.Model,
				platform,
			)
		}
		payload, err = normalizePPVideoKuaishouPayload(payload, body, &public)
		if err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	}

	prepared, err := json.Marshal(payload)
	if err != nil {
		return nil, PPVideoPublicRequest{}, fmt.Errorf("marshal PP video request body: %w", err)
	}
	return prepared, public, nil
}

func normalizePPVideoByteDancePayload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if payload == nil || public == nil {
		return nil, fmt.Errorf("ByteDance request body is required")
	}
	upstreamModel := byteDanceUpstreamModel(public.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = byteDanceUpstreamModel(public.Model)
	}
	if upstreamModel == "" {
		upstreamModel = ByteDanceVideoDefaultModel
	}
	maxDurationSeconds := byteDanceMaxDurationSeconds(upstreamModel)
	if public.DurationMilliseconds <= 0 || public.DurationMilliseconds%1000 != 0 {
		return nil, fmt.Errorf("ByteDance duration must be an integer number of seconds from 4 to %d", maxDurationSeconds)
	}
	durationSeconds := public.DurationMilliseconds / 1000
	if durationSeconds < 4 || durationSeconds > int64(maxDurationSeconds) {
		return nil, fmt.Errorf("ByteDance duration must be an integer number of seconds from 4 to %d", maxDurationSeconds)
	}

	content, err := normalizePPVideoByteDanceContent(body, public)
	if err != nil {
		return nil, err
	}

	parameters := make(map[string]any)
	for _, key := range []string{
		"execution_expires_after",
		"generate_audio",
		"resolution",
		"duration",
		"seed",
		"camera_fixed",
		"watermark",
		"callback_url",
		"seedance_tools",
		"omni_reference_task_type",
	} {
		if value, ok := ppVideoJSONValue(body, "parameters."+key, key); ok {
			parameters[key] = value
		}
	}
	if value, ok := ppVideoJSONValue(body, "parameters.ratio", "ratio", "parameters.aspect_ratio", "aspect_ratio"); ok {
		parameters["ratio"] = value
	}
	parameters["duration"] = int(durationSeconds)
	if resolution := byteDanceResolutionForModel(upstreamModel, public.Resolution); resolution != "" {
		parameters["resolution"] = resolution
	} else {
		if byteDanceIsSeedance25Model(upstreamModel) {
			return nil, fmt.Errorf("ByteDance resolution must be one of 480p, 720p, or 1080p")
		}
		return nil, fmt.Errorf("ByteDance resolution must be one of 480p, 720p, 1080p, or 4K")
	}
	if _, ok := parameters["ratio"]; !ok {
		parameters["ratio"] = "adaptive"
	}
	if _, ok := parameters["generate_audio"]; !ok {
		parameters["generate_audio"] = true
	}

	result := map[string]any{
		"model":      upstreamModel,
		"input":      map[string]any{"content": content},
		"parameters": parameters,
	}
	public.UpstreamModel = upstreamModel
	return result, nil
}

func normalizePPVideoByteDanceContent(body []byte, public *PPVideoPublicRequest) ([]any, error) {
	rawContent := gjson.GetBytes(body, "input.content")
	if rawContent.Exists() {
		if !rawContent.IsArray() || len(rawContent.Array()) == 0 {
			return nil, fmt.Errorf("ByteDance input.content must contain at least one item")
		}
		content := make([]any, 0, len(rawContent.Array()))
		for _, item := range rawContent.Array() {
			var source map[string]any
			if err := json.Unmarshal([]byte(item.Raw), &source); err != nil || source == nil {
				return nil, fmt.Errorf("ByteDance input.content items must be JSON objects")
			}
			normalized, err := normalizePPVideoByteDanceContentItem(source)
			if err != nil {
				return nil, err
			}
			if normalized["type"] == "text" && public.Prompt == "" {
				if text, ok := normalized["text"].(string); ok {
					public.Prompt = strings.TrimSpace(text)
				}
			}
			if normalized["type"] == "image_url" {
				public.HasImage = true
			}
			content = append(content, normalized)
		}
		return content, nil
	}

	content := make([]any, 0, 4)
	if prompt := strings.TrimSpace(public.Prompt); prompt != "" {
		content = append(content, map[string]any{"type": "text", "text": prompt})
	}
	if startFrame := ppVideoJSONText(body, "start_frame_url", "input.start_frame_url", "data.start_frame_url"); startFrame != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": startFrame},
			"role":      "first_frame",
		})
		public.HasImage = true
	}
	if endFrame := ppVideoJSONText(body, "end_frame_url", "input.end_frame_url", "data.end_frame_url"); endFrame != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": endFrame},
			"role":      "last_frame",
		})
		public.HasImage = true
	}
	if image := ppVideoJSONText(body, "image", "image_url", "input.image", "input.image_url", "images.0", "data.images.0"); image != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": image},
			"role":      "first_frame",
		})
		public.HasImage = true
	}
	if referenceImage := ppVideoJSONText(body, "reference_image_url", "input.reference_image_url", "data.reference_image_url"); referenceImage != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": referenceImage},
			"role":      "reference_image",
		})
		public.HasImage = true
	}
	if assetURL := ppVideoByteDanceAssetURL(body); assetURL != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": assetURL},
			"role":      "reference_image",
		})
		public.HasImage = true
	}
	if video := ppVideoJSONText(body, "video", "video_url", "videos.0", "input.video", "input.video_url"); video != "" {
		content = append(content, map[string]any{
			"type":      "video_url",
			"video_url": map[string]any{"url": video},
			"role":      "reference_video",
		})
	}
	if audio := ppVideoJSONText(body, "audio_url", "audio", "audios.0", "input.audio_url", "input.audio"); audio != "" {
		content = append(content, map[string]any{
			"type":      "audio_url",
			"audio_url": map[string]any{"url": audio},
			"role":      "reference_audio",
		})
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("ByteDance request requires prompt, input.content, or supported media input")
	}
	return content, nil
}

func normalizePPVideoByteDanceContentItem(source map[string]any) (map[string]any, error) {
	contentType, _ := source["type"].(string)
	contentType = strings.TrimSpace(contentType)
	if contentType != "text" && contentType != "image_url" && contentType != "video_url" && contentType != "audio_url" {
		return nil, fmt.Errorf("ByteDance content type %q is not supported", contentType)
	}
	result := map[string]any{"type": contentType}
	switch contentType {
	case "text":
		text, _ := source["text"].(string)
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("ByteDance text content must not be empty")
		}
		result["text"] = text
	case "image_url":
		value, ok := source["image_url"]
		if !ok {
			if assetURL := ppVideoByteDanceAssetURLFromValue(source["asset_id"]); assetURL != "" {
				value = map[string]any{"url": assetURL}
			} else {
				return nil, fmt.Errorf("ByteDance image_url content requires image_url")
			}
		}
		normalized, err := normalizePPVideoByteDanceURLObject(value)
		if err != nil {
			return nil, fmt.Errorf("ByteDance image_url content: %w", err)
		}
		result["image_url"] = normalized
		role, _ := source["role"].(string)
		role = strings.TrimSpace(role)
		if role == "" {
			role = "first_frame"
		}
		if role != "first_frame" && role != "last_frame" && role != "reference_image" {
			return nil, fmt.Errorf("ByteDance image role %q is not supported", role)
		}
		result["role"] = role
	case "video_url":
		value, ok := source["video_url"]
		if !ok {
			return nil, fmt.Errorf("ByteDance video_url content requires video_url")
		}
		normalized, err := normalizePPVideoByteDanceURLObject(value)
		if err != nil {
			return nil, fmt.Errorf("ByteDance video_url content: %w", err)
		}
		result["video_url"] = normalized
		result["role"] = "reference_video"
	case "audio_url":
		value, ok := source["audio_url"]
		if !ok {
			return nil, fmt.Errorf("ByteDance audio_url content requires audio_url")
		}
		normalized, err := normalizePPVideoByteDanceURLObject(value)
		if err != nil {
			return nil, fmt.Errorf("ByteDance audio_url content: %w", err)
		}
		result["audio_url"] = normalized
		result["role"] = "reference_audio"
	}
	return result, nil
}

func normalizePPVideoByteDanceURLObject(value any) (map[string]any, error) {
	if rawURL, ok := value.(string); ok {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			return nil, fmt.Errorf("url must not be empty")
		}
		return map[string]any{"url": rawURL}, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("url object is invalid")
	}
	rawURL, _ := object["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		rawURL = ppVideoByteDanceAssetURLFromValue(object["asset_id"])
	}
	if rawURL == "" {
		return nil, fmt.Errorf("url must not be empty")
	}
	return map[string]any{"url": rawURL}, nil
}

func ppVideoByteDanceAssetURL(body []byte) string {
	for _, path := range []string{
		"asset_id",
		"image_asset_id",
		"reference_asset_id",
		"input.asset_id",
		"input.image_asset_id",
		"input.reference_asset_id",
		"assets.0",
		"asset_ids.0",
		"input.assets.0",
		"input.asset_ids.0",
	} {
		if assetURL := ppVideoByteDanceAssetURLFromValue(gjson.GetBytes(body, path).Value()); assetURL != "" {
			return assetURL
		}
	}
	return ""
}

func ppVideoByteDanceAssetURLFromValue(value any) string {
	raw, _ := value.(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "asset://") {
		return raw
	}
	return "asset://" + raw
}

func normalizePPVideoWan30Payload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if payload == nil || public == nil {
		return nil, fmt.Errorf("Wan3.0 request body is required")
	}
	if public.DurationMilliseconds <= 0 || public.DurationMilliseconds%1000 != 0 {
		return nil, fmt.Errorf("Wan3.0 duration must be an integer number of seconds from 2 to 30")
	}
	durationSeconds := public.DurationMilliseconds / 1000
	if durationSeconds < 2 || durationSeconds > 30 {
		return nil, fmt.Errorf("Wan3.0 duration must be an integer number of seconds from 2 to 30")
	}

	prompt := ppVideoJSONText(body, "input.prompt", "prompt", "data.prompt", "parameters.prompt")
	if prompt == "" {
		return nil, fmt.Errorf("Wan3.0 input.prompt is required")
	}
	public.Prompt = prompt

	resolutionSource := ppVideoJSONText(
		body,
		"parameters.resolution",
		"resolution",
		"data.resolution",
		"metadata.resolution",
	)
	resolution := "1080P"
	if resolutionSource != "" {
		resolution = wan30Resolution(resolutionSource)
		if resolution == "" {
			return nil, fmt.Errorf("Wan3.0 resolution must be 480P, 720P, or 1080P")
		}
	}

	ratio, err := wan30RatioFromRequest(body)
	if err != nil {
		return nil, err
	}
	if ratio == "" {
		ratio = "adaptive"
	}
	if !wan30RatioAllowed(ratio) {
		return nil, fmt.Errorf("Wan3.0 ratio is not supported")
	}

	audio, err := wan30BooleanParameter(body, "audio", true)
	if err != nil {
		return nil, err
	}
	promptExtend, err := wan30BooleanParameter(body, "prompt_extend", true)
	if err != nil {
		return nil, err
	}
	watermark, err := wan30BooleanParameter(body, "watermark", false)
	if err != nil {
		return nil, err
	}
	seed, hasSeed, err := wan30SeedFromRequest(body)
	if err != nil {
		return nil, err
	}

	media, err := normalizePPVideoWan30Media(body)
	if err != nil {
		return nil, err
	}

	upstreamModel := wan30UpstreamModel(public.Model)
	public.UpstreamModel = upstreamModel
	public.Resolution = normalizePPVideoResolution(resolution)
	public.VideoCount = 1

	input := map[string]any{"prompt": prompt}
	if len(media) > 0 {
		input["media"] = media
	}
	parameters := map[string]any{
		"resolution":    resolution,
		"duration":      int(durationSeconds),
		"ratio":         ratio,
		"audio":         audio,
		"prompt_extend": promptExtend,
		"watermark":     watermark,
	}
	if hasSeed {
		parameters["seed"] = seed
	}

	return map[string]any{
		"model":      upstreamModel,
		"input":      input,
		"parameters": parameters,
	}, nil
}

func normalizePPVideoWan30Media(body []byte) ([]any, error) {
	rawMedia := gjson.GetBytes(body, "input.media")
	if !rawMedia.Exists() {
		rawMedia = gjson.GetBytes(body, "media")
	}

	media := make([]any, 0, 4)
	if rawMedia.Exists() {
		if !rawMedia.IsArray() {
			return nil, fmt.Errorf("Wan3.0 input.media must be an array")
		}
		for _, item := range rawMedia.Array() {
			var source map[string]any
			if err := json.Unmarshal([]byte(item.Raw), &source); err != nil || source == nil {
				return nil, fmt.Errorf("Wan3.0 input.media items must be JSON objects")
			}
			media = append(media, source)
		}
	} else {
		appendMedia := func(mediaType, rawURL string) {
			if rawURL == "" {
				return
			}
			media = append(media, map[string]any{"type": mediaType, "url": rawURL})
		}
		appendMedia("first_frame", ppVideoJSONText(body, "start_frame_url", "input.start_frame_url", "data.start_frame_url"))
		appendMedia("last_frame", ppVideoJSONText(body, "end_frame_url", "input.end_frame_url", "data.end_frame_url"))
		appendMedia("first_frame", ppVideoJSONText(body, "image", "image_url", "input.image", "input.image_url"))
		for _, item := range gjson.GetBytes(body, "images").Array() {
			appendMedia("reference_image", strings.TrimSpace(item.String()))
		}
		appendMedia("reference_video", ppVideoJSONText(body, "video", "video_url", "input.video", "input.video_url"))
		appendMedia("reference_audio", ppVideoJSONText(body, "audio_url", "audio", "input.audio_url", "input.audio"))
		appendMedia("file", ppVideoJSONText(body, "file_url", "input.file_url"))
		appendMedia("link", ppVideoJSONText(body, "link", "link_url", "input.link", "input.link_url"))
	}

	firstFrameCount := 0
	lastFrameCount := 0
	referenceImageCount := 0
	referenceVideoCount := 0
	referenceAudioCount := 0
	fileCount := 0
	linkCount := 0
	hasFrameMedia := false
	hasReferenceFileOrLink := false
	normalizedMedia := make([]any, 0, len(media))
	for _, item := range media {
		source, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Wan3.0 input.media items must be JSON objects")
		}
		normalized, err := normalizePPVideoWan30MediaItem(source)
		if err != nil {
			return nil, err
		}
		switch normalized["type"] {
		case "first_frame":
			firstFrameCount++
			hasFrameMedia = true
		case "last_frame":
			lastFrameCount++
			hasFrameMedia = true
		case "reference_image":
			referenceImageCount++
			hasReferenceFileOrLink = true
		case "reference_video":
			referenceVideoCount++
			hasReferenceFileOrLink = true
		case "reference_audio":
			referenceAudioCount++
			hasReferenceFileOrLink = true
		case "file":
			fileCount++
			hasReferenceFileOrLink = true
		case "link":
			linkCount++
			hasReferenceFileOrLink = true
		}
		normalizedMedia = append(normalizedMedia, normalized)
	}

	if firstFrameCount > 1 || lastFrameCount > 1 {
		return nil, fmt.Errorf("Wan3.0 supports at most one first_frame and one last_frame")
	}
	if referenceImageCount > 10 || referenceVideoCount > 5 || referenceAudioCount > 5 {
		return nil, fmt.Errorf("Wan3.0 input.media exceeds the supported reference-media limit")
	}
	if fileCount > 1 || linkCount > 1 || (fileCount > 0 && linkCount > 0) {
		return nil, fmt.Errorf("Wan3.0 supports at most one file or one link, but not both")
	}
	if hasFrameMedia && hasReferenceFileOrLink {
		return nil, fmt.Errorf("Wan3.0 cannot combine first/last frames with reference media, files, or links")
	}
	return normalizedMedia, nil
}

func normalizePPVideoWan30MediaItem(source map[string]any) (map[string]any, error) {
	mediaType, _ := source["type"].(string)
	mediaType = strings.TrimSpace(mediaType)
	switch mediaType {
	case "first_frame", "last_frame", "reference_image", "reference_video", "reference_audio", "file", "link":
	default:
		return nil, fmt.Errorf("Wan3.0 media type %q is not supported", mediaType)
	}
	rawURL, _ := source["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("Wan3.0 %s media requires url", mediaType)
	}
	if !wan30MediaURLAllowed(mediaType, rawURL) {
		return nil, fmt.Errorf("Wan3.0 %s media URL is not supported", mediaType)
	}
	return map[string]any{"type": mediaType, "url": rawURL}, nil
}

func wan30MediaURLAllowed(mediaType, rawURL string) bool {
	lowerURL := strings.ToLower(strings.TrimSpace(rawURL))
	if strings.HasPrefix(lowerURL, "data:image/") {
		return (mediaType == "first_frame" || mediaType == "last_frame" || mediaType == "reference_image") &&
			strings.Contains(lowerURL, ";base64,")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Host) == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch mediaType {
	case "first_frame", "last_frame", "reference_image":
		return scheme == "http" || scheme == "https" || scheme == "oss"
	case "reference_video", "reference_audio", "file":
		return scheme == "http" || scheme == "https" || scheme == "oss"
	case "link":
		return scheme == "http" || scheme == "https"
	default:
		return false
	}
}

func wan30Resolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "480", "480p", "sd":
		return "480P"
	case "720", "720p", "hd":
		return "720P"
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
		return "1080P"
	default:
		return ""
	}
}

func wan30RatioFromRequest(body []byte) (string, error) {
	value, ok := ppVideoJSONValue(body, "parameters.ratio", "ratio", "parameters.aspect_ratio", "aspect_ratio")
	if !ok {
		return "", nil
	}
	ratio, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("Wan3.0 ratio must be a string")
	}
	return strings.TrimSpace(ratio), nil
}

func wan30RatioAllowed(ratio string) bool {
	switch strings.TrimSpace(ratio) {
	case "adaptive", "16:9", "9:16", "1:1", "4:3", "3:4":
		return true
	default:
		return false
	}
}

func wan30BooleanParameter(body []byte, field string, defaultValue bool) (bool, error) {
	value, ok := ppVideoJSONValue(body, "parameters."+field, field)
	if !ok {
		return defaultValue, nil
	}
	parsed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("Wan3.0 %s must be boolean", field)
	}
	return parsed, nil
}

func wan30SeedFromRequest(body []byte) (int64, bool, error) {
	value, ok := ppVideoJSONValue(body, "parameters.seed", "seed")
	if !ok {
		return 0, false, nil
	}
	seed, ok := value.(float64)
	if !ok || seed != float64(int64(seed)) || seed < 0 || seed > 2147483647 {
		return 0, false, fmt.Errorf("Wan3.0 seed must be an integer from 0 to 2147483647")
	}
	return int64(seed), true, nil
}

func normalizePPVideoPixverseV6Payload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if payload == nil || public == nil {
		return nil, fmt.Errorf("Pixverse v6 request body is required")
	}
	if public.DurationMilliseconds <= 0 || public.DurationMilliseconds%1000 != 0 {
		return nil, fmt.Errorf("Pixverse v6 duration must be an integer number of seconds from 1 to 15")
	}
	durationSeconds := public.DurationMilliseconds / 1000
	if durationSeconds < 1 || durationSeconds > 15 {
		return nil, fmt.Errorf("Pixverse v6 duration must be an integer number of seconds from 1 to 15")
	}

	prompt := ppVideoJSONText(body, "input.prompt", "prompt", "data.prompt", "parameters.prompt")
	if prompt == "" {
		return nil, fmt.Errorf("Pixverse v6 input.prompt is required")
	}
	public.Prompt = prompt
	public.UpstreamModel = PixverseV6VideoDefaultModel
	public.VideoCount = 1

	resolutionSource := ppVideoJSONText(body, "parameters.resolution", "resolution", "data.resolution", "metadata.resolution")
	resolution := "720p"
	if resolutionSource != "" {
		resolution = pixverseV6Resolution(resolutionSource)
		if resolution == "" {
			return nil, fmt.Errorf("Pixverse v6 resolution must be 360p, 540p, 720p, or 1080p")
		}
	}
	public.Resolution = pixverseV6BillingResolution(resolution)

	firstFrame := ppVideoJSONText(body, "input.first_frame_url", "first_frame_url", "input.start_frame_url", "start_frame_url", "data.first_frame_url", "data.start_frame_url")
	lastFrame := ppVideoJSONText(body, "input.last_frame_url", "last_frame_url", "input.end_frame_url", "end_frame_url", "data.last_frame_url", "data.end_frame_url")
	if (firstFrame == "") != (lastFrame == "") {
		return nil, fmt.Errorf("Pixverse v6 first_frame_url and last_frame_url must be provided together")
	}
	if firstFrame != "" && (!pixverseV6ImageURLAllowed(firstFrame) || !pixverseV6ImageURLAllowed(lastFrame)) {
		return nil, fmt.Errorf("Pixverse v6 first_frame_url and last_frame_url must be image URLs or Base64 values")
	}

	image := ppVideoJSONText(body, "input.img_url", "img_url", "input.image_url", "image_url", "input.image", "image", "images.0", "data.img_url", "data.image_url")
	if image != "" && !pixverseV6ImageURLAllowed(image) {
		return nil, fmt.Errorf("Pixverse v6 img_url must be an image URL or Base64 value")
	}
	video := ppVideoJSONText(body, "input.video_url", "video_url", "input.video", "video", "videos.0", "data.video_url")
	if video != "" && !pixverseV6VideoURLAllowed(video) {
		return nil, fmt.Errorf("Pixverse v6 video_url must be an HTTP(S) URL")
	}

	hasReferenceMedia := firstFrame != "" || image != "" || video != ""
	public.HasImage = firstFrame != "" || image != ""

	parameters := map[string]any{
		"resolution":     resolution,
		"duration":       int(durationSeconds),
		"generate_audio": true,
	}
	aspectRatio, hasAspectRatio, err := pixverseV6AspectRatioFromRequest(body)
	if err != nil {
		return nil, err
	}
	if hasAspectRatio && !hasReferenceMedia {
		parameters["aspect_ratio"] = aspectRatio
	}
	if generateAudio, hasGenerateAudio, err := pixverseV6GenerateAudioFromRequest(body); err != nil {
		return nil, err
	} else if hasGenerateAudio {
		parameters["generate_audio"] = generateAudio
	}
	if seed, hasSeed, err := pixverseV6IntegerParameter(body, "seed"); err != nil {
		return nil, err
	} else if hasSeed {
		if seed < 0 || seed > 2147483647 {
			return nil, fmt.Errorf("Pixverse v6 seed must be an integer from 0 to 2147483647")
		}
		parameters["seed"] = seed
	}

	input := map[string]any{"prompt": prompt}
	if firstFrame != "" {
		input["first_frame_url"] = firstFrame
		input["last_frame_url"] = lastFrame
	}
	if image != "" {
		input["img_url"] = image
	}
	if video != "" {
		input["video_url"] = video
	}
	return map[string]any{
		"model":      PixverseV6VideoDefaultModel,
		"input":      input,
		"parameters": parameters,
	}, nil
}

func normalizePPVideoGrokImaginePayload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if payload == nil || public == nil {
		return nil, fmt.Errorf("Grok Imagine Video request body is required")
	}

	durationMilliseconds, durationFound := extractPPVideoDurationMillisecondsWithPresence(
		body,
		"parameters.duration",
		"duration",
		"data.duration",
	)
	if !durationFound {
		durationMilliseconds = 8000
	}
	if durationMilliseconds <= 0 || durationMilliseconds%1000 != 0 || durationMilliseconds < 1000 || durationMilliseconds > 15000 {
		return nil, fmt.Errorf("Grok Imagine Video duration must be an integer number of seconds from 1 to 15")
	}

	prompt := ppVideoJSONText(body, "input.prompt", "prompt", "data.prompt", "parameters.prompt")
	if prompt == "" {
		return nil, fmt.Errorf("Grok Imagine Video input.prompt is required")
	}

	resolution := "480p"
	if resolutionSource := ppVideoJSONText(body, "parameters.resolution", "resolution", "data.resolution"); resolutionSource != "" {
		switch strings.ToLower(strings.TrimSpace(resolutionSource)) {
		case "480", "480p":
			resolution = "480p"
		case "720", "720p":
			resolution = "720p"
		default:
			return nil, fmt.Errorf("Grok Imagine Video resolution must be 480p or 720p")
		}
	}

	aspectRatio := "16:9"
	if aspectRatioSource := ppVideoJSONText(body, "parameters.aspect_ratio", "aspect_ratio", "data.aspect_ratio"); aspectRatioSource != "" {
		switch strings.TrimSpace(aspectRatioSource) {
		case "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3":
			aspectRatio = strings.TrimSpace(aspectRatioSource)
		default:
			return nil, fmt.Errorf("Grok Imagine Video aspect_ratio is invalid")
		}
	}

	imageURL := ppVideoJSONText(body,
		"input.img_url",
		"img_url",
		"data.img_url",
	)
	if imageURL != "" && !grokImagineVideoURLAllowed(imageURL) {
		return nil, fmt.Errorf("Grok Imagine Video input.img_url must be an HTTP(S) URL")
	}
	referenceURLs, referencesFound, err := grokImagineVideoReferenceURLs(body)
	if err != nil {
		return nil, err
	}
	if imageURL != "" && len(referenceURLs) > 0 {
		return nil, fmt.Errorf("Grok Imagine Video input.img_url and input.reference_urls are mutually exclusive")
	}
	if referencesFound && len(referenceURLs) == 0 {
		return nil, fmt.Errorf("Grok Imagine Video input.reference_urls must contain at least one URL")
	}
	if imageURL == "" && len(referenceURLs) == 0 {
		return nil, fmt.Errorf("Grok Imagine Video requires input.img_url or input.reference_urls; text-to-video is not supported")
	}

	public.Prompt = prompt
	public.DurationMilliseconds = durationMilliseconds
	public.VideoCount = 1
	public.Resolution = normalizePPVideoResolution(resolution)
	public.HasImage = imageURL != "" || len(referenceURLs) > 0
	public.UpstreamModel = GrokImagineVideoDefaultModel

	input := map[string]any{"prompt": prompt}
	if imageURL != "" {
		input["img_url"] = imageURL
	} else if len(referenceURLs) > 0 {
		input["reference_urls"] = referenceURLs
	}
	return map[string]any{
		"model": GrokImagineVideoDefaultModel,
		"input": input,
		"parameters": map[string]any{
			"resolution":   resolution,
			"aspect_ratio": aspectRatio,
			"duration":     int(durationMilliseconds / 1000),
		},
	}, nil
}

func grokImagineVideoReferenceURLs(body []byte) ([]string, bool, error) {
	for _, path := range []string{"input.reference_urls", "reference_urls", "data.reference_urls"} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if !value.IsArray() {
			return nil, true, fmt.Errorf("Grok Imagine Video input.reference_urls must be an array")
		}
		urls := make([]string, 0, len(value.Array()))
		for _, item := range value.Array() {
			candidate := strings.TrimSpace(item.String())
			if candidate == "" || !grokImagineVideoURLAllowed(candidate) {
				return nil, true, fmt.Errorf("Grok Imagine Video input.reference_urls must contain only HTTP(S) URLs")
			}
			urls = append(urls, candidate)
		}
		return urls, true, nil
	}
	return nil, false, nil
}

func grokImagineVideoURLAllowed(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed != nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func normalizePPVideoKuaishouPayload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if payload == nil || public == nil {
		return nil, fmt.Errorf("Kuaishou request body is required")
	}

	durationMilliseconds, durationFound := extractPPVideoDurationMillisecondsWithPresence(
		body,
		"parameters.duration",
		"duration",
		"data.duration",
	)
	if !durationFound {
		durationMilliseconds = SiteVideoDefaultDurationMilliseconds
	}
	if durationMilliseconds <= 0 || durationMilliseconds%1000 != 0 {
		return nil, fmt.Errorf("Kuaishou duration must be an integer number of seconds from 3 to 15")
	}
	durationSeconds := durationMilliseconds / 1000
	if durationSeconds < 3 || durationSeconds > 15 {
		return nil, fmt.Errorf("Kuaishou duration must be an integer number of seconds from 3 to 15")
	}

	input, err := normalizePPVideoKuaishouInput(body, public)
	if err != nil {
		return nil, err
	}

	parameters := map[string]any{
		"duration": int(durationSeconds),
		"mode":     "std",
	}
	if modeSource := ppVideoJSONText(body, "parameters.mode", "mode", "data.mode"); modeSource != "" {
		mode, ok := normalizePPVideoKlingMode(modeSource)
		if !ok {
			return nil, fmt.Errorf("Kuaishou mode must be std, pro, or 4k")
		}
		parameters["mode"] = mode
	}
	if ratio := ppVideoJSONText(body, "parameters.aspect_ratio", "aspect_ratio", "data.aspect_ratio"); ratio != "" {
		ratio = strings.TrimSpace(ratio)
		if !kuaishouAspectRatioAllowed(ratio) {
			return nil, fmt.Errorf("Kuaishou aspect_ratio must be 16:9, 9:16, or 1:1")
		}
		parameters["aspect_ratio"] = ratio
	} else {
		parameters["aspect_ratio"] = "16:9"
	}
	if value, ok := ppVideoJSONValue(body, "parameters.watermark_enabled", "watermark_enabled", "data.watermark_enabled"); ok {
		parsed, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("Kuaishou watermark_enabled must be boolean")
		}
		parameters["watermark_enabled"] = parsed
	}
	if externalTaskID := ppVideoJSONText(body, "parameters.external_task_id", "external_task_id", "data.external_task_id"); externalTaskID != "" {
		parameters["external_task_id"] = externalTaskID
	}
	if sound, ok, err := ppVideoKlingSoundFromRequest(body); err != nil {
		return nil, fmt.Errorf("Kuaishou sound must be on or off")
	} else if ok {
		parameters["sound"] = sound
	}
	if keepOriginalSound := ppVideoJSONText(body, "parameters.keep_original_sound", "keep_original_sound", "data.keep_original_sound"); keepOriginalSound != "" {
		keepOriginalSound = strings.ToLower(strings.TrimSpace(keepOriginalSound))
		if keepOriginalSound != "yes" && keepOriginalSound != "no" {
			return nil, fmt.Errorf("Kuaishou keep_original_sound must be yes or no")
		}
		parameters["keep_original_sound"] = keepOriginalSound
	}
	if orientation := ppVideoJSONText(body, "parameters.character_orientation", "character_orientation", "data.character_orientation"); orientation != "" {
		orientation = strings.ToLower(strings.TrimSpace(orientation))
		if orientation != "image" && orientation != "video" {
			return nil, fmt.Errorf("Kuaishou character_orientation must be image or video")
		}
		parameters["character_orientation"] = orientation
	}
	if image := ppVideoKlingRequestImage(body); image != "" {
		parameters["image"] = image
	}
	if imageTail := ppVideoKlingRequestImageTail(body); imageTail != "" {
		if _, hasImage := parameters["image"]; !hasImage {
			return nil, fmt.Errorf("Kuaishou image_tail requires image")
		}
		parameters["image_tail"] = imageTail
	}
	if multiShot, ok, err := kuaishouBooleanParameter(body, "multi_shot", false); err != nil {
		return nil, err
	} else if ok {
		parameters["multi_shot"] = multiShot
	}
	if shotType := ppVideoJSONText(body, "parameters.shot_type", "shot_type", "data.shot_type"); shotType != "" {
		if strings.TrimSpace(shotType) != "customize" {
			return nil, fmt.Errorf("Kuaishou shot_type must be customize")
		}
		parameters["shot_type"] = "customize"
	}
	if multiPrompt, ok := ppVideoJSONValue(body, "parameters.multi_prompt", "multi_prompt", "data.multi_prompt"); ok {
		if _, ok := multiPrompt.([]any); !ok {
			return nil, fmt.Errorf("Kuaishou multi_prompt must be an array")
		}
		parameters["multi_prompt"] = multiPrompt
	}

	klingType := strings.TrimSpace(ppVideoJSONText(body, "parameters.kling_v3_type", "kling_v3_type", "data.kling_v3_type"))
	if klingType == "" {
		switch {
		case ppVideoJSONText(body, "input.video_url", "video_url", "videos.0", "data.video_url") != "":
			klingType = "motion_control"
		case public.HasImage || parameters["image"] != nil:
			klingType = "i2v"
		default:
			klingType = "t2v"
		}
	}
	if !kuaishouTypeAllowed(klingType) {
		return nil, fmt.Errorf("Kuaishou kling_v3_type must be t2v, i2v, or motion_control")
	}
	parameters["kling_v3_type"] = klingType
	if klingType == "motion_control" {
		if ppVideoJSONText(body, "input.img_url", "img_url", "data.img_url") == "" || ppVideoJSONText(body, "input.video_url", "video_url", "videos.0", "data.video_url") == "" {
			return nil, fmt.Errorf("Kuaishou motion_control requires input.img_url and input.video_url")
		}
		if _, ok := parameters["character_orientation"]; !ok {
			parameters["character_orientation"] = "image"
		}
	}

	public.DurationMilliseconds = durationMilliseconds
	public.VideoCount = 1
	public.Resolution = kuaishouBillingResolution(fmt.Sprint(parameters["mode"]))
	public.UpstreamModel = KuaishouVideoDefaultModel

	return map[string]any{
		"model":      KuaishouVideoDefaultModel,
		"input":      input,
		"parameters": parameters,
	}, nil
}

func normalizePPVideoKuaishouInput(body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	prompt := strings.TrimSpace(ppVideoJSONText(body, "input.prompt", "prompt", "data.prompt", "parameters.prompt"))
	negativePrompt := strings.TrimSpace(ppVideoJSONText(body, "input.negative_prompt", "negative_prompt", "data.negative_prompt", "parameters.negative_prompt"))
	if err := validateVideoPromptFieldLength("Kuaishou", "prompt", prompt, ppVideoKlingPromptMaxRunes); err != nil {
		return nil, err
	}
	if err := validateVideoPromptFieldLength("Kuaishou", "negative_prompt", negativePrompt, ppVideoKlingPromptMaxRunes); err != nil {
		return nil, err
	}

	input := map[string]any{}
	if prompt != "" {
		input["prompt"] = prompt
		public.Prompt = prompt
	}
	if negativePrompt != "" {
		input["negative_prompt"] = negativePrompt
	}
	if imgURL := ppVideoJSONText(body, "input.img_url", "img_url", "data.img_url"); imgURL != "" {
		input["img_url"] = imgURL
		public.HasImage = true
	}
	if videoURL := ppVideoJSONText(body, "input.video_url", "video_url", "videos.0", "data.video_url"); videoURL != "" {
		input["video_url"] = videoURL
	}
	if firstFrame := ppVideoJSONText(body, "input.first_frame_url", "first_frame_url", "start_frame_url", "data.first_frame_url", "data.start_frame_url"); firstFrame != "" {
		input["first_frame_url"] = firstFrame
		public.HasImage = true
	}
	if images, ok := ppVideoJSONValue(body, "input.images", "images", "data.images"); ok {
		if _, ok := images.([]any); !ok {
			return nil, fmt.Errorf("Kuaishou images must be an array")
		}
		input["images"] = images
		public.HasImage = true
	}
	return input, nil
}

func kuaishouBooleanParameter(body []byte, field string, defaultValue bool) (bool, bool, error) {
	value, ok := ppVideoJSONValue(body, "parameters."+field, field, "data."+field)
	if !ok {
		return defaultValue, false, nil
	}
	parsed, ok := value.(bool)
	if !ok {
		return false, true, fmt.Errorf("Kuaishou %s must be boolean", field)
	}
	return parsed, true, nil
}

func kuaishouAspectRatioAllowed(ratio string) bool {
	switch strings.TrimSpace(ratio) {
	case "16:9", "9:16", "1:1":
		return true
	default:
		return false
	}
}

func kuaishouTypeAllowed(value string) bool {
	switch strings.TrimSpace(value) {
	case "t2v", "i2v", "motion_control":
		return true
	default:
		return false
	}
}

func kuaishouBillingResolution(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "pro", "2x_pro", "2x-pro":
		return VideoBillingResolution1080P
	case "4k":
		return VideoBillingResolution1080P
	default:
		return VideoBillingResolution720P
	}
}

func pixverseV6Resolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "360", "360p":
		return "360p"
	case "540", "540p":
		return "540p"
	case "720", "720p":
		return "720p"
	case "1080", "1080p":
		return "1080p"
	default:
		return ""
	}
}

func pixverseV6BillingResolution(resolution string) string {
	switch pixverseV6Resolution(resolution) {
	case "360p":
		return VideoBillingResolution480P
	case "540p":
		return VideoBillingResolution720P
	case "720p":
		return VideoBillingResolution720P
	case "1080p":
		return VideoBillingResolution1080P
	default:
		return ""
	}
}

func pixverseV6AspectRatioFromRequest(body []byte) (string, bool, error) {
	value, ok := ppVideoJSONValue(body, "parameters.aspect_ratio", "aspect_ratio")
	if !ok {
		return "", false, nil
	}
	aspectRatio, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("Pixverse v6 aspect_ratio must be a string")
	}
	aspectRatio = strings.TrimSpace(aspectRatio)
	switch aspectRatio {
	case "16:9", "4:3", "1:1", "3:4", "9:16", "2:3", "3:2", "21:9":
		return aspectRatio, true, nil
	default:
		return "", false, fmt.Errorf("Pixverse v6 aspect_ratio is not supported")
	}
}

func pixverseV6IntegerParameter(body []byte, field string) (int64, bool, error) {
	value, ok := ppVideoJSONValue(body, "parameters."+field, field)
	if !ok {
		return 0, false, nil
	}
	number, ok := value.(float64)
	if !ok || number != float64(int64(number)) {
		return 0, false, fmt.Errorf("Pixverse v6 %s must be an integer", field)
	}
	return int64(number), true, nil
}

func pixverseV6GenerateAudioFromRequest(body []byte) (bool, bool, error) {
	value, ok := ppVideoJSONValue(body, "parameters.generate_audio", "generate_audio")
	if !ok {
		return false, false, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, true, nil
	case float64:
		if typed == 0 {
			return false, true, nil
		}
		if typed == 1 {
			return true, true, nil
		}
	}
	return false, false, fmt.Errorf("Pixverse v6 generate_audio must be boolean or 0/1")
}

func pixverseV6ImageURLAllowed(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	lowerURL := strings.ToLower(rawURL)
	if strings.HasPrefix(lowerURL, "data:image/") {
		return strings.Contains(lowerURL, ";base64,")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Host) == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

func pixverseV6VideoURLAllowed(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Host) == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

func normalizePPVideoMiniMaxH3Payload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if payload == nil || public == nil {
		return nil, fmt.Errorf("MiniMax-H3 request body is required")
	}
	if len(body) > 64*1024*1024 {
		return nil, fmt.Errorf("MiniMax-H3 request body must not exceed 64 MB")
	}
	if public.DurationMilliseconds <= 0 || public.DurationMilliseconds%1000 != 0 {
		return nil, fmt.Errorf("MiniMax-H3 duration must be an integer number of seconds from 4 to 15")
	}
	durationSeconds := public.DurationMilliseconds / 1000
	if durationSeconds < 4 || durationSeconds > 15 {
		return nil, fmt.Errorf("MiniMax-H3 duration must be an integer number of seconds from 4 to 15")
	}

	content, hasFrameImage, hasReferenceImage, hasReferenceVideo, hasReferenceAudio, err := normalizePPVideoMiniMaxH3Content(body, public)
	if err != nil {
		return nil, err
	}
	if hasFrameImage && (hasReferenceImage || hasReferenceVideo || hasReferenceAudio) {
		return nil, fmt.Errorf("MiniMax-H3 cannot combine first/last-frame images with reference media")
	}
	if hasReferenceAudio && !hasReferenceImage && !hasReferenceVideo {
		return nil, fmt.Errorf("MiniMax-H3 reference audio requires a reference image or reference video")
	}

	resolution := miniMaxH3Resolution(public.Resolution)
	if resolution == "" {
		return nil, fmt.Errorf("MiniMax-H3 resolution must be 768P or 2K")
	}
	public.Resolution = miniMaxH3BillingResolution(resolution)
	public.VideoCount = 1
	public.UpstreamModel = MiniMaxH3VideoDefaultModel

	ratio, err := miniMaxH3RatioFromRequest(body)
	if err != nil {
		return nil, err
	}
	hasReferenceMedia := hasReferenceImage || hasReferenceVideo || hasReferenceAudio
	if hasFrameImage {
		ratio = "adaptive"
	} else if ratio == "" {
		if hasReferenceMedia {
			ratio = "adaptive"
		} else {
			ratio = "16:9"
		}
	}
	if !miniMaxH3RatioAllowed(ratio) {
		return nil, fmt.Errorf("MiniMax-H3 ratio is not supported")
	}
	if !hasFrameImage && !hasReferenceMedia && ratio == "adaptive" {
		return nil, fmt.Errorf("MiniMax-H3 text-to-video requests require a non-adaptive ratio")
	}

	watermark := false
	if value, ok := ppVideoJSONValue(body, "parameters.aigc_watermark", "aigc_watermark"); ok {
		parsed, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("MiniMax-H3 aigc_watermark must be boolean")
		}
		watermark = parsed
	}

	return map[string]any{
		"model": MiniMaxH3VideoDefaultModel,
		"input": map[string]any{
			"content": content,
		},
		"parameters": map[string]any{
			"resolution":     resolution,
			"duration":       int(durationSeconds),
			"ratio":          ratio,
			"aigc_watermark": watermark,
		},
	}, nil
}

func normalizePPVideoMiniMaxHailuo23Payload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error) {
	if payload == nil || public == nil {
		return nil, fmt.Errorf("MiniMax-Hailuo-2.3 request body is required")
	}
	if len(body) > 64*1024*1024 {
		return nil, fmt.Errorf("MiniMax-Hailuo-2.3 request body must not exceed 64 MB")
	}
	if gjson.GetBytes(body, "input.content").Exists() {
		return nil, fmt.Errorf("MiniMax-Hailuo-2.3 does not support input.content; use input.prompt and input.first_frame_image")
	}

	prompt := strings.TrimSpace(extractPPVideoText(body,
		"input.prompt",
		"prompt",
		"data.prompt",
		"parameters.prompt",
	))
	if err := validateVideoPromptFieldLength("MiniMax-Hailuo-2.3", "prompt", prompt, 2000); err != nil {
		return nil, err
	}
	if prompt == "" {
		return nil, fmt.Errorf("MiniMax-Hailuo-2.3 input.prompt is required")
	}

	durationSeconds := int64(6)
	if duration, found := extractPPVideoDurationMillisecondsWithPresence(body,
		"parameters.duration",
		"duration",
		"data.duration",
		"parameters.duration_seconds",
		"duration_seconds",
	); found {
		if duration <= 0 || duration%1000 != 0 {
			return nil, fmt.Errorf("MiniMax-Hailuo-2.3 duration must be 6 or 10 seconds")
		}
		durationSeconds = duration / 1000
	}
	if durationSeconds != 6 && durationSeconds != 10 {
		return nil, fmt.Errorf("MiniMax-Hailuo-2.3 duration must be 6 or 10 seconds")
	}

	resolution := miniMaxHailuo23Resolution(extractPPVideoText(body,
		"parameters.resolution",
		"resolution",
		"data.resolution",
	))
	if resolution == "" {
		resolution = "768P"
	}
	if durationSeconds == 10 && resolution == "1080P" {
		return nil, fmt.Errorf("MiniMax-Hailuo-2.3 10-second videos only support 768P")
	}

	firstFrameImage := ""
	if value, found := ppVideoJSONValue(body, "input.first_frame_image", "first_frame_image", "data.first_frame_image"); found {
		var ok bool
		firstFrameImage, ok = value.(string)
		if !ok {
			return nil, fmt.Errorf("MiniMax-Hailuo-2.3 first_frame_image must be a string")
		}
		firstFrameImage = strings.TrimSpace(firstFrameImage)
		if firstFrameImage == "" {
			return nil, fmt.Errorf("MiniMax-Hailuo-2.3 first_frame_image must not be empty")
		}
		if !miniMaxHailuo23ImageURLAllowed(firstFrameImage) {
			return nil, fmt.Errorf("MiniMax-Hailuo-2.3 first_frame_image must be a public HTTP/HTTPS URL or an image Base64 data URL")
		}
	}

	promptOptimizer, err := miniMaxHailuo23BooleanParameter(body, "prompt_optimizer", true)
	if err != nil {
		return nil, err
	}
	fastPretreatment, err := miniMaxHailuo23BooleanParameter(body, "fast_pretreatment", false)
	if err != nil {
		return nil, err
	}
	watermark, err := miniMaxHailuo23BooleanParameter(body, "aigc_watermark", false)
	if err != nil {
		return nil, err
	}

	public.Prompt = prompt
	public.DurationMilliseconds = durationSeconds * 1000
	public.Resolution = normalizePPVideoResolution(resolution)
	public.VideoCount = 1
	public.HasImage = firstFrameImage != ""
	public.UpstreamModel = MiniMaxHailuo23VideoModel

	input := map[string]any{"prompt": prompt}
	if firstFrameImage != "" {
		input["first_frame_image"] = firstFrameImage
	}
	return map[string]any{
		"model": MiniMaxHailuo23VideoModel,
		"input": input,
		"parameters": map[string]any{
			"duration":          int(durationSeconds),
			"resolution":        resolution,
			"prompt_optimizer":  promptOptimizer,
			"fast_pretreatment": fastPretreatment,
			"aigc_watermark":    watermark,
		},
	}, nil
}

func miniMaxHailuo23BooleanParameter(body []byte, field string, defaultValue bool) (bool, error) {
	value, ok := ppVideoJSONValue(body, "parameters."+field, field)
	if !ok {
		return defaultValue, nil
	}
	parsed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("MiniMax-Hailuo-2.3 %s must be boolean", field)
	}
	return parsed, nil
}

func miniMaxHailuo23Resolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "", "768", "768p":
		return "768P"
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
		return "1080P"
	default:
		return ""
	}
}

func miniMaxHailuo23ImageURLAllowed(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	lowerURL := strings.ToLower(rawURL)
	if strings.HasPrefix(lowerURL, "data:image/") {
		return strings.Contains(lowerURL, ";base64,")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Host) == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

func normalizePPVideoMiniMaxH3Content(body []byte, public *PPVideoPublicRequest) ([]any, bool, bool, bool, bool, error) {
	rawContent := gjson.GetBytes(body, "input.content")
	content := make([]any, 0, 4)
	if rawContent.Exists() {
		if !rawContent.IsArray() || len(rawContent.Array()) == 0 || len(rawContent.Array()) > 16 {
			return nil, false, false, false, false, fmt.Errorf("MiniMax-H3 input.content must contain 1 to 16 items")
		}
		for _, item := range rawContent.Array() {
			var source map[string]any
			if err := json.Unmarshal([]byte(item.Raw), &source); err != nil || source == nil {
				return nil, false, false, false, false, fmt.Errorf("MiniMax-H3 input.content items must be JSON objects")
			}
			normalized, err := normalizePPVideoMiniMaxH3ContentItem(source)
			if err != nil {
				return nil, false, false, false, false, err
			}
			content = append(content, normalized)
		}
	} else {
		if prompt := strings.TrimSpace(public.Prompt); prompt != "" {
			content = append(content, map[string]any{"type": "text", "text": prompt})
		}
		if startFrame := ppVideoJSONText(body, "start_frame_url", "input.start_frame_url", "data.start_frame_url"); startFrame != "" {
			content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": startFrame}, "role": "first_frame"})
		}
		if endFrame := ppVideoJSONText(body, "end_frame_url", "input.end_frame_url", "data.end_frame_url"); endFrame != "" {
			content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": endFrame}, "role": "last_frame"})
		}
		if image := ppVideoJSONText(body, "image", "image_url", "input.image", "input.image_url"); image != "" {
			content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": image}, "role": "first_frame"})
		}
		for _, item := range gjson.GetBytes(body, "images").Array() {
			if image := strings.TrimSpace(item.String()); image != "" {
				content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": image}, "role": "reference_image"})
			}
		}
		if video := ppVideoJSONText(body, "video", "video_url", "videos.0", "input.video", "input.video_url"); video != "" {
			content = append(content, map[string]any{"type": "video_url", "video_url": map[string]any{"url": video}, "role": "reference_video"})
		}
		if audio := ppVideoJSONText(body, "audio", "audio_url", "audios.0", "input.audio", "input.audio_url"); audio != "" {
			content = append(content, map[string]any{"type": "audio_url", "audio_url": map[string]any{"url": audio}, "role": "reference_audio"})
		}
	}

	if len(content) == 0 || len(content) > 16 {
		return nil, false, false, false, false, fmt.Errorf("MiniMax-H3 input.content must contain 1 to 16 items")
	}

	canonicalContent := make([]any, 0, len(content))
	for _, item := range content {
		source, ok := item.(map[string]any)
		if !ok {
			return nil, false, false, false, false, fmt.Errorf("MiniMax-H3 input.content items must be JSON objects")
		}
		normalized, err := normalizePPVideoMiniMaxH3ContentItem(source)
		if err != nil {
			return nil, false, false, false, false, err
		}
		canonicalContent = append(canonicalContent, normalized)
	}
	content = canonicalContent

	textCount := 0
	firstFrameCount := 0
	lastFrameCount := 0
	referenceImageCount := 0
	referenceVideoCount := 0
	referenceAudioCount := 0
	for _, item := range content {
		normalized, ok := item.(map[string]any)
		if !ok {
			return nil, false, false, false, false, fmt.Errorf("MiniMax-H3 input.content items must be JSON objects")
		}
		switch normalized["type"] {
		case "text":
			textCount++
			if text, ok := normalized["text"].(string); ok {
				public.Prompt = strings.TrimSpace(text)
			}
		case "image_url":
			public.HasImage = true
			switch normalized["role"] {
			case "first_frame":
				firstFrameCount++
			case "last_frame":
				lastFrameCount++
			case "reference_image":
				referenceImageCount++
			}
		case "video_url":
			referenceVideoCount++
		case "audio_url":
			referenceAudioCount++
		}
	}
	if textCount != 1 {
		return nil, false, false, false, false, fmt.Errorf("MiniMax-H3 input.content must contain exactly one non-empty text item")
	}
	if firstFrameCount > 1 || lastFrameCount > 1 || referenceImageCount > 9 || referenceVideoCount > 3 || referenceAudioCount > 3 {
		return nil, false, false, false, false, fmt.Errorf("MiniMax-H3 input.content exceeds the supported media-item limit")
	}
	return content, firstFrameCount > 0 || lastFrameCount > 0, referenceImageCount > 0, referenceVideoCount > 0, referenceAudioCount > 0, nil
}

func normalizePPVideoMiniMaxH3ContentItem(source map[string]any) (map[string]any, error) {
	contentType, _ := source["type"].(string)
	contentType = strings.TrimSpace(contentType)
	result := map[string]any{"type": contentType}
	switch contentType {
	case "text":
		text, _ := source["text"].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, fmt.Errorf("MiniMax-H3 text content must not be empty")
		}
		result["text"] = text
	case "image_url":
		value, ok := source["image_url"]
		if !ok {
			return nil, fmt.Errorf("MiniMax-H3 image_url content requires image_url")
		}
		normalized, err := normalizePPVideoMiniMaxH3URLObject(value)
		if err != nil {
			return nil, fmt.Errorf("MiniMax-H3 image_url content: %w", err)
		}
		role, _ := source["role"].(string)
		role = strings.TrimSpace(role)
		if role == "" {
			role = "first_frame"
		}
		if role != "first_frame" && role != "last_frame" && role != "reference_image" {
			return nil, fmt.Errorf("MiniMax-H3 image role %q is not supported", role)
		}
		result["image_url"] = normalized
		result["role"] = role
	case "video_url":
		value, ok := source["video_url"]
		if !ok {
			return nil, fmt.Errorf("MiniMax-H3 video_url content requires video_url")
		}
		normalized, err := normalizePPVideoMiniMaxH3URLObject(value)
		if err != nil {
			return nil, fmt.Errorf("MiniMax-H3 video_url content: %w", err)
		}
		role, _ := source["role"].(string)
		role = strings.TrimSpace(role)
		if role == "" {
			role = "reference_video"
		}
		if role != "reference_video" {
			return nil, fmt.Errorf("MiniMax-H3 video role %q is not supported", role)
		}
		result["video_url"] = normalized
		result["role"] = role
	case "audio_url":
		value, ok := source["audio_url"]
		if !ok {
			return nil, fmt.Errorf("MiniMax-H3 audio_url content requires audio_url")
		}
		normalized, err := normalizePPVideoMiniMaxH3URLObject(value)
		if err != nil {
			return nil, fmt.Errorf("MiniMax-H3 audio_url content: %w", err)
		}
		role, _ := source["role"].(string)
		role = strings.TrimSpace(role)
		if role == "" {
			role = "reference_audio"
		}
		if role != "reference_audio" {
			return nil, fmt.Errorf("MiniMax-H3 audio role %q is not supported", role)
		}
		result["audio_url"] = normalized
		result["role"] = role
	default:
		return nil, fmt.Errorf("MiniMax-H3 content type %q is not supported", contentType)
	}
	return result, nil
}

func normalizePPVideoMiniMaxH3URLObject(value any) (map[string]any, error) {
	normalized, err := normalizePPVideoByteDanceURLObject(value)
	if err != nil {
		return nil, err
	}
	rawURL, _ := normalized["url"].(string)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("url must be a public http or https URL")
	}
	return normalized, nil
}

func miniMaxH3Resolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "", "480", "480p", "720", "720p", "hd", "768", "768p":
		return "768P"
	case "1080", "1080p", "full_hd", "full-hd", "fhd", "2k":
		return "2K"
	default:
		return ""
	}
}

func miniMaxH3BillingResolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "768p":
		return VideoBillingResolution720P
	case "2k":
		return VideoBillingResolution1080P
	default:
		return ""
	}
}

func miniMaxH3RatioFromRequest(body []byte) (string, error) {
	value, ok := ppVideoJSONValue(body, "parameters.ratio", "ratio", "parameters.aspect_ratio", "aspect_ratio")
	if !ok {
		return "", nil
	}
	ratio, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("MiniMax-H3 ratio must be a string")
	}
	return strings.TrimSpace(ratio), nil
}

func miniMaxH3RatioAllowed(ratio string) bool {
	switch strings.TrimSpace(ratio) {
	case "adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16":
		return true
	default:
		return false
	}
}

func ppVideoJSONValue(body []byte, paths ...string) (any, bool) {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		var result any
		if err := json.Unmarshal([]byte(value.Raw), &result); err != nil {
			continue
		}
		return result, true
	}
	return nil, false
}

func ppVideoJSONText(body []byte, paths ...string) string {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if value.Exists() && strings.TrimSpace(value.String()) != "" {
			return strings.TrimSpace(value.String())
		}
	}
	return ""
}

func byteDanceResolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "480p":
		return "480p"
	case "720p":
		return "720p"
	case "1080p":
		return "1080p"
	case "4k":
		return "4K"
	default:
		return ""
	}
}

func byteDanceResolutionForModel(model, resolution string) string {
	normalized := byteDanceResolution(resolution)
	if byteDanceIsSeedance25Model(model) && normalized == "4K" {
		return ""
	}
	return normalized
}

func normalizePPVideoKlingPayload(payload map[string]any, operation PPVideoOperation, body []byte, public *PPVideoPublicRequest) error {
	if payload == nil || public == nil {
		return fmt.Errorf("Kling request body is required")
	}
	duration, err := ppVideoKlingDurationString(public.DurationMilliseconds)
	if err != nil {
		return err
	}
	mode, err := ppVideoKlingModeFromRequest(body, public.Resolution)
	if err != nil {
		return err
	}
	if err := normalizePPVideoKlingPromptFields(payload, body, public); err != nil {
		return err
	}

	payload["model_name"] = public.UpstreamModel
	payload["duration"] = duration
	payload["mode"] = mode
	public.VideoCount = 1

	if sound, ok, err := ppVideoKlingSoundFromRequest(body); err != nil {
		return err
	} else if ok {
		payload["sound"] = sound
	}

	switch operation {
	case PPVideoOperationKlingImageToVideo:
		image := ppVideoKlingRequestImage(body)
		if image == "" {
			return fmt.Errorf("Kling image-to-video request requires image, start_frame_url, image_url, or images[0]")
		}
		payload["image"] = image
		if imageTail := ppVideoKlingRequestImageTail(body); imageTail != "" {
			payload["image_tail"] = imageTail
		} else {
			delete(payload, "image_tail")
		}
	case PPVideoOperationKlingTextToVideo:
		delete(payload, "image")
		delete(payload, "image_tail")
		delete(payload, "element_list")
	default:
		return fmt.Errorf("platform %q does not support operation %q", PlatformKling, operation)
	}

	ppVideoKeepOnlyKlingSupportedFields(payload, operation)
	return nil
}

func normalizePPVideoKlingPromptFields(payload map[string]any, body []byte, public *PPVideoPublicRequest) error {
	prompt := strings.TrimSpace(public.Prompt)
	if err := validatePPVideoKlingTextLength("prompt", prompt); err != nil {
		return err
	}
	if prompt != "" {
		payload["prompt"] = prompt
		public.Prompt = prompt
	} else if raw, ok := payload["prompt"].(string); ok {
		payload["prompt"] = strings.TrimSpace(raw)
	}

	negativePrompt := extractPPVideoText(body, "negative_prompt", "data.negative_prompt", "parameters.negative_prompt", "input.negative_prompt")
	if err := validatePPVideoKlingTextLength("negative_prompt", negativePrompt); err != nil {
		return err
	}
	if negativePrompt != "" {
		payload["negative_prompt"] = negativePrompt
	} else if raw, ok := payload["negative_prompt"].(string); ok {
		payload["negative_prompt"] = strings.TrimSpace(raw)
	}
	return nil
}

func validatePPVideoKlingTextLength(field, value string) error {
	return validateVideoPromptFieldLength("Kling", field, value, ppVideoKlingPromptMaxRunes)
}

func PPVideoUpstreamPath(platform string, operation PPVideoOperation, taskID string) (string, error) {
	return ppVideoUpstreamPath(platform, operation, taskID, false)
}

func PPVideoStatusUpstreamPath(platform string, operation PPVideoOperation, taskID string) (string, error) {
	return ppVideoUpstreamPath(platform, operation, taskID, true)
}

func ppVideoUpstreamPath(platform string, operation PPVideoOperation, taskID string, requireTaskID bool) (string, error) {
	if !IsPPVideoPlatform(platform) {
		return "", fmt.Errorf("unsupported PP video platform %q", platform)
	}
	taskID = strings.TrimSpace(taskID)
	if requireTaskID && taskID == "" {
		return "", fmt.Errorf("PP video task id is required for status requests")
	}

	var path string
	switch platform {
	case PlatformMiniMaxH3CompShare:
		if operation == PPVideoOperationCancel && taskID != "" {
			return "/minimax/v2/video_generation/" + url.PathEscape(taskID), nil
		}
		if operation != PPVideoOperationGeneric {
			return "", fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		if taskID != "" {
			return "/minimax/v2/query/video_generation/" + url.PathEscape(taskID), nil
		}
		return "/minimax/v2/video_generation", nil
	case PlatformByteDance, PlatformWan3, PlatformMiniMaxH3, PlatformPixverseV6, PlatformGrokImagineVideo, PlatformKuaishou:
		switch operation {
		case PPVideoOperationGeneric:
			path = "/v1/tasks/submit"
		case PPVideoOperationCancel:
			if platform != PlatformByteDance {
				return "", fmt.Errorf("platform %q does not support operation %q", platform, operation)
			}
			if taskID == "" {
				return "", fmt.Errorf("PP video task id is required for cancel requests")
			}
			return "/v1/tasks/cancel?task_id=" + url.QueryEscape(taskID), nil
		default:
			return "", fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
	case PlatformSeedance, PlatformHappyHourse:
		if operation != PPVideoOperationGeneric {
			return "", fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
		path = "/v1/video/generations"
	case PlatformKling:
		switch operation {
		case PPVideoOperationKlingTextToVideo:
			path = "/v1/videos/text2video"
		case PPVideoOperationKlingImageToVideo:
			path = "/v1/videos/image2video"
		default:
			return "", fmt.Errorf("platform %q does not support operation %q", platform, operation)
		}
	}

	if taskID == "" {
		return path, nil
	}
	if platform == PlatformByteDance || platform == PlatformWan3 || platform == PlatformMiniMaxH3 || platform == PlatformPixverseV6 || platform == PlatformGrokImagineVideo || platform == PlatformKuaishou {
		return "/v1/tasks/status?task_id=" + url.QueryEscape(taskID), nil
	}
	return path + "/" + url.PathEscape(taskID), nil
}

func PPVideoRequestHeaders(platform string, apiKey string) (http.Header, error) {
	return ppVideoRequestHeaders(platform, apiKey, true)
}

func PPVideoStatusRequestHeaders(platform string, apiKey string) (http.Header, error) {
	return ppVideoRequestHeaders(platform, apiKey, false)
}

func ppVideoRequestHeaders(platform string, apiKey string, isSubmission bool) (http.Header, error) {
	if !IsPPVideoPlatform(platform) {
		return nil, fmt.Errorf("unsupported PP video platform %q", platform)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("PP video api key is required")
	}

	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Authorization", "Bearer "+apiKey)
	headers.Set("Content-Type", "application/json")
	if isSubmission && platform == PlatformHappyHourse {
		headers.Set("X-DashScope-Async", "enable")
	}
	return headers, nil
}

func ParsePPVideoResponse(platform string, body []byte) (PPVideoResponse, error) {
	if !IsPPVideoPlatform(platform) {
		return PPVideoResponse{}, fmt.Errorf("unsupported PP video platform %q", platform)
	}
	if !gjson.ValidBytes(body) {
		return PPVideoResponse{}, fmt.Errorf("parse PP video response: invalid JSON")
	}
	if platform == PlatformMiniMaxH3CompShare {
		return parseCompShareVideoResponse(body)
	}

	result := PPVideoResponse{
		TaskID: extractPPVideoText(body,
			"output.task_id",
			"output.id",
			"id",
			"task_id",
			"request_id",
			"generation_id",
			"generationId",
			"data.id",
			"data.task_id",
			"data.request_id",
			"data.generation_id",
			"data.generationId",
			"data.data.id",
			"data.data.task_id",
			"data.data.request_id",
			"data.data.generation_id",
			"data.data.generationId",
			"data.data.data.id",
			"data.data.data.task_id",
			"data.data.data.request_id",
			"data.data.data.generation_id",
			"data.data.data.generationId",
		),
		Status: NormalizePPVideoTaskStatus(extractPPVideoText(body,
			"output.task_status",
			"output.status",
			"output.state",
			"status",
			"state",
			"task_status",
			"data.status",
			"data.state",
			"data.task_status",
			"data.data.status",
			"data.data.state",
			"data.data.task_status",
			"data.data.data.status",
			"data.data.data.state",
			"data.data.data.task_status",
		)),
		GeneratedDurationMilliseconds: extractPPVideoDurationMilliseconds(body,
			"data.task_result.videos.0.duration",
			"data.task_result.duration",
			"data.data.task_result.videos.0.duration",
			"data.data.task_result.duration",
			"data.data.data.task_result.videos.0.duration",
			"data.data.data.task_result.duration",
			"output.usage.output_video_duration",
			"data.data.usage.output_video_duration",
			"data.usage.output_video_duration",
			"usage.output_video_duration",
			"output.usage.duration",
			"data.data.usage.duration",
			"data.usage.duration",
			"usage.duration",
			"data.data.duration",
			"data.duration",
			"duration",
		),
		VideoCount: extractPPVideoPositiveInt(body,
			"data.data.usage.video_count",
			"data.usage.video_count",
			"usage.video_count",
			"data.data.video_count",
			"data.video_count",
			"video_count",
			"data.task_result.videos.#",
			"data.data.task_result.videos.#",
			"data.data.data.task_result.videos.#",
		),
		VideoResolution: normalizePPVideoResolution(extractPPVideoText(body,
			"data.data.usage.SR",
			"data.data.usage.sr",
			"data.data.usage.resolution",
			"data.usage.SR",
			"data.usage.sr",
			"data.usage.resolution",
			"usage.SR",
			"usage.sr",
			"usage.resolution",
			"data.data.resolution",
			"data.resolution",
			"resolution",
		)),
		OutputWidth: extractPPVideoPositiveInt(body,
			"data.data.usage.output_width",
			"data.usage.output_width",
			"usage.output_width",
			"data.data.output_width",
			"data.output_width",
			"output_width",
			"data.data.width",
			"data.width",
			"width",
		),
		OutputHeight: extractPPVideoPositiveInt(body,
			"data.data.usage.output_height",
			"data.usage.output_height",
			"usage.output_height",
			"data.data.output_height",
			"data.output_height",
			"output_height",
			"data.data.height",
			"data.height",
			"height",
		),
		FrameRate: extractPPVideoFloat(body,
			"data.data.usage.fps",
			"data.usage.fps",
			"usage.fps",
			"data.data.usage.frame_rate",
			"data.usage.frame_rate",
			"usage.frame_rate",
			"data.data.fps",
			"data.fps",
			"fps",
			"data.data.frame_rate",
			"data.frame_rate",
			"frame_rate",
		),
		ErrorMessage: extractPPVideoText(body,
			"output.error_message",
			"output.error",
			"error_message",
			"error.message",
			"data.error_message",
			"data.error.message",
			"data.data.error_message",
			"data.data.error.message",
		),
		InputVideoDurationMilliseconds: extractPPVideoDurationMilliseconds(body,
			"data.data.usage.input_video_duration",
			"data.usage.input_video_duration",
			"usage.input_video_duration",
			"data.data.input_video_duration",
			"data.input_video_duration",
			"input_video_duration",
			"input_duration",
		),
		RawBody: append([]byte(nil), body...),
	}
	hasFinalVideo := ppVideoResponseHasFinalVideo(body)
	if hasFinalVideo && result.Status != PPVideoTaskStatusFailed {
		result.Status = PPVideoTaskStatusSucceeded
	}
	if result.Status == PPVideoTaskStatusSucceeded && !hasFinalVideo {
		result.Status = PPVideoTaskStatusProcessing
	}
	if result.VideoCount <= 0 {
		result.VideoCount = 1
	}
	return result, nil
}

func NormalizePPVideoPublicResponse(platform string, raw []byte, public PPVideoPublicRequest, parsed PPVideoResponse) []byte {
	if !IsPPVideoPlatform(platform) || len(raw) == 0 || !gjson.ValidBytes(raw) {
		return raw
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return raw
	}
	taskID := strings.TrimSpace(parsed.TaskID)
	if taskID == "" {
		taskID = extractPPVideoText(raw, "id", "task_id", "request_id")
	}
	if taskID != "" {
		payload["id"] = taskID
		payload["task_id"] = taskID
	}
	payload["object"] = ppVideoPublicResponseObject
	status := NormalizePPVideoTaskStatus(parsed.Status)
	if status != "" {
		payload["status"] = status
	}
	model := strings.TrimSpace(public.Model)
	if model == "" {
		model = strings.TrimSpace(public.UpstreamModel)
	}
	if model == "" {
		model = ppVideoUpstreamModel(platform, public)
	}
	payload["model"] = model
	if videoURL := ppVideoExtractVideoURL(raw); videoURL != "" {
		ppVideoSetPublicVideoURL(payload, videoURL)
	}
	usage := ppVideoPublicUsage(raw, public, parsed)
	if len(usage) > 0 {
		payload["usage"] = usage
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return normalized
}

func ppVideoSetPublicVideoURL(payload map[string]any, videoURL string) {
	videoURL = strings.TrimSpace(videoURL)
	if payload == nil || videoURL == "" {
		return
	}
	payload["video_url"] = videoURL
	payload["url"] = videoURL
	payload["result_url"] = videoURL
	ppVideoSetVideosField(payload, videoURL)
	ppVideoSetDataVideoURL(payload, videoURL)
}

func ppVideoSetDataVideoURL(payload map[string]any, videoURL string) {
	videoURL = strings.TrimSpace(videoURL)
	if payload == nil || videoURL == "" {
		return
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		if _, exists := payload["data"]; exists {
			return
		}
		data = map[string]any{}
		payload["data"] = data
	}
	data["video_url"] = videoURL
	data["url"] = videoURL
	data["result_url"] = videoURL
	ppVideoSetVideosField(data, videoURL)
}

func ppVideoSetVideosField(payload map[string]any, videoURL string) {
	if payload == nil || strings.TrimSpace(videoURL) == "" {
		return
	}
	if videos, ok := payload["videos"].([]any); ok && len(videos) > 0 {
		return
	}
	payload["videos"] = []any{map[string]any{"url": videoURL, "video_url": videoURL}}
}

func ppVideoWithoutPublicVideoURL(raw []byte) []byte {
	if len(raw) == 0 || !gjson.ValidBytes(raw) {
		return raw
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return raw
	}
	ppVideoRemoveVideoURLFields(payload)
	ppVideoMarkProcessingStatusFields(payload)
	normalized, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return normalized
}

func ppVideoRemoveVideoURLFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"video_url", "result_url", "url", "download_url", "video_url_download", "video_urls", "urls", "videos"} {
			delete(typed, key)
		}
		for _, child := range typed {
			ppVideoRemoveVideoURLFields(child)
		}
	case []any:
		for _, child := range typed {
			ppVideoRemoveVideoURLFields(child)
		}
	}
}

func ppVideoMarkProcessingStatusFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"status", "state", "task_status"} {
			if raw, ok := typed[key].(string); ok && NormalizePPVideoTaskStatus(raw) == PPVideoTaskStatusSucceeded {
				typed[key] = PPVideoTaskStatusProcessing
			}
		}
		for _, child := range typed {
			ppVideoMarkProcessingStatusFields(child)
		}
	case []any:
		for _, child := range typed {
			ppVideoMarkProcessingStatusFields(child)
		}
	}
}

func NormalizePPVideoTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "processing", "running", "queued", "created", "submitted", "in_progress", "waiting":
		return PPVideoTaskStatusProcessing
	case "success", "succeed", "succeeded", "completed", "complete", "done", "finished", "successful":
		return PPVideoTaskStatusSucceeded
	case "fail", "failed", "failure", "error", "cancelled", "canceled", "rejected", "refunded", "timeout", "timed_out", "expired":
		return PPVideoTaskStatusFailed
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func PPVideoBillingMetadataFromRequest(platform string, body []byte) PPVideoBillingMetadata {
	metadata := PPVideoBillingMetadata{
		Platform:   platform,
		VideoCount: 1,
	}
	if !IsPPVideoPlatform(platform) {
		metadata.ValidationError = fmt.Errorf("unsupported PP video platform %q", platform)
		return metadata
	}
	if !gjson.ValidBytes(body) {
		metadata.ValidationError = fmt.Errorf("PP video request body must be valid JSON")
		return metadata
	}
	metadata.Model = PPVideoModelFromBody(body)

	resolution := extractPPVideoText(body,
		"resolution",
		"size",
		"quality",
		"data.resolution",
		"data.size",
		"parameters.resolution",
		"parameters.size",
		"metadata.resolution",
		"metadata.size",
		"metadata.quality",
		"video.resolution",
		"video.size",
	)
	klingMode := ""
	if platform == PlatformKling {
		klingMode = strings.ToLower(strings.TrimSpace(extractPPVideoText(body, "mode")))
		if klingMode == "" {
			klingMode = ppVideoKlingModeForResolution(normalizePPVideoResolution(resolution))
		}
		switch klingMode {
		case "std", "2x":
			resolution = VideoBillingResolution720P
		case "pro", "2x_pro", "2x-pro":
			resolution = VideoBillingResolution1080P
		case "4k":
			resolution = "4k"
		default:
			resolution = ""
		}
	}

	duration, durationFound := extractPPVideoDurationMillisecondsWithPresence(body,
		"duration",
		"duration_seconds",
		"seconds",
		"data.duration",
		"data.duration_seconds",
		"parameters.duration",
		"parameters.duration_seconds",
		"metadata.duration",
		"metadata.duration_seconds",
		"video.duration",
		"video.duration_seconds",
	)
	metadata.RequestedDurationMilliseconds = duration
	if duration > 0 && duration%1000 == 0 {
		metadata.RequestedDurationSeconds = int(duration / 1000)
	}
	metadata.VideoCount = extractPPVideoPositiveInt(body,
		"video_count",
		"n",
		"count",
		"data.video_count",
		"parameters.video_count",
		"parameters.n",
		"metadata.video_count",
		"metadata.n",
		"video.count",
	)
	if metadata.VideoCount <= 0 {
		metadata.VideoCount = 1
	}
	metadata.VideoResolution = normalizePPVideoResolution(resolution)
	if platform == PlatformMiniMaxH3CompShare {
		metadata.VideoResolution = strings.ToLower(resolution)
	}
	metadata.KlingMode = klingMode
	metadata.InputVideoDurationMilliseconds = extractPPVideoDurationMilliseconds(body,
		"input_video_duration",
		"input_video_duration_seconds",
		"input_duration",
		"input_duration_seconds",
		"source_video_duration",
		"source_video_duration_seconds",
		"reference_video_duration",
		"reference_video_duration_seconds",
		"data.input_video_duration",
		"data.input_video_duration_seconds",
		"parameters.input_video_duration",
		"parameters.input_video_duration_seconds",
		"metadata.input_video_duration",
		"metadata.input_video_duration_seconds",
	)
	metadata.OutputWidth = extractPPVideoPositiveInt(body,
		"output_width",
		"width",
		"data.output_width",
		"data.width",
		"parameters.output_width",
		"parameters.width",
		"metadata.output_width",
		"metadata.width",
	)
	metadata.OutputHeight = extractPPVideoPositiveInt(body,
		"output_height",
		"height",
		"data.output_height",
		"data.height",
		"parameters.output_height",
		"parameters.height",
		"metadata.output_height",
		"metadata.height",
	)
	if metadata.OutputWidth <= 0 || metadata.OutputHeight <= 0 {
		defaultWidth, defaultHeight := ppVideoResolutionDimensions(metadata.VideoResolution)
		if metadata.OutputWidth <= 0 {
			metadata.OutputWidth = defaultWidth
		}
		if metadata.OutputHeight <= 0 {
			metadata.OutputHeight = defaultHeight
		}
	}
	metadata.FrameRate = extractPPVideoFloat(body,
		"fps",
		"frame_rate",
		"frameRate",
		"data.fps",
		"data.frame_rate",
		"parameters.fps",
		"parameters.frame_rate",
		"metadata.fps",
		"metadata.frame_rate",
	)
	if metadata.FrameRate <= 0 {
		metadata.FrameRate = PPVideoDefaultFrameRate
	}
	metadata.HasAudio = extractPPVideoBool(body,
		"generate_audio",
		"with_audio",
		"audio",
		"sound",
		"enable_audio",
		"data.generate_audio",
		"data.with_audio",
		"data.audio",
		"data.sound",
		"parameters.generate_audio",
		"parameters.with_audio",
		"metadata.generate_audio",
	)
	if platform == PlatformMiniMaxH3CompShare {
		metadata.HasAudio = !gjson.GetBytes(body, "mute_audio").Bool()
		metadata.OutputWidth, metadata.OutputHeight = 0, 0
	}

	switch {
	case platform == PlatformKling &&
		klingMode != "std" &&
		klingMode != "2x" &&
		klingMode != "pro" &&
		klingMode != "2x_pro" &&
		klingMode != "2x-pro" &&
		klingMode != "4k":
		metadata.ValidationError = fmt.Errorf("unsupported Kling mode %q", klingMode)
	case !durationFound || duration <= 0:
		metadata.ValidationError = fmt.Errorf("PP video request duration must be positive")
	}
	return metadata
}

func normalizePPVideoPublicModel(model string) string {
	return strings.TrimSpace(model)
}

func normalizePPVideoPublicDurationMilliseconds(body []byte) int64 {
	duration, found := extractPPVideoDurationMillisecondsWithPresence(body,
		"duration",
		"duration_seconds",
		"seconds",
		"data.duration",
		"data.duration_seconds",
		"parameters.duration",
		"parameters.duration_seconds",
		"metadata.duration",
		"metadata.duration_seconds",
		"video.duration",
		"video.duration_seconds",
	)
	if !found || duration <= 0 {
		return SiteVideoDefaultDurationMilliseconds
	}
	return duration
}

func normalizePPVideoPublicCount(body []byte) int {
	count := extractPPVideoPositiveInt(body,
		"video_count",
		"n",
		"count",
		"data.video_count",
		"parameters.video_count",
		"parameters.n",
		"metadata.video_count",
		"metadata.n",
		"video.count",
	)
	if count <= 0 {
		return 1
	}
	return count
}

func normalizePPVideoPublicResolution(body []byte) string {
	resolution := normalizePPVideoResolution(extractPPVideoText(body,
		"resolution",
		"size",
		"quality",
		"data.resolution",
		"data.size",
		"parameters.resolution",
		"parameters.size",
		"metadata.resolution",
		"metadata.size",
		"metadata.quality",
		"video.resolution",
		"video.size",
	))
	if resolution == "" {
		return SiteVideoDefaultResolution
	}
	return resolution
}

func ppVideoRequestHasImage(body []byte) bool {
	if strings.TrimSpace(extractPPVideoText(body,
		"image",
		"image_url",
		"input.image",
		"input.image_url",
		"data.image",
		"data.image_url",
		"parameters.image",
		"parameters.image_url",
		"input.first_frame_image",
		"first_frame_image",
		"data.first_frame_image",
	)) != "" {
		return true
	}
	images := gjson.GetBytes(body, "images")
	return images.Exists() && images.IsArray() && len(images.Array()) > 0
}

func normalizePPVideoSeedanceMediaFields(payload map[string]any, body []byte) error {
	if payload == nil {
		return fmt.Errorf("Seedance request body is required")
	}

	image := strings.TrimSpace(gjson.GetBytes(body, "image").String())
	imageURL := strings.TrimSpace(gjson.GetBytes(body, "image_url").String())
	images := gjson.GetBytes(body, "images")
	hasImages := images.Exists() && images.IsArray() && len(images.Array()) > 0

	referenceFields := make([]string, 0, 3)
	if hasImages {
		referenceFields = append(referenceFields, "images")
	}
	if image != "" {
		referenceFields = append(referenceFields, "image")
	}
	if imageURL != "" {
		referenceFields = append(referenceFields, "image_url")
	}
	if len(referenceFields) > 1 {
		return fmt.Errorf(
			"Seedance accepts only one reference image field: use images, image, or image_url",
		)
	}

	hasStartFrame := strings.TrimSpace(gjson.GetBytes(body, "start_frame_url").String()) != ""
	hasEndFrame := strings.TrimSpace(gjson.GetBytes(body, "end_frame_url").String()) != ""
	if len(referenceFields) > 0 && (hasStartFrame || hasEndFrame) {
		return fmt.Errorf(
			"Seedance cannot combine reference images with start_frame_url or end_frame_url; use either reference-image mode or frame-control mode",
		)
	}

	// Normalize legacy single-image aliases so Seedance always receives one
	// canonical reference-image field.
	if image != "" {
		payload["images"] = []string{image}
		delete(payload, "image")
	} else if imageURL != "" {
		payload["images"] = []string{imageURL}
		delete(payload, "image_url")
	}
	return nil
}

func ppVideoUpstreamModel(platform string, public PPVideoPublicRequest) string {
	model := strings.TrimSpace(public.Model)
	switch platform {
	case PlatformMiniMaxH3CompShare:
		return compShareVideoUpstreamModel(public, nil)
	case PlatformByteDance:
		if upstreamModel := byteDanceUpstreamModel(model); upstreamModel != "" {
			return upstreamModel
		}
		return ByteDanceVideoDefaultModel
	case PlatformWan3:
		return wan30UpstreamModel(model)
	case PlatformMiniMaxH3:
		if ppVideoIsMiniMaxModel(model) {
			return model
		}
		return MiniMaxH3VideoDefaultModel
	case PlatformPixverseV6:
		return PixverseV6VideoDefaultModel
	case PlatformGrokImagineVideo:
		return GrokImagineVideoDefaultModel
	case PlatformKuaishou:
		return KuaishouVideoDefaultModel
	case PlatformKling:
		if ppVideoIsPublicDefaultAlias(model) {
			return ppVideoKlingDefaultModel
		}
		return model
	case PlatformHappyHourse:
		if ppVideoIsHappyHorseModel(model) {
			return model
		}
		if ppVideoIsPublicDefaultAlias(model) && public.HasImage {
			return ppVideoHappyHorseImageDefaultModel
		}
		if ppVideoIsPublicDefaultAlias(model) {
			return ppVideoHappyHorseTextDefaultModel
		}
		return model
	case PlatformSeedance:
		if ppVideoIsPublicDefaultAlias(model) {
			return ppVideoSeedanceDefaultModel
		}
		return model
	default:
		return model
	}
}

func ppVideoUpstreamModelForAccount(platform string, public PPVideoPublicRequest, account *Account) string {
	if platform == PlatformMiniMaxH3CompShare {
		return compShareVideoUpstreamModel(public, account)
	}
	if platform == PlatformByteDance {
		if account != nil {
			if mappedModel, matched := account.ResolveMappedModel(public.Model); matched {
				if mappedModel = byteDanceUpstreamModel(mappedModel); mappedModel != "" {
					return mappedModel
				}
			}
			if strings.TrimSpace(public.Model) == "" {
				if mappedModel := ppVideoFirstMappedModelForAccount(platform, account); mappedModel != "" {
					if mappedModel = byteDanceUpstreamModel(mappedModel); mappedModel != "" {
						return mappedModel
					}
				}
			}
		}
		return ppVideoUpstreamModel(platform, public)
	}
	if platform == PlatformWan3 {
		return wan30UpstreamModel(public.Model)
	}
	if platform == PlatformMiniMaxH3 {
		if account != nil {
			if mappedModel, matched := account.ResolveMappedModel(public.Model); matched {
				if mappedModel = strings.TrimSpace(mappedModel); ppVideoIsMiniMaxModel(mappedModel) {
					return mappedModel
				}
			}
			if strings.TrimSpace(public.Model) == "" {
				if mappedModel := ppVideoFirstMappedModelForAccount(platform, account); mappedModel != "" {
					return mappedModel
				}
			}
		}
		return ppVideoUpstreamModel(platform, public)
	}
	if platform == PlatformPixverseV6 {
		return PixverseV6VideoDefaultModel
	}
	if platform == PlatformGrokImagineVideo {
		return GrokImagineVideoDefaultModel
	}
	if platform == PlatformKuaishou {
		return KuaishouVideoDefaultModel
	}
	if account != nil {
		if mappedModel, matched := account.ResolveMappedModel(public.Model); matched {
			if mappedModel = strings.TrimSpace(mappedModel); mappedModel != "" {
				return mappedModel
			}
		}
		if strings.TrimSpace(public.Model) == "" {
			if mappedModel := ppVideoFirstMappedModelForAccount(platform, account); mappedModel != "" {
				return mappedModel
			}
		}
	}
	return ppVideoUpstreamModel(platform, public)
}

func ppVideoFirstMappedModelForAccount(platform string, account *Account) string {
	if account == nil {
		return ""
	}
	mapping := account.GetModelMapping()
	if len(mapping) == 0 {
		return ""
	}

	collect := func(useValues bool) []string {
		candidates := make([]string, 0, len(mapping))
		for publicModel, upstreamModel := range mapping {
			candidate := publicModel
			if useValues {
				candidate = upstreamModel
			}
			candidate = strings.TrimSpace(candidate)
			if candidate != "" && ppVideoModelBelongsToPlatform(platform, candidate) {
				candidates = append(candidates, candidate)
			}
		}
		sort.Strings(candidates)
		return candidates
	}

	// Synced mappings use the upstream model as the value. Only fall back to
	// keys for legacy/custom mappings that do not carry a valid upstream value.
	if candidates := collect(true); len(candidates) > 0 {
		return candidates[0]
	}
	if candidates := collect(false); len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func ppVideoIsPublicDefaultAlias(model string) bool {
	model = strings.TrimSpace(model)
	return model == "" || model == ppVideoLegacySiteModel
}

func ppVideoModelBelongsToPlatform(platform string, model string) bool {
	switch platform {
	case PlatformMiniMaxH3CompShare:
		return model == MiniMaxH3VideoDefaultModel || model == CompShareVideoLiteModel
	case PlatformKling:
		return ppVideoIsKlingModel(model)
	case PlatformHappyHourse:
		return ppVideoIsHappyHorseModel(model)
	case PlatformSeedance:
		return ppVideoIsSeedanceModel(model)
	case PlatformByteDance:
		return ppVideoIsByteDanceModel(model)
	case PlatformWan3:
		return ppVideoIsWan30Model(model)
	case PlatformMiniMaxH3:
		return ppVideoIsMiniMaxModel(model)
	case PlatformPixverseV6:
		return ppVideoIsPixverseV6Model(model)
	case PlatformGrokImagineVideo:
		return ppVideoIsGrokImagineVideoModel(model)
	case PlatformKuaishou:
		return ppVideoIsKuaishouModel(model)
	default:
		return false
	}
}

func ppVideoModelKnownForeignToPlatform(platform string, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" || ppVideoIsPublicDefaultAlias(model) {
		return false
	}
	if ppVideoIsJimengModel(model) {
		return true
	}
	if ppVideoModelBelongsToPlatform(platform, model) {
		return false
	}
	for _, candidate := range []string{PlatformKling, PlatformHappyHourse, PlatformSeedance, PlatformByteDance, PlatformWan3, PlatformMiniMaxH3, PlatformMiniMaxH3CompShare, PlatformPixverseV6, PlatformGrokImagineVideo, PlatformKuaishou} {
		if candidate == platform {
			continue
		}
		if ppVideoModelBelongsToPlatform(candidate, model) {
			return true
		}
	}
	return false
}

func filterPPVideoPublicModelsForPlatform(platform string, models []string) []string {
	if !IsPPVideoPlatform(platform) {
		return models
	}

	filtered := make([]string, 0, len(models))
	for _, model := range models {
		if (platform == PlatformByteDance && ppVideoByteDanceModelAllowed(model)) ||
			(platform == PlatformWan3 && ppVideoWan30ModelAllowed(model)) ||
			(platform == PlatformMiniMaxH3 && ppVideoMiniMaxH3ModelAllowed(model)) ||
			(platform == PlatformPixverseV6 && ppVideoPixverseV6ModelAllowed(model)) ||
			(platform == PlatformGrokImagineVideo && ppVideoGrokImagineVideoModelAllowed(model)) ||
			(platform == PlatformKuaishou && ppVideoKuaishouModelAllowed(model)) ||
			(platform != PlatformByteDance && platform != PlatformWan3 && platform != PlatformMiniMaxH3 && platform != PlatformPixverseV6 && platform != PlatformGrokImagineVideo && platform != PlatformKuaishou &&
				(ppVideoIsPublicDefaultAlias(model) || ppVideoModelBelongsToPlatform(platform, model))) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

// FilterPPVideoPublicModelsForPlatform keeps only model IDs that belong to the
// selected PP video platform. It is shared by public model listing and admin
// account model inspection so both surfaces enforce the same isolation rules.
func FilterPPVideoPublicModelsForPlatform(platform string, models []string) []string {
	return filterPPVideoPublicModelsForPlatform(platform, models)
}

func ppVideoIsSeedanceModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "seedance") ||
		strings.HasPrefix(model, "doubao-seedance-")
}

func ppVideoIsByteDanceModel(model string) bool {
	return byteDanceUpstreamModel(model) != ""
}

func byteDanceModels() []string {
	return []string{
		ByteDanceVideoDefaultModel,
		ByteDanceSeedance25Model,
		ByteDanceSeedance25GlobalModel,
	}
}

func byteDanceUpstreamModel(model string) string {
	model = strings.TrimSpace(model)
	for _, candidate := range byteDanceModels() {
		if strings.EqualFold(model, candidate) {
			return candidate
		}
	}
	return ""
}

func byteDanceIsSeedance25Model(model string) bool {
	model = strings.TrimSpace(model)
	return strings.EqualFold(model, ByteDanceSeedance25Model) ||
		strings.EqualFold(model, ByteDanceSeedance25GlobalModel)
}

func byteDanceMaxDurationSeconds(model string) int {
	if byteDanceIsSeedance25Model(model) {
		return 30
	}
	return 15
}

func ppVideoByteDanceModelAllowed(model string) bool {
	model = strings.TrimSpace(model)
	return ppVideoIsPublicDefaultAlias(model) || ppVideoIsByteDanceModel(model)
}

func ppVideoIsWan30Model(model string) bool {
	model = strings.TrimSpace(model)
	return strings.EqualFold(model, Wan30VideoDefaultModel) ||
		strings.EqualFold(model, Wan30VideoPrimeModel)
}

func ppVideoWan30ModelAllowed(model string) bool {
	model = strings.TrimSpace(model)
	return ppVideoIsPublicDefaultAlias(model) || ppVideoIsWan30Model(model)
}

func wan30UpstreamModel(model string) string {
	if strings.EqualFold(strings.TrimSpace(model), Wan30VideoPrimeModel) {
		return Wan30VideoPrimeModel
	}
	return Wan30VideoDefaultModel
}

func ppVideoIsMiniMaxH3Model(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), MiniMaxH3VideoDefaultModel)
}

func ppVideoIsMiniMaxHailuo23Model(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), MiniMaxHailuo23VideoModel)
}

func ppVideoIsMiniMaxModel(model string) bool {
	return ppVideoIsMiniMaxH3Model(model) || ppVideoIsMiniMaxHailuo23Model(model)
}

func ppVideoMiniMaxH3ModelAllowed(model string) bool {
	model = strings.TrimSpace(model)
	return ppVideoIsPublicDefaultAlias(model) || ppVideoIsMiniMaxModel(model)
}

func ppVideoMiniMaxH3ModelAllowedForAccount(model string, account *Account) bool {
	model = strings.TrimSpace(model)
	if ppVideoMiniMaxH3ModelAllowed(model) {
		return true
	}
	if account == nil {
		return false
	}
	if mappedModel, matched := account.ResolveMappedModel(model); matched {
		return ppVideoIsMiniMaxModel(mappedModel)
	}
	return false
}

func ppVideoIsPixverseV6Model(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), PixverseV6VideoDefaultModel)
}

func ppVideoPixverseV6ModelAllowed(model string) bool {
	model = strings.TrimSpace(model)
	return ppVideoIsPublicDefaultAlias(model) || ppVideoIsPixverseV6Model(model)
}

func ppVideoIsGrokImagineVideoModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), GrokImagineVideoDefaultModel)
}

func ppVideoGrokImagineVideoModelAllowed(model string) bool {
	model = strings.TrimSpace(model)
	return ppVideoIsPublicDefaultAlias(model) || ppVideoIsGrokImagineVideoModel(model)
}

func ppVideoIsKuaishouModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), KuaishouVideoDefaultModel)
}

func ppVideoKuaishouModelAllowed(model string) bool {
	model = strings.TrimSpace(model)
	return ppVideoIsPublicDefaultAlias(model) || ppVideoIsKuaishouModel(model)
}

func ppVideoIsJimengModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "by-seedance")
}

func ppVideoIsHappyHorseModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "happyhorse-")
}

func ppVideoIsKlingModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "kling-")
}

func ppVideoDurationString(milliseconds int64) string {
	if milliseconds <= 0 {
		milliseconds = SiteVideoDefaultDurationMilliseconds
	}
	if milliseconds%1000 == 0 {
		return strconv.FormatInt(milliseconds/1000, 10)
	}
	text := strconv.FormatFloat(float64(milliseconds)/1000, 'f', 3, 64)
	return strings.TrimRight(strings.TrimRight(text, "0"), ".")
}

func ppVideoDurationJSONValue(milliseconds int64) any {
	if milliseconds <= 0 {
		milliseconds = SiteVideoDefaultDurationMilliseconds
	}
	if milliseconds%1000 == 0 {
		return int(milliseconds / 1000)
	}
	seconds, _ := strconv.ParseFloat(ppVideoDurationString(milliseconds), 64)
	return seconds
}

func ppVideoKlingDurationString(milliseconds int64) (string, error) {
	if milliseconds <= 0 {
		milliseconds = SiteVideoDefaultDurationMilliseconds
	}
	if milliseconds%1000 != 0 {
		return "", fmt.Errorf("Kling duration must be an integer number of seconds from 3 to 15")
	}
	seconds := milliseconds / 1000
	if seconds < 3 || seconds > 15 {
		return "", fmt.Errorf("Kling duration must be an integer number of seconds from 3 to 15")
	}
	return strconv.FormatInt(seconds, 10), nil
}

func ppVideoKlingModeFromRequest(body []byte, resolution string) (string, error) {
	rawMode := strings.TrimSpace(extractPPVideoText(body, "mode", "parameters.mode", "data.mode", "metadata.mode"))
	if rawMode == "" {
		return ppVideoKlingModeForResolution(resolution), nil
	}
	mode, ok := normalizePPVideoKlingMode(rawMode)
	if !ok {
		return "", fmt.Errorf("unsupported Kling mode %q", rawMode)
	}
	return mode, nil
}

func normalizePPVideoKlingMode(mode string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "std", "2x", "720", "720p":
		return "std", true
	case "pro", "2x_pro", "2x-pro", "1080", "1080p":
		return "pro", true
	case "4k", "uhd":
		return "4k", true
	default:
		return "", false
	}
}

func ppVideoKlingModeForResolution(resolution string) string {
	switch normalizePPVideoResolution(resolution) {
	case VideoBillingResolution1080P:
		return "pro"
	case "4k":
		return "4k"
	default:
		return "std"
	}
}

func ppVideoKlingRequestImage(body []byte) string {
	return extractPPVideoText(body,
		"image",
		"start_frame_url",
		"image_url",
		"input.image",
		"input.image_url",
		"data.image",
		"data.start_frame_url",
		"data.image_url",
		"parameters.image",
		"parameters.start_frame_url",
		"parameters.image_url",
		"images.0",
		"data.images.0",
		"parameters.images.0",
	)
}

func ppVideoKlingRequestImageTail(body []byte) string {
	return extractPPVideoText(body,
		"image_tail",
		"end_frame_url",
		"data.image_tail",
		"data.end_frame_url",
		"parameters.image_tail",
		"parameters.end_frame_url",
	)
}

func ppVideoKlingSoundFromRequest(body []byte) (string, bool, error) {
	for _, path := range []string{
		"sound",
		"generate_audio",
		"with_audio",
		"enable_audio",
		"data.sound",
		"data.generate_audio",
		"data.with_audio",
		"data.enable_audio",
		"parameters.sound",
		"parameters.generate_audio",
		"parameters.with_audio",
		"parameters.enable_audio",
		"metadata.sound",
		"metadata.generate_audio",
		"metadata.with_audio",
		"metadata.enable_audio",
	} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		sound, ok := normalizePPVideoKlingSoundValue(value)
		if !ok {
			return "", false, fmt.Errorf("Kling sound must be on or off")
		}
		return sound, true, nil
	}
	return "", false, nil
}

func normalizePPVideoKlingSoundValue(value gjson.Result) (string, bool) {
	switch value.Type {
	case gjson.True:
		return "on", true
	case gjson.False:
		return "off", true
	}
	switch strings.ToLower(strings.TrimSpace(value.String())) {
	case "on", "true", "1", "yes", "enabled":
		return "on", true
	case "off", "false", "0", "no", "disabled":
		return "off", true
	default:
		return "", false
	}
}

func ppVideoKeepOnlyKlingSupportedFields(payload map[string]any, operation PPVideoOperation) {
	allowed := map[string]struct{}{
		"model_name":      {},
		"prompt":          {},
		"negative_prompt": {},
		"multi_shot":      {},
		"shot_type":       {},
		"multi_prompt":    {},
		"mode":            {},
		"duration":        {},
		"aspect_ratio":    {},
		"sound":           {},
	}
	if operation == PPVideoOperationKlingImageToVideo {
		allowed["image"] = struct{}{}
		allowed["image_tail"] = struct{}{}
		allowed["element_list"] = struct{}{}
	}
	for key := range payload {
		if _, ok := allowed[key]; !ok {
			delete(payload, key)
		}
	}
}

func ppVideoExtractVideoURL(body []byte) string {
	return extractPPVideoText(body, ppVideoVideoURLPaths()...)
}

func ppVideoVideoURLPaths() []string {
	suffixes := []string{
		"task.content.url",
		"video_url",
		"result_url",
		"url",
		"download_url",
		"video_url_download",
		"video_urls.0",
		"urls.0",
		"content.video_url",
		"content.url",
		"result.video_url",
		"result.url",
		"result.video_urls.0",
		"result.urls.0",
		"task_result.video_url",
		"task_result.url",
		"task_result.video_urls.0",
		"task_result.urls.0",
		"task_result.videos.0.url",
		"task_result.videos.0.video_url",
		"task_result.videos.0.download_url",
		"task_result.videos.0.video_url_download",
		"output.video_url",
		"output.url",
		"output.video_urls.0",
		"output.urls.0",
		"output.0.video_url",
		"output.0.url",
		"outputs.0.video_url",
		"outputs.0.url",
		"videos.0.url",
		"videos.0.video_url",
		"videos.0.download_url",
		"videos.0.video_url_download",
	}
	prefixes := []string{"", "data.", "data.data.", "data.data.data."}
	paths := make([]string, 0, len(prefixes)*len(suffixes))
	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			paths = append(paths, prefix+suffix)
		}
	}
	return paths
}

func ppVideoResponseHasFinalVideo(body []byte) bool {
	return ppVideoExtractVideoURL(body) != ""
}

func ppVideoPublicUsage(raw []byte, public PPVideoPublicRequest, parsed PPVideoResponse) map[string]any {
	providerUsage := json.RawMessage(nil)
	if value := gjson.GetBytes(raw, "task.usage"); value.Exists() {
		providerUsage = json.RawMessage(value.Raw)
	} else if value := gjson.GetBytes(raw, "usage"); value.Exists() {
		providerUsage = json.RawMessage(value.Raw)
	} else if value := gjson.GetBytes(raw, "output.usage"); value.Exists() {
		providerUsage = json.RawMessage(value.Raw)
	} else if value := gjson.GetBytes(raw, "data.usage"); value.Exists() {
		providerUsage = json.RawMessage(value.Raw)
	} else if value := gjson.GetBytes(raw, "data.data.usage"); value.Exists() {
		providerUsage = json.RawMessage(value.Raw)
	}
	if parsed.GeneratedDurationMilliseconds <= 0 && len(providerUsage) == 0 {
		return nil
	}
	durationMs := parsed.GeneratedDurationMilliseconds
	if durationMs <= 0 {
		durationMs = public.DurationMilliseconds
	}
	count := parsed.VideoCount
	if count <= 0 {
		count = public.VideoCount
	}
	if count <= 0 {
		count = 1
	}
	resolution := normalizePPVideoResolution(parsed.VideoResolution)
	if resolution == "" {
		resolution = normalizePPVideoResolution(public.Resolution)
	}
	if gjson.GetBytes(raw, "task.resolution").Exists() {
		resolution = strings.ToLower(gjson.GetBytes(raw, "task.resolution").String())
	}
	usage := make(map[string]any)
	if durationMs > 0 {
		usage["duration_ms"] = durationMs
		usage["duration_seconds"] = float64(durationMs) / 1000
	}
	usage["video_count"] = count
	if resolution != "" {
		usage["resolution"] = resolution
	}
	if len(providerUsage) > 0 {
		usage["provider_usage"] = providerUsage
	}
	return usage
}

func extractPPVideoText(body []byte, paths ...string) string {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		if text := strings.TrimSpace(value.String()); text != "" {
			return text
		}
	}
	return ""
}

func extractPPVideoPositiveInt(body []byte, paths ...string) int {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value.String()))
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func extractPPVideoFloat(body []byte, paths ...string) float64 {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value.String()), 64)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func extractPPVideoBool(body []byte, paths ...string) bool {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		if value.Type == gjson.True {
			return true
		}
		switch strings.ToLower(strings.TrimSpace(value.String())) {
		case "true", "1", "yes", "on", "enabled":
			return true
		case "false", "0", "no", "off", "disabled":
			return false
		}
	}
	return false
}

func extractPPVideoDurationMilliseconds(body []byte, paths ...string) int64 {
	duration, _ := extractPPVideoDurationMillisecondsWithPresence(body, paths...)
	return duration
}

func extractPPVideoDurationMillisecondsWithPresence(body []byte, paths ...string) (int64, bool) {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		return parsePPVideoDurationMilliseconds(value.String()), true
	}
	return 0, false
}

func parsePPVideoDurationMilliseconds(raw string) int64 {
	seconds, ok := new(big.Rat).SetString(strings.TrimSpace(raw))
	if !ok || seconds.Sign() <= 0 {
		return 0
	}
	milliseconds := new(big.Rat).Mul(seconds, big.NewRat(1000, 1))
	whole := new(big.Int)
	remainder := new(big.Int)
	whole.QuoRem(milliseconds.Num(), milliseconds.Denom(), remainder)
	if remainder.Sign() != 0 {
		doubledRemainder := new(big.Int).Lsh(remainder, 1)
		if doubledRemainder.Cmp(milliseconds.Denom()) >= 0 {
			whole.Add(whole, big.NewInt(1))
		}
	}
	if !whole.IsInt64() {
		return 0
	}
	return whole.Int64()
}

func normalizePPVideoResolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "480", "480p", "sd":
		return VideoBillingResolution480P
	case "720", "720p", "768", "768p", "hd":
		return VideoBillingResolution720P
	case "1080", "1080p", "2k", "full_hd", "full-hd", "fhd":
		return VideoBillingResolution1080P
	default:
		return strings.ToLower(strings.TrimSpace(resolution))
	}
}

func ppVideoResolutionDimensions(resolution string) (int, int) {
	switch normalizePPVideoResolution(resolution) {
	case VideoBillingResolution480P:
		return 854, 480
	case VideoBillingResolution720P:
		return 1280, 720
	case VideoBillingResolution1080P:
		return 1920, 1080
	case "4k":
		return 3840, 2160
	default:
		return 0, 0
	}
}
