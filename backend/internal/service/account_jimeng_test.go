package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJimengAPIKeyAccountIsOpenAICompatible(t *testing.T) {
	account := &Account{
		Platform: "jimeng",
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://jimeng-proxy.example.com/v1",
			"api_key":  "jm-secret",
		},
	}

	require.False(t, account.IsOpenAI())
	require.True(t, account.IsOpenAICompatible())
	require.Equal(t, "https://jimeng-proxy.example.com/v1", account.GetOpenAIBaseURL())
	require.Equal(t, "jm-secret", account.GetOpenAIApiKey())
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
}

func TestBuildUpstreamModelsRequestSupportsJimengPreviewCredentials(t *testing.T) {
	svc := &AccountTestService{cfg: upstreamModelSyncTestConfig()}

	req, err := svc.buildUpstreamModelsRequest(context.Background(), &Account{
		Platform: "jimeng",
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "jm-secret",
			"base_url": "https://jimeng-proxy.example.com/custom/v1",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "https://jimeng-proxy.example.com/custom/v1/models", req.URL.String())
	require.Equal(t, "Bearer jm-secret", req.Header.Get("Authorization"))
}

func TestJimengPlatformIsPreservedForOpenAICompatibleScheduling(t *testing.T) {
	require.Equal(t, "jimeng", normalizeOpenAICompatiblePlatform("jimeng"))
}
