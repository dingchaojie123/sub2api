package admin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test88APISyncPreviewUsesLiveModels(t *testing.T) {
	upstream := &syncUpstreamHTTPUpstream{resp: &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"SD2.5 720P"},{"id":"future-model-4k"}]}`))}}
	router := setupSyncUpstreamModelsRouter(newStubAdminService(), upstream)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/models/sync-upstream-preview", strings.NewReader(`{"platform":"88api-video","type":"apikey","base_url":"https://88api.ai/v1","api_key":"test-key"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, "https://88api.ai/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer test-key", upstream.lastReq.Header.Get("Authorization"))
	require.Contains(t, rec.Body.String(), "future-model-4k")
}
