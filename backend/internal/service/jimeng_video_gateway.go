package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type JimengVideoEndpoint string

const (
	JimengVideoEndpointGenerations JimengVideoEndpoint = "video_generations"
	JimengVideoEndpointStatus      JimengVideoEndpoint = "video_status"
	JimengVideoRoutingModel        string              = "by-seedance2.0-933"
	JimengVideoLegacyRoutingModel  string              = "seedance 2.0"
	JimengVideoBillingModel        string              = "video-v1"
	JimengVideoDefaultModel        string              = JimengVideoBillingModel
	jimengVideoPromptMaxRunes      int                 = 2000
)

func (e JimengVideoEndpoint) httpMethod() string {
	if e == JimengVideoEndpointStatus {
		return http.MethodGet
	}
	return http.MethodPost
}

func (s *OpenAIGatewayService) ForwardJimengVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint JimengVideoEndpoint,
	taskID string,
	body []byte,
	publicModels ...string,
) (*OpenAIForwardResult, error) {
	return s.forwardJimengVideo(ctx, c, account, endpoint, taskID, body, true, publicModels...)
}

func (s *OpenAIGatewayService) ForwardJimengVideoBuffered(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint JimengVideoEndpoint,
	taskID string,
	body []byte,
	publicModels ...string,
) (*OpenAIForwardResult, error) {
	return s.forwardJimengVideo(ctx, c, account, endpoint, taskID, body, false, publicModels...)
}

func (s *OpenAIGatewayService) forwardJimengVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint JimengVideoEndpoint,
	taskID string,
	body []byte,
	writeSuccessResponse bool,
	publicModels ...string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if s == nil || s.httpUpstream == nil {
		return nil, fmt.Errorf("jimeng upstream transport is unavailable")
	}
	if account == nil {
		return nil, fmt.Errorf("jimeng account is required")
	}
	if account.Platform != PlatformJimeng {
		return nil, fmt.Errorf("account platform %s is not supported for jimeng video", account.Platform)
	}
	token := strings.TrimSpace(account.GetOpenAIApiKey())
	if token == "" {
		return nil, fmt.Errorf("jimeng api key not found in credentials")
	}
	baseURL, err := normalizeJimengBaseURL(account.GetOpenAIBaseURL())
	if err != nil {
		return nil, err
	}

	targets, err := jimengVideoUpstreamTargets(baseURL, endpoint, taskID)
	if err != nil {
		return nil, err
	}
	SetActualOpenAIUpstreamEndpoint(c, targets[0].endpoint)

	var upstreamBody []byte
	publicModel := jimengVideoPublicModelFromArgs(publicModels...)
	upstreamModel := ""
	if endpoint != JimengVideoEndpointStatus {
		if len(body) == 0 {
			return nil, fmt.Errorf("jimeng video request body is empty")
		}
		if publicModel == "" {
			publicModel = JimengVideoRequestedModelFromBody(body)
		}
		publicModel = NormalizeJimengVideoRequestedModel(publicModel)
		upstreamModel = jimengVideoUpstreamModelForAccount(account, publicModel)
		upstreamBody, err = normalizeJimengVideoGenerationBody(body, upstreamModel)
		if err != nil {
			return nil, err
		}
	}
	if publicModel == "" {
		publicModel = JimengVideoRoutingModel
	}
	if upstreamModel == "" {
		upstreamModel = jimengVideoUpstreamModelForAccount(account, publicModel)
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()

	for idx, target := range targets {
		var bodyReader io.Reader
		if endpoint != JimengVideoEndpointStatus {
			bodyReader = bytes.NewReader(upstreamBody)
		}
		upstreamReq, err := http.NewRequestWithContext(upstreamCtx, endpoint.httpMethod(), target.url, bodyReader)
		if err != nil {
			return nil, err
		}
		upstreamReq.Header.Set("Authorization", "Bearer "+token)
		upstreamReq.Header.Set("Accept", "application/json")
		if endpoint != JimengVideoEndpointStatus {
			upstreamReq.Header.Set("Content-Type", "application/json")
		}
		account.ApplyHeaderOverrides(upstreamReq.Header)
		SetActualOpenAIUpstreamEndpoint(c, target.endpoint)

		resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if err != nil {
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
		}

		respBody, readErr := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode >= http.StatusBadRequest {
			if shouldFallbackJimengVideoEndpoint(resp.StatusCode) && idx+1 < len(targets) {
				continue
			}
			writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)
			return nil, fmt.Errorf("jimeng upstream returned HTTP %d", resp.StatusCode)
		}

		parsed, err := parseJimengGenerationResult(respBody)
		if err != nil {
			return nil, err
		}
		responseBody := NormalizeJimengVideoPublicResponse(respBody, parsed, publicModel)
		if writeSuccessResponse {
			writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
		}

		responseID := strings.TrimSpace(parsed.TaskID)
		if responseID == "" {
			responseID = strings.TrimSpace(taskID)
		}
		result := &OpenAIForwardResult{
			RequestID:           responseID,
			ResponseID:          responseID,
			Usage:               parsed.Usage,
			HasUsage:            parsed.HasUsage,
			Model:               publicModel,
			BillingModel:        JimengVideoBillingModel,
			UpstreamModel:       upstreamModel,
			UpstreamEndpoint:    target.endpoint,
			ResponseHeaders:     resp.Header.Clone(),
			Duration:            time.Since(startTime),
			TaskStatus:          parsed.Status,
			ResponseStatusCode:  resp.StatusCode,
			ResponseContentType: strings.TrimSpace(resp.Header.Get("Content-Type")),
			ResponseBody:        append([]byte(nil), responseBody...),
		}
		if endpoint == JimengVideoEndpointGenerations {
			billingMeta := jimengVideoBillingMetadataFromRequest(body)
			result.HasUsage = false
			result.ImageCount = 1
			result.VideoCount = 1
			result.VideoResolution = billingMeta.VideoResolution
			result.VideoDurationSeconds = billingMeta.VideoDurationSeconds
		}
		return result, nil
	}

	return nil, fmt.Errorf("jimeng upstream returned no usable response")
}

func ValidateJimengVideoGenerationRequestBody(body []byte) error {
	if !gjson.ValidBytes(body) {
		return nil
	}
	prompt := extractPPVideoText(body, "prompt", "data.prompt", "parameters.prompt", "input.prompt")
	return validateVideoPromptFieldLength("Jimeng", "prompt", prompt, jimengVideoPromptMaxRunes)
}

func normalizeJimengVideoGenerationBody(body []byte, upstreamModel string) ([]byte, error) {
	if err := ValidateJimengVideoGenerationRequestBody(body); err != nil {
		return nil, err
	}
	if !gjson.ValidBytes(body) {
		return body, nil
	}

	if upstreamModel == "" {
		upstreamModel = NormalizeJimengVideoRequestedModel(gjson.GetBytes(body, "model").String())
	}

	normalized, err := sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return body, nil
	}
	normalized, err = sjson.SetBytes(normalized, "async", true)
	if err != nil {
		return body, nil
	}
	return normalized, nil
}

func NormalizeJimengVideoRequestedModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" ||
		strings.EqualFold(model, JimengVideoLegacyRoutingModel) ||
		strings.EqualFold(model, JimengVideoBillingModel) {
		return JimengVideoRoutingModel
	}
	return model
}

func JimengVideoRequestedModelFromBody(body []byte) string {
	if !gjson.ValidBytes(body) {
		return JimengVideoRoutingModel
	}
	return NormalizeJimengVideoRequestedModel(gjson.GetBytes(body, "model").String())
}

func jimengVideoUpstreamModelForAccount(account *Account, publicModel string) string {
	publicModel = NormalizeJimengVideoRequestedModel(publicModel)
	if account != nil {
		if mappedModel, matched := account.ResolveMappedModel(publicModel); matched {
			if mappedModel = strings.TrimSpace(mappedModel); mappedModel != "" {
				return mappedModel
			}
		}
	}
	return publicModel
}

func jimengVideoPublicModelFromArgs(models ...string) string {
	for _, model := range models {
		if strings.TrimSpace(model) == "" {
			continue
		}
		if normalized := NormalizeJimengVideoRequestedModel(model); normalized != "" {
			return normalized
		}
	}
	return ""
}

func NormalizeJimengVideoPublicResponse(raw []byte, parsed *JimengVideoGenerationResult, publicModels ...string) []byte {
	if len(raw) == 0 || !gjson.ValidBytes(raw) {
		return raw
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return raw
	}
	taskID := ""
	status := ""
	if parsed != nil {
		taskID = strings.TrimSpace(parsed.TaskID)
		status = NormalizeJimengTaskStatus(parsed.Status)
	}
	if taskID == "" {
		taskID = extractJimengStringFromBytes(raw, jimengVideoTaskIDPaths()...)
	}
	if taskID != "" {
		payload["id"] = taskID
		payload["task_id"] = taskID
	}
	payload["object"] = "video.generation.task"
	if status == "" {
		status = NormalizeJimengTaskStatus(extractJimengStringFromBytes(raw, jimengVideoStatusPaths()...))
	}
	if jimengVideoResponseHasFinalVideo(raw) && status != JimengTaskStatusFailed {
		status = JimengTaskStatusSucceeded
	}
	if status != "" {
		payload["status"] = status
	}
	responseModel := jimengVideoPublicModelFromArgs(publicModels...)
	if responseModel == "" {
		responseModel = JimengVideoRoutingModel
	}
	payload["model"] = responseModel
	if videoURL := jimengVideoExtractVideoURL(raw); videoURL != "" {
		jimengVideoSetPublicVideoURL(payload, videoURL)
	}
	if usage := jimengVideoPublicUsage(raw, parsed); len(usage) > 0 {
		payload["usage"] = usage
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return normalized
}

func jimengVideoSetPublicVideoURL(payload map[string]any, videoURL string) {
	videoURL = strings.TrimSpace(videoURL)
	if payload == nil || videoURL == "" {
		return
	}
	payload["video_url"] = videoURL
	payload["url"] = videoURL
	payload["result_url"] = videoURL
	jimengVideoSetVideosField(payload, videoURL)
	jimengVideoSetDataVideoURL(payload, videoURL)
}

func jimengVideoSetDataVideoURL(payload map[string]any, videoURL string) {
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
	jimengVideoSetVideosField(data, videoURL)
}

func jimengVideoSetVideosField(payload map[string]any, videoURL string) {
	if payload == nil || strings.TrimSpace(videoURL) == "" {
		return
	}
	if videos, ok := payload["videos"].([]any); ok && len(videos) > 0 {
		return
	}
	payload["videos"] = []any{map[string]any{"url": videoURL, "video_url": videoURL}}
}

func jimengVideoResponseHasFinalVideo(body []byte) bool {
	return jimengVideoExtractFinalVideoURL(body) != ""
}

func jimengVideoExtractFinalVideoURL(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return ""
	}
	return extractJimengVideoURL(value)
}

var jimengVideoURLFields = []string{
	"video_url",
	"mp4_url",
	"url",
	"result_url",
	"download_url",
	"video_url_download",
	"file_url",
	"media_url",
	"play_url",
}

var jimengVideoIgnoredURLFields = map[string]struct{}{
	"status_url":         {},
	"image_references":   {},
	"ref_images":         {},
	"start_frame_url":    {},
	"end_frame_url":      {},
	"reference_images":   {},
	"reference_videos":   {},
	"reference_audios":   {},
}

func extractJimengVideoURL(value any) string {
	var walk func(any) string
	walk = func(current any) string {
		switch typed := current.(type) {
		case map[string]any:
			for _, field := range jimengVideoURLFields {
				if _, ignored := jimengVideoIgnoredURLFields[field]; ignored {
					continue
				}
				if candidate, ok := typed[field].(string); ok {
					if candidate = strings.TrimSpace(candidate); candidate != "" {
						return candidate
					}
				}
			}
			keys := make([]string, 0, len(typed))
			for key := range typed {
				if _, ignored := jimengVideoIgnoredURLFields[key]; ignored {
					continue
				}
				skip := false
				for _, field := range jimengVideoURLFields {
					if key == field {
						skip = true
						break
					}
				}
				if !skip {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				if candidate := walk(typed[key]); candidate != "" {
					return candidate
				}
			}
		case []any:
			for _, item := range typed {
				if candidate := walk(item); candidate != "" {
					return candidate
				}
			}
		}
		return ""
	}
	return walk(value)
}

func jimengVideoExtractVideoURL(body []byte) string {
	if videoURL := jimengVideoExtractFinalVideoURL(body); videoURL != "" {
		return videoURL
	}
	return extractJimengStringFromBytes(body,
		"url",
		"download_url",
		"result_url",
	)
}

func jimengVideoPublicUsage(raw []byte, parsed *JimengVideoGenerationResult) map[string]any {
	providerUsage := json.RawMessage(nil)
	if value := gjson.GetBytes(raw, "usage"); value.Exists() {
		providerUsage = json.RawMessage(value.Raw)
	} else if value := gjson.GetBytes(raw, "data.usage"); value.Exists() {
		providerUsage = json.RawMessage(value.Raw)
	}
	if len(providerUsage) == 0 && (parsed == nil || !parsed.HasUsage) {
		return nil
	}
	usage := make(map[string]any)
	if parsed != nil && parsed.HasUsage {
		usage["input_tokens"] = parsed.Usage.InputTokens
		usage["output_tokens"] = parsed.Usage.OutputTokens
		usage["image_input_tokens"] = parsed.Usage.ImageInputTokens
		usage["image_output_tokens"] = parsed.Usage.ImageOutputTokens
		usage["cache_creation_input_tokens"] = parsed.Usage.CacheCreationInputTokens
		usage["cache_read_input_tokens"] = parsed.Usage.CacheReadInputTokens
	}
	if len(providerUsage) > 0 {
		usage["provider_usage"] = providerUsage
	}
	return usage
}

func extractJimengStringFromBytes(body []byte, paths ...string) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
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

func (s *OpenAIGatewayService) WriteJimengVideoForwardResult(c *gin.Context, result *OpenAIForwardResult) {
	if c == nil || result == nil {
		return
	}
	status := result.ResponseStatusCode
	if status == 0 {
		status = http.StatusOK
	}
	contentType := strings.TrimSpace(result.ResponseContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	headers := result.ResponseHeaders.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", contentType)
	}
	resp := &http.Response{
		StatusCode: status,
		Header:     headers,
	}
	writeGrokMediaResponse(c, resp, result.ResponseBody, s.responseHeaderFilter)
}

func jimengVideoUpstreamEndpoint(endpoint JimengVideoEndpoint) string {
	switch endpoint {
	case JimengVideoEndpointStatus:
		return "/v1/videos/{task_id}"
	default:
		return "/v1/videos/generations"
	}
}

type jimengVideoUpstreamTarget struct {
	url      string
	endpoint string
}

func jimengVideoLegacyUpstreamEndpoint(endpoint JimengVideoEndpoint) string {
	switch endpoint {
	case JimengVideoEndpointStatus:
		return "/v1/video/generations/{task_id}"
	default:
		return "/v1/video/generations"
	}
}

func jimengVideoUpstreamTargets(baseURL string, endpoint JimengVideoEndpoint, taskID string) ([]jimengVideoUpstreamTarget, error) {
	if endpoint == JimengVideoEndpointStatus {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			return nil, fmt.Errorf("jimeng task id is required")
		}
		return []jimengVideoUpstreamTarget{
			{url: buildJimengVideoGenerationQueryURL(baseURL, taskID), endpoint: jimengVideoUpstreamEndpoint(endpoint)},
			{url: buildLegacyJimengVideoGenerationQueryURL(baseURL, taskID), endpoint: jimengVideoLegacyUpstreamEndpoint(endpoint)},
		}, nil
	}
	return []jimengVideoUpstreamTarget{
		{url: buildJimengVideoGenerationURL(baseURL), endpoint: jimengVideoUpstreamEndpoint(endpoint)},
		{url: buildLegacyJimengVideoGenerationURL(baseURL), endpoint: jimengVideoLegacyUpstreamEndpoint(endpoint)},
	}, nil
}

func shouldFallbackJimengVideoEndpoint(status int) bool {
	return status == http.StatusNotFound || status == http.StatusMethodNotAllowed
}

type JimengVideoBillingMetadata struct {
	VideoResolution      string
	VideoDurationSeconds int
}

func jimengVideoBillingMetadataFromRequest(body []byte) JimengVideoBillingMetadata {
	return JimengVideoBillingMetadata{
		VideoResolution: NormalizeVideoBillingResolutionOrDefault(firstJimengJSONText(body,
			"resolution",
			"size",
			"quality",
			"data.resolution",
			"data.size",
			"parameters.resolution",
			"parameters.size",
			"video.resolution",
			"video.size",
		)),
		VideoDurationSeconds: NormalizeJimengVideoDurationSecondsOrDefault(firstJimengJSONInt(body,
			"duration",
			"duration_seconds",
			"seconds",
			"data.duration",
			"data.duration_seconds",
			"parameters.duration",
			"parameters.duration_seconds",
			"video.duration",
			"video.duration_seconds",
		)),
	}
}

const jimengVideoDefaultDurationSeconds = 5

func NormalizeJimengVideoDurationSecondsOrDefault(durationSeconds int) int {
	if durationSeconds <= 0 {
		return jimengVideoDefaultDurationSeconds
	}
	return NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds)
}

func JimengVideoBillingMetadataFromRequest(body []byte) JimengVideoBillingMetadata {
	return jimengVideoBillingMetadataFromRequest(body)
}

func firstJimengJSONText(body []byte, paths ...string) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if value.Exists() {
			if text := strings.TrimSpace(value.String()); text != "" {
				return text
			}
		}
	}
	return ""
}

func firstJimengJSONInt(body []byte, paths ...string) int {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return 0
	}
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		if value.Type == gjson.String {
			parsed, err := strconv.Atoi(strings.TrimSpace(value.String()))
			if err == nil {
				return parsed
			}
			continue
		}
		return int(value.Int())
	}
	return 0
}
