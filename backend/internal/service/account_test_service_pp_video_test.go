//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountTestService_PPVideoUsesModelsProbeInsteadOfClaudeMessages(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          22,
		Platform:    PlatformKling,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "pp-key",
			"base_url": "https://app.ppapi.ai/v1",
		},
	}
	repo := &mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{account.ID: account},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"kling-v3"}]}`)),
	}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	c, rec := newTestContext()
	c.Request = c.Request.WithContext(context.Background())

	err := svc.TestAccountConnection(c, account.ID, "", "", "")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Equal(t, "https://app.ppapi.ai/v1/models", upstream.lastReq.URL.String())
	require.NotContains(t, upstream.lastReq.URL.String(), "/v1/v1/messages")
	require.Equal(t, "Bearer pp-key", upstream.lastReq.Header.Get("Authorization"))
	require.Contains(t, rec.Body.String(), `"success":true`)
	require.Contains(t, rec.Body.String(), "kling-v3")
}
