package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	JimengTaskStatusProcessing = "processing"
	JimengTaskStatusSucceeded  = "succeeded"
	JimengTaskStatusFailed     = "failed"

	jimengResponseBodyLimit int64 = 2 << 20
)

type JimengVideoClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type JimengVideoGenerationRequest struct {
	Model         string
	Prompt        string
	Image         string
	Images        []string
	Videos        []string
	Audios        []string
	AspectRatio   string
	Resolution    string
	GenerateAudio *bool
	StartFrameURL string
	EndFrameURL   string
	Seed          *int
	Duration      int
	Extra         map[string]any
}

type JimengVideoGenerationResult struct {
	TaskID   string
	Status   string
	Usage    OpenAIUsage
	HasUsage bool
	Raw      json.RawMessage
}

func NewJimengVideoClient(baseURL string, apiKey string, httpClient *http.Client) (*JimengVideoClient, error) {
	normalizedBaseURL, err := normalizeJimengBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("jimeng api key is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &JimengVideoClient{baseURL: normalizedBaseURL, apiKey: apiKey, httpClient: httpClient}, nil
}

func (c *JimengVideoClient) CreateGeneration(ctx context.Context, input JimengVideoGenerationRequest) (*JimengVideoGenerationResult, error) {
	if c == nil {
		return nil, fmt.Errorf("jimeng client is nil")
	}
	payload := make(map[string]any, len(input.Extra)+14)
	for key, value := range input.Extra {
		key = strings.TrimSpace(key)
		if key != "" {
			payload[key] = value
		}
	}
	if strings.TrimSpace(input.Model) != "" {
		payload["model"] = strings.TrimSpace(input.Model)
	}
	prompt := strings.TrimSpace(input.Prompt)
	if err := validateVideoPromptFieldLength("Jimeng", "prompt", prompt, jimengVideoPromptMaxRunes); err != nil {
		return nil, err
	}
	if prompt != "" {
		payload["prompt"] = prompt
	}
	if strings.TrimSpace(input.Image) != "" {
		payload["image"] = strings.TrimSpace(input.Image)
	}
	if len(input.Images) > 0 {
		payload["images"] = input.Images
	}
	if len(input.Videos) > 0 {
		payload["videos"] = input.Videos
	}
	if len(input.Audios) > 0 {
		payload["audios"] = input.Audios
	}
	if value := strings.TrimSpace(input.AspectRatio); value != "" {
		payload["aspect_ratio"] = value
	}
	if value := strings.TrimSpace(input.Resolution); value != "" {
		payload["resolution"] = value
	}
	if input.GenerateAudio != nil {
		payload["generate_audio"] = *input.GenerateAudio
	}
	if value := strings.TrimSpace(input.StartFrameURL); value != "" {
		payload["start_frame_url"] = value
	}
	if value := strings.TrimSpace(input.EndFrameURL); value != "" {
		payload["end_frame_url"] = value
	}
	if input.Seed != nil {
		payload["seed"] = *input.Seed
	}
	if input.Duration > 0 {
		payload["duration"] = input.Duration
	}
	payload["async"] = true

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal jimeng create request: %w", err)
	}
	req, err := c.newJSONRequest(ctx, http.MethodPost, buildJimengVideoGenerationURL(c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return c.doGenerationRequest(req)
}

func (c *JimengVideoClient) GetGeneration(ctx context.Context, taskID string) (*JimengVideoGenerationResult, error) {
	if c == nil {
		return nil, fmt.Errorf("jimeng client is nil")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("jimeng task id is required")
	}
	req, err := c.newJSONRequest(ctx, http.MethodGet, buildJimengVideoGenerationQueryURL(c.baseURL, taskID), nil)
	if err != nil {
		return nil, err
	}
	return c.doGenerationRequest(req)
}

func ProbeJimengAPIKey(ctx context.Context, baseURL string, apiKey string, httpClient *http.Client) (bool, error) {
	client, err := NewJimengVideoClient(baseURL, apiKey, httpClient)
	if err != nil {
		return false, err
	}
	req, err := client.newJSONRequest(ctx, http.MethodGet, buildJimengModelsURL(client.baseURL), nil)
	if err != nil {
		return false, err
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request jimeng models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, jimengResponseBodyLimit))
	return decideJimengAPIKeyProbe(resp.StatusCode)
}

func decideJimengAPIKeyProbe(status int) (bool, error) {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return true, nil
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return false, fmt.Errorf("jimeng api key is invalid: upstream returned HTTP %d", status)
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return true, nil
	default:
		return false, fmt.Errorf("jimeng models probe failed with HTTP %d", status)
	}
}

func (c *JimengVideoClient) newJSONRequest(ctx context.Context, method string, requestURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("build jimeng request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *JimengVideoClient) doGenerationRequest(req *http.Request) (*JimengVideoGenerationResult, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request jimeng upstream: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, jimengResponseBodyLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read jimeng response: %w", err)
	}
	if int64(len(body)) > jimengResponseBodyLimit {
		return nil, fmt.Errorf("jimeng response exceeds %d bytes", jimengResponseBodyLimit)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("jimeng upstream returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return parseJimengGenerationResult(body)
}

func parseJimengGenerationResult(body []byte) (*JimengVideoGenerationResult, error) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, fmt.Errorf("parse jimeng response: %w", err)
	}
	status := NormalizeJimengTaskStatus(extractJimengString(value, jimengVideoStatusPaths()...))
	if jimengVideoResponseHasFinalVideo(body) && status != JimengTaskStatusFailed {
		status = JimengTaskStatusSucceeded
	}
	result := &JimengVideoGenerationResult{
		TaskID:   extractJimengString(value, jimengVideoTaskIDPaths()...),
		Status:   status,
		Usage:    extractJimengUsage(value, jimengVideoUsagePaths()...),
		HasUsage: hasJimengUsage(value, jimengVideoUsagePaths()...),
		Raw:      append(json.RawMessage(nil), body...),
	}
	return result, nil
}

func jimengVideoTaskIDPaths() []string {
	return []string{
		"task_id", "request_id", "id",
		"data.0.task_id", "data.0.request_id", "data.0.id",
		"data.task_id", "data.request_id", "data.id",
		"data.data.task_id", "data.data.request_id", "data.data.id",
		"data.data.data.task_id", "data.data.data.request_id", "data.data.data.id",
	}
}

func jimengVideoStatusPaths() []string {
	return []string{
		"status", "state", "task_status",
		"data.0.status", "data.0.state", "data.0.task_status",
		"data.status", "data.state", "data.task_status",
		"data.data.status", "data.data.state", "data.data.task_status",
		"data.data.data.status", "data.data.data.state", "data.data.data.task_status",
	}
}

func jimengVideoUsagePaths() []string {
	return []string{"usage", "data.usage", "data.data.usage", "data.data.data.usage"}
}

func extractJimengUsage(value any, keys ...string) OpenAIUsage {
	for _, key := range keys {
		if usageValue, ok := extractJimengValueByPath(value, strings.Split(key, ".")); ok {
			return OpenAIUsage{
				InputTokens:              extractJimengInt(usageValue, "input_tokens", "prompt_tokens"),
				OutputTokens:             extractJimengInt(usageValue, "output_tokens", "completion_tokens"),
				ImageInputTokens:         extractJimengInt(usageValue, "image_input_tokens"),
				ImageOutputTokens:        extractJimengInt(usageValue, "image_output_tokens"),
				CacheCreationInputTokens: extractJimengInt(usageValue, "cache_creation_input_tokens"),
				CacheReadInputTokens:     extractJimengInt(usageValue, "cache_read_input_tokens", "cached_tokens"),
			}
		}
	}
	return OpenAIUsage{}
}

func hasJimengUsage(value any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := extractJimengValueByPath(value, strings.Split(key, ".")); ok {
			return true
		}
	}
	return false
}

func extractJimengInt(value any, keys ...string) int {
	for _, key := range keys {
		if v, ok := extractJimengValueByPath(value, strings.Split(key, ".")); ok {
			switch typed := v.(type) {
			case float64:
				return int(typed)
			case int:
				return typed
			case json.Number:
				i, _ := typed.Int64()
				return int(i)
			}
		}
	}
	return 0
}

func NormalizeJimengTaskStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "pending", "processing", "running", "queued", "created", "submitted", "in_progress", "waiting":
		return JimengTaskStatusProcessing
	case "success", "succeeded", "completed", "complete", "done", "finished", "successful":
		return JimengTaskStatusSucceeded
	case "fail", "failed", "failure", "error", "cancelled", "canceled", "rejected", "refunded", "timeout", "timed_out", "expired":
		return JimengTaskStatusFailed
	default:
		return normalized
	}
}

func extractJimengString(value any, keys ...string) string {
	for _, key := range keys {
		if v := extractJimengStringByPath(value, strings.Split(key, ".")); v != "" {
			return v
		}
	}
	return ""
}

func extractJimengStringByPath(value any, path []string) string {
	value, ok := extractJimengValueByPath(value, path)
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", v))
	default:
		return ""
	}
}

func extractJimengValueByPath(value any, path []string) (any, bool) {
	if len(path) == 0 {
		return value, true
	}
	switch typed := value.(type) {
	case map[string]any:
		next, ok := typed[path[0]]
		if !ok {
			return nil, false
		}
		return extractJimengValueByPath(next, path[1:])
	case []any:
		index, err := strconv.Atoi(path[0])
		if err != nil || index < 0 || index >= len(typed) {
			return nil, false
		}
		return extractJimengValueByPath(typed[index], path[1:])
	default:
		return nil, false
	}
}

func normalizeJimengBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", fmt.Errorf("jimeng base url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid jimeng base url: %q", raw)
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	parsed.RawPath = ""
	return parsed.String(), nil
}

func buildJimengVideoGenerationURL(base string) string {
	return buildOpenAIEndpointURL(base, jimengVideoGenerationPath())
}

func buildLegacyJimengVideoGenerationURL(base string) string {
	return buildOpenAIEndpointURL(base, legacyJimengVideoGenerationPath())
}

func buildJimengModelsURL(base string) string {
	return buildOpenAIEndpointURL(base, "/v1/models")
}

func buildJimengVideoGenerationQueryURL(base string, taskID string) string {
	return strings.TrimRight(buildOpenAIEndpointURL(base, "/v1/videos"), "/") + "/" + url.PathEscape(taskID)
}

func buildLegacyJimengVideoGenerationQueryURL(base string, taskID string) string {
	return strings.TrimRight(buildLegacyJimengVideoGenerationURL(base), "/") + "/" + url.PathEscape(taskID)
}

func jimengVideoGenerationPath() string {
	return "/v1/videos/generations"
}

func legacyJimengVideoGenerationPath() string {
	return "/v1/video/generations"
}
