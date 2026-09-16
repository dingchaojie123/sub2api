package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseQwenTTSSpeechRequest(t *testing.T) {
	request, err := ParseQwenTTSSpeechRequest([]byte(`{
        "model":"qwen3-tts-next",
        "input":"你好，世界",
        "voice":"Cherry",
        "metadata":{"language_type":"Chinese"}
    }`))
	require.NoError(t, err)
	require.Equal(t, "qwen3-tts-next", request.Model)
	require.Equal(t, "你好，世界", request.Input)
	require.Equal(t, "Cherry", request.Voice)
}

func TestParseQwenTTSSpeechRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"requires model", `{"input":"hello","voice":"Cherry"}`},
		{"requires input", `{"model":"qwen3-tts-flash","voice":"Cherry"}`},
		{"requires voice", `{"model":"qwen3-tts-flash","input":"hello"}`},
		{"rejects unsupported language", `{"model":"qwen3-tts-flash","input":"hello","voice":"Cherry","metadata":{"language_type":"Thai"}}`},
		{"rejects more than 600 characters", `{"model":"qwen3-tts-flash","input":"` + strings.Repeat("a", 601) + `","voice":"Cherry"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseQwenTTSSpeechRequest([]byte(tt.body))
			require.Error(t, err)
		})
	}
}

func TestNormalizeAudioBaseURL(t *testing.T) {
	baseURL, err := normalizeAudioBaseURL("https://proxy.example.com")
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com/v1", baseURL)

	baseURL, err = normalizeAudioBaseURL("https://proxy.example.com/v1/")
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com/v1", baseURL)

	_, err = normalizeAudioBaseURL("https://proxy.example.com/v1?tenant=qwen")
	require.Error(t, err)
}

func TestForwardQwenTTSSpeechUsesCharactersForBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"qwen3-tts-next","input":"hello","voice":"Cherry"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, QwenTTSSpeechEndpoint, bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
            "request_id":"request-123",
            "code":"",
            "output":{"audio":{"url":"https://audio.example.com/result.wav"}},
            "usage":{"input_tokens":0,"output_tokens":0,"characters":195}
        }`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:          42,
		Platform:    PlatformQwenTTS,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "qwen-secret",
			"base_url": "https://api.modelverse.cn/v1",
		},
	}

	forward, err := svc.ForwardQwenTTSSpeech(context.Background(), c, account, body, "qwen3-tts-next")
	require.NoError(t, err)
	require.NotNil(t, forward)
	require.True(t, forward.Successful)
	require.Equal(t, "qwen3-tts-next", forward.Result.Model)
	require.Equal(t, 195, forward.Result.Usage.InputTokens)
	require.Equal(t, "request-123", forward.Result.RequestID)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://api.modelverse.cn/v1/audio/speech", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer qwen-secret", upstream.lastReq.Header.Get("Authorization"))
	require.JSONEq(t, string(body), string(upstream.lastBody))
}
