package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func midjourneyModerationTestBody(t *testing.T, code int, description string) []byte {
	t.Helper()
	inner, err := json.Marshal(map[string]any{"code": code, "description": description, "type": "upstream_error"})
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{"error": map[string]any{
		"code": "model_server_error", "type": "internal_error",
		"message": "[trace_id: test-trace] Request llm server Error: server error, status code: 500, err: " + string(inner),
	}})
	require.NoError(t, err)
	return body
}

func TestMidjourneyImagesModerationDoesNotFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stage := range []string{"submit", "poll"} {
		t.Run(stage, func(t *testing.T) {
			body := []byte(`{"model":"midjourney-fast-imagine","prompt":"A fashion reference sheet","size":"1024x1536"}`)
			writer := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			moderationBody := midjourneyModerationTestBody(t, 4, "May contain sensitive words (request id: test-request)")
			responses := []*http.Response{}
			if stage == "poll" {
				responses = append(responses, &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"output":{"task_id":"mj-task"}}`))})
			}
			responses = append(responses, &http.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(moderationBody))})
			upstream := &httpUpstreamRecorder{responses: responses}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 59, Platform: PlatformMidjourney, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.modelverse.cn/v1"}}
			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
			require.Nil(t, result)
			var clientErr *OpenAIImagesUpstreamError
			require.ErrorAs(t, err, &clientErr)
			require.False(t, IsOpenAIImagesRetryableUpstreamError(clientErr))
			require.Equal(t, http.StatusBadRequest, writer.Code)
			require.Equal(t, "content_policy_violation", gjson.Get(writer.Body.String(), "error.code").String())
			require.Contains(t, writer.Body.String(), "prompt")
			require.NotContains(t, writer.Body.String(), "test-trace")
			require.True(t, IsResponseCommitted(c))
			require.Len(t, upstream.requests, len(responses))
		})
	}
}

func TestMidjourneyImagesModerationLeavesOtherErrorsUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		platform string
		status   int
		body     []byte
	}{
		{name: "other platform", platform: PlatformOpenAI, status: 500, body: midjourneyModerationTestBody(t, 4, "May contain sensitive words")},
		{name: "authentication error", platform: PlatformMidjourney, status: 401, body: midjourneyModerationTestBody(t, 4, "May contain sensitive words")},
		{name: "different code", platform: PlatformMidjourney, status: 500, body: midjourneyModerationTestBody(t, 5, "May contain sensitive words")},
		{name: "server outage", platform: PlatformMidjourney, status: 500, body: midjourneyModerationTestBody(t, 4, "Server temporarily unavailable")},
		{name: "unstructured mention", platform: PlatformMidjourney, status: 500, body: []byte(`{"error":{"code":"model_server_error","message":"May contain sensitive words"}}`)},
		{name: "invalid nested JSON", platform: PlatformMidjourney, status: 500, body: []byte(`{"error":{"code":"model_server_error","message":"err: {invalid}"}}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			svc := &OpenAIGatewayService{}
			err := svc.handleMidjourneyImagesModerationError(c, &Account{Platform: tc.platform}, &http.Response{StatusCode: tc.status}, tc.body)
			require.Nil(t, err)
			require.False(t, c.Writer.Written())
			require.False(t, IsResponseCommitted(c))
		})
	}
}
