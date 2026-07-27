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
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, quotaPlatform); err != nil {
		reqLog.Info("jimeng_video.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}

	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	boundLookupAccountID := int64(0)
	if endpoint == service.JimengVideoEndpointStatus {
		sessionHash = service.VideoRequestSessionHash(service.PlatformJimeng, requestID, subject.UserID, apiKey.ID)
		boundLookupAccountID, err = h.gatewayService.ResolveVideoRequestAccount(
			c.Request.Context(), service.PlatformJimeng, apiKey.GroupID, requestID, subject.UserID, apiKey.ID,
		)
		if err != nil || boundLookupAccountID <= 0 {
			reqLog.Info("jimeng_video.lookup_owner_binding_missing", zap.Error(err))
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}
	}

	requestCtx := c.Request.Context()
	failedAccountIDs := make(map[int64]struct{})
	routingStart := time.Now()
	selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
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
	if selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimited(c)
		h.errorResponse(c, http.StatusServiceUnavailable, "jimeng_no_eligible_account", "No eligible Jimeng video accounts")
		return
	}
	if boundLookupAccountID > 0 && selection.Account.ID != boundLookupAccountID {
		reqLog.Warn("jimeng_video.lookup_bound_account_unavailable",
			zap.Int64("bound_account_id", boundLookupAccountID),
			zap.Int64("selected_account_id", selection.Account.ID),
		)
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
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

	service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
	forwardStart := time.Now()
	writerSizeBeforeForward := c.Writer.Size()
	result, err := func() (*service.OpenAIForwardResult, error) {
		defer func() {
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
		}()
		return h.gatewayService.ForwardJimengVideo(requestCtx, c, account, endpoint, requestID, body)
	}()

	forwardDurationMs := time.Since(forwardStart).Milliseconds()
	upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
	responseLatencyMs := forwardDurationMs
	if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
		responseLatencyMs = forwardDurationMs - upstreamLatencyMs
	}
	service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)

	if err != nil {
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, model, false, nil)
		if !service.IsResponseCommitted(c) && c.Writer.Size() == writerSizeBeforeForward {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		}
		reqLog.Warn("jimeng_video.forward_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		return
	}

	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, model, true, nil)
	if endpoint == service.JimengVideoEndpointGenerations && strings.TrimSpace(result.ResponseID) != "" {
		if err := h.gatewayService.BindVideoRequestAccount(
			requestCtx, service.PlatformJimeng, apiKey.GroupID, result.ResponseID, subject.UserID, apiKey.ID, account.ID,
		); err != nil {
			reqLog.Warn("jimeng_video.bind_request_account_failed",
				zap.Int64("account_id", account.ID),
				zap.String("request_id", result.ResponseID),
				zap.Error(err),
			)
		}
	}
	if result.HasUsage {
		recordJimengVideoUsage(c, h, reqLog, apiKey, subject, subscription, account, result, body, requestID, quotaPlatform)
	}
}

func recordJimengVideoUsage(
	c *gin.Context,
	h *OpenAIGatewayHandler,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	subscription *service.UserSubscription,
	account *service.Account,
	result *service.OpenAIForwardResult,
	body []byte,
	requestID string,
	quotaPlatform string,
) {
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	payloadForHash := body
	if len(payloadForHash) == 0 && strings.TrimSpace(requestID) != "" {
		payloadForHash = []byte(requestID)
	}
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := result.UpstreamEndpoint
	if strings.TrimSpace(upstreamEndpoint) == "" {
		upstreamEndpoint = GetUpstreamEndpoint(c, account.Platform)
	}
	channelUsageFields := service.ChannelUsageFields{
		OriginalModel:      service.JimengVideoBillingModel,
		ChannelMappedModel: service.JimengVideoBillingModel,
	}
	h.submitOpenAIUsageRecordTask(c.Request.Context(), result, func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: service.HashUsageRequestPayload(payloadForHash),
			APIKeyService:      h.apiKeyService,
			QuotaPlatform:      quotaPlatform,
			ChannelUsageFields: channelUsageFields,
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.jimeng_video"),
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Any("group_id", apiKey.GroupID),
				zap.Int64("account_id", account.ID),
			).Error("jimeng_video.record_usage_failed", zap.Error(err))
			reqLog.Debug("jimeng_video.record_usage_failed", zap.Error(err))
		}
	})
}
