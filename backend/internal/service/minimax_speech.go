package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const AudioDefaultBaseURL = "https://api.modelverse.cn/v1"

const MiniMaxSpeechDefaultBaseURL = AudioDefaultBaseURL

const QwenTTSSpeechEndpoint = "/v1/audio/speech"

type QwenTTSSpeechRequest struct {
	Model    string          `json:"model"`
	Input    string          `json:"input"`
	Voice    string          `json:"voice"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type qwenTTSSpeechUsage struct {
	Characters int `json:"characters"`
}

type qwenTTSSpeechResponse struct {
	Code    string             `json:"code"`
	Usage   qwenTTSSpeechUsage `json:"usage"`
	Request string             `json:"request_id"`
}

// AudioSpeechForwardResult preserves the upstream payload for direct response
// passthrough and supplies normalized character usage for token billing.
type AudioSpeechForwardResult struct {
	Result      *OpenAIForwardResult
	StatusCode  int
	ContentType string
	Headers     http.Header
	Body        []byte
	Successful  bool
}

func IsAudioPlatform(platform string) bool {
	return platform == PlatformMiniMaxSpeech || platform == PlatformQwenTTS
}

func ParseQwenTTSSpeechRequest(body []byte) (QwenTTSSpeechRequest, error) {
	var request QwenTTSSpeechRequest
	if len(body) == 0 {
		return request, fmt.Errorf("request body is empty")
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return request, fmt.Errorf("request body must be valid JSON")
	}
	request.Model = strings.TrimSpace(request.Model)
	request.Input = strings.TrimSpace(request.Input)
	request.Voice = strings.TrimSpace(request.Voice)
	if request.Model == "" {
		return request, fmt.Errorf("model is required")
	}
	if request.Input == "" {
		return request, fmt.Errorf("input is required")
	}
	if utf8.RuneCountInString(request.Input) > 600 {
		return request, fmt.Errorf("input must not exceed 600 characters")
	}
	if request.Voice == "" {
		return request, fmt.Errorf("voice is required")
	}
	if len(request.Metadata) > 0 && string(request.Metadata) != "null" {
		var metadata struct {
			LanguageType string `json:"language_type"`
		}
		if err := json.Unmarshal(request.Metadata, &metadata); err != nil {
			return request, fmt.Errorf("metadata must be an object")
		}
		if language := strings.TrimSpace(metadata.LanguageType); language != "" {
			switch strings.ToLower(language) {
			case "chinese", "english", "german", "italian", "portuguese", "spanish", "japanese", "korean", "french", "russian", "auto":
			default:
				return request, fmt.Errorf("unsupported metadata.language_type")
			}
		}
	}
	return request, nil
}

func (s *OpenAIGatewayService) SelectAudioAccount(ctx context.Context, groupID *int64, platform, model string) (*AccountSelectionResult, error) {
	if s == nil || s.accountRepo == nil || !IsAudioPlatform(platform) {
		return nil, ErrNoAvailableAccounts
	}
	var (
		accounts []Account
		err      error
	)
	if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, platform)
	} else {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, platform)
	}
	if err != nil {
		return nil, err
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].Priority != accounts[j].Priority {
			return accounts[i].Priority < accounts[j].Priority
		}
		return accounts[i].ID < accounts[j].ID
	})
	for i := range accounts {
		account := accounts[i]
		if !account.IsModelSupported(model) || account.GetMappedModel(model) != model {
			continue
		}
		slot, acquireErr := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
		if acquireErr != nil || slot == nil || !slot.Acquired {
			continue
		}
		return &AccountSelectionResult{Account: &account, Acquired: true, ReleaseFunc: slot.ReleaseFunc}, nil
	}
	return nil, ErrNoAvailableAccounts
}

func (s *OpenAIGatewayService) ForwardQwenTTSSpeech(ctx context.Context, c *gin.Context, account *Account, body []byte, model string) (*AudioSpeechForwardResult, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, fmt.Errorf("audio upstream transport is unavailable")
	}
	if account == nil || account.Platform != PlatformQwenTTS {
		return nil, fmt.Errorf("Qwen TTS account is required")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("Qwen TTS model is required")
	}
	token := strings.TrimSpace(account.GetOpenAIApiKey())
	if token == "" {
		return nil, fmt.Errorf("Qwen TTS API key is not available")
	}
	baseURL, err := normalizeAudioBaseURL(account.GetOpenAIBaseURL())
	if err != nil {
		return nil, err
	}
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	req, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	started := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if c != nil {
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	}
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	result := &AudioSpeechForwardResult{
		StatusCode:  resp.StatusCode,
		ContentType: strings.TrimSpace(resp.Header.Get("Content-Type")),
		Headers:     resp.Header.Clone(),
		Body:        responseBody,
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, nil
	}
	var upstream qwenTTSSpeechResponse
	if err := json.Unmarshal(responseBody, &upstream); err != nil {
		return nil, fmt.Errorf("invalid Qwen TTS upstream response")
	}
	if strings.TrimSpace(upstream.Code) != "" {
		return result, nil
	}
	if upstream.Usage.Characters < 0 {
		return nil, fmt.Errorf("invalid Qwen TTS usage.characters")
	}
	result.Successful = true
	result.Result = &OpenAIForwardResult{
		RequestID:        strings.TrimSpace(upstream.Request),
		ResponseID:       strings.TrimSpace(upstream.Request),
		Model:            model,
		BillingModel:     model,
		UpstreamModel:    model,
		UpstreamEndpoint: QwenTTSSpeechEndpoint,
		Usage:            OpenAIUsage{InputTokens: upstream.Usage.Characters},
		HasUsage:         true,
		ResponseHeaders:  result.Headers,
		Duration:         time.Since(started),
	}
	return result, nil
}

func (s *OpenAIGatewayService) WriteAudioSpeechForwardResult(c *gin.Context, result *AudioSpeechForwardResult) {
	if c == nil || result == nil {
		return
	}
	headers := result.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	contentType := result.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	headers.Set("Content-Type", contentType)
	writeGrokMediaResponse(c, &http.Response{StatusCode: result.StatusCode, Header: headers}, result.Body, s.responseHeaderFilter)
}

func normalizeAudioBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = AudioDefaultBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid audio base URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("audio base URL must use http or https")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("audio base URL must not include a query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(parsed.Path, "/v1") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1"
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
