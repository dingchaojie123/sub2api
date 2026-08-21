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

	ppVideoLegacySiteModel             = "video-v1"
	ppVideoSeedanceDefaultModel        = "doubao-seedance-2-0-260128"
	ppVideoHappyHorseTextDefaultModel  = "happyhorse-1.0-t2v"
	ppVideoHappyHorseImageDefaultModel = "happyhorse-1.0-i2v"
	ppVideoKlingDefaultModel           = "kling-v3"
	ppVideoPublicResponseObject        = "video.generation.task"
)

type PPVideoOperation string

const (
	PPVideoOperationGeneric           PPVideoOperation = "generic"
	PPVideoOperationKlingTextToVideo  PPVideoOperation = "kling_text_to_video"
	PPVideoOperationKlingImageToVideo PPVideoOperation = "kling_image_to_video"
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
	case PlatformKling, PlatformHappyHourse, PlatformSeedance:
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

	switch platform {
	case PlatformKling:
		if err := normalizePPVideoKlingPayload(payload, operation, body, &public); err != nil {
			return nil, PPVideoPublicRequest{}, err
		}
	case PlatformSeedance:
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
	}

	prepared, err := json.Marshal(payload)
	if err != nil {
		return nil, PPVideoPublicRequest{}, fmt.Errorf("marshal PP video request body: %w", err)
	}
	return prepared, public, nil
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
	if strings.TrimSpace(public.Prompt) != "" && strings.TrimSpace(extractPPVideoText(body, "prompt")) == "" {
		payload["prompt"] = public.Prompt
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

	result := PPVideoResponse{
		TaskID: extractPPVideoText(body,
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
	if ppVideoResponseHasFinalVideo(body) && result.Status != PPVideoTaskStatusFailed {
		result.Status = PPVideoTaskStatusSucceeded
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
	case PlatformKling:
		return ppVideoIsKlingModel(model)
	case PlatformHappyHourse:
		return ppVideoIsHappyHorseModel(model)
	case PlatformSeedance:
		return ppVideoIsSeedanceModel(model)
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
	for _, candidate := range []string{PlatformKling, PlatformHappyHourse, PlatformSeedance} {
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
		if ppVideoIsPublicDefaultAlias(model) || ppVideoModelBelongsToPlatform(platform, model) {
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
	if value := gjson.GetBytes(raw, "usage"); value.Exists() {
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
	case "720", "720p", "hd":
		return VideoBillingResolution720P
	case "1080", "1080p", "full_hd", "full-hd", "fhd":
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
