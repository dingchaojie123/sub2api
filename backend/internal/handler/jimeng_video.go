package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) JimengVideoGeneration(c *gin.Context) {
	h.handleJimengVideo(c, service.JimengVideoEndpointGenerations, "")
}

func (h *OpenAIGatewayHandler) JimengVideoStatus(c *gin.Context) {
	h.handleJimengVideo(c, service.JimengVideoEndpointStatus, c.Param("request_id"))
}

func (h *OpenAIGatewayHandler) handleJimengVideo(c *gin.Context, endpoint service.JimengVideoEndpoint, requestID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	requestStart := time.Now()
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformJimeng {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}

	reqLog := requestLogger(
		c,
		"handler.openai_gateway.jimeng_video",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("endpoint", string(endpoint)),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	var body []byte
	var err error
	idempotencyKey := ""
	requestPayloadHash := ""
	if endpoint == service.JimengVideoEndpointGenerations {
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
				return
			}
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
			return
		}
		if len(body) == 0 {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
			return
		}
		rawIdempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if len(rawIdempotencyKey) > service.JimengVideoIdempotencyKeyMaxLength {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Idempotency-Key must be at most 255 characters")
			return
		}
		requestPayloadHash = service.HashUsageRequestPayload(body)
		idempotencyKey, legacyIdempotency := resolveJimengVideoIdempotencyKey(rawIdempotencyKey, requestPayloadHash)
		if legacyIdempotency {
			reqLog.Info("jimeng_video.legacy_idempotency_key_fallback",
				zap.String("request_payload_hash", requestPayloadHash),
			)
		}
		existingTask, lookupErr := h.gatewayService.GetJimengVideoTaskByIdempotencyKey(
			c.Request.Context(), subject.UserID, apiKey.ID, idempotencyKey,
		)
		if lookupErr == nil {
			if strings.TrimSpace(existingTask.RequestHash) != requestPayloadHash {
				h.errorResponse(c, http.StatusConflict, "idempotency_error", "Idempotency-Key was reused with a different video request")
				return
			}
			if strings.TrimSpace(existingTask.ResponseBody) == "" {
				h.writeUnsubmittedJimengVideoIdempotencyResponse(c, existingTask)
				return
			}
			c.Header("X-Idempotency-Replayed", "true")
			writeStoredJimengVideoResponse(c, existingTask)
			return
		}
		if !errors.Is(lookupErr, service.ErrJimengVideoTaskNotFound) {
			reqLog.Warn("jimeng_video.idempotency_lookup_failed", zap.Error(lookupErr))
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Video idempotency storage is unavailable")
			return
		}
	} else if strings.TrimSpace(requestID) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "request_id is required")
		return
	}

	model := service.JimengVideoRoutingModel
	reqLog = reqLog.With(zap.String("model", model))
	setOpsRequestContext(c, model, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	if endpoint == service.JimengVideoEndpointGenerations {
		decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, body)
		if decision != nil && !decision.AllowNextStage {
			h.openAISecurityAuditError(c, decision)
			return
		}
	}

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
	if endpoint == service.JimengVideoEndpointGenerations {
		if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, quotaPlatform); err != nil {
			reqLog.Info("jimeng_video.billing_eligibility_check_failed", zap.Error(err))
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.errorResponse(c, status, code, message)
			return
		}
	}

	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	boundLookupAccountID := int64(0)
	var videoTask *service.JimengVideoTask
	legacyJimengVideoStatus := false
	if endpoint == service.JimengVideoEndpointStatus {
		sessionHash = service.VideoRequestSessionHash(service.PlatformJimeng, requestID, subject.UserID, apiKey.ID)
		videoTask, err = h.gatewayService.GetJimengVideoTaskForOwner(c.Request.Context(), subject.UserID, apiKey.ID, requestID)
		if err != nil || videoTask == nil || videoTask.AccountID <= 0 {
			if !errors.Is(err, service.ErrJimengVideoTaskNotFound) {
				reqLog.Warn("jimeng_video.lookup_task_failed", zap.Error(err))
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Video task storage is unavailable")
				return
			}
			boundLookupAccountID, err = h.gatewayService.ResolveVideoRequestAccount(
				c.Request.Context(), service.PlatformJimeng, apiKey.GroupID, requestID, subject.UserID, apiKey.ID,
			)
			if err != nil || boundLookupAccountID <= 0 {
				reqLog.Info("jimeng_video.lookup_task_missing", zap.Error(err))
				h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
				return
			}
			legacyJimengVideoStatus = true
		} else {
			boundLookupAccountID = videoTask.AccountID
		}
	}

	requestCtx := c.Request.Context()
	failedAccountIDs := make(map[int64]struct{})
	routingStart := time.Now()
	var selection *service.AccountSelectionResult
	var scheduleDecision service.OpenAIAccountScheduleDecision
	if endpoint == service.JimengVideoEndpointStatus && videoTask != nil && !legacyJimengVideoStatus {
		selection, scheduleDecision, err = h.gatewayService.SelectJimengVideoTaskAccountForStatus(requestCtx, videoTask.AccountID)
		if err != nil {
			if failoverClientGone(c) {
				reqLog.Info("jimeng_video.bound_account_select_aborted_client_disconnected", zap.Error(err))
				return
			}
			reqLog.Warn("jimeng_video.bound_account_unavailable",
				zap.Int64("bound_account_id", videoTask.AccountID),
				zap.Error(err),
			)
			h.errorResponse(c, http.StatusServiceUnavailable, "jimeng_task_account_unavailable", "Video request account is unavailable")
			return
		}
	} else {
		selection, scheduleDecision, err = h.gatewayService.SelectAccountWithSchedulerForCapability(
			requestCtx,
			apiKey.GroupID,
			"",
			sessionHash,
			model,
			failedAccountIDs,
			service.OpenAIUpstreamTransportHTTPSSE,
			"",
			false,
			false,
			false,
			service.PlatformJimeng,
		)
		if err != nil {
			if failoverClientGone(c) {
				reqLog.Info("jimeng_video.account_select_aborted_client_disconnected", zap.Error(err))
				return
			}
			if errors.Is(err, service.ErrNoAvailableAccounts) {
				markOpsRoutingCapacityLimited(c)
				h.errorResponse(c, http.StatusServiceUnavailable, "jimeng_no_eligible_account", "No eligible Jimeng video accounts")
				return
			}
			cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, model, model, service.PlatformJimeng)
			if !cls.ModelNotFound {
				markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
			}
			h.errorResponse(c, cls.Status, cls.ErrType, cls.Message)
			return
		}
		if boundLookupAccountID > 0 && selection != nil && selection.Account != nil && selection.Account.ID != boundLookupAccountID {
			reqLog.Warn("jimeng_video.lookup_bound_account_unavailable",
				zap.Int64("bound_account_id", boundLookupAccountID),
				zap.Int64("selected_account_id", selection.Account.ID),
			)
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}
	}
	if selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimited(c)
		h.errorResponse(c, http.StatusServiceUnavailable, "jimeng_no_eligible_account", "No eligible Jimeng video accounts")
		return
	}

	reqLog.Debug("jimeng_video.account_schedule_decision",
		zap.String("layer", scheduleDecision.Layer),
		zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
		zap.Int("candidate_count", scheduleDecision.CandidateCount),
		zap.Int("top_k", scheduleDecision.TopK),
		zap.Int64("latency_ms", scheduleDecision.LatencyMs),
		zap.Float64("load_skew", scheduleDecision.LoadSkew),
	)

	account := selection.Account
	sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
	setOpsSelectedAccount(c, account.ID, account.Platform)

	accountReleaseFunc, accountAcquired := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, false, &streamStarted, reqLog)
	if !accountAcquired {
		return
	}
	if accountReleaseFunc != nil {
		defer func() {
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
		}()
	}

	if endpoint == service.JimengVideoEndpointGenerations {
		videoTask, err = h.createAndReserveJimengVideoTask(
			c,
			reqLog,
			apiKey,
			subject,
			subscription,
			account,
			body,
			requestPayloadHash,
			idempotencyKey,
		)
		if err != nil {
			return
		}
	}

	service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
	forwardStart := time.Now()
	writerSizeBeforeForward := c.Writer.Size()
	result, err := h.gatewayService.ForwardJimengVideoBuffered(requestCtx, c, account, endpoint, requestID, body)
	if accountReleaseFunc != nil {
		accountReleaseFunc()
		accountReleaseFunc = nil
	}

	forwardDurationMs := time.Since(forwardStart).Milliseconds()
	upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
	responseLatencyMs := forwardDurationMs
	if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
		responseLatencyMs = forwardDurationMs - upstreamLatencyMs
	}
	service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)

	if err != nil {
		if endpoint == service.JimengVideoEndpointGenerations && videoTask != nil {
			if releaseErr := h.gatewayService.ReleaseJimengVideoBalanceHold(requestCtx, videoTask, requestPayloadHash); releaseErr != nil {
				reqLog.Warn("jimeng_video.release_hold_after_forward_failed", zap.Error(releaseErr))
			}
			if markErr := h.gatewayService.MarkJimengVideoTaskSubmitFailed(requestCtx, videoTask.LocalTaskID, "upstream_error", err.Error()); markErr != nil {
				reqLog.Warn("jimeng_video.mark_submit_failed", zap.Error(markErr))
			}
			h.invalidateJimengVideoBalanceCache(requestCtx, videoTask.UserID)
		}
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, model, false, nil)
		if !service.IsResponseCommitted(c) && c.Writer.Size() == writerSizeBeforeForward {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		}
		reqLog.Warn("jimeng_video.forward_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		return
	}

	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, model, true, nil)
	if endpoint == service.JimengVideoEndpointGenerations {
		if strings.TrimSpace(result.ResponseID) == "" {
			if videoTask != nil {
				if releaseErr := h.gatewayService.ReleaseJimengVideoBalanceHold(requestCtx, videoTask, requestPayloadHash); releaseErr != nil {
					reqLog.Warn("jimeng_video.release_hold_after_missing_task_id", zap.Error(releaseErr))
				}
				if markErr := h.gatewayService.MarkJimengVideoTaskSubmitFailed(requestCtx, videoTask.LocalTaskID, "missing_task_id", "upstream response missing task id"); markErr != nil {
					reqLog.Warn("jimeng_video.mark_missing_task_id_failed", zap.Error(markErr))
				}
				h.invalidateJimengVideoBalanceCache(requestCtx, videoTask.UserID)
			}
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream response missing video task id")
			return
		}
		status := strings.TrimSpace(result.TaskStatus)
		if status == "" {
			status = service.JimengTaskStatusProcessing
		}
		submittedTask, markErr := h.gatewayService.MarkJimengVideoTaskSubmitted(requestCtx, service.MarkJimengVideoSubmittedParams{
			LocalTaskID:        videoTask.LocalTaskID,
			TaskID:             result.ResponseID,
			Status:             status,
			ResponseStatus:      result.ResponseStatusCode,
			ResponseContentType: result.ResponseContentType,
			ResponseBody:        string(result.ResponseBody),
		})
		if markErr != nil {
			if releaseErr := h.gatewayService.ReleaseJimengVideoBalanceHold(requestCtx, videoTask, requestPayloadHash); releaseErr != nil {
				reqLog.Warn("jimeng_video.release_hold_after_submit_persist_failed", zap.Error(releaseErr))
			}
			if submitFailedErr := h.gatewayService.MarkJimengVideoTaskSubmitFailed(requestCtx, videoTask.LocalTaskID, "persist_failed", markErr.Error()); submitFailedErr != nil {
				reqLog.Warn("jimeng_video.mark_submit_persist_failed", zap.Error(submitFailedErr))
			}
			h.invalidateJimengVideoBalanceCache(requestCtx, subject.UserID)
			reqLog.Warn("jimeng_video.persist_submitted_task_failed", zap.Error(markErr))
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to persist video task")
			return
		}
		videoTask = submittedTask
		if strings.TrimSpace(submittedTask.TaskID) != strings.TrimSpace(result.ResponseID) || submittedTask.SettledAt != nil {
			reqLog.Warn("jimeng_video.submit_result_ignored_after_task_finalized",
				zap.String("local_task_id", submittedTask.LocalTaskID),
				zap.String("upstream_task_id", result.ResponseID),
				zap.String("persisted_task_id", submittedTask.TaskID),
				zap.String("status", submittedTask.Status),
				zap.String("billing_status", submittedTask.BillingStatus),
			)
			if strings.TrimSpace(submittedTask.ResponseBody) != "" {
				writeStoredJimengVideoResponse(c, submittedTask)
				return
			}
			h.writeUnsubmittedJimengVideoIdempotencyResponse(c, submittedTask)
			return
		}
		if err := h.gatewayService.BindVideoRequestAccount(
			requestCtx, service.PlatformJimeng, apiKey.GroupID, result.ResponseID, subject.UserID, apiKey.ID, account.ID,
		); err != nil {
			reqLog.Warn("jimeng_video.bind_request_account_failed",
				zap.Int64("account_id", account.ID),
				zap.String("request_id", result.ResponseID),
				zap.Error(err),
			)
		}
		h.gatewayService.WriteJimengVideoForwardResult(c, result)
		return
	}

	if endpoint == service.JimengVideoEndpointStatus {
		if legacyJimengVideoStatus || videoTask == nil {
			h.gatewayService.WriteJimengVideoForwardResult(c, result)
			return
		}
		status := strings.TrimSpace(result.TaskStatus)
		if status == "" && videoTask != nil {
			status = videoTask.Status
		}
		if status == "" {
			status = service.JimengTaskStatusProcessing
		}
		updatedTask, updateErr := h.gatewayService.MarkJimengVideoTaskStatus(requestCtx, service.MarkJimengVideoStatusParams{
			TaskID:              requestID,
			Status:              status,
			ResponseStatus:      result.ResponseStatusCode,
			ResponseContentType: result.ResponseContentType,
			ResponseBody:        string(result.ResponseBody),
		})
		if updateErr != nil {
			reqLog.Warn("jimeng_video.persist_status_failed", zap.Error(updateErr))
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to persist video task status")
			return
		}
		persistedStatus := service.NormalizeJimengTaskStatus(updatedTask.Status)
		if persistedStatus == "" {
			persistedStatus = service.NormalizeJimengTaskStatus(status)
		}
		if updatedTask.SettledAt != nil && strings.TrimSpace(updatedTask.ResponseBody) != "" {
			writeStoredJimengVideoResponse(c, updatedTask)
			return
		}
		if service.IsTerminalJimengVideoTaskStatus(persistedStatus) {
			if settleErr := h.gatewayService.SettleJimengVideoTask(requestCtx, &service.JimengVideoSettlementInput{
				Task:               updatedTask,
				FinalStatus:        persistedStatus,
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            account,
				Subscription:       subscription,
				InboundEndpoint:    GetInboundEndpoint(c),
				UpstreamEndpoint:   result.UpstreamEndpoint,
				UserAgent:          c.GetHeader("User-Agent"),
				IPAddress:          ip.GetClientIP(c),
				RequestPayloadHash: strings.TrimSpace(updatedTask.RequestHash),
				APIKeyService:      h.apiKeyService,
				QuotaPlatform:      quotaPlatform,
			}); settleErr != nil {
				reqLog.Warn("jimeng_video.settlement_failed", zap.Error(settleErr))
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to settle video billing")
				return
			}
			h.invalidateJimengVideoBalanceCache(requestCtx, updatedTask.UserID)
		}
		if service.IsTerminalJimengVideoTaskStatus(persistedStatus) && persistedStatus != service.NormalizeJimengTaskStatus(status) && strings.TrimSpace(updatedTask.ResponseBody) != "" {
			writeStoredJimengVideoResponse(c, updatedTask)
			return
		}
		h.gatewayService.WriteJimengVideoForwardResult(c, result)
		return
	}
}

const legacyJimengVideoIdempotencyKeyPrefix = "legacy:"

var errJimengVideoResponseAlreadyWritten = errors.New("jimeng video response already written")

// resolveJimengVideoIdempotencyKey keeps old callers working by deriving a stable
// internal key from the payload hash when the header is absent.
func resolveJimengVideoIdempotencyKey(rawKey string, requestPayloadHash string) (string, bool) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey != "" {
		return rawKey, false
	}
	requestPayloadHash = strings.TrimSpace(requestPayloadHash)
	if requestPayloadHash == "" {
		return "", true
	}
	return legacyJimengVideoIdempotencyKeyPrefix + requestPayloadHash, true
}

func (h *OpenAIGatewayHandler) createAndReserveJimengVideoTask(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	subscription *service.UserSubscription,
	account *service.Account,
	body []byte,
	requestPayloadHash string,
	idempotencyKey string,
) (*service.JimengVideoTask, error) {
	meta := service.JimengVideoBillingMetadataFromRequest(body)
	cost, err := h.gatewayService.CalculateJimengVideoCost(c.Request.Context(), apiKey, apiKey.User, meta)
	if err != nil {
		reqLog.Warn("jimeng_video.calculate_hold_cost_failed", zap.Error(err))
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to calculate video generation cost")
		return nil, err
	}
	holdAmount := 0.0
	if cost != nil && cost.ActualCost > 0 {
		holdAmount = cost.ActualCost
	}
	estimatedTotalCost := 0.0
	if cost != nil && cost.TotalCost > 0 {
		estimatedTotalCost = cost.TotalCost
	}
	localTaskID, err := service.NewJimengVideoLocalTaskID()
	if err != nil {
		reqLog.Warn("jimeng_video.generate_local_task_id_failed", zap.Error(err))
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to create video task")
		return nil, err
	}
	billingStatus := service.JimengVideoBillingStatusHeld
	if subscription != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		billingStatus = service.JimengVideoBillingStatusNone
	}
	task, err := h.gatewayService.CreateJimengVideoTask(c.Request.Context(), service.CreateJimengVideoTaskParams{
		LocalTaskID:          localTaskID,
		UserID:               subject.UserID,
		APIKeyID:             apiKey.ID,
		GroupID:              apiKey.GroupID,
		AccountID:            account.ID,
		Model:                service.JimengVideoBillingModel,
		Status:               service.JimengVideoTaskStatusSubmitting,
		BillingStatus:        billingStatus,
		RequestHash:          strings.TrimSpace(requestPayloadHash),
		IdempotencyKey:       strings.TrimSpace(idempotencyKey),
		HoldID:               service.JimengVideoHoldRequestID(localTaskID),
		CaptureID:            service.JimengVideoCaptureRequestID(localTaskID),
		ReleaseID:            service.JimengVideoReleaseRequestID(localTaskID),
		EstimatedTotalCost:   estimatedTotalCost,
		HoldAmount:           holdAmount,
		Currency:             "USD",
		VideoDurationSeconds: meta.VideoDurationSeconds,
		VideoResolution:      meta.VideoResolution,
	})
	if err != nil {
		if errors.Is(err, service.ErrJimengVideoIdempotencyConflict) {
			existing, lookupErr := h.gatewayService.GetJimengVideoTaskByIdempotencyKey(c.Request.Context(), subject.UserID, apiKey.ID, idempotencyKey)
			if lookupErr == nil && existing != nil {
				if strings.TrimSpace(existing.RequestHash) != strings.TrimSpace(requestPayloadHash) {
					h.errorResponse(c, http.StatusConflict, "idempotency_error", "Idempotency-Key was reused with a different video request")
					return nil, err
				}
				if strings.TrimSpace(existing.ResponseBody) != "" {
					c.Header("X-Idempotency-Replayed", "true")
					writeStoredJimengVideoResponse(c, existing)
					return nil, errJimengVideoResponseAlreadyWritten
				}
				h.writeUnsubmittedJimengVideoIdempotencyResponse(c, existing)
				return nil, err
			}
		}
		reqLog.Warn("jimeng_video.create_task_failed", zap.Error(err))
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to create video task")
		return nil, err
	}
	if billingStatus == service.JimengVideoBillingStatusHeld && task.HoldAmount > 0 {
		if err := h.gatewayService.ReserveJimengVideoBalanceHold(c.Request.Context(), task, requestPayloadHash); err != nil {
			if markErr := h.gatewayService.MarkJimengVideoTaskSubmitFailed(c.Request.Context(), task.LocalTaskID, "hold_failed", err.Error()); markErr != nil {
				reqLog.Warn("jimeng_video.mark_hold_failed_task_failed", zap.Error(markErr))
			}
			if errors.Is(err, service.ErrJimengVideoInsufficientBalance) {
				h.errorResponse(c, http.StatusPaymentRequired, "insufficient_quota", "Insufficient balance for video generation")
				return nil, err
			}
			reqLog.Warn("jimeng_video.reserve_hold_failed", zap.Error(err))
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Failed to reserve video generation balance")
			return nil, err
		}
		h.invalidateJimengVideoBalanceCache(c.Request.Context(), task.UserID)
	}
	return task, nil
}

func writeStoredJimengVideoResponse(c *gin.Context, task *service.JimengVideoTask) {
	if c == nil || task == nil {
		return
	}
	status := task.ResponseStatus
	if status == 0 {
		status = http.StatusOK
	}
	contentType := strings.TrimSpace(task.ResponseContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(status, contentType, []byte(task.ResponseBody))
}

func (h *OpenAIGatewayHandler) writeUnsubmittedJimengVideoIdempotencyResponse(c *gin.Context, task *service.JimengVideoTask) {
	if task != nil && service.NormalizeJimengTaskStatus(task.Status) == service.JimengTaskStatusFailed {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Previous video generation submission failed; use a new Idempotency-Key to retry")
		return
	}
	h.errorResponse(c, http.StatusConflict, "idempotency_error", "Video generation request is still being submitted")
}

func (h *OpenAIGatewayHandler) invalidateJimengVideoBalanceCache(ctx context.Context, userID int64) {
	if h == nil || h.billingCacheService == nil || userID <= 0 {
		return
	}
	if err := h.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
		logger.L().Warn("jimeng_video.invalidate_balance_cache_failed",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
	}
}
