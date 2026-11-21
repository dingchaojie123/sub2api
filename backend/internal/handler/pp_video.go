package handler

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) PPVideoGeneration(c *gin.Context) {
	h.handlePPVideo(c, "", "")
}

func (h *OpenAIGatewayHandler) PPKlingTextToVideo(c *gin.Context) {
	h.handlePPVideo(c, service.PPVideoOperationKlingTextToVideo, "")
}

func (h *OpenAIGatewayHandler) PPKlingImageToVideo(c *gin.Context) {
	h.handlePPVideo(c, service.PPVideoOperationKlingImageToVideo, "")
}

func (h *OpenAIGatewayHandler) PPVideoStatus(c *gin.Context) {
	h.handlePPVideo(c, "", strings.TrimSpace(c.Param("request_id")))
}

func (h *OpenAIGatewayHandler) PPVideoContent(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil ||
		(apiKey.Group.Platform != service.PlatformCTMOSeedance && apiKey.Group.Platform != service.PlatformXingAPISeedance && apiKey.Group.Platform != service.PlatformJijiuSeedance && apiKey.Group.Platform != service.PlatformMiaowuSeedance && apiKey.Group.Platform != service.PlatformPaisioSeedance && apiKey.Group.Platform != service.PlatformApophisSeedance && apiKey.Group.Platform != service.PlatformXSNSeedance && apiKey.Group.Platform != service.PlatformFengyueSeedance && apiKey.Group.Platform != service.PlatformHongpeiSeedance && apiKey.Group.Platform != service.PlatformVshiSeedance && apiKey.Group.Platform != service.PlatformHuahuoSeedance && apiKey.Group.Platform != service.PlatformZexiSeedance && apiKey.Group.Platform != service.PlatformXMSeedance && apiKey.Group.Platform != service.PlatformNaiwaSeedance && apiKey.Group.Platform != service.PlatformV6KSeedance && apiKey.Group.Platform != service.PlatformMaiTokenSeedance && apiKey.Group.Platform != service.PlatformOPONE && apiKey.Group.Platform != service.PlatformSeedanceHub) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video content is not supported for this platform")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Video request id is required")
		return
	}
	task, err := h.gatewayService.GetPPVideoTaskForOwner(c.Request.Context(), subject.UserID, apiKey.ID, requestID)
	// A task records the platform of the account that ran it, so a Seedance aggregate group
	// serves the tasks of every member platform.
	if err != nil || task == nil || !service.PPVideoGroupServesPlatform(apiKey.Group.Platform, task.Platform) ||
		(task.Platform != service.PlatformCTMOSeedance && task.Platform != service.PlatformXingAPISeedance && task.Platform != service.PlatformJijiuSeedance && task.Platform != service.PlatformMiaowuSeedance && task.Platform != service.PlatformPaisioSeedance && task.Platform != service.PlatformApophisSeedance && task.Platform != service.PlatformXSNSeedance && task.Platform != service.PlatformFengyueSeedance && task.Platform != service.PlatformHongpeiSeedance && task.Platform != service.PlatformVshiSeedance && task.Platform != service.PlatformHuahuoSeedance && task.Platform != service.PlatformZexiSeedance && task.Platform != service.PlatformXMSeedance && task.Platform != service.PlatformNaiwaSeedance && task.Platform != service.PlatformV6KSeedance && task.Platform != service.PlatformMaiTokenSeedance && task.Platform != service.PlatformOPONE) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
		return
	}
	if !service.PPVideoTaskContentAvailable(task.Platform, task.Status, task.BillingStatus) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video content is not available")
		return
	}
	selection, err := h.gatewayService.SelectPPVideoTaskAccountForStatus(c.Request.Context(), task.AccountID, task.Platform)
	if err != nil || selection == nil || selection.Account == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", "No eligible video account")
		return
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}
	// 这个接口原先没有任何日志，下载慢或失败时只能看到访问日志里的总耗时。
	// 记录首字节耗时（上游响应头到达）和实际传输字节数，才能区分是上游慢、本站出口慢还是客户端慢。
	reqLog := requestLogger(c, "handler.openai_gateway.pp_video_content",
		zap.String("task", requestID),
		zap.String("platform", task.Platform),
		zap.Int64("account_id", task.AccountID),
	)
	contentStarted := time.Now()
	resp, err := h.gatewayService.ForwardPPVideoTaskContentWithMethod(c.Request.Context(), c.Request.Method, selection.Account, task, c.GetHeader("Range"), c.GetHeader("If-Range"))
	if err != nil {
		reqLog.Warn("pp_video.content_forward_failed",
			zap.String("client_range", c.GetHeader("Range")),
			zap.Int64("elapsed_ms", time.Since(contentStarted).Milliseconds()),
			zap.Error(err),
		)
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video download failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	firstByteMs := time.Since(contentStarted).Milliseconds()
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Disposition", "Cache-Control", "Accept-Ranges", "Content-Range", "ETag", "Last-Modified"} {
		if value := strings.TrimSpace(resp.Header.Get(header)); value != "" {
			c.Header(header, value)
		}
	}
	if strings.TrimSpace(c.Writer.Header().Get("Content-Type")) == "" {
		c.Header("Content-Type", "application/octet-stream")
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(resp.StatusCode)
	if c.Request.Method == http.MethodHead {
		return
	}
	copied, copyErr := io.Copy(c.Writer, resp.Body)
	copyFields := []zap.Field{
		zap.Int("status", resp.StatusCode),
		zap.String("client_range", c.GetHeader("Range")),
		zap.Int64("content_length", resp.ContentLength),
		zap.Int64("copied_bytes", copied),
		zap.Int64("first_byte_ms", firstByteMs),
		zap.Int64("total_ms", time.Since(contentStarted).Milliseconds()),
	}
	if copyErr != nil || (resp.ContentLength > 0 && copied != resp.ContentLength) {
		// 状态行和响应头已经发出，这里只能记录：客户端会收到被截断的文件。
		reqLog.Warn("pp_video.content_incomplete", append(copyFields, zap.Error(copyErr))...)
		return
	}
	reqLog.Info("pp_video.content_done", copyFields...)
}

func (h *OpenAIGatewayHandler) PPVideoCancel(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil ||
		(apiKey.Group.Platform != service.PlatformByteDance && apiKey.Group.Platform != service.PlatformMiniMaxH3CompShare) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video cancellation is not supported for this platform")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	if !h.ensureResponsesDependencies(c, requestLogger(c, "handler.openai_gateway.pp_video_cancel")) {
		return
	}

	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Video request id is required")
		return
	}
	task, err := h.gatewayService.GetPPVideoTaskForOwner(c.Request.Context(), subject.UserID, apiKey.ID, requestID)
	if err != nil || task == nil || task.Platform != apiKey.Group.Platform {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
		return
	}
	if service.IsTerminalPPVideoTaskStatus(task.Status) {
		h.errorResponse(c, http.StatusConflict, "invalid_request_error", "Video request is already finished")
		return
	}
	if strings.TrimSpace(task.TaskID) == "" {
		h.errorResponse(c, http.StatusConflict, "invalid_request_error", "Video request has not been submitted upstream")
		return
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	selection, err := h.gatewayService.SelectPPVideoTaskAccountForStatus(c.Request.Context(), task.AccountID, task.Platform)
	if err != nil || selection == nil || selection.Account == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", "No eligible PP video account")
		return
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}

	durationMilliseconds := task.GeneratedVideoDurationMilliseconds
	if durationMilliseconds <= 0 {
		durationMilliseconds = task.RequestedVideoDurationMilliseconds
	}
	result, err := h.gatewayService.ForwardPPVideoBuffered(
		c.Request.Context(),
		c,
		selection.Account,
		service.PPVideoOperationCancel,
		task.TaskID,
		nil,
		service.PPVideoPublicRequest{
			Model:                task.Model,
			DurationMilliseconds: durationMilliseconds,
			VideoCount:           task.VideoCount,
			Resolution:           task.VideoResolution,
		},
	)
	if err != nil {
		if !service.IsResponseCommitted(c) {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video cancellation failed")
		}
		return
	}
	if result == nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream video cancellation returned no response")
		return
	}
	// CompShare can acknowledge cancellation while the worker is still running.
	// Keep the hold and let the poller observe the eventual terminal state.
	if task.Platform == service.PlatformMiniMaxH3CompShare && result.TaskStatus != service.PPVideoTaskStatusFailed {
		h.gatewayService.WritePPVideoForwardResult(c, result)
		return
	}

	task, err = h.gatewayService.MarkPPVideoTaskStatus(c.Request.Context(), service.MarkPPVideoTaskStatusParams{
		TaskID:              task.TaskID,
		Status:              service.PPVideoTaskStatusFailed,
		LastErrorCode:       "cancelled",
		LastErrorMessage:    "video task cancelled",
		ResponseStatus:      result.ResponseStatusCode,
		ResponseContentType: result.ResponseContentType,
		ResponseBody:        string(result.ResponseBody),
	})
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to persist video cancellation")
		return
	}
	if err := h.gatewayService.SettlePPVideoTask(c.Request.Context(), &service.PPVideoSettlementInput{
		Task: task, FinalStatus: service.PPVideoTaskStatusFailed, APIKey: apiKey, User: apiKey.User,
		Account: selection.Account, Subscription: subscription,
		InboundEndpoint: GetInboundEndpoint(c), UpstreamEndpoint: result.UpstreamEndpoint,
		RequestPayloadHash: task.RequestHash, APIKeyService: h.apiKeyService,
		QuotaPlatform: service.QuotaPlatform(c.Request.Context(), apiKey),
	}); err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to settle cancelled video billing")
		return
	}
	h.invalidatePPVideoBalanceCache(c.Request.Context(), task.UserID)
	h.gatewayService.WritePPVideoForwardResult(c, result)
}

func preparePPVideoMultipartRequest(platform string, req *http.Request) (*service.PPVideoSubmissionBody, service.PPVideoPublicRequest, error) {
	if platform != service.PlatformMaiTokenSeedance {
		return nil, service.PPVideoPublicRequest{}, errors.New("multipart video requests are only supported for Mai Token Seedance")
	}
	submission, err := service.PrepareMaiTokenMultipartSubmission(req)
	if err != nil {
		return nil, service.PPVideoPublicRequest{}, err
	}
	prepared, public, err := service.PreparePPVideoRequestBody(platform, service.PPVideoOperationGeneric, submission.JSON)
	if err != nil {
		_ = submission.Cleanup()
		return nil, service.PPVideoPublicRequest{}, err
	}
	submission.JSON = prepared
	if submission.Multipart != nil && len(submission.Multipart.Uploads) > 0 {
		public.HasImage = true
	}
	return submission, public, nil
}

// isPPVideoUnifiedRequest reports whether the request was sent to the unified
// (v2) video generation endpoint, whose body must be translated before the
// platform normalizers see it.
func isPPVideoUnifiedRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	return strings.HasSuffix(strings.TrimRight(c.Request.URL.Path, "/"), "/v2/videos/generations")
}

// ppVideoSubmissionSourceBody returns the body that each selected account's request
// preparation starts from: the JSON a multipart submission was reduced to, else the
// translated body of a v2 request, else the client's own body. Starting from the
// client's original body would hand a v2 request to the v1 preparation, which silently
// drops the fields it does not know, such as assets.
func ppVideoSubmissionSourceBody(isMultipart bool, submission *service.PPVideoSubmissionBody, pipelineSource []byte, original []byte) []byte {
	if isMultipart && submission != nil {
		return submission.JSON
	}
	if len(pipelineSource) > 0 {
		return pipelineSource
	}
	return original
}

func (h *OpenAIGatewayHandler) handlePPVideo(c *gin.Context, operation service.PPVideoOperation, requestID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil || !service.IsPPVideoGroupPlatform(apiKey.Group.Platform) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	reqLog := requestLogger(c, "handler.openai_gateway.pp_video",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.String("platform", apiKey.Group.Platform),
	)
	isStatus := strings.TrimSpace(requestID) != ""
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}
	if operation != "" && apiKey.Group.Platform != service.PlatformKling {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Kling video API is only supported for Kling platform groups")
		return
	}
	if !isStatus {
		quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
		if err := h.billingCacheService.CheckBillingEligibility(
			c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, quotaPlatform,
		); err != nil {
			reqLog.Info("pp_video.billing_eligibility_check_failed", zap.Error(err))
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.errorResponse(c, status, code, message)
			return
		}
	}

	var body []byte
	var originalBody []byte
	// The body each selected account's request preparation starts from. For a v2 request
	// it is the translated body; originalBody stays reserved for audit and hashing.
	var pipelineSourceBody []byte
	var submission *service.PPVideoSubmissionBody
	isMultipartSubmission := false
	publicRequest := service.PPVideoPublicRequest{}
	var payloadHash, rawPayloadHash, idempotencyKey string
	// requestPlatform is the platform whose request pipeline handles a submission. It is the
	// group's platform, except in a Seedance aggregate group, where each model line belongs to
	// one platform and the request is handled as one sent to a group of that platform.
	requestPlatform := apiKey.Group.Platform
	var hubLine *service.SeedanceHubLine
	// routedSubmission is set when the group routes the requested line to another line (a line
	// route): the request is then run by the target line's platform, while hubLine, the line the
	// client asked for, keeps naming, pricing and recording the task. revertRoute puts the
	// request back on the requested line when the target turns out to have no usable account.
	var routedSubmission *service.PPVideoRoutedRequest
	var revertRoute func()
	// routedSourceResolution is the resolution the requested line itself would have run (the
	// request's own, or that line's default). A routed request is priced on it, so that a target
	// with another default resolution cannot move the request into a different price tier.
	var routedSourceResolution string
	// An ordinary group of a Seedance platform (no hubLine) is billed and recorded as the request
	// the platform itself would have run: its platform, prepared body and public model name.
	var routedSourcePlatform, routedSourceModel string
	var routedSourceBody []byte
	// routeLine is the line the client asked for, when the request can be routed (nil otherwise).
	var routeLine *service.SeedanceHubLine
	if !isStatus {
		mediaType, _, contentTypeErr := mime.ParseMediaType(c.GetHeader("Content-Type"))
		isMultipartSubmission = contentTypeErr == nil && strings.EqualFold(mediaType, "multipart/form-data")
		if isMultipartSubmission && isPPVideoUnifiedRequest(c) {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "The unified video API (v2) only accepts application/json requests")
			return
		}
		if isMultipartSubmission && apiKey.Group.Platform == service.PlatformSeedanceHub {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "This group only accepts application/json requests")
			return
		}
		if isMultipartSubmission {
			if operation == "" {
				operation = service.PPVideoOperationGeneric
			}
			var prepareErr error
			submission, publicRequest, prepareErr = preparePPVideoMultipartRequest(apiKey.Group.Platform, c.Request)
			if prepareErr != nil {
				h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", prepareErr.Error())
				return
			}
			defer func() { _ = submission.Cleanup() }()
			body = submission.JSON
			originalBody = append([]byte(nil), body...)
			rawPayloadHash = submission.PayloadHash()
		} else {
			var err error
			body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
			if err != nil {
				h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
				return
			}
			if len(body) == 0 {
				h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
				return
			}
			originalBody = append([]byte(nil), body...)
			rawPayloadHash = service.HashUsageRequestPayload(originalBody)
			pipelineSourceBody = originalBody
			if operation == "" {
				if apiKey.Group.Platform == service.PlatformKling {
					operation = ppVideoOperationFromRequest(body)
				} else {
					operation = service.PPVideoOperationGeneric
				}
			}
			if apiKey.Group.Platform == service.PlatformSeedanceHub {
				// Find the model line first: its `model` is rewritten to the name the line's
				// platform accepts, and from here on the request is that platform's request.
				line, hubBody, hubErr := service.ResolveSeedanceHubRequest(body)
				if hubErr != nil {
					h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", hubErr.Error())
					return
				}
				hubLine = &line
				requestPlatform = line.Platform
				body = hubBody
				pipelineSourceBody = hubBody
			}
			if isPPVideoUnifiedRequest(c) {
				// originalBody keeps the client's v2 request for security audit and
				// idempotency hashing; only the body handed to the pipeline is translated.
				translatedBody, translateErr := service.TranslatePPVideoUnifiedRequest(requestPlatform, body)
				if translateErr != nil {
					h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", translateErr.Error())
					return
				}
				body = translatedBody
				pipelineSourceBody = translatedBody
			}
			preparedBody, preparedPublicRequest, prepareErr := service.PreparePPVideoRequestBody(requestPlatform, operation, body)
			if prepareErr != nil {
				h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", prepareErr.Error())
				return
			}
			body = preparedBody
			publicRequest = preparedPublicRequest
			submission = &service.PPVideoSubmissionBody{JSON: body}
		}
		model := strings.TrimSpace(publicRequest.Model)
		if model == "" {
			model = strings.TrimSpace(publicRequest.UpstreamModel)
		}
		if model == "" {
			model = apiKey.Group.Platform
		}
		if hubLine != nil {
			model = hubLine.Name
		}
		setOpsRequestContext(c, model, false)
		setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
		if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, originalBody); decision != nil && !decision.AllowNextStage {
			h.openAISecurityAuditError(c, decision)
			return
		}
		payloadHash = submission.PayloadHash()
		idempotencyKey = resolvePPVideoIdempotencyKey(c.GetHeader("Idempotency-Key"))
		if len(idempotencyKey) > service.PPVideoIdempotencyKeyMaxLength {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Idempotency-Key must be at most 255 characters")
			return
		}
		if idempotencyKey != "" {
			existing, lookupErr := h.gatewayService.GetPPVideoTaskByIdempotencyKey(c.Request.Context(), subject.UserID, apiKey.ID, idempotencyKey)
			if lookupErr == nil && existing != nil {
				if existing.RequestHash != payloadHash && existing.RequestHash != rawPayloadHash {
					h.errorResponse(c, http.StatusConflict, "idempotency_error", "Idempotency-Key was reused with a different video request")
					return
				}
				if existing.ResponseBody != "" {
					c.Header("X-Idempotency-Replayed", "true")
					writeStoredPPVideoResponse(c, existing)
					return
				}
				if existing.Status == service.PPVideoTaskStatusFailed {
					released, releaseErr := h.gatewayService.ReleaseFailedPPVideoTaskIdempotencyKey(
						c.Request.Context(), existing.LocalTaskID, idempotencyKey,
					)
					if releaseErr != nil {
						h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Video task storage is unavailable")
						return
					}
					if !released {
						h.errorResponse(c, http.StatusConflict, "idempotency_error", "Previous video generation submission failed; retry with a new Idempotency-Key")
						return
					}
				} else {
					h.errorResponse(c, http.StatusConflict, "idempotency_error", "Video generation request is still being submitted")
					return
				}
			}
			if lookupErr != nil && !errors.Is(lookupErr, service.ErrPPVideoTaskNotFound) {
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Video task storage is unavailable")
				return
			}
		}
		// The line the client asked for: in an aggregate group the one its model names; in the
		// ordinary group of a Seedance platform the one that platform serves under the requested model.
		routeLine = hubLine
		if routeLine == nil && !isMultipartSubmission && service.IsSeedanceHubMemberPlatform(apiKey.Group.Platform) {
			routeLine = ppVideoRouteSourceLine(apiKey.Group.Platform, publicRequest, body)
		}
		if routeLine != nil && !isMultipartSubmission {
			// The request was validated and hashed above as a request for the line the client asked
			// for. If the group routes that line to another one, the request is handed to the
			// target line's platform from here on.
			if routed := h.routePPVideoSubmission(c, reqLog, apiKey.GroupID, *routeLine, operation, originalBody, isPPVideoUnifiedRequest(c)); routed != nil {
				sourcePlatform, sourcePipelineBody, sourceBody, sourcePublic, sourceSubmission :=
					requestPlatform, pipelineSourceBody, body, publicRequest, submission
				revertRoute = func() {
					requestPlatform, pipelineSourceBody, body, publicRequest, submission =
						sourcePlatform, sourcePipelineBody, sourceBody, sourcePublic, sourceSubmission
				}
				routedSubmission = routed
				routedSourceResolution = service.PPVideoBillingMetadataFromRequest(sourcePlatform, sourceBody).VideoResolution
				if hubLine == nil {
					routedSourcePlatform, routedSourceBody, routedSourceModel = sourcePlatform, sourceBody, sourcePublic.Model
				}
				requestPlatform = routed.Platform
				pipelineSourceBody = routed.PipelineBody
				body = routed.Body
				publicRequest = routed.Public
				submission = &service.PPVideoSubmissionBody{JSON: body}
			}
		}
	}

	var task *service.PPVideoTask
	if isStatus {
		var err error
		task, err = h.gatewayService.GetPPVideoTaskForOwner(c.Request.Context(), subject.UserID, apiKey.ID, requestID)
		if err != nil || task == nil {
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}
		if shouldReplayStoredPPVideoStatus(task) {
			if task.Platform == service.PlatformOPONE && task.Status == service.PPVideoTaskStatusSucceeded &&
				service.PPVideoProviderURLFromResponse([]byte(task.ResponseBody)) == "" {
				selection, lookupErr := h.gatewayService.SelectPPVideoTaskAccountForStatus(c.Request.Context(), task.AccountID, task.Platform)
				if lookupErr == nil && selection != nil && selection.Account != nil {
					if selection.ReleaseFunc != nil {
						defer selection.ReleaseFunc()
					}
					task = h.gatewayService.RefreshPPVideoResultURL(c.Request.Context(), selection.Account, task)
				}
			}
			writeStoredPPVideoResponse(c, task)
			return
		}
		operation = task.Operation
		durationMilliseconds := task.GeneratedVideoDurationMilliseconds
		if durationMilliseconds <= 0 {
			durationMilliseconds = task.RequestedVideoDurationMilliseconds
		}
		publicRequest = service.PPVideoPublicRequest{
			Model:                task.Model,
			DurationMilliseconds: durationMilliseconds,
			VideoCount:           task.VideoCount,
			Resolution:           task.VideoResolution,
		}
	}
	if operation == "" {
		if apiKey.Group.Platform == service.PlatformKling {
			operation = ppVideoOperationFromRequest(body)
		} else {
			operation = service.PPVideoOperationGeneric
		}
	}

	selection, err := func() (*service.AccountSelectionResult, error) {
		if task != nil {
			return h.gatewayService.SelectPPVideoTaskAccountForStatus(c.Request.Context(), task.AccountID, task.Platform)
		}
		return h.gatewayService.SelectPPVideoAccountForRequest(c.Request.Context(), apiKey.GroupID, requestPlatform, publicRequest)
	}()
	if (err != nil || selection == nil || selection.Account == nil) && routedSubmission != nil && revertRoute != nil {
		// The group has no usable account for the route's target right now (all busy, disabled or
		// removed after the route was saved): serve the line the client asked for, as if there
		// were no route, instead of failing the request.
		reqLog.Warn("pp_video.line_route_target_unavailable",
			zap.String("line", routeLine.Name),
			zap.String("target", routedSubmission.Line.Name),
			zap.Error(err),
		)
		revertRoute()
		routedSubmission = nil
		selection, err = h.gatewayService.SelectPPVideoAccountForRequest(c.Request.Context(), apiKey.GroupID, requestPlatform, publicRequest)
	}
	if err == nil && selection != nil && selection.Account != nil && routedSubmission != nil && revertRoute != nil {
		// The request was checked for the target platform without an account. The checks that depend
		// on the account (its model mapping) can still refuse it; that must not reach the client
		// either, so the request goes back to the line the client asked for.
		if reason, accountErr := ppVideoRoutedAccountCheck(selection.Account, operation,
			ppVideoSubmissionSourceBody(isMultipartSubmission, submission, pipelineSourceBody, originalBody)); accountErr != nil {
			reqLog.Warn("pp_video.line_route_account_rejected",
				zap.String("line", routeLine.Name),
				zap.String("target", routedSubmission.Line.Name),
				zap.Int64("account_id", selection.Account.ID),
				zap.String("reason", reason),
				zap.Error(accountErr),
			)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			revertRoute()
			routedSubmission = nil
			selection, err = h.gatewayService.SelectPPVideoAccountForRequest(c.Request.Context(), apiKey.GroupID, requestPlatform, publicRequest)
		}
	}
	if err != nil || selection == nil || selection.Account == nil {
		if isStatus && canFallbackToStoredPPVideoStatus(task) {
			reqLog.Info("pp_video.status_stored_fallback",
				zap.String("reason", "account_unavailable"),
				zap.String("task_id", task.TaskID),
				zap.Error(err),
			)
			writeStoredPPVideoResponse(c, task)
			return
		}
		h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", "No eligible PP video account")
		return
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}
	account := selection.Account
	// 记录选中的账号，否则失败响应写入 Ops 错误日志时 account_id 为空，按账号排查会查不到。
	setOpsSelectedAccount(c, account.ID, account.Platform)
	if routedSubmission != nil {
		reqLog.Info("pp_video.line_routed",
			zap.String("line", routeLine.Name),
			zap.String("target", routedSubmission.Line.Name),
			zap.Int64("account_id", account.ID),
			zap.String("account_platform", account.Platform),
		)
	}

	if task == nil {
		var prepareErr error
		requestToPrepare := ppVideoSubmissionSourceBody(isMultipartSubmission, submission, pipelineSourceBody, originalBody)
		body, publicRequest, prepareErr = service.PreparePPVideoRequestBodyForAccount(account, operation, requestToPrepare)
		if prepareErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", prepareErr.Error())
			return
		}
		if hubLine != nil {
			// The task, the response and the usage record carry the name the client knows the
			// line by; the request that was prepared for the platform already carries its own.
			publicRequest.Model = hubLine.Name
		} else if routedSubmission != nil && routedSourceModel != "" {
			// A routed request of an ordinary group is recorded under the model the client asked for.
			publicRequest.Model = routedSourceModel
		}
		if isMultipartSubmission && submission.Multipart != nil && len(submission.Multipart.Uploads) > 0 {
			publicRequest.HasImage = true
		}
		submission.JSON = body
		if mediaErr := service.ValidatePPVideoInputMedia(c.Request.Context(), account.Platform, body); mediaErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", mediaErr.Error())
			return
		}
		meta := service.PPVideoBillingMetadataFromRequest(account.Platform, body)
		if hubLine == nil && routedSubmission != nil && routedSourceBody != nil {
			// An ordinary group's channel prices its own platform's models. A routed request is run
			// by another platform, but billed as the request the group's platform would have run:
			// the metadata (model, resolution, duration) is that of the request prepared for it.
			meta = service.PPVideoBillingMetadataFromRequest(routedSourcePlatform, routedSourceBody)
		}
		if meta.ValidationError != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", meta.ValidationError.Error())
			return
		}
		if hubLine != nil {
			// The prepared request carries the upstream (or account-mapped) name of the model,
			// but the price of a hub line is configured under the line's public name, whatever
			// the account maps it to.
			meta.Model = hubLine.Name
			// A routed request is run by another platform than the line's own, and is still priced
			// as the line the client asked for.
			meta.HubLineRouted = routedSubmission != nil
			if routedSubmission != nil && routedSourceResolution != "" {
				meta.VideoResolution = routedSourceResolution
			}
		}
		cost, costErr := h.gatewayService.CalculatePPVideoCost(c.Request.Context(), apiKey, apiKey.User, account, meta)
		if costErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", costErr.Error())
			return
		}
		if cost == nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "PP video pricing is not configured")
			return
		}
		localTaskID, idErr := service.NewPPVideoLocalTaskID()
		if idErr != nil {
			h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to create video task")
			return
		}
		holdAmount := 0.0
		if cost != nil && cost.ActualCost > 0 {
			holdAmount = cost.ActualCost
		}
		billingStatus := service.PPVideoBillingStatusHeld
		if subscription != nil && apiKey.Group.IsSubscriptionType() {
			billingStatus = service.PPVideoBillingStatusNone
		}
		billingSource := service.BillingSourceSnapshotFromAPIKey(apiKey)
		task, err = h.gatewayService.CreatePPVideoTask(c.Request.Context(), service.CreatePPVideoTaskParams{
			LocalTaskID: localTaskID, UserID: subject.UserID, APIKeyID: apiKey.ID,
			BillingSource: billingSource.Type, BillingOrganizationID: billingSource.OrganizationID,
			BillingMemberUserID: billingSource.OrganizationMemberID,
			GroupID:             apiKey.GroupID, AccountID: account.ID, Platform: account.Platform,
			Operation: operation, Model: publicRequest.Model,
			Status: service.PPVideoTaskStatusSubmitting, BillingStatus: billingStatus,
			RequestHash: payloadHash, IdempotencyKey: idempotencyKey,
			RequestedVideoDurationSeconds:      meta.RequestedDurationSeconds,
			RequestedVideoDurationMilliseconds: meta.RequestedDurationMilliseconds,
			InputVideoDurationMilliseconds:     meta.InputVideoDurationMilliseconds,
			VideoCount:                         meta.VideoCount, VideoResolution: meta.VideoResolution,
			OutputWidth:              meta.OutputWidth,
			OutputHeight:             meta.OutputHeight,
			FrameRate:                meta.FrameRate,
			HasAudio:                 meta.HasAudio,
			KlingMode:                meta.KlingMode,
			BillingFormula:           cost.BillingFormula,
			BillingUnits:             cost.BillingUnits,
			BillingUnitPrice:         cost.BillingUnitPrice,
			BillingFallbackUnitPrice: cost.BillingFallbackUnitPrice,
			HoldID:                   service.PPVideoHoldRequestID(localTaskID),
			CaptureID:                service.PPVideoCaptureRequestID(localTaskID),
			ReleaseID:                service.PPVideoReleaseRequestID(localTaskID),
			EstimatedTotalCost:       cost.TotalCost, HoldAmount: holdAmount, Currency: "USD",
		})
		if err != nil {
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to create video task")
			return
		}
		// 提示词只用于「视频任务」记录页展示，写入失败不能影响任务提交。
		if promptErr := h.gatewayService.RecordPPVideoTaskPrompt(c.Request.Context(), task.LocalTaskID, originalBody); promptErr != nil {
			reqLog.Warn("pp_video.record_prompt_failed", zap.String("local_task_id", task.LocalTaskID), zap.Error(promptErr))
		}
		if billingStatus == service.PPVideoBillingStatusHeld {
			if err := h.gatewayService.ReservePPVideoBalanceHold(c.Request.Context(), task, payloadHash); err != nil {
				_ = h.gatewayService.MarkPPVideoTaskSubmitFailed(c.Request.Context(), task.LocalTaskID, "hold_failed", err.Error())
				h.errorResponse(c, http.StatusPaymentRequired, "insufficient_quota", err.Error())
				return
			}
			h.invalidatePPVideoBalanceCache(c.Request.Context(), task.UserID)
		}
	}
	if task != nil && task.TaskID == "" && (account.Platform == service.PlatformGuangying || account.Platform == service.PlatformMaiTokenSeedance || account.Platform == service.PlatformHongpeiSeedance || account.Platform == service.PlatformVshiSeedance || account.Platform == service.PlatformHuahuoSeedance) {
		publicRequest.UpstreamIdempotencyKey = task.LocalTaskID
	}

	result, err := h.gatewayService.ForwardPPVideoSubmissionBuffered(c.Request.Context(), c, account, operation, requestID, submission, publicRequest)
	// 上游调用不随客户端断开而中止，可能比客户端自己的超时更久（miaowu 的提交就出现过几分钟才返回）。
	// 返回之后的落库、释放冻结和结算必须脱离请求上下文，否则客户端先断开会让写库报
	// context canceled，上游已接受的任务就丢了任务 ID（persist_failed）。
	persistCtx, cancelPersist := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 30*time.Second)
	defer cancelPersist()
	if err != nil {
		if isStatus && !service.IsResponseCommitted(c) && canFallbackToStoredPPVideoStatus(task) {
			reqLog.Info("pp_video.status_stored_fallback",
				zap.String("reason", "upstream_unavailable"),
				zap.String("task_id", task.TaskID),
				zap.Error(err),
			)
			writeStoredPPVideoResponse(c, task)
			return
		}
		if task != nil && task.TaskID == "" {
			if releaseErr := h.gatewayService.ReleasePPVideoBalanceHold(persistCtx, task, payloadHash); releaseErr != nil {
				reqLog.Warn("pp_video.release_hold_after_forward_failed", zap.Error(releaseErr))
			}
			_ = h.gatewayService.MarkPPVideoTaskSubmitFailed(persistCtx, task.LocalTaskID, "upstream_error", ppVideoSubmitFailureMessage(err))
			h.invalidatePPVideoBalanceCache(persistCtx, task.UserID)
		}
		if !service.IsResponseCommitted(c) {
			var upstreamErr *service.PPVideoUpstreamError
			if errors.As(err, &upstreamErr) && upstreamErr.StatusCode >= http.StatusBadRequest && upstreamErr.StatusCode < http.StatusInternalServerError {
				message := strings.TrimSpace(service.ExtractUpstreamErrorMessage(upstreamErr.ResponseBody))
				if message == "" {
					message = "Upstream rejected the video request"
				}
				service.SetOpsUpstreamError(c, upstreamErr.StatusCode, message, string(upstreamErr.ResponseBody))
				h.errorResponse(c, upstreamErr.StatusCode, "upstream_error", message)
				return
			}
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		}
		return
	}

	if task.TaskID == "" {
		if strings.TrimSpace(result.ResponseID) == "" {
			if releaseErr := h.gatewayService.ReleasePPVideoBalanceHold(persistCtx, task, payloadHash); releaseErr != nil {
				reqLog.Warn("pp_video.release_hold_after_missing_task_id_failed", zap.Error(releaseErr))
			}
			_ = h.gatewayService.MarkPPVideoTaskSubmitFailed(persistCtx, task.LocalTaskID, "missing_task_id", "upstream response missing task id")
			h.invalidatePPVideoBalanceCache(persistCtx, task.UserID)
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream response missing video task id")
			return
		}
		status := result.TaskStatus
		if status == "" {
			status = service.PPVideoTaskStatusProcessing
		}
		submittedLocalTaskID := task.LocalTaskID
		pendingTask := task
		task, err = h.markPPVideoTaskSubmittedWithRetry(persistCtx, reqLog, pendingTask, service.MarkPPVideoTaskSubmittedParams{
			LocalTaskID: submittedLocalTaskID, TaskID: result.ResponseID, Status: status,
			GeneratedVideoDurationMilliseconds: result.VideoDurationMilliseconds,
			InputVideoDurationMilliseconds:     result.VideoInputDurationMilliseconds,
			ProviderBillingSeconds:             result.VideoProviderBillingSeconds,
			VideoCount:                         result.VideoCount,
			VideoResolution:                    result.VideoResolution,
			OutputWidth:                        result.VideoOutputWidth,
			OutputHeight:                       result.VideoOutputHeight,
			FrameRate:                          result.VideoFrameRate,
			ResponseStatus:                     result.ResponseStatusCode, ResponseContentType: result.ResponseContentType,
			ResponseBody: string(result.ResponseBody),
		})
		if err != nil {
			// 上游已经接受了任务，这里写库失败时必须留下真实原因，否则只能看到泛化的 503。
			reqLog.Error("pp_video.persist_submitted_task_failed",
				zap.String("local_task_id", submittedLocalTaskID),
				zap.String("upstream_task_id", result.ResponseID),
				zap.String("upstream_status", status),
				zap.String("platform", account.Platform),
				zap.Int("upstream_task_id_len", len(result.ResponseID)),
				zap.Error(err),
			)
			// 客户拿不到任务 ID，这笔冻结没有任何后续会结算它，必须在这里释放并把任务标记失败。
			h.recoverPPVideoTaskAfterPersistFailure(c, reqLog, pendingTask, payloadHash, result.ResponseID)
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to persist video task")
			return
		}
	} else {
		status := service.NormalizePPVideoTaskStatus(result.TaskStatus)
		if status == "" {
			status = service.NormalizePPVideoTaskStatus(task.Status)
		}
		if status == "" {
			status = service.PPVideoTaskStatusProcessing
		}
		statusTaskID := task.TaskID
		task, err = h.gatewayService.MarkPPVideoTaskStatus(persistCtx, service.MarkPPVideoTaskStatusParams{
			TaskID: task.TaskID, Status: status,
			GeneratedVideoDurationMilliseconds: result.VideoDurationMilliseconds,
			ProviderBillingSeconds:             result.VideoProviderBillingSeconds,
			VideoCount:                         result.VideoCount, VideoResolution: result.VideoResolution,
			ResponseStatus: result.ResponseStatusCode, ResponseContentType: result.ResponseContentType,
			ResponseBody: string(result.ResponseBody),
			LastErrorCode: func() string {
				if status == service.PPVideoTaskStatusFailed && strings.TrimSpace(result.ErrorMessage) != "" {
					return "upstream_task_failed"
				}
				return ""
			}(),
			LastErrorMessage: result.ErrorMessage,
		})
		if err != nil {
			reqLog.Error("pp_video.persist_task_status_failed",
				zap.String("upstream_task_id", statusTaskID),
				zap.String("status", status),
				zap.String("platform", account.Platform),
				zap.Error(err),
			)
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to persist video task status")
			return
		}
	}
	if service.IsTerminalPPVideoTaskStatus(task.Status) {
		if err := h.gatewayService.SettlePPVideoTask(persistCtx, &service.PPVideoSettlementInput{
			Task: task, FinalStatus: task.Status, APIKey: apiKey, User: apiKey.User,
			Account: account, Subscription: subscription,
			InboundEndpoint: GetInboundEndpoint(c), UpstreamEndpoint: result.UpstreamEndpoint,
			RequestPayloadHash: task.RequestHash, APIKeyService: h.apiKeyService,
			QuotaPlatform: service.QuotaPlatform(c.Request.Context(), apiKey),
		}); err != nil {
			reqLog.Warn("pp_video.settlement_failed", zap.Error(err))
			if ppVideoTaskBillingPending(task) && !service.IsResponseCommitted(c) {
				writeStoredPPVideoResponse(c, task)
				return
			}
			if !service.IsResponseCommitted(c) {
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to settle video billing")
				return
			}
		} else {
			h.invalidatePPVideoBalanceCache(persistCtx, task.UserID)
		}
	}
	h.gatewayService.WritePPVideoForwardResult(c, result)
}

// ppVideoPersistRetryDelays 是上游已接受任务后写库失败时，两次重试之前的等待时间。
// 总共最多尝试 3 次，等待合计不到 1 秒，不会明显拖慢响应。
var ppVideoPersistRetryDelays = []time.Duration{200 * time.Millisecond, 600 * time.Millisecond}

// markPPVideoTaskSubmittedWithRetry 把上游返回的任务 ID 写回本地任务，遇到瞬时错误时重试。
// 上游此时已经创建并计费，写库失败会让这个任务丢失，所以值得多试几次。
//
// 不重试的情况：任务状态已经不符合写入条件（ErrPPVideoTaskNotFound）、幂等冲突，以及 ctx 已结束。
// 重试前一次若其实已经提交成功、只是应答丢失，下一次会因状态不符返回 NotFound，
// 这时按上游任务 ID 回查一次：查到了就当作成功，避免把已落库的任务当成失败去释放冻结。
func (h *OpenAIGatewayHandler) markPPVideoTaskSubmittedWithRetry(
	ctx context.Context,
	reqLog *zap.Logger,
	pending *service.PPVideoTask,
	params service.MarkPPVideoTaskSubmittedParams,
) (*service.PPVideoTask, error) {
	var lastErr error
	for attempt := 0; attempt <= len(ppVideoPersistRetryDelays); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, lastErr
			case <-time.After(ppVideoPersistRetryDelays[attempt-1]):
			}
		}
		task, err := h.gatewayService.MarkPPVideoTaskSubmitted(ctx, params)
		if err == nil {
			if attempt > 0 {
				reqLog.Warn("pp_video.persist_submitted_task_recovered_by_retry",
					zap.String("local_task_id", params.LocalTaskID),
					zap.String("upstream_task_id", params.TaskID),
					zap.Int("attempts", attempt+1),
				)
			}
			return task, nil
		}
		lastErr = err
		if errors.Is(err, service.ErrPPVideoTaskNotFound) {
			if attempt > 0 && pending != nil {
				// 前一次可能已提交成功但应答丢失：已经带着这个上游任务 ID 落库就算成功。
				if stored, lookupErr := h.gatewayService.GetPPVideoTaskForOwner(ctx, pending.UserID, pending.APIKeyID, params.TaskID); lookupErr == nil &&
					stored != nil && stored.LocalTaskID == params.LocalTaskID {
					reqLog.Warn("pp_video.persist_submitted_task_already_stored",
						zap.String("local_task_id", params.LocalTaskID),
						zap.String("upstream_task_id", params.TaskID),
					)
					return stored, nil
				}
			}
			return nil, err
		}
		if errors.Is(err, service.ErrPPVideoIdempotencyConflict) || ctx.Err() != nil {
			return nil, err
		}
		reqLog.Warn("pp_video.persist_submitted_task_retrying",
			zap.String("local_task_id", params.LocalTaskID),
			zap.String("upstream_task_id", params.TaskID),
			zap.Int("attempt", attempt+1),
			zap.Error(err),
		)
	}
	return nil, lastErr
}

// recoverPPVideoTaskAfterPersistFailure 在上游已接受任务、但本地写回结果失败时收尾：
// 释放余额冻结，并把任务标记为提交失败。客户收到的是 503，没有任务 ID，之后不会有人查询
// 或结算这个任务，不收尾就会一直占用余额，直到轮询器按提交超时兜底。
// 收尾本身失败只记录日志，不改变返回给客户的响应；上游已创建的任务需要与供应商对账。
func (h *OpenAIGatewayHandler) recoverPPVideoTaskAfterPersistFailure(c *gin.Context, reqLog *zap.Logger, task *service.PPVideoTask, payloadHash string, upstreamTaskID string) {
	if h == nil || h.gatewayService == nil || task == nil {
		return
	}
	// 客户端断开不应让收尾半途而废。
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer cancel()

	fields := []zap.Field{
		zap.String("local_task_id", task.LocalTaskID),
		zap.String("upstream_task_id", upstreamTaskID),
	}
	if err := h.gatewayService.ReleasePPVideoBalanceHold(ctx, task, payloadHash); err != nil {
		reqLog.Error("pp_video.release_hold_after_persist_failed", append(fields, zap.Error(err))...)
		return
	}
	if err := h.gatewayService.MarkPPVideoTaskSubmitFailed(ctx, task.LocalTaskID, "persist_failed", "failed to persist upstream task id "+upstreamTaskID); err != nil {
		reqLog.Error("pp_video.mark_failed_after_persist_failed", append(fields, zap.Error(err))...)
	}
	h.invalidatePPVideoBalanceCache(ctx, task.UserID)
	reqLog.Warn("pp_video.recovered_after_persist_failed", fields...)
}

func (h *OpenAIGatewayHandler) invalidatePPVideoBalanceCache(ctx context.Context, userID int64) {
	if h == nil || h.billingCacheService == nil || userID <= 0 {
		return
	}
	if err := h.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
		logger.L().Warn("pp_video.invalidate_balance_cache_failed",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
	}
}

func ppVideoOperationFromRequest(body []byte) service.PPVideoOperation {
	if strings.TrimSpace(gjson.GetBytes(body, "image").String()) != "" ||
		strings.TrimSpace(gjson.GetBytes(body, "image_url").String()) != "" ||
		strings.TrimSpace(gjson.GetBytes(body, "input.image").String()) != "" ||
		strings.TrimSpace(gjson.GetBytes(body, "start_frame_url").String()) != "" ||
		strings.TrimSpace(gjson.GetBytes(body, "end_frame_url").String()) != "" {
		return service.PPVideoOperationKlingImageToVideo
	}
	images := gjson.GetBytes(body, "images")
	if images.Exists() && images.IsArray() && len(images.Array()) > 0 {
		return service.PPVideoOperationKlingImageToVideo
	}
	return service.PPVideoOperationKlingTextToVideo
}

// resolvePPVideoIdempotencyKey enables request deduplication only when the
// caller explicitly opts in. Content-derived keys prevent legitimate repeated
// generations and permanently collide with failed submissions.
func resolvePPVideoIdempotencyKey(rawKey string) string {
	return strings.TrimSpace(rawKey)
}

func shouldReplayStoredPPVideoStatus(task *service.PPVideoTask) bool {
	if task == nil || !service.IsTerminalPPVideoTaskStatus(task.Status) || strings.TrimSpace(task.ResponseBody) == "" {
		return false
	}
	if task.Platform == service.PlatformCTMOSeedance {
		parsed, err := service.ParsePPVideoResponse(task.Platform, []byte(task.ResponseBody))
		if err == nil && !service.IsTerminalPPVideoTaskStatus(parsed.Status) {
			return false
		}
	}
	return true
}

func canFallbackToStoredPPVideoStatus(task *service.PPVideoTask) bool {
	return task != nil && strings.TrimSpace(task.ResponseBody) != ""
}

// ppVideoSubmitFailureMessage 生成提交失败任务的失败原因。传输层失败（没拿到任何 HTTP 响应）时
// err.Error() 只是 "upstream error: 502 (failover)"，看不出原因；这里改用不含上游地址的简短分类，
// 例如 "upstream connection failed: connection reset by upstream"。这个文案会展示给客户，
// 所以不能带上游域名、IP 和端口，完整错误只留在 Ops 错误日志里。
func ppVideoSubmitFailureMessage(err error) string {
	if err == nil {
		return ""
	}
	var failoverErr *service.UpstreamFailoverError
	if errors.As(err, &failoverErr) && failoverErr != nil {
		if cause := strings.TrimSpace(failoverErr.TransportCause); cause != "" {
			return "upstream connection failed: " + cause
		}
	}
	return err.Error()
}

func writeStoredPPVideoResponse(c *gin.Context, task *service.PPVideoTask) {
	if c == nil || task == nil {
		return
	}
	status := task.ResponseStatus
	if status == 0 {
		status = http.StatusOK
	}
	contentType := task.ResponseContentType
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(status, contentType, normalizeStoredPPVideoResponseBody(task))
}

func normalizeStoredPPVideoResponseBody(task *service.PPVideoTask) []byte {
	if task == nil {
		return nil
	}
	raw := []byte(task.ResponseBody)
	if len(raw) == 0 || !gjson.ValidBytes(raw) {
		return raw
	}
	parsed, err := service.ParsePPVideoResponse(task.Platform, raw)
	if err != nil {
		return raw
	}
	if strings.TrimSpace(parsed.TaskID) == "" {
		parsed.TaskID = strings.TrimSpace(task.TaskID)
	}
	if ppVideoTaskBillingPending(task) {
		parsed.Status = service.PPVideoTaskStatusProcessing
	}
	durationMilliseconds := task.GeneratedVideoDurationMilliseconds
	if durationMilliseconds <= 0 {
		durationMilliseconds = task.RequestedVideoDurationMilliseconds
	}
	normalized := service.NormalizePPVideoPublicResponse(task.Platform, raw, service.PPVideoPublicRequest{
		Model:                task.Model,
		DurationMilliseconds: durationMilliseconds,
		VideoCount:           task.VideoCount,
		Resolution:           task.VideoResolution,
	}, parsed)
	if ppVideoTaskBillingPending(task) {
		return normalized
	}
	if task.Platform == service.PlatformCTMOSeedance && parsed.Status != service.PPVideoTaskStatusSucceeded {
		return normalized
	}
	if strings.TrimSpace(gjson.GetBytes(normalized, "video_url").String()) == "" {
		return raw
	}
	return normalized
}

func ppVideoTaskBillingPending(task *service.PPVideoTask) bool {
	if task == nil || task.Platform != service.PlatformMaiTokenSeedance || task.BillingStatus == service.PPVideoBillingStatusCaptured {
		return false
	}
	if service.NormalizePPVideoTaskStatus(task.Status) == service.PPVideoTaskStatusSucceeded {
		return true
	}
	return service.NormalizePPVideoTaskStatus(gjson.Get(task.ResponseBody, "status").String()) == service.PPVideoTaskStatusSucceeded
}
