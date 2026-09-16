package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ForwardPPVideoBuffered forwards one PP video submission or status request
// without writing the response. The caller persists the response before it is
// sent back to the client so billing state cannot be lost.
func (s *OpenAIGatewayService) ForwardPPVideoBuffered(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	operation PPVideoOperation,
	taskID string,
	body []byte,
	publicRequests ...PPVideoPublicRequest,
) (*OpenAIForwardResult, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, fmt.Errorf("PP video upstream transport is unavailable")
	}
	if account == nil || !IsPPVideoPlatform(account.Platform) {
		return nil, fmt.Errorf("PP video account is required")
	}

	isCancel := operation == PPVideoOperationCancel
	isStatus := strings.TrimSpace(taskID) != "" && !isCancel
	isSubmission := !isStatus && !isCancel
	publicRequest := PPVideoPublicRequest{}
	if len(publicRequests) > 0 {
		publicRequest = publicRequests[0]
	}
	upstreamBody := body
	var path string
	var err error
	if isStatus {
		path, err = PPVideoStatusUpstreamPath(account.Platform, operation, taskID)
	} else if isCancel {
		path, err = PPVideoUpstreamPath(account.Platform, operation, taskID)
	} else {
		path, err = PPVideoUpstreamPath(account.Platform, operation, "")
	}
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(account.GetOpenAIApiKey())
	if token == "" {
		return nil, fmt.Errorf("PP video api key not found in credentials")
	}
	baseURLRaw := account.GetOpenAIBaseURL()
	if strings.TrimSpace(baseURLRaw) == "" &&
		(account.Platform == PlatformByteDance || account.Platform == PlatformWan3 || account.Platform == PlatformMiniMaxH3 || account.Platform == PlatformPixverseV6 || account.Platform == PlatformGrokImagineVideo || account.Platform == PlatformKuaishou) {
		baseURLRaw = ModelVerseVideoDefaultBaseURL
	}
	baseURL, err := normalizePPVideoBaseURL(baseURLRaw)
	if err != nil {
		return nil, err
	}

	method := http.MethodPost
	var reader io.Reader
	if isStatus || isCancel {
		method = http.MethodGet
	} else {
		if len(upstreamBody) == 0 {
			return nil, fmt.Errorf("PP video request body is empty")
		}
		if strings.TrimSpace(publicRequest.Model) == "" {
			var prepared []byte
			prepared, publicRequest, err = PreparePPVideoRequestBodyForAccount(account, operation, upstreamBody)
			if err != nil {
				return nil, err
			}
			upstreamBody = prepared
		}
		reader = bytes.NewReader(upstreamBody)
	}
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	req, err := http.NewRequestWithContext(upstreamCtx, method, strings.TrimRight(baseURL, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	headers, err := PPVideoRequestHeaders(account.Platform, token)
	if err != nil {
		return nil, err
	}
	if isStatus || isCancel {
		headers, err = PPVideoStatusRequestHeaders(account.Platform, token)
		if err != nil {
			return nil, err
		}
	}
	req.Header = headers
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	start := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if c != nil {
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(start).Milliseconds())
	}
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if isStatus && resp.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("PP video upstream returned HTTP %d", resp.StatusCode)
		}
		writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)
		if c != nil {
			MarkResponseCommitted(c)
		}
		return nil, fmt.Errorf("PP video upstream returned HTTP %d", resp.StatusCode)
	}
	parsed, err := ParsePPVideoResponse(account.Platform, respBody)
	if err != nil {
		return nil, err
	}
	responseBody := respBody
	if !isCancel {
		responseBody = NormalizePPVideoPublicResponse(account.Platform, respBody, publicRequest, parsed)
	}
	responseHeaders := resp.Header.Clone()
	if responseHeaders == nil {
		responseHeaders = make(http.Header)
	}
	responseHeaders.Del("Content-Length")
	if len(responseBody) > 0 {
		responseHeaders.Set("Content-Type", "application/json")
	}
	model := strings.TrimSpace(publicRequest.Model)
	if model == "" {
		model = PPVideoModelFromBody(upstreamBody)
	}
	if model == "" {
		model = account.Platform
	}
	upstreamModel := strings.TrimSpace(publicRequest.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = PPVideoModelFromBody(upstreamBody)
	}
	if upstreamModel == "" {
		upstreamModel = model
	}
	result := &OpenAIForwardResult{
		RequestID:                      parsed.TaskID,
		ResponseID:                     parsed.TaskID,
		Model:                          model,
		BillingModel:                   model,
		UpstreamModel:                  upstreamModel,
		UpstreamEndpoint:               path,
		ResponseHeaders:                responseHeaders,
		Duration:                       time.Since(start),
		TaskStatus:                     parsed.Status,
		ErrorMessage:                   parsed.ErrorMessage,
		ResponseStatusCode:             resp.StatusCode,
		ResponseContentType:            strings.TrimSpace(responseHeaders.Get("Content-Type")),
		ResponseBody:                   append([]byte(nil), responseBody...),
		VideoCount:                     parsed.VideoCount,
		VideoResolution:                parsed.VideoResolution,
		VideoDurationMilliseconds:      parsed.GeneratedDurationMilliseconds,
		VideoInputDurationMilliseconds: parsed.InputVideoDurationMilliseconds,
		VideoOutputWidth:               parsed.OutputWidth,
		VideoOutputHeight:              parsed.OutputHeight,
		VideoFrameRate:                 parsed.FrameRate,
	}
	if isSubmission {
		meta := PPVideoBillingMetadataFromRequest(account.Platform, upstreamBody)
		result.VideoDurationSeconds = meta.RequestedDurationSeconds
		if result.VideoResolution == "" {
			result.VideoResolution = meta.VideoResolution
		}
		if result.VideoCount <= 0 {
			result.VideoCount = meta.VideoCount
		}
		if result.VideoInputDurationMilliseconds <= 0 {
			result.VideoInputDurationMilliseconds = meta.InputVideoDurationMilliseconds
		}
		if result.VideoOutputWidth <= 0 {
			result.VideoOutputWidth = meta.OutputWidth
		}
		if result.VideoOutputHeight <= 0 {
			result.VideoOutputHeight = meta.OutputHeight
		}
		if result.VideoFrameRate <= 0 {
			result.VideoFrameRate = meta.FrameRate
		}
	}
	return result, nil
}

func (s *OpenAIGatewayService) WritePPVideoForwardResult(c *gin.Context, result *OpenAIForwardResult) {
	if c == nil || result == nil {
		return
	}
	status := result.ResponseStatusCode
	if status == 0 {
		status = http.StatusOK
	}
	contentType := result.ResponseContentType
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
	writeGrokMediaResponse(c, &http.Response{StatusCode: status, Header: headers}, result.ResponseBody, s.responseHeaderFilter)
}

func (s *OpenAIGatewayService) SelectPPVideoAccount(ctx context.Context, groupID *int64, platform string, requestedModel string) (*AccountSelectionResult, error) {
	return s.selectPPVideoAccount(ctx, groupID, platform, func(account *Account) bool {
		return isPPVideoAccountEligibleForModel(account, requestedModel)
	})
}

func (s *OpenAIGatewayService) SelectPPVideoAccountForRequest(ctx context.Context, groupID *int64, platform string, public PPVideoPublicRequest) (*AccountSelectionResult, error) {
	return s.selectPPVideoAccount(ctx, groupID, platform, func(account *Account) bool {
		return isPPVideoAccountEligibleForRequest(account, public)
	})
}

func (s *OpenAIGatewayService) selectPPVideoAccount(ctx context.Context, groupID *int64, platform string, eligible func(*Account) bool) (*AccountSelectionResult, error) {
	if s == nil || s.accountRepo == nil || !IsPPVideoPlatform(platform) {
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
		if eligible == nil || !eligible(&account) {
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

func isPPVideoAccountEligibleForModel(account *Account, requestedModel string) bool {
	if account == nil {
		return false
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return true
	}
	if ppVideoModelKnownForeignToPlatform(account.Platform, requestedModel) {
		return false
	}
	if account.Platform == PlatformByteDance && ppVideoByteDanceModelAllowed(requestedModel) {
		return true
	}
	if account.Platform == PlatformWan3 && ppVideoWan30ModelAllowed(requestedModel) {
		return true
	}
	if account.Platform == PlatformMiniMaxH3 && ppVideoMiniMaxH3ModelAllowedForAccount(requestedModel, account) {
		return true
	}
	if account.Platform == PlatformPixverseV6 && ppVideoPixverseV6ModelAllowed(requestedModel) {
		return true
	}
	if account.Platform == PlatformGrokImagineVideo && ppVideoGrokImagineVideoModelAllowed(requestedModel) {
		return true
	}
	if account.Platform == PlatformKuaishou && ppVideoKuaishouModelAllowed(requestedModel) {
		return true
	}
	if account.IsModelSupported(requestedModel) {
		return true
	}
	for _, mappedModel := range account.GetModelMapping() {
		if strings.EqualFold(strings.TrimSpace(mappedModel), requestedModel) {
			return true
		}
	}
	return false
}

func isPPVideoAccountEligibleForRequest(account *Account, public PPVideoPublicRequest) bool {
	if account == nil {
		return false
	}
	if isPPVideoAccountEligibleForModel(account, public.Model) {
		return true
	}
	return isPPVideoAccountEligibleForModel(
		account,
		ppVideoUpstreamModelForAccount(account.Platform, public, account),
	)
}

func (s *OpenAIGatewayService) SelectPPVideoTaskAccountForStatus(ctx context.Context, accountID int64, platform string) (*AccountSelectionResult, error) {
	if s == nil || s.accountRepo == nil || accountID <= 0 || !IsPPVideoPlatform(platform) {
		return nil, ErrNoAvailableAccounts
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.Platform != platform {
		return nil, ErrNoAvailableAccounts
	}
	slot, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil || slot == nil || !slot.Acquired {
		return nil, ErrNoAvailableAccounts
	}
	return &AccountSelectionResult{Account: account, Acquired: true, ReleaseFunc: slot.ReleaseFunc}, nil
}

func normalizePPVideoBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("PP video base URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid PP video base URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("PP video base URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(parsed.Path, "/v1") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/v1")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func PPVideoModelFromBody(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range []string{"model", "model_name", "data.model", "parameters.model"} {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			return value
		}
	}
	return ""
}
