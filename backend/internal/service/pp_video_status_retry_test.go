package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ppVideoStatusRetryUpstream plays back one scripted step per upstream call so a test can make a
// call fail, stall, or succeed in a chosen order. A call past the script fails loudly.
type ppVideoStatusRetryUpstream struct {
	steps    []func(req *http.Request) (*http.Response, error)
	requests []*http.Request
}

func (u *ppVideoStatusRetryUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	index := len(u.requests)
	u.requests = append(u.requests, req)
	if index >= len(u.steps) {
		return nil, errors.New("unexpected extra upstream request")
	}
	return u.steps[index](req)
}

func (u *ppVideoStatusRetryUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func ppVideoStatusRetryResetStep(_ *http.Request) (*http.Response, error) {
	return nil, errors.New(`Get "https://pp.example/v1/videos/text2video/provider-task-1": read tcp 10.0.0.1:5000->172.67.1.1:443: read: connection reset by peer`)
}

func ppVideoStatusRetrySucceededStep(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"provider-task-1","status":"succeeded","usage":{"duration":5.041,"video_count":1,"resolution":"1080p"}}`)),
	}, nil
}

func ppVideoStatusRetryAccount() *Account {
	return &Account{
		ID:          7,
		Platform:    PlatformKling,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "test-pp-token", "base_url": "https://pp.example/v1"},
	}
}

func ppVideoStatusRetryContext(method string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, "/v1/videos/text2video/provider-task-1", nil)
	return c
}

func TestForwardPPVideoStatusRetriesAfterTransportError(t *testing.T) {
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoStatusRetryResetStep,
		ppVideoStatusRetrySucceededStep,
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardPPVideoBuffered(context.Background(), ppVideoStatusRetryContext(http.MethodGet),
		ppVideoStatusRetryAccount(), PPVideoOperationKlingTextToVideo, "provider-task-1", nil)

	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, result.TaskStatus)
	require.Len(t, upstream.requests, 2)
	for _, req := range upstream.requests {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "https://pp.example/v1/videos/text2video/provider-task-1", req.URL.String())
		require.Equal(t, "Bearer test-pp-token", req.Header.Get("Authorization"))
	}
}

func TestForwardPPVideoStatusGivesUpAfterMaxAttempts(t *testing.T) {
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoStatusRetryResetStep,
		ppVideoStatusRetryResetStep,
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardPPVideoBuffered(context.Background(), ppVideoStatusRetryContext(http.MethodGet),
		ppVideoStatusRetryAccount(), PPVideoOperationKlingTextToVideo, "provider-task-1", nil)

	require.Nil(t, result)
	require.Error(t, err)
	require.Len(t, upstream.requests, ppVideoStatusMaxAttempts)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, "connection reset by upstream", failoverErr.TransportCause)
	// The poller log relies on this to show why a poll failed.
	require.Equal(t, "connection reset by upstream", ppVideoTransportCause(err))
}

func TestForwardPPVideoStatusRetriesAfterAttemptTimeout(t *testing.T) {
	previous := ppVideoStatusAttemptTimeout
	ppVideoStatusAttemptTimeout = 50 * time.Millisecond
	t.Cleanup(func() { ppVideoStatusAttemptTimeout = previous })

	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		// A stalled connection: nothing comes back until the attempt's own deadline cuts it off.
		func(req *http.Request) (*http.Response, error) {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(5 * time.Second):
				return nil, errors.New("the attempt deadline did not cut off the stalled request")
			}
		},
		ppVideoStatusRetrySucceededStep,
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	started := time.Now()
	result, err := svc.ForwardPPVideoBuffered(context.Background(), ppVideoStatusRetryContext(http.MethodGet),
		ppVideoStatusRetryAccount(), PPVideoOperationKlingTextToVideo, "provider-task-1", nil)

	require.NoError(t, err)
	require.Equal(t, PPVideoTaskStatusSucceeded, result.TaskStatus)
	require.Len(t, upstream.requests, 2)
	require.True(t, time.Since(started) < 3*time.Second, "the stalled attempt must be cut off by its own deadline")
}

func TestForwardPPVideoStatusTimeoutOnEveryAttemptReportsTimeout(t *testing.T) {
	previous := ppVideoStatusAttemptTimeout
	ppVideoStatusAttemptTimeout = 30 * time.Millisecond
	t.Cleanup(func() { ppVideoStatusAttemptTimeout = previous })

	stall := func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){stall, stall}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	_, err := svc.ForwardPPVideoBuffered(context.Background(), ppVideoStatusRetryContext(http.MethodGet),
		ppVideoStatusRetryAccount(), PPVideoOperationKlingTextToVideo, "provider-task-1", nil)

	require.Error(t, err)
	require.Len(t, upstream.requests, ppVideoStatusMaxAttempts)
	require.Equal(t, "upstream request timed out", ppVideoTransportCause(err))
}

func TestForwardPPVideoStatusDoesNotRetryAfterCallerDeadline(t *testing.T) {
	// The poller gives every task a deadline; once it has passed there is nobody left to wait.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoStatusRetryResetStep,
		ppVideoStatusRetrySucceededStep,
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	_, err := svc.ForwardPPVideoBuffered(ctx, ppVideoStatusRetryContext(http.MethodGet),
		ppVideoStatusRetryAccount(), PPVideoOperationKlingTextToVideo, "provider-task-1", nil)

	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
}

func TestForwardPPVideoSubmissionIsNeverRetried(t *testing.T) {
	upstream := &ppVideoStatusRetryUpstream{steps: []func(*http.Request) (*http.Response, error){
		ppVideoStatusRetryResetStep,
		ppVideoStatusRetrySucceededStep,
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	submissionBody := []byte(`{"model_name":"kling-v3","prompt":"a cat by the sea","duration":"5","mode":"pro"}`)

	_, err := svc.ForwardPPVideoBuffered(context.Background(), ppVideoStatusRetryContext(http.MethodPost),
		ppVideoStatusRetryAccount(), PPVideoOperationKlingTextToVideo, "", submissionBody)

	require.Error(t, err)
	// A repeated submission would create, and bill, a second video.
	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.MethodPost, upstream.requests[0].Method)
}

func TestPPVideoTransportCauseIgnoresOtherErrors(t *testing.T) {
	require.Equal(t, "", ppVideoTransportCause(nil))
	require.Equal(t, "", ppVideoTransportCause(errors.New("plain")))
	require.Equal(t, "", ppVideoTransportCause(&UpstreamFailoverError{StatusCode: http.StatusBadGateway}))
	wrapped := errors.Join(errors.New("outer"), &UpstreamFailoverError{StatusCode: http.StatusBadGateway, TransportCause: " upstream TLS error "})
	require.Equal(t, "upstream TLS error", ppVideoTransportCause(wrapped))
}
