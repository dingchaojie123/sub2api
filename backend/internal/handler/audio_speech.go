package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AudioSpeech handles Qwen TTS-compatible speech synthesis.
// POST /v1/audio/speech
func (h *OpenAIGatewayHandler) AudioSpeech(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	requestStarted := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if apiKey.Group.Platform != service.PlatformQwenTTS {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Audio speech is only supported for Qwen TTS groups")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.audio_speech",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	request, err := service.ParseQwenTTSSpeechRequest(body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	reqLog = reqLog.With(zap.String("model", request.Model))
	setOpsRequestContext(c, request.Model, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, request.Model)
	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, "qwen_tts", request.Model, body); decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	userRelease, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}

	routingStarted := time.Now()
	selection, err := h.gatewayService.SelectAudioAccount(c.Request.Context(), apiKey.GroupID, apiKey.Group.Platform, request.Model)
	if err != nil || selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", "No eligible Qwen TTS account")
		return
	}
	if selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}
	account := selection.Account
	setOpsSelectedAccount(c, account.ID, account.Platform)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStarted).Milliseconds())
	service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStarted).Milliseconds())

	forwardStarted := time.Now()
	forward, err := h.gatewayService.ForwardQwenTTSSpeech(c.Request.Context(), c, account, body, request.Model)
	service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, time.Since(forwardStarted).Milliseconds())
	if err != nil {
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, request.Model, false, nil)
		if !service.IsResponseCommitted(c) {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream audio request failed")
		}
		reqLog.Warn("audio_speech.forward_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		return
	}
	if forward == nil {
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, request.Model, false, nil)
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream audio request returned no response")
		return
	}

	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, request.Model, forward.Successful, nil)
	if forward.Successful && forward.Result != nil {
		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		payloadHash := service.HashUsageRequestPayload(body)
		quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
		inboundEndpoint := GetInboundEndpoint(c)
		h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
			if recordErr := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             forward.Result,
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            account,
				Subscription:       subscription,
				InboundEndpoint:    inboundEndpoint,
				UpstreamEndpoint:   service.QwenTTSSpeechEndpoint,
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: payloadHash,
				APIKeyService:      h.apiKeyService,
				QuotaPlatform:      quotaPlatform,
				ChannelUsageFields: channelMapping.ToUsageFields(request.Model, forward.Result.UpstreamModel),
			}); recordErr != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.audio_speech"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Int64("account_id", account.ID),
				).Error("audio_speech.record_usage_failed", zap.Error(recordErr))
			}
		})
	}
	h.gatewayService.WriteAudioSpeechForwardResult(c, forward)
}
