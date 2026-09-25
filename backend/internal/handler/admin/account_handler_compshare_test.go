package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCompShareAccountModelsStaySeparateFromModelVerse(t *testing.T) {
	svc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: service.Account{
		ID: 61, Platform: service.PlatformMiniMaxH3CompShare, Type: service.AccountTypeAPIKey,
	}}
	router := setupAvailableModelsRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/61/models", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "MiniMax-H3")
	require.Contains(t, rec.Body.String(), "minimax-h3-lite")
	require.NotContains(t, rec.Body.String(), "MiniMax-Hailuo-2.3")

	upstream := &syncUpstreamHTTPUpstream{}
	router = setupSyncUpstreamModelsRouter(newStubAdminService(), upstream)
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/models/sync-upstream-preview", bytes.NewBufferString(`{"platform":"minimax-h3-compshare","type":"apikey","base_url":"https://cp.compshare.cn","api_key":"sk-ml-test"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, upstream.lastReq)
	require.Contains(t, rec.Body.String(), "minimax-h3-lite")
}
