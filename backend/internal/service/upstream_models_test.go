package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func upstreamModelSyncTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
}

func TestBuildV1ModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1"))
	require.Equal(t, "https://api.anthropic.com/v1/models", buildV1ModelsURL("https://api.anthropic.com/v1/models"))
	require.Equal(t, "https://gateway.example.com/antigravity/v1/models", buildV1ModelsURL("https://gateway.example.com/antigravity/"))
}

func TestBuildOpenAIModelsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		{
			name: "zhipu v4 coding base url",
			base: "https://open.bigmodel.cn/api/coding/paas/v4",
			want: "https://open.bigmodel.cn/api/coding/paas/v4/models",
		},
		{
			name: "openai v1 base url",
			base: "https://api.openai.com/v1",
			want: "https://api.openai.com/v1/models",
		},
		{
			name: "models url unchanged",
			base: "https://api.openai.com/v1/models",
			want: "https://api.openai.com/v1/models",
		},
		{
			name: "host fallback uses v1",
			base: "https://api.openai.com",
			want: "https://api.openai.com/v1/models",
		},
		{
			name: "trailing slash on v4",
			base: "https://open.bigmodel.cn/api/coding/paas/v4/",
			want: "https://open.bigmodel.cn/api/coding/paas/v4/models",
		},
		{
			name: "v2 base url",
			base: "https://gateway.example.com/openai/v2",
			want: "https://gateway.example.com/openai/v2/models",
		},
		{
			name: "v3 base url",
			base: "https://gateway.example.com/openai/v3",
			want: "https://gateway.example.com/openai/v3/models",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, buildOpenAIModelsURL(tt.base))
		})
	}
}

func TestBuildGeminiModelsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta"))
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta/models"))
}

func TestExtractUpstreamModelIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "openai and anthropic data array",
			body: `{"data":[{"id":"claude-sonnet-4-5"},{"id":"gpt-5"},{"id":"gpt-5"},{"id":""}]}`,
			want: []string{"claude-sonnet-4-5", "gpt-5"},
		},
		{
			name: "gemini models array strips prefix",
			body: `{"models":[{"name":"models/gemini-2.5-pro"},{"name":"gemini-2.5-flash"}]}`,
			want: []string{"gemini-2.5-flash", "gemini-2.5-pro"},
		},
		{
			name: "top level array",
			body: `[{"id":"z-model"},{"name":"models/a-model"}]`,
			want: []string{"a-model", "z-model"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := extractUpstreamModelIDs([]byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFilterUpstreamModelsForPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		models   []string
		want     []string
	}{
		{
			name:     "Kling keeps only Kling models",
			platform: PlatformKling,
			models:   []string{"kling-v3", "gpt-5", "Kling-v4"},
			want:     []string{"kling-v3", "Kling-v4"},
		},
		{
			name:     "Happy Horse keeps only Happy Horse models",
			platform: PlatformHappyHourse,
			models:   []string{"happyhorse-1.0-t2v", "kling-v3", "happyhorse-1.0-i2v"},
			want:     []string{"happyhorse-1.0-t2v", "happyhorse-1.0-i2v"},
		},
		{
			name:     "Seedance keeps only PP upstream model families",
			platform: PlatformSeedance,
			models: []string{
				"by-seedance2.0-933",
				"doubao-seedance-2-0-260128",
				"seedance2.0-431",
				"gpt-5",
			},
			want: []string{
				"doubao-seedance-2-0-260128",
				"seedance2.0-431",
			},
		},
		{
			name:     "ByteDance keeps only fixed ModelVerse models",
			platform: PlatformByteDance,
			models: []string{
				ByteDanceVideoDefaultModel,
				ByteDanceSeedance25Model,
				ByteDanceSeedance25GlobalModel,
				"doubao-seedance-2-0-mini-260615",
				"gpt-5",
			},
			want: []string{
				ByteDanceVideoDefaultModel,
				ByteDanceSeedance25Model,
				ByteDanceSeedance25GlobalModel,
			},
		},
		{
			name:     "Wan3.0 keeps only its fixed ModelVerse models",
			platform: PlatformWan3,
			models:   []string{Wan30VideoDefaultModel, Wan30VideoPrimeModel, "MiniMax-H3", "gpt-5"},
			want:     []string{Wan30VideoDefaultModel, Wan30VideoPrimeModel},
		},
		{
			name:     "MiniMax-H3 keeps only its fixed ModelVerse models",
			platform: PlatformMiniMaxH3,
			models:   []string{MiniMaxH3VideoDefaultModel, MiniMaxHailuo23VideoModel, "MiniMax-Video-01", "gpt-5"},
			want:     []string{MiniMaxH3VideoDefaultModel, MiniMaxHailuo23VideoModel},
		},
		{
			name:     "Pixverse v6 keeps only the fixed ModelVerse model",
			platform: PlatformPixverseV6,
			models:   []string{PixverseV6VideoDefaultModel, "pixverse-v5", "gpt-5"},
			want:     []string{PixverseV6VideoDefaultModel},
		},
		{
			name:     "Grok Imagine Video keeps only the fixed ModelVerse model",
			platform: PlatformGrokImagineVideo,
			models:   []string{GrokImagineVideoDefaultModel, "grok-imagine-video-1.5", "gpt-5"},
			want:     []string{GrokImagineVideoDefaultModel},
		},
		{
			name:     "MiniMax Speech keeps only live speech models",
			platform: PlatformMiniMaxSpeech,
			models:   []string{"speech-2.8-hd", "MiniMax-Speech-3", "MiniMax-H3", "gpt-5"},
			want:     []string{"speech-2.8-hd", "MiniMax-Speech-3"},
		},
		{
			name:     "Qwen TTS keeps only live Qwen TTS models",
			platform: PlatformQwenTTS,
			models:   []string{"qwen3-tts-flash", "qwen-tts-pro", "qwen3", "IndexTTS-2"},
			want:     []string{"qwen3-tts-flash", "qwen-tts-pro"},
		},
		{
			name:     "non video platforms are unchanged",
			platform: PlatformOpenAI,
			models:   []string{"gpt-5", "qwen3"},
			want:     []string{"gpt-5", "qwen3"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, filterUpstreamModelsForPlatform(tt.platform, tt.models))
		})
	}
}

func TestFetchUpstreamSupportedModelsUsesByteDanceFixedModelWithoutHTTPProbe(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       12,
		Platform: PlatformByteDance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "bd-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, byteDanceModels(), models)
	require.Empty(t, upstream.requests)
}

func TestFetchUpstreamSupportedModelsFetchesLiveAudioModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		body     string
		want     []string
	}{
		{
			name:     "MiniMax Speech",
			platform: PlatformMiniMaxSpeech,
			body:     `{"data":[{"id":"speech-2.9-hd"},{"id":"MiniMax-H3"},{"id":"speech-2.8-turbo"}]}`,
			want:     []string{"speech-2.8-turbo", "speech-2.9-hd"},
		},
		{
			name:     "Qwen TTS",
			platform: PlatformQwenTTS,
			body:     `{"data":[{"id":"qwen3-tts-flash"},{"id":"qwen-tts-pro"},{"id":"qwen3"}]}`,
			want:     []string{"qwen-tts-pro", "qwen3-tts-flash"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}}
			svc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

			models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
				ID:          99,
				Platform:    tt.platform,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":  "audio-key",
					"base_url": "https://api.modelverse.cn/v1",
				},
			})

			require.NoError(t, err)
			require.Equal(t, tt.want, models)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "https://api.modelverse.cn/v1/models", upstream.lastReq.URL.String())
			require.Equal(t, "Bearer audio-key", upstream.lastReq.Header.Get("Authorization"))
		})
	}
}

func TestFetchUpstreamSupportedModelsUsesMiniMaxH3FixedModelWithoutHTTPProbe(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       13,
		Platform: PlatformMiniMaxH3,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "h3-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{MiniMaxH3VideoDefaultModel, MiniMaxHailuo23VideoModel}, models)
	require.Empty(t, upstream.requests)
}

func TestFetchUpstreamSupportedModelsUsesPixverseV6FixedModelWithoutHTTPProbe(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       15,
		Platform: PlatformPixverseV6,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "pix-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{PixverseV6VideoDefaultModel}, models)
	require.Empty(t, upstream.requests)
}

func TestFetchUpstreamSupportedModelsUsesGrokImagineVideoFixedModelWithoutHTTPProbe(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       16,
		Platform: PlatformGrokImagineVideo,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "grok-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{GrokImagineVideoDefaultModel}, models)
	require.Empty(t, upstream.requests)
}

func TestFetchUpstreamSupportedModelsUsesWan30FixedModelsWithoutHTTPProbe(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       14,
		Platform: PlatformWan3,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "wan-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{Wan30VideoDefaultModel, Wan30VideoPrimeModel}, models)
	require.Empty(t, upstream.requests)
}

func TestBuildUpstreamModelsRequestsForAPIKeyAccounts(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	ctx := context.Background()

	anthropicReq, err := svc.buildAnthropicUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "anthropic-key",
			"base_url": "https://anthropic.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://anthropic.example.com/v1/models", anthropicReq.URL.String())
	require.Equal(t, "anthropic-key", anthropicReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", anthropicReq.Header.Get("anthropic-version"))

	anthropicBearerReq, err := svc.buildAnthropicUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "ollama-key",
			"base_url": "https://ollama.com",
		},
		Extra: map[string]any{
			"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://ollama.com/v1/models", anthropicBearerReq.URL.String())
	require.Equal(t, "Bearer ollama-key", anthropicBearerReq.Header.Get("Authorization"))
	require.Empty(t, anthropicBearerReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", anthropicBearerReq.Header.Get("anthropic-version"))

	openAIReq, err := svc.buildOpenAIUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://openai.example.com/v1/models", openAIReq.URL.String())
	require.Equal(t, "Bearer openai-key", openAIReq.Header.Get("Authorization"))

	doubaoReq, err := svc.buildUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformDoubao,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "ark-key",
			"base_url": "https://ark.cn-beijing.volces.com/api/v3",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/models", doubaoReq.URL.String())
	require.Equal(t, "Bearer ark-key", doubaoReq.Header.Get("Authorization"))

	grokReq, err := svc.buildUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "xai-key",
			"base_url": "https://xai.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://xai.example.com/v1/models", grokReq.URL.String())
	require.Equal(t, "Bearer xai-key", grokReq.Header.Get("Authorization"))

	geminiReq, err := svc.buildGeminiUpstreamModelsRequest(ctx, &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com/v1beta",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models", geminiReq.URL.String())
	require.Equal(t, "gemini-key", geminiReq.Header.Get("x-goog-api-key"))

	antigravityReq, err := svc.buildAntigravityAPIKeyModelsRequest(ctx, &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "antigravity-key",
			"base_url": "https://gateway.example.com/antigravity",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example.com/antigravity/v1/models", antigravityReq.URL.String())
	require.Equal(t, "antigravity-key", antigravityReq.Header.Get("x-api-key"))
}

func TestBuildUpstreamModelsRequestRejectsGrokOAuth(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildUpstreamModelsRequest(context.Background(), &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
	require.Contains(t, syncErr.SafeMessage(), "Unsupported Grok account type")
}

func TestBuildAntigravityAPIKeyModelsRequestRejectsOfficialCloudCodeBase(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildAntigravityAPIKeyModelsRequest(context.Background(), &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "antigravity-key",
			"base_url": "https://cloudcode-pa.googleapis.com",
		},
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
	require.Contains(t, syncErr.SafeMessage(), "compatible gateway")
}

func TestBuildAnthropicUpstreamModelsRequestRejectsBedrock(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
	_, err := svc.buildAnthropicUpstreamModelsRequest(context.Background(), &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeBedrock,
	})
	require.Error(t, err)

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUnsupported, syncErr.Kind)
}

func TestFetchUpstreamSupportedModelsParsesOpenAIResponse(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5"},{"id":"gpt-5"},{"name":"o3"}]}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       7,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5", "o3"}, models)
	require.Equal(t, "https://openai.example.com/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer openai-key", upstream.lastReq.Header.Get("Authorization"))
}

func TestFetchUpstreamSupportedModelsParsesGrokAPIKeyResponse(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"grok-4.5"},{"id":"grok-4.5"},{"id":"grok-imagine"}]}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       9,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "xai-key",
			"base_url": "https://xai.example.com/v1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"grok-4.5", "grok-imagine"}, models)
	require.Equal(t, "https://xai.example.com/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer xai-key", upstream.lastReq.Header.Get("Authorization"))
}

func TestFetchUpstreamSupportedModelsDoesNotExposeUpstreamBody(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"SECRET_TOKEN should not be exposed"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}

	_, err := svc.FetchUpstreamSupportedModels(context.Background(), &Account{
		ID:       8,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "openai-key",
			"base_url": "https://openai.example.com/v1",
		},
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET_TOKEN")

	var syncErr *UpstreamModelSyncError
	require.True(t, errors.As(err, &syncErr))
	require.Equal(t, UpstreamModelSyncErrorUpstream, syncErr.Kind)
	require.NotContains(t, syncErr.SafeMessage(), "SECRET_TOKEN")
	require.Contains(t, syncErr.SafeMessage(), "HTTP 502")
}
