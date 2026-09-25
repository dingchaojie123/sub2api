package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
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
	h.gatewayService.WritePPVideoForwardResult(c, result)
}

func (h *OpenAIGatewayHandler) handlePPVideo(c *gin.Context, operation service.PPVideoOperation, requestID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil || !service.IsPPVideoPlatform(apiKey.Group.Platform) {
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
	publicRequest := service.PPVideoPublicRequest{}
	var payloadHash, rawPayloadHash, idempotencyKey string
	if !isStatus {
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
		if operation == "" {
			if apiKey.Group.Platform == service.PlatformKling {
				operation = ppVideoOperationFromRequest(body)
			} else {
				operation = service.PPVideoOperationGeneric
			}
		}
		preparedBody, preparedPublicRequest, prepareErr := service.PreparePPVideoRequestBody(apiKey.Group.Platform, operation, body)
		if prepareErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", prepareErr.Error())
			return
		}
		body = preparedBody
		publicRequest = preparedPublicRequest
		model := strings.TrimSpace(publicRequest.Model)
		if model == "" {
			model = strings.TrimSpace(publicRequest.UpstreamModel)
		}
		if model == "" {
			model = apiKey.Group.Platform
		}
		setOpsRequestContext(c, model, false)
		setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
		if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, originalBody); decision != nil && !decision.AllowNextStage {
			h.openAISecurityAuditError(c, decision)
			return
		}
		payloadHash = service.HashUsageRequestPayload(body)
		rawIdempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if len(rawIdempotencyKey) > service.PPVideoIdempotencyKeyMaxLength {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Idempotency-Key must be at most 255 characters")
			return
		}
		idempotencyKey = rawIdempotencyKey
		if idempotencyKey == "" {
			idempotencyKey = "pp_video:" + payloadHash
		}
		if existing, lookupErr := h.gatewayService.GetPPVideoTaskByIdempotencyKey(c.Request.Context(), subject.UserID, apiKey.ID, idempotencyKey); lookupErr == nil && existing != nil {
			if existing.RequestHash != payloadHash && existing.RequestHash != rawPayloadHash {
				h.errorResponse(c, http.StatusConflict, "idempotency_error", "Idempotency-Key was reused with a different video request")
				return
			}
			if existing.ResponseBody != "" {
				c.Header("X-Idempotency-Replayed", "true")
				writeStoredPPVideoResponse(c, existing)
				return
			}
			h.errorResponse(c, http.StatusConflict, "idempotency_error", "Video generation request is still being submitted")
			return
		} else if lookupErr != nil && !errors.Is(lookupErr, service.ErrPPVideoTaskNotFound) {
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Video task storage is unavailable")
			return
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
		return h.gatewayService.SelectPPVideoAccountForRequest(c.Request.Context(), apiKey.GroupID, apiKey.Group.Platform, publicRequest)
	}()
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

	if task == nil {
		var prepareErr error
		body, publicRequest, prepareErr = service.PreparePPVideoRequestBodyForAccount(account, operation, originalBody)
		if prepareErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", prepareErr.Error())
			return
		}
		meta := service.PPVideoBillingMetadataFromRequest(account.Platform, body)
		if meta.ValidationError != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", meta.ValidationError.Error())
			return
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
		task, err = h.gatewayService.CreatePPVideoTask(c.Request.Context(), service.CreatePPVideoTaskParams{
			LocalTaskID: localTaskID, UserID: subject.UserID, APIKeyID: apiKey.ID,
			GroupID: apiKey.GroupID, AccountID: account.ID, Platform: account.Platform,
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
		if billingStatus == service.PPVideoBillingStatusHeld {
			if err := h.gatewayService.ReservePPVideoBalanceHold(c.Request.Context(), task, payloadHash); err != nil {
				_ = h.gatewayService.MarkPPVideoTaskSubmitFailed(c.Request.Context(), task.LocalTaskID, "hold_failed", err.Error())
				h.errorResponse(c, http.StatusPaymentRequired, "insufficient_quota", err.Error())
				return
			}
		}
	}

	result, err := h.gatewayService.ForwardPPVideoBuffered(c.Request.Context(), c, account, operation, requestID, body, publicRequest)
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
			_ = h.gatewayService.ReleasePPVideoBalanceHold(c.Request.Context(), task, payloadHash)
			_ = h.gatewayService.MarkPPVideoTaskSubmitFailed(c.Request.Context(), task.LocalTaskID, "upstream_error", err.Error())
		}
		if !service.IsResponseCommitted(c) {
			var upstreamErr *service.PPVideoUpstreamError
			if errors.As(err, &upstreamErr) && upstreamErr.StatusCode >= http.StatusBadRequest && upstreamErr.StatusCode < http.StatusInternalServerError {
				message := strings.TrimSpace(service.ExtractUpstreamErrorMessage(upstreamErr.ResponseBody))
				if message == "" {
					message = "ByteDance reference image registration failed"
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
			_ = h.gatewayService.ReleasePPVideoBalanceHold(c.Request.Context(), task, payloadHash)
			_ = h.gatewayService.MarkPPVideoTaskSubmitFailed(c.Request.Context(), task.LocalTaskID, "missing_task_id", "upstream response missing task id")
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream response missing video task id")
			return
		}
		status := result.TaskStatus
		if status == "" {
			status = service.PPVideoTaskStatusProcessing
		}
		task, err = h.gatewayService.MarkPPVideoTaskSubmitted(c.Request.Context(), service.MarkPPVideoTaskSubmittedParams{
			LocalTaskID: task.LocalTaskID, TaskID: result.ResponseID, Status: status,
			ResponseStatus: result.ResponseStatusCode, ResponseContentType: result.ResponseContentType,
			ResponseBody: string(result.ResponseBody),
		})
		if err != nil {
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
		task, err = h.gatewayService.MarkPPVideoTaskStatus(c.Request.Context(), service.MarkPPVideoTaskStatusParams{
			TaskID: task.TaskID, Status: status,
			GeneratedVideoDurationMilliseconds: result.VideoDurationMilliseconds,
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
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to persist video task status")
			return
		}
	}
	if service.IsTerminalPPVideoTaskStatus(task.Status) {
		if err := h.gatewayService.SettlePPVideoTask(c.Request.Context(), &service.PPVideoSettlementInput{
			Task: task, FinalStatus: task.Status, APIKey: apiKey, User: apiKey.User,
			Account: account, Subscription: subscription,
			InboundEndpoint: GetInboundEndpoint(c), UpstreamEndpoint: result.UpstreamEndpoint,
			RequestPayloadHash: task.RequestHash, APIKeyService: h.apiKeyService,
			QuotaPlatform: service.QuotaPlatform(c.Request.Context(), apiKey),
		}); err != nil {
			reqLog.Warn("pp_video.settlement_failed", zap.Error(err))
			if !service.IsResponseCommitted(c) {
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to settle video billing")
				return
			}
		}
	}
	h.gatewayService.WritePPVideoForwardResult(c, result)
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

func shouldReplayStoredPPVideoStatus(task *service.PPVideoTask) bool {
	return task != nil &&
		service.IsTerminalPPVideoTaskStatus(task.Status) &&
		strings.TrimSpace(task.ResponseBody) != ""
}

func canFallbackToStoredPPVideoStatus(task *service.PPVideoTask) bool {
	return task != nil && strings.TrimSpace(task.ResponseBody) != ""
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
	if strings.TrimSpace(gjson.GetBytes(normalized, "video_url").String()) == "" {
		return raw
	}
	return normalized
}
