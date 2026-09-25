package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMidjourneyImagesValidationErrorRemainsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name      string
		endpoint  string
		multipart bool
		stream    bool
		model     string
		message   string
	}{
		{name: "edits", endpoint: "/v1/images/edits", message: "JSON requests only"},
		{name: "multipart", multipart: true, message: "JSON requests only"},
		{name: "streaming", stream: true, message: "does not support streaming"},
		{name: "action without task", model: "midjourney-fast-variation", message: "parameters.mj_task_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := tc.endpoint
			if endpoint == "" {
				endpoint = "/v1/images/generations"
			}
			model := tc.model
			if model == "" {
				model = "midjourney-fast-imagine"
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{}`))
			parsed := &service.OpenAIImagesRequest{
				Endpoint: endpoint, Multipart: tc.multipart, Stream: tc.stream,
				Model: model, ExplicitModel: true, Prompt: "A mountain landscape",
			}
			svc := &service.OpenAIGatewayService{}
			account := &service.Account{Platform: service.PlatformMidjourney, Type: service.AccountTypeAPIKey}
			before := c.Writer.Size()
			result, err := svc.ForwardImages(context.Background(), c, account, []byte(`{}`), parsed, "")
			require.Error(t, err)
			require.Nil(t, result)

			// Exercise the handler fallback with JSON keepalive disabled.
			h := &OpenAIGatewayHandler{}
			if !openAIForwardErrorAlreadyCommunicated(c, before, err) {
				require.False(t, h.ensureForwardErrorResponse(c, false))
			}
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.True(t, json.Valid(rec.Body.Bytes()), rec.Body.String())
			require.Contains(t, rec.Body.String(), tc.message)
			require.NotContains(t, rec.Body.String(), "event: error")
		})
	}
}
