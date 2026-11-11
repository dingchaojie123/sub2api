package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsPPVideoUnifiedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/v2/videos/generations", true},
		{"/v2/videos/generations/", true},
		{"/api/model-proxy/v2/videos/generations", true},
		{"/v1/videos/generations", false},
		{"/videos/generations", false},
		{"/v1/video/generations", false},
		{"/v2/videos/task_123", false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
		require.Equal(t, tc.want, isPPVideoUnifiedRequest(c), "path=%s", tc.path)
	}
}

func TestIsPPVideoUnifiedRequestToleratesMissingRequest(t *testing.T) {
	require.False(t, isPPVideoUnifiedRequest(nil))

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.False(t, isPPVideoUnifiedRequest(c))
}

func TestPPVideoSubmissionSourceBody(t *testing.T) {
	original := []byte(`{"prompt":"v2 body","assets":[{"type":"image","url":"https://example.com/a.png"}]}`)
	translated := []byte(`{"prompt":"v2 body","input":{"content":[]}}`)
	multipartJSON := []byte(`{"prompt":"multipart"}`)

	// A v2 request is prepared from its translated body, never from the client's original.
	require.Equal(t, translated, ppVideoSubmissionSourceBody(false, nil, translated, original))
	// A native request has no translation and starts from the client's own body.
	require.Equal(t, original, ppVideoSubmissionSourceBody(false, nil, nil, original))
	// A multipart submission starts from the JSON it was reduced to.
	require.Equal(t, multipartJSON, ppVideoSubmissionSourceBody(true, &service.PPVideoSubmissionBody{JSON: multipartJSON}, nil, original))
}
