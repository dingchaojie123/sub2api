package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type jimengUpdateHTTPUpstream struct {
	resp    *http.Response
	err     error
	lastReq *http.Request
}

func (u *jimengUpdateHTTPUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	if u.err != nil {
		return nil, u.err
	}
	return u.resp, nil
}

func (u *jimengUpdateHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestAccountHandlerUpdate_JimengPreservesExistingAPIKeyWhenUpdatingModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	adminSvc.getAccountResult = &service.Account{
		ID:       46,
		Name:     "jimeng-apikey",
		Platform: service.PlatformJimeng,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"api_key":  "jm-key",
			"base_url": "https://jimeng-proxy.example.com/v1",
			"model_mapping": map[string]any{
				"existing-model": "existing-model",
			},
		},
	}

	upstream := &jimengUpdateHTTPUpstream{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"by-seedance2.0-933"}]}`)),
		},
	}
	accountTestSvc := service.NewAccountTestService(
		nil,
		nil,
		nil,
		nil,
		nil,
		upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		nil,
	)

	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/accounts/:id", handler.Update)

	body, err := json.Marshal(map[string]any{
		"credentials": map[string]any{
			"base_url": "https://jimeng-proxy.example.com/v1",
			"model_mapping": map[string]any{
				"by-seedance2.0-933": "by-seedance2.0-933",
			},
		},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/46", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, adminSvc.updateAccountCalls)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "Bearer jm-key", upstream.lastReq.Header.Get("Authorization"))
	require.True(t, strings.HasSuffix(upstream.lastReq.URL.Path, "/v1/models"))
}
