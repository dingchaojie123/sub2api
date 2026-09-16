package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GeminiImages exposes Gemini image-generation models through the OpenAI
// Images API used by downstream clients.
func (h *GatewayHandler) GeminiImages(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformGemini {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Images API is not supported for this platform")
		return
	}

	var body []byte
	var err error
	if isMultipartImagesContentType(c.GetHeader("Content-Type")) {
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	} else {
		body, err = readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	}
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	parsed, err := h.openAIGatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	requestModel := parsed.Model
	reqLog := requestLogger(
		c,
		"handler.gateway.gemini_images",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("model", requestModel),
		zap.Bool("stream", parsed.Stream),
	)

	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, requestModel, parsed.ModerationBody()); decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}

	nativeBody, err := service.BuildGeminiImageRequestBody(parsed)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	setOpsRequestContext(c, requestModel, parsed.Stream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(parsed.Stream, false)))

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	geminiConcurrency := NewConcurrencyHelper(h.concurrencyHelper.concurrencyService, SSEPingFormatNone, 0)
	streamStarted := false
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	userReleaseFunc, err := geminiConcurrency.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, parsed.Stream, &streamStarted)
	if err != nil {
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", err.Error())
		return
	}
	userReleaseFunc = wrapReleaseOnDone(c.Request.Context(), userReleaseFunc)
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, _, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", formatRetryAfter(retryAfter))
		}
		h.errorResponse(c, status, "billing_error", message)
		return
	}

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, requestModel)
	modelName := requestModel
	if channelMapping.Mapped {
		modelName = channelMapping.MappedModel
	}

	var sessionHash string
	if parsedReq, parseErr := service.ParseGatewayRequest(service.NewRequestBodyRef(nativeBody), domain.PlatformGemini); parseErr == nil {
		parsedReq.SessionContext = &service.SessionContext{
			ClientIP:  ip.GetClientIP(c),
			UserAgent: c.GetHeader("User-Agent"),
			APIKeyID:  apiKey.ID,
		}
		sessionHash = h.gatewayService.GenerateSessionHash(parsedReq)
	}

	requestCtx := service.WithOpenAIImageGenerationIntent(c.Request.Context())
	failedAccountIDs := make(map[int64]struct{})
	maxSwitches := h.maxAccountSwitchesGemini
	if maxSwitches <= 0 {
		maxSwitches = 3
	}
	geminiAction := "generateContent"
	if parsed.Stream {
		geminiAction = "streamGenerateContent"
	}

	for switchCount := 0; ; switchCount++ {
		selection, err := h.gatewayService.SelectAccountWithLoadAwareness(
			requestCtx,
			apiKey.GroupID,
			sessionHash,
			modelName,
			failedAccountIDs,
			"",
			0,
		)
		if err != nil || selection == nil || selection.Account == nil {
			if err == nil {
				err = errors.New("no available Gemini accounts")
			}
			if len(failedAccountIDs) == 0 {
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, modelName, requestModel, service.PlatformGemini)
				if cls.ModelNotFound {
					h.errorResponse(c, cls.Status, cls.ErrType, cls.Message)
					return
				}
			}
			markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
			h.errorResponse(c, http.StatusServiceUnavailable, "rate_limit_error", "No available Gemini accounts: "+err.Error())
			return
		}

		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)
		accountReleaseFunc := selection.ReleaseFunc
		if !selection.Acquired {
			if selection.WaitPlan == nil {
				h.errorResponse(c, http.StatusServiceUnavailable, "rate_limit_error", "No available Gemini accounts")
				return
			}
			accountReleaseFunc, err = geminiConcurrency.AcquireAccountSlotWithWaitTimeout(
				c,
				account.ID,
				selection.WaitPlan.MaxConcurrency,
				selection.WaitPlan.Timeout,
				parsed.Stream,
				&streamStarted,
			)
			if err != nil {
				h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", err.Error())
				return
			}
		}

		capture := newGeminiImageCaptureWriter()
		originalWriter := c.Writer
		c.Writer = capture
		result, forwardErr := func() (*service.ForwardResult, error) {
			defer func() {
				c.Writer = originalWriter
			}()
			defer func() {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
			}()
			if account.Platform == service.PlatformAntigravity && account.Type != service.AccountTypeAPIKey {
				return h.antigravityGatewayService.ForwardGemini(
					requestCtx,
					c,
					account,
					modelName,
					geminiAction,
					parsed.Stream,
					nativeBody,
					false,
					service.WithForwardGeminiSession(derefGroupID(apiKey.GroupID), sessionHash),
				)
			}
			return h.geminiCompatService.ForwardNative(requestCtx, c, account, modelName, geminiAction, parsed.Stream, nativeBody)
		}()

		if forwardErr != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(forwardErr, &failoverErr) && switchCount < maxSwitches {
				failedAccountIDs[account.ID] = struct{}{}
				continue
			}
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Gemini image generation failed: "+forwardErr.Error())
			return
		}

		responseBody, err := service.ConvertGeminiImageResponse(capture.body.Bytes(), parsed.Stream, parsed.ResponseFormat)
		if err != nil {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}
		if requestID := capture.Header().Get("x-request-id"); requestID != "" {
			c.Header("x-request-id", requestID)
		}
		if parsed.Stream {
			c.Data(http.StatusOK, "text/event-stream; charset=utf-8", responseBody)
		} else {
			c.Data(http.StatusOK, "application/json", responseBody)
		}
		service.MarkResponseCommitted(c)

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
		quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
		h.submitUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
			if err := h.gatewayService.RecordUsageWithLongContext(ctx, &service.RecordUsageLongContextInput{
				Result:                result,
				QuotaPlatform:         quotaPlatform,
				APIKey:                apiKey,
				User:                  apiKey.User,
				Account:               account,
				Subscription:          subscription,
				InboundEndpoint:       inboundEndpoint,
				UpstreamEndpoint:      upstreamEndpoint,
				UserAgent:             userAgent,
				IPAddress:             clientIP,
				RequestPayloadHash:    requestPayloadHash,
				LongContextThreshold:  200000,
				LongContextMultiplier: 2.0,
				APIKeyService:         h.apiKeyService,
				ChannelUsageFields:    channelMapping.ToUsageFields(requestModel, result.UpstreamModel),
			}); err != nil {
				reqLog.Error("gemini_images.record_usage_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			}
		})
		reqLog.Debug("gemini_images.request_completed", zap.Int64("account_id", account.ID))
		return
	}
}

type geminiImageCaptureWriter struct {
	gin.ResponseWriter
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func newGeminiImageCaptureWriter() *geminiImageCaptureWriter {
	return &geminiImageCaptureWriter{header: make(http.Header)}
}

func (w *geminiImageCaptureWriter) Header() http.Header {
	return w.header
}

func (w *geminiImageCaptureWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.status = statusCode
	w.wroteHeader = true
}

func (w *geminiImageCaptureWriter) WriteHeaderNow() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
}

func (w *geminiImageCaptureWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	return w.body.Write(data)
}

func (w *geminiImageCaptureWriter) WriteString(data string) (int, error) {
	w.WriteHeaderNow()
	return w.body.WriteString(data)
}

func (w *geminiImageCaptureWriter) Flush() {
	w.WriteHeaderNow()
}

func (w *geminiImageCaptureWriter) Status() int {
	return w.status
}

func (w *geminiImageCaptureWriter) Size() int {
	return w.body.Len()
}

func (w *geminiImageCaptureWriter) Written() bool {
	return w.wroteHeader || w.body.Len() > 0
}

func formatRetryAfter(seconds int) string {
	return strconv.Itoa(seconds)
}
