package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// PPVideoUpstreamError preserves an HTTP error returned before a video task is
// created, such as ByteDance's reference-image asset registration endpoints.
// Callers can distinguish a provider validation error from a transport failure.
type PPVideoUpstreamError struct {
	StatusCode   int
	ResponseBody []byte
	err          error
}

func (e *PPVideoUpstreamError) Error() string {
	if e == nil || e.err == nil {
		return "PP video upstream request failed"
	}
	return e.err.Error()
}

func (e *PPVideoUpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

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
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()

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
		if account.Platform == PlatformByteDance {
			upstreamBody, err = s.prepareByteDancePrivateImageAssets(upstreamCtx, baseURL, token, proxyURL, account, upstreamBody)
			if err != nil {
				return nil, err
			}
		}
		reader = bytes.NewReader(upstreamBody)
	}
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
	if isStatus && account.Platform == PlatformPixverseV6 && parsed.Status == PPVideoTaskStatusSucceeded {
		if mediaURL := ppVideoExtractVideoURL(respBody); mediaURL != "" {
			ready, probeErr := s.ppVideoPixverseV6MediaURLReady(upstreamCtx, mediaURL, proxyURL, account)
			if probeErr != nil || !ready {
				parsed.Status = PPVideoTaskStatusProcessing
				respBody = ppVideoWithoutPublicVideoURL(respBody)
			}
		}
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

func (s *OpenAIGatewayService) ppVideoPixverseV6MediaURLReady(ctx context.Context, mediaURL string, proxyURL string, account *Account) (bool, error) {
	mediaURL = strings.TrimSpace(mediaURL)
	if mediaURL == "" {
		return false, nil
	}
	if _, err := url.ParseRequestURI(mediaURL); err != nil {
		return false, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return false, nil
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("Accept", "video/*,*/*;q=0.8")
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices, nil
}

func (s *OpenAIGatewayService) prepareByteDancePrivateImageAssets(ctx context.Context, baseURL, token, proxyURL string, account *Account, body []byte) ([]byte, error) {
	imageURLs := ppVideoByteDancePublicImageURLPaths(body)
	if len(imageURLs) == 0 {
		return body, nil
	}
	upstreamModel := byteDanceUpstreamModel(PPVideoModelFromBody(body))
	if upstreamModel == "" {
		upstreamModel = ByteDanceVideoDefaultModel
	}
	groupID, err := s.ensureByteDanceAIGCAssetGroup(ctx, baseURL, token, proxyURL, account, upstreamModel)
	if err != nil {
		return nil, err
	}
	rewritten := body
	for _, item := range imageURLs {
		assetID, err := s.ensureByteDanceImageAsset(ctx, baseURL, token, proxyURL, account, groupID, item.url)
		if err != nil {
			return nil, err
		}
		rewritten, err = sjson.SetBytes(rewritten, item.path, "asset://"+assetID)
		if err != nil {
			return nil, err
		}
	}
	return rewritten, nil
}

type ppVideoByteDanceImageURLPath struct {
	path string
	url  string
}

func ppVideoByteDancePublicImageURLPaths(body []byte) []ppVideoByteDanceImageURLPath {
	content := gjson.GetBytes(body, "input.content")
	if !content.Exists() || !content.IsArray() {
		return nil
	}
	var result []ppVideoByteDanceImageURLPath
	for index, item := range content.Array() {
		if strings.TrimSpace(item.Get("type").String()) != "image_url" {
			continue
		}
		if strings.TrimSpace(item.Get("role").String()) != "reference_image" {
			continue
		}
		rawURL := strings.TrimSpace(item.Get("image_url.url").String())
		lowerURL := strings.ToLower(rawURL)
		if rawURL == "" || strings.HasPrefix(lowerURL, "asset://") || strings.HasPrefix(lowerURL, "data:") {
			continue
		}
		if strings.HasPrefix(lowerURL, "http://") || strings.HasPrefix(lowerURL, "https://") {
			result = append(result, ppVideoByteDanceImageURLPath{
				path: fmt.Sprintf("input.content.%d.image_url.url", index),
				url:  rawURL,
			})
		}
	}
	return result
}

const ppVideoByteDanceSeedance20AssetGroupName = "seedance-2.0-i2v"
const ppVideoByteDanceAssetStatusPolls = 30
const ppVideoByteDanceAssetStatusPollInterval = 2 * time.Second

func (s *OpenAIGatewayService) ensureByteDanceAIGCAssetGroup(ctx context.Context, baseURL, token, proxyURL string, account *Account, model string) (string, error) {
	groupName := ppVideoByteDanceAssetGroupName(model)
	listBody := []byte(fmt.Sprintf(`{"filter":{"group_type":"AIGC","name":%q},"page_number":1,"page_size":100}`, groupName))
	respBody, err := s.doByteDanceAssetRequest(ctx, baseURL, token, proxyURL, account, "/v1/volce-asset/groups/list", listBody)
	if err != nil {
		return "", err
	}
	for _, item := range gjson.GetBytes(respBody, "items").Array() {
		if strings.TrimSpace(item.Get("name").String()) == groupName {
			if id := strings.TrimSpace(item.Get("id").String()); id != "" {
				return id, nil
			}
		}
	}
	createBody, err := json.Marshal(map[string]any{
		"name":       groupName,
		"group_type": "AIGC",
		"model":      model,
	})
	if err != nil {
		return "", err
	}
	respBody, err = s.doByteDanceAssetRequest(ctx, baseURL, token, proxyURL, account, "/v1/volce-asset/groups/create", createBody)
	if err != nil {
		return "", err
	}
	if id := strings.TrimSpace(gjson.GetBytes(respBody, "id").String()); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("ByteDance asset group response missing id")
}

func ppVideoByteDanceAssetGroupName(model string) string {
	if strings.EqualFold(strings.TrimSpace(model), ByteDanceVideoDefaultModel) {
		return ppVideoByteDanceSeedance20AssetGroupName
	}
	return "sub2api-private-portrait-assets"
}

func (s *OpenAIGatewayService) ensureByteDanceImageAsset(ctx context.Context, baseURL, token, proxyURL string, account *Account, groupID, imageURL string) (string, error) {
	assetName := ppVideoByteDanceAssetName(imageURL)
	if assetID, err := s.findActiveByteDanceImageAsset(ctx, baseURL, token, proxyURL, account, groupID, assetName, imageURL); err != nil {
		return "", err
	} else if assetID != "" {
		return assetID, nil
	}
	return s.createByteDanceImageAsset(ctx, baseURL, token, proxyURL, account, groupID, assetName, imageURL)
}

func (s *OpenAIGatewayService) findActiveByteDanceImageAsset(ctx context.Context, baseURL, token, proxyURL string, account *Account, groupID, assetName, imageURL string) (string, error) {
	listPayload, _ := json.Marshal(map[string]any{
		"filter": map[string]any{
			"group_ids":  []string{groupID},
			"group_type": "AIGC",
			"statuses":   []string{"Active"},
			"name":       assetName,
		},
		"page_number": 1,
		"page_size":   100,
		"sort_by":     "UpdateTime",
		"sort_order":  "Desc",
	})
	respBody, err := s.doByteDanceAssetRequest(ctx, baseURL, token, proxyURL, account, "/v1/volce-asset/assets/list", listPayload)
	if err != nil {
		return "", err
	}
	for _, item := range gjson.GetBytes(respBody, "items").Array() {
		if strings.TrimSpace(item.Get("name").String()) != assetName {
			continue
		}
		if strings.TrimSpace(item.Get("url").String()) != imageURL {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.Get("asset_type").String()), "Image") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.Get("status").String()), "Active") {
			continue
		}
		if id := strings.TrimSpace(item.Get("id").String()); id != "" {
			return id, nil
		}
	}
	return "", nil
}

func (s *OpenAIGatewayService) createByteDanceImageAsset(ctx context.Context, baseURL, token, proxyURL string, account *Account, groupID, assetName, imageURL string) (string, error) {
	createPayload, _ := json.Marshal(map[string]any{
		"group_id":   groupID,
		"url":        imageURL,
		"name":       assetName,
		"asset_type": "Image",
	})
	respBody, err := s.doByteDanceAssetRequest(ctx, baseURL, token, proxyURL, account, "/v1/volce-asset/assets/create", createPayload)
	if err != nil {
		return "", err
	}
	assetID := strings.TrimSpace(gjson.GetBytes(respBody, "id").String())
	if assetID == "" {
		return "", fmt.Errorf("ByteDance asset create response missing id")
	}
	for attempt := 0; attempt < ppVideoByteDanceAssetStatusPolls; attempt++ {
		status, statusErr := s.getByteDanceImageAssetStatus(ctx, baseURL, token, proxyURL, account, assetID)
		if statusErr != nil {
			return "", statusErr
		}
		switch strings.ToLower(status) {
		case "active":
			return assetID, nil
		case "failed":
			return "", fmt.Errorf("ByteDance image asset %s failed processing", assetID)
		}
		if attempt < ppVideoByteDanceAssetStatusPolls-1 {
			timer := time.NewTimer(ppVideoByteDanceAssetStatusPollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
	}
	return "", fmt.Errorf("ByteDance image asset %s is not active yet", assetID)
}

func ppVideoByteDanceAssetName(imageURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(imageURL)))
	return "sub2api-image-" + hex.EncodeToString(sum[:])[:16]
}

func (s *OpenAIGatewayService) getByteDanceImageAssetStatus(ctx context.Context, baseURL, token, proxyURL string, account *Account, assetID string) (string, error) {
	getPayload, _ := json.Marshal(map[string]any{"id": assetID})
	respBody, err := s.doByteDanceAssetRequest(ctx, baseURL, token, proxyURL, account, "/v1/volce-asset/assets/get", getPayload)
	if err != nil {
		return "", err
	}
	status := strings.TrimSpace(gjson.GetBytes(respBody, "status").String())
	if status == "" {
		return "", fmt.Errorf("ByteDance asset get response missing status")
	}
	return status, nil
}

func (s *OpenAIGatewayService) doByteDanceAssetRequest(ctx context.Context, baseURL, token, proxyURL string, account *Account, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(req.Header)
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("ByteDance asset request returned no response")
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, nil, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &PPVideoUpstreamError{
			StatusCode:   resp.StatusCode,
			ResponseBody: append([]byte(nil), respBody...),
			err:          fmt.Errorf("ByteDance asset request %s returned HTTP %d", path, resp.StatusCode),
		}
	}
	return respBody, nil
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
