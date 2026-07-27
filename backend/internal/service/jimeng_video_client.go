package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Model    string
	Prompt   string
	Image    string
	Images   []string
	Duration int
	Extra    map[string]any
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
	payload := make(map[string]any, len(input.Extra)+6)
	for key, value := range input.Extra {
		key = strings.TrimSpace(key)
		if key != "" {
			payload[key] = value
		}
	}
	if strings.TrimSpace(input.Model) != "" {
		payload["model"] = strings.TrimSpace(input.Model)
	}
	if strings.TrimSpace(input.Prompt) != "" {
		payload["prompt"] = strings.TrimSpace(input.Prompt)
	}
	if strings.TrimSpace(input.Image) != "" {
		payload["image"] = strings.TrimSpace(input.Image)
	}
	if len(input.Images) > 0 {
		payload["images"] = input.Images
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
	result := &JimengVideoGenerationResult{
		TaskID: extractJimengString(value, "task_id", "request_id", "id", "data.task_id", "data.request_id", "data.id"),
		Status: NormalizeJimengTaskStatus(extractJimengString(value,
			"status", "state", "task_status", "data.status", "data.state", "data.task_status",
		)),
		Usage:    extractJimengUsage(value, "usage", "data.usage"),
		HasUsage: hasJimengUsage(value, "usage", "data.usage"),
		Raw:      append(json.RawMessage(nil), body...),
	}
	return result, nil
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
	case "pending", "processing", "running", "queued", "created":
		return JimengTaskStatusProcessing
	case "success", "succeeded", "completed", "complete", "done":
		return JimengTaskStatusSucceeded
	case "fail", "failed", "failure", "error", "cancelled", "canceled", "rejected", "refunded":
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
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	next, ok := obj[path[0]]
	if !ok {
		return nil, false
	}
	return extractJimengValueByPath(next, path[1:])
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
	return buildOpenAIEndpointURL(base, "/v1/video/generations")
}

func buildJimengModelsURL(base string) string {
	return buildOpenAIEndpointURL(base, "/v1/models")
}

func buildJimengVideoGenerationQueryURL(base string, taskID string) string {
	return strings.TrimRight(buildJimengVideoGenerationURL(base), "/") + "/" + url.PathEscape(taskID)
}
