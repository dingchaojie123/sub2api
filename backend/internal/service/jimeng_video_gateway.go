package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type JimengVideoEndpoint string

const (
	JimengVideoEndpointGenerations JimengVideoEndpoint = "video_generations"
	JimengVideoEndpointStatus      JimengVideoEndpoint = "video_status"
	JimengVideoRoutingModel        string              = "seedance 2.0"
	JimengVideoBillingModel        string              = "video-v1"
	JimengVideoDefaultModel        string              = JimengVideoBillingModel
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

	targetURL := buildJimengVideoGenerationURL(baseURL)
	if endpoint == JimengVideoEndpointStatus {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			return nil, fmt.Errorf("jimeng task id is required")
		}
		targetURL = buildJimengVideoGenerationQueryURL(baseURL, taskID)
	}
	SetActualOpenAIUpstreamEndpoint(c, jimengVideoUpstreamEndpoint(endpoint))

	var bodyReader io.Reader
	if endpoint != JimengVideoEndpointStatus {
		if len(body) == 0 {
			return nil, fmt.Errorf("jimeng video request body is empty")
		}
		bodyReader = bytes.NewReader(body)
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, endpoint.httpMethod(), targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	upstreamReq.Header.Set("Accept", "application/json")
	if endpoint != JimengVideoEndpointStatus {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}
	account.ApplyHeaderOverrides(upstreamReq.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("jimeng upstream returned HTTP %d", resp.StatusCode)
	}

	parsed, err := parseJimengGenerationResult(respBody)
	if err != nil {
		return nil, err
	}
	writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)

	responseID := strings.TrimSpace(parsed.TaskID)
	if responseID == "" {
		responseID = strings.TrimSpace(taskID)
	}
	result := &OpenAIForwardResult{
		RequestID:        responseID,
		ResponseID:       responseID,
		Usage:            parsed.Usage,
		HasUsage:         parsed.HasUsage,
		Model:            JimengVideoDefaultModel,
		BillingModel:     JimengVideoDefaultModel,
		UpstreamModel:    JimengVideoDefaultModel,
		UpstreamEndpoint: jimengVideoUpstreamEndpoint(endpoint),
		ResponseHeaders:  resp.Header.Clone(),
		Duration:         time.Since(startTime),
	}
	if endpoint == JimengVideoEndpointGenerations {
		billingMeta := jimengVideoBillingMetadataFromRequest(body)
		result.HasUsage = true
		result.ImageCount = 1
		result.VideoCount = 1
		result.VideoResolution = billingMeta.VideoResolution
		result.VideoDurationSeconds = billingMeta.VideoDurationSeconds
	}
	return result, nil
}

func jimengVideoUpstreamEndpoint(endpoint JimengVideoEndpoint) string {
	switch endpoint {
	case JimengVideoEndpointStatus:
		return "/v1/video/generations/{task_id}"
	default:
		return "/v1/video/generations"
	}
}

type jimengVideoBillingMetadata struct {
	VideoResolution      string
	VideoDurationSeconds int
}

func jimengVideoBillingMetadataFromRequest(body []byte) jimengVideoBillingMetadata {
	return jimengVideoBillingMetadata{
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
		VideoDurationSeconds: NormalizeVideoBillingDurationSecondsOrDefault(firstJimengJSONInt(body,
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
