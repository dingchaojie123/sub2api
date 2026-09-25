package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func (s *OpenAIGatewayService) handleMidjourneyImagesModerationError(c *gin.Context, account *Account, resp *http.Response, body []byte) *OpenAIImagesUpstreamError {
	if account == nil || account.Platform != PlatformMidjourney || resp.StatusCode < 500 || resp.StatusCode > 599 {
		return nil
	}
	if gjson.GetBytes(body, "error.code").String() != "model_server_error" {
		return nil
	}
	// ModelVerse wraps the structured Midjourney rejection inside a 500 message.
	message := gjson.GetBytes(body, "error.message").String()
	_, nestedJSON, found := strings.Cut(message, "err: ")
	if !found {
		return nil
	}
	var rejection struct {
		Code        int    `json:"code"`
		Description string `json:"description"`
	}
	if json.Unmarshal([]byte(nestedJSON), &rejection) != nil || rejection.Code != 4 ||
		!strings.HasPrefix(strings.ToLower(strings.TrimSpace(rejection.Description)), "may contain sensitive words") {
		return nil
	}

	upstreamMessage := sanitizeUpstreamErrorMessage(message)
	detail := s.openAIImagesUpstreamErrorDetail(body)
	setOpsUpstreamError(c, resp.StatusCode, upstreamMessage, detail)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
		UpstreamStatusCode: resp.StatusCode, UpstreamRequestID: resp.Header.Get("x-request-id"),
		Kind: "http_error", Message: upstreamMessage, Detail: detail,
	})
	err := &OpenAIImagesUpstreamError{
		StatusCode: http.StatusBadRequest, ErrorType: "invalid_request_error",
		Code: "content_policy_violation", Param: "prompt",
		Message:           "Midjourney rejected the prompt because it may contain sensitive words. Please revise the prompt before retrying.",
		UpstreamRequestID: resp.Header.Get("x-request-id"),
	}
	if writeOpenAIImagesUpstreamErrorResponse(c, err) {
		MarkResponseCommitted(c)
	}
	return err
}
